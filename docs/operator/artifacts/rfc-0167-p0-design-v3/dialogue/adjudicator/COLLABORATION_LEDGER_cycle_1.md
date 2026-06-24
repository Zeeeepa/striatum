---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0167-p0-design-v3"
run_id: "run_49a3891779491dd139b98c46a6ab2160"
cycle: 1
topic: "RFC 0167 P0 falsifiable implementation SPEC — REVISION v3 (operator identity & run attribution): discharge v2 C1′ (operator-session token authorizes run.prepare without over-granting lane admin) + C2′ (operator_sessions client_id→principal_id mapping unreadable by the runtime) without regressing any v1+v2 carry-forward"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "The v3 revision claims it discharges BOTH residual v2 constraints and carries every v1+v2 clearance forward unregressed. C1′: a DISTINCT operator-token mint, mintOperatorSessionToken (the composition of admin.insertTokenClient's caller-supplied capability set, tokens.go:286-315, with mintSessionBoundToken's session binding, session_token.go:60-97), inserts operatorSessionCapabilities = {admin, read}, each a client_capabilities row carrying session_id = operator_session_id; the admin row clears the exact-match Authorize (auth_pg.go:104-117) for run.prepare (CapabilityAdmin, registry_methods.go:110), the prelude sets app.session_id, and created_by_handle_id is stamped — so the two-terminal proof executes through the REAL run.prepare authorization path (maya#7f3 vs theo#7f3). The shared sessionBoundCapabilities lane slice {claim,write,read,review} is UNCHANGED so lane tokens cannot gain admin; graceful close revokes the operator token (client_capabilities.revoked_at) + releases the handle in one txn, and TTL expiry bounds it, so a closed/expired operator session cannot stamp. The holder pre-emptively rebuts the over-grant worry: admin is 'faithful, not an over-grant' because the human already holds admin via the static admin token and the session-bound token is a TTL-bounded, revocable, strictly-less-standing materialization, and run.pause/resume/cancel/retry_job are also admin and all operator-driven so 'exactly prepare/start' is too narrow. C2′: bundle 0021 grants operator_sessions COLUMN-scoped SELECT excluding principal_id AND client_id (no REVOKE — the table is new in 0021), keeping only liveness/state columns; operator_handles gets FULL SELECT on the argument that it 'has no client_id column → no identity map'; a negative control asserts SELECT client_id, principal_id FROM operator_sessions fails 42501 and a positive control proves create/heartbeat/close/stamp/whose/status --mine still work. Carry-forwards R1a, the R1b architecture, the v2 C1 storage substrate, the v2 C2 projection stamp, R1c, the R2 write-once trigger, bundle ordinal 0021, the four open questions, and R4 reuse are asserted INTACT."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "C1′ / A29-A32 — the authorization gap is closed but a NEW over-grant gap opens, and the seed made over-grant a gate question. admin is NOT scoped to run.prepare/run.start: the registry maps the same CapabilityAdmin to workflow.accept_risk, review.override, decision.record, checkpoint.resolve, verifier.attest, branch.confirm, run.prepare, run.start, run.pause/resume/cancel/retry_job, and repo.init (registry_methods.go:102-117; pinned subset registry_rfc0043_test.go:19-31). The dispatcher authorizes only by required capability before routing (server.go:107-124) and the authorizer accepts any non-revoked repo-scoped row whose capability=$2 (auth_pg.go:104-156) — there is NO method allowlist or 'operator token may only call run.prepare/start' check. So the {admin, read} operator token is strictly broader than the C1′ proof needs: the same token that clears run.prepare also clears review.override (mutations/review.go:172-328), checkpoint.resolve (mutations/operator.go:197-360), workflow.accept_risk (workflow_accepted_risk.go:15-97), and branch.confirm (run.go:818-899) — none of which refuse a session-bound token. The codebase ALREADY has the gap-closing pattern: verifier.attest is admin-routed but explicitly refuses any session-bound token (verifier_attest.go:13-33,49-59), with the comment that the refusal 'holds even for a hypothetical admin-capable session token' — i.e. session-bound admin is a recognized threat. The v3 SPEC creates exactly that token (and, per the ~15-terminals RFC, in MULTIPLES — one session-bound admin credential per operator terminal) and adds no equivalent refusal/scope control; its only control proves a LANE token cannot call run.prepare, which is too narrow. The holder's 'human already holds static admin' rebuttal does not discharge this specific proof obligation: a single static operator secret is replaced by N session-bound admin tokens, each session-bound = each lane-impersonable in the codebase's own threat model. Refuting test the SPEC lacks: with a valid minted operator-session token, attempt review.override/checkpoint.resolve/workflow.accept_risk/branch.confirm and require typed capability_denied/route-not-allowed, OR explicitly accept+justify the full admin surface as the P0 security boundary. Until then C1′ is only partially discharged — the stamp proof passes while the operator token still carries a strictly broader repo-admin surface than run-origin stamping requires."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "C2′ / A33-A34 — the direct operator_sessions negative control is too narrow; striatumd_rw still reconstructs client_id→principal_id by a COMPOSED route the v3 design itself builds. SELECT DISTINCT cc.client_id, oh.principal_id FROM client_capabilities cc JOIN operator_handles oh ON oh.leased_session_id = cc.session_id WHERE cc.session_id IS NOT NULL AND cc.revoked_at IS NULL AND oh.released_at IS NULL succeeds under the runtime role. Both edges are runtime-readable: (1) client_capabilities keeps table SELECT — 0005 grants SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw and revokes only events/artifacts UPDATE/DELETE (0005:467-472); it carries client_id (0001_baseline.sql:57-66) + session_id (0022_session_bound_capability.sql:12-15); owner bundle 0006 closes client_sessions/principals/principal_clients but NOT client_capabilities (0006:212-221), and read_authority_inventory.go:52-56 still classifies client_capabilities as a sensitive broad SELECT surface. (2) v3 grants operator_handles FULL SELECT (HOLDER.md:653-654,686-688) carrying leased_session_id + principal_id (HOLDER.md:470-478). The v3 token design deliberately binds client_capabilities.session_id = operator_session_id = operator_handles.leased_session_id (HOLDER.md:204-218,887-891), so the join edge exists by construction. This is EXACTLY the client_id→principal_id mapping bundle 0006 deliberately closed ('a leaked runtime credential sees client ids and timestamps, not whose credentials they are', 0006:207-221). The named A33 control only asserts the DIRECT SELECT client_id, principal_id FROM operator_sessions fails — it misses the join and produces a FALSE clearing signal; A34 also still passes because create/heartbeat/close/stamp/whose/status do not require closing the join. C2′ was explicitly 'by ANY route', not one table; the proof must reason over the composed ACL graph. Refuting test the SPEC lacks: a striatumd_rw negative control on the composed cc⋈oh query that must fail 42501 / return no identity rows / be structurally impossible. Buildable fixes: column-scope operator_handles SELECT to exclude principal_id or leased_session_id, route identity-bearing handle reads through a SECURITY DEFINER projection, or close/project the client_capabilities.session_id read surface. Until one is specified, C2′ remains open and the R4 read-scope carry-forward is regressed even though the direct operator_sessions control passes."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "C1″ (residual; the v3 cycle is the single allowed one, so this routes to the operator): the pre-run operator-session token must not carry repo-admin authority strictly broader than run.prepare/run.start (run-lifecycle) need. The v3 mint authorizes run.prepare correctly (that core v2 gap IS resolved) but grants coarse CapabilityAdmin, which the dispatcher/authorizer admit for the whole repo-admin surface (review.override, checkpoint.resolve, decision.record, workflow.accept_risk, branch.confirm, repo.init) with no method allowlist (registry_methods.go:102-117; server.go:107-124; auth_pg.go:104-156), and the codebase's own verifier.attest precedent (verifier_attest.go:49-59) treats session-bound admin as a threat requiring explicit refusal. The revision must EITHER (a) introduce a narrower operator capability (or method allowlist) admitting exactly the run-lifecycle routes, OR (b) add daemon-enforced session-bound-token refusals on the non-run-preparation admin routes mirroring verifier.attest, OR (c) explicitly accept and justify the full repo-admin surface on the per-terminal session-bound operator token as the P0 security boundary, with an analysis of the N-tokens blast radius. Add a C1″ control: with a valid minted operator-session token, the representative non-stamping admin routes either fail typed capability_denied/route-not-allowed or are documented as an accepted, justified part of the P0 boundary."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "C2″ (residual; routes to the operator): bundle 0021 must not let striatumd_rw reconstruct the operator-session client_id→principal_id mapping by ANY route, including the composed client_capabilities ⋈ operator_handles join the v3 design itself enables (client_capabilities runtime SELECT supplies client_id→session_id; v3's full operator_handles SELECT supplies session_id→principal_id; the token's session_id = operator_handles.leased_session_id closes the loop). Column-scoping operator_sessions alone is necessary but NOT sufficient. The revision must close the composed route — e.g. column-scope operator_handles direct SELECT to exclude principal_id or leased_session_id, route identity-bearing handle reads through a SECURITY DEFINER projection with daemon authority, or close/project the client_capabilities.session_id read surface — and ADD a two-role negative control under striatumd_rw on the composed cc⋈oh query asserting it fails 42501 / returns no identity-bearing rows / is structurally impossible (at least one join edge not directly runtime-readable), with the positive control still proving create/heartbeat/close/stamp/whose/status --mine work. The direct operator_sessions negative control as specified is a false-clearing signal."
verdict: "needs_revision"
rationale: "needs_revision — and because the single allowed v3 revision cycle is now exhausted, the falsification gate ends unCleared and routes the two residual constraints (C1″, C2″) to the operator. The v3 revision is genuinely strong and RESOLVED the narrow core of BOTH v2 constraints: (C1′ core) the operator token now AUTHORIZES run.prepare end-to-end — a distinct mintOperatorSessionToken inserts {admin, read} (reusing insertTokenClient's caller-supplied set, tokens.go:286-315, + the session binding of session_token.go:60-97), the admin row clears the exact-match Authorize for run.prepare (auth_pg.go:104-117; registry_methods.go:110, VERIFIED), the lane slice {claim,write,read,review} is untouched (session_token.go:46) so lane tokens cannot gain admin, and close-revoke + TTL expiry close the stale-token stamp; the two-terminal proof can now execute; Falsifier 1 concedes this direction. (C2′ core) the table-wide operator_sessions grant-back is gone — the direct SELECT client_id, principal_id FROM operator_sessions fails 42501 under the column-scoped grant; Falsifier 2 concedes this. BUT neither constraint is GENUINELY discharged, because each falsifier landed a NEW, source-verified, material, unrebutted challenge. (1) C1′ — the operator-session token carries coarse CapabilityAdmin, which the registry maps to the whole repo-admin surface (review.override/checkpoint.resolve/decision.record/workflow.accept_risk/branch.confirm/repo.init, registry_methods.go:102-117) and the dispatcher/authorizer admit with no method allowlist (server.go:107-124; auth_pg.go:104-156). The seed explicitly asked whether the fix over-grants 'strictly more than run.prepare/start need' — it does. The codebase's own verifier.attest refuses session-bound admin tokens precisely because they are a recognized threat 'even for a hypothetical admin-capable session token' (verifier_attest.go:49-59); the v3 SPEC mints exactly such tokens (in multiples, one per operator terminal) and adds neither the analogous refusal nor a justified acceptance, and its control proves only lane-denial, not operator-token-scope. C1′ remains OPEN on the over-grant axis (the holder's 'human already holds static admin' rebuttal does not dispose of N session-bound admin credentials). (2) C2′ — the read-scope closure is REGRESSED by a route the v3 design itself constructs: client_capabilities stays runtime-readable (SELECT on ALL TABLES, 0005:467-472; client_id 0001:57-66 + session_id 0022:12-15; not closed by 0006:212-221; flagged sensitive at read_authority_inventory.go:52-56), v3 grants operator_handles FULL SELECT (carrying leased_session_id + principal_id), and the token binds session_id = operator_session_id = leased_session_id — so SELECT cc.client_id, oh.principal_id FROM client_capabilities cc JOIN operator_handles oh ON oh.leased_session_id = cc.session_id reconstructs the exact mapping 0006 closed. C2′ was explicitly 'by ANY route'; the named negative control tests only the direct operator_sessions read and misses the join — a false-clearing signal. C2′ remains OPEN and the R4 read-scope carry-forward is REGRESSED. The objective's clearing rubric requires BOTH C1′ and C2′ GENUINELY discharged AND no carry-forward regressed AND no new material challenge standing — all three fail (C2′ alone is decisive: the seed lists 'client_id+principal_id still readable' as a needs_revision trigger, and it is readable via the composed route). The remaining carry-forwards are INTACT (R1a honesty; the R1b disambiguation architecture, now executable through the real authorization path; the v2 C1 storage substrate; the v2 C2 projection stamp; R1c guarded-UPDATE flap; the R2 BEFORE UPDATE write-once trigger; bundle ordinal 0021 / forward-only / watermark; the four R3 open questions), so the defects are narrow, concrete, and in-P0-buildable — recoverable in a focused follow-up; this is needs_revision, not reject. Per the gate rule, this second needs_revision (v2 was the first) exhausts the cycle: the gate is unCleared and the two residual constraints (C1″ operator-token scope; C2″ composed read-scope closure) route to the operator to decide a targeted follow-up or scope adjustment before rfc-0167-p0-build proceeds."
findings:
  - id: F-C1PRIME-OPERATOR-TOKEN-OVERGRANT
    severity: critical
    posture: privilege_surface_overgrant
    status: converted_to_constraint
    challenge: "The C1′ authorization gap is closed (the {admin, read} operator-session token authorizes run.prepare and the two-terminal stamp proof executes), but the token carries coarse CapabilityAdmin, which the registry maps to the entire repo-admin surface — workflow.accept_risk, review.override, decision.record, checkpoint.resolve, branch.confirm, repo.init in addition to run.prepare/start (registry_methods.go:102-117; pinned registry_rfc0043_test.go:19-31) — and the dispatcher authorizes only by required capability with no method allowlist (server.go:107-124; auth_pg.go:104-156). It is strictly broader than run.prepare/start need. The codebase's verifier.attest already treats session-bound admin tokens as a threat requiring an explicit IsSessionBound() refusal 'even for a hypothetical admin-capable session token' (verifier_attest.go:49-59); the v3 SPEC mints exactly such tokens — and, per the ~15-terminals RFC, in multiples (one per operator terminal) — with no analogous refusal, no narrower capability, and no justified acceptance. The C1′ control only proves a lane token cannot call run.prepare; it does not prove the operator token is scoped to the run-lifecycle authority the proof requires. C1′ not genuinely discharged on the over-grant axis."
    affected_invariants: ["C1-OPERATOR-TOKEN-AUTHORIZATION", "R1b-sufficiency", "R4-least-privilege"]
    source_refs: ["dialogue:2"]
  - id: F-C2PRIME-COMPOSED-IDENTITY-MAP
    severity: critical
    posture: read_scope_regression
    status: converted_to_constraint
    challenge: "The v3 column-scoped operator_sessions grant closes the DIRECT client_id→principal_id read, but striatumd_rw reconstructs the same mapping through a COMPOSED route the v3 design itself builds: SELECT cc.client_id, oh.principal_id FROM client_capabilities cc JOIN operator_handles oh ON oh.leased_session_id = cc.session_id. client_capabilities stays runtime-readable (SELECT on ALL TABLES, 0005:467-472; client_id 0001_baseline.sql:57-66 + session_id 0022_session_bound_capability.sql:12-15; not closed by owner bundle 0006:212-221; classified sensitive at read_authority_inventory.go:52-56); v3 grants operator_handles FULL SELECT (HOLDER.md:653-654,686-688) carrying leased_session_id + principal_id (HOLDER.md:470-478); and the token design binds client_capabilities.session_id = operator_session_id = operator_handles.leased_session_id (HOLDER.md:204-218,887-891), closing the join. This recovers exactly the client_id→principal_id mapping bundle 0006 deliberately closed (0006:207-221). The named A33 negative control tests only the direct operator_sessions read and misses the join, producing a false clearing signal; A34 still passes. C2′ was explicitly 'by ANY route'. C2′ not genuinely discharged; the R4/RFC-0114 read-scope carry-forward is regressed."
    affected_invariants: ["C2-OPERATOR-SESSIONS-READ-SCOPE", "R4-read-scope-closure", "RFC-0114-bundle-0006"]
    source_refs: ["dialogue:3"]
