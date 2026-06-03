---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "adjudicated_constraint_extraction"
author: adjudicator-codex-gpt-5.5-xhigh-004
workflow: "rfc-0110-pg-auth-panel"
run_id: "run_8e14cb48342e929d30043d6be24f9101"
cycle: 2
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
    text: "The cycle-2 synthesis discharges the cycle-1 ledger by narrowing the leaked-credential claim, adding a RAM-only daemon_auth authority gate, moving L3 into the mutation transaction, adopting v3 bytea audit hashing, and sequencing L0-L3 rollout gates."
  - kind: challenge
    by: product_cross_examiner
    refs: ["dialogue:2"]
    text: "The product posture accepts the direct write-spoofing fix in concept but challenges read-scope overclaiming, phase-scope for the sole durable write path, the v3 hash cutover as a release gate, and premature #87 closure language."
  - kind: challenge
    by: implementation_cross_examiner
    refs: ["dialogue:3"]
    text: "The implementation posture challenges the daemon_auth prelude over pgx simple protocol, the lack of an unavoidable authorized transaction wrapper, audit append fail-open behavior, owner-only DDL delivery, and pgtest privilege fidelity."
  - kind: challenge
    by: operations_cross_examiner
    refs: ["dialogue:4"]
    text: "The operations posture credits the cycle-1 operations discharges but challenges the new daemon_auth freshness lifecycle, authority-capability deploy parity, role-scoped rotator detection, and bounded reconnect behavior."
  - kind: challenge
    by: privacy_cross_examiner
    refs: ["dialogue:5"]
    text: "The privacy posture is degraded for cycle 2 because it targeted the closed cycle-1 convener session; its rows are treated as cycle-1 re-statements, except that its GUC visibility concern corroborates the implementation daemon_auth transport challenge."
  - kind: challenge
    by: eval_cross_examiner
    refs: ["dialogue:5"]
    text: "The eval posture is degraded for cycle 2 because it targeted the closed cycle-1 convener session; its rows are treated as already discharged cycle-1 re-statements, except that reset robustness corroborates the operations reconnect/discard concern."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2", "dialogue:3", "dialogue:4"]
    text: "The fresh critical and high cycle-2 findings are converted into binding constraints. Medium findings remain publication and implementation-sequencing follow-ups unless folded into a high constraint."
