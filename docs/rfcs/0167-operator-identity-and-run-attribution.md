# RFC 0167: Operator identity and run attribution (leased handles on the principal model)

Status: accepted
Date: 2026-06-23
Decision: D260 — accepted 2026-06-23 by the human principal, directing autonomous
design → build → verify landing (defer nothing). Design-run falsification still
runs as the first build-pipeline stage, not as an acceptance gate.
Context: RFC 0107 (multi-principal trust model — `principals`,
`principal_clients`); RFC 0053 (human-principal and terminology truing);
RFC 0114 (read-scope principals/sessions); RFC 0026 (lane attestation and
operator-byline honesty); RFC 0122 (scheduler principal auto-spawn);
RFC 0127 (retire lane git identity); RFC 0033 + migration 0033 (daemon-owned
PostgreSQL as authoritative state; reconcile/heartbeat sweep). Origin:
operator request — a single human running ~15 concurrent terminal operators
cannot tell which operator owns which run.
author: proposer-claude-opus-4-8-001

## Problem

A single human routinely drives many concurrent Striatum operators — separate
terminal sessions, each bootstrapped against the one local daemon, often across
several registered target repositories. Today nothing ties a run back to the
operator that started it. A run is identified only by an opaque
`run_02c4fc6ad7cb5092ae4d5c67651e22a8`. When fifteen terminals are open and one
run wedges at a checkpoint, the human has no fast way to answer *which of these
windows owns this run* short of grepping scrollback in each pane.

The seed proposal: give each operator a memorable name at initialization and
stamp every run with that name as a byline, so a glance maps a run to an
operator.

The naive form of that idea fails three ways, and the fixes are the substance of
this RFC:

1. **A name is an assertion, not a proof.** On a shared box (one OS user, one
   daemon socket) a memorable string carries no authority. If attribution is
   read off terminal titles, env vars, or tmux panes — all explicitly
   non-authoritative under the product boundary — the daemon will confidently
   print the *wrong* operator, which is worse than printing nothing.
2. **Names are finite and reused.** Two live operators want to be `maya`; a name
   is reclaimed after a restart; a human renames themselves mid-run. Without a
   stable identity underneath, the friendly word silently relabels history.
3. **A run changes hands.** Recovery requeues it, a fresh operator resumes a
   wedged run, the auto-spawn scheduler (RFC 0122) creates child runs under a
   captured grant. "Who started it" and "who holds it now" are different facts;
   one byline cannot carry both honestly.

## What already exists (do not rebuild it)

This RFC deliberately rides the RFC 0107 multi-principal substrate rather than
adding a parallel identity system:

- `striatumd.principals` — `principal_id` (PK), `principal_kind`
  (`human` | `ai_operator` | `service` | …), `display_name`, `disabled_at`. A
  human operator already maps to a `principal_kind='human'` row. **The immutable
  operator-id this RFC needs already exists: it is `principal_id`.**
- `striatumd.principal_clients` — the rotation-stable attribution graph binding
  capability-token client identities to a principal. Token-binding of a name is
  therefore a dereference, not new machinery.
- `striatumd.sessions.last_session_heartbeat_at` — the liveness signal the
  reconcile sweep (migration 0033) already maintains and reaps. A handle lease
  can reuse it directly.
- `runs` has **no** operator column yet; `branch_confirmed_by` is the closest
  existing attribution field and is a viable backfill source.

The gap is narrow: a *leased, memorable rendering layer* over `principal_id`, a
*write-once run stamp*, an *append-only custody log*, and the *read surfaces* that
make all of it glanceable.

## Goals

- Map any run to the operator that started it, and to the operator that holds it
  now, from authoritative PostgreSQL alone.
- Give operators memorable, privacy-safe, lowercase handles that are stable
  across reconnects and unique among *live* operators on a repo.
- Make the friendly handle non-load-bearing: every surface renders a
  disambiguating id-suffix, and identity degrades to the bare id when a name
  cannot be proven.
- Keep durable artifact provenance (the `author:` byline) honest under rename
  and handle reuse.
