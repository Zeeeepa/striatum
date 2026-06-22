---
schema_version: striatum.decision.v1
decision_id: "DECISION-rfc-0162-design-override-fold-f1-f2"
run_id: "run_623ba123a529b1c867186c759ac02015"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "Override RFC 0162 design needs_revision: commit with cycle-2 F1/F2 folded as binding criteria"
created_at: "2026-06-22T16:49:22Z"
---

# Override RFC 0162 design needs_revision: commit with cycle-2 F1/F2 folded as binding criteria

Decision ID: `DECISION-rfc-0162-design-override-fold-f1-f2`
Run ID: `run_623ba123a529b1c867186c759ac02015`
Outcome: `accepted_with_follow_up`

## Rationale

Cycle-2 needs_revision is a template limitation (revision edge routes to falsifier_1, never the holder, so HOLDER.md was never revised) plus two over-claim findings with concrete honest fixes, NOT fundamental defects. All load-bearing design is credited sound (codex-scoped L3, OQ4 operator-declared threshold, OQ3 numeric cap, Non-Goal/RFC0143 boundary). Authorize commit_proposal to fold cycle-2 F1 (per-lane roster vector striatum_lane_auth_expected{lane,provider,kind}; absence rule via unless/absent preserving lane label; narrow MVP to expiring OAuth creds, non-codex api_key deferred/accepted-risk) and F2 (provider-agnostic resolver CONTRACT that fails closed into a pageable resolver_mismatch when runtime source unproven, OR narrow the L1 same-credential claim) per each finding's closest_acceptable_answer in COLLABORATION_LEDGER_cycle_2.md. Safe because the narrowing route ships nothing broken and the rfc-0162-build (contract-first) + verify (game-day) runs catch any residual over-claim downstream.

## Follow-Up

commit_proposal folds F1+F2 per cycle-2 closest_acceptable_answer; rfc-0162-build SEED carries them as build acceptance criteria; rfc-0162-verify confirms no over-claim survives into code.
