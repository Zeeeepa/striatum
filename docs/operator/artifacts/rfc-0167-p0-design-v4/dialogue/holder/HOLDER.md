# HOLDER — RFC 0167 P0 falsifiable implementation SPEC, REVISION v4 (CONSOLIDATED, operator identity & run attribution)

author: holder-author-001

> This is the **v4 revision** and **consolidated whole-P0 SPEC** of the RFC 0167 P0
> implementation, published as the claim the two falsifiers re-attack. The v3 falsification
> gate (`rfc-0167-p0-design-v3`) RESOLVED the cores of both v2 constraints (a distinct
> `mintOperatorSessionToken` authorizes `run.prepare`; the *direct* `operator_sessions`
> identity read is closed) but, its single revision cycle exhausted, routed two NEW,
> source-verified, critical residuals to the operator:
> **C2″** — `striatumd_rw` can still reconstruct `client_id → principal_id` by a COMPOSED route
> over the ACL graph (the v3 falsifier named ONE route; the operator identified a SECOND,
> deeper one); and **C1″** — the `{admin,read}` operator-session token carries the whole
> coarse `CapabilityAdmin` surface, broader than run-lifecycle alone. The operator analyzed
> both and prescribed the fixes in the v4 SEED. This SPEC **discharges both** and
> **carries forward, UNREGRESSED, everything cleared through v1+v2+v3**. It is the SINGLE
> source of truth the downstream `rfc-0167-p0-build` `code_change` run consumes, so it is the
> WHOLE consolidated P0 SPEC (not just deltas). Every load-bearing claim is a **falsifiable
> assertion** paired with the **named test/check that would refute it**, anchored to
> **verified current-branch source** (`go/pkg/...` `file:line`). I re-read the source on the
> `striatum/rfc-0167-p0-design-v4` worktree; §E records the v3-constraint discharge map,
> §C2″/§C1″ record the new mechanisms + verifications, §0–§9 are the consolidated whole-P0
> spec. Scope is **P0 only**.

---

## §E — Addressing the v3 constraints (the auditable revision map)

This section exists so the falsifiers can verify the discharge directly. C2″ (the decisive,
non-negotiable blocker) and C1″ are each RESOLVED with a named, source-anchored mechanism and
named two-role pgtests/controls; every v1+v2+v3 carry-forward is INTACT.

| v3 residual | Disposition in v4 | Mechanism (§) | Gate test |
|---|---|---|---|
| **C2″** composed `client_id → principal_id` over the full ACL graph (Route 1 `cc ⋈ oh`; Route 2 `cc ⋈ oh ⋈ runs`) | **RESOLVED** — both routes closed at the column layer; identity reads routed through SECURITY DEFINER projections; composed negative control added | §C2″ | `composed_identity_map_unreadable` (NEG), `owner_bundle_applies_clean` (NEG+POS), `whose_status_mine_via_projection` (POS) |
| **C1″** operator-token admin over-grant | **RESOLVED via JUSTIFIED-ACCEPTANCE** (RFC-ledger option (c)) hardened by the confirmed `verifier.attest` session-bound fence (option (b)); N-tokens blast-radius + repo-scope bound documented | §C1″ | `operator_token_admin_surface` (the operator token runs `run.prepare` + representative operator-admin routes; `verifier.attest` refused typed; lane denied; closed/expired denied) |

**The one honest carry-forward DELTA the falsifiers must see (not an accidental regression).**
v3's **A19** asserted "0021 carries no privilege-stripping REVOKE." **v4 necessarily breaks
that literal claim**: closing C2″ Route 2 REQUIRES a `REVOKE SELECT ON striatumd.runs` followed
by a column-scoped re-GRANT (the exact RFC 0114 / bundle 0006 `principal_clients` precedent).
A19 is therefore **REVISED, not regressed** (§C2″.5): the safety it protected is preserved by a
*different, stronger* mechanism — `runs` is owner-held (C-1) so the REVOKE is an irreversible
boundary the runtime cannot self-undo, the bundle is atomic-per-version + forward-only +
idempotent, and the column gate is registered in `readScopeReasserts` so drift repair re-closes
it. This is deliberate and load-bearing; it is called out here so the carry-forward lens reads
it as the prescribed fix, not a slip.

---

## §C2″ — The COMPOSED read-scope discharge (THE decisive blocker; full mechanism)

### C2″.1 The two composed routes, verified at source

Both join edges are runtime-readable today because migration `0005` grants the runtime role
`SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd` and revokes back only
`UPDATE, DELETE` on `events`/`artifacts`
(`0005_repo_local_workflow_state.sql:470-472`, VERIFIED). A *table-level* `SELECT` grant
covers columns added later, so the 0021 `runs.created_by_principal_id` column inherits runtime
`SELECT` unless explicitly revoked. `client_capabilities` carries `client_id` + `session_id`
and is **not** closed by bundle 0006 (`0006` closes only `client_sessions`/`principals`/
`principal_clients`, lines 195-221, VERIFIED). The v3 design binds
`client_capabilities.session_id = operator_session_id = operator_handles.leased_session_id`
by construction, so both join edges exist:

- **Route 1 (v3 falsifier-named):**
  `SELECT DISTINCT cc.client_id, oh.principal_id
     FROM striatumd.client_capabilities cc
     JOIN striatumd.operator_handles oh ON oh.leased_session_id = cc.session_id`
  → reads `oh.principal_id` (v3 granted `operator_handles` FULL SELECT). Yields
  `(client_id, principal_id)` — the mapping bundle 0006 closed.
- **Route 2 (operator-identified, deeper):**
  `SELECT DISTINCT cc.client_id, r.created_by_principal_id
     FROM striatumd.client_capabilities cc
     JOIN striatumd.operator_handles oh ON oh.leased_session_id = cc.session_id
     JOIN striatumd.runs r ON r.created_by_handle_id = oh.handle_id`
  → reads `r.created_by_principal_id` (runs is runtime-readable). Yields the SAME mapping even
  if `oh.principal_id` is closed. `oh.leased_session_id` and `oh.handle_id` **cannot** be
  revoked — the lease heartbeat (`WHERE leased_session_id = $`) and the stamp subquery
  (`SELECT handle_id ... WHERE leased_session_id = app.session_id`) read them — so the **only**
  column that closes Route 2 is `runs.created_by_principal_id`.

### C2″.2 The prescribed fix (the established RFC 0114 projection + column-gate pattern)

**(a) Column-scope `operator_handles` SELECT to EXCLUDE `principal_id` (closes Route 1).**
`operator_handles` is NEW in 0021, so the runtime never had a table-wide grant on it — this is a
column-scoped GRANT *from creation* (no REVOKE). Grant exactly the columns the lease walk /
lazy-expiry / heartbeat / stamp subquery read:

```sql
-- operator_handles is NEW in 0021: column-scoped GRANT from creation, EXCLUDING principal_id.
GRANT SELECT (handle_id, leased_session_id, handle, repository_id,
              released_at, leased_until, last_heartbeat_at)
  ON striatumd.operator_handles TO striatumd_rw;            -- principal_id NOT granted
GRANT INSERT, UPDATE ON striatumd.operator_handles TO striatumd_rw;   -- no DELETE (snapshots never dangle)
```

`principal_id` stays a column (FK to `principals`, written at lease INSERT from a Go-held value,
read by the SECURITY DEFINER projections) but is **never SELECTable directly by `striatumd_rw`**.

**(b) Column-revoke `runs.created_by_principal_id` SELECT (closes Route 2).** `runs` already
exists with a table-level runtime `SELECT` (0005), so this is a `REVOKE` + column re-GRANT
mirroring bundle 0006's `principal_clients` gate (`0006:218-220`):

```sql
-- runs EXISTS with table-level runtime SELECT (0005:470). Revoke, then re-grant every column
-- EXCEPT created_by_principal_id. INSERT(created_by_principal_id) is KEPT (the stamp writes it;
-- INSERT(col) is independent of SELECT(col)). The BEFORE UPDATE write-once trigger reads OLD/NEW
-- row variables (no column SELECT privilege needed), so it is unaffected.
REVOKE SELECT ON striatumd.runs FROM striatumd_rw;
GRANT SELECT (
  repository_id, run_id, workflow_snapshot_id, repo_root, state,
  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at,
  started_at, completed_at, stop_reason, paused_at, paused_reason, cross_repo_run_id,
  created_by_handle_id                                     -- the handle FK stays selectable (no identity map alone)
  -- created_by_principal_id deliberately OMITTED
) ON striatumd.runs TO striatumd_rw;
-- runs INSERT/UPDATE/DELETE table-level DML unchanged (0005:470); the stamp's INSERT(created_by_principal_id) rides table INSERT.
```

> **Build-time enumeration, not a hard-coded list.** The runs column set above is the live
> baseline (0005) plus 0021's `created_by_handle_id`. The build run MUST regenerate the
> `GRANT SELECT (...)` from `information_schema.columns` for `striatumd.runs` at 0021-authorship
> time, MINUS `created_by_principal_id`, so a column added by a concurrently-landing runtime
> migration is not silently stranded out of the grant. (A19 revision, §C2″.5.)

