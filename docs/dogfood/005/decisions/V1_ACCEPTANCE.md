---
schema_version: striatum.decision.v1
decision_id: "dec_f3cb9562eabb48d2b8db23436719ecf2"
run_id: "run_833b407118184930b154288684dadbee"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "Accept RFC 0014 V1 build slice"
created_at: "2026-05-08T16:36:40Z"
---

# Accept RFC 0014 V1 build slice

Decision ID: `dec_f3cb9562eabb48d2b8db23436719ecf2`
Run ID: `run_833b407118184930b154288684dadbee`
Outcome: `accepted_with_follow_up`

## Follow-Up

Ship migrations v8+v9, post-exit validation + envelope, --timeout-seconds (capped at 86400 per F3), recovery process-reconcile + two doctor checks, status process_health summary. Adopt design-review F1 (clarify reconciler priority), F2 (single event type for V1; document SSE filter rule), F3 (adapter_timeout_seconds upper bound), F4 (namespace reproduction fixture output path), F5 (dual-update PROCESS_SCHEMA_SQL + schema.py), F6 (keep shell-string recovery_commands for V1).
