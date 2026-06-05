---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: medium
tags: ["rfc-0112", "option-c", "fixture-rigor"]
author: scorekeeper-codex-gpt-5.5-xhigh-001
run_id: run_c57e270528b569e2c53c2befec8c3b82
session_id: sess_04d74e64973ed4d2b9387d56c82b10ce
title: "Scorecard C"
---

# Scorecard C: Fixture-First Plan

author: scorekeeper-codex-gpt-5.5-xhigh-001

## Verdict

`accept_with_findings`.

Proposal C is the strongest fit for the RFC 0105/RFC 0106 proof bar: it treats
ACE graduation evidence as a standing production-handler fixture rather than a
unit predicate test, and it answers the six panel questions in a concrete
implementation shape. It is not implementation-ready as written because its
fixture proof can still be too shallow and its release-hook bypass guard depends
on an unspecified central transition helper.

## Scores

| Dimension | Score | Finding |
| --- | ---: | --- |
| Maintainability | 3/5 | The plan keeps direct-dependent compatibility and avoids a new RPC family, but querying `workflow_json` with lateral JSONB expansion inside the pending-consumer predicate is harder to reason about and test than a resolved consumer relation/helper. The proposed `db.WithJobStateTransition` choke point is named as "or equivalent," not made concrete. |
| Migration risk | 4/5 | Avoiding a structural schema migration for V1 is attractive. The main risk moves to runtime query shape, snapshot-version handling, and making validation/run-prepare enforce the exact same interpretation the predicate later reads. |
| Reversibility | 4/5 | The field is additive, unknown entry fields warn in V1, and workflows without `interrogation_targets` retain existing behavior. Reversibility is good if the JSONB predicate is isolated behind one resolver. |
| Operator legibility | 4/5 | `context.interrogation_targets[]` with `available`/`unavailable`/`not_ready` is legible and avoids blind `interrogation.open` calls. The plan should add an explicit `reason`/`message` for unavailable states so packet users can distinguish panel-window closure from retired or superseded target attempts. |
| Lifecycle correctness | 3/5 | The proposal names the right terminal paths and revision fixture, but it does not fully specify how explicit consumers bind to the current attempt after `reopenJobForAttempt`, nor how the generalized release hook is guaranteed on every future terminalizing path. |
| Fixture rigor | 3/5 | Fixture-first is the right posture, and the happy/revision/fault fixtures map to RFC 0112 AC 2-4. The gap is that "open, ask, answer" can prove plumbing without proving preserved-context cross-examination. The fixture must seed a convener-only fact or state and assert the answer uses it, mirroring RFC 0082's intention test. |

## Six-Question Coverage

Proposal C answers all six questions, with partial gaps:

- Q1 field shape: complete. It confirms `{workflow_job_id, required}`,
  validation teeth, and V1 unknown-field linting.
- Q2 required semantics: complete enough for V1. Advisory only, non-wedging
  fallback, and durable event only on explicit `interrogation.open` are coherent.
- Q3 multiple targets: directionally complete. It allows N targets, but should
  define consumer behavior when several required targets have mixed states.
- Q4 terminal hook: incomplete. The mutation list is correct, but the central
  hook point and future-path guard are still aspirational.
- Q5 RFC 0105 fixture: strong but not sufficient. It names the three required
  fixtures, but must assert preserved-context content, same-attempt requeue after
  dead lane, and no silent leaked-window state.
- Q6 packet namespace: mostly complete. The namespace and states are correct,
  but unavailable/not_ready entries need explicit omission/null rules for
  `target_session_id` and a machine-legible reason.

## Blocking Gaps To Close

1. Add preserved-context assertions to the ACE happy-path and revision fixtures:
   seed target-only context, ask for it through interrogation, and fail if the
   answer can be produced from artifacts alone.
2. Make the release hook a real choke point. Either route all terminal job
   transitions through one helper or add an authority/guard test that enumerates
   every terminalizing RPC path and fails on omissions.
3. Specify current-attempt resolution for explicit consumers after revision
   reopen: old target session retired, old open interrogations closed/retired,
   fan-out re-blocked, and packet projection bound only to the fresh attempt.
4. Decide whether the JSONB snapshot query is the durable implementation or a
   temporary resolver. If it remains, add tests for absent arrays, malformed
   entry shapes, multiple targets, and direct-plus-explicit duplicate consumers.
5. Extend packet projection with `reason` and `target_session_id` nullability
   rules for `unavailable` and `not_ready`.

With those changes, Option C is a credible implementation plan and should feed
the arbitration as the fixture-rigor baseline.
