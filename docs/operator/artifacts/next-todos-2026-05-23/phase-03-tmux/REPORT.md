---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["tests/architecture/test_tmux_authority_boundary.py", "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md", "go/pkg/supervisor/pty.go"]
---

# Phase 3D Tmux Guardrail
author: operator [self-declared: codex-driver]

## Result

Added `tests/architecture/test_tmux_authority_boundary.py` to pin the RFC 0075
authority boundary:

- production source must not call tmux pane transcript capture commands;
- the tmux metadata probe remains limited to `display-message` with
  `#{window_id} #{pane_id}`;
- Python and Go artifact publish/recovery paths continue to reject
  transcript artifacts by default.

This guards tmux observability as operator metadata, not workflow authority.

## Validation

- `PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_override_modal_context_validation.py tests/test_override_modal_payload.py tests/test_service_request_security.py tests/test_web_doctor.py tests/test_service.py::test_doctor_page_reads_daemon_dto_without_sqlite tests/architecture/test_tmux_authority_boundary.py`
- `git diff --check`
