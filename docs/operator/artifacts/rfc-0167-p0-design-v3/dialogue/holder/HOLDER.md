# HOLDER — RFC 0167 P0 falsifiable implementation SPEC, REVISION v3 (operator identity & run attribution)

author: holder-author-001

> This is the **v3 revision** of the RFC 0167 P0 implementation SPEC, published as the
> claim the two falsifiers re-attack. The v2 falsification gate
> (`rfc-0167-p0-design-v2`) returned `needs_revision` and, its single revision cycle
> exhausted, routed two NEW, source-verified, critical constraints to the operator:
> **C1′** — the pre-run operator-session token cannot authorize `run.prepare` (an admin
> route) because the reused `mintSessionBoundToken` carries no `admin`, so the
> two-terminal sufficiency proof collapses at the *authorization* layer; and **C2′** —
> the v2 `operator_sessions` substrate (carrying both `principal_id` and `client_id`)
> plus 0021's table-wide `SELECT` grant reopens the `client_id → principal_id` mapping
> bundle 0006 deliberately closed. This revision **discharges both** and **carries
> forward, unregressed, everything cleared through v1+v2**. It is built on the v2 SPEC
> (`docs/operator/artifacts/rfc-0167-p0-design-v2/dialogue/holder/HOLDER.md`), **not**
> rewritten from scratch; the v2 cycle-1 ledger's `findings:`/`constraints:` blocks
> (C1′ = `C1-OPERATOR-TOKEN-AUTHORIZATION`, C2′ = `C2-OPERATOR-SESSIONS-READ-SCOPE`) are
> the prescribed fixes. Every load-bearing claim is a **falsifiable assertion** paired
> with the **named test/check that would refute it**, anchored to **verified
> current-branch source** (`go/pkg/...` `file:line`). I re-read the source on the
> `striatum/rfc-0167-p0-design-v3` worktree; §D records the discharge map, §C1′/§C2′
> record the new mechanisms + verifications. Scope is **P0 only**.

---

## §D — Addressing the v2 constraints (the auditable revision map)

This section exists so the falsifiers can verify the discharge directly. C1′ and C2′ are
each RESOLVED with a named, source-anchored mechanism and named two-role pgtests/controls;
every v1+v2 carry-forward is INTACT.

### C1′ — RESOLVED: the operator-session token authorizes `run.prepare` via a DISTINCT operator-token mint carrying `admin`, leaving the lane slice untouched

**The v2 gap (verified, re-confirmed on this branch).** `run.prepare` (and `run.start`)
require **`CapabilityAdmin`** (`registry_methods.go:110-111`, VERIFIED; pinned in
`registry_rfc0043_test.go:27-28`). Capability resolution is an **exact match**: `Authorize`
selects a `client_capabilities` row `WHERE client_id = $1 AND capability = $2` with
`$2 = string(*required)` (`auth_pg.go:104-117`, VERIFIED) — `admin` is **not** a superset of
`read`/`write`/`claim`, and a token lacking an exact `capability='admin'` row is denied
`capability_missing` (`auth_pg.go:140`). The reused `mintSessionBoundToken` inserts exactly
the fixed slice `sessionBoundCapabilities = {claim, write, read, review}`
(`session_token.go:46`, inserted verbatim `session_token.go:77-89`, VERIFIED) — **no admin**.
The dispatcher authorizes **before** the handler: `auth = s.Authorizer.Authorize(...)` →
`RequireAllowed(...)` → only then `s.route(routeCtx, envelope)` (`server.go:107-124`,
VERIFIED). So a v2-shaped operator token is rejected `capability_missing` **before**
`HandleRunPrepare` runs, before the prelude sets `app.session_id`, before the
`created_by_handle_id` subquery can run. `S1→RA` / `S2→RB` stop at authorization.

**The resolution.** Mint the pre-run operator-session token through a **DISTINCT operator-
token mint path** that inserts an **explicit capability set including `admin`**, bound to the
`operator_session_id` — **not** by widening the shared lane slice. This is reuse, not new
machinery: `admin.insertTokenClient` (`tokens.go:286-315`, VERIFIED) **already** mints a
client + a *caller-supplied* set of capability grants (one `client_capabilities` row per
grant); `mintSessionBoundToken` (`session_token.go:60-97`) **already** binds those rows to a
`session_id`. The operator mint is their composition — `mintOperatorSessionToken(ctx, tx,
repositoryID, operatorSessionID)` — inserting `operatorSessionCapabilities = {admin, read}`,
each a distinct row carrying `session_id = operator_session_id` (§C1′). Because resolution is
exact-match, the `admin` row authorizes `run.prepare`/`run.start`, and the `read` row
authorizes the manifest surfaces (`whose`, `status --mine`), while the prelude installs
`app.session_id` from the bound grant (`auth_pg.go:145-153` → `authority.go:79,119,158`).

- **Lane tokens are unchanged and CANNOT gain admin** — a *source fact*, not merely a test.
  `sessionBoundCapabilities` stays `{claim, write, read, review}` (`session_token.go:46`); a
  lane token has no `capability='admin'` row, so `run.prepare` is denied `capability_missing`
  by exact-match (`auth_pg.go:104-140`). No widening of the shared slice anywhere.
- **`admin` is faithful, not an over-grant.** The operator (the human) **already** holds
  `admin` today via the shared static admin runtime token — that is how operators prepare
  runs now. The session-bound operator token is the *lifecycle-limited* materialization of
  that same authority: TTL-bounded, session-bound, and **revoked on operator-session close**
  (below) — strictly **less** standing than today's static admin token. It is not strictly
  more than the operator role needs: `run.pause/resume/cancel/retry_job` are also `admin`
  (`registry_methods.go:112-115`) and all legitimately operator-driven, so "exactly
  prepare/start" is too narrow for the human-at-a-terminal; `admin` is the correct grant.
  The boundary that matters — **lane ≠ admin** — is preserved.
- **A closed/expired operator session cannot keep authorizing or stamping.** Graceful close
  **revokes** the operator token's capability grants (`client_capabilities.revoked_at =
  now()`) and releases the handle in one transaction; TTL expiry bounds the token's
  `expires_at`. `Authorize` rejects a revoked grant (`auth_pg.go:111` `revoked_at IS NULL`
  filter → `capability_missing`) and an expired one (`auth_pg.go:90-91,142-143` →
  `token_expired`/`capability_expired`) **before** `run.prepare` runs → no stale-token NULL
  stamp, no expired-handle reuse.

**Gate + controls (`operator_session_pre_run_stamp`, two-role, §4.5):** two pre-run operator
sessions for ONE human, minted via the operator path; one run per session through the **real
`run.prepare` authorization path**; assert two **NON-NULL DISTINCT** `created_by_handle_id`
and `whose RA != whose RB`. PLUS (i) an ordinary **lane** session token is denied
`run.prepare` `capability_missing` (no admin row); (ii) a **closed/expired** operator session
is denied `run.prepare` and creates no run / no stamp.

### C2′ — RESOLVED: `operator_sessions` is granted COLUMN-scoped SELECT that excludes `principal_id` AND `client_id` (no table-wide grant, no REVOKE needed)

**The v2 gap (verified, re-confirmed on this branch).** v2 §2.6's `operator_sessions` carries
both `principal_id` and `client_id`, and v2 §4.2(5) granted table-wide `SELECT, INSERT,
UPDATE ON striatumd.operator_sessions TO striatumd_rw`. So `SELECT client_id, principal_id
FROM operator_sessions WHERE state='active'` succeeds under `striatumd_rw`, recovering the
`client_id → principal_id` mapping bundle 0006 deliberately closed on `principal_clients`
(`REVOKE SELECT`; grant back only `(client_id, linked_at, unlinked_at)`; rationale: "Without
`principal_id` a leaked runtime credential sees client ids and timestamps, not whose
credentials they are" — `0006_identity_read_scope.sql:218-220`, VERIFIED; reasserted
`owner.go:464-467`, VERIFIED). v2's "no `SELECT(principal_id)` grant-back" was scoped only to
`principal_clients`; the same identity linkage reopened by another name.

**The resolution (Option A — column-level grant, the 0006 precedent; chosen over a SECURITY
DEFINER projection).** Bundle 0021 grants `operator_sessions` **column-scoped SELECT that
excludes `principal_id` AND `client_id`**, keeping only the liveness/state columns the runtime
legitimately reads:

```sql
-- operator_sessions is NEW in 0021, so the runtime never had table-wide SELECT on it:
-- grant only the narrow column set from creation. NO REVOKE (nothing to revoke).
GRANT SELECT (operator_session_id, repository_id, state,
              registered_at, last_heartbeat_at, expires_at, closed_at, close_reason)
  ON striatumd.operator_sessions TO striatumd_rw;
