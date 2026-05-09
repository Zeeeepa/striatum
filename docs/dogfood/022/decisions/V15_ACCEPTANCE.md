---
schema_version: striatum.decision.v1
decision_id: "dec_321ad5c93a9640f3bbe0940f5cecb598"
run_id: "run_8f44b5148cb0494ead1026ed6fe53b63"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0023 V1.5 (chat tool use + briefing) accepted with prompt-injection delimiter fix"
created_at: "2026-05-09T16:26:38Z"
---

# RFC 0023 V1.5 (chat tool use + briefing) accepted with prompt-injection delimiter fix

Decision ID: `dec_321ad5c93a9640f3bbe0940f5cecb598`
Run ID: `run_8f44b5148cb0494ead1026ed6fe53b63`
Outcome: `accepted_with_follow_up`

## Rationale

V1.5 ships six read-only chat tools (read_file, list_dir, striatum_status, striatum_why, git_log, git_diff), a system-prompt briefing at chat creation, JSONL role extensions for tool_use/tool_result, and three ride-along fixes from v1.12.0 dogfooding. F1 (acceptance-blocking): wrap tool results in delimiters and instruct the model to treat content as data. F2/F3: notes folded into implementation.

## Follow-Up

Bundle ride-along fixes: graph-node route tolerance + doctor template + chat double-render
