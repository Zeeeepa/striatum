---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0110", "constraint-discharge", "pg-auth", "database-write-boundary"]
---

# RFC 0110 Constraint Discharge Review (Cycle 1)
author: adjudicator-codex-gpt-5.5-xhigh-005

## Verdict

`accept_with_findings`

The revised synthesis discharges all twelve binding cycle-2 constraints from
`COLLABORATION_LEDGER_cycle_2.md`. No critical or high binding constraint
remains open. The remaining findings are forward gates and explicit residuals:
they must be preserved by the implementation spec and later build reviews, but
they do not require another revision of this synthesis.

## Scope Reviewed

- `docs/operator/artifacts/rfc-0110-pg-auth/adjudication/adjudicator/COLLABORATION_LEDGER_cycle_2.md`
- `docs/operator/artifacts/rfc-0110-pg-auth/adjudication/DECISION_cycle2_proceed.md`
- `docs/operator/artifacts/rfc-0110-pg-auth/revision_synthesis/synthesis/REVISION_SYNTHESIS_cycle_1.md`
- Spot checks against current source cited by the revision:
  `go/pkg/db/connection.go`, `go/pkg/rpc/server.go`, `go/pkg/db/audit.go`,
  and `go/pkg/pgtest/pgtest.go`

## Constraint Discharge

| constraint | review result |
| --- | --- |
| `C-EXTENDED-AUTH-PRELUDE` | Discharged. The revision replaces the unsafe simple-protocol prelude with per-call extended protocol through `ExecBound`, keeps the pool default unchanged, and names `T-PRELUDE-OBSERVER` plus `G-PRELUDE-MODE`. |
| `C-AUTH-TX-WRAPPER` | Discharged. `BeginAuthorizedMutation`, type-level write helpers, `G-MUTATION-TX`, and `T-SQL-ORDER` turn the prelude into an enforced transaction constructor instead of a convention. |
| `C-AUDIT-AUTH-PRELUDE` | Discharged with accepted availability coupling. Mutating audit append is moved into the same `MutationTx`; standalone audit failures become RPC errors. `T-AUDIT-FAILCLOSED` is the right gate. |
| `C-OWNER-DDL-SPLIT` | Discharged. Owner-only DDL is moved to versioned, atomic, stamped owner bundles, with runtime migrations barred from owner objects and skew covered by `T-DEPLOY-SKEW`. |
| `C-PGTEST-NO-DML-GRANT` | Discharged. pgtest is required to consume production-shaped migrations and owner bundles, with `G-PGTEST-GRANTS` and ordered `T-42501` coverage. |
| `C-DSN-READ-SCOPE` | Discharged by narrowing. The revision retracts broad read-confidentiality claims and limits RFC 0110 to unauthorized mutation and provenance invariants, with a named read-scope successor. |
| `C-PHASED-WRITE-CLOSURE` | Discharged. The `audit_only`, `audit_artifacts`, and `full` phases make the protected surfaces and allowed product claims explicit, backed by per-phase 42501 tests and doctor posture strings. |
| `C-AUDIT-FORMAT-CUTOVER` | Discharged. R-V3 is correctly treated as one release gate: SQL writer, SQL/Go hash builders, verifier dispatch, default-v2 flag, mixed-format tests, unknown-format failure, and rollback runbook. |
| `C-87-CLOSURE-GATE` | Discharged by corrected status language. #87 is now "mitigated, pending lane-OS-user default" until the PG-less lane user, 0700 socket posture, negative isolation test, and blocking doctor posture are live. |
| `C-AUTH-WINDOW-LIVENESS` | Discharged. The unsafe freshness-window concept is deleted; validity is lifetime-of-instance, and missing or superseded registry rows fail closed through a structured `daemon_auth_lost` path. |
| `C-DEPLOY-CAPABILITY-PARITY` | Discharged with scoped pre-N residual. Capability markers and startup parity cover the full dependency set; binaries older than release N are honestly bounded by deploy order and 42501 denial, not claimed as startup-covered. |
| `C-ROTATOR-PROBE-ROLE-SCOPED` | Discharged. The probe keys on `role_name` collision, preserving sanctioned per-instance-role multi-host deployments while still catching shared-role rotators. |

## Findings To Carry Forward

1. The discharge depends on the step-0 doc gate. D164, `docs/reference/spec.md`,
   and RFC 0110 must be amended before behavior-changing implementation lands,
   especially for narrowed read scope, phase-specific write closure, #87 status,
   and the authority-prelude model.
2. The accepted residuals are real and should stay visible: fail-closed audit
   couples availability to audit appendability; binaries older than release N
   cannot be retroactively startup-gated; live runtime credentials still carry
   broad read authority; #87 remains open until L2 isolation gates are live.
3. Later implementation reviews must treat the named gates as normative, not
   illustrative. In particular: `T-PRELUDE-OBSERVER`, `G-MUTATION-TX`,
   `T-AUDIT-FAILCLOSED`, `T-DEPLOY-SKEW`, `T-42501-P0/P1/P2`,
   `T-VERIFY-MIXED`, `T-LANE-ISOLATION-NEG`, `T-AUTH-LIVENESS`, and
   `T-ROTATOR-SCOPE`.

## Source Spot Checks

The revision's current-state premises match the tree:

- `go/pkg/db/connection.go` exposes generic `BeginTx` and sets
  `DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol`.
- `go/pkg/rpc/server.go` currently ignores audit append failures except for
  omitting `AuditID`.
- `go/pkg/db/audit.go` currently opens an independent transaction for audit
  transport rows and `VerifyRows` still computes `V2RowHash` unconditionally.
- `go/pkg/pgtest/pgtest.go` still imperatively grants broad table DML and then
  applies ad hoc revokes.

These checks support the revision's problem statement and the proposed gate
set. They are not acceptance evidence for implementation; they only confirm the
constraints were discharged at the synthesis/spec level.
