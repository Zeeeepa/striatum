# HOLDER — RFC 0167 P0 falsifiable implementation SPEC, REVISION v2 (operator identity & run attribution)

author: holder-author-001

> This is the **v2 revision** of the RFC 0167 P0 implementation SPEC, published as the
> claim the two falsifiers re-attack. The v1 falsification gate
> (`rfc-0167-p0-design`) returned `needs_revision` with two binding, source-verified
> constraints — **C1** (the R1b sufficiency proof rested on a pre-run operator session
> current source cannot create) and **C2** (the run-origin principal stamp read the
> `principal_clients.principal_id` column that owner bundle 0006 revoked from
> `striatumd_rw`, failing `42501` under the two-role fixture). This revision **discharges
> both** and **carries forward, unregressed, everything v1 cleared**. It is built on the
> v1 SPEC (`docs/operator/artifacts/rfc-0167-p0-design/dialogue/holder/HOLDER.md`), not
> rewritten from scratch; the v1 cycle-1 ledger's `findings:`/`constraints:` blocks are
> the prescribed fixes. Every load-bearing claim is a **falsifiable assertion** paired
> with the **named test/check that would refute it**, anchored to **verified
> current-branch source** (`go/pkg/...` `file:line`). I re-read the source on the
> `striatum/rfc-0167-p0-design-v2` worktree; §0 records the corrections, and §C records
> the new verifications that discharge C1/C2. Scope is **P0 only**.

---

## §C — Addressing the v1 cycle-1 constraints (the auditable revision map)

This section exists so the falsifiers can verify the discharge directly. C1 and C2 are
each RESOLVED with a named, source-anchored mechanism and a named two-role pgtest; every
v1 carry-forward is INTACT.

### C1 — RESOLVED: the pre-run operator session is a dedicated owner-held `operator_sessions` table (not `striatumd.sessions`)

**The v1 gap (verified, re-confirmed on this branch).** v1's §2.5/A7 sufficiency proof
needs each terminal to hold a **distinct pre-run operator session** whose token sets
`app.session_id` at `run.prepare`. But current source cannot create one through the
session machinery v1 named:

- `striatumd.sessions.run_id` is `text NOT NULL` with `FOREIGN KEY (repository_id,
  run_id) REFERENCES striatumd.runs` **and** two `UNIQUE` constraints keyed on `run_id`
  (`(repository_id, run_id, slug)`, `(repository_id, run_id, role_id, lane_id,
  ordinal)`) — `0005_repo_local_workflow_state.sql:44-70` (VERIFIED on this branch).
- `HandleRegisterSession` aborts unless `run_id`/`role`/`lane` are all present
  (`lifecycle.go:41-46`, VERIFIED), then `lockRun` + `rowByID(..., "runs", run_id, true)`
  requires the run row to **already exist** and refuses terminal runs
  (`lifecycle.go:82-95`, VERIFIED). So `session.register` is structurally **run-bound**
  and `mintSessionBoundToken` cannot be reached before a run exists.