GRANT INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw;   -- no DELETE: append/retain
```

This is chosen over a projection because: (1) it is the **exact 0006 precedent** — a column
gate, not a function (`0006:218-220`; `owner.go:464-467`); (2) the runtime **genuinely never
reads** `operator_sessions.principal_id` or `client_id` on any path (proof below), so no
projection is warranted; (3) because the table is **new in 0021** the grant is column-scoped
*from creation* with **no `REVOKE`**, so **carry-forward A19 (no revoke-last watermark hazard)
stays intact** — strictly simpler than a projection (which adds a function + EXECUTE grant +
a Go shim). PostgreSQL requires `SELECT` only on the columns a write statement *reads*
(`0006:204-208`); the heartbeat/close `UPDATE`s read only `operator_session_id` + `state` in
their `WHERE` (both granted), and `INSERT` needs no `SELECT` — so the narrow grant suffices.

**Why no runtime path needs the two withheld columns.** The run-origin stamp resolves the
principal in **Go** via `admin.ResolvePrincipalForClient(ctx, tx, auth.ClientID)` from the
**live token's** `client_id` (the C2 projection, INTACT) — never from `operator_sessions`; the
handle comes from `operator_handles` keyed on `app.session_id` (§1(2)); `status --mine`
filters `runs` by the Go-resolved principal and renders the live handle from
`operator_handles`; liveness reads use only `state`/`expires_at`/`last_heartbeat_at`. The
`principal_id`/`client_id` columns are written at **INSERT** (table-level INSERT privilege,
no SELECT needed; the values are Go-held from the mint, not read back) and retained for
owner-context provenance and the `operator_sessions_principal_live` index, but are **never
SELECTed by the runtime role**.

**Controls (§4.5):** **Negative** — `SELECT client_id, principal_id FROM operator_sessions
WHERE state='active'` as `striatumd_rw` fails `42501` (columns ungranted). **Positive** —
create / heartbeat / close + the run stamp + authorized `whose` / `status --mine` all work
through the narrow grant.

### Carry-forwards — INTACT through v1+v2 (not reopened, not weakened)

| Carry-forward | Status in v3 | Where |
|---|---|---|
| **R1a honesty (A1–A5)** | INTACT — stamp from the live-token prelude GUC / Go-resolved principal, never an envelope/tty/tmux/title/env; reads are pure PG joins | §1 |
| **R1b ARCHITECTURE** — per-session `created_by_handle_id` snapshot + live-unique partial index + deterministic principal-seeded escalation walk + run→handle_id join | INTACT (now proven executable through the real `run.prepare` authorization path — C1′) | §2 |
| **v2 C1 storage substrate** — dedicated owner-held `operator_sessions` pre-run liveness anchor; never touches `striatumd.sessions`; the `auth_pg.go:104-156` no-`sessions`-join fact | INTACT (the table stays; only its GRANT narrows — C2′; its token now carries admin — C1′) | §2.6, §C1′, §C2′ |
| **v2 C2 projection stamp** — principal resolved in Go via `resolve_principal_for_client`, bound `$N`; direct `pc.principal_id` subquery deleted; `42501` direct-read control | INTACT (unchanged) | §1(2) |
| **R1c flap renewal (A12)** — guarded UPDATE, never release-then-reacquire | INTACT (also governs operator-session + token heartbeat) | §3 |
| **R2 DB-write-once** — `BEFORE UPDATE` trigger `refuse_run_origin_change()`; owner bundle **0021** (`LatestOwnerBundleVersion==20`, VERIFIED → next-free 21); forward-only / watermark interlock | INTACT (no REVOKE added — A19 intact) | §4 |
| **R3 four open questions** (OQ1 pool/default/escalation/denylist; OQ2 NULL+advisory, no backfill; OQ3 per-repo, P3 deferred; OQ4 byline P2) | INTACT | §5 |
| **R4 reuse** — FK rendering layer over `principal_id`, no parallel identity table, opaque `run_id`; the operator mint reuses `insertTokenClient` + `mintSessionBoundToken` shapes | INTACT (operator_sessions is a liveness anchor, not an identity store) | §6 |
| **Source corrections C-1..C-4** | INTACT | §0 |

**No carry-forward regressed.** The only deltas v3 introduces over v2 are: (1) a **distinct
operator-token mint** carrying `{admin, read}` bound to the operator session, with close/expiry
revoking it (C1′); (2) the `operator_sessions` GRANT narrows to a **column subset excluding
`principal_id`/`client_id`** (C2′). The lane slice, the storage table, the projection stamp,
the write-once trigger, bundle ordinal 21, the watermark interlock, and the four OQs are all
unchanged.

---

## How this SPEC discharges R1a / R1b / R1c / R2 / R3 / R4 (auditable coverage map)

| Req | What it demands | Where | Load-bearing assertion(s) |
|-----|-----------------|-------|---------------------------|
| **R1a** | Identity bound server-side at token-mint against the live token; never tty/tmux/title/env; reads resolve through `principal_id`, only snapshot the handle | §1 | A1–A5 |
| **R1b** | THE CRUX — one human = one `principal_id` across ~15 terminals; the deterministic escalation rule, the exact run→handle join, and **two same-human terminals return two distinct answers** on a **buildable + authorizable** substrate | §2, §C1′ | A6–A11, A27, **A29–A32** |
| **R1c** | Heartbeat renews via guarded UPDATE, never release-then-reacquire | §3 | A12 |
| **R2** | Owner-bundle migration at the next free ordinal; DB write-once + justify; pin retained privileges (no identity grant-back); prove clean apply + write-once + two-role stamp safety; forward-only, watermark-consistent | §4, §C2′ | A13–A19, A28, **A33–A34** |
| **R3** | Resolve all four open questions concretely | §5 | A20–A23 |
| **R4** | Ride RFC 0107 (operator-id IS `principal_id`); no parallel identity table; reuse principals/principal_clients/session liveness/the identity projection/the token-mint shapes; product-boundary clean | §6 | A24–A26, A28 |

The full assertion ledger is §8 (A1–A34). The P0 boundary and P1–P3 seams are §7. The build
manifest is §9.

---

## §C1′ — The operator-token authorization discharge (full mechanism)

### C1′.1 The exact source chain v2 broke (and why)

```
run.prepare → CapabilityAdmin              registry_methods.go:110 (pinned registry_rfc0043_test.go:27)
Authorize: WHERE capability = $2 (exact)   auth_pg.go:104-117   → no admin row ⇒ capability_missing (140)
lane mint slice = {claim,write,read,review} session_token.go:46  → inserted verbatim 77-89  → NO admin
dispatcher: Authorize → RequireAllowed → route  server.go:107-124  → denial precedes HandleRunPrepare
                                                                     ⇒ before prelude sets app.session_id
