---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Go PostgreSQL Harness Migration
author: operator [self-declared: pg-harness-codex-gpt-5-001]

## Rows Replaced Or Retired

Replaced/strengthened:

- `tests/daemon_rpc/test_daemon_method_contract.py`: already covered by `go/pkg/rpc/registry_contract_test.go`; strengthened by `go/pkg/rpc/pg_harness_test.go`.
- `tests/daemon_pg/test_repo_registration.py` and `tests/daemon_pg/handlers/reads/test_registration.py`: strengthened by `go/pkg/repositories/pg_harness_test.go`.
- `tests/test_cross_repo_lifecycle.py`: already covered by `go/pkg/crossrepo/lifecycle_test.go`.
- `tests/test_daemon_go_audit.py`, `tests/test_daemon_go_mutations.py`, `tests/test_daemon_go_supervisor.py`: retire pytest shims after Go package checks are in the aggregate command.

Still not replaced:

- Live PostgreSQL fixture lifecycle from `tests/_harness/pg.py`.
- Live daemon lifecycle and recovery route coverage from `tests/test_daemon_pg.py`, `tests/test_daemon_pg_lifecycle.py`, and recovery evidence pytest files.

## Files Changed

- `go/pkg/rpc/pg_harness_test.go`
- `go/pkg/repositories/pg_harness_test.go`

## Command Evidence

- `cd go && go test ./cmd/striatum ./pkg/rpc ./pkg/repositories ./pkg/mutations ./pkg/reads` passed.
- `cd go && go test ./...` passed.

## Remaining Blockers

- A reusable Go PostgreSQL harness package is still missing; current Go coverage relies heavily on package-local fakes.
- Recovery side-effect tests need live route coverage before deleting the recovery pytest suite.