- Surface attribution where the human already looks: `bootstrap`, `status`,
  `dashboard`, a new `whose` reverse-lookup, and `striatum-handoff` filenames.

## Non-goals

- Multi-user authentication or cross-machine identity. This is single-human,
  single-daemon legibility; the trust model is RFC 0107's, unchanged.
- Any hosted service, directory, telemetry, or external identity provider.
- Treating terminal titles, tmux state, or env vars as authoritative. They may
  be *rendered to* (display-only) but never *read from* for state.
- Replacing `run_id`. Run-ids stay opaque and stable; the handle is never
  encoded into them.

## Design

### D1 — Operator identity is `principal_id`; the handle is a leased label

At `striatum operator bootstrap` the daemon, inside the same transaction that
mints the session capability token:

1. Resolves/creates the caller's `principal` (`kind='human'`).
2. Mints the token by linking a fresh `principal_clients` row (existing rotation
   path), so the live token dereferences to exactly one `principal_id`.
3. Acquires a **handle lease** in a new owner-held table:

```sql
-- owner bundle migration (touches owner-held / FK-bearing tables)
CREATE TABLE striatumd.operator_handles (
  handle_id        text PRIMARY KEY,
  repository_id    text NOT NULL,
  principal_id     text NOT NULL REFERENCES striatumd.principals(principal_id),
  handle           text NOT NULL,              -- lowercase, privacy-safe
  leased_session_id text NOT NULL,
  leased_until     timestamptz NOT NULL,
  released_at      timestamptz
);
-- uniqueness is scoped to LIVE handles on a repo, matching the misattribution
-- blast radius (RFC 0107 namespacing; same word on a different repo is fine).
CREATE UNIQUE INDEX operator_handles_live_uq
  ON striatumd.operator_handles (repository_id, lower(handle))
  WHERE released_at IS NULL;
```

The rendered identity is `handle#suffix`, e.g. `maya#7f3`, where `suffix` is a
short stable slice of `principal_id` (first hex of a hash) — **computed, not
stored**. The suffix is the truth the human is trained to read; the bare word is
a convenience. When the token is expired/revoked the client no longer
dereferences to a live principal and the renderer falls back to the bare
`principal_id` — "the name lapses to the id" is free, not a special case.

**Leased, not owned.** The lease reuses `sessions.last_session_heartbeat_at`: the
same reconcile sweep that reaps stale sessions flips `released_at` and returns the
word to the pool. A dead operator's handle cannot be impersonated by a survivor
because the live-uniqueness index only constrains un-released rows. Heartbeat must
*renew an existing lease*, never release-then-reacquire, or a racing session could
steal the word during a flap.

**Default handles are deterministic.** Most operators will not pick a name; mint a
deterministic adjective-animal (or first-name) from a hash of `principal_id` so the
same human reattaches to the same word per repo across reconnects, escalating to
`#suffix` only on a live collision. Memorable, stable, self-healing, zero naming UI.

### D2 — Write-once run origin stamp

```sql
ALTER TABLE runs ADD COLUMN created_by_principal_id text;  -- owner bundle
```

Stamped exactly once in the run-creation transaction; **write-once enforced at
the database** (REVOKE UPDATE on the column from the runtime role, or a BEFORE
UPDATE trigger that raises on change). Backfillable from `branch_confirmed_by`.
"Which operator owns run_02c4fc" becomes a column join, not a guess. The same
column carries `principal_kind='service'`/`'ai_operator'` rows, so an auto-spawned
scheduler run (RFC 0122) renders `scheduler#a19` next to `maya#7f3` with the same
rules — autonomous runs get provenance for free, no second scheme.

### D3 — Custody over time (origin vs current holder)

Origin is immutable; custody is a log, not a mutable `current_owner`:

```sql
CREATE TABLE striatumd.run_custody_log (   -- append-only
  run_id     text NOT NULL,
  principal_id text NOT NULL,
  transition text NOT NULL,   -- create | resume | requeue | recover | spawn_inherit | grant_expired
  reason     text,
  seq        bigserial,
  at         timestamptz NOT NULL DEFAULT now()
);
```

