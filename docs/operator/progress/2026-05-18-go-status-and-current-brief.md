---
schema_version: "striatum.progress_note.v1"
artifact_kind: "progress_note"
note_date: "2026-05-18"
session_slug: "go-status-and-current-brief"
related_plan: "plan_rfc-0069-pg-only-daemon-global-surfaces"
related_brief: "brief_2026-05-17_go-daemon-remediation"
retrieval_priority: "normal"
---

# Go Status and Current Brief
author: coordinator-codex-gpt-5.5-001

Go `status` now returns the PostgreSQL/Python read-model shape for jobs,
verdict posture counts, claimable queue messages, blockers, process health,
supervisor stalls, phase progress, provenance mode, auto-finalize dry-run
visibility, and next actions.

RFC 0058 V1.5 also landed: `striatum operator current-brief` reads the
current operator brief locally, and `operator_brief` context-budget overruns
are schema errors.

Follow-up diagnostic cleanup: `daemon status --json` now reports PostgreSQL
migration privilege failures as structured CLI errors with repair hints, and
`daemon doctor --postgres-url` threads that explicit URL into secondary daemon
diagnostics instead of probing implicit legacy registry configuration.