```

The v2 operator token (minted via the unmodified `mintSessionBoundToken`) therefore cannot
clear `Authorize` for `run.prepare`; the prelude never runs; `created_by_handle_id` is never
stamped; the §2.5 proof cannot execute end-to-end. **This is the whole of C1′.**

### C1′.2 The distinct operator-token mint (the fix)

A new daemon-side mint, `mintOperatorSessionToken`, is the **composition of two existing
shapes** (reuse, R4):

```go
// operatorSessionCapabilities — the EXPLICIT capability set for the pre-run operator
// session token. Distinct from sessionBoundCapabilities (the lane slice, UNCHANGED).
// admin: run.prepare / run.start / run.pause / run.resume / run.cancel / run.retry_job —
//        the run-lifecycle routes the human operator legitimately drives (the same admin
//        authority the human holds today via the shared static admin runtime token).
// read:  whose / status --mine / dashboard — the manifest surfaces (exact-match, so read
//        needs its own row; admin does NOT subsume read — auth_pg.go:104-117).
var operatorSessionCapabilities = []rpc.Capability{rpc.CapabilityAdmin, rpc.CapabilityRead}

// mintOperatorSessionToken: like mintSessionBoundToken (session-bound, same HMAC/insert
// shape, session_token.go:60-97) but inserts operatorSessionCapabilities instead of the
// lane slice — exactly the caller-supplied-capability-set shape insertTokenClient already
// uses (tokens.go:286-315). One client_capabilities row per capability, each carrying
// session_id = operatorSessionID so the prelude installs app.session_id (authority.go:119).
```

- The `admin` row satisfies `Authorize` for `run.prepare` by exact match
  (`auth_pg.go:104-117`); `HandleRunPrepare` runs; the prelude sets `app.session_id =
  operator_session_id` (`authority.go:79,119,158`); the §1(2) `created_by_handle_id` subquery
  resolves the lease.
- The shared `sessionBoundCapabilities` slice is **NOT touched** (`session_token.go:46`
  unchanged). Lane tokens carry no admin row ⇒ `run.prepare` denied `capability_missing`.
- The operator mint runs inside the operator-bootstrap transaction (§1(1)), atomic with the
  `operator_sessions` row, the `principal_clients` link, and the handle lease (A3).

> **Capability-authority-matrix note.** No `run.*` method changes its required capability;
> `registry_rfc0043_test.go:27-28` stays green. The only registry-surface additions are the
> P0 read RPCs (`whose`, and the `status --mine` extension), per §9. The operator token rides
> the **existing** `admin`→`run.prepare` mapping; nothing in the authority matrix is rewritten.

### C1′.3 Lifecycle vs. authorization (close/expiry cannot keep stamping)

- **Graceful close** (operator session end / `operator bootstrap --close`): one transaction
  sets `operator_sessions.state='closed', closed_at=now()`, sets
  `operator_handles.released_at=now(), release_reason='operator_session_closed'`, **and
  revokes the operator token** (`UPDATE striatumd.client_capabilities SET revoked_at=now()
  WHERE client_id=$operator_client AND revoked_at IS NULL`). A subsequent `run.prepare` with
  the stale token is denied `capability_missing` (`auth_pg.go:111` filters `revoked_at IS
  NULL`; `:140`) **before** the handler.
- **Lazy expiry** (no reaper, C-3): the operator token's `expires_at` (clients +
  client_capabilities) is `≤ operator_sessions.expires_at`. An expired token fails
  `token_expired` / `capability_expired` (`auth_pg.go:90-91,142-143`) at `Authorize`.
- **Belt-and-suspenders:** even were a token momentarily valid after the handle released,
  the `created_by_handle_id` subquery (`WHERE released_at IS NULL`, §1(2)) yields NULL — but
  the revoke above means `run.prepare` is refused outright, so no run is created at all.

### Falsifiable assertions (C1′)

- **A29 (operator token authorizes `run.prepare`).** The operator-session token (admin row,
  session-bound) clears `Authorize` for `run.prepare`, the prelude sets `app.session_id`, and
  `created_by_handle_id` is stamped NON-NULL. *Refuting test:* `operator_session_pre_run_stamp`
  (§4.5) — a `capability_missing` on `run.prepare`, or a NULL `created_by_handle_id`, refutes
  A29 (re-opens C1′).
- **A30 (lane tokens do NOT gain admin — source fact + control).** A lane token (slice
  `{claim,write,read,review}`, `session_token.go:46`) has no admin row and is denied
  `run.prepare` `capability_missing` by exact-match (`auth_pg.go:104-140`). *Refuting test:*
  the `operator_session_pre_run_stamp` control (i) — a lane token that successfully prepares a
  run, or an `admin` row found on a lane token, refutes A30.
- **A31 (closed/expired operator session cannot stamp).** Close revokes the operator token +
  releases the handle in one txn; expiry bounds the TTL. A stale/expired operator token is
  denied `run.prepare` and creates no run / no NULL stamp. *Refuting test:* control (ii) — a
  stamped (or NULL-stamped) run via a closed/expired operator token refutes A31.
- **A32 (distinct mint, not the lane slice).** The operator capability set lives in a separate
  mint; `sessionBoundCapabilities` is unchanged. *Refuting test:* a grep/unit check — `admin`
  appended to `sessionBoundCapabilities`, or any lane-minted token carrying an admin grant,
  refutes A32.

---

## §C2′ — The operator_sessions read-scope discharge (full mechanism)

### C2′.1 The grant delta (replaces v2 §4.2(5) for operator_sessions)

```sql
-- v2 (REGRESSION):  GRANT SELECT, INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw;
-- v3 (DISCHARGE):   column-scoped SELECT excluding principal_id AND client_id; INSERT/UPDATE table-level.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT (operator_session_id, repository_id, state,
                  registered_at, last_heartbeat_at, expires_at, closed_at, close_reason)
      ON striatumd.operator_sessions TO striatumd_rw;
    GRANT INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw;   -- no DELETE
    -- operator_handles is unchanged (no identity-mapping columns: principal_id alone, no client_id):
    GRANT SELECT, INSERT, UPDATE ON striatumd.operator_handles TO striatumd_rw;  -- no DELETE
  END IF;
