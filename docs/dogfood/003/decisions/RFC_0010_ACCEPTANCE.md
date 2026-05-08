---
schema_version: striatum.decision.v1
decision_id: "dec_6abd3957ab1748949ff0967221b346c4"
run_id: "run_0e6a74ae8feb481cbc18a4b1435552b6"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "Accept RFC 0010 first implementation slice"
created_at: "2026-05-08T07:09:11Z"
---

# Accept RFC 0010 first implementation slice

Decision ID: `dec_6abd3957ab1748949ff0967221b346c4`
Run ID: `run_0e6a74ae8feb481cbc18a4b1435552b6`
Outcome: `accepted_with_follow_up`

## Follow-Up

Implement reviewed V1 slice. Adopt design-review F2 (narrow V1 lint-warnings to unknown-sibling-field rule only; defer supervised-lane and missing-lane-command-path lints to V1.5). Adopt F3 (update DECISION_LOG.md and TODO.md alongside SPEC/README/CHANGELOG). Adopt F4 (add a test that loads the dogfood-003 fixture). Adopt F5 (rename 'compact projection' to 'passthrough projection' in SPEC + synthesis references). Adopt F6 (document prompt_envelope_path V1 validation: non-empty, no leading slash, no '..', existence not checked). Adopt F7 (SPEC notes lane-only profile reference for V1). F1: implementer or operator publishes at least one harness_improvement_proposal during run.
