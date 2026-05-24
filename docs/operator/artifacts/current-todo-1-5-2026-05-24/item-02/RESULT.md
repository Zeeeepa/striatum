---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["tests/architecture/test_legacy_sqlite_quarantine.py", "pyproject.toml", "docs/TODO.md", "docs/ROADMAP.md"]
---

# Item 2 Legacy SQLite Cleanup Result
author: operator [self-declared: current-todo-item2]

Result: complete.

Deleted six fully module-level-skipped legacy SQLite fixtures:

- `tests/test_cli_run_cancel.py`
- `tests/test_pause_resume.py`
- `tests/test_web_run_cancel.py`
- `tests/test_web_run_pause_resume.py`
- `tests/test_retry_job.py`
- `tests/test_recovery_resume.py`

The residual legacy-SQLite import allowlist and mypy exclusion regex were
trimmed to match. TODO and roadmap text now record that the quarantine narrowed
instead of moving the same debt elsewhere.

Validation:

```bash
.venv/bin/python -m pytest -q tests/architecture/test_legacy_sqlite_quarantine.py tests/daemon_pg/handlers/run_lifecycle/test_run_state.py::test_retry_job_revives_terminal_run_and_reenqueues_work tests/test_service.py::test_web_run_cancel_posts_daemon_rpc_without_sqlite tests/test_service.py::test_web_run_pause_resume_post_daemon_rpc_without_sqlite tests/test_cli_daemon_rpc_route.py::test_recovery_resume_preserves_blocker_and_completion_fields
```

Result: `19 passed`.