**Why the `runs` REVOKE is a real boundary (and needs no ownership transfer, unlike 0006).**
Bundle 0006 had to `ALTER TABLE principal_clients OWNER TO CURRENT_USER` *before* its REVOKE,
because `principal_clients` was runtime-OWNED and "a plain REVOKE SELECT is NOT a boundary — the
owning role can re-grant itself SELECT in one statement" (`0006:10-17,190-197`, VERIFIED). `runs`
is **already owner-held** on a two-role deploy (CORRECTION C-1, §0: not in bundle 0018's transfer
cohort; runtime `ALTER runs` dies `42501 must be owner of table runs`). So `striatumd_rw` cannot
re-grant itself the missing column — the REVOKE is immediately an irreversible boundary, with
**no ownership transfer step required**.

**(c) Route identity-bearing reads through SECURITY DEFINER projections** (owned by the owner
role, `EXECUTE` to `striatumd_rw`, daemon-secret-gated by `assert_daemon_authority()`, mirroring
`resolve_principal_for_client`, `0006:56-79,181,186`). Three new projections in 0021, each
reading `principal_id` / `created_by_principal_id` AS THE DEFINER (never directly by
`striatumd_rw`):

```sql
-- whose <run-id> projection: returns the origin identity for one run.
CREATE OR REPLACE FUNCTION striatumd.run_origin_identity(p_daemon_secret text, p_repository_id text, p_run_id text)
RETURNS TABLE (run_id text, state text, created_by_principal_id text, origin_handle text,
               principal_kind text, disabled_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = striatumd, public, pg_temp AS $$
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  RETURN QUERY
    SELECT r.run_id, r.state, r.created_by_principal_id, oh.handle, p.principal_kind, p.disabled_at
      FROM striatumd.runs r
      LEFT JOIN striatumd.operator_handles oh ON oh.handle_id = r.created_by_handle_id
      LEFT JOIN striatumd.principals       p  ON p.principal_id = r.created_by_principal_id
     WHERE r.repository_id = p_repository_id AND r.run_id = p_run_id;
END $$;

-- status --mine projection: run-ids whose origin principal = the live caller's principal.
CREATE OR REPLACE FUNCTION striatumd.runs_for_origin_client(p_daemon_secret text, p_repository_id text, p_client_id text)
RETURNS TABLE (run_id text, state text, created_by_handle_id text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = striatumd, public, pg_temp AS $$
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  RETURN QUERY
    SELECT r.run_id, r.state, r.created_by_handle_id
      FROM striatumd.runs r
      JOIN striatumd.principal_clients pc ON pc.principal_id = r.created_by_principal_id
     WHERE r.repository_id = p_repository_id AND pc.client_id = p_client_id AND pc.unlinked_at IS NULL;
END $$;
-- (doctor attribution_unknown reuses run_origin_identity / a NULL-origin scan projection.)

REVOKE ALL ON FUNCTION striatumd.run_origin_identity(text,text,text)      FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.runs_for_origin_client(text,text,text)   FROM PUBLIC;
GRANT EXECUTE ON FUNCTION striatumd.run_origin_identity(text,text,text)    TO striatumd_rw;
GRANT EXECUTE ON FUNCTION striatumd.runs_for_origin_client(text,text,text) TO striatumd_rw;
```

The projections require `p_daemon_secret` and call `assert_daemon_authority()`, so a leaked
runtime credential ALONE (no daemon secret) cannot invoke them to map `client → principal` — the
exact gate the C2 run-origin stamp already relies on.

**(d) Convert the three `runs` star-readers to explicit column lists (closes the 42501 trap the
column-revoke would otherwise spring).** A `SELECT *` / `SELECT r.*` on `runs` requires `SELECT`
on **every** column, so after (b) it fails `42501` for `striatumd_rw`. Enumerated readers
(§C2″.3) — the build MUST convert these to explicit column lists excluding
`created_by_principal_id`:

| Reader | File:line | Fix |
|---|---|---|
| `run.detail` | `reads/detail.go:73` | `SELECT *` → explicit run columns (the page never needs origin principal; surface origin via `run_origin_identity` if/when shown) |
| `job.detail` | `reads/detail.go:174` | `SELECT *` → explicit run columns (same) |
| `archive`/`evidence` export | `reads/archive.go:118` | `SELECT r.*` → explicit `r.<col>` list (export carries no identity principal in P0) |

### C2″.3 Enumeration: every reader of `runs.created_by_principal_id` (proving the revoke breaks no other path)

The column is **NEW** in 0021, so no pre-existing query names it. The only paths that read it
are the ones P0 introduces, and **all** route through the DEFINER projections, never a direct
`striatumd_rw` SELECT:

- `whose <run-id>` → `run_origin_identity` projection (DEFINER reads the column).
- `status --mine` → `runs_for_origin_client` projection (DEFINER reads the column).
- `doctor attribution_unknown` → a DEFINER NULL-origin scan (or reuses `run_origin_identity`).

Every OTHER runs reader selects explicit non-identity columns and is **unaffected** by the
revoke (verified: `status.go`, `dashboard_all.go`, `run_graph.go`, `concurrent_runs.go`,
`metrics/collector.go`, `doctor*.go`, `run.go`, `interrogation.go`, `workflow_*` all use
`FROM striatumd.runs r` with explicit `r.<col>` projections — no `r.*`). The ONLY star-readers
are the three in §C2″.2(d), converted there. **Net: no daemon read path besides
whose/status-mine/doctor-attribution reads `created_by_principal_id`, and those go through the
DEFINER.**

### C2″.4 Third-route analysis over the full ACL graph (the falsifier's obligation, pre-discharged)

Every table column that exposes a `principal_id` to the runtime role is closed; the projections
are daemon-secret-gated. Enumerated:

| Surface exposing `principal_id` | Closure | Where |
|---|---|---|
| `principals.principal_id` | `REVOKE SELECT ON principals` (full) | `0006:215`, `readScopeReasserts` |
| `principal_clients.principal_id` | column gate (`GRANT SELECT (client_id, linked_at, unlinked_at)`) | `0006:218-220`, `readScopeReasserts` |
| `operator_sessions.principal_id` (+ `client_id`) | column-scoped GRANT excluding both (v3 C2′, carried) | §2.6, §C2″.5 |
| `operator_handles.principal_id` | column-scoped GRANT excluding it (NEW — C2″ Route 1) | §C2″.2(a) |
| `runs.created_by_principal_id` | `REVOKE` + column re-GRANT excluding it (NEW — C2″ Route 2) | §C2″.2(b) |
| DEFINER projections returning `principal_id` (`resolve_principal_for_client`, `run_origin_identity`, `runs_for_origin_client`, `get_principal`) | `assert_daemon_authority()` secret gate; runtime can `EXECUTE` only WITH the daemon secret | `0006:46-47,72`, §C2″.2(c) |

P1's `run_custody_log.principal_id` is **out of P0** (not built), so it cannot be a third route.
No remaining runtime-readable column pairs a credential identifier with a `principal_id`.

### C2″.5 A19 REVISED (the honest carry-forward delta), and the drift-repair registration

v3 A19 ("0021 carries no privilege-stripping REVOKE") is **no longer literally true** — C2″
Route 2 requires the `runs` REVOKE. The invariant A19 actually protected (a column gate cannot be
silently re-opened, and a half-applied REVOKE cannot strand the surface) is preserved by a
stronger mechanism:

1. **Owner-held boundary (no self-re-grant).** `runs` is owner-held (C-1), so unlike a
   runtime-owned table the REVOKE is irreversible by `striatumd_rw` — no ownership transfer
   needed (0006 needed one; we do not).
2. **Atomic-per-version + forward-only + idempotent.** `applyOneOwnerBundle` runs the whole
   bundle in one transaction and stamps `owner_bundle_meta` last (`owner.go:511-541`, VERIFIED),
   so a `REVOKE` without its re-`GRANT` cannot persist; `applyPendingOwnerBundles` skips already
   stamped versions (`owner.go:312`); re-running `REVOKE … ; GRANT SELECT(cols) …` is idempotent
   (same resulting ACL). Forward-only and watermark interlock (A17/A18) are untouched.
3. **Drift-repair registration.** Add the new column gates to `readScopeReasserts`
   (`owner.go:444-469`) keyed on the 0021 capability stamp
   `operator_identity_run_attribution`, so `ReassertReadRevokes` (run after every
   `ApplyOwnerBundles` and any doctor/repair-grant) re-closes them — exactly how 0006 keeps
   `principal_clients` closed against a stray `GRANT`:

```go
// owner.go readScopeReasserts — NEW entry keyed on the 0021 stamp:
"operator_identity_run_attribution": {
  // Route 2: runs column gate (re-asserted; the GRANT list is regenerated from the catalog at build).
  "REVOKE SELECT ON striatumd.runs FROM striatumd_rw",
  "GRANT SELECT (<every runs column except created_by_principal_id>) ON striatumd.runs TO striatumd_rw",
  // Route 1: operator_handles column gate (new table; restated so a drift GRANT cannot widen it).
  "REVOKE SELECT ON striatumd.operator_handles FROM striatumd_rw",
  "GRANT SELECT (handle_id, leased_session_id, handle, repository_id, released_at, leased_until, last_heartbeat_at) ON striatumd.operator_handles TO striatumd_rw",
  // operator_sessions column gate (v3 C2′, restated):
  "REVOKE SELECT ON striatumd.operator_sessions FROM striatumd_rw",
  "GRANT SELECT (operator_session_id, repository_id, state, registered_at, last_heartbeat_at, expires_at, closed_at, close_reason) ON striatumd.operator_sessions TO striatumd_rw",
},
```