Current holder = the last `seq` row for the run. Every handoff, recovery resume,
or scheduler grant **appends** in the same transaction as the state transition
that triggered it; there is no client RPC that can forge custody. This folds
directly into existing pain: when a stale lease is reaped (#579, #292) the reaper
appends a `recover` row naming the dead operator as `from`, turning "who took over
the wedged run" into a queryable fact instead of log-scraping. When the auto-spawn
captured grant expires mid-run (the recurring creds-expire wedge), append
`grant_expired` and refuse further spawns under it.

### D4 — Lineage of spawned runs (optional, later phase)

A dotted `runs.lineage_id` (LTREE or text), child = `parent.lineage_id ||
nextval(per-parent)`, gives `maya.3.1` and reveals the whole spawn tree, including
scheduler-spawned children stamped under the captured-grant principal. Lets an
operator inheriting `run_02c4` see at once: *started by maya, now held by you,
spawned 3 children, currently driven by the scheduler under maya's grant.* Deferred
to a later phase because the attribution problem is solved by D1–D3; lineage is
situational-awareness polish.

### D5 — Honest artifact bylines

Durable provenance keys on the immutable id, never the mutable handle. At publish
time the daemon persists `(principal_id, handle_snapshot, ordinal)` in the
artifact's anchor metadata and derives the displayed byline from `principal_id`.
A later self-rename re-displays the new handle in live views but cannot rewrite the
id baked into already-published provenance — laundering is impossible. The existing
byline shape `author: <role>-<model>-<ordinal>` is preserved; an optional
operator suffix `author: <role>-<model>-<ordinal>@maya#7f3` binds the durable
artifact to the operator who drove the run while staying lowercase and privacy-safe,
and the committed `#suffix` is independently verifiable against handle history.

### D6 — Legibility surfaces (the payoff)

All read from the PG identity join; display-only layers carry zero workflow
meaning and fail closed.

- **`striatum whose <run-id>`** — new read-only verb. One line:
  `handle#suffix`, run state/phase, and a paste-able switch-here hint. Built
  first, as a pure `{handle, suffix, run_state, phase}` join that *cannot lie* —
  no tty/pane/title in the authoritative answer.
- **Operator manifest** — `striatum operator bootstrap` and `striatum status
  --mine` gain a "your load this session" section: this operator's non-terminal
  runs as chip + run-id + `[phase/blocked]`, attention-need first. Answers the
  standing question "what am I on right now".
- **Pre-attentive chips** — one shared pure function `handle -> {color, glyph}`
  (e.g. FNV hash into a fixed 16-color/glyph palette) in a single Go package,
  consumed identically by `whose`, `status`, `bootstrap`, and `dashboard`, with a
  golden test pinning stability. `maya` is the same chip everywhere, forever.
- **Opt-in terminal title** — `bootstrap` can emit an OSC-2/OSC-0 title escape
  (`maya:run_02c4 [blocked]`), gated strictly on `stdout` being a TTY *and* an
  explicit flag/env, with restore-on-exit. Highest-blast-radius display piece;
  fail-closed so it can never masquerade as state.
- **Handoff filenames carry the handle** — `striatum-handoff-<handle>-<date>.md`
  (e.g. `striatum-handoff-maya-2026-06-23.md`). `bootstrap`/`operator-report`
  emit an authoritative `handoff_filename` field so the `striatum-handoff` skill
  reads it instead of hand-templating, and no filename can disagree with the
  daemon's identity. Generalizes to any operator-authored artifact.

### D7 — Doctor integrity rules

Match Striatum's doctor-as-provenance-guard culture (the `artifact_anchor_*`
family):

- `attribution_unknown` (advisory, not red) — non-terminal runs whose
  `created_by_principal_id` is NULL/unresolvable. Surfaces the load-bearing risk
  as visible, fixable debt rather than silent misattribution; kept out of the
  hard-integrity lane so it never blocks dogfoods.