verdict: "needs_revision"
rationale: "Cycle 2 materially improves the RFC 0110 plan: it resolves the Go-JSON-to-PL/pgSQL chain-hash risk by choosing an explicit v3 bytea hash format with verifier dispatch, and it answers the original direct-EXECUTE hole by adding a daemon-authority gate instead of relying on spoofable attribution GUCs. That is the right direction, but the curated cycle-2 cross-exam record still contains one critical and eleven high fresh findings with no convener rebuttal. The critical finding is load-bearing: current pgx simple-protocol posture can expose the daemon_auth secret in query text unless the prelude uses extended protocol or another non-text carrier. Product and operations also show that claim scope, phase closure, #87 status, authority liveness, deploy parity, and rotator detection must be pinned before implementation starts. The gate therefore remains needs_revision, narrowly scoped to the constraints below."
findings:
  - id: IX2-001
    severity: critical
    posture: implementation
    status: converted_to_constraint
    challenge: "The daemon_auth prelude is specified as parameterized set_config, but the daemon currently forces pgx QueryExecModeSimpleProtocol, which can interpolate parameters into query text observable by a same-role striatumd_rw session."
    source_refs: ["dialogue:3"]
  - id: IX2-002
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "A prose rule to run attribution and authority after BeginTx is not enforceable while mutating handlers can still use generic withTx or BeginTx and issue writes before the prelude."
    source_refs: ["dialogue:3"]
  - id: IX2-003
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "Audit append opens its own transaction and current RPC handling can ignore audit append errors, so an L1 authority failure could allow a mutation to succeed without its mandatory audit row."
    source_refs: ["dialogue:3"]
  - id: IX2-004
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "Owner-only L1 function and revoke DDL cannot be dropped into runtime-role auto-migrations without crash-loop or skew risk."
    source_refs: ["dialogue:3"]
  - id: IX2-005
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "pgtest can still grant or revoke protected-table DML imperatively, so privilege tests can validate a test-only grant layout rather than production migrations."
    source_refs: ["dialogue:3"]
  - id: PX3-001
    severity: high
    posture: product
    status: converted_to_constraint
    challenge: "The cycle-2 claim that a leaked DSN string is uninteresting is too broad if striatumd_rw retains broad SELECT access to artifacts, events, sessions, queue messages, principals, blockers, or payload JSON."
    source_refs: ["dialogue:2"]
  - id: PX3-002
    severity: high
    posture: product
    status: converted_to_constraint
    challenge: "The sole durable write-path claim can be asserted too early because Phase 0 protects audit_log while artifacts and events remain directly writable until later phases."
    source_refs: ["dialogue:2"]
  - id: PX3-003
    severity: high
    posture: product
    status: converted_to_constraint
    challenge: "The v3 hash cutover is a provenance release gate, not merely an eval detail; no SQL append should emit v3 rows before the shipped verifier dispatch and mixed-format tests exist."
    source_refs: ["dialogue:2"]
  - id: PX3-004
    severity: high
    posture: product
    status: converted_to_constraint
    challenge: "The operator brief still records #87 as partial, so L2 must not be called closed or a hardened default until the PG-less lane OS user, negative isolation test, and blocking doctor posture are live."
    source_refs: ["dialogue:2"]
  - id: PX3-005
    severity: medium
    posture: product
    status: open
    challenge: "D164 and the spec must be amended before behavior-changing implementation so the operator-facing contract matches cycle-2 scope rather than the old BeforeAcquire/GUC wording."
    source_refs: ["dialogue:2"]
  - id: PX3-006
    severity: medium
    posture: product
    status: open
    challenge: "The durable-table write authority inventory should cover the whole daemon-owned schema, not only audit_log, artifacts, and events."
    source_refs: ["dialogue:2"]
  - id: OPS-9
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "The daemon_auth registry freshness window is undefined; a finite unrefreshed window can silently age out a correct secret and wedge every write, while an infinite window undercuts stale-instance detection."
    source_refs: ["dialogue:4"]
  - id: OPS-10
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "Startup preconditions check function presence but not authority-capability parity, so old binary plus authority-bearing owner DDL can pass startup and fail every write at runtime."
    source_refs: ["dialogue:4"]
  - id: OPS-11
    severity: high
    posture: operations
    status: converted_to_constraint
    challenge: "The concurrent-rotator probe keys on different instance_id even though the sanctioned multi-host posture uses distinct per-instance roles, causing false positives on healthy multi-host deployments."
    source_refs: ["dialogue:4"]
  - id: OPS-12
    severity: medium
    posture: operations
    status: open
    challenge: "Destroying connections on every tx error or cancellation can turn transient PostgreSQL stress into a reconnect storm that collides with runtime-password rotation."
    source_refs: ["dialogue:4"]