constraints:
  - id: C1PRIME2-OPERATOR-TOKEN-SCOPE
    source_finding: F-C1PRIME-OPERATOR-TOKEN-OVERGRANT
    posture: privilege_surface_overgrant
    severity: critical
    kind: gate
    binding: true
    text: "The pre-run operator-session token must not carry repo-admin authority strictly broader than the run-lifecycle (run.prepare/start/pause/resume/cancel/retry_job) the operator role needs. EITHER introduce a narrower operator capability / method allowlist admitting exactly those routes, OR add daemon-enforced session-bound-token refusals on the non-run-preparation admin routes (review.override, checkpoint.resolve, decision.record, workflow.accept_risk, branch.confirm, repo.init) mirroring verifier.attest's IsSessionBound() refusal, OR explicitly accept and justify the full repo-admin surface on the per-terminal session-bound operator token as the P0 security boundary with an N-tokens blast-radius analysis. The authorization mechanism (distinct mint, lane slice untouched, close/expiry revoke) is otherwise discharged and must be kept."
    source_refs: ["dialogue:2"]
    verification:
      gate: "operator_session_pre_run_stamp (or a sibling control): a valid minted operator-session token CAN call run.prepare (NON-NULL DISTINCT created_by_handle_id, whose RA != whose RB) AND, with that same token, representative non-stamping admin routes (review.override, checkpoint.resolve, workflow.accept_risk, branch.confirm) either FAIL typed capability_denied/route-not-allowed for session-bound operator tokens, or are documented as an explicitly accepted+justified part of the P0 security boundary; a lane token still cannot call run.prepare; a closed/expired operator session cannot stamp."
    final_review_required: true
  - id: C2PRIME2-COMPOSED-READ-SCOPE
    source_finding: F-C2PRIME-COMPOSED-IDENTITY-MAP
    posture: read_scope_regression
    severity: critical
    kind: gate
    binding: true
    text: "Bundle 0021 must not let striatumd_rw reconstruct the operator-session client_id→principal_id mapping by ANY route, including the composed client_capabilities ⋈ operator_handles join the v3 design enables. Column-scoping operator_sessions is necessary but insufficient: close the composed route by column-scoping operator_handles direct SELECT to exclude principal_id or leased_session_id, routing identity-bearing handle reads through a SECURITY DEFINER projection with daemon authority, or closing/projecting the client_capabilities.session_id read surface. Keep the v2 C2 projection stamp and the direct operator_sessions column gate."
    source_refs: ["dialogue:3"]
    verification:
      gate: "Two-role negative control under striatumd_rw on SELECT DISTINCT cc.client_id, oh.principal_id FROM client_capabilities cc JOIN operator_handles oh ON oh.leased_session_id = cc.session_id WHERE cc.session_id IS NOT NULL AND cc.revoked_at IS NULL AND oh.released_at IS NULL — must fail 42501, return no identity-bearing rows, or be structurally impossible (at least one join edge not directly runtime-readable); PLUS the existing direct operator_sessions negative control; PLUS a positive control proving operator-session create/heartbeat/close + run stamp + authorized whose/status --mine still work."
    final_review_required: true
