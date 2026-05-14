# RFC 0048 V1.5 Fix-Up Design

author: designer-unknown-model-001
date: 2026-05-14
status: handoff
target: RFC 0048 V1.5 daemon-required fix-up

## Objective

Dogfood-057 proved that the 16 Phase A handlers can be registered and
called, but it did not make the daemon-required path safe to use. V1.5
closes the gap between "ported handlers exist" and "the daemon is the
only live workflow authority" by making routing fail closed, adding the
missing Unix-socket accept loop, moving event-chain hashes into schema,
and replacing smoke tests with real parity and authorization coverage.

The implementation should land in two disjoint tracks:

- Track A (codex): router, transport, handler internals, event-chain
  append protocol, append-only role enforcement.
- Track B (claude): parity and denial tests, schema migration, dead-code
  cleanup, doctor UX, Postgres transition runbook.

Out of scope remains RFC 0048 Phase B Go parity, Phase C SQLite escape
removal, hosted mode, bundled Postgres, multi-tenancy, and broad product
doc updates beyond `docs/POSTGRES_TRANSITION.md`.

## Track A: Router, Transport, Handler Internals

### A1. Fail-Closed PG Routing

Add an explicit PG-registry predicate in
`src/striatum/daemon_pg/handlers/registry.py`:

```python
def is_pg_backed(method: str) -> bool:
    return method in _PG_HANDLERS
```

`DaemonRpcRouter._route` must import the handler package before checking
the predicate, then branch in this order:

1. If `is_pg_backed(envelope.method)` is true and `self.pg_conn is None`,
   raise `RpcError("daemon_db_missing", "...PG-backed route requires daemon PostgreSQL")`.
2. If PG-backed and `repository_id` is missing, raise
   `RpcError("repo_not_registered", ...)`.
3. If PG-backed, call the handler.
4. Any exception from the handler is translated into an RPC error and
   recorded through the normal audit/request-log path.
5. Never consult `CLI_ROUTES`, `striatum.api.invoke`, or
   `striatum.db.connect` for a method that is PG-backed.

Only methods not present in the PG registry may use the legacy
`CLI_ROUTES` fallback during this V1.5 transition. Parameter validation
failures and capability denials must happen before the handler mutates
state, but they are still terminal for PG-backed methods. The error
envelope should keep the existing shape:

```json
{"ok": false, "error": {"code": "schema_invalid|capability_denied|...", "message": "..."}}
```

Add `tests/daemon_pg/handlers/test_router_fail_closed.py` with one
parametrized case per registered method. Each case monkeypatches the PG
handler to raise, monkeypatches `striatum.api.invoke` and
`striatum.db.connect` to fail the test if called, sends an RPC request
through `DaemonRpcRouter.handle`, and asserts `ok == false`.

### A2. Event Chain Transaction Protocol

Use both a short transaction and an explicit row lock. The transaction
gives every handler one commit boundary for workflow mutation plus event
append; the chain-head row lock gives deterministic event ordering
without relying on `ORDER BY event_id DESC FOR UPDATE`.

Track A owns `RepoHandlerContext.append_event()` and the handler
transaction helper:

- Add `RepoHandlerContext.write_transaction()` wrapping
  `ctx.pg_conn.transaction(isolation_level="SERIALIZABLE")`, with a
  small serialization-failure retry wrapper at the router boundary for
  `40001`.
- Require every mutating handler in `workflow_loop/` and
  `recovery_evidence/` to call `append_event()` inside that transaction.
- `append_event()` locks
  `striatumd.repo_event_chain_heads WHERE repository_id = %s FOR UPDATE`,
  inserts the event row with `previous_hash = head.last_row_hash`,
  computes `row_hash`, then updates the head row before returning.
- Read-only handlers may use read-only/repeatable-read transactions, but
  any handler that appends an event is a write handler and follows this
  protocol.

Add `tests/daemon_pg/handlers/test_event_hash_chain.py`:

- Starts with an empty head row and verifies the first event has
  `previous_hash IS NULL`.
- Appends multiple events inside one handler transaction and verifies
  each row points to the previous row hash.