constraints:
  - id: C-EXTENDED-AUTH-PRELUDE
    source_finding: IX2-001
    posture: implementation
    severity: critical
    kind: invariant
    binding: true
    text: "The daemon-authority prelude must not execute through pgx simple protocol or any other path that serializes the daemon_auth secret into SQL query text visible to striatumd_rw observers; use extended protocol for the prelude or choose a proven non-text authority carrier."
    source_refs: ["dialogue:3"]
    verification:
      gate: "A PG-gated same-role observer regression triggers a daemon-authorized mutation and proves the daemon_auth value never appears in pg_stat_activity or statement tracing; a unit guard fails if the prelude path runs under QueryExecModeSimpleProtocol."
    final_review_required: true
  - id: C-AUTH-TX-WRAPPER
    source_finding: IX2-002
    posture: implementation
    severity: high
    kind: invariant
    binding: true
    text: "Mutating RPC handlers must enter PostgreSQL through a dedicated authorized mutation transaction constructor that applies authority and attribution before user handler SQL, and generic withTx or BeginTx must remain unavailable to authoritative mutations."
    source_refs: ["dialogue:3"]
    verification:
      gate: "A guard enumerates mutating handlers and fails any direct generic transaction use; an SQL-order test records set_config('striatum.daemon_auth', ...) as statement 1 before the first DML or write-function call."
    final_review_required: true
  - id: C-AUDIT-AUTH-PRELUDE
    source_finding: IX2-003
    posture: implementation
    severity: high
    kind: gate
    binding: true
    text: "Audit append must receive the same daemon authority and attribution context as the mutation it records, and audit append failure after a mutation must fail loudly under an explicit contract rather than returning success without an audit row."
    source_refs: ["dialogue:3"]
    verification:
      gate: "A regression forces append_audit_row to reject and proves the RPC path rolls back or fails according to the documented fail-closed contract; it never returns success while omitting the audit row."
    final_review_required: true
  - id: C-OWNER-DDL-SPLIT
    source_finding: IX2-004
    posture: implementation
    severity: high
    kind: schema
    binding: true
    text: "Owner-only L1 function, revoke, pgcrypto, registry, and authority-marker DDL must have a distinct idempotent delivery path or marker; runtime ConnectAndMigrate must not attempt owner-only DDL, and startup must fail before mutation on schema/binary skew."
    source_refs: ["dialogue:3"]
    verification:
      gate: "Skew tests cover new-binary/old-schema, old-binary/premature-revoke, and partial owner-DDL states, each failing with an actionable pre-mutation diagnostic."
    final_review_required: true
  - id: C-PGTEST-NO-DML-GRANT
    source_finding: IX2-005
    posture: implementation
    severity: high
    kind: gate
    binding: true
    text: "pgtest role and privilege setup for the RFC 0110 security gates must come from migration-owned SQL; Go helpers must not grant or revoke protected-table DML outside the owner migration contract."
    source_refs: ["dialogue:3"]
    verification:
      gate: "A guard fails any pgtest path that grants or revokes protected-table DML, and T-42501 runs against migration-defined roles after migrations, owner DDL, and grant repair."
    final_review_required: true
  - id: C-DSN-READ-SCOPE
    source_finding: PX3-001
    posture: product
    severity: high
    kind: policy
    binding: true
    text: "The spec must either define and test least-privilege read surfaces for striatumd_rw or narrow the RFC 0110 claim to unauthorized mutation and hash/attempt-invariant violation, explicitly excluding broad read confidentiality from this phase."
    source_refs: ["dialogue:2"]
    verification:
      expected_stage: "Before implementation merge, D164/spec language states the read-scope posture; if private read denial is claimed, PG-gated tests prove raw runtime connections cannot read those surfaces."
    final_review_required: true
  - id: C-PHASED-WRITE-CLOSURE
    source_finding: PX3-002
    posture: product
    severity: high
    kind: policy
    binding: true
    text: "The product contract must name which durable write surfaces are protected at each L1 phase and reserve the full sole durable write-path claim until audit_log, artifacts, and events are all gated with direct-DML negative tests."
    source_refs: ["dialogue:2"]
    verification:
      expected_stage: "Phase text, release notes, and doctor posture strings distinguish audit-only, audit-plus-artifacts, and full audit/artifacts/events protection; each phase has matching 42501-style negative tests."
    final_review_required: true
  - id: C-AUDIT-FORMAT-CUTOVER
    source_finding: PX3-003
    posture: product
    severity: high
    kind: gate
    binding: true
    text: "The v3 audit hash cutover must ship as one release gate: SQL append, SQL v3 hash, Go V3RowHash, VerifyRows v2/v3 dispatch, unknown-format failure, default-v2 cutover flag, mixed-format tests, and rollback/skew runbook."
    source_refs: ["dialogue:2"]
    verification:
      gate: "With the flag off, no v3 row is producible and a v2-only verifier remains green; with the flag on, SQL-written v3 rows verify through the dispatch path and mixed v2 to v3 chains remain continuous."
    final_review_required: true
  - id: C-87-CLOSURE-GATE
    source_finding: PX3-004
    posture: product
    severity: high
    kind: policy
    binding: true
    text: "#87 remains partial until the dedicated PG-less lane OS user, protected 0700 socket posture, negative lane isolation test, and blocking doctor behavior are live in the default or named secure profile."
    source_refs: ["dialogue:2"]
    verification:
      expected_stage: "Spec, runbook, issue status, and operator-facing doctor text say partial until T-LANE-ISOLATION-NEG is green and the hardened profile blocks PG-reachable lanes."
    final_review_required: true
  - id: C-AUTH-WINDOW-LIVENESS
    source_finding: OPS-9
    posture: operations
    severity: high
    kind: invariant
    binding: true
    text: "The daemon_auth freshness lifecycle must be explicit: a live instance must not self-wedge on an aged registry row, and any authority lapse or refresh failure must fail closed with owner-attributable diagnostics and doctor visibility."
    source_refs: ["dialogue:4"]
    verification:
      gate: "T-AUTH-LIVENESS ages or refresh-fails the registry row and proves the daemon either continues writing under lifetime-of-instance validity or fails closed with a doctor-visible owner-attributable finding, never silent 28000 write failure."
    final_review_required: true
  - id: C-DEPLOY-CAPABILITY-PARITY
    source_finding: OPS-10
    posture: operations
    severity: high
    kind: gate
    binding: true
    text: "Startup preconditions must verify binary-to-schema authority-capability parity over the full dependency set, including append functions, assert_daemon_authority, daemon_auth_registry, pgcrypto, and any requires_daemon_auth marker."
    source_refs: ["dialogue:4"]
    verification:
      gate: "Owner DDL stamps a schema authority requirement and startup skew tests prove an old binary against authority-bearing schema, missing registry, or missing assert function fails before serving mutations."
    final_review_required: true
  - id: C-ROTATOR-PROBE-ROLE-SCOPED
    source_finding: OPS-11
    posture: operations
    severity: high
    kind: invariant
    binding: true
    text: "Concurrent-rotator detection must be scoped to role collision, not merely distinct instance ids; sanctioned per-instance-role multi-host deployments must not trip the probe, while two live instances rotating the same runtime role must."
    source_refs: ["dialogue:4"]
    verification:
      gate: "Registry rows record the rotated runtime role; tests cover a no-false-positive per-instance-role multi-host case and a positive same-role concurrent-rotator case."
    final_review_required: true
