---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "adjudicated_constraint_extraction"
author: adjudicator-codex-gpt-5.5-xhigh-002
workflow: "rfc-0110-pg-auth-panel"
run_id: "run_8e14cb48342e929d30043d6be24f9101"
cycle: 1
topic: "RFC 0110 daemon to PostgreSQL authentication and database-enforced write boundary"
participants:
  - convener
  - product_cross_examiner
  - implementation_cross_examiner
  - privacy_cross_examiner
  - eval_cross_examiner
  - operations_cross_examiner
  - adjudicator
entries:
  - kind: claim
    by: convener
    refs: ["dialogue:1"]
    text: "The candidate synthesis presents RFC 0110 as an implementation-ready L0-L3 plan with v3 audit hashing, revoked direct table DML, owner-owned write functions, L2 socket hardening, and L3 attribution."
  - kind: challenge
    by: product_cross_examiner
    refs: ["dialogue:2"]
    text: "The product posture challenges direct EXECUTE on write functions by a leaked runtime DSN, spoofable GUC/RLS authority labels, and the unresolved secure-adoption default for L2."
  - kind: challenge
    by: implementation_cross_examiner
    refs: ["dialogue:3"]
    text: "The implementation posture challenges transaction-local GUC placement, grant drift, and SECURITY DEFINER hardening."
  - kind: challenge
    by: privacy_cross_examiner
    refs: ["dialogue:4"]
    text: "The privacy posture challenges credential heap lifetime, daemon_auth_log redaction, and durable event transcript exclusion."
  - kind: challenge
    by: eval_cross_examiner
    refs: ["dialogue:5"]
    text: "The eval posture challenges harness privilege fidelity, negative L2 isolation testing, and attribution reset robustness."
  - kind: challenge
    by: operations_cross_examiner
    refs: ["dialogue:6"]
    text: "The operations posture challenges restart owner dependency, single-role L0 no-op visibility, v3 rollback, owner DDL deploy ordering, and shared rotated-role concurrency."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2", "dialogue:3", "dialogue:4", "dialogue:5", "dialogue:6"]
    text: "The critical and high findings are converted into binding revision constraints, except the credential heap-lifetime conflict is carried as an explicit unresolved question."
verdict: "needs_revision"
rationale: "The written candidate resolves the original hash-format fork in a viable direction, but the curated cross-exam trajectory contains one critical and sixteen high findings with no candidate rebuttal because the interrogation window was closed. The direct-EXECUTE and GUC-authority challenges are load-bearing for the D164 claim that leaked runtime credentials become uninteresting; operations and eval also show rollout and test-fidelity gaps that can wedge the daemon or produce false security evidence. The plan is salvageable, but not implementation-ready until the constraints below are discharged."
findings:
  - id: PX-001
    severity: critical
    posture: product
    status: converted_to_constraint
    challenge: "Owner-owned write functions still grant EXECUTE to striatumd_rw, so a process with only the runtime DSN can call append_audit_row and later artifact/event functions outside daemon RPC; in-DB hashing makes those writes hash-correct, not authorized."
    source_refs: ["dialogue:2"]
  - id: PX-002
    severity: high
    posture: product
    status: converted_to_constraint
    challenge: "Custom GUCs and RLS keyed on current_setting are client-controlled labels unless anchored to an authority source a raw SQL caller cannot spoof."
    source_refs: ["dialogue:2"]
  - id: PX-003
    severity: high
    posture: product
    status: converted_to_constraint
    challenge: "The L2 target posture is underspecified for new secure adoptions because the hardened socket flag is default-false for upgrades and the default-on evidence gate is not named."
    source_refs: ["dialogue:2"]
  - id: IX-001
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "pgxpool.BeforeAcquire cannot provide a transaction-local SET LOCAL invariant; the L3 labels must be set after BEGIN and before first DML or the guarantee must be renamed."
    source_refs: ["dialogue:3"]
  - id: IX-002
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "Broad migration and pgtest grants can silently reopen direct INSERT privileges after a one-off REVOKE, so negative grants must become schema contract."
    source_refs: ["dialogue:3"]
  - id: IX-003
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "SECURITY DEFINER write functions without fixed search_path, fully-qualified references, and explicit ACLs replace direct DML exposure with an owner-privilege confusion surface."
    source_refs: ["dialogue:3"]
  - id: PR-001
    severity: high
    posture: privacy
    status: deferred_with_owner
    challenge: "The rotated DB password remains in pgxpool.Config for reconnects, creating a heap-resident credential surface that conflicts with the candidate's honest C-SECRET-HONEST scope."
    source_refs: ["dialogue:4"]
  - id: PR-002
    severity: high
    posture: privacy
    status: converted_to_constraint
    challenge: "daemon_auth_log.detail is unconstrained JSONB and can persist raw DSNs, driver errors, or credentials unless serialization is whitelisted and redacted."
    source_refs: ["dialogue:4"]
  - id: PR-005
    severity: high
    posture: privacy
    status: converted_to_constraint
    challenge: "events.payload_json remains broad durable JSON; without DB-level validation, transcript-like provider output can enter authoritative storage."
    source_refs: ["dialogue:4"]
  - id: EV-001
    severity: high
    posture: eval
    status: converted_to_constraint
    challenge: "pgtest imperatively patches role privileges in Go, so privilege tests can validate a polluted harness instead of migrations-enforced production grants."
    source_refs: ["dialogue:5"]
  - id: EV-002
    severity: high
    posture: eval
    status: converted_to_constraint
    challenge: "The L2 plan lacks a negative lane-like connection test over socket and loopback, so pg_hba or socket-permission drift can pass undetected."
    source_refs: ["dialogue:5"]
  - id: EV-004
    severity: high
    posture: eval
    status: converted_to_constraint
    challenge: "Attribution reset must survive transaction aborts, cancellations, and panics, not only clean pool release."
    source_refs: ["dialogue:5"]
  - id: OPS-1
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "L0 rotation adds an every-restart owner-bootstrap dependency to a daemon that gates every Striatum verb; transient owner failure must fail closed with an owner-attributable diagnostic."
    source_refs: ["dialogue:6"]
  - id: OPS-2
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "In the documented live single-role posture, the owner==runtime guard skips L0 rotation, so the security property is inert unless surfaced as an adoption posture."
    source_refs: ["dialogue:6"]
  - id: OPS-3
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "After v3 rows are written, rollback to a v2-only verifier makes format skew indistinguishable from audit tamper unless Phase 0 has an explicit rollback posture."
    source_refs: ["dialogue:6"]
  - id: OPS-4
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "Owner-applied L1 DDL and runtime binary rollout have no deploy ordering or startup precondition, so old-schema/new-binary or premature REVOKE can wedge all audit writes."
    source_refs: ["dialogue:6"]
  - id: OPS-5
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "A single shared striatumd_rw password rotated per restart is implicitly single-writer-per-PG and breaks rolling, concurrent, or remote-PG daemons unless scoped or detected."
    source_refs: ["dialogue:6"]
