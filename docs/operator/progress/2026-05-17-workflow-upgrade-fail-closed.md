---
schema_version: "striatum.progress_note.v1"
artifact_kind: "progress_note"
note_date: "2026-05-17"
session_slug: "workflow-upgrade-fail-closed"
related_plan: "plan_rfc-0069-pg-only-daemon-global-surfaces"
related_brief: "brief_2026-05-17_go-daemon-remediation"
retrieval_priority: "normal"
---

# Workflow Upgrade Fail-Closed Hardening
author: coordinator-codex-gpt-5.5-001

`workflow upgrade` now refuses whenever daemon PostgreSQL running-run state is
unknown. The legacy repo-local SQLite guard only remains available under the
paired test-harness compatibility escape, and the refusal applies to both
normal harness-profile upgrades and `--add-phases --apply`.

The remaining RFC 0069 production read-path gap is the terminal dashboard's
legacy repo-local payload gatherer.
