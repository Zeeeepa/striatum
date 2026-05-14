---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/058/design/codex/DESIGN.md", "docs/dogfood/058/design/claude_code/DESIGN.md", "docs/dogfood/058/design/gemini/DESIGN.md"]
---
author: designer-unknown-model-001

# RFC 0048 V1.5 Fix-up Scope Lock

## Decision

RFC 0048 V1.5 hardens the 16 Phase A PostgreSQL handlers from dogfood-057 so they can become the daemon-required path without the `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` escape. Track A owns router, transport, handler transaction discipline, and SQL privileges. Track B owns parity and denial tests, schema migration, dead-code cleanup, doctor UX, and the Postgres transition runbook.

This attempt is the operative V1.5 scope lock for the current synthesis job.

Track A can land independently. Track B can develop in parallel, but integration order is Track A then Track B because `daemon doctor --explain` and the denial/parity rigs read routing, registry, and role-grant behavior Track A changes. This release ships as v1.50.0 by the existing `pyproject.toml` version convention.

## Track A: Codex

### A1. Fail-closed routing

File: `src/striatum/daemon_pg/handlers/registry.py`.

Add:

```python
def is_pg_backed(method: str) -> bool:
    return method in _PG_HANDLERS
```

Keep `register_pg_handler(*methods: str) -> Callable[[PgHandler], PgHandler]` and `resolve_pg_handler(method: str) -> PgHandler | None`; export all three registry functions and keep `_PG_HANDLERS` private.

File: `src/striatum/daemon_rpc/server.py`.

Change `DaemonRpcRouter._route(self, envelope: RpcEnvelope, *, repo_root: Path, auth: RpcAuthContext) -> dict[str, Any]` so it imports `striatum.daemon_pg.handlers` before registry lookup, then checks `is_pg_backed(envelope.method)` before `CLI_ROUTES`. A PG-backed method is terminal:

- `self.pg_conn is None` raises `RpcError("daemon_db_missing", "PG-backed route requires daemon PostgreSQL", exit_code=10)`.
- missing repository scope raises `RpcError("repo_not_registered", "PG handler requires repository_id")`.
- handler `RpcError` is re-raised.
- non-`RpcError` handler exceptions raise `RpcError("handler_failed", f"{type(exc).__name__}: {exc}", exit_code=1)`.
- no PG-backed path calls `striatum.api.invoke`, `striatum.db.connect`, or `CLI_ROUTES`.

Error envelope shape stays the existing RPC response:

```json
{"ok": false, "error": {"code": "daemon_db_missing|repo_not_registered|handler_failed|...", "message": "..."}}
```

Regression test: `tests/daemon_rpc/test_fail_closed_routing.py`.

The test parameterizes all 16 Phase A methods, monkeypatches each resolved PG handler to raise, monkeypatches `striatum.api.invoke` and `striatum.db.connect` to fail the test if called, routes through `DaemonRpcRouter.handle(...)`, and asserts `ok is false` with the PG error code.

### A2. Handler transactions and event-chain locking

File: `src/striatum/daemon_pg/handlers/context.py`.

Add:

```python
@contextmanager
def write_transaction(self, *, retries: int = 1) -> Iterator[None]:
    ...
```

`RepoHandlerContext.write_transaction()` opens `BEGIN ISOLATION LEVEL SERIALIZABLE`, commits on success, rolls back on failure, and retries exactly one `SerializationFailure` before raising `RpcError("serialization_failure", ...)`.

File: `src/striatum/daemon_rpc/server.py`.

`DaemonRpcRouter._route(...)` wraps every PG write handler in `ctx.write_transaction()`. Read-only handlers use `REPEATABLE READ READ ONLY` through a registry flag added to `register_pg_handler(*methods: str, read_only: bool = False)`.

File: `src/striatum/daemon_pg/handlers/context.py`.

Rewrite `RepoHandlerContext.append_event(...) -> int` to lock the per-repo chain head:

```sql
SELECT last_event_id, last_row_hash
FROM striatumd.repo_event_chain_heads
WHERE repository_id = %s
FOR UPDATE
```

