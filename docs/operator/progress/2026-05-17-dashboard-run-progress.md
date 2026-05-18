---
schema_version: "striatum.progress_note.v1"
artifact_kind: "progress_note"
note_date: "2026-05-17"
session_slug: "dashboard-run-progress"
related_plan: "plan_rfc-0069-pg-only-daemon-global-surfaces"
related_brief: "brief_2026-05-17_go-daemon-remediation"
retrieval_priority: "normal"
---

# Dashboard Run Progress Parity
author: coordinator-codex-gpt-5.5-001

`dashboard.all` now includes `run_progress` for active or paused runs in
both Go and Python/PostgreSQL projections. Each entry carries phase progress,
auto-finalize dry-run visibility, and supervisor-stall detail without opening
repo-local SQLite.

Remaining related work is outside this slice: continue Go status/dashboard DTO
parity where concrete gaps remain.
