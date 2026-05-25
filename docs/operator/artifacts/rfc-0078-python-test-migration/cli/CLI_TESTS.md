---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# CLI Test Migration
author: operator [self-declared: cli-tests-codex-gpt-5-001]

## Command Families Covered

- `striatum --help`
- unknown/retired top-level command refusal (`self-update`)
- `striatum workflow validate --json`
- `striatum workflow validate` same-model lint refusal and opt-in allowance
- daemon method contract metadata through Go RPC registry tests

## Rows Replaced, Retired, Or Blocked

- Covered: `tests/cli/test_parser_help.py`.
- Retired with evidence: `tests/cli/test_self_update.py`, `tests/cli/test_no_daemon_retired.py`, and `tests/cli/test_daemon_sqlite_import_retired.py` can be deleted after the Go CLI unknown/retired command gate.
- Needs replacement: daemon RPC process routing from `tests/test_cli_daemon_rpc_route.py`, `tests/cli/test_dispatch_daemon_doctor.py`, and exit-code coverage from `tests/exit_codes/test_rfc0043_refusals.py`.

## Files Changed

- `go/cmd/striatum/main_test.go`
- `go/pkg/rpc/pg_harness_test.go`

## Command Evidence

- `cd go && go test ./cmd/striatum ./pkg/rpc` passed as part of the focused Go command.

## Remaining CLI Parity Gaps

- The Go `striatum` CLI currently exposes only the workflow validation scaffold in tracked code. Most daemon client command families still need Go CLI implementations or explicit retirement decisions.
- Process-level exit-code tests should be added once the Go CLI routes daemon RPC methods.