Then insert `striatumd.events.previous_hash = head.last_row_hash`, compute `row_hash` with `canonical_event_hash(row, previous_hash=previous_hash)`, insert the event row, and update `repo_event_chain_heads.last_event_id`, `last_row_hash`, and `updated_at` before the transaction commits. Every mutating handler in `workflow_loop/` and `recovery_evidence/` must call `append_event()` inside this transaction and must lock the run/job/message/lease rows it mutates with `FOR UPDATE`.

Concurrent test: `tests/daemon_pg/handlers/test_event_hash_chain_concurrent.py`.

It drives overlapping allowed and denied calls for `work.claim_next`, `artifact.publish`, `review.verdict`, `work.complete`, and `recovery.requeue_stale`; verifies one contiguous per-repository `previous_hash` chain; verifies denied calls leave workflow tables unchanged; and verifies no workflow mutation commits without its business event.

### A3. Unix-socket accept loop

File: `src/striatum/daemon.py`.

Change `run_daemon_foreground(*, sweep_interval_seconds: float = 60.0, max_sweeps: int | None = None, postgres_url: str | None = None) -> dict[str, Any]`.

Use the existing synchronous daemon shape with one accept thread:

- Bind through `striatum.daemon_rpc.transport_unix.bind_unix_socket(socket_path())` so the socket remains owner-only and `0600`.
- Start an accept thread before the sweep loop.
- The accept thread calls `sock.accept()` with a short timeout while a `threading.Event` is unset.
- Each accepted connection gets a daemon thread.
- The connection handler wraps `conn.makefile("rwb")`, reads envelopes via `striatum.daemon_rpc.framing.read_envelopes(stream)`, calls `DaemonRpcRouter.handle(envelope, connection_id=<uuid>, transport="unix", require_handshake=True)`, and writes through `striatum.daemon_rpc.framing.write_response(stream, response)`.
- The same connection must send `daemon.hello` before ordinary calls; the router's existing connection-local handshake set remains the enforcement point.
- SIGTERM/SIGINT sets the stop event, closes the listener to break `accept()`, joins the accept thread briefly, closes live connection threads best-effort, and unlinks socket and pid files in the existing `finally`.

Router construction: create `DaemonRpcRouter(pg_conn=<daemon PG connection>, repo_root=<registered active repo root when known>, substrate_schema=<doctor version>)` in `run_daemon_foreground` after `daemon_pg.connection.doctor(..., apply=True)` succeeds.

End-to-end test: `tests/daemon_rpc/test_unix_daemon_foreground.py`.

It starts `run_daemon_foreground(...)` against a temp runtime dir and test Postgres, calls the Unix client for `daemon.hello`, then calls `status` with a registered repository id and asserts an audit/request-log row shows transport `"unix"` and the request reached `DaemonRpcRouter._route`.

### A4. Append-only role enforcement

File: `src/striatum/daemon_pg/sql/0007_daemon_role_grants.sql`.

Do not edit `0001_baseline.sql` or `0005_repo_local_workflow_state.sql`. Add sequential migration 0007 after Track B's 0006.

The migration body is:

```sql
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    RAISE EXCEPTION
      'striatumd_rw role missing; provision per docs/POSTGRES_TRANSITION.md'
      USING ERRCODE = 'insufficient_privilege';
  END IF;
END;
$$;

REVOKE ALL ON SCHEMA striatumd FROM striatumd_rw;
GRANT USAGE ON SCHEMA striatumd TO striatumd_rw;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA striatumd TO striatumd_rw;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw;

REVOKE UPDATE, DELETE ON striatumd.audit_log FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;
GRANT SELECT, INSERT ON striatumd.audit_log TO striatumd_rw;
GRANT SELECT, INSERT ON striatumd.events TO striatumd_rw;
GRANT SELECT, INSERT ON striatumd.artifacts TO striatumd_rw;

REVOKE DELETE ON striatumd.repo_event_chain_heads FROM striatumd_rw;
```

Privilege test: `tests/daemon_pg/test_role_grants.py`.

It applies migrations 0001-0007, connects as `striatumd_rw`, asserts `INSERT` into `events`, `artifacts`, and `audit_log` works with required columns, asserts `UPDATE` and `DELETE` on those tables fail with `insufficient_privilege`, asserts `UPDATE` on `repo_event_chain_heads` works, and asserts `DELETE` on `repo_event_chain_heads` fails.

