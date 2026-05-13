author: designer-unknown-model-001

# DESIGN: RFC 0039 V1.5 (Go daemon findings F1-F5)

This design addresses the high and medium severity findings (F1-F5) identified in the dogfood-042 Track A build review of the Go daemon core.

## F1. Go RPC fail-open authorization

The Go daemon currently uses `rpc.AllowAllAuthorizer{}` in `go/cmd/striatumd/main.go:53`, which grants access to all methods regardless of the token.

### Proposed Change
Replace `AllowAllAuthorizer` with a `PostgresAuthorizer` that validates tokens and capabilities against the `striatumd.clients` and `striatumd.client_capabilities` tables defined in `go/pkg/db/sql/0001_baseline.sql:66-88`.

### Specification
- **Interface**: Implement the `Authorizer` interface defined in `go/pkg/rpc/capability.go:19`.
- **Lookup Path**:
    1. Split the token into `token_id` and `secret` (`go/pkg/rpc/capability.go:146`).
    2. Query `striatumd.clients` by `token_id` to get `token_hash` and `token_salt`.
    3. Validate `secret` against `token_hash` using Argon2id (replacing the current SHA-256 in `go/pkg/rpc/capability.go:141`).
    4. Query `striatumd.client_capabilities` for the required capability and `repository_id`.
- **Cache Policy**: No caching for Phase 1. Every RPC call (beyond `daemon.hello`) performs a DB lookup to ensure immediate revocation/expiry enforcement.
- **Denial Response**: Return `AuthContext` with `Decision: "denied"` and a `DenialReason` (e.g., `token_invalid`, `token_expired`, `capability_missing`) as defined in `go/pkg/rpc/capability.go:12-17`.
- **Audit Emission**: The `AuditRecorder.RecordRPC` (`go/pkg/db/audit.go:42`) already takes the `AuthContext` and records the decision and denial reason.

### Parity
The implementation must match the Python parity surface in `tests/_harness/tokens.py`.

## F2. `daemon_core="go"` harness launch broken

The harness launch path in `tests/_harness/daemon.py:117-125` is inconsistent with the flags and build output of the Go daemon.

### Proposed Change
1. Align `go/Makefile` to produce the binary at `go/bin/striatumd`.
2. Align the Go `main.go` flags with the harness expectations.

### Specification
- **`go/Makefile`**: Update the `build` target to use `-o bin/striatumd`.
    ```makefile
    build:
        go build -o bin/striatumd ./cmd/striatumd
    ```
- **Go `main.go` flags**: Update `go/cmd/striatumd/main.go:24-28` to accept `--db-url` (alias for `--postgres-url`) and `--migrations-dir`.
- **Harness launch**: Ensure `tests/_harness/daemon.py:117-125` passes the correct flags.
- **Smoke Test**: Add a test case in `tests/test_daemon.py` that instantiates `MultiRepoHarness(daemon_core="go")` and performs a `daemon.hello` handshake to prove the launch works.

## F3. `make test-multi-repo CORE=go` not wired

The top-level `Makefile` and `tests/conftest.py` do not support selecting the Go core for multi-repo tests.

### Proposed Change
Plumb the `CORE` variable from the `Makefile` to `pytest` and parametrize the harness fixture.

### Specification
- **Top-level `Makefile`**: Update `test-multi-repo` to pass `CORE` to `pytest`.
    ```makefile
    test-multi-repo: $(VENV)/.installed
        $(PYTHON) -m pytest -m multi_repo \
            --daemon-core=$(CORE) \
            tests/test_multi_repo_harness.py ...
    ```
- **Pytest Parametrization**: In `tests/conftest.py`, add a `--daemon-core` command-line option and use it to set the `daemon_core` parameter when constructing `MultiRepoHarness` in `tests/_harness/multi_repo.py:32`.
- **Test Opt-in**: Initially, only the read-only and Step 1+2 smoke tests will opt-in to the Go core.

## F4. Go audit-chain race

`go/pkg/db/audit.go:57-87` performs a non-transactional read-then-write for the audit chain, which is subject to race conditions.

### Proposed Change
Wrap the audit append and chain-head update in a single PostgreSQL transaction.

### Specification
- **Transactional Wrapper**: Use `BEGIN; ... COMMIT;` around the `INSERT INTO striatumd.audit_log` and `UPDATE striatumd.audit_chain_head`.
- **Isolation**: Use `READ COMMITTED` isolation. The `UPDATE` on `striatumd.audit_chain_head` (which is a singleton row) will naturally serialize concurrent appenders by locking the row.
- **Hash Chain Read**: Read the `previous_hash` from `striatumd.audit_chain_head` *inside* the transaction after the row is locked by the `UPDATE` (e.g., using `UPDATE ... RETURNING`).
- **Regression Test**: Add a test in `go/pkg/db/audit_test.go` that spawns multiple goroutines attempting to `RecordRPC` simultaneously and verifies the resulting hash chain is linear and valid.

## F5. Replace `psql` shell-out

The `PsqlRunner` in `go/pkg/db/connection.go:110, 119` shells out to `psql`, which is unpinned and introduces environment risks.

### Proposed Change
Migrate to the `pgx` pure-Go driver.

### Specification
- **Driver Choice**: `pgx` (v5). Justification: It is the standard, high-performance, pure-Go PostgreSQL driver with excellent support for modern PostgreSQL features and connection pooling.
- **Migration**: Replace `PsqlRunner` with a `pgx`-backed implementation of the `Runner` interface. Use `database/sql` with the `pgx` driver or use `pgxpool` directly for better performance.
- **`go.mod` Changes**: Add `github.com/jackc/pgx/v5` to `go/go.mod`.
- **Connection String**: `pgx` handles the standard `postgres://` URL format.
- **TLS**: `pgx` supports TLS via the connection string or explicit `tls.Config`. The implementation will respect `sslmode` in the URL.