END $$;
```

`principal_id` and `client_id` remain columns of the table (written at INSERT, retained for
provenance + the principal-live index) but are **never granted SELECT** to the runtime role.
No `REVOKE` appears — the table is new in 0021, so there is no prior table-wide SELECT to
strand (A19 intact).

> **Drift parity (optional hardening).** Mirror 0006's `ReassertReadRevokes` cohort
> (`owner.go:459-468`) by adding the `operator_sessions` column-scoped GRANT to the reassert
> list, so a startup parity pass re-applies the narrow grant and a manual drift cannot
> silently widen it. Not load-bearing for the gate (the negative-control pgtest is), but it
> matches how 0006 keeps `principal_clients` closed.

### C2′.2 Why `operator_handles` does NOT need the same treatment

`operator_handles` carries `principal_id` but **no `client_id`** (§2.2) — it is the rendering
layer (FK to `principals`), and the runtime must SELECT it to render `whose`/`status --mine`
and to run the `created_by_handle_id` subquery. There is no `client_id → principal_id` pairing
in a single `operator_handles` row, so reading it does not reconstitute the credential→identity
map 0006 closed. (`principal_id` alone is RFC 0107's public operator-id; `whose` renders it
intentionally.) Only `operator_sessions` pairs `client_id` with `principal_id`, so only it
needs the column gate.

### Falsifiable assertions (C2′)

- **A33 (identity mapping unreadable by the runtime).** `SELECT client_id, principal_id FROM
  striatumd.operator_sessions WHERE state='active'` as `striatumd_rw` fails `42501` (columns
  ungranted). *Refuting test:* `owner_bundle_0021_applies_clean` negative control — the read
  succeeding (by table-wide grant residue, a view, or a function) refutes A33 (re-opens C2′).
- **A34 (narrow grants still suffice — positive control).** Create (INSERT), heartbeat
  (UPDATE liveness), close (UPDATE) + the run stamp (reads `operator_handles`, not
  `operator_sessions`) + authorized `whose` / `status --mine` all succeed through the narrow
  grant. *Refuting test:* the positive control — a `42501` on any legitimate operator-session
  op refutes A34.
- **A19 (no revoke-last hazard) — REASSERTED.** 0021 still carries **no privilege-stripping
  REVOKE** (the operator_sessions grant is column-scoped from creation). *Refuting test:* grep
  0021 for `REVOKE`; presence refutes A19.

---

## §0 — Verified source baseline and the four corrections (carried forward from v2; + v3 verifications)

The holder verifies, does not trust. I re-read the source on this branch. **Verified true
(load-bearing, unchanged):**

- `LatestOwnerBundleVersion == 20`, `RequiredOwnerBundleVersion == LatestOwnerBundleVersion`
  (`owner.go:23,35`). **Next free owner-bundle ordinal is 21.**
- Tables are owner-held by default and runtime-`ALTER`-able only after an owner bundle
  transfers ownership; `runs`/`sessions` are not in bundle 0018's transfer cohort, so both are
  owner-held (C-1).
- `mintSessionBoundToken` runs inside the registration transaction (`session_token.go:48-53`),
  writes `clients` + `client_capabilities` only.
- The authority prelude installs `striatum.principal_id` (= `auth.ClientID`) and
  `app.session_id` (= `auth.SessionID`) as transaction-local GUCs
  (`authority.go:79,116-120,158`).
- `pgtest.TwoRole(t)` with `OwnerPool`/`SUTPool` (`two_role.go:47-78`); the `42501` oracle is
  `assertSQLState42501` (`two_role_pg_test.go:161-176`).
- **v2 verifications (INTACT):** `auth_pg.go:104-156` reads the bound session from
  `client_capabilities.session_id` and never joins `sessions` (makes the pre-run operator
  session buildable, C1); `resolve_principal_for_client` is `SECURITY DEFINER`, `GRANT EXECUTE
  ... TO striatumd_rw` (`owner/0006:56-79,181,186`) (makes the projection stamp safe, C2).
- **v3 verifications (new):** capability resolution is exact-match
  (`auth_pg.go:104-117`, `:169-178`); `run.prepare`/`run.start` → `CapabilityAdmin`
  (`registry_methods.go:110-111`; `registry_rfc0043_test.go:27-28`); the dispatcher authorizes
  before routing (`server.go:107-124`); `admin.insertTokenClient` mints a caller-supplied
  capability set (`tokens.go:286-315`); the 0006 column-gate precedent and its reassertion
  (`0006:218-220`; `owner.go:464-467`).

**CORRECTION C-1 — `ALTER runs ADD COLUMN` and the two new tables are OWNER-bundle changes
(ownership, not FKs).** `runs` is owner-held on a two-role deploy and not in 0018's cohort, so
a runtime `ALTER runs` dies `42501 must be owner of table runs` (the #441/#442 / D248 trap,
`0018:8-22`). `ALTER runs ADD COLUMN`, `operator_handles`, and `operator_sessions` all go in
owner bundle 0021.

**CORRECTION C-2 — the authority GUC `striatum.principal_id` holds the `client_id`; the
dereference goes through the projection, not a direct column read.** The run-origin principal
resolves in Go via `admin.ResolvePrincipalForClient` (the `resolve_principal_for_client`
projection) and binds as a parameter (§1(2)). (INTACT from v2.)

**CORRECTION C-3 — there is no periodic session reaper; release is graceful-close + lazy
expiry.** P0 releases handle leases (a) gracefully in a close transaction (now also revoking
the operator token, §C1′.3) plus (b) lazily via a `leased_until` TTL (the `striatumd.leases`
idiom, `0005:166-186`). The operator session uses a dedicated close path, never run-scoped
`closeRemainingSessions` (`mutations.go:1432`).

**CORRECTION C-4 — `created_by_principal_id` alone is insufficient; OQ2's backfill source
carries no identity.** `branch_confirmed_by` holds `'daemon'`/`'human'` literals
(`run.go:1053,887-891`), not a `principal_id`; the disambiguator is the per-session
`runs.created_by_handle_id` snapshot (§2).

---

## §1 — R1a: identity bound server-side, at token-mint, against the live token

### Design

**(1) The handle lease + operator token are minted in one operator-bootstrap transaction.**
The operator-bootstrap mint RPC, in one transaction: (a) resolves/creates the caller's
`principal` (`kind='human'`, via the owner-owned create + `link_client_to_principal`
projection, `owner/0006:160-188`); (b) mints a session-bound operator token via
**`mintOperatorSessionToken`** (§C1′.2 — capability set `{admin, read}`, bound to a fresh
`operator_session_id`); (c) INSERTs the `operator_sessions` row; (d) acquires the handle lease
into `operator_handles` keyed on `principal_id` and `leased_session_id = operator_session_id`
(§2.2). Mint + link + lease + operator-session share one transaction, so no token exists
without its principal link and handle lease, and no client RPC can interpose a name between
them (R1a / A3).

> The `principal_id` for the lease is the one resolved/created **in this transaction** — held
> directly in Go, bound as a parameter to the lease INSERT (no `principal_clients` read on this
> path). The projection route (C2) governs the *run-origin* stamp, where only a `client_id` is
> in hand. The `operator_sessions.principal_id`/`client_id` columns are likewise written from
> Go-held values at INSERT (no SELECT, §C2′).

**(2) `runs.created_by_principal_id` is resolved from the live token through the identity
projection (C2, INTACT).** In `HandleRunPrepare`'s authorized transaction:

```go
auth := rpc.AuthFromContext(ctx)                                         // ClientID from the validated live token
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
    WHERE oh.leased_session_id = current_setting('app.session_id', true)  -- = operator_session_id (set by the admin-authorized prelude, C1′)
      AND oh.released_at IS NULL)
);
```

`HandleRunPrepare` reads only `repository_id`/`workflow` from the envelope (`run.go:21-28`); a
forged `created_by_*` request param is structurally ignored. The principal comes from
`auth.ClientID` through the projection; the handle from the prelude `app.session_id` GUC — now
reachable because the operator token authorizes `run.prepare` (C1′). The v1 direct
`principal_clients` subquery stays **deleted**.

**(3) Every read surface resolves through `principal_id` and only snapshots the handle.**
`whose`, `status --mine`, `doctor`, evidence export render from a pure PG join over
`created_by_principal_id`/`created_by_handle_id`/`operator_handles`/`principals` (§2.4). No
tty/pane/tmux/title/env value enters any authoritative answer.

### Falsifiable assertions

- **A1 (server-side stamp).** `runs.created_by_principal_id` equals the live token's principal,
  dereferenced server-side through the projection. *Refuting test:*
  `run_origin_stamp_uses_identity_projection` (§4.5) — create a run on `P_A`'s token passing
  envelope `created_by_principal_id = P_B` + tty/tmux/title/env spoofs; the stored value must
  be `P_A`. `P_B` or a spoof leak refutes A1.
- **A2 (no client-name path).** No client-supplied string becomes `created_by_principal_id` or
  a rendered handle. *Refuting test:* a static guard greps the `run.prepare` / operator-mint
  handlers for `stringParam(envelope, "created_by*"|"handle"|"operator*")` feeding the
  stamp/lease; presence refutes A2.
- **A3 (mint+link+lease atomicity).** The handle lease, `principal_clients` link, and
  `operator_sessions` row commit in the same transaction as the operator token. *Refuting
  test:* inject a fault between `mintOperatorSessionToken` and the lease INSERT; assert the
  whole bootstrap rolls back (no token without its link + lease). A token committed without a
  lease refutes A3.
- **A4 (live-token resolution).** Identity derives from `auth.ClientID` set by `Authorize` from
  the validated token. *Refuting test:* present a revoked/expired token to `run.prepare`;
  assert it is rejected (`auth_pg.go:87-92`) and no run row is created.
- **A5 (read surfaces cannot lie).** `whose`'s authoritative answer is a function of
  `{created_by_principal_id, created_by_handle_id}` joined to `operator_handles`/`principals`
  only. *Refuting test:* a unit test asserts the `whose` SQL references no tty/pane/title/env.

---

## §2 — R1b (THE CRUX): per-human `principal_id` vs per-terminal session granularity (carried forward; now authorizable)

The architecture is unchanged from v2; C1′ makes the proof *execute* through the real
`run.prepare` authorization path.

### 2.1–2.4 (carried forward, unchanged)

`created_by_principal_id` alone cannot answer "which window" (one human = one principal,
identical suffix). The disambiguator is a **per-session leased handle** in `operator_handles`
(§2.2) with the live-unique partial index `operator_handles_live_uq (repository_id,
lower(handle)) WHERE released_at IS NULL` and `operator_handles_live_session_uq`. A
deterministic, principal-seeded candidate walk drives both default and escalation (§2.3): a
lone session lands on `candidates[0]`; a second concurrent same-human session hits `23505` and
deterministically leases `candidates[1]` (a distinct curated word). The run carries a
**write-once `runs.created_by_handle_id` snapshot** (FK → `operator_handles.handle_id`); `whose`
joins through it (§2.4) — never through the live lease (ambiguous: one principal, ≤15 live
leases). Lazy expiry only sets `released_at`; rows are never deleted (§4.4), so a snapshot
`handle_id` never dangles and a re-lease creates a *new* row (no silent relabel).

```sql
-- whose <run-id> — the pure join that cannot lie (carried forward):
SELECT r.run_id, r.state, r.created_by_principal_id,
       oh.handle AS origin_handle, p.principal_kind, p.disabled_at
  FROM striatumd.runs r
  LEFT JOIN striatumd.operator_handles oh ON oh.handle_id = r.created_by_handle_id
  LEFT JOIN striatumd.principals       p  ON p.principal_id = r.created_by_principal_id
 WHERE r.repository_id = $1 AND r.run_id = $2;
