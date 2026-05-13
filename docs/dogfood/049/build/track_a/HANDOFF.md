# Track A Handoff

author: implementer-unknown-model-001
date: 2026-05-13
status: partial

## Shipped Scope

Implemented the Go daemon Track A foundation inside the allowed Go packages:

- Expanded `go/pkg/rpc/registry.go` to the RFC 0043 canonical dotted method vocabulary, including `session.register`, `work.*`, `artifact.publish`, `review.*`, `decision.record`, `checkpoint.resolve`, `recovery.*`, `worktree.*`, `branch.confirm`, `run.*`, and `workflow.*`. Legacy undotted aliases remain registered and marked deprecated.
- Added `go/pkg/apply/{receipt.go,service.go}` with apply receipt lookup and fail-closed sealed-apply behavior matching the Python daemon skeleton: missing key returns `sealed_key_missing`; loaded key still returns `apply_gate_unsatisfied`.
- Added `go/pkg/mcp/{capabilities.go,tools.go}` with capability-filtered tool visibility and `tools/call` dispatch through the Go RPC server without requiring a synthetic handshake.
- Added `go/pkg/crossrepo/{prepare.go,lifecycle.go}` with cross-repo lifecycle helpers over the existing Postgres tables. Read paths for list/describe/why are wired; local participant mutation still requires a real local runner.
- Wired `go/cmd/striatumd/main.go` to register apply and cross-repo handlers and to register stable fail-closed handlers for the broader mutation surface so calls audit and return deterministic `not_implemented` instead of `method_unknown`.
- Added `Query` methods to the Go Postgres runner adapters for row-set queries used by cross-repo helpers.
- Added focused Go tests for registry parity, apply fail-closed behavior, MCP capability filtering, and cross-repo workflow parsing.

Implemented the narrow Python CLI surface inside the allowed files:

- Added `daemon start --core {python,go}` to `src/striatum/cli/parser.py`, defaulting from `STRIATUM_DAEMON_CORE` with the Phase 2 fallback still resolved as `python`.
- Added daemon-core resolver and Go binary resolver/launcher helpers in `src/striatum/cli/daemon.py`. Resolution order is packaged `_daemongo` public resolver when present, `STRIATUMD_GO_BIN`, in-tree `go/bin/striatumd`, then PATH.
- Added Python tests for parser/env/default behavior and Go binary env override.
- Added a gated Go-daemon mutation registry smoke test in `tests/test_daemon_go_mutations.py`.

## Deviations

`src/striatum/cli/dispatch.py` is in this packet's forbidden paths. The current dispatcher still calls `striatum.daemon.run_daemon_foreground(...)` directly for `daemon start`, so the new `launch_daemon_start(...)` helper is present but not connected. Completing CLI launch dispatch requires a follow-up edit in that forbidden file or a Track B-owned dispatcher change.

The Go daemon registers the mutation surface and audits calls, but repo-local mutation execution is intentionally fail-closed with `not_implemented` for most methods. The Python daemon still owns the actual CLI-backed mutation behavior today.

## Verification

- `cd go && go build ./...`: passed.
- `cd go && go test ./...`: passed.
- `STRIATUM_DAEMON_REQUIRED=0 pytest -q tests/cli/test_daemon_core.py tests/daemon_rpc/test_registry_rfc0043_coverage.py`: passed, `12 passed`.
- `STRIATUM_DAEMON_REQUIRED=0 pytest -q tests/test_daemon_go_mutations.py`: skipped as expected without `STRIATUM_MULTI_REPO_DAEMON_CORE=go`.
- `.venv/bin/python -m ruff check src/striatum/cli/daemon.py src/striatum/cli/parser.py tests/cli/test_daemon_core.py tests/test_daemon_go_mutations.py`: passed.
- `.venv/bin/python -m mypy src/striatum/cli/daemon.py src/striatum/cli/parser.py tests/cli/test_daemon_core.py tests/test_daemon_go_mutations.py`: passed.

Broader verification is currently blocked by pre-existing branch failures outside this Track A slice:

- `make lint`: failed on unused `sqlite3` in `tests/daemon_pg/test_repo_local_migration_crash_resume.py`.
- `make typecheck`: failed in `src/striatum/daemon_pg/repo_local_migration.py`, `tests/daemon_rpc/test_registry_rfc0043_coverage.py`, and `tests/exit_codes/test_rfc0043_refusals.py`.
- `make test`: `768 passed, 43 skipped, 14 failed`; failures are in repo-local migration crash-resume, RFC0043 refusal tests, legacy daemon tests, daemon DB version assertion, and the D094 decision-log word budget.