- Drives concurrent allowed and denied calls across representative
  `work.claim_next`, `artifact.publish` or `review.submit`,
  `review.verdict`, `work.complete`, and `recovery.requeue_stale` paths.
- Verifies a single contiguous per-repository chain and no event whose
  corresponding workflow mutation rolled back.

Denied authorization requests belong to `striatumd.audit_log`, not
`striatumd.events`, unless the handler was entered and deliberately
emits a business event.

### A3. Unix-Socket Accept Loop

`src/striatum/daemon.py::run_daemon_foreground` already binds and
listens on `socket_path()`, but it never accepts connections. Implement
a small threaded Unix-socket server in the existing foreground process;
do not introduce asyncio unless the rest of the daemon loop is moved at
the same time.

Design:

- Use `striatum.daemon_rpc.transport_unix.bind_unix_socket()` or fold the
  existing local bind code into that helper so socket permissions remain
  `0600`.
- Open one daemon PostgreSQL connection pool or connection factory at
  startup after `daemon_pg.connection.doctor(..., apply=True)` succeeds.
- Create a `DaemonRpcRouter(pg_conn=<connection>, repo_root=<active repo
  root when known>, substrate_schema=<doctor version>)` per accepted
  connection or per request. A per-connection router is simplest because
  handshake state is connection-local.
- Start an accept thread before the sweep loop. The thread blocks on
  `sock.accept()` with a timeout, and spawns one daemon thread per
  connection.
- Each connection handler wraps `conn.makefile("rwb")`, reads NDJSON
  envelopes via `daemon_rpc.framing.read_envelopes`, calls
  `router.handle(envelope, connection_id=<uuid>, transport="unix")`,
  and writes responses via `write_response`.
- Require the first ordinary call to be preceded by `daemon.hello` on the
  same connection; this uses the router's existing
  `require_handshake=True` behavior.
- On SIGTERM/SIGINT, set the existing `stopping` flag, close the
  listening socket to break `accept()`, join the accept thread with a
  short timeout, then remove socket and pid files.

The end-to-end test is `tests/daemon_rpc/test_unix_daemon_foreground.py`:
start `run_daemon_foreground(max_sweeps=None, sweep_interval_seconds=...)`
in a background thread/process with a temp runtime dir, call
`daemon_rpc.client.call_unix()` for `daemon.hello`, then call `status`
with a registered repository id and assert the request reaches
`DaemonRpcRouter._route`. This test should fail on the current branch
because the client can connect but never receives a response.

### A4. Append-Only Role Enforcement

Do not mutate `0001_baseline.sql`; it has already shipped. Add
`src/striatum/daemon_pg/sql/0007_append_only_role_grants.sql` after the
0006 event-chain migration.

The migration should:

- Create or assume the runtime role name `striatumd_rw` as documented by
  the operator runbook.
- Grant `USAGE` on schema and required sequences.
- Grant table-level permissions needed for normal runtime operation.
- Explicitly revoke `UPDATE` and `DELETE` on
  `striatumd.audit_log`, `striatumd.events`, and
  `striatumd.artifacts`.
- Grant only `SELECT, INSERT` on those append-only tables to
  `striatumd_rw`.
- Keep owner/admin migrations running as the schema owner, not as
  `striatumd_rw`.

Also add or keep triggers as a defense-in-depth guard, but tests must
exercise SQL privileges specifically by connecting as `striatumd_rw` and
proving:

- `INSERT` into `events` and `artifacts` works when required columns are
  present.
- `UPDATE` and `DELETE` on `events`, `artifacts`, and `audit_log` fail
  with insufficient privilege.
- No handler uses `ON CONFLICT DO UPDATE` against append-only
  provenance rows. A grep-style regression test may enforce this for
  `striatumd.events`, `striatumd.artifacts`, and `striatumd.audit_log`.

## Track B: Tests, Schema, Docs, UX

### B1. Byte-Equivalence Parity Rig

Promote the advertised
`tests/daemon_pg/handlers/recovery_evidence/conftest.py` rig into a
shared `tests/daemon_pg/handlers/conftest.py` used by both
`workflow_loop/` and `recovery_evidence/`.