branches:
  privilege_surface_overgrant: "blocked"
  read_scope_regression: "blocked"
---

# Collaboration Ledger — RFC 0167 P0 design v3 (cycle 1)

**Verdict: `needs_revision`.** This was the single allowed v3 revision cycle, so the
falsification gate now ends **unCleared** and routes the two residual constraints
(C1″, C2″) to the operator. The v3 revision is genuinely strong — it *resolved the
narrow core of both v2 constraints* (the operator token now authorizes `run.prepare`;
the direct `operator_sessions` grant-back is gone) — but each falsifier landed a new,
source-verified, unrebutted material challenge, so neither C1′ nor C2′ is **genuinely
discharged**, and the R4 read-scope carry-forward regressed by a new route.

## What the revision genuinely fixed (conceded by the falsifiers)

- **C1′ authorization gap → core RESOLVED.** A distinct `mintOperatorSessionToken`
  (reusing `insertTokenClient`'s caller-supplied capability set, `tokens.go:286-315`, +
  `mintSessionBoundToken`'s session binding, `session_token.go:60-97`) inserts
  `{admin, read}`. The `admin` row clears the **exact-match** `Authorize`
  (`auth_pg.go:104-117`) for `run.prepare` (`CapabilityAdmin`, `registry_methods.go:110`,
  VERIFIED); the prelude sets `app.session_id`; `created_by_handle_id` is stamped through
  the **real** authorization path. The shared lane slice `{claim,write,read,review}` is
  **untouched** (`session_token.go:46`), so lane tokens still cannot gain admin; close
  revokes the token + releases the handle, and TTL expiry bounds it. Falsifier 1 concedes
  this direction ("a distinct mint path is better than widening `sessionBoundCapabilities`,
  and the `{admin, read}` token should make the two-terminal proof execute").
- **C2′ direct grant-back → RESOLVED.** Bundle 0021's `operator_sessions` SELECT is
  column-scoped to exclude `principal_id` **and** `client_id`, with no `REVOKE` (the table
  is new in 0021, so A19 stays intact). The direct `SELECT client_id, principal_id FROM
  operator_sessions` fails `42501`. Falsifier 2 concedes this ("the obvious table-wide
  `operator_sessions` grant-back is gone").

## Why the gate still does not clear

| Constraint | v2 core | New challenge (v3) | Disposition |
|-----------|---------|--------------------|-------------|
| **C1′** | authorization RESOLVED | **Operator token over-grants the full repo-admin surface.** `{admin, read}` is coarse `CapabilityAdmin`, which the registry maps to `review.override`, `checkpoint.resolve`, `decision.record`, `workflow.accept_risk`, `branch.confirm`, `repo.init` as well as `run.prepare/start` (`registry_methods.go:102-117`); the dispatcher/authorizer admit it with no method allowlist (`server.go:107-124`; `auth_pg.go:104-156`). The codebase's own `verifier.attest` refuses session-bound admin tokens as a recognized threat (`verifier_attest.go:49-59`); the SPEC mints exactly such tokens (one per operator terminal) with no scope control or justified acceptance. The C1′ control proves only lane-denial. | **OPEN — over-grant; control too narrow.** F1 stands. |
| **C2′** | direct grant-back RESOLVED | **Read-scope closure regressed by a composed route the v3 design builds.** `client_capabilities` (runtime SELECT; `client_id` + `session_id`) ⋈ `operator_handles` (full v3 SELECT; `leased_session_id` + `principal_id`) on `session_id` reconstructs `client_id → principal_id` — the mapping 0006 closed. The token binds `session_id = operator_session_id = leased_session_id` by construction. The named negative control tests only the direct `operator_sessions` read and misses the join (false-clearing signal). | **OPEN — composed grant-back; R4 reuse regressed.** F2 stands. |

## Per-requirement disposition

| Req | Demand | Status | Basis |
|-----|--------|--------|-------|
| **R1a** | identity bound server-side at token-mint; no client-name/display-signal path | **INTACT** | §1 A1–A5: stamp from the live-token prelude GUC / Go-resolved principal; reads are pure PG joins. Unchallenged. |
| **R1b** | SUFFICIENCY PROVEN — two same-human terminals return two DISTINCT `whose` via a per-session handle on a buildable + **authorizable** substrate | **OPEN** | The architecture is INTACT and the proof now *executes* through the real `run.prepare` authorization path (C1′ core fixed). But the authorizing token over-grants the full repo-admin surface (F1). **C1″.** |
| **R1c** | heartbeat renews via guarded UPDATE, never release-then-reacquire | **INTACT** | §3 A12 guarded `WHERE leased_session_id=$ AND released_at IS NULL`. Unchallenged. |
| **R2** | owner-bundle two-role safety; DB write-once; retained privileges; named pgtests | **PARTIAL / OPEN** | DB-write-once face INTACT (BEFORE UPDATE trigger; ordinal 21; forward-only/watermark sound; no REVOKE → A19 intact). The projection stamp passes two-role. BUT 0021's grants leave the composed `cc ⋈ oh` identity route open (F2). **C2″.** |
| **R3** | four open questions resolved | **INTACT** | OQ1 pool/default/escalation/denylist; OQ2 NULL+advisory, no backfill; OQ3 per-repo P0 / P3 deferred; OQ4 byline P2. Unchallenged. |
| **R4** | ride RFC 0107/0114; no parallel identity table; no read-scope punch-through; P0-only | **REGRESSED** | FK rendering layer, no parallel identity, opaque `run_id`, projection-routed stamp are clean — but the new full `operator_handles` SELECT + the token's `session_id` binding reopen the RFC 0114 read-scope closure via the composed join (F2). **C2″.** |

## Per-falsifier disposition

- **Falsifier 1 — C1′ operator-token over-grant (dialogue:2): MATERIAL, STANDING.**
  Verified at source: `CapabilityAdmin` covers the whole repo-admin route set
  (`registry_methods.go:102-117`, confirmed lines 102/105/106/107/108/109/110/111/117);
  authorize-by-capability-only (`server.go:107-124`); exact-match accept of any non-revoked
  admin row (`auth_pg.go:104-156`); and the `verifier.attest` session-bound-admin refusal
  precedent (`verifier_attest.go:49-59`, comment "holds even for a hypothetical
  admin-capable session token"). The seed explicitly listed over-grant as a gate question.
  The holder's pre-emptive "human already holds static admin" rebuttal does not dispose of
  N per-terminal session-bound admin tokens. Not rebutted. → **C1″.**
- **Falsifier 2 — C2′ composed identity-map route (dialogue:3): MATERIAL, STANDING.**
  Verified at source: `client_capabilities` retains runtime SELECT (`0005:467-472` grants
  `SELECT … ON ALL TABLES`, revokes only `events`/`artifacts` UPDATE/DELETE), carries
  `client_id` (`0001_baseline.sql:57-66`) + `session_id` (`0022:12-15`), is **not** closed
  by 0006 (`0006:212-221` closes `client_sessions`/`principals`/`principal_clients` only),
  and is flagged a sensitive runtime SELECT surface (`read_authority_inventory.go:52-56`);
  v3 grants `operator_handles` full SELECT carrying `leased_session_id` + `principal_id`;
  the token binds `session_id = operator_session_id = leased_session_id`. The composed join
  recovers `client_id → principal_id`. The direct `operator_sessions` control misses it.
  Not rebutted. → **C2″.**

## Gate outcome

Both gate-critical constraints still carry a standing, source-verified material challenge;
the R4 read-scope carry-forward regressed. Per the objective's clearing rubric this is
**not** a clearing verdict — C2′ alone is decisive (the seed lists "client_id+principal_id
still readable" as a `needs_revision` trigger, and it is readable via the composed route).
Because the v3 revision cycle is the single allowed one, the gate is now **exhausted and
unCleared**: the two residual constraints route to the **operator**, who decides a targeted
follow-up (a narrower operator capability or session-bound refusals / justified acceptance
for **C1″**; closing the composed `client_capabilities ⋈ operator_handles` route for
**C2″**) or a scope adjustment before `rfc-0167-p0-build` proceeds. This is
`needs_revision`, not `reject`: the architecture is sound, the v2 cores are genuinely
fixed, and both residual defects are narrow and mechanically recoverable.
