# Design-Run Seed — RFC 0167 P0 (operator identity & run attribution)

> This document is the **required input** for the RFC 0167 **P0** design run. It is
> operator-supplied design-run scaffolding, not the canonical RFC. The accepted
> RFC lives on `main` at `docs/rfcs/0167-operator-identity-and-run-attribution.md`
> (Status: accepted, decision D260) and is a required context doc for this run.
> Read this whole file, then the RFC, before producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. The deliverable (the
committed `PROPOSAL.md`, henceforth "the SPEC") is a **falsifiable
implementation spec for RFC 0167 P0**: a concrete, buildable specification that
the downstream `rfc-0167-p0-build` `code_change` run executes contract-first.
It is produced by hardening the accepted RFC's P0 against adversarial
falsification.

RFC 0167 P0 is, verbatim from the RFC §Phasing:

> **P0 — identity + stamp + reverse lookup.** Owner-bundle migration:
> `operator_handles` (with the live-unique index) + `runs.created_by_principal_id`
> write-once. Bootstrap mints/leases the handle; `striatum whose <run-id>` and the
> bootstrap/`status --mine` manifest read it. pgtests: two-live-`maya` collision;
> token-revoked → bare-id render; forged UPDATE to `created_by_principal_id`
> rejected. This phase alone retires the stated problem.

The SPEC MUST satisfy every requirement below. A design run that leaves any one
unresolved has **not** cleared the gate.

## R1 — Resolve the load-bearing risk, two faces, concretely

The RFC names one load-bearing risk (honesty). Building it surfaces a second
(sufficiency). The SPEC must resolve **both** with a mechanism anchored to real
current source (`go/pkg/...`), not prose.

### R1a — Honesty: identity is bound server-side, at token-mint, against the live token

From RFC §"The load-bearing risk": attribution is only as honest as the identity
stamped **at token-mint / run-creation time, against the live token, inside the
transaction**. On a shared box (one OS user, one daemon socket, ~15 terminals)
the SPEC must show that:

- The principal is resolved/created and the handle leased **inside the same
  daemon-side transaction that mints the session capability token** (RFC D1),
  never derived client-side, after the fact, or from a display signal
  (tty, tmux, terminal title, env var). Anchor to the real bootstrap /
  token-mint path in current source (the `principals` / `principal_clients`
  mint, RFC 0107).
- `runs.created_by_principal_id` is resolved from the **live token presented on
  the run-creation RPC**, server-side, never from a client-supplied name.
- Every read surface (`whose`, `status --mine`, `doctor`, evidence export)
  resolves identity through `principal_id` and only *snapshots* the handle for
  display.

State, as a falsifiable assertion + the test that would refute it, the claim:
"no client input and no display signal can cause run X created by operator A to
render as operator B's handle."

### R1b — Sufficiency: per-human `principal_id` vs per-terminal session granularity

This is the sharp, easy-to-miss gap and the SPEC must confront it head-on. Under
RFC 0107 **one human maps to one `principal_kind='human'` principal_id**. But the
stated problem is *"fifteen terminals are open and one run wedges — which of these
windows owns it"* — a **per-terminal-session** question. If all 15 terminals of
one human share one `principal_id`, then `runs.created_by_principal_id` alone
**cannot distinguish them**, and the suffix `#7f3` (computed from `principal_id`)
is **identical** for all 15 — so the suffix cannot disambiguate same-human
terminals either.

The SPEC must specify exactly how P0 still answers "which window". The RFC's
machinery points the way and the SPEC must make it explicit and provable:

- The **handle** is leased **per session** (`operator_handles.leased_session_id`),
  and the live-unique index `(repository_id, lower(handle)) WHERE released_at IS
  NULL` forces two concurrent same-human sessions onto **distinct words** (the
  second `maya` cannot lease while the first holds it). So within one human's
  terminals the **word** (maya vs the next deterministic pick) is the
  disambiguator; across humans the **#suffix** is. The default-handle generator
  (deterministic from `principal_id`) therefore needs a **collision-escalation
  rule** (what the second same-human session is named) that is itself
  deterministic and stable across reconnect.
- `whose <run-id>` and the `status --mine` manifest must therefore render the
  **per-session leased handle that owned the run at creation**, not merely
  `created_by_principal_id`'s bare handle — otherwise two same-human runs are
  indistinguishable, and P0 fails its own goal. The SPEC must specify the
  exact join (run → which lease? the lease live at creation? a handle snapshot
  stamped on the run?) and prove the two-same-human-terminals case returns two
  distinct answers.

This sufficiency resolution is the single most important thing this design run
must get right. If the SPEC stamps only `created_by_principal_id` and renders
only the principal-derived handle, **P0 does not retire the stated problem** and
the gate must not clear.

### R1c — Lease flap (secondary risk)

Heartbeat must **renew an existing lease** (extend `leased_until`), never
release-then-reacquire, or a racing same-human session could steal the word
during a flap. The SPEC must specify the renewal as an UPDATE of the existing
row guarded so it cannot transit through a released state, and name the test
that refutes "a flap lets another session steal the handle".

## R2 — The owner-bundle migration is the gating, hardest-to-reverse change

P0 adds an `operator_handles` table and `ALTER runs ADD COLUMN
created_by_principal_id`, both touching owner-held / FK-bearing tables. This is an
**owner bundle**, NOT a runtime migration (a runtime ALTER of an owner table
boot-crashes the single writer with `42501`; D248 / D258). The SPEC must:

