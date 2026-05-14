---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/057/DESIGN_SYNTHESIS.md", "docs/dogfood/057/review/build/codex/REVIEW.md", "docs/dogfood/057/review/build/claude/REVIEW.md"]
---
author: designer-unknown-model-001

# RFC 0048 V1.5 Fix-up Design

## Decision

Finalize the RFC 0048 Phase A substrate migration by addressing the threat-model (codex F1-F4) and ergonomics (claude HIGH#1-#2, #4-#6) findings from dogfood-057. The primary goal is to close the "substrate facade" by ensuring fail-closed routing, verifiable audit chain continuity with row-level locking, and a functional Unix-socket accept loop.

## Track A: Router + Transport + Handler Internals

### 1. Fail-closed Routing (codex F1)

`DaemonRpcRouter._route` must never fall back to `CLI_ROUTES` / `striatum.api.invoke` / SQLite-backed dispatch once a method is identified as PG-backed.

- **Routing Rule**: If `resolve_pg_handler(envelope.method)` is not None, the method is PG-backed. Any exception (missing `pg_conn`, handler exception, parameter validation failure) must result in a terminal `RpcError` (exit 1).
- **Implementation**: Wrap the PG handler call in a `try...except` block. If the handler is resolved but `pg_conn` is missing, raise an `RpcError` instead of falling through to the `CLI_ROUTES` branch.
- **Negative Test**: Add a test that monkeypatches a registered PG handler to raise `ValueError` and asserts that `DaemonRpcRouter._route` raises `RpcError` and no `striatum.api.invoke` call occurs.

### 2. Audit-chain SERIALIZABLE / row-lock (codex F3)

Verifiable event continuity requires top-level columns and a row-locking head table to prevent chain forks during concurrent writes.

- **Migration 0006**:
  - `ALTER TABLE striatumd.events ADD COLUMN previous_hash bytea;`
  - `ALTER TABLE striatumd.events ADD COLUMN row_hash bytea;` (both initially NULL, populated via backfill).
  - `CREATE TABLE striatumd.repo_event_chain_heads (repository_id integer PRIMARY KEY, last_event_id bigint NOT NULL, last_row_hash bytea NOT NULL, updated_at timestamptz NOT NULL);`
- **Locking Pattern**: `RepoHandlerContext.append_event` must:
  1. Open a `SERIALIZABLE` transaction.
  2. `SELECT last_row_hash FROM striatumd.repo_event_chain_heads WHERE repository_id = %s FOR UPDATE`.
  3. Compute `row_hash` over the canonical event material (including `previous_hash`).
  4. `INSERT INTO striatumd.events (...)` with top-level hash columns.
  5. `UPDATE striatumd.repo_event_chain_heads` with the new head state.
- **Concurrent Test**: `tests/daemon_pg/handlers/test_event_hash_chain.py` must drive overlapping `claim_next` and `work.complete` calls for the same repository and verify a single contiguous `previous_hash` chain.

### 3. Unix-socket Accept Loop

The daemon must functional as a standalone server to eliminate the `STRIATUM_DAEMON_REQUIRED=0` escape path.

- **`run_daemon_foreground`**:
  - Use `asyncio.start_unix_server(path=socket_path())`.
  - Handler `_handle_client(reader, writer)`:
    - Perform handshake via `striatum.daemon_rpc.handshake`.
    - Read length-prefixed frames via `striatum.daemon_rpc.framing`.
    - Unpack `RpcEnvelope`, dispatch to `DaemonRpcRouter._route`.
    - Pack response and write frame back.
- **End-to-End Goal**: `striatum status` reaches the PG-backed handler via the Unix socket without falling back to SQLite.

### 4. Append-only Role Enforcement (codex F4)

- **Migration 0007**:
  ```sql
  REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;
  REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;
  REVOKE UPDATE, DELETE ON striatumd.audit_log FROM striatumd_rw;
  ```
- **Test**: Add a privilege-refusal test under `tests/daemon_pg/handlers/` that attempts to `DELETE FROM striatumd.events` using the `striatumd_rw` role and asserts a Postgres `insufficient_privilege` error.

## Track B: Tests + Schema + Docs + UX

### 1. Byte-equivalence Parity Rig (claude HIGH #1)

- **`tests/daemon_pg/handlers/conftest.py`**:
  - Implement `parity_seed(request)` fixture: seeds the same `Seed` data into both `sqlite_conn` and `pg_ctx`.
  - Implement `assert_state_parity(sqlite_conn, pg_conn, tables)`: uses `dictdiffer` to compare row dicts for all listed tables and asserts an empty diff.
- **Enforcement**: Remove the `RFC0048_PARITY` environment gate from Track B handler tests. Parity checks are now the default for all 16 ported methods.

### 2. Capability-denial Test Matrix (codex F2)

- **Scaffolding**: `tests/daemon_pg/handlers/test_capability_denial_matrix.py` (or a shared helper).
- **Matrix**: For every PG-backed write handler, test:
  - Missing token -> `RpcError(auth_denied)`.
  - Revoked/Expired token -> `RpcError(auth_denied)`.
  - Wrong capability (e.g., `read` for `work.complete`) -> `RpcError(capability_denied)`.
  - Wrong repository scope -> `RpcError(repo_not_registered)`.
  - Replayed `request_id` -> `RpcError(duplicate_request)`.
- **Assertion**: Verify no workflow table mutation and no audit-log append occurred on the allow path.

### 3. Dead Code Cleanup (claude HIGH #2)

- **Extract Helpers**:
  - `src/striatum/daemon_pg/handlers/workflow_loop/complete_job.py`: extract `complete_inline(ctx, session_id, job_id, lease_id, summary)`.
  - `src/striatum/daemon_pg/handlers/workflow_loop/ack_work.py`: extract `ack_inline(ctx, session_id, message_id, lease_id)`.
- **Wiring**: Update `recovery.resume --complete` and `recovery.auto` (live mode) in `recovery_evidence/` to call these `_inline` helpers instead of raising `InvalidTransitionError`.

### 4. `striatum daemon doctor --explain` (claude #5)

- **Registry Update**: Add `pg_backed: bool` to `MethodEntry.public_dict()`. Derived at import time via `resolve_pg_handler(method) is not None`.
- **Doctor Flag**: New flag `--explain` for `daemon doctor` that prints a table:
  | Method | PG-Backed | SQLite Fallback |
  |--------|-----------|-----------------|
  | status | Yes       | No              |
  | ...    | ...       | ...             |

### 5. `docs/POSTGRES_TRANSITION.md` Runbook

- **New Section**: "Provision the daemon-required role" under "Operator Runbook".
- **Content**: Copy-pasteable SQL for `CREATE ROLE striatumd_rw`, `GRANT CONNECT`, and the REVOKE/GRANT pattern for append-only tables.
- **Reference**: Link to the doctor refusal message that directs operators to this section when the role is missing or misconfigured.

## Implementation Lock

- **Handlers**: Use `_inline` extraction for all cross-track calls to ensure Track B does not re-enter the RPC router.
- **Transactions**: Every mutating handler remains in one `SERIALIZABLE` transaction.
- **Auditing**: `ctx.append_event` is the sole source of event provenance; it must be updated to use the 0006 columns.
- **Errors**: Fail-closed is the default. Any uncertainty in routing or handler state results in an RPC error.

## Out of Scope

- Multi-tenancy (RFC 0027).
- Go core parity (Phase B).
- Full SQLite removal (Phase C).