```

### 2.5 PROOF — two same-human terminals → two distinct answers, through the REAL authorization path

Human `H` → principal `P`; `defaultHandle(P) = candidates[0] = "maya"`.

1. **Operator session `S1`** at bootstrap → one txn: resolve `P`, mint operator token via
   `mintOperatorSessionToken` (capabilities `{admin, read}`, binding `session_id = S1`),
   `link_client_to_principal(client_S1, P)`, INSERT `operator_sessions(S1)`, lease walk
   `INSERT maya` → `S1` holds `maya` (`handle_id = h1`).
2. **Operator session `S2`** (same human, second terminal) → one txn: resolve `P`, mint
   operator token (`{admin, read}`, binding `session_id = S2`), link `client_S2 → P`, INSERT
   `operator_sessions(S2)`, lease walk `INSERT maya` → `23505` → `INSERT theo` → `S2` holds
   `theo` (`handle_id = h2`).
3. `S1` calls `run.prepare` — its token's **admin** row clears `Authorize`
   (`auth_pg.go:104-117`); the prelude sets `app.session_id = S1`, `auth.ClientID = client_S1`
   → run `RA` → `created_by_principal_id = ResolvePrincipalForClient(client_S1) = P`,
   `created_by_handle_id = (SELECT handle_id WHERE leased_session_id = S1 AND released_at IS
   NULL) = h1`.
4. `S2` calls `run.prepare` (token `S2` → `app.session_id = S2`) → run `RB` →
   `created_by_principal_id = P`, `created_by_handle_id = h2`.
5. `whose RA` → `maya#7f3`. `whose RB` → `theo#7f3`.

**Two distinct answers** (`maya#7f3` ≠ `theo#7f3`); the identical suffix signals "same human."
Every step now both **builds** (operator session in `operator_sessions`, no `sessions` row, C1)
**and authorizes** (admin-bearing operator token clears `run.prepare`, C1′). This is the
`operator_session_pre_run_stamp` pgtest (§4.5).

### 2.6 The pre-run operator-session substrate + lifecycle (carried forward; close now revokes the token)

**Storage — `operator_sessions` (owner bundle 0021), unchanged from v2** except its GRANT
(§C2′). No `run_id`, no FK to `runs`; the `sessions.run_id NOT NULL` FK is structurally
inapplicable.

```sql
CREATE TABLE striatumd.operator_sessions (
  repository_id     text NOT NULL REFERENCES striatumd.repositories(repository_id),
  operator_session_id text NOT NULL,
  principal_id      text NOT NULL REFERENCES striatumd.principals(principal_id),  -- SELECT NOT granted to runtime (C2′)
  client_id         text NOT NULL,                                               -- SELECT NOT granted to runtime (C2′)
  state             text NOT NULL CHECK (state IN ('active','closed','expired')),
  registered_at     timestamptz NOT NULL,
  last_heartbeat_at timestamptz,
  expires_at        timestamptz NOT NULL,
  closed_at         timestamptz,
  close_reason      text,
  PRIMARY KEY (repository_id, operator_session_id)
);
CREATE INDEX operator_sessions_principal_live
  ON striatumd.operator_sessions (repository_id, principal_id)
  WHERE state = 'active';
```

**Lifecycle:**
- **Create** — the operator-bootstrap mint, one transaction (§1(1)): resolve/create `P`, mint
  operator token (`{admin, read}`) bound to a fresh `operator_session_id`, link `client → P`,
  INSERT `operator_sessions(state='active', expires_at=now()+TTL)`, acquire the lease. Atomic
  (A3).
- **Heartbeat** — guarded UPDATE (R1c, §3): renew `operator_sessions` (liveness columns) and
  `operator_handles` (lease) and the operator token's `expires_at`. Never
  release-then-reacquire.
- **Graceful close** — one transaction: `operator_sessions.state='closed', closed_at=now()`;
  `operator_handles.released_at=now(), release_reason='operator_session_closed'`; **and revoke
  the operator token** (`client_capabilities.revoked_at=now()`, §C1′.3). Dedicated path — never
  `closeRemainingSessions` (`mutations.go:1432`, keys on `run_id`).
- **Lazy expiry** — no reaper (C-3); on the next lease walk an `expires_at < now()` session has
  its handle reclaimed; an expired operator token fails `Authorize` (`auth_pg.go:90-91,
  142-143`).
- **run → operator_handles join** — `run.prepare` reads `app.session_id` (=
  `operator_session_id`) → the `created_by_handle_id` subquery (§1(2)).

### Falsifiable assertions (carried forward)

- **A6 (live-unique forces distinct words).** Two concurrent same-human operator sessions hold
  two distinct live words. *Refuting test:* `operator_session_pre_run_stamp` — a duplicate live
  `maya` or deadlock refutes.
- **A7 (distinct `whose`).** `whose RA != whose RB`. *Refuting test:* §2.5 asserts
  `maya#7f3` vs `theo#7f3`; equal answers refute (gate-critical).
- **A8 (deterministic default, reconnect-stable).** *Refuting test:* lease → close →
  re-bootstrap for `P`; a different word refutes.
