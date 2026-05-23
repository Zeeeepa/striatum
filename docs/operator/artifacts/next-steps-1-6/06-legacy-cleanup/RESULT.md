---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/workflows/next-steps-1-6/prompts/track_06_legacy_cleanup.md", "docs/rfcs/0068-go-production-daemon-port.md", "docs/rfcs/0069-pg-only-daemon-global-surfaces.md", "docs/rfcs/0070-daemon-client-service-boundary.md"]
---

# Track 6 Result: Legacy Cleanup
author: operator
date: 2026-05-23

## Result

One safe legacy cleanup slice landed.

- The retired `STRIATUM_DAEMON_REGISTRY` production export was removed.
- Tests that still need the retired env var now use it as a raw compatibility
  input, not as a production constant.
- A guardrail blocks `STRIATUM_DAEMON_REGISTRY` from returning to
  `src/striatum`.
- Stale compatibility wording in source comments was cleaned up.

## Validation

- `.venv/bin/python -m pytest tests/architecture/test_legacy_sqlite_quarantine.py`
- `.venv/bin/python -m pytest tests/cli/test_daemon_sqlite_import_retired.py tests/cli/test_daemon_core.py tests/test_daemon_pg.py`
- `.venv/bin/python -m pytest tests/test_daemon_pg_audit.py tests/test_daemon_pg_doctor.py tests/test_daemon_pg_health.py tests/test_daemon_pg_lifecycle.py tests/test_daemon_pg_sweep.py`

