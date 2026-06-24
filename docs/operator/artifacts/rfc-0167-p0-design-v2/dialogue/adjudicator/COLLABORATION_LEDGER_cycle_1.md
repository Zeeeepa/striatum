---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0167-p0-design-v2"
run_id: "run_3bde7bb14a9536d97dcfb72434aa147f"
cycle: 1
topic: "RFC 0167 P0 falsifiable implementation SPEC — REVISION v2 (operator identity & run attribution): discharge v1 C1 (pre-run operator-session substrate) + C2 (identity-projection stamp) without regressing any v1 carry-forward"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "The v2 revision claims it discharges BOTH binding v1 constraints and carries every v1 clearance forward unregressed. C1: a dedicated owner-held operator_sessions table (bundle 0021) is the pre-run, per-terminal liveness + token-binding anchor; it never touches striatumd.sessions, so the sessions.run_id NOT NULL FK and its run-keyed UNIQUE constraints are untouched; the load-bearing source fact is that PostgresAuthorizer/Authorize resolves the bound session purely from client_capabilities.session_id and never joins striatumd.sessions (auth_pg.go:104-156), so a session-bound operator token sets app.session_id at run.prepare with no sessions row; §2.5/A27 then renders maya#7f3 vs theo#7f3 via the operator_session_pre_run_stamp pgtest. C2: the run-origin principal is resolved in Go via admin.ResolvePrincipalForClient -> the SECURITY DEFINER resolve_principal_for_client projection (EXECUTE granted to striatumd_rw, owner/0006:181,186) and bound as $N; the direct principal_clients.principal_id subquery is deleted; no SELECT(principal_id) grant-back is issued; the run_origin_stamp_uses_identity_projection pgtest carries the 42501 direct-read control. Carry-forwards R1a/R1b-architecture/R1c/R2-write-once-trigger/bundle-0021/R3-four-OQs/R4 are asserted INTACT."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "C1 / A27 capability gap — the C1 proof still cannot execute end-to-end. The storage fix is conceded (client_capabilities.session_id is text and PostgresAuthorizer copies it into AuthContext.SessionID without joining striatumd.sessions, auth_pg.go:104-156), but the TOKEN that must carry operator_session_id cannot authorize run.prepare. run.prepare is an admin route (registry_methods.go:110; pinned CapabilityAdmin in registry_rfc0043_test.go:27 + the command-authority matrix), whereas the reused mintSessionBoundToken grants only the fixed sessionBoundCapabilities slice {claim, write, read, review} (session_token.go:23-46) and inserts exactly those into client_capabilities (session_token.go:77-86) — no admin. The dispatcher authorizes BEFORE threading AuthContext into the handler (server.go:107-124), so an operator-session token minted exactly as v2 describes is rejected capability_missing before HandleRunPrepare starts, before the authority prelude can set app.session_id, and before the created_by_handle_id subquery can run. S1->RA and S2->RB stop at authorization; the SPEC never names a capability/lifecycle shape for the operator token (a distinct mint path, or parameterized capabilities), and a naive add of admin to the shared slice would over-grant admin to every supervised lane token. operator_session_pre_run_stamp, as specified, cannot return two NON-NULL DISTINCT created_by_handle_id through the real run.prepare authorization path; it must also assert lane tokens do NOT receive admin and that a closed/expired operator session cannot keep stamping with a stale token."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "C2 / A28 carry-forward regression — the run INSERT is repaired, but the C1 substrate change reopens the bundle-0006 read-scope closure through a new table. operator_sessions carries BOTH principal_id and client_id (§2.6), and 0021 grants table-wide SELECT, INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw (§4.2(5)). After 0021, SELECT client_id, principal_id FROM striatumd.operator_sessions WHERE state='active' succeeds under striatumd_rw, recovering exactly the client_id -> principal_id mapping bundle 0006 deliberately closed on principal_clients: 0006 REVOKEs SELECT on principal_clients and grants back only (client_id, linked_at, unlinked_at), with the explicit rationale that 'without principal_id a leaked runtime credential sees client ids and timestamps, not whose credentials they are' (0006_identity_read_scope.sql:204-221; reasserted owner.go:454-468). The SPEC's 'No SELECT(principal_id) grant-back is issued' is scoped only to principal_clients; the same identity linkage is reopened by another name for active operator sessions. The named tests only assert the DIRECT principal_clients read still fails 42501; none assert the operator_sessions route is closed. This regresses the C2/R4 requirement to reuse RFC 0107/0114 without punching through the read-scope boundary. Fix: column-level grants excluding principal_id/client_id (or a SECURITY DEFINER projection for any identity-bearing liveness read), plus a two-role negative control asserting the operator_sessions client_id+principal_id read fails 42501 while create/heartbeat/close/stamp/render still work."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "C1' (residual; routes to the operator — the single v2 revision cycle is now exhausted): the operator-session token that carries operator_session_id must be authorized to call run.prepare WITHOUT over-granting admin to supervised lane tokens. Name the mechanism explicitly — a distinct operator-token mint path (or a parameterized mintSessionBoundToken capability set), or a narrowly scoped capability that admits run.prepare/run.start — and re-specify operator_session_pre_run_stamp to exercise the REAL run.prepare RPC authorization path with the minted operator token (asserting two NON-NULL DISTINCT created_by_handle_id and whose RA != whose RB), plus controls that (i) ordinary supervised lane session tokens do NOT gain admin and (ii) a closed/expired operator session cannot keep preparing runs (no stale-token NULL stamp / expired-handle reuse)."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "C2' (residual; routes to the operator): bundle 0021 must NOT grant the runtime role a readable operator-session client_id -> principal_id mapping. Replace the table-wide GRANT SELECT ON striatumd.operator_sessions with column-level grants that EXCLUDE principal_id and client_id (keeping the liveness/state columns the runtime needs), or route any identity-bearing operator-session read through a SECURITY DEFINER projection with daemon authority, mirroring resolve_principal_for_client. Add a two-role negative control (to owner_bundle_0021_applies_clean or run_origin_stamp_uses_identity_projection): SELECT client_id, principal_id FROM striatumd.operator_sessions WHERE state='active' as SUTPool/striatumd_rw must fail 42501 (or be impossible because the columns are ungranted), with a sibling positive control proving create/heartbeat/close + run stamp + authorized whose/status --mine still work through the narrower grants."
verdict: "needs_revision"
rationale: "needs_revision — and, because this is the single allowed v2 revision cycle, the gate ends unCleared and routes to the operator. The revision is genuinely strong: it RESOLVED the narrow core of BOTH v1 constraints. C1's v1 storage chicken-and-egg (sessions.run_id NOT NULL FK; session.register run-bound) is correctly fixed by the dedicated owner-held operator_sessions table, and the load-bearing source fact is real — PostgresAuthorizer resolves the bound session purely from client_capabilities.session_id and never joins striatumd.sessions (auth_pg.go:104-156), so app.session_id can be set with no sessions row (Falsifier 1 concedes this). C2's v1 direct-read defect is correctly fixed — the run-origin principal now resolves in Go via admin.ResolvePrincipalForClient through the SECURITY DEFINER resolve_principal_for_client projection (EXECUTE granted to striatumd_rw, owner/0006:181,186) and binds as $N; the direct pc.principal_id subquery is deleted; run_origin_stamp_uses_identity_projection + its 42501 control are specified (Falsifier 2 concedes this). BUT neither constraint is GENUINELY discharged, because each new falsifier challenge is source-verified, material, and unrebutted. (1) C1 — the sufficiency proof still COLLAPSES, now at the authorization layer, not the storage layer: the operator token is minted via the reused mintSessionBoundToken, whose fixed slice {claim,write,read,review} carries no admin (session_token.go:46), while run.prepare requires CapabilityAdmin (registry_methods.go:110) and the dispatcher authorizes before the handler runs (server.go:107-124). So the operator token is rejected capability_missing before app.session_id is set; S1->RA / S2->RB cannot execute; created_by_handle_id is never stamped through the real path; the SPEC names no capability shape for the operator token. C1 remains OPEN (proof still collapses). (2) C2 — the read-scope closure is REGRESSED by the C1 substrate change: 0021 grants table-wide SELECT on operator_sessions (§4.2(5)), a table carrying both client_id and principal_id (§2.6), so striatumd_rw can read the client_id -> principal_id mapping bundle 0006 deliberately closed (0006:204-221; owner.go:454-468). 'No grant-back' is true only for principal_clients; an equivalent grant-back sneaks in by another name. C2 remains OPEN (grant-back) and the R4 RFC-0114-reuse carry-forward is REGRESSED. The objective's clearing rubric requires BOTH C1 and C2 genuinely discharged AND no carry-forward regressed AND no new material challenge standing — all three fail. The remaining carry-forwards are INTACT (R1a honesty; the R1b disambiguation architecture; R1c guarded-UPDATE flap; the R2 BEFORE UPDATE write-once trigger; bundle ordinal 0021 + forward-only/watermark; the four R3 open questions), so the defects are narrow, concrete, and in-P0-buildable — recoverable in a focused revision; this is needs_revision, not reject. Per the gate rule, a second needs_revision exhausts the cycle: the gate is unCleared and the two residual constraints (C1', C2') route to the operator to decide a targeted follow-up or scope adjustment before rfc-0167-p0-build proceeds."
findings:
  - id: F-C1-OPERATOR-TOKEN-CAPABILITY
    severity: critical
    posture: sufficiency_unbuildable
    status: converted_to_constraint
    challenge: "The C1 sufficiency proof requires the operator-session token to call run.prepare so app.session_id is set and created_by_handle_id is stamped, but run.prepare is admin (registry_methods.go:110; registry_rfc0043_test.go:27) while the reused mintSessionBoundToken grants only {claim,write,read,review} (session_token.go:23-46,77-86) and the dispatcher authorizes before the handler (server.go:107-124). The operator token is rejected capability_missing before app.session_id is set, so the two-terminal end-to-end proof cannot execute and created_by_handle_id is never stamped through the real path. The SPEC names no operator-token capability shape, and adding admin to the shared slice would over-grant admin to every supervised lane token. C1 not genuinely discharged."
    source_refs: ["dialogue:2"]
  - id: F-C2-OPERATOR-SESSIONS-GRANTBACK
    severity: critical
    posture: read_scope_regression
    status: converted_to_constraint
    challenge: "The C1 substrate change regresses the C2/R4 read-scope closure. operator_sessions carries both principal_id and client_id (§2.6) and 0021 grants table-wide SELECT to striatumd_rw (§4.2(5)), so SELECT client_id, principal_id FROM striatumd.operator_sessions WHERE state='active' succeeds under the runtime role — recovering the client_id -> principal_id mapping bundle 0006 deliberately closed (REVOKE SELECT ON principal_clients; grant back only (client_id, linked_at, unlinked_at); 0006:204-221; owner.go:454-468). The SPEC's 'no SELECT(principal_id) grant-back' is scoped only to principal_clients; the named tests assert only the direct principal_clients read fails 42501, none assert the operator_sessions route is closed. C2 not genuinely discharged; R4 RFC-0114 reuse regressed."
    source_refs: ["dialogue:3"]
