---
schema_version: striatum.decision.v1
decision_id: "dec_d38dd9ec5eb5452196090b8335cb3689"
run_id: "run_5f439894bda2486c8668b17a096a0fd2"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0018 V1 (review postures) accepted with workflow-validation re-cast"
created_at: "2026-05-09T02:00:29Z"
---

# RFC 0018 V1 (review postures) accepted with workflow-validation re-cast

Decision ID: `dec_d38dd9ec5eb5452196090b8335cb3689`
Run ID: `run_5f439894bda2486c8668b17a096a0fd2`
Outcome: `accepted_with_follow_up`

## Rationale

V1 ships RFC 0018 steps 1+2; step 3 deferred per the RFC's own implementation path. The RFC's step-2 runtime build-completion gate is re-cast as a workflow-validation gate (raises WorkflowError exit 8 at workflow validate / run prepare) because the runtime gate as originally written deadlocks against striatum's lifecycle: a build's complete mutation precedes its downstream review's verdict by construction. The validation-time gate walks the directed edge graph in both directions from each build with required_review_postures and verifies each required posture is the review_posture of at least one reachable review job. Runtime enforcement is preserved by today's edge-verdict gate plus run-completion semantics; no new runtime gate added in V1.

## Follow-Up

V1.5: revisit verdicts.posture column + introspection (RFC 0018 step 3)
