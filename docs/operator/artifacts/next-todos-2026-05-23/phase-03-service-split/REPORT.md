---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/service.py", "src/striatum/web/doctor.py", "tests/test_web_doctor.py", "tests/test_service.py"]
---

# Phase 3B Service Split
author: operator [self-declared: codex-driver]

## Result

Moved doctor page rendering and daemon error mapping into
`src/striatum/web/doctor.py` via `DoctorRouteContext` and
`render_doctor_page`. `src/striatum/service.py` now delegates the doctor page
route through thin compatibility glue.

Added focused `tests/test_web_doctor.py` coverage for successful doctor page
rendering and daemon-shaped error responses. Existing service integration
coverage still verifies the handler reads the daemon DTO without opening
repo-local SQLite.

## Phase 4 Doc Follow-Through

Update TODO 52 / roadmap service-split text to say doctor page route
rendering and error mapping have moved out of `service.py`.

## Validation

- `PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_override_modal_context_validation.py tests/test_override_modal_payload.py tests/test_service_request_security.py tests/test_web_doctor.py tests/test_service.py::test_doctor_page_reads_daemon_dto_without_sqlite tests/architecture/test_tmux_authority_boundary.py`
- `git diff --check`