Static regression test: `tests/daemon_pg/test_handler_no_upsert_on_append_only.py`.

It scans `src/striatum/daemon_pg/handlers/` and fails on `ON CONFLICT DO UPDATE` targeting `striatumd.events`, `striatumd.artifacts`, or `striatumd.audit_log`.

## Track B: Claude

### B1. Byte-equivalence parity rig

Move `tests/daemon_pg/handlers/recovery_evidence/conftest.py` to `tests/daemon_pg/handlers/conftest.py`.

Add `tests/daemon_pg/handlers/_parity.py` with:

```python
@dataclass(frozen=True)
class Seed: ...

@pytest.fixture
def parity_seed(pg_ctx: RepoHandlerContext, sqlite_conn: sqlite3.Connection, tmp_path: Path) -> Seed: ...

def state_snapshot(conn_or_pg: Any, *, tables: Sequence[str]) -> dict[str, list[dict[str, Any]]]: ...

def assert_state_parity(pg_state: Mapping[str, Any], sqlite_state: Mapping[str, Any]) -> None: ...
```

`assert_state_parity(...)` raises an `AssertionError` with table, primary key, column, SQLite value, and PG value for every mismatch. It normalizes generated ids, timestamps, `repository_id`, and chain-hash columns before comparing.

Remove every `RFC0048_PARITY` skip gate. The only skip is the repository-wide Postgres fixture skip when no `STRIATUM_TEST_POSTGRES_URL` is available.

Wire one `test_parity_with_sqlite` into all 16 handler test files:

- `tests/daemon_pg/handlers/workflow_loop/test_register_session.py`
- `tests/daemon_pg/handlers/workflow_loop/test_claim_next.py`
- `tests/daemon_pg/handlers/workflow_loop/test_ack_work.py`
- `tests/daemon_pg/handlers/workflow_loop/test_complete_job.py`
- `tests/daemon_pg/handlers/workflow_loop/test_release_lease.py`
- `tests/daemon_pg/handlers/workflow_loop/test_block_job.py`
- `tests/daemon_pg/handlers/workflow_loop/test_record_verdict.py`
- `tests/daemon_pg/handlers/workflow_loop/test_submit_review.py`
- `tests/daemon_pg/handlers/workflow_loop/test_override_review_verdict.py`
- `tests/daemon_pg/handlers/recovery_evidence/test_stale_leases.py`
- `tests/daemon_pg/handlers/recovery_evidence/test_requeue_stale.py`
- `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py`
- `tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py`
- `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py`
- `tests/daemon_pg/handlers/recovery_evidence/test_auto_publish_stale_artifacts.py`
- `tests/daemon_pg/handlers/recovery_evidence/test_evidence_export.py`

Each test invokes the PG handler and the matching SQLite path on the same seed, normalizes the response, then compares touched tables and event payloads with `assert_state_parity`.

### B2. Capability-denial matrix

Add `tests/daemon_pg/handlers/_denial.py`.

Required helper signature:

```python
def assert_denied_without_workflow_mutation(
    method: str,
    params: Mapping[str, Any],
    denial_case: DenialCase,
    *,
    router: DaemonRpcRouter,
    pg_ctx: RepoHandlerContext,
) -> None:
    ...
```

`DenialCase` covers exactly six cases per handler:

- missing token -> `token_missing`
- revoked token -> `token_revoked`
- expired token -> `token_expired`
- wrong required capability -> `capability_required`
- wrong repository scope -> `repo_not_registered`
- replayed `request_id` -> `duplicate_request`

Every case snapshots workflow tables, `striatumd.events`, `striatumd.artifacts`, and `striatumd.audit_log`; calls `DaemonRpcRouter.handle(...)`; asserts an RPC error with the expected code; asserts no workflow/event/artifact mutation; and asserts exactly one denied audit row for parseable envelopes.

Per-handler additions go in the same 16 test files listed in B1. Add one separate `test_replay_after_success` per handler: first allowed request commits, second request reuses the same `request_id`, returns `duplicate_request`, and appends no second business event or allow audit row.

### B3. Schema migration 0006

File: `src/striatum/daemon_pg/sql/0006_event_chain_columns.sql`.

