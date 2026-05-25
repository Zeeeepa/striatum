---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Closure Aggregate Verify Summary
author: closer-codex-gpt-5-001

## Result

aggregate_status: green

All required aggregate gate commands exited 0 on 2026-05-25.

## Command Results

| Command | Result | Notes |
|---|---|---|
| `make python-trace-guardrail` | pass | Strict guardrail reported `blocked=0` and `unclassified=0`. |
| `cd go && go test ./... ; cd ..` | pass | All Go packages passed; packages without tests reported `[no test files]`. |
| `(cd src/striatum/web/frontend && npm test)` | pass | Vitest reported 6 test files passed and 35 tests passed. The run emitted React `act(...)` environment warnings from `recovery-panel.test.tsx`, but exited 0. |
| `scripts/go_release_metadata_check.sh` | pass | Exited 0 with no output. |
| `scripts/go_package_smoke.sh` | pass | Reported `go package smoke: ok`; PostgreSQL integration was skipped because `STRIATUM_DAEMON_DB_URL` is not set. |
| `scripts/go_fresh_clone_smoke.sh` | pass | Reported `go fresh clone smoke: ok`; PostgreSQL integration was skipped because `STRIATUM_DAEMON_DB_URL` is not set. |

## Python Trace Report Counts

Final `make python-trace-report` class counts:

| Classification | Count |
|---|---:|
| blocked | 0 |
| unclassified | 0 |
| historical_provenance | 820 |
| target_workload_allowed | 107 |

Class breakdown:

| Classification | Class | Count |
|---|---|---:|
| historical_provenance | archived_python_reference | 820 |
| target_workload_allowed | target_repo_python_reference | 107 |

## Failures

None.