Required fixtures and helpers:

- `Seed`: one dataclass describing a minimal workflow/run/job/session/
  queue/lease/artifact/verdict/blocker/process state.
- `parity_seed`: creates the same state in SQLite and Postgres.
- `pg_ctx`: returns `RepoHandlerContext` bound to the seeded repository.
- `sqlite_conn`: returns the seeded SQLite connection for the legacy
  command/helper path.
- `state_snapshot(conn_or_pg, *, tables: Sequence[str]) -> dict`.
- `assert_state_parity(pg_state, sqlite_state)`: raises an
  `AssertionError` with a per-table, per-primary-key, per-column diff.

Every one of the 16 handler test files gets at least one happy-path
parity test that:

1. Seeds both substrates from `parity_seed`.
2. Invokes the PG handler and the corresponding SQLite-backed path with
   equivalent input.
3. Compares the RPC response shape after normalizing generated ids and
   timestamps.
4. Compares touched table state and event payloads with
   `assert_state_parity`.

Remove all `RFC0048_PARITY` skip gates. If CI lacks Postgres, use the
existing repository-wide PG test sentinel so the whole PG suite is
skipped consistently, not individual parity assertions hidden behind a
bespoke environment variable.

### B2. Capability-Denial Matrix

Add `tests/daemon_pg/handlers/test_capability_denials.py` with a helper:

```python
assert_denied_without_workflow_mutation(method, params, denial_case)
```

For every PG write handler, cover:

- Missing token.
- Revoked token.
- Expired token.
- Wrong required capability.
- Wrong repository scope.
- Replayed `request_id`.

The helper snapshots workflow tables before and after, sends the request
through `DaemonRpcRouter.handle`, and asserts:

- The response is an RPC error with the documented reason.
- No workflow table changed.
- No `striatumd.events` or `striatumd.artifacts` row was appended.
- A denied `striatumd.audit_log` row exists when the envelope was
  parseable.
- A valid allow-path request appends exactly the expected audit row and
  no denied audit row.

Replay should be tested in two phases: first an allowed request commits,
then a second request with the same `request_id` is denied as
`duplicate_request` and leaves workflow tables unchanged.

### B3. Migration 0006: Event Chain Columns

Add `src/striatum/daemon_pg/sql/0006_event_chain_columns.sql`.

The migration should:

- Add nullable `previous_hash text` and `row_hash text` columns first.
- Backfill existing rows from `payload_json -> '_event_chain'`.
- For rows missing JSON chain metadata, re-anchor deterministically by
  ordering `(repository_id, event_id)` and recomputing with
  `canonical_event_hash` semantics. If doing that in SQL is too brittle,
  add a Python migration hook in the daemon PG migration runner for this
  version.
- Set `row_hash NOT NULL`, keep `previous_hash` nullable for the first
  event in a repository, and add `UNIQUE(repository_id, row_hash)`.
- Create `striatumd.repo_event_chain_heads(repository_id PRIMARY KEY
  REFERENCES striatumd.repositories, last_event_id bigint,
  last_row_hash text, updated_at timestamptz NOT NULL DEFAULT now())`.
- Backfill one head row per repository from the last event.
- Bump `schema_meta.substrate_version` and record the migration through
  the existing `schema_migrations` mechanism.

After this migration, `payload_json._event_chain` is no longer written.
Tests may tolerate old rows containing the JSON key, but new rows must
use the columns.

### B4. Dead-Code Cleanup

Define and wire the inline helpers; do not delete the live-mode features.
They are existing operator-facing surfaces and Track B already wrote
actionable messages around them.

- Export `ack_inline(ctx, *, session_id, message_id, lease_id) -> None`
  from `workflow_loop/ack_work.py`. It should contain the current
  transition body from `handle()` and assume the caller already opened
  the transaction.
- Export `complete_inline(ctx, *, session_id, job_id, lease_id,
  summary=None) -> dict[str, Any]` from
  `workflow_loop/complete_job.py`, likewise sharing the current
  completion body.
