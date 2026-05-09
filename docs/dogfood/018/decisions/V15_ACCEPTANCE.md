---
schema_version: striatum.decision.v1
decision_id: "dec_7c30c5c37e7648f493ef8abf67a23eb6"
run_id: "run_436d02f8e8404dc1b953a7f2026c8df0"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0018 step 3 (V1.5) accepted with web UI chip CSS pinned"
created_at: "2026-05-09T05:03:41Z"
---

# RFC 0018 step 3 (V1.5) accepted with web UI chip CSS pinned

Decision ID: `dec_7c30c5c37e7648f493ef8abf67a23eb6`
Run ID: `run_436d02f8e8404dc1b953a7f2026c8df0`
Outcome: `accepted_with_follow_up`

## Rationale

V1.5 ships RFC 0018's deferred step 3: verdicts.posture column + per-surface introspection. Migration v10 ALTERs the verdicts table with a 'neutral' default that backfills existing rows; record_review_verdict reads the review job's posture from the workflow snapshot. Six surfaces gain posture rendering. One regression admitted: evidence-export adds a 'Posture' line to every verdict block, a format change downstream parsers must handle. Two findings folded into the implementation: web UI chip CSS rule pinned (gray background, max-width 12em, ellipsis, tooltip — Finding 2 acceptance-blocking), and dashboard sort uses (count desc, posture name asc) for deterministic tie-break (Finding 3).

## Follow-Up

Note Finding 1 in BUILD_HANDOFF and CHANGELOG: evidence-export format change