constraints:
  - id: C-EXEC-AUTH
    source_finding: PX-001
    posture: product
    severity: critical
    kind: invariant
    binding: true
    text: "The revised spec must either narrow the product claim to say leaked runtime credentials can still perform syntactically valid write-function calls, or add a non-spoofable daemon-authority gate to every SECURITY DEFINER write function so direct calls as striatumd_rw fail without mutating audit_log, artifacts, or events while normal daemon RPC succeeds."
    source_refs: ["dialogue:2"]
    verification:
      gate: "PG-gated negative tests connect as striatumd_rw without daemon capability context and prove each protected write function fails without mutation; the daemon RPC positive path still succeeds."
    final_review_required: true
  - id: C-GUC-NONAUTH
    source_finding: PX-002
    posture: product
    severity: high
    kind: invariant
    binding: true
    text: "The revised spec must treat striatum.rpc_id, striatum.principal_id, and app.session_id GUCs as attribution labels only unless write functions verify a server-side, unexpired authority record that striatumd_rw cannot spoof."
    source_refs: ["dialogue:2"]
    verification:
      gate: "A direct SQL test sets fake GUC values as striatumd_rw and every protected write still fails; a daemon RPC test proves the same labels are set and cleared for attribution."
    final_review_required: true
  - id: C-L2-DEFAULT
    source_finding: PX-003
    posture: product
    severity: high
    kind: policy
    binding: true
    text: "The revised spec must separate legacy upgrade compatibility from the target secure posture: fresh or secure-profile adoptions either enable the PG-less lane user plus 0700 socket directory or emit a blocking doctor finding, while legacy upgrades warn until a named successor default-on release."
    source_refs: ["dialogue:2"]
    verification:
      expected_stage: "Spec/runbook revision names the secure-adoption posture, legacy-warning posture, default-on criteria, and successor release gate."
    final_review_required: true
  - id: C-TX-GUC-PRELUDE
    source_finding: IX-001
    posture: implementation
    severity: high
    kind: invariant
    binding: true
    text: "The revised spec must place L3 attribution inside the mutation transaction after BeginTx and before first DML through a shared prelude, or explicitly choose session-level SET plus poisoned-connection discard and rename the guarantee."
    source_refs: ["dialogue:3"]
    verification:
      gate: "A guard test fails mutating transactions that omit the attribution prelude and proves labels are absent on the next checkout after commit and rollback."
    final_review_required: true
  - id: C-GRANT-DRIFT
    source_finding: IX-002
    posture: implementation
    severity: high
    kind: gate
    binding: true
    text: "The revised implementation plan must make negative grants part of the schema contract so migration helpers, pgtest setup, and doctor grant repair cannot reopen direct INSERT on audit_log, artifacts, or events."
    source_refs: ["dialogue:3"]
    verification:
      gate: "Migration-forward and repair-grants tests assert direct INSERT to each protected table still fails with SQLSTATE 42501 after all grant setup runs."
    final_review_required: true
  - id: C-SD-HARDEN
    source_finding: IX-003
    posture: implementation
    severity: high
    kind: schema
    binding: true
    text: "Every owner-owned SECURITY DEFINER write function must use a locked-down search_path or fully-qualified references, avoid caller-controlled object names/operators, revoke ambient public execute, and grant EXECUTE only to intended roles."
    source_refs: ["dialogue:2", "dialogue:3"]
    verification:
      gate: "Migration tests inspect pg_proc/proconfig and ACLs and include a hostile search-path regression proving each function reaches intended striatumd objects."
    final_review_required: true
  - id: Q-DYNAMIC-CREDENTIALS
    source_finding: PR-001
    posture: privacy
    severity: high
    kind: unresolved_question
    binding: false
    text: "Resolve whether dynamic password providers are required for RFC 0110 or explicitly deferred as out of scope. Until resolved, the spec must preserve the candidate's honest guarantee: RAM-only plus rotation-on-restart, not unrecoverable live-process memory secrecy."
    source_refs: ["dialogue:4"]
    verification:
      expected_stage: "Revision synthesis records one disposition: folded in as C-DYNAMIC-CREDENTIALS, rejected with rationale against C-SECRET-HONEST, accepted as risk, or deferred with successor owner."
    final_review_required: true
  - id: C-AUTH-LOG-PRIVACY
    source_finding: PR-002
    posture: privacy
    severity: high
    kind: gate
    binding: true
    text: "daemon_auth_log.detail must be written through a strict key whitelist and DSN/credential redaction path; raw driver configs, unredacted DSNs, passwords, tokens, and connection parameters must not become durable records."
    source_refs: ["dialogue:4"]
    verification:
      gate: "Error-path serialization tests feed DSNs, driver errors, passwords, tokens, and query parameters and assert only whitelisted redacted detail is inserted."
    final_review_required: true
  - id: C-EVENT-NO-TRANSCRIPTS
    source_finding: PR-005
    posture: privacy
    severity: high
    kind: gate
    binding: true
    text: "The L1 events phase must include DB-level or shared contract validation that prevents raw provider output, terminal stdout/stderr, transcript-like keys, or durable transcript payloads from entering events.payload_json."
    source_refs: ["dialogue:4"]
    verification:
      gate: "Event insertion tests reject stdout, stderr, transcript, raw_output, and provider-output payload shapes while allowing curated metadata events."
    final_review_required: true
  - id: C-HARNESS-PRIVILEGES
    source_finding: EV-001
    posture: eval
    severity: high
    kind: gate
    binding: true
    text: "pgtest must verify roles and privileges produced by owner-applied migrations rather than imperatively patching GRANT/REVOKE state in Go during a run."
    source_refs: ["dialogue:5"]
    verification:
      gate: "Harness setup test fails if pgtest issues ad-hoc GRANT/REVOKE for protected-table privileges and confirms T-42501 runs against migration-defined roles."
    final_review_required: true
  - id: C-L2-NEG-TEST
    source_finding: EV-002
    posture: eval
    severity: high
    kind: gate
    binding: true
    text: "The hardened L2 posture must be proven by a lane-like unprivileged process that cannot connect to PostgreSQL over the protected UNIX socket or loopback TCP path."
    source_refs: ["dialogue:5"]
    verification:
      gate: "T-LANE-ISOLATION-NEG launches a mock lane identity and asserts socket and loopback connection attempts fail under the hardened posture."
    final_review_required: true
  - id: C-ATTR-RESET-FAIL
    source_finding: EV-004
    posture: eval
    severity: high
    kind: gate
    binding: true
    text: "Attribution reset must be proven across clean commit, rollback, query cancellation, transaction abort, and panic/error paths; dirty connections must be reset or discarded before reuse."
    source_refs: ["dialogue:5"]
    verification:
      gate: "T-ATTR-RESET covers commit, rollback, cancellation, abort, and panic/error paths and proves the next checkout observes no prior rpc_id, principal_id, or app.session_id."
    final_review_required: true
  - id: C-RESTART-OWNER-DEP
    source_finding: OPS-1
    posture: operations
    severity: high
    kind: policy
    binding: true
    text: "L0 owner bootstrap failure must fail closed with an owner-attributable diagnostic, and the every-restart owner-connectivity dependency must be documented as an accepted operational trade."
    source_refs: ["dialogue:6"]
    verification:
      gate: "Startup and doctor error-path tests simulate owner bootstrap failure and assert no stale runtime credential fallback occurs and the diagnostic names owner connectivity."
    final_review_required: true
  - id: C-L0-ADOPTION-VISIBLE
    source_finding: OPS-2
    posture: operations
    severity: high
    kind: policy
    binding: true
    text: "The owner==runtime rotation skip must surface as a daemon doctor posture finding, and the owner/runtime role split must be documented as an L0 adoption prerequisite before the spec claims the runtime credential is made uninteresting."
    source_refs: ["dialogue:6"]
    verification:
      gate: "Doctor posture tests on a single-role fixture report rotation_skipped_single_role as a visible finding and the runbook names the owner/runtime split."
    final_review_required: true
  - id: C-ROLLBACK-FORWARD-ONLY
    source_finding: OPS-3
    posture: operations
    severity: high
    kind: policy
    binding: true
    text: "L1 Phase 0 v3 writes must be treated as a forward-only cutover unless the rollback target already understands v3; the spec must choose a flagged commit point, backported verifier, or explicit verify-tolerance runbook."
    source_refs: ["dialogue:6"]
    verification:
      expected_stage: "Revision synthesis records the selected rollback posture and final review confirms the acceptance gate distinguishes format skew from tamper."
    final_review_required: true
  - id: C-DDL-DEPLOY-ORDER
    source_finding: OPS-4
    posture: operations
    severity: high
    kind: gate
    binding: true
    text: "Owner-applied L1 DDL and runtime binary rollout must have an explicit idempotent deploy order plus startup schema preconditions, so missing append_audit_row or premature REVOKE fails fast with actionable diagnostics instead of wedging audit writes."
    source_refs: ["dialogue:6"]
    verification:
      gate: "Startup-precondition tests cover new-binary/old-schema and old-binary/premature-REVOKE skew and assert actionable failure before serving mutations."
    final_review_required: true
  - id: C-ROTATION-SINGLE-WRITER
    source_finding: OPS-5
    posture: operations
    severity: high
    kind: invariant
    binding: true
    text: "Runtime password rotation must either declare and doctor-detect a single-daemon-per-striatumd_rw-role invariant, or use per-instance roles for remote/concurrent PG deployments; shared rotated credentials across active daemons are not allowed."
    source_refs: ["dialogue:6"]
    verification:
      expected_stage: "Revision synthesis selects single-daemon-per-role with detection or per-instance roles, and names the remote-PG/rolling-restart behavior explicitly."
    final_review_required: true