> The reassert statements restate the bundle SQL verbatim (the `readScopeReasserts` contract,
> `owner.go:439-443`). Restating `operator_handles`/`operator_sessions` as REVOKE-then-column-
> GRANT (rather than relying on "granted narrow from creation") makes the reassert pass converge
> to the same closed ACL whether or not a prior stray table-wide GRANT exists — drift-proof.

### Controls (§4.5) — two-role, the COMPOSED graph

- **`composed_identity_map_unreadable` (NEGATIVE, gate-critical).** As `striatumd_rw`, BOTH
  composed queries must fail `42501` / return zero identity rows:
  - Route 1: `SELECT DISTINCT cc.client_id, oh.principal_id FROM client_capabilities cc JOIN operator_handles oh ON oh.leased_session_id = cc.session_id WHERE cc.session_id IS NOT NULL AND cc.revoked_at IS NULL AND oh.released_at IS NULL` → `42501` (`oh.principal_id` ungranted).
  - Route 2: `SELECT DISTINCT cc.client_id, r.created_by_principal_id FROM client_capabilities cc JOIN operator_handles oh ON oh.leased_session_id = cc.session_id JOIN runs r ON r.created_by_handle_id = oh.handle_id WHERE …` → `42501` (`r.created_by_principal_id` ungranted).
  - PLUS the direct controls (carried): `SELECT principal_id FROM operator_sessions` and
    `SELECT principal_id FROM operator_handles` and `SELECT created_by_principal_id FROM runs`
    each fail `42501`.
- **`whose_status_mine_via_projection` (POSITIVE).** Through the DEFINER projections (with the
  daemon secret), `whose` and `status --mine` resolve correctly; the run-origin stamp, the lease
  heartbeat, and the operator-session close all still work; the three converted star-readers
  (`run.detail`/`job.detail`/`archive`) still return their (non-identity) run rows under the
  column-scoped grant. A `42501` on any of these refutes the positive control.

### Falsifiable assertions (C2″)

- **A35 (Route 1 closed).** `cc ⋈ oh` on `oh.principal_id` fails `42501` for `striatumd_rw`.
  *Refuting test:* `composed_identity_map_unreadable` Route 1 — a returned `principal_id` refutes.
- **A36 (Route 2 closed).** `cc ⋈ oh ⋈ runs` on `r.created_by_principal_id` fails `42501`.
  *Refuting test:* `composed_identity_map_unreadable` Route 2 — a returned `principal_id` refutes.
- **A37 (no third route).** No other runtime-readable column pairs a credential id with a
  `principal_id`; the DEFINER projections are secret-gated. *Refuting test:* an ACL-graph scan
  (`information_schema.role_column_grants` for `striatumd_rw` over every `*principal_id*` column)
  finding any granted identity column, or a projection callable without the daemon secret,
  refutes.
- **A38 (column-revoke breaks no other read path).** The three star-readers are converted; every
  other runs reader uses explicit non-identity columns; `whose`/`status --mine`/stamp/heartbeat
  pass. *Refuting test:* `whose_status_mine_via_projection` — a `42501` on any legitimate runs
  read refutes.
- **A39 (drift-proof gate).** `ReassertReadRevokes` re-closes all three column gates from the
  `operator_identity_run_attribution` stamp. *Refuting test:* apply 0021, `GRANT SELECT ON
  striatumd.runs TO striatumd_rw` (simulate drift), run `ReassertReadRevokes`, re-run Route 2 —
  a returned `principal_id` refutes.
- **A19′ (REVISED — REVOKE is safe, not absent).** 0021 carries the `runs` REVOKE + column
  re-GRANT; it is atomic-per-version, idempotent, owner-held-irreversible, and reasserted.
  *Refuting test:* apply 0021 twice (idempotent no-op); inject a fault after the REVOKE before the
  GRANT and assert the bundle rolls back leaving runtime `SELECT` on runs intact (atomicity).

---

## §C1″ — The operator-token admin scope: JUSTIFIED-ACCEPTANCE as the P0 boundary (full analysis)

v3's `{admin, read}` operator-session token authorizes `run.prepare` (C1′ core, INTACT), but
`CapabilityAdmin` is coarse: the registry maps it to the whole repo-admin surface, and
`Authorize` accepts any non-revoked matching admin row with **no method allowlist**
(`auth_pg.go:104-140`, VERIFIED). The operator analysis, adopted here: **these admin verbs ARE
the operator's legitimate authority** — a real operator drives `checkpoint resolve`,
`review override`, `decision record`, `branch confirm` (this very campaign does). A
"run-lifecycle-only" capability would BREAK the operator's job. The correct resolution is
**explicit, justified acceptance** (RFC ledger option (c)), hardened by confirming the existing
trust-root fence (option (b)).

### C1″.1 The reachable admin surface, enumerated and BOUNDED by repo-scope

The operator-session token's capability rows are **repo-scoped**: the mint inserts
`client_capabilities(... , repository_id, ...)` with `repository_id = <repo>` (the
`mintSessionBoundToken` shape, `session_token.go:77-88`, VERIFIED; `mintOperatorSessionToken`
reuses it). `Authorize` matches `WHERE capability = $2 AND (repository_id IS NULL OR
repository_id = $3)` and, for a method whose `repositoryID` differs, returns
`capability_scope_mismatch`, or for a daemon-global method (`repositoryID = ""`) returns
`capability_missing` (`auth_pg.go:104-140`, VERIFIED). Therefore:

| `CapabilityAdmin` method | Scope (`registry_methods.go`) | Reachable by repo-scoped operator token? |
|---|---|---|
| `run.prepare` / `run.start` / `run.pause` / `run.resume` / `run.cancel` / `run.retry_job` | `ScopeSingleRepo` (110-115) | **YES** — the run-lifecycle the operator drives |
| `branch.confirm` (109), `checkpoint.resolve` (107), `review.override` (105), `decision.record` (106), `workflow.accept_risk` (102), `escalation.resolve` (62), `work.claim_override` (72), `repo.init` (117) | `ScopeSingleRepo` | **YES** — the operator's legitimate repo-admin authority (ACCEPTED) |
| `verifier.attest` (108) | `ScopeSingleRepo` | **REACHABLE by capability but FENCED** — refused `capability_denied` for any session-bound token (§C1″.2) |
| `repo.add` (136), `repo.remove` (137), `daemon.token.create` (138), `daemon.token.revoke` (139), `daemon.token.rotate` (140), `daemon.key.rotate` (141), `daemon.shutdown` (142), `daemon.migrate` (143) | `ScopeDaemonGlobal` | **NO** — daemon-global; a repo-scoped row cannot satisfy them (`capability_missing` / `capability_scope_mismatch`) |

**This is a material, source-grounded blast-radius bound:** a leaked operator-session token
**cannot** mint/revoke/rotate daemon credentials, rotate the signing key, shut down the daemon,
migrate the database, or add/remove repositories — the highest-consequence admin verbs are
structurally unreachable because the token is repo-scoped. The accepted surface is **per-repo
run-lifecycle + operator repo-admin, MINUS the fenced trust-root route.**

### C1″.2 The trust-root fence is confirmed; no other admin route needs (or may have) an analogous refusal

