---
schema_version: striatum.decision.v1
decision_id: "dec_191214fea393400db73657720b6181bc"
run_id: "run_341193641a8e4e528333a704908acda4"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "Accept wrapper design and proceed to implementation"
created_at: "2026-05-08T08:50:06Z"
---

# Accept wrapper design and proceed to implementation

Decision ID: `dec_191214fea393400db73657720b6181bc`
Run ID: `run_341193641a8e4e528333a704908acda4`
Outcome: `accepted_with_follow_up`

## Follow-Up

Ship .striatum/bin/claude-supervised-wrapper.sh and the named-pipe verification test as designed. Adopt design-review F1 (drop --output-format stream-json from inner invocation), F3 (add EOF-after-one-packet test variant), F4 (BUILD_HANDOFF records V1.5 lint warning closure with before/after validate output), F5 (one-line UBIQUITOUS_LANGUAGE entry for 'supervised lane wrapper'), F6 (two-sentence CHANGELOG noting per-packet semantics). F2 (null-byte note in script header) is optional documentation.