**The resolution (the constraint's "dedicated operator-session table" option, justified
over nullable `run_id`).** P0 adds a new **owner-held** `operator_sessions` table (owner
bundle 0021, §4) that is the per-terminal, **pre-run** liveness + token-binding anchor.
It deliberately does **not** touch `striatumd.sessions`, so the `sessions.run_id NOT
NULL` FK and its two run-keyed `UNIQUE` constraints are **untouched** — the
nullable-`run_id` alternative would have to relax that FK *and* both UNIQUE constraints
*and* audit every `WHERE run_id =`/`GROUP BY run_id` consumer of `sessions`, a standing
regression surface the falsifier explicitly flagged ("a nullable-run_id ALTER that breaks
an existing NOT-NULL assumption elsewhere"). A new table has **zero** such surface. This
is the decisive justification (§2.6 expands it).

**Why this is buildable where v1 was not — the load-bearing source fact.**
`PostgresAuthorizer.Authorize` resolves the token's bound session **purely from
`striatumd.client_capabilities.session_id`** (`auth_pg.go:104-156`, VERIFIED) and carries
it into `AuthContext.SessionID` (`auth_pg.go:145-156`); it **never** validates that the
bound session is a live row in `striatumd.sessions`. The authority prelude then sets
`app.session_id` from `AuthContext.SessionID` (`authority.go:79,120`). Therefore a
session-bound operator token minted with `session_id = <operator_session_id>` populates
`app.session_id` at `run.prepare` **without any `striatumd.sessions` row** — exactly the
object v1 could not create. The operator session lives in `operator_sessions`; the token
binding lives in `client_capabilities`; the run→handle join keys on `app.session_id`. No
part of this rebuilds session machinery — it **reuses** `mintSessionBoundToken`
(`session_token.go:60-97`), `link_client_to_principal` (`owner/0006:160-188`), the
`app.session_id` GUC (`authority.go:79,120`), and the `striatumd.leases` lazy-TTL idiom
(`0005:166-186`).

**Lifecycle, fully specified (§2.6):** create (operator-bootstrap mint+lease, one
transaction), heartbeat (guarded UPDATE, R1c idiom), graceful close (a dedicated
operator-session close path, **not** run-scoped `closeRemainingSessions` — which keys on
`run_id`, `mutations.go:1432`, VERIFIED), lazy expiry (TTL reclaim on the next
lease-acquisition walk, no background reaper), and the `run → operator_handles` join via
`app.session_id`.

**Gate (pgtest `operator_session_pre_run_stamp`, two-role, §4.5):** two pre-run operator
sessions for ONE human lease two distinct words; one run per session; assert two
**NON-NULL, DISTINCT** `created_by_handle_id` and `whose RA != whose RB`.

### C2 — RESOLVED: the principal stamp routes through the identity projection, resolved in Go and bound as a parameter

**The v1 gap (verified, re-confirmed on this branch).** v1's run-origin INSERT resolved
`created_by_principal_id` with a **direct** subquery `SELECT pc.principal_id FROM
striatumd.principal_clients pc WHERE pc.client_id = current_setting('striatum.principal_id',
true)`. But owner bundle 0006 **REVOKEs `SELECT ON striatumd.principal_clients FROM
striatumd_rw`** and grants back only `SELECT (client_id, linked_at, unlinked_at)` —
`principal_id` is omitted on purpose (`owner/0006_identity_read_scope.sql:218-221`,
VERIFIED). Under the two-role fixture that subquery fails **`42501`** before any trigger
or handle grant matters, and it contradicts v1's own C-2 reuse claim.

**The resolution.** The run-origin principal is resolved in **Go** via
`admin.ResolvePrincipalForClient(ctx, tx, auth.ClientID)` and **bound as a parameter** in
the runs INSERT — **never** a direct column read. That function calls the owner-owned
`SECURITY DEFINER` projection `striatumd.resolve_principal_for_client(p_daemon_secret,
p_client_id)` (`owner/0006:56-79`), which is `REVOKE ALL ... FROM PUBLIC` but
**`GRANT EXECUTE ... TO striatumd_rw`** (`owner/0006:181,186`, VERIFIED) — so
`striatumd_rw` *can* call it, and because it is `SECURITY DEFINER` owned by the role that
owns `principal_clients` (`owner/0006:197` `ALTER TABLE ... principal_clients OWNER TO
CURRENT_USER`), the function's own read of `principal_clients.principal_id` is **not**
subject to the runtime role's column revoke. The Go shim already prepends the daemon-auth
secret over the extended protocol and falls back to direct SQL only on `42883`
(function-absent) — `queryIdentityRow` (`admin/principals.go:53-63`, VERIFIED). **No
`SELECT(principal_id)` is granted back to `striatumd_rw`** — bundle 0006's read-scope
closure stays closed.

**INSERT-shape reconciliation (the constraint's explicit ask).** A SQL subquery cannot
inline-call the projection: it needs the daemon-auth secret as its first argument, which
travels from Go (`db.AuthorityFromContext(ctx).Secret`, `principals.go:54-57`), not from
inside an arbitrary INSERT. So the **principal** is resolved in Go and bound as `$N`; the
**handle** stays a runtime-readable subquery over `operator_handles` (granted in 0021, no
projection needed). §1 carries the corrected INSERT.

**Gate (pgtest `run_origin_stamp_uses_identity_projection`, two-role, §4.5):** (a) a
direct `pc.principal_id` read fails `42501` as a control; (b) the projection-routed stamp
succeeds and stores the right principal; (c) a forged envelope/request param cannot affect
the stored value.

### Carry-forwards — INTACT (the v1 adjudicator discharged these; not reopened, not weakened)

| Carry-forward | Status in v2 | Where |
|---|---|---|
| **R1a honesty (A1–A5)** | INTACT — stamp from the live-token prelude GUC / Go-resolved principal, never an envelope/tty/tmux/title/env; reads are pure PG joins | §1 |
| **R1b ARCHITECTURE** — per-session `created_by_handle_id` snapshot + live-unique partial index + deterministic principal-seeded escalation walk + run→handle_id join | INTACT (now proven buildable on the operator-session substrate) | §2 |
| **R1c flap renewal (A12)** — guarded UPDATE, never release-then-reacquire | INTACT (now also governs operator-session heartbeat) | §3 |
| **R2 DB-write-once** — `BEFORE UPDATE` trigger `refuse_run_origin_change()`; owner bundle **0021** (`LatestOwnerBundleVersion==20`, VERIFIED); forward-only / watermark interlock | INTACT (+ operator_sessions grants, still no REVOKE) | §4 |
| **R3 four open questions** (OQ1 pool/default/escalation/denylist; OQ2 NULL+advisory, no backfill; OQ3 per-repo, P3 deferred; OQ4 byline P2) | INTACT | §5 |
| **R4 reuse** — FK rendering layer over `principal_id`, no parallel identity table, opaque `run_id` | INTACT (operator_sessions is a liveness anchor, not an identity store; C2's projection *is* the reuse) | §6 |
| **Source corrections C-1..C-4** | INTACT; C-2's fix is now the projection route (§0) | §0 |

**No carry-forward regressed.** The only deltas are: (1) the operator-session substrate is
now a named, buildable table + lifecycle (was an unspecified "seam"); (2) the run-origin
INSERT resolves the principal through the projection in Go (was a direct, `42501`-failing
subquery). Bundle ordinal stays 21; no REVOKE is introduced; the watermark interlock is
unchanged.

---

## How this SPEC discharges R1a / R1b / R1c / R2 / R3 / R4 (auditable coverage map)

| Req | What it demands | Where discharged | Load-bearing assertion(s) |
|-----|-----------------|------------------|---------------------------|
| **R1a** | Identity bound server-side, at token-mint, against the live token; never tty/tmux/title/env/client name; reads resolve through `principal_id`, only snapshot the handle | §1 | A1–A5 |
| **R1b** | THE CRUX — one human = one `principal_id` across ~15 terminals; specify how P0 answers "which window", the deterministic escalation rule, the exact run→handle join, and **prove two same-human terminals return two distinct answers** on a **buildable** substrate | §2 | A6–A11, **A27** |
| **R1c** | Heartbeat renews via guarded UPDATE, never release-then-reacquire | §3 | A12 |
| **R2** | Owner-bundle migration at the next free ordinal; DB write-once + justify; pin retained privileges; prove clean apply + write-once + **two-role stamp safety** under the RFC 0142 fixture; forward-only, watermark-consistent | §4 | A13–A19, **A28** |
| **R3** | Resolve all four open questions concretely | §5 | A20–A23 |
| **R4** | Ride RFC 0107 (operator-id IS `principal_id`); no parallel identity table; reuse principals/principal_clients/session liveness/**the identity projection**; product-boundary clean | §6 | A24–A26, **A28** |

The full assertion ledger is §8 (A1–A28). The P0 boundary and P1–P3 seams are §7. The
build manifest is §9.

---

## §0 — Verified source baseline and the four corrections to the RFC's anchors

The holder verifies, does not trust. I re-read the source on this branch. **Verified true
(load-bearing, unchanged from v1):**

- `LatestOwnerBundleVersion == 20`, `RequiredOwnerBundleVersion == LatestOwnerBundleVersion`
  — `owner.go:23,35` (re-verified this branch). **Next free owner-bundle ordinal is 21.**
- A table is **owner-held by default** and runtime-`ALTER`-able only after an owner bundle
  transfers ownership to `striatumd_rw` (`owner_runtime_ownership.go:8-11`; transferred set
  derived from owner-bundle SQL by `RuntimeOwnedTablesAlterable()`). `runs` and `sessions`
  are created by runtime migration 0005 but applied **by the owner role** on a two-role
  deploy and are **not** in bundle 0018's transfer cohort, so both are owner-held (C-1).
- The session capability token is minted **inside** the registration transaction —
  `mintSessionBoundToken` "Runs inside the registration transaction so the token is
  committed atomically with the session row" (`session_token.go:48-53,60-97`). It writes
  `clients` + `client_capabilities` only; it does **not** itself write `sessions` or
  `principal_clients` (so the operator-bootstrap mint must also call
  `link_client_to_principal`, §1).
- The caller's identity is resolvable server-side from the live token in any authorized
  mutation: the RFC 0110 prelude installs `striatum.principal_id`/`app.session_id` as
  transaction-local GUCs (`authority.go:75-85,116-120,135-158`), sourced from
  `rpc.AuthFromContext(ctx)` whose `ClientID`/`SessionID` the `PostgresAuthorizer`
  resolves by validating the bearer token against `striatumd.clients` and reading the
  grant's `session_id` from `client_capabilities` (`auth_pg.go:49-157`).
- The two-role pgtest fixture exists: `pgtest.TwoRole(t)` with `OwnerPool`/`SUTPool`
  (`pgtest/two_role.go:47-78`); the `42501` oracle is `assertSQLState42501`
  (`db/two_role_pg_test.go:161-176`).
- **New this revision:** `auth_pg.go:104-156` reads the bound session from
  `client_capabilities.session_id` and never joins `sessions` — the fact that makes the
  pre-run operator session buildable (§C/C1). And `resolve_principal_for_client` is
  `SECURITY DEFINER`, `GRANT EXECUTE ... TO striatumd_rw` (`owner/0006:56-79,181,186`) —
  the fact that makes the projection route runtime-safe (§C/C2).

**CORRECTION C-1 — `ALTER runs ADD COLUMN` is an OWNER-bundle change because of
*ownership*, not FKs.** `runs` is created by runtime migration 0005
(`0005:13-36`) but on a two-role deploy is owned by the owner role and is **not** in
bundle 0018's ownership-transfer cohort; the runtime role has table-level DML on `runs`
(`0005:467-475`) but not ownership, so a runtime-migration `ALTER runs` dies `42501 must
be owner of table runs` (the #441/#442 / D248 trap, `0018:8-22`). **`ALTER runs ADD
COLUMN` goes in owner bundle 0021.** The same ownership logic puts the new
`operator_handles`/`operator_sessions` tables in the owner bundle.

**CORRECTION C-2 — the authority GUC `striatum.principal_id` holds the `client_id`, not
the `principal_id`; and the dereference must go through the projection, not a direct
column read.** The prelude sets `set_config('striatum.principal_id', $3, true)` where
`$3 = auth.ClientID` (`authority.go:78,116-120`). v1 dereferenced this via a direct
`principal_clients` subquery — which fails `42501` under the two-role fixture (bundle 0006
revoke, §C/C2). **The corrected dereference resolves the principal in Go via
`admin.ResolvePrincipalForClient` (the `resolve_principal_for_client` projection) and
binds it as a parameter** (§1). This is the single substantive change C2 forced, and it
makes the SPEC *consistent* with v1's own stated reuse intent.

**CORRECTION C-3 — there is no periodic session reaper; release is graceful-close + lazy
expiry.** The liveness column is `sessions.last_heartbeat_at` (`0005:44-70`), not
`last_session_heartbeat_at`; migration 0033 reaps terminal-run **supervisors**, not
sessions; graceful session teardown is `closeRemainingSessions` (`mutations.go:1432`),
which keys on `run_id`. There is **no** background session reaper. P0 therefore releases
handle leases **(a) gracefully** in a close transaction **plus (b) lazily** via a
`leased_until` TTL (the `striatumd.leases` idiom, `0005:166-186`). For the **operator
session** specifically (no `run_id`), release uses a **dedicated** operator-session close
path, never run-scoped `closeRemainingSessions` (§2.6).

**CORRECTION C-4 — `created_by_principal_id` alone is insufficient, and OQ2's backfill
source carries no identity.** `branch_confirmed_by` holds `'daemon'`/`'human'` literals
(`run.go:1053,887-891`), not a `principal_id`; a "backfill from `branch_confirmed_by`"
would fabricate identity from a non-identity field (the dishonest stamp R1a forbids),
informing the OQ2 no-backfill decision (§5.2). And since one human = one `principal_id`,
the disambiguator must be the per-session **`runs.created_by_handle_id`** snapshot (§2).

---

## §1 — R1a: identity is bound server-side, at token-mint, against the live token

### Design

P0 introduces two write paths and binds identity server-side at both, with **no envelope,
tty, tmux, title, or env value** reaching either.

**(1) The handle lease is acquired inside a token-mint transaction — at operator
bootstrap.** The load-bearing lease is the **operator session's**, acquired pre-run (§2.6),
because the operator session is what creates runs. `mintSessionBoundToken` already runs in
a single transaction (`session_token.go:48-53`). The operator-bootstrap mint RPC, in one
transaction, (a) resolves/creates the caller's `principal` (`kind='human'`, via the
owner-owned create + `link_client_to_principal` projection — `owner/0006:160-188`), (b)
mints a session-bound operator token via `mintSessionBoundToken` keyed on a fresh
`operator_session_id`, (c) INSERTs the `operator_sessions` row, and (d) acquires the
handle lease into `operator_handles` (§2.2) keyed on `principal_id` and
`leased_session_id = operator_session_id`. Because mint + link + lease share one
transaction, no token exists without its principal link and handle lease, and no client
RPC can interpose a name between them (R1a / A3).

> The `principal_id` for the lease is the one resolved/created **in this transaction** —
> held directly in Go, so the lease INSERT binds it as a parameter (no `principal_clients`
> read at all on this path). The projection route (C2) governs the *run-origin* stamp,
> where only a `client_id` is in hand.

**(2) `runs.created_by_principal_id` is resolved from the live token at run creation,
server-side, through the identity projection (C2 fix).** The runs INSERT (`run.go:1056-1074`)
is extended. The principal is resolved in Go and bound; the handle is a runtime-readable
subquery:

```go
// In HandleRunPrepare's authorized transaction, before the runs INSERT:
auth := rpc.AuthFromContext(ctx)                          // ClientID from the validated live token
ref, ok, err := admin.ResolvePrincipalForClient(ctx, tx, auth.ClientID)  // -> resolve_principal_for_client projection
// principalID := nil if !ok (no active link: bootstrap admin / pre-RFC-0107) -> honest unknown
```

```sql
INSERT INTO striatumd.runs (
  repository_id, run_id, workflow_snapshot_id, repo_root, state,
  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at,
  created_by_principal_id, created_by_handle_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
  $11,                                              -- bound: ref.PrincipalID resolved in Go via the projection (NEVER a direct pc.principal_id read)
  (SELECT oh.handle_id
     FROM striatumd.operator_handles oh
    WHERE oh.leased_session_id = current_setting('app.session_id', true)  -- = operator_session_id (C1)
      AND oh.released_at IS NULL)
);
```

`HandleRunPrepare` reads only `repository_id` and `workflow` from the envelope
(`run.go:21-28`); there is no `created_by` parameter path, so a forged
`created_by_principal_id` in the request is structurally ignored. The principal comes from
`auth.ClientID` (the validated live token, `auth_pg.go:49-157`) through the
`SECURITY DEFINER` projection; the handle comes from the prelude `app.session_id` GUC. The
old v1 direct `principal_clients` subquery is **deleted** (it failed `42501`).

**(3) Every read surface resolves through `principal_id` and only snapshots the handle.**
`whose`, `status --mine`, `doctor`, and evidence export render from a pure PG join over
`created_by_principal_id` / `created_by_handle_id` / `operator_handles` / `principals`
(§2.4). No tty, pane, tmux, title, or env value enters any authoritative answer.

### Falsifiable assertions

- **A1 (server-side stamp).** `runs.created_by_principal_id` equals the principal of the
  live token presented on the run-creation RPC, dereferenced server-side through the
  projection. *Refuting test:* `run_origin_stamp_uses_identity_projection` (§4.5, C2) —
  create a run on `P_A`'s token while passing envelope param `created_by_principal_id =
  P_B` and tty/tmux/title/env spoofs; assert the stored value is `P_A`. `P_B` or a spoof
  leak refutes A1.
- **A2 (no client-name path).** No client-supplied string becomes
  `created_by_principal_id` or a rendered handle. *Refuting test:* a static guard greps
  the `run.prepare`/`operator bootstrap` handlers for any `stringParam(envelope,
  "created_by*"|"handle"|"operator*")` feeding the stamp/lease; presence refutes A2.
  (`operator_label` stays display-only, `lifecycle.go:71,114-121`.)
- **A3 (mint+link+lease atomicity).** The operator-session handle lease and the
  `principal_clients` link are committed in the same transaction as the operator token.
  *Refuting test:* inject a fault between `mintSessionBoundToken` and the lease INSERT and
  assert the whole operator-bootstrap mint rolls back (no token row without its link + lease
  rows). A token committed without a lease refutes A3.
- **A4 (live-token resolution).** Identity derives from `auth.ClientID` set by
  `PostgresAuthorizer.Authorize` from the validated token. *Refuting test:* present a
  revoked/expired token to `run.prepare`; assert it is rejected at `auth_pg.go:87-92` and
  no run row is created.
- **A5 (read surfaces cannot lie).** `whose`'s authoritative answer is a function of
  `{created_by_principal_id, created_by_handle_id}` joined to
  `operator_handles`/`principals` only. *Refuting test:* a unit test asserts the `whose`
  SQL references no tty/pane/title/env; a column outside that join refutes A5.

---

## §2 — R1b (THE CRUX): per-human `principal_id` vs per-terminal session granularity, on a buildable substrate

The single most important resolution. v1's *architecture* was discharged; v1's *proof*
was not buildable. This revision keeps the architecture verbatim and makes the substrate
buildable (§2.6).

### 2.1 Why `created_by_principal_id` alone fails

Under RFC 0107 one human is one `principal_kind='human'` `principal_id`
(`0023_principals.sql:30-36`). The fifteen terminals share one `created_by_principal_id`,
and the suffix `#7f3` (computed from `principal_id`) is identical for all. Neither the
column nor the suffix can answer "which window owns run X." The disambiguator must be
**per session**, not per principal.

### 2.2 The mechanism: per-session leased handle + live-unique partial index

`operator_handles` (owner bundle 0021, §4) leases a **word per operator session**:

```sql
CREATE TABLE striatumd.operator_handles (
  handle_id         text PRIMARY KEY,
  repository_id     text NOT NULL REFERENCES striatumd.repositories(repository_id),
  principal_id      text NOT NULL REFERENCES striatumd.principals(principal_id),
  handle            text NOT NULL,              -- lowercase, privacy-safe (curated pool, §5.1)
  leased_session_id text NOT NULL,              -- the operator_session_id (§2.6)
  leased_at         timestamptz NOT NULL,
  leased_until      timestamptz NOT NULL,       -- lazy-expiry TTL (mirrors striatumd.leases, 0005:166-186)
  last_heartbeat_at timestamptz,
  released_at       timestamptz,
  release_reason    text
);
CREATE UNIQUE INDEX operator_handles_live_uq
  ON striatumd.operator_handles (repository_id, lower(handle))
  WHERE released_at IS NULL;                     -- the disambiguator engine
CREATE UNIQUE INDEX operator_handles_live_session_uq
  ON striatumd.operator_handles (repository_id, leased_session_id)
  WHERE released_at IS NULL;                     -- one live lease per operator session
```

The partial-unique pattern is proven in-repo: `striatumd.leases` uses `CREATE UNIQUE INDEX
uq_active_resource_lease ... WHERE state = 'active'` (`0005:184-186`) and renews by
extending a TTL (`0005:166-178`). `operator_handles` is the same shape, scoped to
`released_at IS NULL`. Two concurrent same-human operator sessions cannot both hold `maya`:
the second `INSERT` of `lower(handle)='maya'` raises `23505` and escalates (§2.3).

### 2.3 The lease algorithm and the deterministic collision-escalation rule

A deterministic, principal-seeded candidate sequence drives both default and escalation:

```
seed       = fnv64a(principal_id)                       -- stable per human, per repo
candidates = [ POOL[(seed + k) mod len(POOL)] | k = 0,1,2,... ]
```

Lease acquisition (inside the operator-bootstrap mint transaction, §1/§2.6):

```
for k in 0,1,2,...:
    w := candidates[k]
    -- lazy expiry (C-3): reclaim an abandoned word whose TTL lapsed, no background sweep.
    UPDATE operator_handles SET released_at = now(), release_reason = 'lease_expired_lazy'
      WHERE repository_id = $r AND lower(handle) = w AND released_at IS NULL AND leased_until < now();
    try:
        INSERT operator_handles(handle_id, repository_id, principal_id, handle=w,
                                leased_session_id, leased_at=now(), leased_until=now()+TTL);
        return w
    catch unique_violation (23505 on operator_handles_live_uq):
        continue
```

- **Deterministic default.** A lone session lands on `candidates[0]`. A reconnect (new
  operator_session_id) re-runs the walk, finds `candidates[0]` free, re-leases it. *(R3-OQ1)*
- **Deterministic escalation.** A second concurrent same-human session finds
  `candidates[0]` live-held → `23505` → leases `candidates[1]` (a distinct curated word,
  e.g. `theo`, not numeric `maya2`). Stable across reconnect while the peer holds
  `candidates[0]`.
- **The only relabel is convergent and harmless.** If the first session dies,
  `candidates[0]` frees; the escalated session converges to it on next reconnect. This
  changes only the **live** word, never a run's attribution — runs carry a **frozen**
  `created_by_handle_id` snapshot (§2.4) protected write-once (§4.3).
- **No deadlock, one winner.** Two sessions racing for `candidates[0]` each attempt one
  INSERT; the partial-unique index serializes them — one commits, the loser catches
  `23505` and advances. No row locked across contention; no lock-ordering cycle.

### 2.4 The exact run → handle join (the decision R1b demands)

**Decision: the run carries a write-once snapshot — `runs.created_by_handle_id` (FK →
`operator_handles.handle_id`)** — *in addition to* `created_by_principal_id`. `whose`
joins through it; it does **not** join `run → created_by_principal_id → live lease` (one
principal has up to fifteen live leases — ambiguous).

Rationale (unchanged from v1):
- vs `run → session → current lease`: a session could re-lease a different word later;
  the `handle_id` FK pins the exact lease row live at creation.
- vs a denormalized `created_by_handle text`: a `handle_id` FK is verifiable against
  handle history (RFC D5) and never drifts. `operator_handles` rows are retained (no
  DELETE, §4.4), so the FK never dangles and the snapshot word is permanently stable —
  **including across lazy expiry**, because expiry only sets `released_at` on the row, it
  does not delete it; a later re-lease of the same word creates a *new* `handle_id` row,
  so the old run still resolves its old `handle_id` to the old word (this directly answers
  the falsifier's "lazy expiry that frees a word a live run still references" — it does
  not; the snapshot is by immutable PK, not by word).

`whose <run-id>` is the pure join that cannot lie:

```sql
SELECT r.run_id, r.state, r.created_by_principal_id,
       oh.handle AS origin_handle, p.principal_kind, p.disabled_at
  FROM striatumd.runs r
  LEFT JOIN striatumd.operator_handles oh ON oh.handle_id = r.created_by_handle_id
  LEFT JOIN striatumd.principals       p  ON p.principal_id = r.created_by_principal_id
 WHERE r.repository_id = $1 AND r.run_id = $2;
```

Render rule:
- `created_by_principal_id IS NULL` → bare `run_id` + advisory `attribution_unknown`
  (§5.2).
- else `word = COALESCE(oh.handle, defaultHandle(created_by_principal_id))`, `suffix =
  hexPrefix(fnv(created_by_principal_id))` (computed, per RFC D1), render `word#suffix` +
  state/phase + a paste-able switch hint.

### 2.5 PROOF — two same-human terminals return two distinct answers (now on a buildable substrate)

Human `H` → principal `P`; `defaultHandle(P) = candidates[0] = "maya"`.

1. **Operator session `S1`** created at bootstrap → one txn: resolve `P`, mint operator
   token (binding `session_id = S1`), `link_client_to_principal(client_S1, P)`, INSERT
   `operator_sessions(S1)`, lease walk `INSERT maya` → succeeds → `S1` holds `maya`
   (`handle_id = h1`).
2. **Operator session `S2`** (same human, second terminal) → one txn: resolve `P`, mint
   token (binding `session_id = S2`), link `client_S2 → P`, INSERT `operator_sessions(S2)`,
   lease walk `INSERT maya` → `23505` → `INSERT theo` → `S2` holds `theo` (`handle_id =
   h2`).
3. `S1` runs `run.prepare` (presents the `S1` token → `app.session_id = S1`,
   `auth.ClientID = client_S1`) → creates run `RA` → stamp `created_by_principal_id =
   ResolvePrincipalForClient(client_S1) = P`, `created_by_handle_id = (SELECT handle_id
   WHERE leased_session_id = S1 AND released_at IS NULL) = h1`.
4. `S2` runs `run.prepare` (token `S2` → `app.session_id = S2`) → creates run `RB` → stamp
   `created_by_principal_id = P`, `created_by_handle_id = h2`.
5. `whose RA` → `oh.handle='maya'`, `suffix=#7f3` → **`maya#7f3`**.
6. `whose RB` → `oh.handle='theo'`, `suffix=#7f3` → **`theo#7f3`**.

**Two distinct answers** (`maya#7f3` ≠ `theo#7f3`); the identical suffix correctly signals
"same human." Critically, every object in steps 1–2 is now **buildable on current source**
(the operator session lives in `operator_sessions`, not the run-bound `sessions`; the
token binding flows through `client_capabilities.session_id` → `app.session_id` with no
`sessions` row required — §0, §C/C1). This is the `operator_session_pre_run_stamp` pgtest
(§4.5).

### 2.6 The pre-run operator-session substrate + lifecycle (the C1 discharge)

**Storage — `operator_sessions` (owner bundle 0021).** A new owner-held table; the
per-terminal, pre-run liveness anchor. **No `run_id`, no FK to `runs`** — that is the
whole point; the `sessions.run_id NOT NULL` FK is structurally inapplicable because the
operator session does not live in `striatumd.sessions`.

```sql
CREATE TABLE striatumd.operator_sessions (
  repository_id     text NOT NULL REFERENCES striatumd.repositories(repository_id),
  operator_session_id text NOT NULL,
  principal_id      text NOT NULL REFERENCES striatumd.principals(principal_id),
  client_id         text NOT NULL,              -- the session-bound token's client (clients.client_id)
  state             text NOT NULL CHECK (state IN ('active','closed','expired')),
  registered_at     timestamptz NOT NULL,
  last_heartbeat_at timestamptz,
  expires_at        timestamptz NOT NULL,       -- lazy-expiry TTL (striatumd.leases idiom)
  closed_at         timestamptz,
  close_reason      text,
  PRIMARY KEY (repository_id, operator_session_id)
);
CREATE INDEX operator_sessions_principal_live
  ON striatumd.operator_sessions (repository_id, principal_id)
  WHERE state = 'active';
```

**Why a dedicated table, not nullable `sessions.run_id`.** The constraint offered both;
the dedicated table is chosen because the nullable-`run_id` alternative would have to (a)
drop the `sessions.run_id NOT NULL`, (b) relax the `FK (repository_id, run_id) → runs`,
(c) relax **both** `UNIQUE (repository_id, run_id, slug)` and `UNIQUE (repository_id,
run_id, role_id, lane_id, ordinal)` (which assume `run_id` present, `0005:68-69`), and (d)
audit every `WHERE run_id =` / `GROUP BY run_id` / `idx_sessions_run_state`
(`0005:72-73`) consumer of `sessions` for a silent NULL-handling regression — the exact
"breaks an existing NOT-NULL assumption elsewhere" risk the falsifier named. A new table
has **none** of that surface. The cost is one small owner-held table; the benefit is zero
regression to the run-bound session machinery. (And it is a strict **reuse**, not a
rebuild — see below.)

**Reuse, not rebuild (R4).** The operator session is *materialized* from existing
machinery: the token is `mintSessionBoundToken` (`session_token.go:60-97`, unchanged); the
principal link is `link_client_to_principal` (`owner/0006:160-188`, unchanged); the
`app.session_id` GUC is the prelude's (`authority.go:79,120`, unchanged); the lease TTL is
the `striatumd.leases` idiom (`0005:166-186`). `operator_sessions` stores **no identity**
(identity is `principal_id` via `principal_clients`), only liveness + the token binding.
It is not a parallel `sessions` table — lane `sessions` rows still serve run-scoped lane
work; `operator_sessions` is the distinct, pre-run, per-terminal object that lane sessions
cannot be.

**Lifecycle, fully specified:**

- **Create** — the operator-bootstrap mint RPC, one transaction (§1(1)): resolve/create
  `P`, mint operator token bound to a fresh `operator_session_id`, link `client → P`,
  INSERT `operator_sessions(state='active', expires_at = now()+TTL)`, acquire the handle
  lease (§2.3). Atomic (A3).
- **Heartbeat** — `operator.heartbeat` (or the bootstrap manifest refresh) runs a guarded
  UPDATE (R1c idiom, §3): `UPDATE operator_sessions SET last_heartbeat_at=now(),
  expires_at=now()+TTL WHERE operator_session_id=$ AND state='active'` **and** the
  `operator_handles` renewal. Never release-then-reacquire.
- **Graceful close** — operator session end / `operator bootstrap --close`: one
  transaction sets `operator_sessions.closed_at=now(), state='closed'` **and**
  `operator_handles.released_at=now(), release_reason='operator_session_closed'`. This is a
  **dedicated** path; it does **not** call `closeRemainingSessions` (which keys on
  `run_id`, `mutations.go:1432`, and would never match an operator session).
- **Lazy expiry** — no background reaper (C-3). On the next lease-acquisition walk, an
  operator session whose `expires_at < now()` has its handle reclaimed (the §2.3
  `lease_expired_lazy` UPDATE); reads treat an `expires_at < now()` operator session as
  inactive. Mirrors the project's "lease expiry is lazy" policy and `striatumd.leases`.
- **run → operator_handles join** — `run.prepare` reads `app.session_id` (=
  `operator_session_id`, from the token binding) → the `created_by_handle_id` subquery
  (§1(2)). No `closeRemainingSessions` dependency anywhere on this path.

### Falsifiable assertions

- **A6 (live-unique forces distinct words).** Two concurrent same-human operator sessions
  hold two distinct live words. *Refuting test:* `operator_session_pre_run_stamp` (§4.5)
  — exactly one live `lower(handle)='maya'`, the second on a distinct word; a duplicate or
  deadlock refutes A6.
- **A7 (distinct `whose` answers).** `whose RA != whose RB`. *Refuting test:* the §2.5
  scenario asserts `maya#7f3` vs `theo#7f3`; equal answers refute A7 (gate-critical).
- **A8 (deterministic default, reconnect-stable).** A lone session for `P` always leases
  `candidates[0]`. *Refuting test:* lease → close → re-bootstrap for `P`; a different word
  refutes A8.
- **A9 (deterministic escalation, reconnect-stable).** While `candidates[0]` is peer-held,
  a session deterministically (re)leases `candidates[1]`. *Refuting test:* hold
  `candidates[0]`; bootstrap + reconnect a second session; a non-`candidates[1]` word
  refutes A9.
- **A10 (no silent relabel).** A reconnect never rewrites a run's `created_by_handle_id`.
  *Refuting test:* create `RB` under `theo`; close `S1` (frees `maya`); reconnect `S2`
  (converges live to `maya`); assert `whose RB` still renders `theo#7f3`. A change refutes
  A10.
- **A11 (one winner, no deadlock).** Concurrent lease of `candidates[0]` yields exactly one
  holder and a clean escalation. *Refuting test:* concurrent two-session lease in the
  two-role fixture; `40P01` / duplicate / both-fail refutes A11.
- **A27 (operator session is buildable pre-run).** An operator session + its handle lease
  can be created with **no** `striatumd.sessions` row and **no** run, and its token sets
  `app.session_id` at `run.prepare`. *Refuting test:* `operator_session_pre_run_stamp`
  asserts both runs get NON-NULL DISTINCT `created_by_handle_id`; a NULL `created_by_handle_id`
  (because `app.session_id` resolved empty, or because `operator_sessions`/the lease could
  not be created without a run) refutes A27 and re-opens C1.

---

## §3 — R1c: lease flap (heartbeat renews, never release-then-reacquire)

### Design

Heartbeat is a **guarded UPDATE** of the existing row; it never deletes, never sets
`released_at`, never re-INSERTs. For the handle lease:

```sql
UPDATE striatumd.operator_handles
   SET leased_until = now() + $TTL, last_heartbeat_at = now()
 WHERE handle_id = $1 AND leased_session_id = $2 AND released_at IS NULL;
```

and, in the same operator-session heartbeat transaction, the `operator_sessions` renewal
(§2.6). The guard `leased_session_id = $2 AND released_at IS NULL` means only the owning,
still-live session renews, and the row **never transits through a released state** during
renewal — so `operator_handles_live_uq` never frees the word mid-flap and a racing
same-human session cannot steal it. Mirrors the `striatumd.leases` renewal idiom
(`0005:166-178`).

### Falsifiable assertion

- **A12 (flap-resistance).** A heartbeat renewal cannot let another session steal the word.
  *Refuting test:* `lease_flap_steal` (two-role) — `S1` holds `maya`; interleave `S1`'s
  renewal UPDATE with `S2`'s attempt to lease `maya`; assert `S2` always gets `23505` and
  escalates, and `S1`'s row was never `released_at`-set. A steal during the flap refutes
  A12.

---

## §4 — R2: the owner-bundle migration (the gating, hardest-to-reverse change)

### 4.1 Ordinal and placement

Owner bundle **0021** (next free after `LatestOwnerBundleVersion == 20`, `owner.go:23`).
New file `go/pkg/db/sql/owner/0021_operator_identity_run_attribution.sql`, auto-discovered
by the embedded loader (`owner.go:157` `//go:embed sql/owner/*.sql`), plus a label entry
`21: "RFC 0167 P0 operator identity + run attribution"` and `LatestOwnerBundleVersion =
21`. It is an **owner** bundle because `operator_handles`, `operator_sessions` (new
owner-held tables), and `ALTER runs` all touch tables the runtime role does not own (C-1).

### 4.2 Bundle SQL (additive — no privilege-stripping REVOKE)

```sql
-- owner bundle 0021 — applied OUT-OF-BAND as the owner via `striatum daemon owner-ddl apply`, THEN restart.

-- (1) operator_handles: owner-held lease/rendering layer over principal_id (R4). §2.2.
CREATE TABLE IF NOT EXISTS striatumd.operator_handles ( ... );          -- §2.2
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_uq         ...;  -- §2.2
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_session_uq ...;  -- §2.2

-- (2) operator_sessions: owner-held pre-run per-terminal liveness anchor (C1). §2.6.
CREATE TABLE IF NOT EXISTS striatumd.operator_sessions ( ... );         -- §2.6
CREATE INDEX IF NOT EXISTS operator_sessions_principal_live      ...;   -- §2.6

-- (3) runs origin stamp (owner-held table -> owner bundle, C-1).
ALTER TABLE striatumd.runs
  ADD COLUMN IF NOT EXISTS created_by_principal_id text REFERENCES striatumd.principals(principal_id);
ALTER TABLE striatumd.runs
  ADD COLUMN IF NOT EXISTS created_by_handle_id text REFERENCES striatumd.operator_handles(handle_id);

-- (4) write-once enforced at the DB (BEFORE UPDATE trigger, §4.3).
CREATE OR REPLACE FUNCTION striatumd.refuse_run_origin_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.created_by_principal_id IS DISTINCT FROM OLD.created_by_principal_id
     OR NEW.created_by_handle_id  IS DISTINCT FROM OLD.created_by_handle_id THEN
    RAISE EXCEPTION 'runs.created_by_* origin stamp is write-once (set at run creation, immutable thereafter)';
  END IF;
  RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS runs_origin_write_once ON striatumd.runs;
CREATE TRIGGER runs_origin_write_once
  BEFORE UPDATE ON striatumd.runs
  FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_run_origin_change();

-- (5) runtime-role DML on the new owner-held tables (the 0005 ALL-TABLES grant predates them).
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.operator_handles  TO striatumd_rw;  -- no DELETE: rows retained
    GRANT SELECT, INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw;  -- no DELETE: append/retain
  END IF;
END $$;

-- (6) watermark/capability stamp (mirrors every bundle, e.g. 0018:108-110).
INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('operator_identity_run_attribution', false, 21)
ON CONFLICT (capability) DO NOTHING;
```

No `EXECUTE` grant on `resolve_principal_for_client` is added here — it already exists from
bundle 0006 (`owner/0006:186`), and C2 reuses it. **No `SELECT(principal_id)` grant-back is
issued** (that would reopen the 0006 read-scope closure — explicitly forbidden by C2).

Precedents: `ALTER ... ADD COLUMN` on an owner table is owner-bundle 0009's pattern; the
ownership/grant idiom and the `EXISTS striatumd_rw` guard mirror owner-bundle 0018
(`0018:77-110`).

### 4.3 Write-once enforcement: CHOSEN = BEFORE UPDATE trigger (not column REVOKE)

Unchanged from v1 (DISCHARGED). **Chosen: a `BEFORE UPDATE` trigger** that raises when
either origin column changes. Justification:
1. `runs` is actively UPDATEd on ~six paths (`state`/`started_at`/`paused_*`/`completed_at`/
   `stop_reason`/`branch_*`, `run.go:116-119,450-453,487-490,657-660,781-784,887-891`),
   including `branch_confirmed_by` *after* creation — a blanket `REVOKE UPDATE ON runs` is
   impossible, and the column-grant alternative must enumerate ~15 columns and be
   re-maintained on every future `runs ADD COLUMN` (a standing footgun).
2. Direct in-repo precedent: `0010_artifact_blob_update_trigger.sql:19-49` already enforces
   column-selective immutability via a `BEFORE UPDATE` trigger.
3. Stronger invariant: the trigger refuses the change for any role and any path; the
   non-superuser runtime role cannot `SET session_replication_role = replica` to bypass it.
4. Keeps 0021 purely forward-additive: no `REVOKE`, so 0021 is not subject to the
   revoke-last watermark-ordering rule.

The stamp happens at **INSERT** (§1), which does not fire the `BEFORE UPDATE` trigger;
post-insert the columns are immutable. `IS DISTINCT FROM` also forbids a later `NULL →
value` UPDATE, consistent with OQ2 "no backfill" (§5.2).

### 4.4 Privileges the runtime role must RETAIN (pinned)

- **`runs`** — `SELECT, INSERT, UPDATE, DELETE` unchanged (`0005:467-475`). No REVOKE
  issued; the trigger blocks only the two origin columns.
- **`operator_handles`** — `SELECT` (render/join), `INSERT` (lease), `UPDATE` (heartbeat +
  graceful release + lazy reclaim). **No `DELETE`** — rows retained so
  `created_by_handle_id` snapshots never dangle.
- **`operator_sessions`** — `SELECT` (manifest/liveness), `INSERT` (create), `UPDATE`
  (heartbeat/close). **No `DELETE`** — append/retain.
- **`resolve_principal_for_client`** — `EXECUTE` already granted to `striatumd_rw`
  (`owner/0006:186`); C2's run-origin resolution rides it. **`SELECT(principal_id)` on
  `principal_clients` stays REVOKED** — not granted back.

### 4.5 Proof under the RFC 0142 two-role fixture; named pgtests

All DB-boundary tests use `pgtest.TwoRole(t)` (`two_role.go:47-78`): bundle DDL via
`OwnerPool` (non-superuser owner), runtime behavior via `SUTPool` (`striatumd_rw`), with
daemon authority bootstrapped for the SUT so the projection path is exercised (not a
literal insert). `assertSQLState42501` (`two_role_pg_test.go:161-176`) is the privilege
oracle.

1. **`owner_bundle_0021_applies_clean`** — apply 0021 via `OwnerPool`; assert
   `operator_handles` + `operator_sessions` + their indexes exist, `runs` has both columns,
   the trigger exists; then as `SUTPool` assert the runtime role **can** INSERT a run
   carrying `created_by_principal_id`, INSERT/UPDATE `operator_handles` and
   `operator_sessions`, **can** `EXECUTE resolve_principal_for_client` (with daemon auth),
   and **cannot** `ALTER TABLE striatumd.runs ...` (`assertSQLState42501`) nor `DELETE` from
   the two new tables — proving the runtime retains exactly its needed privileges and no
   more.
2. **`run_origin_stamp_uses_identity_projection`** (C2 gate) — as `SUTPool`: **(a)** a
   direct `SELECT pc.principal_id FROM striatumd.principal_clients pc WHERE pc.client_id=$1`
   fails `42501` (`assertSQLState42501`) as a control; **(b)** the real stamp path
   (`admin.ResolvePrincipalForClient` → projection → bound `$N`) succeeds and the stored
   `created_by_principal_id` equals the linked principal `P`; **(c)** a run created while
   passing envelope param `created_by_principal_id = P_other` (+ tty/env spoofs) still
   stores `P`. Any of: (a) succeeding, (b) `42501`/wrong principal, (c) the forged value
   landing — refutes the C2 discharge.
3. **`operator_session_pre_run_stamp`** (C1 gate) — as `SUTPool`: create two pre-run
   operator sessions for one human `P` (no `sessions` row, no run), lease two distinct
   words, create one run per session presenting each operator token; assert two NON-NULL
   DISTINCT `created_by_handle_id` and `whose RA != whose RB` (`maya#7f3` vs `theo#7f3`). A
   NULL or equal `created_by_handle_id`, or an inability to create the operator session
   without a run, refutes the C1 discharge.
4. **`forged_update_created_by_rejected`** — as `SUTPool`, `UPDATE striatumd.runs SET
   created_by_principal_id = '<other>'` on a stamped run raises the trigger exception
   (write-once). *(plpgsql `RAISE`, SQLSTATE `P0001`, asserted with a sibling helper, not
   `assertSQLState42501`.)*
5. **`two_live_maya`** — the §2.3 collision/escalation invariants (A6, A11) in isolation
   (subsumed by test 3's end-to-end path, retained as a focused unit).
6. **`token_revoked_bare_id`** — revoke the creating client and close its operator session
   (release the lease via the dedicated close path); assert the live-identity render
   (`status --mine`) falls back to the **bare id**, while `whose <past-run>` still renders
   the **frozen** historical `word#suffix`.
7. **`lease_flap_steal`** — A12 (§3).

### 4.6 Forward-only and watermark consistency

- **Forward-only.** `applyPendingOwnerBundles` applies only bundles `> current`
  (`owner.go:305-322`); each commits its `owner_bundle_meta` watermark in the same
  transaction as its DDL (`owner.go:528-532`). 0021 is forward-only by construction. The
  two new tables + the `runs` ALTER + the trigger + grants all live in the one 0021
  transaction — no intra-bundle ordering hazard (no REVOKE).
- **Advance `RequiredOwnerBundleVersion` to 21.** The serving binary's `run.prepare`
  references the new `runs` columns and `operator_handles`/`operator_sessions`; it
  hard-depends on 0021. Setting both `LatestOwnerBundleVersion = 21` and
  `RequiredOwnerBundleVersion = 21` makes `CheckOwnerBundleWatermark` (`owner.go:124-154`,
  run before `ApplyMigrations` at `connection.go:349-351`) halt cleanly
  (`AwaitingOwnerDDLError`) if the daemon restarts before `owner-ddl apply` ran 0021.
- **Deploy ordering (apply THEN restart).** `striatum daemon owner-ddl apply --owner-url …`
  (`daemon.go:84-159` → `db.ApplyOwnerBundles`) applies 0021 as the owner; then restart.
  Restart-first → applied `20 < required 21` → clean halt with the remediation command.

### Falsifiable assertions

- **A13 (owner-only ALTER).** A runtime `ALTER TABLE runs` fails `42501`; under the owner
  it succeeds. *Refuting test:* `owner_bundle_0021_applies_clean`. A runtime ALTER that
  succeeds refutes A13.
- **A14 (write-once at the DB).** No role-routed UPDATE changes a stamped origin column.
  *Refuting test:* `forged_update_created_by_rejected`.
- **A15 (retained privileges exact).** The runtime role can do everything P0 needs (INSERT
  run w/ column; lease+heartbeat+release on both new tables; EXECUTE the projection) and
  nothing more (no `ALTER`, no `DELETE` on the new tables, no `SELECT(principal_id)`).
  *Refuting test:* the positive+negative assertions in `owner_bundle_0021_applies_clean`.
- **A16 (clean apply, non-superuser owner).** 0021 applies via the two-role `OwnerPool`
  with no privilege gap. *Refuting test:* the apply step; any `42501`/`must be member of
  role`/`permission denied for schema` during apply refutes A16.
- **A17 (forward-only).** Re-applying 0021 is a no-op; never applied below its watermark.
  *Refuting test:* apply twice.
- **A18 (watermark interlock).** A binary built against 21 refuses to serve on a DB at
  watermark 20 with `AwaitingOwnerDDLError`. *Refuting test:* boot the new binary against a
  20-watermark DB.
- **A19 (no revoke-last hazard).** 0021 carries no privilege-stripping REVOKE. *Refuting
  test:* grep 0021 for `REVOKE`; presence refutes A19.
- **A28 (two-role stamp safety via the projection).** The run-origin principal stamp PASSES
  the two-role fixture (no `42501`) and routes through `resolve_principal_for_client`, not a
  direct column read. *Refuting test:* `run_origin_stamp_uses_identity_projection` (parts a
  and b). A `42501` on the real stamp path, or evidence the stamp reads
  `principal_clients.principal_id` directly, refutes A28 (and re-opens C2/R2/R4).

---

## §5 — R3: resolve all four open questions (carried forward, DISCHARGED)

### 5.1 OQ1 — Handle pool, default, escalation, denylist → **IN P0**

A **curated lowercase first-names pool** (~256 neutral given names), privacy-safe and
memorable, a Go slice in one package. **Default** = `POOL[fnv64a(principal_id) mod
len(POOL)]` (deterministic from `principal_id`, reconnect-stable, not tty). **Escalation**
= the principal-seeded walk to the next distinct curated word (not numeric suffixes — they
collide with `#suffix`). **Denylist** = reserved words (`daemon`, `scheduler`, `system`,
`admin`, `root`, `unknown`, `anon`, `none`, the `principal_kind` names) are excluded from
the pool entirely; service/`ai_operator` principals draw from a disjoint reserved sub-pool.
Operator-chosen naming deferred (P2).

- **A20.** *Refuting test:* a golden test pins `defaultHandle(P)` stable across runs and
  reconnects, asserts no pool word is on the denylist, and asserts escalation yields
  distinct words; instability or a denied word refutes A20.

### 5.2 OQ2 — Backfill vs NULL → **NULL + advisory `attribution_unknown`, in P0**

Historical runs below the cutover keep `created_by_principal_id = NULL`; the advisory
(non-red) doctor rule `attribution_unknown` surfaces them. **No backfill** — `branch_confirmed_by`
holds `'daemon'`/`'human'` literals (C-4), not a `principal_id`; backfilling would
fabricate identity (the dishonesty R1a forbids). Matches RFC D7.

- **A21.** *Refuting test:* a doctor test asserts a NULL-`created_by_principal_id`
  non-terminal run yields `attribution_unknown` as advisory (not red) and that no migration
  writes a non-NULL value to a pre-cutover run.

### 5.3 OQ3 — Cross-repo board → **per-repo only in P0; daemon-wide DEFERRED (P3)**

P0 is per-repo. `operator_handles` and `operator_sessions` are keyed `(repository_id, …)`;
the live-unique index is per-repo; `whose`/`status --mine` are per-repo. A daemon-wide
board is deferred to P3.

- **A22.** *Refuting test:* assert no P0 surface performs a cross-repo identity
  aggregation.

### 5.4 OQ4 — `@handle#suffix` artifact byline → **OUT of P0 (lands in P2)**

The artifact-byline suffix is out of P0 (scope decision, per SEED). P0 delivers run-origin
attribution; the durable-byline suffix is a P2 change.

- **A23.** *Refuting test:* assert P0 changes no artifact `author_line`/anchor-metadata
  derivation.

---

## §6 — R4: ride RFC 0107; do not rebuild it

- **Operator-id IS `principal_id`.** `operator_handles.principal_id` and
  `operator_sessions.principal_id` are FKs to `striatumd.principals(principal_id)`
  (`0023_principals.sql:30-36`); neither table stores identity attributes — only
  `(repository_id, principal_id, handle/liveness, lease)`. Rendering/liveness layers,
  nothing more.
- **Reuse, don't duplicate.** Client→principal dereference reuses the
  `resolve_principal_for_client` **projection** (C2; `owner/0006:56-79`,
  `principals.go:266-292`) — the reuse v1 claimed and this revision makes real. The lease
  shares the existing session-bound token mint (`mintSessionBoundToken`,
  `session_token.go:60-97`), the `principal_clients` link (`link_client_to_principal`,
  `owner/0006:160-188`), and the `app.session_id` GUC (`authority.go:79,120`). Release is a
  dedicated operator-session close + lazy `leased_until` expiry (the `striatumd.leases`
  idiom, `0005:166-186`). **No parallel identity table, no new reaper** (C-3). The new
  `operator_sessions` table is a liveness anchor, not an identity store, and is the minimal
  delta that makes the pre-run object buildable without touching the run-bound `sessions`
  machinery.
- **Product-boundary clean.** No hosted service, directory, telemetry, or external
  identity; single-human/single-daemon legibility. tty/tmux/title/env never read for state
  (A2/A5). `run_id` stays opaque — the handle lives in a separate column.

- **A24 (no parallel identity).** *Refuting test:* assert `operator_handles`/`operator_sessions`
  have no `display_name`/`kind`/auth columns and identity is read only from `principals`.
- **A25 (no new reaper).** *Refuting test:* assert release happens in the dedicated
  operator-session close + lazy acquisition-path expiry, with no new background
  goroutine/scheduler.
- **A26 (opaque run_id).** *Refuting test:* assert `run_id` generation is unchanged and
  contains no handle.

---

## §7 — P0 boundary and seams for P1–P3 (noted, not designed)

P0 ships: `operator_handles` + live-unique index (§2.2); `operator_sessions` + lifecycle
(§2.6); `runs.created_by_principal_id` + `runs.created_by_handle_id` write-once (§4); the
operator-bootstrap mint+lease RPC riding `mintSessionBoundToken` + `link_client_to_principal`
(§1, §2.6); the projection-routed run-origin stamp (§1(2), C2); `whose <run-id>` (new read
RPC, registered in `contracts/daemon_methods.json` + routes +
`docs/reference/command-authority-matrix.md` + `registry_contract_test`, §9); `status
--mine` manifest; the `attribution_unknown` advisory doctor rule (§5.2). Seams left,
explicitly **not** designed:

- **P1 (custody).** `run_custody_log` appends in the same transaction as the triggering
  state transition; P0 leaves the run-termination/recovery transactions untouched except
  the dedicated operator-session handle release.
- **P2 (honest bylines + handoff naming + chips + OSC title).** OQ4's byline suffix
  (§5.4), the `handle → {color, glyph}` chip function, `handoff_filename`, opt-in OSC-2
  title.
- **P3 (lineage + cross-repo board).** `runs.lineage_id` and OQ3's daemon-wide board
  (§5.3).

---

## §8 — Consolidated falsifiable-assertion ledger

| # | Claim | Supporting evidence (anchor) | Refuting observation / named test |
|---|-------|------------------------------|-----------------------------------|
| A1 | Stamp = live-token principal, server-side, via projection | `run.go:1056-1074` INSERT (bound `$N`); `authority.go:116-120` | forged param / spoof leaks — `run_origin_stamp_uses_identity_projection` |
| A2 | No client-name path to attribution | `run.go:21-28`; `lifecycle.go:71,114-121` | grep finds `created_by`/handle param feeding stamp/lease |
| A3 | Mint+link+lease atomic | `session_token.go:48-53`; `owner/0006:160-188` | token committed without link+lease |
| A4 | Identity from validated token only | `auth_pg.go:49-157,87-92` | revoked token still stamps a run |
| A5 | Read surfaces cannot lie | §2.4 join | tty/pane/title/env in the authoritative answer |
| A6 | Live-unique forces distinct words | `operator_handles_live_uq` (§2.2); `0005:184-186` | duplicate live `maya` / deadlock — `operator_session_pre_run_stamp` |
| A7 | Two terminals → distinct `whose` | §2.5 proof | `whose RA == whose RB` (gate-critical) |
| A8 | Deterministic default, reconnect-stable | §2.3 walk | different word on reconnect |
| A9 | Deterministic escalation, reconnect-stable | §2.3 walk | non-`candidates[1]` / drift |
| A10 | No silent relabel | write-once `created_by_handle_id` (§4.3) | `whose RB` changes after peer close/reconnect |
| A11 | One winner, no deadlock | partial-index serialization (§2.3) | `40P01` / duplicate / both-fail |
| A12 | Flap-resistant renewal | guarded UPDATE (§3) | steal during flap — `lease_flap_steal` |
| A13 | Owner-only ALTER | C-1; `0018:8-22` | runtime ALTER succeeds |
| A14 | Write-once at the DB | trigger (§4.3); `0010:19-49` | UPDATE changes a stamped column — `forged_update_created_by_rejected` |
| A15 | Retained privileges exact | §4.4; `0005:467-475`; `owner/0006:186` | needed op `42501` / surplus grant |
| A16 | Clean apply, non-superuser owner | two-role `OwnerPool` | `42501`/`must be member`/schema-perm on apply |
| A17 | Forward-only | `owner.go:305-322,528-532` | second-apply error / watermark regression |
| A18 | Watermark interlock | `owner.go:124-154`; `connection.go:349-351` | serves on 20-watermark DB |
| A19 | No revoke-last hazard | §4.2 (no REVOKE) | `REVOKE` present in 0021 |
| A20 | Pool/default/escalation/denylist | §5.1 golden test | unstable default / denied word |
| A21 | NULL + advisory, no backfill | §5.2; C-4 | red classification / backfill write |
| A22 | Per-repo only in P0 | §5.3 | cross-repo aggregation in P0 |
| A23 | Byline suffix out of P0 | §5.4 | `author_line` change in P0 |
| A24 | No parallel identity table | §6; `0023_principals.sql:30-36` | identity attribute on the new tables |
| A25 | No new reaper | §6; dedicated close path | new periodic session reaper |
| A26 | Opaque run_id | §6 | handle encoded into `run_id` |
| **A27** | **Operator session buildable pre-run (C1)** | §2.6; `auth_pg.go:104-156`; `authority.go:79,120` | NULL `created_by_handle_id` / can't create without a run — `operator_session_pre_run_stamp` |
| **A28** | **Two-role stamp safety via the projection (C2)** | §1(2); `owner/0006:56-79,181,186`; `principals.go:53-63,266-292` | real stamp `42501` / direct `pc.principal_id` read — `run_origin_stamp_uses_identity_projection` |

---

## §9 — Build manifest (P0 scope, for the downstream `code_change` run)

1. **Owner bundle** — `go/pkg/db/sql/owner/0021_operator_identity_run_attribution.sql`
   (§4.2: `operator_handles` + `operator_sessions` + `runs` columns + trigger + grants +
   watermark); `owner.go` label entry + `LatestOwnerBundleVersion = 21` +
   `RequiredOwnerBundleVersion = 21` (§4.6).
2. **Lease + operator-session layer** — a Go package owning `defaultHandle`/escalation walk
   (§2.3, §5.1 pool), lease acquisition + guarded heartbeat renewal (§3), the
   `operator_sessions` create/heartbeat/close lifecycle (§2.6), graceful release via the
   dedicated operator-session close path (NOT `closeRemainingSessions`).
3. **Operator-bootstrap mint RPC** — a daemon-side mint+lease entry reusing
   `mintSessionBoundToken` + `link_client_to_principal` (projection), creating the operator
   session + token the CLI presents on `run.prepare` so `app.session_id` is populated
   (§1(1), §2.6). `striatum operator bootstrap` becomes its client.
4. **Run stamp** — extend the `runs` INSERT (`run.go:1056-1074`): resolve the principal in
   Go via `admin.ResolvePrincipalForClient` and bind it; `created_by_handle_id` as the
   `operator_handles` subquery keyed on `app.session_id` (§1(2), C2). Delete v1's direct
   `principal_clients` subquery.
5. **`whose <run-id>`** — new read handler (§2.4 join) + `contracts/daemon_methods.json` +
   regenerated routes + `docs/reference/command-authority-matrix.md` row +
   `registry_contract_test`.
6. **`status --mine`** — manifest section + flag, live-handle render with bare-id fallback
   (§4.5 test 6).
7. **Doctor** — `attribution_unknown` advisory rule (§5.2), following
   `doctor_artifact_anchor.go`.
8. **pgtests** — the seven named two-role tests (§4.5), all on `pgtest.TwoRole`, with the
   C1 gate (`operator_session_pre_run_stamp`) and the C2 gate
   (`run_origin_stamp_uses_identity_projection`) exercising the real projection path.
9. **Docs** — update `docs/decisions/decision-log.md`, `docs/reference/spec.md`,
   `CHANGELOG.md`, and re-triage `docs/operator/rfc-roadmap.md` when P0 ships.

This is the published claim. Gate-critical targets: **A7/A27** (R1b sufficiency PROVEN on a
buildable operator-session substrate — C1) and **A28/A13/A14/A16** (R2/R4 owner-bundle
two-role safety with the projection-routed stamp + DB write-once — C2). The §0 corrections
and the §C discharge map are load-bearing; challenge them at source if you believe any is
wrong.