branches:
  product: blocked_pending_answer
  implementation: blocked_pending_answer
  privacy: blocked_pending_answer
  eval: blocked_pending_answer
  operations: blocked_pending_answer
---

# RFC 0110 Adjudication Ledger (Cycle 1)

Verdict: **needs_revision**.

The candidate is directionally sound on the v3 hash-format decision and the
L0-L3 sequencing, but the curated record does not support a clearing verdict.
The cross-exam window was closed, so there are no candidate rebuttals to weigh
against the critical/high findings. The revision convener should treat the
constraints in the front matter as binding inputs.

## Disposition

- **Product:** blocked pending revision. `C-EXEC-AUTH` and `C-GUC-NONAUTH` decide
  whether the headline leaked-runtime-credential claim survives.
- **Implementation:** blocked pending revision. L3 placement, grant drift, and
  `SECURITY DEFINER` hardening must become executable contracts.
- **Privacy:** blocked pending revision. `daemon_auth_log` and event payloads
  need durable redaction/exclusion gates; dynamic credential handling remains an
  explicit unresolved question.
- **Eval:** blocked pending revision. The harness must prove production
  privileges and negative L2 isolation rather than validate a patched test state.
- **Operations:** blocked pending revision. Restart dependency, single-role L0
  visibility, v3 rollback, deploy ordering, and shared-role rotation must be
  resolved before implementation.

## Medium Follow-Ups

The medium rows in the curated trajectory are not required as binding revision
constraints in this cycle, but the revision should fold them into the relevant
high constraints where practical: `C-HASH-V3`, `C-HASH-BYTEA`,
`C-SHA-PRIMITIVE`, `C-VERIFY-DISPATCH`, `C-TEST-ROW-WRITER`,
`C-TZ-INDEPENDENT-HASH`, `C-GUC-PARAMETERIZED`, `C-SOCKET-DIR-PERMS`,
`C-OWNER-DR`, `C-DOCTOR-OWNER-REACH`, and
`C-SOCKET-RELOCATE-MIGRATION`.