constraints:
  - id: C1-OPERATOR-TOKEN-AUTHORIZATION
    source_finding: F-C1-OPERATOR-TOKEN-CAPABILITY
    posture: sufficiency_unbuildable
    severity: critical
    kind: gate
    binding: true
    text: "Name how the pre-run operator-session token is authorized to call run.prepare (admin route) WITHOUT over-granting admin to supervised lane tokens — a distinct operator-token mint path, a parameterized mintSessionBoundToken capability set, or a narrowly scoped capability admitting run.prepare/run.start. Re-specify operator_session_pre_run_stamp to drive the REAL run.prepare RPC authorization path with the minted operator token."
    source_refs: ["dialogue:2"]
    verification:
      gate: "Two-role pgtest operator_session_pre_run_stamp via the real run.prepare authorization path: two pre-run operator sessions for one human lease two distinct words; one run per session yields two NON-NULL DISTINCT created_by_handle_id and whose RA != whose RB; PLUS controls that (i) ordinary supervised lane session tokens do NOT gain admin and (ii) a closed/expired operator session cannot prepare a run with a stale token (no NULL stamp / expired-handle reuse)."
    final_review_required: true
  - id: C2-OPERATOR-SESSIONS-READ-SCOPE
    source_finding: F-C2-OPERATOR-SESSIONS-GRANTBACK
    posture: read_scope_regression
    severity: critical
    kind: gate
    binding: true
    text: "Bundle 0021 must not give striatumd_rw a readable operator-session client_id -> principal_id mapping. Replace the table-wide GRANT SELECT ON striatumd.operator_sessions with column-level grants excluding principal_id and client_id (keeping the liveness/state columns), or route any identity-bearing operator-session read through a SECURITY DEFINER projection with daemon authority. Keep the run-origin stamp's Go/projection route and the handle subquery unchanged."
    source_refs: ["dialogue:3"]
    verification:
      gate: "Two-role negative control (in owner_bundle_0021_applies_clean or run_origin_stamp_uses_identity_projection): SELECT client_id, principal_id FROM striatumd.operator_sessions WHERE state='active' as striatumd_rw fails 42501 (or is impossible — columns ungranted); sibling positive control proves operator-session create/heartbeat/close + run stamp + authorized whose/status --mine still work through the narrower grants."
    final_review_required: true