- **`verifier.attest` is already refused for the operator token.** The handler refuses any
  session-bound token: `if auth.IsSessionBound() { return … "capability_denied" … }`
  (`verifier_attest.go:53-59`, VERIFIED; comment: the refusal "holds even for a hypothetical
  admin-capable session token"). The operator-session token is session-bound
  (`SessionID = operator_session_id ≠ ""`, `IsSessionBound()` true, `capability.go:33-39`), so it
  is **already refused** at `verifier.attest`. The operator still attests pins with their
  **session-UNBOUND** static admin token — so the fence does NOT break the operator's job; it
  correctly bars the session-bound token from the one trust-root route.
- **Enumeration of every session-bound-refusing route.** The only HARD session-bound refusal in
  the codebase is `verifier.attest` (`verifier_attest.go:53`). The other `IsSessionBound()` call
  sites — `enforceSessionBinding`/`enforceSessionBindingForSession` (`mutations.go:227,247`) and
  `MayActAsSession` (`principal_session.go:21-26`) — are NOT refusals; they enforce "a
  session-bound token may act ONLY as its OWN session," and they gate **write/claim** routes
  (`interrogation.answer`, the `work.*` family, `capability.go:19-23`). The `{admin, read}`
  operator token carries no `write`/`claim` capability, so it cannot reach those routes at all
  (`capability_missing`) — they are irrelevant to the operator surface.
- **Why no NEW refusal is added (and adding one would be wrong).** Adding a `verifier.attest`-
  style `IsSessionBound()` refusal to `checkpoint.resolve` / `review.override` /
  `decision.record` / `branch.confirm` would refuse the **operator's own** session-bound token —
  i.e. it would break exactly the operator-driven verbs the SEED says must keep working. The
  trust-root threat ("a verified LANE blesses its own pins") does not transfer to these routes:
  lanes can never obtain admin (the lane slice `{claim,write,read,review}` is unchanged,
  `session_token.go:46`), and the operator token is minted only via the operator-bootstrap path
  for the human. So `verifier.attest` is the unique route requiring the refusal, and it already
  has it.

### C1″.3 N-tokens blast-radius (the justified-acceptance core)

The accepted surface is the operator's authority MINUS the trust-root route the codebase already
fences. The multiplicity (one token per terminal, ~15) is **strictly-less-standing** than the
single static admin credential the human already holds:

- **TTL-bounded.** Each operator-session token's `expires_at` ≤ the operator session's TTL
  (`sessionBoundTokenTTL = 24h`, `session_token.go:21`); an expired token fails `Authorize`
  (`token_expired`/`capability_expired`, `auth_pg.go:90-91,142-143`).
- **Revoked on graceful close.** The dedicated operator-session close sets
  `client_capabilities.revoked_at = now()` (§C1′.3 carried); a revoked grant fails `Authorize`
  (`auth_pg.go:111` filters `revoked_at IS NULL`).
- **Repo-scoped.** Each token's admin authority is bounded to its own repository and away from the
  daemon-global credential/key/shutdown/migrate surface (§C1″.1).
- **Independently revocable, count-bounded.** The live count is bounded by live operator sessions;
  each is revocable on its own without touching the others or the static token.

Net: replacing "the human drives runs with a long-lived, session-unbound, daemon-reachable static
admin token" by "N TTL-bounded, close-revoked, repo-scoped, session-bound operator tokens" is a
**reduction** in standing admin authority over time and scope, not an increase — fewer durable
credentials, each narrower and self-expiring, and the unique trust-root route fenced from all of
them.

### Control (§4.5) — `operator_token_admin_surface`

With a valid minted operator-session token: (a) `run.prepare` is authorized (the C1′ core,
NON-NULL DISTINCT `created_by_handle_id`, `whose RA ≠ whose RB`); (b) representative accepted
operator-admin routes (`checkpoint.resolve`, `review.override`, `branch.confirm`) are authorized
(documented as the accepted P0 boundary); (c) `verifier.attest` is refused typed
`capability_denied` (the trust-root fence); (d) a daemon-global admin route (`daemon.token.create`)
is denied `capability_missing`/`capability_scope_mismatch` (repo-scope bound); (e) a lane token is
denied `run.prepare` `capability_missing`; (f) a closed/expired operator session is denied and
creates no run / no stamp.

### Falsifiable assertions (C1″)

- **A40 (accepted surface authorized + documented).** The operator token clears `run.prepare`
  and the representative accepted operator-admin routes. *Refuting test:* `operator_token_admin_surface`
  (a)/(b) — a `capability_missing` on the valid operator token for an accepted route refutes the
  acceptance.
- **A41 (trust-root fenced).** `verifier.attest` refuses the session-bound operator token typed
  `capability_denied`. *Refuting test:* `operator_token_admin_surface` (c) — a successful
  attestation by the operator-session token refutes A41 (re-opens C1″).
- **A42 (repo-scope bound).** A daemon-global admin route is unreachable by the repo-scoped
  operator token. *Refuting test:* (d) — a `daemon.token.create`/`daemon.shutdown` authorized by
  the operator-session token refutes A42.
- **A43 (lane ≠ admin, unchanged).** The lane slice has no admin row; a lane token is denied
  `run.prepare`. *Refuting test:* (e), and a grep that `sessionBoundCapabilities`
  (`session_token.go:46`) is unchanged — `admin` appended, or a lane token that prepares a run,
  refutes.

---

## §0 — Verified source baseline and corrections (carried from v1+v2+v3; + v4 verifications)

The holder verifies, does not trust. I re-read the source on this branch. **Verified true
(load-bearing):**

- `LatestOwnerBundleVersion == 20`, `RequiredOwnerBundleVersion == LatestOwnerBundleVersion`
  (`owner.go:23,35`, VERIFIED). **Next free owner-bundle ordinal is 21 at design time.**
  ⚠️ **Build-time ordinal note:** a concurrent **RFC 0142 P4** build may land bundle 0021 first;
  the build run MUST re-verify the next-free ordinal at authorship time and use **0022** if 0021
  is taken. The label, `LatestOwnerBundleVersion`, and `RequiredOwnerBundleVersion` advance to
  whichever ordinal is used; nothing in this SPEC hard-depends on the literal "21".
- `0005_repo_local_workflow_state.sql:470-472` grants the runtime `SELECT,INSERT,UPDATE,DELETE
  ON ALL TABLES` and revokes back only `UPDATE,DELETE` on `events`/`artifacts` (so
  `client_capabilities`/`runs` are runtime-SELECTable, and runs columns inherit table SELECT).
- The `runs` table columns (`0005:13-35`): `repository_id, run_id, workflow_snapshot_id,
  repo_root, state, branch_name, branch_base, branch_confirmed_at, branch_confirmed_by,
  created_at, started_at, completed_at, stop_reason, paused_at, paused_reason, cross_repo_run_id`
  (+ 0021's `created_by_principal_id`, `created_by_handle_id`).
- Three `runs` star-readers: `reads/detail.go:73` (run.detail), `reads/detail.go:174`
  (job.detail), `reads/archive.go:118` (`SELECT r.*`). No other `SELECT *`/`r.*` on runs.
- Capability resolution is exact-match + repo-scoped (`auth_pg.go:104-140`); `run.prepare`/
  `run.start` → `CapabilityAdmin` (`registry_methods.go:110-111`); the full `CapabilityAdmin`
  surface and its `ScopeSingleRepo` vs `ScopeDaemonGlobal` split (`registry_methods.go:62-143`).
- `mintSessionBoundToken` inserts repo-scoped, session-bound rows (`session_token.go:60-97`);
  `sessionBoundCapabilities = {claim,write,read,review}` (`session_token.go:46`).
- `verifier.attest` `IsSessionBound()` refusal (`verifier_attest.go:53-59`); the only hard
  session-bound admin refusal; `MayActAsSession`/`enforceSessionBinding` are act-as-own-session
  guards on write/claim routes (`principal_session.go:21-26`, `mutations.go:225-257`).
- `resolve_principal_for_client` is `SECURITY DEFINER`, secret-gated by `assert_daemon_authority`,
  `GRANT EXECUTE … TO striatumd_rw` (`0006:56-79,181,186`); `readScopeReasserts` /
  `ReassertReadRevokes` drift registry (`owner.go:444-509`); owner bundles apply atomic-per-
  version, stamp last, forward-only (`owner.go:305-322,511-541`).

**CORRECTION C-1 — `ALTER runs ADD COLUMN` and the two new tables are OWNER-bundle changes.**
`runs` is owner-held on a two-role deploy and not in 0018's transfer cohort, so a runtime `ALTER
runs` dies `42501 must be owner of table runs`. `ALTER runs ADD COLUMN`, `operator_handles`, and
`operator_sessions` all go in owner bundle 0021. **C2″ consequence:** because `runs` is
owner-held, the §C2″ column-revoke is an irreversible boundary with no ownership transfer (0006
needed one for runtime-owned `principal_clients`; we do not).

**CORRECTION C-2 — the authority GUC `striatum.principal_id` holds the `client_id`; the
run-origin principal resolves in Go via `admin.ResolvePrincipalForClient` (the
`resolve_principal_for_client` projection), bound as a parameter.** (INTACT from v2.)

**CORRECTION C-3 — there is no periodic session reaper; release is graceful-close + lazy
expiry.** The operator session uses a dedicated close path (now also revoking the operator
token), never run-scoped `closeRemainingSessions` (`mutations.go:1432`).

**CORRECTION C-4 — `created_by_principal_id` alone is insufficient to answer "which window";
the disambiguator is the per-session `runs.created_by_handle_id` snapshot.** `branch_confirmed_by`
holds `'daemon'`/`'human'` literals (`run.go`), not a `principal_id` (so OQ2 = NULL, no backfill).

---

## How this SPEC discharges R1a / R1b / R1c / R2 / R3 / R4 (auditable coverage map)

| Req | What it demands | Where | Load-bearing assertion(s) |
|-----|-----------------|-------|---------------------------|
| **R1a** | Identity bound server-side at token-mint against the live token; never tty/tmux/title/env; reads resolve through `principal_id`, only snapshot the handle | §1 | A1–A5 |
| **R1b** | THE CRUX — one human = one `principal_id` across ~15 terminals; deterministic escalation; the run→handle join; two same-human terminals return two distinct answers on a buildable + authorizable substrate | §2, §C1′(carried) | A6–A11, A27, A29–A32 |
| **R1c** | Heartbeat renews via guarded UPDATE, never release-then-reacquire | §3 | A12 |
| **R2** | Owner-bundle migration at the next free ordinal; DB write-once + justify; pin retained privileges (no identity grant-back, **incl. the composed-route closure**); prove clean apply + write-once + two-role stamp safety; forward-only, watermark-consistent | §4, §C2″ | A13–A19′, A28, A33–A39 |
| **R3** | Resolve all four open questions concretely | §5 | A20–A23 |
| **R4** | Ride RFC 0107/0114 (operator-id IS `principal_id`); no parallel identity table; **no read-scope punch-through, including composed routes**; reuse the projection/token-mint shapes; product-boundary clean | §6, §C2″ | A24–A26, A28, A35–A37 |

The full assertion ledger is §8 (A1–A43). The P0 boundary and P1–P3 seams are §7. The build
manifest is §9.

---

## §C1′ — operator-token AUTHORIZATION (carried forward from v3, INTACT; now scope-bounded by §C1″)

The v3 C1′ core is unchanged and INTACT. A distinct **`mintOperatorSessionToken`** — the
composition of `admin.insertTokenClient`'s caller-supplied capability set (`tokens.go:286-315`)
with `mintSessionBoundToken`'s session binding (`session_token.go:60-97`) — inserts
`operatorSessionCapabilities = {admin, read}`, each a `client_capabilities` row carrying
`session_id = operator_session_id` AND `repository_id = <repo>`. The `admin` row clears the
exact-match, repo-scoped `Authorize` for `run.prepare` (`auth_pg.go:104-140`;
`registry_methods.go:110`); the prelude sets `app.session_id` (`authority.go:79,119,158`); the
`created_by_handle_id` subquery resolves the lease. The shared `sessionBoundCapabilities` lane
slice is **UNCHANGED** (`session_token.go:46`), so lane tokens cannot gain admin. Graceful close
revokes the operator token (`client_capabilities.revoked_at`) + releases the handle in one txn;
TTL expiry bounds it. **v4 adds the SCOPE analysis (§C1″): the admin authority is repo-scoped,
the trust-root route is fenced, and the surface is an explicitly justified P0 boundary.**

- **A29 (authorizes `run.prepare`)**, **A30 (lane ≠ admin)**, **A31 (closed/expired cannot
  stamp)**, **A32 (distinct mint, not the lane slice)** — all carried INTACT; tested by
  `operator_session_pre_run_stamp` (§4.5) and `operator_token_admin_surface` (§C1″).

---

## §1 — R1a: identity bound server-side, at token-mint, against the live token (carried; stamp reads via projection)

**(1) The handle lease + operator token are minted in one operator-bootstrap transaction.**
In one transaction: (a) resolve/create the caller's `principal` (`kind='human'`, via the
owner-owned create + `link_client_to_principal` projection, `0006:160-188`); (b) mint a
session-bound operator token via **`mintOperatorSessionToken`** (`{admin, read}`, bound to a
fresh `operator_session_id`, repo-scoped); (c) INSERT the `operator_sessions` row; (d) acquire
the handle lease into `operator_handles` keyed on `principal_id` and `leased_session_id =
operator_session_id`. Mint + link + lease + operator-session share one transaction (A3). The
`principal_id` is held in Go and bound as a parameter; `operator_sessions.{principal_id,client_id}`
and `operator_handles.principal_id` are written from Go-held values at INSERT (no SELECT, §C2″).

**(2) `runs.created_by_principal_id` is resolved from the live token through the projection
(C2, INTACT).** In `HandleRunPrepare`'s authorized transaction:

```go
auth := rpc.AuthFromContext(ctx)                                         // ClientID from the validated live token
ref, ok, err := admin.ResolvePrincipalForClient(ctx, tx, auth.ClientID)  // -> resolve_principal_for_client projection
```

```sql
INSERT INTO striatumd.runs (..., created_by_principal_id, created_by_handle_id)
VALUES (..., $N,                                          -- bound: ref.PrincipalID resolved in Go via the projection
  (SELECT oh.handle_id FROM striatumd.operator_handles oh
    WHERE oh.leased_session_id = current_setting('app.session_id', true)  -- = operator_session_id (admin-authorized prelude, C1′)
      AND oh.released_at IS NULL));
```

`HandleRunPrepare` reads only `repository_id`/`workflow` from the envelope; a forged
`created_by_*` request param is structurally ignored. The `INSERT` uses table-level `INSERT`
privilege (the column-revoke is `SELECT`-only, §C2″), so the stamp is unaffected. The handle
subquery reads only `oh.handle_id`/`oh.leased_session_id`/`oh.released_at` (all granted, §C2″.2a).

**(3) Every read surface resolves through `principal_id` and only snapshots the handle.**
`whose`, `status --mine`, `doctor`, evidence export render from the DEFINER projections over
`created_by_principal_id`/`created_by_handle_id`/`operator_handles`/`principals` (§2.4, §C2″.2c).
No tty/pane/tmux/title/env value enters any authoritative answer.

### Falsifiable assertions

- **A1 (server-side stamp).** `created_by_principal_id` = the live token's principal, via the
  projection. *Refuting test:* `run_origin_stamp_uses_identity_projection` (§4.5) — forge envelope
  `created_by_principal_id = P_B` + spoofs on `P_A`'s token; stored value must be `P_A`.
- **A2 (no client-name path).** *Refuting test:* a guard greps the prepare/operator-mint handlers
  for `stringParam(envelope, "created_by*"|"handle"|"operator*")` feeding the stamp/lease.
- **A3 (mint+link+lease atomicity).** *Refuting test:* fault between mint and lease INSERT; the
  whole bootstrap rolls back.
- **A4 (live-token resolution).** *Refuting test:* a revoked/expired token to `run.prepare` is
  rejected (`auth_pg.go:87-92`) and creates no run.
- **A5 (read surfaces cannot lie).** *Refuting test:* the `whose`/`run_origin_identity` SQL
  references no tty/pane/title/env.

---

## §2 — R1b (THE CRUX): per-human `principal_id` vs per-terminal session granularity (carried; authorizable + read-closed)

### 2.1–2.4 (carried, unchanged)

`created_by_principal_id` alone cannot answer "which window" (one human = one principal). The
disambiguator is a **per-session leased handle** in `operator_handles` (§2.2) with the live-unique
partial index `operator_handles_live_uq (repository_id, lower(handle)) WHERE released_at IS NULL`
and `operator_handles_live_session_uq`. A deterministic, principal-seeded candidate walk drives
default and escalation (§2.3). The run carries a **write-once `runs.created_by_handle_id`
snapshot** (FK → `operator_handles.handle_id`); `whose` joins through it (§2.4, now via the
`run_origin_identity` DEFINER projection — §C2″.2c). Lazy expiry only sets `released_at`; rows are
never deleted (§4.4), so a snapshot `handle_id` never dangles and a re-lease creates a *new* row.

```sql
-- whose <run-id> — resolved through the run_origin_identity DEFINER projection (§C2″.2c),
-- which executes the pure join below AS THE DEFINER (striatumd_rw never selects the identity cols):
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
   `mintOperatorSessionToken` (`{admin, read}`, repo-scoped, binding `session_id = S1`),
   `link_client_to_principal(client_S1, P)`, INSERT `operator_sessions(S1)`, lease walk
   `INSERT maya` → `S1` holds `maya` (`handle_id = h1`).
2. **Operator session `S2`** (same human, second terminal) → one txn: same, lease walk
   `INSERT maya` → `23505` → `INSERT theo` → `S2` holds `theo` (`handle_id = h2`).
3. `S1` calls `run.prepare` — its **admin** row clears the repo-scoped `Authorize`
   (`auth_pg.go:104-140`); prelude sets `app.session_id = S1` → run `RA`
   (`created_by_principal_id = P` via projection, `created_by_handle_id = h1`).
4. `S2` calls `run.prepare` → run `RB` (`created_by_principal_id = P`, `created_by_handle_id = h2`).
5. `whose RA` (via `run_origin_identity`) → `maya#7f3`. `whose RB` → `theo#7f3`.

**Two distinct answers** (`maya#7f3 ≠ theo#7f3`); the identical suffix signals "same human."
Every step **builds** (operator session, no `sessions` row — C1), **authorizes** (admin-bearing
operator token clears `run.prepare` — C1′), and is **read-closed** (the identity columns are
unreadable by `striatumd_rw` outside the DEFINER — C2″). This is the
`operator_session_pre_run_stamp` pgtest (§4.5).

### 2.6 The pre-run operator-session substrate + lifecycle (carried; column-scoped SELECT)

```sql
CREATE TABLE striatumd.operator_sessions (
  repository_id       text NOT NULL REFERENCES striatumd.repositories(repository_id),
  operator_session_id text NOT NULL,
  principal_id        text NOT NULL REFERENCES striatumd.principals(principal_id),  -- SELECT NOT granted (C2′/C2″)
  client_id           text NOT NULL,                                               -- SELECT NOT granted (C2′/C2″)
  state               text NOT NULL CHECK (state IN ('active','closed','expired')),
  registered_at       timestamptz NOT NULL,
  last_heartbeat_at   timestamptz,
  expires_at          timestamptz NOT NULL,
  closed_at           timestamptz,
  close_reason        text,
  PRIMARY KEY (repository_id, operator_session_id)
);
CREATE INDEX operator_sessions_principal_live
  ON striatumd.operator_sessions (repository_id, principal_id) WHERE state = 'active';
```

```sql
CREATE TABLE striatumd.operator_handles (
  handle_id         text PRIMARY KEY,
  repository_id     text NOT NULL REFERENCES striatumd.repositories(repository_id),
  principal_id      text NOT NULL REFERENCES striatumd.principals(principal_id),    -- SELECT NOT granted (C2″ Route 1)
  handle            text NOT NULL,
  leased_session_id text NOT NULL,
  leased_until      timestamptz NOT NULL,
  released_at       timestamptz,
  last_heartbeat_at timestamptz
);
CREATE UNIQUE INDEX operator_handles_live_uq
  ON striatumd.operator_handles (repository_id, lower(handle)) WHERE released_at IS NULL;
CREATE UNIQUE INDEX operator_handles_live_session_uq
  ON striatumd.operator_handles (repository_id, leased_session_id) WHERE released_at IS NULL;
```

**Lifecycle** (carried): **Create** — the atomic operator-bootstrap mint (§1(1)). **Heartbeat** —
guarded UPDATE (§3) renewing `operator_sessions` liveness + `operator_handles` lease + the
operator token `expires_at`. **Graceful close** — one txn: `operator_sessions.state='closed'`,
`operator_handles.released_at=now()`, **revoke the operator token**; dedicated path, never
`closeRemainingSessions`. **Lazy expiry** — no reaper (C-3). **run → handle join** —
`run.prepare` reads `app.session_id` (§1(2)).

### Falsifiable assertions (carried)

- **A6** (live-unique forces distinct words), **A7** (distinct `whose`, gate-critical), **A8**
  (deterministic default), **A9** (deterministic escalation), **A10** (no silent relabel),
  **A11** (one winner, no deadlock), **A27** (operator session buildable pre-run) — all carried;
  tested by `operator_session_pre_run_stamp` / `two_live_maya`.

---

## §3 — R1c: lease flap (carried, unchanged)

```sql
UPDATE striatumd.operator_handles
   SET leased_until = now() + $TTL, last_heartbeat_at = now()
 WHERE handle_id = $1 AND leased_session_id = $2 AND released_at IS NULL;
```

A guarded UPDATE that never deletes, sets `released_at`, or re-INSERTs; in the same heartbeat txn
it renews `operator_sessions` + the operator token `expires_at`. The guard means only the owning,
still-live session renews, and the row never transits a released state mid-renewal — so
`operator_handles_live_uq` never frees the word mid-flap. Mirrors the `striatumd.leases` idiom.

- **A12 (flap-resistance).** *Refuting test:* `lease_flap_steal` (two-role) — interleave `S1`'s
  renewal with `S2`'s attempt to lease `maya`; `S2` must always get `23505` and escalate, and
  `S1`'s row was never `released_at`-set.

---

## §4 — R2: the owner-bundle migration (consolidated; §4.2 grants + §4.5 tests carry the C2″/C1″ controls)

### 4.1 Ordinal and placement

Owner bundle **0021** (next free after `LatestOwnerBundleVersion == 20`; **build re-verifies** —
0022 if RFC 0142 P4 takes 0021, §0). New file
`go/pkg/db/sql/owner/00NN_operator_identity_run_attribution.sql`, auto-discovered
(`owner.go:156` `//go:embed sql/owner/*.sql`), plus the `ownerBundleLabels[NN]` entry,
`LatestOwnerBundleVersion = NN`, `RequiredOwnerBundleVersion = NN`, and the `readScopeReasserts`
entry (§C2″.5). Owner bundle because the two new tables + `ALTER runs` + the runs REVOKE touch
owner-held tables (C-1).

### 4.2 Bundle SQL (consolidated)

```sql
-- owner bundle 00NN — applied OUT-OF-BAND as the owner via `striatum daemon owner-ddl apply`, THEN restart.

-- (1) operator_handles + operator_sessions tables + indexes (§2.6).
CREATE TABLE IF NOT EXISTS striatumd.operator_handles ( ... );            -- §2.6
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_uq          ...;  -- §2.6
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_session_uq  ...;  -- §2.6
CREATE TABLE IF NOT EXISTS striatumd.operator_sessions ( ... );          -- §2.6
CREATE INDEX IF NOT EXISTS operator_sessions_principal_live         ...;  -- §2.6

-- (2) runs origin stamp columns (owner-held table -> owner bundle, C-1).
ALTER TABLE striatumd.runs ADD COLUMN IF NOT EXISTS created_by_principal_id text REFERENCES striatumd.principals(principal_id);
ALTER TABLE striatumd.runs ADD COLUMN IF NOT EXISTS created_by_handle_id    text REFERENCES striatumd.operator_handles(handle_id);

-- (3) write-once at the DB (BEFORE UPDATE trigger, §4.3) — unchanged from v2/v3.
CREATE OR REPLACE FUNCTION striatumd.refuse_run_origin_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.created_by_principal_id IS DISTINCT FROM OLD.created_by_principal_id
     OR NEW.created_by_handle_id  IS DISTINCT FROM OLD.created_by_handle_id THEN
    RAISE EXCEPTION 'runs.created_by_* origin stamp is write-once (set at run creation, immutable thereafter)';
  END IF;
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS runs_origin_write_once ON striatumd.runs;
CREATE TRIGGER runs_origin_write_once BEFORE UPDATE ON striatumd.runs
  FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_run_origin_change();

-- (4) SECURITY DEFINER identity projections (§C2″.2c): run_origin_identity, runs_for_origin_client.
--     (full bodies in §C2″.2c) + REVOKE ALL FROM PUBLIC + GRANT EXECUTE TO striatumd_rw.

-- (5) runtime-role DML + COLUMN-SCOPED read grants (C2′ + C2″ — the composed-route closure).
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    -- operator_handles: NEW table, column-scoped SELECT from creation EXCLUDING principal_id (C2″ Route 1):
    GRANT SELECT (handle_id, leased_session_id, handle, repository_id, released_at, leased_until, last_heartbeat_at)
      ON striatumd.operator_handles TO striatumd_rw;
    GRANT INSERT, UPDATE ON striatumd.operator_handles TO striatumd_rw;        -- no DELETE
    -- operator_sessions: NEW table, column-scoped SELECT EXCLUDING principal_id AND client_id (C2′):
    GRANT SELECT (operator_session_id, repository_id, state, registered_at, last_heartbeat_at, expires_at, closed_at, close_reason)
      ON striatumd.operator_sessions TO striatumd_rw;
    GRANT INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw;       -- no DELETE
    -- runs: EXISTING table with table-level SELECT (0005). REVOKE + column re-GRANT EXCLUDING created_by_principal_id (C2″ Route 2).
    --       Build regenerates the column list from information_schema.columns at authorship time (§C2″.2b).
    REVOKE SELECT ON striatumd.runs FROM striatumd_rw;
    GRANT SELECT (repository_id, run_id, workflow_snapshot_id, repo_root, state,
                  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at,
                  started_at, completed_at, stop_reason, paused_at, paused_reason, cross_repo_run_id,
                  created_by_handle_id)
      ON striatumd.runs TO striatumd_rw;
    -- runs INSERT/UPDATE/DELETE table-level DML unchanged (0005); the stamp's INSERT(created_by_principal_id) rides table INSERT.
  END IF;
END $$;

-- (6) watermark/capability stamp (mirrors every bundle, e.g. 0018).
INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('operator_identity_run_attribution', true, NN)         -- requires_daemon_auth=true: the projections are daemon-gated
ON CONFLICT (capability) DO NOTHING;
```

No `SELECT(principal_id)` grant-back on `principal_clients` (0006 stays closed). The 0021
capability stamp `operator_identity_run_attribution` keys the new `readScopeReasserts` entry
(§C2″.5) so `ReassertReadRevokes` re-closes all three column gates on drift.

### 4.3 Write-once enforcement (carried, unchanged)

A `BEFORE UPDATE` trigger that raises when either origin column changes — chosen over a column
REVOKE for write-once because `runs` is actively UPDATEd on ~six paths
(`0010_artifact_blob_update_trigger.sql:19-49` precedent). The stamp happens at INSERT (no
`BEFORE UPDATE` fire); `IS DISTINCT FROM` also forbids a later `NULL → value` UPDATE.
**Distinct from the §C2″ SELECT-revoke:** the trigger enforces immutability (UPDATE), the
column-revoke enforces read-scope (SELECT) — orthogonal, both required.

### 4.4 Privileges the runtime role must RETAIN / LOSE (pinned)

- **`runs`** — `INSERT, UPDATE, DELETE` table-level (unchanged, `0005`); **`SELECT` column-scoped
  to EXCLUDE `created_by_principal_id`** (C2″ Route 2); the write-once trigger blocks UPDATEs to
  the two origin columns.
- **`operator_handles`** — `SELECT` column-scoped EXCLUDING `principal_id` (C2″ Route 1);
  `INSERT`/`UPDATE`; no `DELETE`.
- **`operator_sessions`** — `SELECT` column-scoped EXCLUDING `principal_id` AND `client_id`
  (C2′); `INSERT`/`UPDATE`; no `DELETE`.
- **`run_origin_identity` / `runs_for_origin_client`** — `EXECUTE` granted; the identity reads
  ride these DEFINER projections.
- **`resolve_principal_for_client`** — `EXECUTE` already granted (`0006:186`); the run-origin
  stamp rides it. **`SELECT(principal_id)` on `principal_clients` stays REVOKED.**

### 4.5 Named two-role pgtests (consolidated)

All DB-boundary tests use `pgtest.TwoRole(t)`: bundle DDL via `OwnerPool`, runtime behavior via
`SUTPool` (`striatumd_rw`), daemon authority bootstrapped so the projection + real `run.prepare`
authorization paths are exercised. `assertSQLState42501` is the privilege oracle.

1. **`owner_bundle_applies_clean`** — apply 0021 via `OwnerPool`; assert the two tables + indexes
   + `runs` columns + trigger + projections exist; then as `SUTPool` assert the runtime role
   **can** INSERT a run carrying `created_by_principal_id` (table INSERT), INSERT/UPDATE
   `operator_handles`/`operator_sessions`, SELECT the *granted* columns, EXECUTE the projections
   (daemon auth), and **cannot** `ALTER TABLE runs` (`42501`), `DELETE` the new tables, **nor**
   `SELECT created_by_principal_id FROM runs` / `SELECT principal_id FROM operator_handles` /
   `SELECT principal_id, client_id FROM operator_sessions` (the **direct** C2″/C2′ negative
   controls, `42501` — A33/A35/A36 direct face).
2. **`composed_identity_map_unreadable`** (C2″ gate, NEW) — as `SUTPool`, the **composed** Route 1
   (`cc ⋈ oh`) AND Route 2 (`cc ⋈ oh ⋈ runs`) queries each fail `42501` (§C2″ controls) — A35/A36.
3. **`whose_status_mine_via_projection`** (C2″ positive + C2 gate) — as `SUTPool`: (a) a direct
   `SELECT pc.principal_id FROM principal_clients` fails `42501`; (b) the run-origin stamp
   (`ResolvePrincipalForClient` → projection → bound `$N`) stores the linked `P`; (c) a forged
   envelope `created_by_principal_id = P_other` still stores `P`; (d) `whose` (via
   `run_origin_identity`) and `status --mine` (via `runs_for_origin_client`) render correctly;
   (e) the three converted star-readers (`run.detail`/`job.detail`/`archive`) still return their
   run rows under the column grant — A1, A28, A38.
4. **`operator_session_pre_run_stamp`** (C1′ gate) — create two pre-run operator sessions for one
   `P` (no `sessions` row, no run), mint each via the operator-token path (`{admin, read}`,
   repo-scoped, session-bound); lease two distinct words; create one run per session via the
   **real `run.prepare` RPC**; assert authorized (not `capability_missing`), two NON-NULL DISTINCT
   `created_by_handle_id`, `whose RA ≠ whose RB` — A29/A7/A27.
5. **`operator_token_admin_surface`** (C1″ gate, NEW) — with a valid minted operator-session
   token: `run.prepare` + representative accepted operator-admin routes
   (`checkpoint.resolve`/`review.override`/`branch.confirm`) authorized; `verifier.attest` refused
   `capability_denied` (trust-root fence — A41); a daemon-global admin route
   (`daemon.token.create`) denied `capability_missing`/`capability_scope_mismatch` (repo-scope —
   A42); a lane token denied `run.prepare` `capability_missing` (A43); a closed/expired operator
   session denied + no run/stamp (A31) — A40/A41/A42/A43.
6. **`forged_update_created_by_rejected`** — `UPDATE runs SET created_by_principal_id` on a
   stamped run raises the trigger (`P0001`) — A14.
7. **`drift_reassert_recloses_routes`** (NEW) — apply 0021; `GRANT SELECT ON striatumd.runs TO
   striatumd_rw` (simulate a drift/repair grant); run `ReassertReadRevokes`; re-run Route 2 —
   must again fail `42501` — A39.
8. **`two_live_maya`** — the §2.3 collision/escalation invariants (A6, A11).
9. **`token_revoked_bare_id`** — revoke the creating client + close its operator session; assert
   `status --mine` falls back to the **bare id** while `whose <past-run>` still renders the
   **frozen** historical `word#suffix`.
10. **`lease_flap_steal`** — A12 (§3).

### 4.6 Forward-only and watermark consistency (carried; now covers the REVOKE)

- **Forward-only + atomic.** `applyPendingOwnerBundles` applies only bundles `> current`
  (`owner.go:312`); `applyOneOwnerBundle` runs the whole bundle (incl. the runs REVOKE + GRANT) in
  one transaction, stamping `owner_bundle_meta` last (`owner.go:511-541`) — no half-applied
  REVOKE. Re-apply is idempotent (REVOKE+GRANT yields the same ACL).
- **Advance `RequiredOwnerBundleVersion` to NN.** The serving binary's `run.prepare`/`whose`
  reference the new columns/tables/projections; `CheckOwnerBundleWatermark` (`owner.go:124-154`)
  halts cleanly (`AwaitingOwnerDDLError`) if the daemon restarts before 0021 applied.
- **Deploy ordering (apply THEN restart).** `striatum daemon owner-ddl apply --owner-url …` →
  restart. Restart-first → `20 < required NN` → clean halt with remediation.

### Falsifiable assertions

- **A13 (owner-only ALTER)** — runtime `ALTER runs` fails `42501`; *test:* `owner_bundle_applies_clean`.
- **A14 (write-once at the DB)** — *test:* `forged_update_created_by_rejected`.
- **A15 (retained privileges exact)** — runtime can do everything P0 needs and nothing more
  (no `ALTER`, no `DELETE`, no `SELECT(created_by_principal_id)`/`SELECT(principal_id on
  operator_handles/operator_sessions)`); *test:* `owner_bundle_applies_clean` (+ the §C2″ negs).
- **A16 (clean apply, non-superuser owner)** — *test:* the apply step.
- **A17 (forward-only / idempotent REVOKE)** — re-applying 0021 is a no-op; *test:* apply twice.
- **A18 (watermark interlock)** — a binary built against NN refuses to serve on a 20-watermark DB.
- **A19′ (REVOKE safe, REVISED)** — §C2″.5; *test:* atomicity + idempotency (above).
- **A28 (two-role stamp safety via the projection)** — *test:*
  `whose_status_mine_via_projection` (a,b).

---

## §5 — R3: resolve all four open questions (carried, DISCHARGED)

- **OQ1 — pool/default/escalation/denylist → IN P0.** Curated lowercase first-names pool (~256
  neutral names, a Go slice). Default = `POOL[fnv64a(principal_id) mod len(POOL)]` (deterministic,
  reconnect-stable, not tty). Escalation = principal-seeded walk to the next distinct curated word.
  Denylist = reserved words (`daemon`, `scheduler`, `system`, `admin`, `root`, `unknown`, `anon`,
  `none`, the `principal_kind` names) excluded; service principals draw a disjoint sub-pool. **A20.**
- **OQ2 — backfill vs NULL → NULL + advisory `attribution_unknown`, in P0.** No backfill
  (`branch_confirmed_by` carries no `principal_id`, C-4). **A21.**
- **OQ3 — cross-repo board → per-repo only in P0; daemon-wide DEFERRED (P3).** Both tables keyed
  `(repository_id, …)`. **A22.**
- **OQ4 — `@handle#suffix` artifact byline → OUT of P0 (P2).** P0 changes no artifact
  `author_line`/anchor metadata. **A23.**

---

## §6 — R4: ride RFC 0107/0114; do not rebuild it (carried; reuse + composed-route closure)

- **Operator-id IS `principal_id`.** `operator_handles.principal_id` / `operator_sessions.
  principal_id` / `runs.created_by_principal_id` are FKs to `striatumd.principals`; none stores
  identity attributes.
- **Reuse, don't duplicate.** Client→principal dereference reuses `resolve_principal_for_client`;
  the new `run_origin_identity`/`runs_for_origin_client` projections follow its exact
  secret-gated DEFINER shape (`0006`). The handle lease reuses the `striatumd.leases` TTL idiom.
  The operator token reuses `insertTokenClient` + `mintSessionBoundToken` shapes. Release is the
  dedicated operator-session close (+ token revoke) + lazy expiry; **no new reaper** (C-3).
- **No read-scope punch-through (incl. composed routes).** The RFC 0114 closure is **preserved,
  not regressed** — the composed `cc ⋈ oh` and `cc ⋈ oh ⋈ runs` routes are closed at the column
  layer and the identity reads ride DEFINER projections (§C2″). This is the R4/RFC-0114
  carry-forward, now provably intact under the composed graph.
- **Product-boundary clean.** No hosted service/directory/telemetry/external identity; tty/tmux/
  title/env never read for state (A2/A5); `run_id` stays opaque.

- **A24 (no parallel identity)** — *test:* the new tables have no `display_name`/`kind`/auth
  columns; identity read only from `principals`.
- **A25 (no new reaper)** — *test:* release via the dedicated close + lazy expiry; no new goroutine.
- **A26 (opaque run_id)** — *test:* `run_id` generation unchanged, no handle encoded.

---

## §7 — P0 boundary and seams for P1–P3 (carried)

P0 ships: `operator_handles` + live-unique indexes (§2.6); `operator_sessions` + lifecycle
(§2.6); `runs.created_by_principal_id`/`created_by_handle_id` write-once (§4) with the composed-
route read closure (§C2″); the operator-bootstrap mint+lease RPC riding
`mintOperatorSessionToken` + `link_client_to_principal` (§1, §C1′), with the justified-acceptance
admin boundary (§C1″); the projection-routed run-origin stamp (§1(2)); `run_origin_identity` /
`runs_for_origin_client` DEFINER projections (§C2″.2c); `whose <run-id>` (new read RPC,
`CapabilityRead`, registered in `contracts/daemon_methods.json` + routes +
`docs/reference/command-authority-matrix.md` + `registry_contract_test`); `status --mine`
manifest; the `attribution_unknown` advisory doctor rule (§5). Seams NOT designed: **P1** custody
(`run_custody_log`); **P2** honest bylines + handoff naming + chips + OSC title; **P3** lineage +
cross-repo board.

---

## §8 — Consolidated falsifiable-assertion ledger

| # | Claim | Anchor | Refuting observation / named test |
|---|-------|--------|-----------------------------------|
| A1 | Stamp = live-token principal, server-side, via projection | §1(2); `authority.go:116-120` | forged param / spoof leak — `run_origin_stamp_uses_identity_projection` |
| A2 | No client-name path to attribution | §1; `run.go` envelope read | grep finds `created_by`/handle param feeding stamp/lease |
| A3 | Mint+link+lease+operator-session atomic | §1(1); `0006:160-188` | token committed without link+lease |
| A4 | Identity from validated token only | `auth_pg.go:49-157,87-92` | revoked token still stamps |
| A5 | Read surfaces cannot lie | §2.4 / `run_origin_identity` | tty/pane/title/env in the authoritative answer |
| A6 | Live-unique forces distinct words | `operator_handles_live_uq` | duplicate live `maya` / deadlock — `two_live_maya` |
| A7 | Two terminals → distinct `whose` | §2.5 | `whose RA == whose RB` (gate-critical) |
| A8 | Deterministic default, reconnect-stable | §2.3, §5 walk | different word on reconnect |
| A9 | Deterministic escalation, reconnect-stable | §2.3 walk | non-`candidates[1]` / drift |
| A10 | No silent relabel | write-once `created_by_handle_id` (§4.3) | `whose RB` changes after peer close/reconnect |
| A11 | One winner, no deadlock | partial-index serialization (§2.3) | `40P01` / duplicate / both-fail |
| A12 | Flap-resistant renewal | guarded UPDATE (§3) | steal during flap — `lease_flap_steal` |
| A13 | Owner-only ALTER | C-1; `0018:8-22` | runtime ALTER succeeds — `owner_bundle_applies_clean` |
| A14 | Write-once at the DB | trigger (§4.3); `0010:19-49` | UPDATE changes a stamped column — `forged_update_created_by_rejected` |
| A15 | Retained privileges exact | §4.4; `0005:470` | needed op `42501` / surplus grant (incl. C2″ negs) |
| A16 | Clean apply, non-superuser owner | two-role `OwnerPool` | `42501`/`must be member` on apply |
| A17 | Forward-only / idempotent REVOKE | `owner.go:312,511-541` | second-apply error / watermark regression |
| A18 | Watermark interlock | `owner.go:124-154` | serves on 20-watermark DB |
| A19′ | **REVOKE safe (REVISED, not absent)** | §C2″.5; `owner.go:511-541,444-469` | half-applied REVOKE persists / non-idempotent re-apply |
| A20 | Pool/default/escalation/denylist | §5 golden test | unstable default / denied word |
| A21 | NULL + advisory, no backfill | §5; C-4 | red classification / backfill write |
| A22 | Per-repo only in P0 | §5 | cross-repo aggregation in P0 |
| A23 | Byline suffix out of P0 | §5 | `author_line` change in P0 |
| A24 | No parallel identity table | §6; `0023:30-36` | identity attribute on the new tables |
| A25 | No new reaper | §6; dedicated close | new periodic session reaper |
| A26 | Opaque run_id | §6 | handle encoded into `run_id` |
| A27 | Operator session buildable pre-run (C1) | §2.6; `auth_pg.go:104-156` | NULL `created_by_handle_id` — `operator_session_pre_run_stamp` |
| A28 | Two-role stamp safety via projection (C2) | §1(2); `0006:56-79,181,186` | real stamp `42501` / direct `pc.principal_id` read |
| A29 | Operator token AUTHORIZES `run.prepare` (C1′) | §C1′; `registry_methods.go:110`; `auth_pg.go:104-140` | `capability_missing` on the valid operator token — `operator_session_pre_run_stamp` |
| A30 | Lane tokens do NOT gain admin (C1′) | `session_token.go:46,77-89` | lane token prepares a run |
| A31 | Closed/expired operator session cannot stamp (C1′) | §C1′; `auth_pg.go:87-92,111,142-143` | stamp via closed/expired token — `operator_token_admin_surface` |
| A32 | Distinct operator mint, not the lane slice (C1′) | §C1′; `tokens.go:286-315` | `admin` appended to `sessionBoundCapabilities` |
| A33 | `operator_sessions` direct identity map unreadable (C2′) | §C2″.5; `0006:218-220` | `SELECT principal_id, client_id FROM operator_sessions` succeeds — `owner_bundle_applies_clean` |
| A34 | Narrow grants still suffice (C2′ positive) | §C2″; §4.4 | `42501` on create/heartbeat/close/stamp |
| **A35** | **C2″ Route 1 closed (`cc ⋈ oh` on principal_id)** | §C2″.2a; `0005:470` | returned `principal_id` — `composed_identity_map_unreadable` Route 1 |
| **A36** | **C2″ Route 2 closed (`cc ⋈ oh ⋈ runs` on created_by_principal_id)** | §C2″.2b | returned `principal_id` — `composed_identity_map_unreadable` Route 2 |
| **A37** | **No third route over the ACL graph** | §C2″.4 | a granted `*principal_id*` column / unsecreted projection — ACL-graph scan |
| **A38** | **Column-revoke breaks no other read path** | §C2″.2d,3 | `42501` on a legitimate runs read — `whose_status_mine_via_projection` |
| **A39** | **Drift-proof gate (reassert re-closes)** | §C2″.5; `owner.go:444-509` | drift GRANT survives reassert — `drift_reassert_recloses_routes` |
| **A40** | **Accepted operator-admin surface authorized + documented (C1″)** | §C1″.1 | accepted route `capability_missing` — `operator_token_admin_surface` |
| **A41** | **Trust-root fenced: `verifier.attest` refuses the session-bound operator token (C1″)** | `verifier_attest.go:53-59` | successful attestation by the operator token — `operator_token_admin_surface` |
| **A42** | **Repo-scope bound: daemon-global admin unreachable (C1″)** | §C1″.1; `session_token.go:84`; `auth_pg.go:104-140` | `daemon.token.create`/`shutdown` authorized by the operator token |
| **A43** | **Lane ≠ admin, unchanged (C1″)** | `session_token.go:46` | lane token prepares a run / admin appended |

---

## §9 — Build manifest (P0 scope, for the downstream `code_change` run)

1. **Owner bundle** — `go/pkg/db/sql/owner/00NN_operator_identity_run_attribution.sql` (§4.2:
   `operator_handles` + `operator_sessions` + `runs` columns + write-once trigger + the two
   DEFINER identity projections + COLUMN-SCOPED grants incl. the **runs REVOKE + column re-GRANT**
   and the **operator_handles principal_id exclusion** + watermark stamp); `owner.go` label entry +
   `LatestOwnerBundleVersion = NN` + `RequiredOwnerBundleVersion = NN`; **the new
   `readScopeReasserts["operator_identity_run_attribution"]` entry** (§C2″.5) — load-bearing, not
   optional. **Build re-verifies the next-free ordinal (0022 if 0021 taken, §0) and regenerates
   the runs `GRANT SELECT(...)` column list from the catalog.**
2. **Lease + operator-session layer** — a Go package owning `defaultHandle`/escalation walk (§2.3,
   §5 pool), lease acquisition + guarded heartbeat renewal (§3), the `operator_sessions`
   create/heartbeat/close lifecycle (§2.6) with graceful release via the dedicated close path that
   **also revokes the operator token** (§C1′).
3. **Operator-token mint** — `mintOperatorSessionToken` (sibling to `mintSessionBoundToken`)
   inserting `operatorSessionCapabilities = {admin, read}`, each a repo-scoped, session-bound
   `client_capabilities` row (§C1′). The shared `sessionBoundCapabilities` slice is **unchanged**.
4. **Operator-bootstrap mint RPC** — a daemon-side mint+lease entry reusing
   `mintOperatorSessionToken` + `link_client_to_principal`, creating the operator session +
   admin-bearing token the CLI presents on `run.prepare` (§1(1), §C1′). `striatum operator
   bootstrap` becomes its client.
5. **Run stamp** — extend the `runs` INSERT: resolve the principal in Go via
   `admin.ResolvePrincipalForClient` and bind it; `created_by_handle_id` via the
   `operator_handles` subquery keyed on `app.session_id` (§1(2)). Delete v1's direct
   `principal_clients` subquery.
6. **Identity reads via projections** — `whose <run-id>` (new read handler over
   `run_origin_identity`, `CapabilityRead`) + `contracts/daemon_methods.json` + regenerated routes
   + `docs/reference/command-authority-matrix.md` row + `registry_contract_test`; `status --mine`
   over `runs_for_origin_client` with bare-id fallback.
7. **Convert the three `runs` star-readers** — `reads/detail.go:73` (run.detail),
   `reads/detail.go:174` (job.detail), `reads/archive.go:118` (`SELECT r.*`) → explicit column
   lists excluding `created_by_principal_id` (§C2″.2d). **Required, or the column-revoke springs a
   `42501`.**
8. **Doctor** — `attribution_unknown` advisory rule (§5) over a daemon-gated NULL-origin scan,
   following `doctor_artifact_anchor.go`.
9. **pgtests** — the ten named two-role tests (§4.5): the C2″ composed-route gate
   (`composed_identity_map_unreadable` + `whose_status_mine_via_projection` +
   `drift_reassert_recloses_routes`), the C1′ stamp gate (`operator_session_pre_run_stamp`), the
   C1″ admin-surface gate (`operator_token_admin_surface`), plus `owner_bundle_applies_clean`,
   `forged_update_created_by_rejected`, `two_live_maya`, `token_revoked_bare_id`, `lease_flap_steal`.
10. **Docs** — update `docs/decisions/decision-log.md`, `docs/reference/spec.md`,
    `docs/reference/command-authority-matrix.md`, `CHANGELOG.md`, and re-triage
    `docs/operator/rfc-roadmap.md` when P0 ships.

This is the published claim. Gate-critical targets: **A35/A36/A37/A38/A39** (C2″ — the composed
read-scope closure, THE decisive blocker, with the composed negative controls and the drift
reassert), **A40/A41/A42/A43** (C1″ — the justified-acceptance admin surface with the trust-root
fence and repo-scope bound), **A29/A7/A27** (R1b sufficiency through the real authorization path),
and **A19′** (the honest REVOKE revision). The §0 corrections, the §E discharge map, and the
§C2″/§C1″ mechanisms are load-bearing; challenge them at source if you believe any is wrong.