- Keep `submit_review.publish_artifact_inline` as the artifact helper,
  but move it to a neutral helper module if importing from
  `submit_review.py` creates a review-specific dependency for
  `recovery.auto`.
- Update `recovery.resume --complete` and `recovery.auto` live mode to
  call these helpers inside their existing PG transaction, never by
  re-entering RPC.

Regression tests:

- `recovery.resume --complete` resolves the blocker and completes the
  job in one transaction, returning `completed_inline: true`.
- `recovery.auto` live mode can ack, publish, complete, and append its
  recovery event.
- Inject a failure after artifact publish but before complete and verify
  the transaction rolls back both the artifact row and event rows.

### B5. `striatum daemon doctor --explain`

Add `--explain` to the `daemon doctor` parser. The JSON result should
include:

```json
{
  "method_substrates": [
    {
      "method": "work.ack",
      "pg_backed": true,
      "sqlite_fallback_active": false,
      "required_capability": "claim",
      "repository_scope_mode": "single_repo"
    }
  ]
}
```

The source of truth is the daemon RPC method registry plus the PG handler
registry after importing `striatum.daemon_pg.handlers`. For each
`MethodEntry`, derive:

- `pg_backed = is_pg_backed(method)`.
- `sqlite_fallback_active = method in CLI_ROUTES and not pg_backed`.
- `substrate = "pg" if pg_backed else "cli_fallback" if method in
  CLI_ROUTES else "native_daemon"`.

Expose the same fields in `daemon.describe` by extending
`MethodEntry.public_dict()` or by augmenting `describe_methods()`.
Human text output should be a compact table:

```text
method                         pg-backed  sqlite-fallback
work.ack                       yes        no
run.summary                    no         yes
daemon.describe                native     no
```

### B6. `POSTGRES_TRANSITION.md` Runbook Updates

Update only `docs/POSTGRES_TRANSITION.md` in this dogfood.

Add "Provision the daemon-required role" before daemon startup with
copy-pasteable SQL:

```sql
CREATE ROLE striatumd_rw LOGIN PASSWORD '<local secret>';
GRANT CONNECT ON DATABASE striatum_daemon TO striatumd_rw;
GRANT USAGE ON SCHEMA striatumd TO striatumd_rw;
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA striatumd TO striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.audit_log FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;
GRANT SELECT, INSERT ON striatumd.audit_log TO striatumd_rw;
GRANT SELECT, INSERT ON striatumd.events TO striatumd_rw;
GRANT SELECT, INSERT ON striatumd.artifacts TO striatumd_rw;
```

The final SQL should match the migration exactly; if migration 0007 uses
more restrictive table-specific grants, document those instead of this
starter shape.

Rewrite the RFC 0048 section to distinguish:

- 16 V1 Phase A methods already PG-native.
- V1.5 hardening work in progress.
- Remaining unported methods that still use CLI fallback in development
  until Phase C.

Mention that `striatum daemon doctor --explain --json` reports
`role_grants` and `method_substrates`, and that missing append-only
revokes are a daemon-doctor refusal before startup.

## Acceptance Checklist

- A PG-backed method cannot fall through to SQLite after handler
  exception, parameter denial, authorization denial, or missing PG
  connection.
- `striatum daemon start` accepts Unix-socket RPC requests and
  `striatum status` reaches `DaemonRpcRouter._route` through the socket.
- Every PG write handler mutates state and appends business events in
  one serializable transaction using the per-repo event-chain head lock.
- `events.previous_hash`, `events.row_hash`, and
  `repo_event_chain_heads` are populated for old and new rows.
- `striatumd_rw` cannot update or delete audit, event, or artifact
  provenance rows.
- All 16 handler test files include byte-equivalence parity coverage
  with loud per-key diffs.
- Every PG write handler has the six capability-denial/replay tests.
- `complete_inline` and `ack_inline` are real helpers, and the two
  recovery live paths are reachable.
- `daemon doctor --explain` and `daemon.describe` expose PG-backed vs
  SQLite-fallback method routing.
- `docs/POSTGRES_TRANSITION.md` gives operators the role-provisioning
  SQL and current RFC 0048 method-substrate state.