- **A9 (deterministic escalation, reconnect-stable).** *Refuting test:* hold `candidates[0]`;
  reconnect a second session; a non-`candidates[1]` word refutes.
- **A10 (no silent relabel).** *Refuting test:* create `RB` under `theo`; close `S1`; reconnect
  `S2` (converges live to `maya`); `whose RB` must still render `theo#7f3`.
- **A11 (one winner, no deadlock).** *Refuting test:* concurrent two-session lease;
  `40P01`/duplicate/both-fail refutes.
- **A27 (operator session buildable pre-run).** An operator session + lease can be created with
  no `sessions` row and no run, and its token sets `app.session_id` at `run.prepare`. *Refuting
  test:* `operator_session_pre_run_stamp` — NON-NULL DISTINCT `created_by_handle_id`. (A27 is
  the *storage/build* claim; A29 is the *authorization* claim that makes it executable.)

---

## §3 — R1c: lease flap (carried forward, unchanged)

Heartbeat is a **guarded UPDATE** that never deletes, sets `released_at`, or re-INSERTs:

```sql
UPDATE striatumd.operator_handles
   SET leased_until = now() + $TTL, last_heartbeat_at = now()
 WHERE handle_id = $1 AND leased_session_id = $2 AND released_at IS NULL;
```

and, in the same operator-session heartbeat transaction, the `operator_sessions` + operator
token `expires_at` renewal. The guard means only the owning, still-live session renews, and the
row never transits through a released state mid-renewal — so `operator_handles_live_uq` never
frees the word mid-flap. Mirrors the `striatumd.leases` idiom (`0005:166-178`).

- **A12 (flap-resistance).** *Refuting test:* `lease_flap_steal` (two-role) — interleave `S1`'s
  renewal with `S2`'s attempt to lease `maya`; `S2` must always get `23505` and escalate, and
  `S1`'s row was never `released_at`-set.

---

## §4 — R2: the owner-bundle migration (carried forward; §4.2 grant + §4.5 tests revised)

### 4.1 Ordinal and placement (unchanged)

Owner bundle **0021** (next free after `LatestOwnerBundleVersion == 20`, `owner.go:23`). New
file `go/pkg/db/sql/owner/0021_operator_identity_run_attribution.sql`, auto-discovered
(`owner.go:157` `//go:embed sql/owner/*.sql`), plus label `21: "RFC 0167 P0 operator identity +
run attribution"` and `LatestOwnerBundleVersion = 21`. Owner bundle because the two new tables
+ `ALTER runs` touch owner-held tables (C-1).

### 4.2 Bundle SQL (additive — no privilege-stripping REVOKE; operator_sessions grant narrowed, C2′)

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

-- (4) write-once at the DB (BEFORE UPDATE trigger, §4.3) — unchanged from v2.
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

-- (5) runtime-role DML on the new owner-held tables — operator_sessions SELECT is COLUMN-SCOPED (C2′).
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    -- operator_handles has no client_id column, so full-table SELECT exposes no identity MAP:
    GRANT SELECT, INSERT, UPDATE ON striatumd.operator_handles TO striatumd_rw;  -- no DELETE
    -- operator_sessions pairs client_id + principal_id, so SELECT is column-scoped to EXCLUDE both (C2′):
    GRANT SELECT (operator_session_id, repository_id, state,
                  registered_at, last_heartbeat_at, expires_at, closed_at, close_reason)
      ON striatumd.operator_sessions TO striatumd_rw;
    GRANT INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw;          -- no DELETE
  END IF;
END $$;

-- (6) watermark/capability stamp (mirrors every bundle, e.g. 0018:108-110).
INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('operator_identity_run_attribution', false, 21)
ON CONFLICT (capability) DO NOTHING;
```

No `EXECUTE` grant on `resolve_principal_for_client` is added (it exists from 0006:186; C2
reuses it). **No `SELECT(principal_id)` grant-back on `principal_clients`** (0006's closure
stays closed), and **no `SELECT` on `operator_sessions.{principal_id,client_id}`** (C2′). No
`REVOKE` anywhere (A19).

### 4.3 Write-once enforcement (carried forward, unchanged)

A `BEFORE UPDATE` trigger that raises when either origin column changes — chosen over a column
REVOKE because `runs` is actively UPDATEd on ~six paths (including `branch_confirmed_by` after
creation), with direct precedent `0010_artifact_blob_update_trigger.sql:19-49`. The stamp
happens at INSERT (no `BEFORE UPDATE` fire); `IS DISTINCT FROM` also forbids a later
`NULL → value` UPDATE (consistent with OQ2 no-backfill).

### 4.4 Privileges the runtime role must RETAIN (pinned; operator_sessions SELECT narrowed)

- **`runs`** — `SELECT, INSERT, UPDATE, DELETE` unchanged (`0005:467-475`); the trigger blocks
  only the two origin columns.
- **`operator_handles`** — `SELECT` (render/join), `INSERT` (lease), `UPDATE` (heartbeat +
  release + lazy reclaim). **No `DELETE`** (snapshots never dangle). Full-table SELECT is safe
  (no `client_id` column → no identity map).
- **`operator_sessions`** — **column-scoped `SELECT`** on
  `{operator_session_id, repository_id, state, registered_at, last_heartbeat_at, expires_at,
  closed_at, close_reason}` (manifest/liveness), `INSERT` (create), `UPDATE` (heartbeat/close).
  **No `SELECT(principal_id)`, no `SELECT(client_id)`** (C2′). **No `DELETE`**.
- **`resolve_principal_for_client`** — `EXECUTE` already granted (`owner/0006:186`); the
  run-origin resolution rides it. **`SELECT(principal_id)` on `principal_clients` stays
  REVOKED.**

### 4.5 Proof under the RFC 0142 two-role fixture; named pgtests (C1′/C2′ revised)

All DB-boundary tests use `pgtest.TwoRole(t)` (`two_role.go:47-78`): bundle DDL via `OwnerPool`,
runtime behavior via `SUTPool` (`striatumd_rw`), daemon authority bootstrapped for the SUT so
the projection and the **real `run.prepare` authorization** paths are exercised.
`assertSQLState42501` (`two_role_pg_test.go:161-176`) is the privilege oracle.

1. **`owner_bundle_0021_applies_clean`** — apply 0021 via `OwnerPool`; assert the two tables +
   indexes + `runs` columns + trigger exist; then as `SUTPool` assert the runtime role **can**
   INSERT a run carrying `created_by_principal_id`, INSERT/UPDATE `operator_handles`, INSERT
   (all columns) + UPDATE (liveness) `operator_sessions`, SELECT the *granted* operator-session
   columns, **can** `EXECUTE resolve_principal_for_client` (daemon auth), and **cannot** `ALTER
   TABLE runs` (`42501`), `DELETE` from the new tables, **nor** `SELECT client_id, principal_id
   FROM operator_sessions` (the **C2′ negative control**, `42501` — A33). Proves the runtime
   retains exactly its needed privileges and no more.
2. **`run_origin_stamp_uses_identity_projection`** (C2 gate, INTACT) — as `SUTPool`: (a) a
   direct `SELECT pc.principal_id FROM principal_clients` fails `42501` (control); (b) the real
   stamp (`admin.ResolvePrincipalForClient` → projection → bound `$N`) succeeds and stores the
   linked principal `P`; (c) a forged envelope `created_by_principal_id = P_other` (+ spoofs)
   still stores `P`.
3. **`operator_session_pre_run_stamp`** (C1/C1′ gate, RE-SPECIFIED) — as `SUTPool` with daemon
   authority: create two pre-run operator sessions for one human `P` (no `sessions` row, no
   run), **mint each via the operator-token path** (`{admin, read}`, session-bound); lease two
   distinct words; create one run per session by calling the **real `run.prepare` RPC** with
   each operator token; assert `run.prepare` is **authorized** (not `capability_missing`), and
   two NON-NULL DISTINCT `created_by_handle_id` with `whose RA != whose RB` (`maya#7f3` vs
   `theo#7f3`) — **A29, A7, A27**. PLUS controls:
   - **(i) lane token denied** — an ordinary lane session token (slice `{claim,write,read,
     review}`) calling `run.prepare` is denied `capability_missing`; assert no admin grant
     exists on it — **A30**.
   - **(ii) closed/expired denied** — after graceful close (token revoked + handle released),
     and separately after TTL expiry, the operator token calling `run.prepare` is denied
     (`capability_missing` / `token_expired` / `capability_expired`) and creates no run / no
     NULL stamp — **A31**.
   A `capability_missing` on the *valid* operator token (a), a NULL/equal `created_by_handle_id`,
   a lane token that prepares a run (i), or a stamp via a closed/expired token (ii) refutes the
   C1′ discharge.
