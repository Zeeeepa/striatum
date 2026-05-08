---
schema_version: striatum.decision.v1
decision_id: "dec_ae012b59f3a745cb922fa3b8cba90fd0"
run_id: "run_c8cd066bc1344571bf875683d4edb892"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "Accept RFC 0012 V1 build slice"
created_at: "2026-05-08T17:23:01Z"
---

# Accept RFC 0012 V1 build slice

Decision ID: `dec_ae012b59f3a745cb922fa3b8cba90fd0`
Run ID: `run_c8cd066bc1344571bf875683d4edb892`
Outcome: `accepted_with_follow_up`

## Follow-Up

Ship src/striatum/service.py with HTTP+Unix-socket server, all V1 endpoints, SSE events, --allow-mutations gating, --token auth, non-loopback refusal, PID-file lifecycle. Adopt design-review F1 (token timing-safe compare), F2 (close SSE connection on disconnect/shutdown), F3 (warn that --web is a no-op in V1).