branches:
  sufficiency_unbuildable: "blocked"
  read_scope_regression: "blocked"
---

# Collaboration Ledger — RFC 0167 P0 design v2 (cycle 1)

**Verdict: `needs_revision`.** This was the single allowed v2 revision cycle, so
the falsification gate now ends **unCleared** and routes the two residual
constraints (C1′, C2′) to the operator. The v2 revision is genuinely strong and
*resolved the narrow core of both v1 constraints* — but each falsifier landed a
new, source-verified, unrebutted material challenge, so neither C1 nor C2 is
**genuinely discharged**, and the R4 read-scope carry-forward regressed.

## What the revision genuinely fixed (conceded by the falsifiers)

- **C1 storage gap → RESOLVED.** The v1 `sessions.run_id NOT NULL` FK
  chicken-and-egg is correctly dissolved by a dedicated owner-held
  `operator_sessions` table that never touches `striatumd.sessions`. The
  load-bearing source fact is real: `PostgresAuthorizer.Authorize` resolves the
  bound session purely from `client_capabilities.session_id` and never joins
  `striatumd.sessions` (`auth_pg.go:104-156`), so a session-bound token can set
  `app.session_id` with no `sessions` row. Falsifier 1 explicitly concedes this.
- **C2 direct-read defect → RESOLVED.** The run-origin principal now resolves in
  Go via `admin.ResolvePrincipalForClient` → the `SECURITY DEFINER`
  `resolve_principal_for_client` projection (EXECUTE granted to `striatumd_rw`,
  `owner/0006:181,186`) and binds as `$N`; the direct `pc.principal_id` subquery
  is deleted; `run_origin_stamp_uses_identity_projection` + its `42501` control
  are specified. Falsifier 2 explicitly concedes this.

