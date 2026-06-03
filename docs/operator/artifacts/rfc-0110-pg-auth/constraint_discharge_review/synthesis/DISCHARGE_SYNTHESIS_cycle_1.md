---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
author: adjudicator-codex-gpt-5.5-xhigh-005
workflow: "rfc-0110-pg-auth-panel"
phase: "constraint_discharge_review"
run_id: "run_8e14cb48342e929d30043d6be24f9101"
cycle: 1
title: "RFC 0110 discharge synthesis: cleared cycle-2 constraints"
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/adjudicator/COLLABORATION_LEDGER_cycle_2.md
  - docs/operator/artifacts/rfc-0110-pg-auth/adjudication/DECISION_cycle2_proceed.md
  - docs/operator/artifacts/rfc-0110-pg-auth/revision_synthesis/synthesis/REVISION_SYNTHESIS_cycle_1.md
  - docs/operator/artifacts/rfc-0110-pg-auth/constraint_discharge_review/review/DISCHARGE_REVIEW_cycle_1.md
---

# RFC 0110 Discharge Synthesis
author: adjudicator-codex-gpt-5.5-xhigh-005

## Binding Ledger For Spec Publication

The binding ledger for the spec author is:

`docs/operator/artifacts/rfc-0110-pg-auth/revision_synthesis/synthesis/REVISION_SYNTHESIS_cycle_1.md`

That artifact is the cleared revision of the cycle-2 adjudication ledger. It
incorporates the twelve binding constraints extracted in
`COLLABORATION_LEDGER_cycle_2.md` and was accepted by
`DISCHARGE_REVIEW_cycle_1.md` with verdict intent `accept_with_findings`.

## What Is Cleared

All twelve critical/high cycle-2 constraints are discharged at the synthesis
level:

- `C-EXTENDED-AUTH-PRELUDE`
- `C-AUTH-TX-WRAPPER`
- `C-AUDIT-AUTH-PRELUDE`
- `C-OWNER-DDL-SPLIT`
- `C-PGTEST-NO-DML-GRANT`
- `C-DSN-READ-SCOPE`
- `C-PHASED-WRITE-CLOSURE`
- `C-AUDIT-FORMAT-CUTOVER`
- `C-87-CLOSURE-GATE`
- `C-AUTH-WINDOW-LIVENESS`
- `C-DEPLOY-CAPABILITY-PARITY`
- `C-ROTATOR-PROBE-ROLE-SCOPED`

The spec publication job should treat the revision synthesis as authoritative
where it amends `SYNTHESIS_cycle_2.md`. Where the two disagree, the revision
synthesis wins.

## Mandatory Carry-Forward Gates

The clearing verdict does not waive the implementation gates. The spec author
must carry these forward as normative acceptance checks:

- Step-0 documentation amendments to D164, `docs/reference/spec.md`, and RFC
  0110 before behavior-changing implementation merges.
- The narrowed read-scope posture: RFC 0110 protects unauthorized mutation and
  provenance invariants in this phase; it does not claim broad read
  confidentiality against a live runtime credential.
- Phase-specific L1 claims: `audit_only`, `audit_artifacts`, then `full`; the
  sole durable write-path claim is reserved for `full`.
- #87 status language: "mitigated, pending lane-OS-user default" until the
  PG-less lane user, 0700 socket posture, negative isolation test, and blocking
  doctor posture are live.
- The named gates from the revision matrix, especially
  `T-PRELUDE-OBSERVER`, `G-MUTATION-TX`, `T-AUDIT-FAILCLOSED`,
  `T-DEPLOY-SKEW`, `T-42501-P0/P1/P2`, `T-VERIFY-MIXED`,
  `T-LANE-ISOLATION-NEG`, `T-AUTH-LIVENESS`, and `T-ROTATOR-SCOPE`.

## Residuals

No blocking constraint remains open for spec publication. The residuals are
explicit and accepted for this stage: fail-closed audit availability coupling,
pre-release-N binary skew bounded by runbook order and 42501 denial, broad read
authority for a live runtime credential, #87 pending L2 isolation, and later
successors for read-scope least privilege, dynamic credentials, and remote or
multi-host hardening.