branches:
  product: blocked_pending_answer
  implementation: blocked_pending_answer
  operations: blocked_pending_answer
  privacy: cleared_with_constraints
  eval: cleared_with_constraints
---

# RFC 0110 Adjudication Ledger (Cycle 2)

Verdict: **needs_revision**.

Cycle 2 moved the RFC in the right direction. The original direct `EXECUTE` hole
is no longer left to spoofable GUCs, and the audit hash risk is resolved at the
design level by switching to an explicit v3 bytea format with mixed v2/v3
verification instead of trying to reproduce Go JSON ordering in PL/pgSQL.

The plan is still not implementation-ready. The new `daemon_auth` authority gate
now carries the security property, so its transport and lifecycle must be
non-leaky, unavoidable, auditable, deployable, and operable. The constraints in
the front matter are the next revision's binding inputs.

## Disposition

- **Product:** blocked pending revision. The design may claim unauthorized
  mutation and provenance invariants only at the phase where those gates are
  live. Read scope, phase closure, v3 cutover, and #87 status must be honest
  before implementation follows D164.
- **Implementation:** blocked pending revision. The authority prelude cannot ride
  simple protocol; mutating handlers need an authorized transaction wrapper; audit
  append must fail closed; owner DDL and pgtest privileges need production-shaped
  delivery.
- **Operations:** blocked pending revision. The new authority gate needs a
  defined freshness lifecycle, binary/schema capability parity, and role-scoped
  rotator detection.
- **Privacy and eval:** cleared with constraints. Their cycle-2 artifacts are
  degraded stale-target re-statements of cycle 1 and do not add fresh blocking
  rows, but they corroborate `C-EXTENDED-AUTH-PRELUDE` and the reconnect/reset
  follow-up.

## Medium Follow-Ups

`PX3-005`, `PX3-006`, and `OPS-12` are not binding rows because the schema only
allows binding constraints to source high or critical findings. The revision
should still fold them where practical:

- amend D164 and `docs/reference/spec.md` before the first behavior-changing PR;
- publish a durable-table write-authority inventory so future tables cannot
  bypass the L1 model silently;
- bound discard-on-error and reconnect behavior so attribution hygiene does not
  amplify transient PostgreSQL failures or rotation collisions.

## Sequencing

1. Spec and decision corrections first: claim scope, v3 cutover, L3 transaction
   prelude, `daemon_auth` authority, #87 partial status, and phased durable write
   closure.
2. Authority mechanism next: extended/non-text prelude path, authorized mutation
   wrapper, audit-authority prelude, and fail-closed audit behavior.
3. Owner DDL and harness fidelity: split owner-only DDL, stamp capability
   requirements, enforce startup parity, and make pgtest use migration-defined
   roles.
4. L0/L1 Phase 0 only after the authority and audit gates are pinned:
   `append_audit_row`, v3 hash helpers, rollback flag, `T-EXEC-AUTH`,
   `T-GRANT-DRIFT`, `T-HARNESS-FIDELITY`, `T-AUDIT-LIVENESS`, and
   `T-DEPLOY-SKEW`.
5. L1 artifacts and events, then L2 hardening. Full #87 closure waits for the
   PG-less lane OS user, protected socket posture, and negative lane isolation
   test in the default or named secure profile.