Sequential version 0006 is unused on main today, so there is no ordering conflict.

DDL:

```sql
ALTER TABLE striatumd.events
  ADD COLUMN IF NOT EXISTS previous_hash bytea,
  ADD COLUMN IF NOT EXISTS row_hash bytea;

CREATE TABLE IF NOT EXISTS striatumd.repo_event_chain_heads (
  repository_id text PRIMARY KEY
    REFERENCES striatumd.repositories(repository_id),
  last_event_id bigint,
  last_row_hash bytea,
  updated_at timestamptz NOT NULL DEFAULT now()
);

UPDATE striatumd.events
SET previous_hash = NULLIF(decode(coalesce(payload_json->'_event_chain'->>'previous_hash', ''), 'hex'), ''::bytea),
    row_hash = NULLIF(decode(coalesce(payload_json->'_event_chain'->>'row_hash', ''), 'hex'), ''::bytea)
WHERE row_hash IS NULL
  AND payload_json ? '_event_chain';

CREATE UNIQUE INDEX IF NOT EXISTS ux_events_repository_row_hash
  ON striatumd.events(repository_id, row_hash)
  WHERE row_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_events_repo_chain
  ON striatumd.events(repository_id, event_id);
```

Python re-anchor hook: add `src/striatum/daemon_pg/migrations.py::reanchor_event_chain_0006(conn: Any) -> None`.

The hook runs in the migration transaction after the DDL. It orders rows by `(repository_id, event_id)`, uses `canonical_event_hash(...)` to compute missing `row_hash` values, fills `previous_hash`, sets `row_hash NOT NULL`, backfills one `repo_event_chain_heads` row per repository from the trailing event, and bumps schema metadata through the existing `schema_migrations` mechanism. It is idempotent: rows with `row_hash` already present are verified, not rewritten.

Tests:

- `tests/daemon_pg/handlers/test_migration_0006_backfill.py` seeds 0005-style `_event_chain` payload rows, applies 0006, and asserts top-level columns match byte-for-byte.
- `tests/daemon_pg/handlers/test_event_hash_chain.py` asserts first event `previous_hash IS NULL`, multi-event appends chain in a single transaction, and the head row advances.

### B4. Dead-code cleanup

Keep and wire all four symbols/surfaces:

- `complete_inline`: define and wire. `recovery.resume --complete` and `recovery.auto` live mode are real operator recovery paths; deleting them would remove documented functionality.
- `ack_inline`: define and wire. `recovery.auto` live mode needs idempotent ack before publish/complete.
- `recovery.resume --complete`: keep. It resolves the blocker and completes the job in one transaction.
- `recovery.auto`: keep. Dry-run is preview; live mode is the recovery primitive.

File: `src/striatum/daemon_pg/handlers/workflow_loop/complete_job.py`.

Add:

```python
def complete_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str | None,
    summary: Mapping[str, Any] | None = None,
    force: bool = False,
) -> dict[str, Any]:
    ...
```

`handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]` normalizes params and calls `complete_inline(...)`.

File: `src/striatum/daemon_pg/handlers/workflow_loop/ack_work.py`.

Add:

```python
def ack_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    message_id: str,
    lease_id: str,
) -> None:
    ...
```

`handle(...)` calls `ack_inline(...)`.

File: `src/striatum/daemon_pg/handlers/recovery_evidence/resume_blocker.py`.

Replace the local `_complete_inline(...)` fallback with an import of `complete_inline(...)` and return `completed_inline: true` when it runs.

File: `src/striatum/daemon_pg/handlers/recovery_evidence/auto_publish_stale_artifacts.py`.

Import `ack_inline(...)`, `complete_inline(...)`, and the existing PG artifact helper; execute ack, publish, complete, and `recovery.auto_published` in one transaction per live candidate.

Tests:

- `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py::test_resume_with_complete_runs_inline`
- `tests/daemon_pg/handlers/recovery_evidence/test_auto_publish_stale_artifacts.py::test_live_mode_ack_publish_complete_rolls_back_on_failure`

Also require `grep -rn 'InvalidTransitionError.*requires Track A' src/striatum/` to return zero hits.

### B5. `daemon doctor --explain`

File: `src/striatum/daemon_rpc/registry.py`.