4. **`forged_update_created_by_rejected`** — `UPDATE runs SET created_by_principal_id` on a
   stamped run raises the trigger (plpgsql `RAISE`, SQLSTATE `P0001`) — A14.
5. **`two_live_maya`** — the §2.3 collision/escalation invariants (A6, A11) in isolation.
6. **`token_revoked_bare_id`** — revoke the creating client + close its operator session
   (dedicated close path, token revoked + lease released); assert `status --mine` falls back to
   the **bare id** while `whose <past-run>` still renders the **frozen** historical
   `word#suffix`.
7. **`lease_flap_steal`** — A12 (§3).

### 4.6 Forward-only and watermark consistency (carried forward, unchanged)

- **Forward-only.** `applyPendingOwnerBundles` applies only bundles `> current`
  (`owner.go:305-322`); each commits its watermark in the same transaction (`owner.go:528-532`).
  0021's two tables + `runs` ALTER + trigger + grants live in one transaction — no intra-bundle
  ordering hazard (no REVOKE).
- **Advance `RequiredOwnerBundleVersion` to 21.** The serving binary's `run.prepare` references
  the new columns/tables; `CheckOwnerBundleWatermark` (`owner.go:124-154`, before
  `ApplyMigrations` at `connection.go:349-351`) halts cleanly (`AwaitingOwnerDDLError`) if the
  daemon restarts before 0021 applied.
- **Deploy ordering (apply THEN restart).** `striatum daemon owner-ddl apply --owner-url …`
  (`daemon.go:84-159`) applies 0021 as owner; then restart. Restart-first → `20 < required 21` →
  clean halt with remediation.

### Falsifiable assertions

- **A13 (owner-only ALTER).** Runtime `ALTER runs` fails `42501`; owner succeeds. *Test:*
  `owner_bundle_0021_applies_clean`.
- **A14 (write-once at the DB).** *Test:* `forged_update_created_by_rejected`.
- **A15 (retained privileges exact).** Runtime can do everything P0 needs (INSERT run w/ column;
  lease+heartbeat+release on `operator_handles`; INSERT/UPDATE + column-SELECT
  `operator_sessions`; EXECUTE the projection) and nothing more (no `ALTER`, no `DELETE`, no
  `SELECT(principal_id)` on `principal_clients` **or** `operator_sessions`). *Test:* the
  positive+negative assertions in `owner_bundle_0021_applies_clean` (incl. A33).
- **A16 (clean apply, non-superuser owner).** *Test:* the apply step; any `42501`/`must be
  member`/schema-perm during apply refutes.
- **A17 (forward-only).** Re-applying 0021 is a no-op. *Test:* apply twice.
- **A18 (watermark interlock).** A binary built against 21 refuses to serve on a 20-watermark DB
  (`AwaitingOwnerDDLError`). *Test:* boot against a 20-watermark DB.
- **A19 (no revoke-last hazard).** 0021 carries no privilege-stripping REVOKE (the
  operator_sessions grant is column-scoped from creation). *Test:* grep 0021 for `REVOKE`.
- **A28 (two-role stamp safety via the projection).** The run-origin stamp PASSES the two-role
  fixture (no `42501`) and routes through `resolve_principal_for_client`. *Test:*
  `run_origin_stamp_uses_identity_projection` (a,b).

---

## §5 — R3: resolve all four open questions (carried forward, DISCHARGED)

- **5.1 OQ1 — pool/default/escalation/denylist → IN P0.** A curated lowercase first-names pool
  (~256 neutral names), a Go slice. Default = `POOL[fnv64a(principal_id) mod len(POOL)]`
  (deterministic, reconnect-stable, not tty). Escalation = principal-seeded walk to the next
  distinct curated word (not numeric suffixes). Denylist = reserved words (`daemon`,
  `scheduler`, `system`, `admin`, `root`, `unknown`, `anon`, `none`, the `principal_kind`
  names) excluded; service principals draw a disjoint reserved sub-pool. **A20.**
- **5.2 OQ2 — backfill vs NULL → NULL + advisory `attribution_unknown`, in P0.** No backfill
  (`branch_confirmed_by` carries no `principal_id`, C-4). **A21.**
- **5.3 OQ3 — cross-repo board → per-repo only in P0; daemon-wide DEFERRED (P3).** Both tables
  keyed `(repository_id, …)`. **A22.**
- **5.4 OQ4 — `@handle#suffix` artifact byline → OUT of P0 (P2).** P0 changes no artifact
  `author_line`/anchor metadata. **A23.**

---

## §6 — R4: ride RFC 0107; do not rebuild it (carried forward; operator mint is reuse)

- **Operator-id IS `principal_id`.** `operator_handles.principal_id` / `operator_sessions.
  principal_id` are FKs to `striatumd.principals` (`0023_principals.sql:30-36`); neither table
  stores identity attributes.
- **Reuse, don't duplicate.** Client→principal dereference reuses the
  `resolve_principal_for_client` projection (C2). The handle lease reuses the
  `striatumd.leases` TTL idiom. The operator token reuses the **existing mint shapes** — the
  caller-supplied capability set of `admin.insertTokenClient` (`tokens.go:286-315`) + the
  session binding of `mintSessionBoundToken` (`session_token.go:60-97`) — so C1′ adds *no* new
  authorization machinery, only a new capability *set* on the same path. Release is the
  dedicated operator-session close (+ token revoke) + lazy `leased_until` expiry; **no new
  reaper** (C-3). The `operator_sessions` table is a liveness anchor, not an identity store.
- **Product-boundary clean.** No hosted service/directory/telemetry/external identity; tty/tmux/
  title/env never read for state (A2/A5); `run_id` stays opaque.

- **A24 (no parallel identity).** *Test:* assert the new tables have no `display_name`/`kind`/
  auth columns; identity read only from `principals`.
- **A25 (no new reaper).** *Test:* release via the dedicated close + lazy acquisition-path
  expiry; no new background goroutine/scheduler.
- **A26 (opaque run_id).** *Test:* `run_id` generation unchanged, no handle encoded.

---

## §7 — P0 boundary and seams for P1–P3 (carried forward)

P0 ships: `operator_handles` + live-unique index (§2.2); `operator_sessions` + lifecycle
(§2.6); `runs.created_by_principal_id` + `runs.created_by_handle_id` write-once (§4); the
operator-bootstrap mint+lease RPC riding `mintOperatorSessionToken` + `link_client_to_principal`
(§1, §C1′); the projection-routed run-origin stamp (§1(2), C2); `whose <run-id>` (new read RPC,
registered in `contracts/daemon_methods.json` + routes +
`docs/reference/command-authority-matrix.md` + `registry_contract_test`); `status --mine`
manifest; the `attribution_unknown` advisory doctor rule (§5.2). Seams NOT designed: **P1**
custody (`run_custody_log`); **P2** honest bylines + handoff naming + chips + OSC title;
**P3** lineage + cross-repo board.