## Why the gate still does not clear

| Constraint | v1 core | New challenge (v2) | Disposition |
|-----------|---------|--------------------|-------------|
| **C1** | storage RESOLVED | **Operator token cannot authorize `run.prepare`.** `run.prepare` is admin (`registry_methods.go:110`); the reused `mintSessionBoundToken` grants only `{claim,write,read,review}` (`session_token.go:23-46,77-86`), no admin; the dispatcher authorizes before the handler (`server.go:107-124`). The token is rejected `capability_missing` before `app.session_id` is set → the two-terminal proof cannot run → `created_by_handle_id` never stamped. SPEC names no operator-token capability shape. | **OPEN — proof still collapses (authorization layer).** F1 stands. |
| **C2** | stamp RESOLVED | **Read-scope closure regressed by the C1 substrate.** `operator_sessions` carries `client_id`+`principal_id` (§2.6) and 0021 grants table-wide `SELECT` (§4.2(5)), so `striatumd_rw` can read the `client_id → principal_id` mapping bundle 0006 closed (`0006:204-221`; `owner.go:454-468`). "No grant-back" is true only for `principal_clients`; an equivalent grant-back sneaks in by another name. | **OPEN — grant-back; R4 reuse regressed.** F2 stands. |

## Per-requirement disposition