Extend `MethodEntry.public_dict(self) -> dict[str, object]` to include:

```python
"pg_backed": is_pg_backed(self.method),
"sqlite_fallback_active": self.method in CLI_ROUTES and not is_pg_backed(self.method),
"substrate": "pg" | "cli_fallback" | "native_daemon",
```

Import handler registration before computing those fields.

File: `src/striatum/cli/parser.py`.

Add `--explain` to the existing `daemon doctor` argparse branch.

File: `src/striatum/cli/introspect.py`.

Add:

```python
def explain_daemon_methods() -> dict[str, Any]:
    ...
```

JSON output shape:

```json
{
  "method_substrates": [
    {
      "method": "work.ack",
      "pg_backed": true,
      "sqlite_fallback_active": false,
      "substrate": "pg",
      "required_capability": "claim",
      "repository_scope_mode": "single_repo"
    }
  ],
  "totals": {"pg_backed": 16, "sqlite_fallback_active": 30, "native_daemon": 0}
}
```

Text output table:

```text
method                         pg-backed  sqlite-fallback  capability  scope
work.ack                       yes        no               claim       single_repo
run.summary                    no         yes              read        single_repo
daemon.describe                native     no               read        daemon_global
```

Test: `tests/daemon_rpc/test_doctor_explain.py`.

It asserts all 16 Phase A methods report `pg_backed=true`, no Phase A method reports `sqlite_fallback_active=true`, and the row count matches `METHOD_REGISTRY`.

### B6. `POSTGRES_TRANSITION.md` runbook

File: `docs/POSTGRES_TRANSITION.md`.

Add heading before daemon startup:

```markdown
## Provision the daemon-required role
```

Include this SQL block:

```sql
CREATE ROLE striatumd_rw LOGIN PASSWORD '<local secret>';
GRANT CONNECT ON DATABASE striatum_daemon TO striatumd_rw;
GRANT USAGE ON SCHEMA striatumd TO striatumd_rw;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA striatumd TO striatumd_rw;
-- Migration 0007 grants table-level runtime privileges and revokes
-- UPDATE/DELETE on append-only tables. Do not grant those manually.
```

Doctor refusal quote:

```text
daemon_role_grants: striatumd_rw role missing; provision per docs/POSTGRES_TRANSITION.md#provision-the-daemon-required-role
```

Rewrite the existing `## RFC 0048 remaining work` section to:

```markdown
## RFC 0048 status (v1.49.0+)
```

It must enumerate the 16 landed PG-backed methods and handler modules, state that V1.5 is hardening fail-closed routing / parity / denial / chain columns / role grants, and leave Go core parity plus full SQLite fallback removal in Phase B/C.

Add documented exit-code row:

```markdown
| 13 | `daemon_role_missing`. | Provision `striatumd_rw` per "Provision the daemon-required role" and rerun `striatum daemon doctor --apply-migrations`. |
```

## Acceptance Gate

Run with `STRIATUM_TEST_POSTGRES_URL` set:

```bash
pytest tests/daemon_rpc/test_fail_closed_routing.py -v
pytest tests/daemon_rpc/test_unix_daemon_foreground.py -v
pytest tests/daemon_rpc/test_doctor_explain.py -v
pytest tests/daemon_pg/test_role_grants.py -v
pytest tests/daemon_pg/test_handler_no_upsert_on_append_only.py -v
pytest tests/daemon_pg/handlers/test_event_hash_chain.py -v
pytest tests/daemon_pg/handlers/test_event_hash_chain_concurrent.py -v
pytest tests/daemon_pg/handlers/test_migration_0006_backfill.py -v
pytest tests/daemon_pg/handlers/ -v
```

Also require:

```bash
grep -r 'RFC0048_PARITY' tests/ || true
grep -rn 'InvalidTransitionError.*requires Track A' src/striatum/ || true
```

Both grep commands must produce no matches. `make test` must be green in the Postgres-enabled CI lane before v1.50.0 release.

## Out Of Scope

RFC 0048 Phase B Go core parity, Phase C SQLite fallback removal, hosted mode, bundled Postgres, multi-tenancy, and broad product-doc rewrites outside `docs/POSTGRES_TRANSITION.md` are deferred.