- Specify the migration as an owner bundle at the next free ordinal (current
  `LatestOwnerBundleVersion == 20`; verify against `go/pkg/db/owner.go` on
  current `main` and use the next ordinal), deployed via `striatum daemon
  owner-ddl apply` **then** a restart, in lockstep.
- Choose and specify the **write-once enforcement at the database**: either
  `REVOKE UPDATE (created_by_principal_id) ON runs FROM <runtime role>` (column
  privilege; the runtime role keeps `INSERT` to stamp it once and `UPDATE` on
  other columns) **or** a `BEFORE UPDATE` trigger that raises when the column
  changes from a non-NULL value. Pick one, justify it, and pin the privileges
  the runtime role must **retain** (INSERT a run row with the column;
  INSERT/UPDATE `operator_handles` for lease + heartbeat renewal +
  reconcile-sweep release) against the owner-bundle grant model.
- Prove the bundle applies cleanly and the write-once REVOKE/trigger holds
  **under the RFC 0142 P0 two-role pgtest fixture (non-superuser owner DSN)** —
  a single-role pgtest will not catch the `42501` / privilege gaps. Name the
  exact pgtests: two-live-`maya` collision (the live-unique index), token-revoked
  → bare-id render, forged UPDATE to `created_by_principal_id` rejected at the DB.
- Keep the migration **forward-only** and consistent with the owner-bundle
  watermark / `RequiredOwnerBundleVersion` rules (do not advance `Required`
  unless the deploy path demands it; justify whichever you choose).

## R3 — Resolve all four RFC Open Questions with a defensible decision

Each must land as a concrete decision (in-P0 / deferred, which mechanism, why):

1. **Handle pool** — curated first-names vs adjective-animal vs operator-chosen
   with a reserved-word denylist. Hard constraints: privacy-safe, lowercase,
   memorable, and a **deterministic default from a hash of `principal_id`** so a
   human reattaches to the same word per repo across reconnects. Pick the pool
   and the collision-escalation order; specify the denylist mechanism if any.
2. **Backfill vs NULL** — backfill `created_by_principal_id` from
   `branch_confirmed_by` for historical runs, or leave NULL below a cutover with
   the advisory `attribution_unknown` doctor rule. Decide for P0.
3. **Cross-repo board** — daemon-wide "all my operators across all repos" vs
   per-repo. Decide P0 scope (the RFC's D6 manifest is per-repo today); if
   deferred, say so explicitly and to which phase.
4. **`@handle#suffix` artifact byline** — in P0/P2 scope or a follow-up given
   RFC 0026's existing byline-honesty surface. Decide; if out of P0, say so.

## R4 — Stay inside the product boundary; ride RFC 0107, do not rebuild it

- The operator-id **is** `principal_id` (RFC 0107). Do **not** add a parallel
  identity table. Reuse `principals`, `principal_clients`,
  `sessions.last_session_heartbeat_at` (the reconcile sweep that already reaps
  stale sessions, migration 0033). `operator_handles` is a *rendering/lease*
  layer over `principal_id`, nothing more.
- No hosted service, directory, telemetry, or external identity. Single-human,
  single-daemon legibility only. Terminal titles / tmux / env may be *rendered
  to* (display-only, P2) but never *read from* for state.
- `run_id` stays opaque; the handle is never encoded into it.

## Phase boundary — P0 only

This run designs **P0**. P1 (custody log), P2 (honest bylines + handoff naming +
chips + OSC title), and P3 (lineage) are **out of scope** for this SPEC and are
sequenced behind P0 in the same campaign. The SPEC may note seams P0 must leave
for them (e.g. the custody log will append in the same txn as state transitions)
but must not design or build them here. Resolve OQ4's byline question only as a
*scope decision* (in/out of P0), not a P2 design.

## Falsifier guidance

The two falsifiers attack from disjoint lenses. The strongest challenges:

- **Falsifier 1 (identity-honesty / sufficiency / spoof lens — R1):** Can a
  spoof make run A render as operator B's handle via tty/env/title/a forged
  client, or by racing a lease flap (R1c)? Does the SPEC's R1b sufficiency
  resolution actually return **two distinct answers** for two same-human
  terminals, or does it collapse to one principal-derived handle (P0 fails its
  goal)? Is the collision-escalation rule deterministic and stable across
  reconnect, or does a reconnect silently relabel a live run? Is identity
  resolved from the live token server-side, or is there a path where a
  client-supplied name or display signal leaks into the authoritative answer?
- **Falsifier 2 (owner-migration / two-role DB-safety / carry-forward lens —
  R2/R4):** Does the owner bundle apply cleanly under the non-superuser owner
  DSN, or does a privilege gap surface `42501`? Is write-once actually enforced
  **at the DB** (not just app-level), and does the chosen REVOKE/trigger leave
  the runtime role exactly the privileges it still needs (INSERT run +
  lease/heartbeat/release on `operator_handles`)? Does the live-unique partial
  index behave under two sessions racing for `maya` (one wins, one escalates —
  not a deadlock, not a duplicate)? Does the SPEC rebuild RFC 0107 machinery it
  should reuse? Does it advance `RequiredOwnerBundleVersion` without
  justification, or break the forward-watermark/revoke-last ordering?

The adjudicator gates on whether a **material** challenge landed and was
**directly** rebutted, and on whether R1 (both faces), R2, R3 (all four OQs),
and R4 are each discharged. A clearing verdict (`accept` /
`accept_with_findings`) requires R1b's sufficiency proof present and the
owner-bundle two-role safety specified; `needs_revision` if either is missing or
hand-waved.