| Req | Demand | Status | Basis |
|-----|--------|--------|-------|
| **R1a** | identity bound server-side at token-mint; no client-name/display-signal path | **INTACT** | §1 A1–A5: stamp from the live-token prelude GUC / Go-resolved principal, never an envelope/tty/tmux/title/env; reads are pure PG joins. No spoof path found. |
| **R1b** | SUFFICIENCY PROVEN — two same-human terminals return two DISTINCT `whose` answers via a per-session handle | **OPEN** | The disambiguation *architecture* is INTACT and correct (per-session leased handle + live-unique partial index + write-once `created_by_handle_id` snapshot + run→handle join). The *proof* still collapses: the operator token lacks `admin` for `run.prepare` (F1), so `created_by_handle_id` is never stamped through the real path. **C1.** |
| **R1c** | heartbeat renews via guarded UPDATE, never release-then-reacquire | **INTACT** | §3 A12: guarded `WHERE leased_session_id=$ AND released_at IS NULL`; mirrors the `striatumd.leases` idiom. Unchallenged. |
| **R2** | owner-bundle two-role safety; DB write-once; retained privileges; named pgtests | **PARTIAL / OPEN** | DB-write-once face INTACT (BEFORE UPDATE trigger, `0010` precedent; ordinal 21 — `LatestOwnerBundleVersion==20` verified; forward-only/watermark sound). The run-origin stamp now passes the two-role fixture via the projection. BUT bundle 0021's `operator_sessions` GRANT reopens the identity mapping (F2). **C2.** |
| **R3** | four open questions resolved | **INTACT** | OQ1 curated pool + deterministic default/escalation + denylist; OQ2 NULL + advisory `attribution_unknown`, no backfill (C-4); OQ3 per-repo P0 / P3 deferred; OQ4 byline P2. Unchallenged. |
| **R4** | ride RFC 0107/0114; no parallel identity table; no read-scope punch-through; P0-only | **REGRESSED** | FK rendering layer, no parallel identity, opaque `run_id`, and the projection-routed *stamp* are clean — but the new `operator_sessions` table-wide `SELECT` punches back through the RFC 0114 read-scope closure bundle 0006 maintained (F2). **C2.** |

## Per-falsifier disposition

- **Falsifier 1 — C1 / A27 operator-token capability gap (dialogue:2): MATERIAL, STANDING.**
  Verified at source: `run.prepare` → `CapabilityAdmin` (`registry_methods.go:110`);
  `sessionBoundCapabilities = {claim,write,read,review}` (`session_token.go:46`),
  inserted verbatim (`session_token.go:77-86`); authorize-before-route
  (`server.go:107-124`). The reused mint cannot produce a token that prepares a
  run, so the sufficiency proof cannot execute end-to-end. Not rebutted. → **C1′.**
- **Falsifier 2 — C2 / A28 operator_sessions grant-back (dialogue:3): MATERIAL, STANDING.**
  Verified at source: 0006 `REVOKE SELECT ON principal_clients … GRANT SELECT
  (client_id, linked_at, unlinked_at)` with the explicit "whose credentials they
  are" rationale (`0006:204-221`), reasserted `owner.go:454-468`; v2 §4.2(5)
  grants table-wide `SELECT` on `operator_sessions` (which carries `client_id` +
  `principal_id`, §2.6). The identity mapping is recoverable by the runtime role.
  Not rebutted. → **C2′.**

## Gate outcome

Both gate-critical requirements still carry a standing, source-verified material
challenge; the R4 read-scope carry-forward regressed. Per the objective's clearing
rubric, this is **not** a clearing verdict. Because the v2 revision cycle is the
single allowed one, the gate is now **exhausted and unCleared**: the two residual
constraints route to the **operator**, who decides a targeted follow-up (the two
fixes are narrow, concrete, and in-P0-buildable — a distinct operator-token
capability path for C1′; column-scoped/projection-gated `operator_sessions` reads
for C2′) or a scope adjustment before `rfc-0167-p0-build` proceeds. This is
`needs_revision`, not `reject`: the architecture is sound and both defects are
mechanically recoverable.