---

## §8 — Consolidated falsifiable-assertion ledger

| # | Claim | Supporting evidence (anchor) | Refuting observation / named test |
|---|-------|------------------------------|-----------------------------------|
| A1 | Stamp = live-token principal, server-side, via projection | `run.go:1056-1074` INSERT (bound `$N`); `authority.go:116-120` | forged param / spoof leaks — `run_origin_stamp_uses_identity_projection` |
| A2 | No client-name path to attribution | `run.go:21-28`; operator-mint handler | grep finds `created_by`/handle param feeding stamp/lease |
| A3 | Mint+link+lease+operator-session atomic | §1(1); `session_token.go:48-53`; `owner/0006:160-188` | token committed without link+lease |
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
| A15 | Retained privileges exact | §4.4; `0005:467-475`; `owner/0006:186` | needed op `42501` / surplus grant (incl. A33) |
| A16 | Clean apply, non-superuser owner | two-role `OwnerPool` | `42501`/`must be member`/schema-perm on apply |
| A17 | Forward-only | `owner.go:305-322,528-532` | second-apply error / watermark regression |
| A18 | Watermark interlock | `owner.go:124-154`; `connection.go:349-351` | serves on 20-watermark DB |
| A19 | No revoke-last hazard | §4.2 / §C2′ (no REVOKE; column grant from creation) | `REVOKE` present in 0021 |
| A20 | Pool/default/escalation/denylist | §5.1 golden test | unstable default / denied word |
| A21 | NULL + advisory, no backfill | §5.2; C-4 | red classification / backfill write |
| A22 | Per-repo only in P0 | §5.3 | cross-repo aggregation in P0 |
| A23 | Byline suffix out of P0 | §5.4 | `author_line` change in P0 |
| A24 | No parallel identity table | §6; `0023_principals.sql:30-36` | identity attribute on the new tables |
| A25 | No new reaper | §6; dedicated close path | new periodic session reaper |
| A26 | Opaque run_id | §6 | handle encoded into `run_id` |
| A27 | Operator session **buildable** pre-run (C1) | §2.6; `auth_pg.go:104-156`; `authority.go:79,119` | NULL `created_by_handle_id` / can't create without a run — `operator_session_pre_run_stamp` |
| A28 | Two-role stamp safety via the projection (C2) | §1(2); `owner/0006:56-79,181,186`; `principals.go:53-63,266-292` | real stamp `42501` / direct `pc.principal_id` read — `run_origin_stamp_uses_identity_projection` |
| **A29** | **Operator token AUTHORIZES `run.prepare` (C1′)** | §C1′.2; `registry_methods.go:110`; `auth_pg.go:104-117`; `authority.go:79,119` | `capability_missing` on the valid operator token / NULL stamp — `operator_session_pre_run_stamp` |
| **A30** | **Lane tokens do NOT gain admin (C1′ control)** | `session_token.go:46,77-89`; exact-match `auth_pg.go:104-140` | lane token prepares a run / lane token has an admin row — control (i) |
| **A31** | **Closed/expired operator session cannot stamp (C1′ control)** | §C1′.3; `auth_pg.go:87-92,111,140-143` | stamp (or NULL stamp) via a closed/expired operator token — control (ii) |
| **A32** | **Distinct operator mint, not the lane slice (C1′)** | §C1′.2; `tokens.go:286-315`; `session_token.go:60-97` | `admin` appended to `sessionBoundCapabilities` / lane token carries admin |
| **A33** | **operator_sessions identity map unreadable by runtime (C2′)** | §C2′.1; `0006:218-220`; `owner.go:464-467` | `SELECT client_id, principal_id FROM operator_sessions` succeeds — `owner_bundle_0021_applies_clean` negative control |
| **A34** | **Narrow grants still suffice (C2′ positive control)** | §C2′.1; §4.4 | `42501` on create/heartbeat/close/stamp/whose/status --mine |

---

## §9 — Build manifest (P0 scope, for the downstream `code_change` run)

1. **Owner bundle** — `go/pkg/db/sql/owner/0021_operator_identity_run_attribution.sql` (§4.2:
   `operator_handles` + `operator_sessions` + `runs` columns + trigger + grants — operator_
   sessions SELECT **column-scoped, C2′** + watermark); `owner.go` label entry +
   `LatestOwnerBundleVersion = 21` + `RequiredOwnerBundleVersion = 21` (§4.6). Optionally add
   the operator_sessions column grant to `ReassertReadRevokes` (`owner.go:459-468`) for drift
   parity (§C2′.1).
2. **Lease + operator-session layer** — a Go package owning `defaultHandle`/escalation walk
   (§2.3, §5.1 pool), lease acquisition + guarded heartbeat renewal (§3), the `operator_sessions`
   create/heartbeat/close lifecycle (§2.6) with graceful release via the dedicated close path
   (NOT `closeRemainingSessions`) **that also revokes the operator token** (§C1′.3).
3. **Operator-token mint** — `mintOperatorSessionToken` (sibling to `mintSessionBoundToken`)
   inserting `operatorSessionCapabilities = {admin, read}`, each `client_capabilities` row
   session-bound to the `operator_session_id` (§C1′.2). The shared `sessionBoundCapabilities`
   slice is **unchanged**.
4. **Operator-bootstrap mint RPC** — a daemon-side mint+lease entry reusing
   `mintOperatorSessionToken` + `link_client_to_principal`, creating the operator session +
   admin-bearing token the CLI presents on `run.prepare` so `app.session_id` is populated (§1(1),
   §C1′). `striatum operator bootstrap` becomes its client.
5. **Run stamp** — extend the `runs` INSERT (`run.go:1056-1074`): resolve the principal in Go via
   `admin.ResolvePrincipalForClient` and bind it; `created_by_handle_id` as the `operator_handles`
   subquery keyed on `app.session_id` (§1(2), C2). Delete v1's direct `principal_clients`
   subquery.
6. **`whose <run-id>`** — new read handler (§2.4 join) + `contracts/daemon_methods.json` +
   regenerated routes + `docs/reference/command-authority-matrix.md` row +
   `registry_contract_test` (CapabilityRead; the operator token's `read` row authorizes it).
7. **`status --mine`** — manifest section + flag, live-handle render with bare-id fallback
   (§4.5 test 6).
8. **Doctor** — `attribution_unknown` advisory rule (§5.2), following `doctor_artifact_anchor.go`.
9. **pgtests** — the seven named two-role tests (§4.5), all on `pgtest.TwoRole`, with the C1′
   gate (`operator_session_pre_run_stamp` driving the **real `run.prepare` authorization path**
   + controls (i)/(ii)) and the C2′ controls (the negative + positive operator_sessions reads in
   `owner_bundle_0021_applies_clean`).
10. **Docs** — update `docs/decisions/decision-log.md`, `docs/reference/spec.md`,
    `docs/reference/command-authority-matrix.md`, `CHANGELOG.md`, and re-triage
    `docs/operator/rfc-roadmap.md` when P0 ships.

This is the published claim. Gate-critical targets: **A29/A7/A27** (R1b sufficiency PROVEN
through the real `run.prepare` authorization path — C1′), **A30/A31** (the lane-non-admin and
closed/expired controls — C1′), and **A33/A34/A19** (the operator_sessions read-scope closure +
positive control + no-REVOKE — C2′). The §0 corrections, the §D discharge map, and the §C1′/§C2′
mechanisms are load-bearing; challenge them at source if you believe any is wrong.
