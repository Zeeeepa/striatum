---
schema_version: "striatum.progress_note.v1"
artifact_kind: "progress_note"
note_date: "2026-05-17"
session_slug: "terminal-dashboard-daemon-dto"
related_plan: "plan_rfc-0069-pg-only-daemon-global-surfaces"
related_brief: "brief_2026-05-17_go-daemon-remediation"
retrieval_priority: "normal"
---

# Terminal Dashboard Daemon DTO Cutover
author: coordinator-codex-gpt-5.5-001

`striatum dashboard --run-id <id> --once` now renders text frames from the
daemon/PostgreSQL `dashboard` DTO in production. JSON single-run dashboard
calls and daemon-global `dashboard --all` remain RPC DTO surfaces.

The former repo-local SQLite payload gatherer now lives under
`striatum.legacy_sqlite.dashboard` and is reachable only through the paired
test-harness compatibility escape.