- `custody_chain_gap` / `origin_byline_mismatch` (integrity) — every run has
  exactly one origin row, a contiguous custody `seq` with no UPDATEs, and every
  published artifact's anchored `principal_id` resolves to a live `principals`
  row.

## The load-bearing risk

**Attribution is only as honest as the identity stamped at token-mint /
run-creation time, against the live token, inside the transaction.** On a shared
box, if the suffix or the `created_by` stamp is ever derived client-side, after
the fact, or from a display signal (tty, tmux, title, env), an operator can spoof
another's handle and the entire "the id is the truth" guarantee inverts into "the
name lies *and* the id is forgeable." Every surface — including `doctor` and
evidence export — must resolve identity through `principal_id` and only
*snapshot* the handle for display. Secondary risk: handle-lease flap; renewal must
extend the existing lease, never release-then-acquire.

## Phasing

- **P0 — identity + stamp + reverse lookup.** Owner-bundle migration:
  `operator_handles` (with the live-unique index) + `runs.created_by_principal_id`
  write-once. Bootstrap mints/leases the handle; `striatum whose <run-id>` and the
  bootstrap/`status --mine` manifest read it. pgtests: two-live-`maya` collision;
  token-revoked → bare-id render; forged UPDATE to `created_by_principal_id`
  rejected. This phase alone retires the stated problem.
- **P1 — custody.** `run_custody_log`; fold `recover`/`requeue` appends into the
  existing reap/resume transactions; `doctor` custody rules.
- **P2 — honest bylines + handoff naming.** Byline derivation from `principal_id`;
  `handoff_filename` surfaced; chip function + golden test; opt-in OSC title.
- **P3 — lineage.** `lineage_id`, spawn-tree read, `grant_expired` bookend.

## Alternatives considered (and why not)

- **Encode the operator name into the run-id (barcode/check-digit/depot-prefix).**
  Rejected. Couples a *mutable, colliding* human name into an *immutable, opaque*
  identifier; breaks on rename, reuse, and handoff. Run-ids stay opaque.
- **Derive the handle deterministically from tty + boot-time.** Rejected as the
  primary scheme. A tty hash is not memorable, and tty churns across reconnects.
  Retained only as the *default-name* seed (hash of `principal_id`, not tty).
- **Pure display-only attribution (no PG), e.g. window-raise via wmctrl.**
  Rejected as the foundation. Re-imports non-authoritative state as truth and is
  environment-specific (X11/Wayland). Allowed only as an opt-in convenience layer
  on top of an authoritative `whose`.
- **A standalone operator-identity table independent of `principals`.** Rejected.
  Duplicates RFC 0107 and would drift from the attestation/rotation model; the
  operator-id *is* a `principal_id`.

## Open questions

1. Handle pool: curated first-names vs adjective-animal vs operator-chosen with a
   reserved-word denylist? (Privacy-safe + memorable is the only hard constraint.)
2. Should `created_by_principal_id` be backfilled from `branch_confirmed_by` for
   historical runs, or left NULL (advisory `attribution_unknown`) below a cutover?
3. Cross-repo view: does the human want a *daemon-wide* "all my operators across
   all repos" board, or is per-repo sufficient? (D6 manifest is per-repo today.)
4. Is the `@handle#suffix` artifact-byline suffix in scope for P2, or does it
   belong to a follow-up given RFC 0026's existing byline-honesty surface?

## Decision

**Accepted (D260, 2026-06-23, human principal).** The path is P0 (identity +
write-once stamp + `whose` + manifest) as the minimum that retires the problem,
then P1–P3 in sequence. Acceptance directs the implementing agent to land this
**autonomously, deferring nothing** — scaffold and drive the Striatum
design → build → verify workflows for it rather than hand-cranking. The design
run (falsification) runs as the first pipeline stage to harden P0's owner-bundle
migration, not as a gate on acceptance; the owner-bundle migration is the gating,
hardest-to-reverse change and must be proven under the two-role pgtest fixture
(RFC 0142 P0) first. Open questions above are resolved during the design run, not
deferred out of scope.
