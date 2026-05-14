# RFC 0048 V1.5 fix-up — Daemon-Side Substrate Migration (claude lane)

author: designer-unknown-model-001

## 1. Scope and framing

dogfood-057 landed V1 Phase A: 16 PG-native handlers under
`src/striatum/daemon_pg/handlers/` self-register through
`@register_pg_handler`, and `DaemonRpcRouter._route`
(`src/striatum/daemon_rpc/server.py:227-243`) resolves them ahead of
the legacy `CLI_ROUTES` fallback. Two build reviews accepted the
landing as Phase A scope but flagged seven non-negotiable risks that
must close before V1 can become the daemon-required default path:

- codex F1 — fail-closed routing.
- codex F3 — audit-chain concurrency.
- codex F4 — append-only role enforcement.
- claude HIGH#1 — byte-equivalence parity rig (advertised, unwired).
- claude HIGH#2 — dead-code import paths (`complete_inline`,
  `ack_inline`, `recovery.resume --complete`, `recovery.auto` live).
- claude MEDIUM#4 — synthesis-mandated chain-columns migration.
- claude MEDIUM#5 — operator surface for "is this method PG-backed?".
- claude MEDIUM#6 — `docs/POSTGRES_TRANSITION.md` does not name the
  ported methods.

This design also closes the operator-visible substrate gap that forced
dogfood-057 itself into legacy SQLite mode: `run_daemon_foreground`
(`src/striatum/daemon.py:879-947`) binds and listens on the Unix
socket but never `accept()`s. The accept loop is in scope here
because RFC 0048 V1.5 is the last fix-up before V1 lands as default.

V2.0 phasing (Go core parity, SQLite removal flip, bundled
distribution, multi-tenancy) stays out of scope per the prompt.

## 2. Two implementer tracks

Same split as dogfood-057: Track A (codex) owns router + transport +
handler internals; Track B (claude) owns tests + schema + docs + UX.
Write-scope discipline is identical to 057 — Track A may not edit
test files Track B owns, Track B may not edit handler internals or
the router. The decorator + registry boundary is the only
synchronization surface; no further coordination commits are needed.

---

## Track A — Router, transport, handler internals (codex)

### A.1 Fail-closed routing (closes codex F1)

**Current behavior.** `DaemonRpcRouter._route`
(`src/striatum/daemon_rpc/server.py:227-244`) does:

```python
if self.pg_conn is not None:
    ...
    handler = resolve_pg_handler(envelope.method)
    if handler is not None:
        ...
        return handler(ctx, envelope.params)
prefix = CLI_ROUTES.get(envelope.method)
...
result = invoke(args, repo=repo_root)
```

Three implicit fall-throughs survive Phase A and are the F1 attack
surface:

1. `self.pg_conn is None` — the entire `if` block is skipped and a
   registered PG method silently routes to `CLI_ROUTES` → SQLite
   `invoke()`. F1 calls this out specifically: a "PG-backed" method
   must not become a SQLite-backed method based on connection state.
2. `handler is not None` but the handler raises. The current code
   lets the exception propagate, but the outer `handle()` only catches
   `RpcError`; any other exception type bubbles to the transport
   layer un-audited. F1 requires this to be a terminal RPC error
   with a stable code.
3. `repository_id is None` — already raises `RpcError("repo_not_registered")`,
   but Phase A leaks this check only when `self.pg_conn is not None`.
   When `pg_conn` is `None` the check is skipped, and the request
   falls into `CLI_ROUTES` carrying no repository scope.

**Fix.** Move the "is this method PG-backed?" decision out of
`_route` and into the registry. Make the decision authoritative,
independent of `self.pg_conn`, and refuse fall-through unconditionally
once a method is registered.

1. Add `is_pg_backed(method: str) -> bool` to
   `src/striatum/daemon_pg/handlers/registry.py`. It returns
   `method in _PG_HANDLERS`. Make the underlying `_PG_HANDLERS` dict
   private (`_PG_HANDLERS`) and `__all__`-export only the three
   functions `register_pg_handler`, `resolve_pg_handler`,
   `is_pg_backed`. The registry must not expose mutating views into
   its internal mapping; the only allowed write path is the
   decorator.

2. In `DaemonRpcRouter._route`, add a pre-CLI_ROUTES branch:

   ```python
   from striatum.daemon_pg.handlers.registry import (
       is_pg_backed, resolve_pg_handler,
   )

   if is_pg_backed(envelope.method):
       if self.pg_conn is None:
           raise RpcError(
               "daemon_db_missing",
               "method is PG-backed but daemon has no PG connection",
               exit_code=EXIT_DAEMON_DB_MISSING,  # new, mapped to 10
           )
       repository_id = auth.repository_id or _repository_id(envelope.params)
       if repository_id is None:
           raise RpcError("repo_not_registered", "PG handler requires repository_id")
       ctx = RepoHandlerContext(
           pg_conn=self.pg_conn,
           repository_id=repository_id,
           repo_root=repo_root,
           auth=auth,
       )
       try:
           with _serializable_handler_txn(self.pg_conn):
               return resolve_pg_handler(envelope.method)(ctx, envelope.params)
       except RpcError:
           raise
       except StriatumError as exc:
           raise RpcError(
               "handler_failed",
               str(exc),
               exit_code=exc.exit_code,
           ) from exc
       except Exception as exc:  # truly unexpected; never fall back
           raise RpcError(
               "handler_failed",
               f"{type(exc).__name__}: {exc}",
               exit_code=1,
           ) from exc

   # Method is *not* PG-backed → legacy CLI_ROUTES path (untouched).
   ```

   The PG branch is therefore terminal for every method it owns. The
   `CLI_ROUTES` branch never runs for a PG-backed method, regardless
   of `pg_conn` state, handler success, or exception type.

3. Add an explicit `EXIT_DAEMON_DB_MISSING = 10` constant in
   `src/striatum/errors.py` and map it through the existing
   exit-code surface (RFC 0030 already reserves 10 for daemon
   transport / version refusals; a PG-handler-but-no-PG-connection
   refusal is the same operator-facing remediation: bring the daemon
   up against Postgres).

4. **Error envelope shape.** Reuse `RpcError` (existing). The new
   codes added in this branch:

   | code | exit | when |
   |---|---:|---|
   | `daemon_db_missing` | 10 | PG-backed method, `self.pg_conn` is `None` |
   | `handler_failed` | varies | PG handler raised non-`RpcError`, non-`StriatumError` |

   Both codes route through the existing audit-log path
   (`_record_and_return`); no schema change is required.

5. **Tests Track B must add.** Per migrated method, monkeypatch the
   handler to raise three exception types and assert (a) the response
   is an `RpcError`, (b) `striatum.api.invoke` was never called
   (`pytest.MonkeyPatch.setattr` a sentinel), (c) no SQLite
   `connect()` call occurred (`monkeypatch.setattr` on
   `striatum.db.connect` to a sentinel that fails the test), and (d)
   the audit log appended one denied row with `decision = denied`
   and `denial_reason` matching the handler error code. See § B.2 for
   the matrix structure.

### A.2 Audit-chain SERIALIZABLE / row-lock (closes codex F3)

**Current behavior.** `RepoHandlerContext.append_event`
(`src/striatum/daemon_pg/handlers/context.py:108-205`) locks the
latest event row `FOR UPDATE` per repo, then inserts. The row-lock
is the chain anchor. But each handler currently runs at the
connection's default isolation level (`READ COMMITTED` per psycopg
default), and there is no top-level transaction marker around the
handler call site in `_route`. Two concurrent handlers can interleave
their `SELECT … FOR UPDATE` and `INSERT` such that both observe the
same `previous_hash` if the row-lock is acquired but the upstream
mutation (e.g. a workflow-state write) was performed under a
different visibility window.

**Fix.** Pattern: **one SERIALIZABLE transaction per handler
invocation, anchored at the router boundary, with one retry on
`SerializationFailure`**. This is the synthesis-locked shape from
dogfood-057 (DESIGN_SYNTHESIS L22-23) and is consistent with
RFC 0033 §270 audit-chain doctrine. The row-lock on
`striatumd.events` (or, post-A.4/§B.3, on
`striatumd.repo_event_chain_heads`) is retained as an explicit
serialization point so two concurrent handlers cannot both compute
`previous_hash` from the same parent.

1. Add `_serializable_handler_txn(pg_conn)` in a new module
   `src/striatum/daemon_pg/handlers/_txn.py`:

   ```python
   @contextmanager
   def serializable_handler_txn(conn, *, retries: int = 1) -> Iterator[None]:
       attempts = 0
       while True:
           attempts += 1
           try:
               conn.execute("BEGIN ISOLATION LEVEL SERIALIZABLE")
               yield
               conn.execute("COMMIT")
               return
           except SerializationFailure:
               conn.execute("ROLLBACK")
               if attempts > retries:
                   raise RpcError(
                       "serialization_failure",
                       "daemon RPC handler hit two consecutive serialization failures",
                       exit_code=1,
                   )
           except BaseException:
               conn.execute("ROLLBACK")
               raise
   ```

   The single retry matches DESIGN_SYNTHESIS L22 ("retries one
   serialization failure at the router boundary"); more aggressive
   retries are a perf concern for V2.0, not a correctness concern
   here.

2. **Read-only methods** (`evidence.export`, the listing path of
   `recovery.stale_leases`) do not need SERIALIZABLE; the wrapper
   should detect them via a registry flag and use
   `REPEATABLE READ READ ONLY` instead. Add a `read_only: bool = False`
   parameter to `register_pg_handler`:

   ```python
   @register_pg_handler("evidence.export", read_only=True)
   def handle(ctx, params): ...
   ```

   The router selects the isolation level from the registry entry.
   This costs Track B one decorator edit per read-only handler.

3. **Within-transaction discipline.** Every PG write handler MUST:
   - Take a row lock on the run/job/message/lease it mutates with
     `SELECT … FOR UPDATE` before issuing the matching `UPDATE`.
     This already holds in dogfood-057 but the design needs to make
     it a non-optional rule: a handler that issues a bare `UPDATE`
     without a prior `FOR UPDATE` lock is rejected by the parity
     test (Track B § B.1 helper greps the handler source for the
     missing `FOR UPDATE`).
   - Issue all `ctx.append_event(...)` calls inside the same
     SERIALIZABLE transaction. The current `append_event` already
     does `SELECT … FROM striatumd.events … FOR UPDATE` on the
     latest event for the repo — keep that lock as the chain anchor
     and rely on the outer SERIALIZABLE isolation for
     non-event-table invariants.
   - Never issue a manual `COMMIT` or `ROLLBACK` from inside the
     handler; the router owns transaction boundaries.

4. **Concurrent verification test.** A new
   `tests/daemon_pg/handlers/test_event_hash_chain_concurrent.py`
   spawns two threads that hit the router with overlapping
   `recovery.requeue_stale`, `work.complete`,
   `artifact.publish`, `review.verdict`, and `recovery.cancel_job`
   requests against the same `repository_id`. After both finish, the
   test:
   - reads all `striatumd.events` rows ordered by `event_id`,
   - recomputes the chain by replaying `canonical_event_hash` (or
     the column form post-§B.3) and asserts each `row_hash` equals
     the recomputed value,
   - asserts no two adjacent rows share the same `previous_hash`,
   - asserts the number of audit rows in `striatumd.audit_log` for
     this request set equals N requests (no orphan workflow
     mutations on a denied request, no missing audit row on an
     allowed request).

5. **Denied-path invariant.** A capability denial happens *before*
   the handler txn opens, so denied requests never mutate workflow
   state. The audit-row append for denials runs through
   `_record_and_return`, which uses its own connection-level
   autocommit-ish path (`append_audit_row`). This is correct as-is;
   the concurrent test in step 4 must still observe a denied audit
   row for every denied request.

### A.3 Unix-socket accept loop (closes the dogfood-057 substrate gap)

**Current behavior.** `run_daemon_foreground`
(`src/striatum/daemon.py:879-947`):

```python
sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
sock.bind(str(socket_path()))
sock.listen(1)
...
while not stopping:
    daemon_sweep_once()
    sweeps += 1
    time.sleep(...)
```

The socket is bound and listened on but never `accept()`s. The
sweep loop runs in the foreground; client connections sit in the
backlog until the kernel times out. The dogfood-057 OPERATOR_REPORT
confirms this: the run was forced into `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` legacy SQLite mode because the daemon was
operationally inaccessible.

**Fix.** Add an asyncio-based accept loop that runs concurrently with
the existing sweep loop. The framing (`daemon_rpc/framing.py`),
handshake (`daemon_rpc/handshake.py`), envelope decoding
(`daemon_rpc/envelope.py`), and `DaemonRpcRouter` are all
already-shipped components — the only missing piece is the loop
that wires them together.

**Why asyncio (not thread-per-connection or `select`).**

| option | argues for | argues against |
|---|---|---|
| `asyncio.start_unix_server` | one event loop owns the socket, sweeps, and signal handlers; integrates with `transport_http.py` if cross-transport parity is needed later; psycopg3 has async support | `DaemonRpcRouter` is sync — handler calls need `asyncio.to_thread` |
| thread-per-connection | simplest; matches current sync handler shape | one Python thread per concurrent CLI invocation; the GIL plus psycopg conn pool fights bound the throughput |
| `select` / `selectors` | minimal deps | every framing read has to handle partial NDJSON lines manually; we'd reimplement what asyncio gives us |

**Chosen: asyncio.start_unix_server, handler calls dispatched via
`asyncio.to_thread`.** The pattern:

```python
async def _serve(sock_path: Path, router: DaemonRpcRouter) -> None:
    server = await asyncio.start_unix_server(
        lambda r, w: _on_connection(r, w, router),
        path=str(sock_path),
    )
    os.chmod(sock_path, 0o600)
    async with server:
        await server.serve_forever()

async def _on_connection(
    reader: asyncio.StreamReader,
    writer: asyncio.StreamWriter,
    router: DaemonRpcRouter,
) -> None:
    connection_id = secrets.token_hex(8)
    try:
        while True:
            line = await reader.readline()
            if not line:
                return
            try:
                envelope = RpcEnvelope.decode(line)
            except RpcError as exc:
                response = RpcResponse.error_response(request_id="", error=exc)
            else:
                response = await asyncio.to_thread(
                    router.handle,
                    envelope,
                    connection_id=connection_id,
                )
            writer.write(response.encode() + b"\n")
            await writer.drain()
    finally:
        writer.close()
        await writer.wait_closed()
```

**Integration with `run_daemon_foreground`.**

1. Replace the synchronous `time.sleep` sweep loop with an asyncio
   event loop that:
   - schedules `_serve()` on the Unix socket,
   - schedules a periodic `daemon_sweep_once()` task on the same
     loop (`loop.call_later` chained, or `asyncio.create_task` with
     `await asyncio.sleep(sweep_interval_seconds)`),
   - installs SIGTERM/SIGINT handlers via `loop.add_signal_handler`
     that set a "stopping" event the accept loop awaits before
     `server.close()`.

2. The existing `sock = socket.socket(...)` binding code is replaced
   by `asyncio.start_unix_server(path=...)`. The 0o600 chmod and
   unlink-on-shutdown survive.

3. `max_sweeps` (existing parameter for tests) becomes a counter on
   the sweep task that triggers the stopping event on reaching
   `max_sweeps`. Tests do not need to change.

4. `DaemonRpcRouter` is constructed once at daemon startup. It takes
   `pg_conn` from the existing `connect_daemon_db()` path and
   `repo_root` from the per-repo dispatch (`_route` already does the
   `_repo_root_for(envelope, auth)` resolve before calling the
   handler). The router is thread-safe per the GIL; the
   `to_thread`-dispatched handler runs against the same `pg_conn`
   so the SERIALIZABLE txn from § A.2 holds.

   **Connection-per-request alternative considered, rejected**: a
   per-request psycopg connection from a pool would avoid lock
   contention between concurrent handler calls, but introduces
   transaction-boundary ambiguity for the `audit_log` /
   `request_log` writes in `_record_and_return` (those use the
   router's `pg_conn` directly, outside the SERIALIZABLE handler
   txn). V1.5 keeps the single-connection model; V2.0 can introduce
   pooling when Go-core parity makes per-request connections
   ergonomic.

5. **End-to-end goal verification.** A new
   `tests/daemon_rpc/test_accept_loop.py`:
   - starts `run_daemon_foreground` in a worker thread with a
     temporary registry + ephemeral PG database,
   - connects a `daemon_rpc.client` over the bound Unix socket,
   - issues a `daemon.hello` → `status` round-trip,
   - asserts the response reaches `DaemonRpcRouter._route` via the
     Unix socket (the `audit_log` row has `transport='rpc'` and the
     `decision='allowed'`).

6. **Failure modes.** Per RFC 0030, malformed framing, oversize
   envelopes, and `request_id` collisions all produce
   `daemon_rpc.envelope.RpcError` instances. The accept loop must
   convert framing decode errors into a response (not a connection
   drop), so misbehaving clients get a typed error rather than a
   silent disconnect; this also keeps the audit log a complete record
   of attempted requests.

7. **Backpressure.** Capped at `socket.listen(128)` (the asyncio
   default). One in-flight handler per connection (we `await` the
   thread before reading the next line). Concurrent connections are
   bounded only by the asyncio loop; the SERIALIZABLE handler txn
   in § A.2 absorbs the resulting write-contention.

### A.4 Append-only role enforcement (closes codex F4)

**Current state.**
`src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql:467-475`
already runs a `DO $$ … END $$` block that, *if the role exists*,
grants `SELECT/INSERT/UPDATE/DELETE` to `striatumd_rw` and revokes
`UPDATE/DELETE` on `striatumd.events` and `striatumd.artifacts`.
There is no migration that *creates* the role, no migration that
fails closed when the role is missing, and no test that asserts the
grant matches doctrine.

**Fix.** Add a new migration `0007_daemon_role_grants.sql`. Use a
fresh file (not a 0005 edit) so the migration runner detects it,
applies it idempotently, and bumps `schema_meta` version to 7. The
0006 migration in § B.3 sits between them and lands the chain
columns; this 0007 migration depends on the columns existing.

```sql
-- 0007_daemon_role_grants.sql
-- Enforce append-only grants for striatumd_rw; refuse start if the
-- daemon connects as a superuser-ish role and the constrained role
-- is not present (operators must provision it explicitly).

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

GRANT SELECT, INSERT, UPDATE, DELETE
  ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;

-- Chain head table is mutable (UPDATE on advance) but never DELETE'd.
REVOKE DELETE ON striatumd.repo_event_chain_heads FROM striatumd_rw;

-- Audit log is append-only at the daemon-global layer too.
REVOKE UPDATE, DELETE ON striatumd.audit_log FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.audit_segments FROM striatumd_rw;

ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO striatumd_rw;
ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd
  REVOKE UPDATE, DELETE ON TABLES FROM striatumd_rw;  -- closed-by-default
```

Rationale for default-privileges: any future migration that adds a
new table without explicit grants will inherit append-only semantics
for the rw role. Migrations that need UPDATE/DELETE on a new table
must grant them explicitly; that grant is then a reviewable change.

**Doctor surface.** `striatum daemon doctor` already inspects
schema migrations. Add a `daemon_role_grants` doctor check
(`src/striatum/cli/introspect.py` check list, alongside the
existing 24 checks). The check:

1. queries `pg_roles` for `striatumd_rw` presence,
2. queries `information_schema.role_table_grants` for the four
   `(privilege_type, table_name)` combinations the migration
   enforces,
3. fails with operator-actionable remediation: "run the SQL block
   in `docs/POSTGRES_TRANSITION.md § Provision the daemon-required
   role` and restart the daemon".

**Tests (Track B owns these, but Track A SQL owns the grants).**

- `tests/daemon_pg/test_role_grants.py`:
  - boots an ephemeral PG database,
  - creates a `striatumd_rw` role (via the test fixture, mirroring
    the runbook the operator runs),
  - applies migrations 0001-0007,
  - connects *as* `striatumd_rw`,
  - asserts `INSERT` into `striatumd.events` succeeds,
  - asserts `UPDATE striatumd.events SET event_type = 'tamper'`
    fails with `insufficient_privilege`,
  - asserts `DELETE FROM striatumd.events` fails,
  - asserts the same for `striatumd.artifacts` (UPDATE/DELETE),
  - asserts the same for `striatumd.audit_log`,
  - asserts `UPDATE striatumd.repo_event_chain_heads
    SET last_row_hash = ...` succeeds (head advance is the
    handler's job; tamper is not),
  - asserts `DELETE FROM striatumd.repo_event_chain_heads` fails.

- `tests/daemon_pg/test_handler_no_upsert_on_append_only.py`:
  greps the handler tree for `ON CONFLICT DO UPDATE` and
  `ON CONFLICT DO NOTHING` on the four append-only tables. The
  current `claim_next.py` and downstream handlers do not, but the
  greppable rule prevents regression.

---

## Track B — Tests, schema, docs, UX (claude)

### B.1 Byte-equivalence parity rig (closes claude HIGH#1)

**Current state.** The conftest at
`tests/daemon_pg/handlers/recovery_evidence/conftest.py` defines
`Seed`, `pg_ctx`, `sqlite_conn`, and an `import_handler` helper, but
**no test calls both fixtures in the same body**. The two existing
"parity" tests (`test_stale_leases.py:54-81`,
`test_requeue_stale.py:55-68`) are skipped by
`@pytest.mark.skipif(not os.environ.get("RFC0048_PARITY"))` and even
when enabled call only the PG handler and assert response *shape*,
never per-key state diff vs SQLite. Track A's nine workflow_loop
test files have no parity rig at all.

**Fix.** Lift the conftest from `recovery_evidence/` to
`tests/daemon_pg/handlers/conftest.py` (the synthesis-locked
location, DESIGN_SYNTHESIS L220), wire it into all 16 handler test
files, remove the `RFC0048_PARITY` env gate, and add a per-key diff
helper that produces a readable assertion message on regression.

1. **Move the conftest.** Move
   `tests/daemon_pg/handlers/recovery_evidence/conftest.py` →
   `tests/daemon_pg/handlers/conftest.py`. Pytest fixture inheritance
   makes both `workflow_loop/` and `recovery_evidence/` test files
   pick it up automatically. The existing recovery_evidence-specific
   `import_handler` helper stays — but the workflow_loop tests can
   now use the same `pg_ctx` / `sqlite_conn` / `parity_seed`
   fixtures without further wiring.

2. **Wire `parity_seed`.** Add a real fixture:

   ```python
   @pytest.fixture
   def parity_seed(pg_ctx, sqlite_conn, tmp_path) -> Seed:
       """Write the same fixture shape into both stores.

       Returns the Seed dataclass already advertised in the
       module docstring. The seed contains the IDs the handler
       under test needs (repository_id, run_id, job_id, …) so a
       per-method test calls a single fixture to get to a state
       where both stores are byte-equivalent."""
       seed = _build_seed(tmp_path)
       _write_sqlite_seed(sqlite_conn, seed)
       _write_pg_seed(pg_ctx, seed)
       return seed
   ```

   `_write_sqlite_seed` and `_write_pg_seed` live in a new
   `tests/daemon_pg/handlers/_parity.py` module. They write the
   same `runs/sessions/jobs/queue_messages/leases/work_packets`
   shape into each store. The synthesis "Source" lines for each
   handler enumerate the read tables that need to be seeded.

3. **Per-key state diff helper.** In `_parity.py`:

   ```python
   def assert_state_parity(
       pg_ctx, sqlite_conn, *, tables: Sequence[str],
       repository_id: str, run_id: str,
   ) -> None:
       """Read `tables` from both stores and assert per-key dict equality.

       Per-key diff format mirrors `dictdiffer.diff` (no external
       dep). On regression, raises AssertionError with a body like:

           Parity diff for runs:
             [-] state: 'pending' (sqlite) / 'running' (pg)
             [+] started_at: None (sqlite) / '2026-05-14T…' (pg)

       The order of fields in the diff is alphabetical for stability."""
       for table in tables:
           sqlite_rows = _read_sqlite(sqlite_conn, table, run_id)
           pg_rows = _read_pg(pg_ctx, table, repository_id, run_id)
           _assert_rows_equal(table, sqlite_rows, pg_rows)
   ```

   Helper reads omit the substrate-specific columns (`repository_id`
   in PG, none in SQLite) so the diff is meaningful. The helper
   raises with a human-readable diff so a regression is debuggable
   without running pdb.

4. **Per-method parity test.** Each of the 16 handler test files
   gains a `test_parity_with_sqlite` function:

   ```python
   def test_parity_with_sqlite(pg_ctx, sqlite_conn, parity_seed):
       # SQLite side
       sqlite_result = sqlite_handler(sqlite_conn, **invocation_params(parity_seed))
       # PG side
       pg_result = handle(pg_ctx, invocation_params(parity_seed))
       # Response parity
       assert _normalize(sqlite_result) == _normalize(pg_result)
       # State parity for the tables the handler writes (per synthesis)
       assert_state_parity(
           pg_ctx, sqlite_conn,
           tables=["runs", "sessions", "jobs", "queue_messages",
                   "leases", "events"],
           repository_id=parity_seed.repository_id,
           run_id=parity_seed.run_id,
       )
   ```

   `invocation_params(seed)` and `_normalize` are tiny per-test
   helpers; the structure is uniform across all 16 tests so the
   diff helper can stay generic.

5. **Remove the env gate.** Delete the
   `@pytest.mark.skipif(not os.environ.get("RFC0048_PARITY"))`
   decorators from `test_stale_leases.py`, `test_requeue_stale.py`,
   and anywhere else they appear. The new gate is the existing
   `pg_url` fixture skip-when-no-PG behavior (already in the
   conftest), which is the right granularity: parity tests run
   wherever PG is available, no separate flag needed.

6. **CI lane.** `make test` already drives the PG fixture when
   `STRIATUM_TEST_POSTGRES_URL` is set. Add a `make test-rfc0048`
   target (or repurpose `make test-multi-repo`) that runs
   `pytest tests/daemon_pg/handlers/ -v` against an ephemeral PG.
   This is the lane the CI workflow targets for V1.5 acceptance.

7. **What "fail loudly with a diff" means in practice.** If a future
   refactor diverges PG state from SQLite — e.g. a `maybe_complete_run`
   variant emits `run_terminal` on one side and `run_canceled` on
   the other (claude review finding #3) — the per-key diff in the
   assertion message points directly at the column. The diff is the
   regression signal the operator/maintainer needs.

### B.2 Capability-denial test matrix (closes codex F2)

**Scope.** Every PG write handler — all 9 Track A workflow_loop
plus 5 of the 7 Track B recovery_evidence (the read-only
`evidence.export` and the lazy-expiry-only `recovery.stale_leases`
listing path do not write workflow state; they still need the
denial matrix for `auth_required` / token validation).

**Helper.** Add `tests/daemon_pg/handlers/_denial.py`:

```python
@dataclass
class DenialCase:
    name: str                   # 'missing_token', 'revoked', …
    setup: Callable[[pg_ctx], None]
    expected_code: str          # 'token_missing', 'token_revoked', …
    expected_decision: str      # 'denied'

DENIAL_MATRIX: tuple[DenialCase, ...] = (
    DenialCase("missing_token",         _no_token,            "token_missing",          "denied"),
    DenialCase("revoked_token",         _revoke_token,        "token_revoked",          "denied"),
    DenialCase("expired_token",         _expire_token,        "token_expired",          "denied"),
    DenialCase("wrong_capability",      _wrong_capability,    "capability_required",    "denied"),
    DenialCase("wrong_repository_scope",_wrong_repository,    "repo_not_registered",    "denied"),
    DenialCase("replayed_request_id",   _replay,              "duplicate_request",      "denied"),
)

def assert_denial(case, pg_ctx, sqlite_conn, *, method, params, baseline):
    """Apply case.setup, call the router with the envelope, then
    assert:
      (1) router returns an error response with case.expected_code,
      (2) no workflow-state mutation vs baseline (per-key state diff
          is empty),
      (3) no allow-path audit row appended (audit_log rows since
          baseline have decision='denied' only),
      (4) one denied audit row with the documented reason matches
          case.expected_code."""
```

**Per-handler test pattern.** Each per-method test file gains
parameterized cases:

```python
@pytest.mark.parametrize("case", DENIAL_MATRIX, ids=lambda c: c.name)
def test_denial_path(case, pg_ctx, sqlite_conn, parity_seed):
    baseline = snapshot_state(pg_ctx, parity_seed)
    assert_denial(case, pg_ctx, sqlite_conn,
                  method="work.ack",
                  params={"message_id": parity_seed.message_id, …},
                  baseline=baseline)
```

**Replay-after-success case.** A separate parametrized
`replay_after_success` case is needed alongside the matrix:

```python
def test_replay_after_success(pg_ctx, parity_seed):
    response_1 = router_call(pg_ctx, method="work.ack", request_id="r1", …)
    assert response_1.ok
    response_2 = router_call(pg_ctx, method="work.ack", request_id="r1", …)
    assert response_2.error.code == "duplicate_request"
    # The second call must not append a second `queue.acked` event.
    assert _count_events(pg_ctx, event_type="queue.acked",
                         message_id=parity_seed.message_id) == 1
```

The `request_id_seen` check at `server.py:115` already enforces
duplicate-request refusal; the test pins that the duplicate refusal
runs before the handler is invoked (no second event row, no second
audit allow row).

**Coverage matrix.** For the 16 ported methods × 6 standard denial
cases + 1 replay-after-success = 112 denial assertions. Pytest
parameterization keeps the source size to one test function per
method.

### B.3 Schema migration 0006 — event chain columns

**Path.** `src/striatum/daemon_pg/sql/0006_event_chain_columns.sql`.

**Goals (claude review finding #4).**

1. Add top-level `previous_hash` / `row_hash` columns on
   `striatumd.events`.
2. Add `striatumd.repo_event_chain_heads` head table per
   `repository_id`.
3. Re-anchor existing rows by reading
   `payload_json->'_event_chain'->>'previous_hash'` /
   `row_hash` written by the dogfood-057 workaround.
4. Idempotent re-run via `schema_meta` version bump.

**Migration body.**

```sql
-- 0006_event_chain_columns.sql

-- Step 1: add nullable columns first so existing rows survive.
ALTER TABLE striatumd.events
  ADD COLUMN IF NOT EXISTS previous_hash bytea,
  ADD COLUMN IF NOT EXISTS row_hash bytea;

-- Step 2: per-repo head table. last_event_id is the trailing event
-- on this repo's chain; last_row_hash is the hash to use as the
-- previous_hash for the next append; updated_at is the head's
-- last advance timestamp.
CREATE TABLE IF NOT EXISTS striatumd.repo_event_chain_heads (
  repository_id text PRIMARY KEY
    REFERENCES striatumd.repositories(repository_id),
  last_event_id bigint,
  last_row_hash bytea,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Step 3: backfill from the dogfood-057 _event_chain payload.
-- The dogfood-057 hash is hex-encoded in the JSON; convert to bytea.
UPDATE striatumd.events
SET previous_hash = decode(
        coalesce(payload_json->'_event_chain'->>'previous_hash', ''),
        'hex'),
    row_hash = decode(
        coalesce(payload_json->'_event_chain'->>'row_hash', ''),
        'hex')
WHERE row_hash IS NULL
  AND payload_json ? '_event_chain';

-- Step 4: events without a chain payload (older test fixtures only)
-- get a deterministic re-anchored hash by recomputing canonically;
-- recomputation is done in Python after the migration runs to keep
-- SQL deterministic, see `daemon_pg.migrations.0006_post.py`.

-- Step 5: head table backfill — the trailing event per repository.
INSERT INTO striatumd.repo_event_chain_heads
    (repository_id, last_event_id, last_row_hash, updated_at)
SELECT repository_id,
       MAX(event_id) AS last_event_id,
       (SELECT row_hash FROM striatumd.events e2
         WHERE e2.repository_id = e1.repository_id
           AND e2.event_id = MAX(e1.event_id)) AS last_row_hash,
       now()
  FROM striatumd.events e1
 GROUP BY repository_id
    ON CONFLICT (repository_id) DO UPDATE
   SET last_event_id = EXCLUDED.last_event_id,
       last_row_hash = EXCLUDED.last_row_hash,
       updated_at = EXCLUDED.updated_at;

-- Step 6: now NOT NULL — only after backfill.
ALTER TABLE striatumd.events
  ALTER COLUMN row_hash SET NOT NULL;

-- previous_hash remains nullable to allow the first event per repo
-- to have NULL parent. The handler enforces continuity for the
-- rest.

-- Step 7: index for chain verification.
CREATE INDEX IF NOT EXISTS ix_events_repo_chain
  ON striatumd.events(repository_id, event_id DESC);

-- Schema version bump.
INSERT INTO striatumd.schema_meta(key, value)
VALUES ('schema_version', '6')
  ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
  WHERE striatumd.schema_meta.value::int < 6;
```

**Migration applier note.** Step 4 (re-anchoring rows that have no
`_event_chain` payload) is operationally rare — the dogfood-057
`append_event` always emits `_event_chain`. The "recompute in
Python" step is gated behind a `--reanchor-legacy` flag on
`daemon doctor --apply-migrations` and is opt-in for V1.5. The
default migration leaves those rows with `previous_hash` /
`row_hash` NULL and emits a `daemon doctor` warning naming the
event IDs; operators apply `--reanchor-legacy` when ready.

**Handler change.** Rewrite
`RepoHandlerContext.append_event` to (a) `SELECT … FOR UPDATE` on
`striatumd.repo_event_chain_heads` for this repo, (b) read
`last_row_hash` as `previous_hash`, (c) compute `row_hash` using
`canonical_event_hash` (already shipped), (d) INSERT the event with
`previous_hash` and `row_hash` populated in their new top-level
columns *plus* the existing `_event_chain` payload (for one-release
overlap, so V1.4 readers still work), (e) UPDATE the head row's
`last_event_id` and `last_row_hash`. All in the SERIALIZABLE handler
txn from § A.2. The V1.6 release can drop the `_event_chain` payload
write; for V1.5 we keep it for forward-compatibility with rolled-back
deployments.

**Tests (B-owned, exercise A-owned schema).**

- `tests/daemon_pg/handlers/test_event_hash_chain.py` (the missing
  file claude review finding #4 calls out):
  - asserts that `append_event` populates `previous_hash` and
    `row_hash` columns,
  - asserts that the head table row advances after each append,
  - asserts that two appends in the same SERIALIZABLE txn chain
    correctly (event N's `previous_hash` = event N-1's `row_hash`),
  - asserts continuity over a concurrent-append scenario
    (cross-references § A.2 concurrent test).
- `tests/daemon_pg/handlers/test_migration_0006_backfill.py`:
  - applies migrations 0001-0005,
  - seeds a few events in the dogfood-057 `_event_chain` payload
    shape,
  - applies migration 0006,
  - asserts `previous_hash` and `row_hash` columns now match the
    payload values byte-for-byte,
  - asserts the head row is populated.

### B.4 Dead code cleanup (closes claude HIGH#2)

Four call sites in dogfood-057 reference symbols that don't exist:

| symbol | callers | decision |
|---|---|---|
| `complete_inline` | `resume_blocker.py:294-309`, `auto_publish_stale_artifacts.py:337-351` | **Define + wire.** Both call sites are real recovery paths (inline `--complete` on resume, live publish-and-complete on auto). Deleting the call sites loses operator-actionable recovery. |
| `ack_inline` | `auto_publish_stale_artifacts.py:353-364` | **Define + wire.** Used in the live auto-publish path when the queue message is still in `claimed` state and needs an idempotent ack-then-complete. |
| `recovery.resume --complete` | CLI surface | **Define + wire.** The flag is documented in operator runbooks; deleting it would break recovery muscle memory. The user-visible behavior must match the operator's expectation: resume the blocker, then call `complete_inline` within the same txn. |
| `recovery.auto` live mode | `auto_publish_stale_artifacts.py:331-380` | **Define + wire.** Dry-run already works; live mode is the actual recovery primitive operators run. |

**Implementation.**

1. In `src/striatum/daemon_pg/handlers/workflow_loop/complete_job.py`,
   extract the body of `handle` into an internal helper
   `complete_inline(ctx, *, session_id, job_id, lease_id, summary,
   force=False) -> dict[str, Any]` that callers from
   `recovery_evidence/` can invoke without re-entering RPC. Keep
   `handle(ctx, params)` as the registered entry point; it
   normalizes `params` and dispatches to `complete_inline`. No
   public API change.

   ```python
   def complete_inline(
       ctx: RepoHandlerContext,
       *, session_id: str, job_id: str, lease_id: str | None,
       summary: Mapping[str, Any] | None,
       force: bool = False,
   ) -> dict[str, Any]:
       """Reusable PG complete-job entry; runs inside the caller's txn."""
       ...

   @register_pg_handler("work.complete", "complete")
   def handle(ctx, params):
       return complete_inline(
           ctx,
           session_id=params["session_id"],
           job_id=params["job_id"],
           lease_id=params.get("lease_id"),
           summary=params.get("summary"),
           force=params.get("force", False),
       )
   ```

2. Same pattern for `ack_inline(ctx, *, session_id, message_id,
   lease_id) -> None` in `workflow_loop/ack_work.py`.

3. In `resume_blocker.py:294-309` and
   `auto_publish_stale_artifacts.py:337-364`, replace
   `from ... import complete_inline` and `from ... import ack_inline`
   with the actual now-shipped imports. Delete the
   `InvalidTransitionError` fallback blocks; on the failure-mode
   side, `complete_inline` raises typed errors that bubble through
   the SERIALIZABLE txn naturally.

4. **Regression tests.**
   - `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py`
     gains `test_resume_with_complete_runs_inline`: calls
     `recovery.resume` with `complete=True`, asserts
     `completed_inline=True` in the response, asserts a `job.completed`
     event was appended, asserts the job state is `completed`.
   - `test_auto_publish_stale_artifacts.py` gains a live-mode test
     that exercises the full `ack → publish → complete` chain.

**Audit.** A `grep -rn "InvalidTransitionError.*requires Track A"
src/striatum/` must return zero hits after this change.

### B.5 `striatum daemon doctor --explain` (closes claude MEDIUM#5)

**Surface.** A new flag on the existing `daemon doctor` verb,
not a separate verb. Output:

```
$ striatum daemon doctor --explain --json
{
  "explain": {
    "methods": [
      {"method": "work.ack",                 "pg_backed": true,  "fallback_active": false},
      {"method": "work.claim_next",          "pg_backed": true,  "fallback_active": false},
      ...
      {"method": "session.register",         "pg_backed": true,  "fallback_active": false},
      {"method": "supervise.start",          "pg_backed": false, "fallback_active": true,  "fallback_path": "CLI_ROUTES['supervise.start']"},
      ...
    ],
    "totals": {"pg_backed": 16, "fallback_active": 30}
  },
  ...
}
```

Pretty (`--explain` without `--json`):

```
RPC method routing (Phase A: 16 PG-backed, 30 CLI fallback):

  PG-backed (16):
    work.ack                          → daemon_pg.handlers.workflow_loop.ack_work.handle
    work.claim_next                   → daemon_pg.handlers.workflow_loop.claim_next.handle
    ...
  CLI fallback (30):
    supervise.start                   → CLI_ROUTES['supervise.start'] → SQLite
    ...
```

**Registry query.** The data source is the union of
`METHOD_REGISTRY` (`daemon_rpc.registry`) and `_PG_HANDLERS`
(`daemon_pg.handlers.registry`). The doctor check builds the
table by:

```python
from striatum.daemon_rpc.registry import METHOD_REGISTRY
from striatum.daemon_pg.handlers.registry import is_pg_backed
from striatum.daemon_rpc.server import CLI_ROUTES

def explain_methods() -> list[dict[str, Any]]:
    rows = []
    for entry in METHOD_REGISTRY:
        method = entry.method
        pg = is_pg_backed(method)
        cli_prefix = CLI_ROUTES.get(method)
        rows.append({
            "method": method,
            "pg_backed": pg,
            "fallback_active": (not pg) and (cli_prefix is not None),
            "fallback_path": (
                f"CLI_ROUTES[{method!r}]" if cli_prefix and not pg else None
            ),
            "required_capability": entry.required_capability,
            "deprecated": entry.deprecated,
        })
    return rows
```

**Where it lives.** Add the verb path in
`src/striatum/cli/introspect.py` (the existing doctor entry point)
behind an `--explain` flag. Importantly: this is *not* a separate
RPC method; it's a doctor lens that the existing read-only `doctor`
path returns when `--explain` is set. No RPC registry change is
needed.

**Tests.**

- `tests/daemon_rpc/test_doctor_explain.py`:
  - asserts the 16 RFC 0048 Phase A method names all appear with
    `pg_backed=true`,
  - asserts no Phase A method has `fallback_active=true`,
  - asserts `len(rows) == len(METHOD_REGISTRY)`,
  - asserts the `totals` keys sum to `len(METHOD_REGISTRY)` minus
    the methods (like `daemon.hello`) that have neither a PG handler
    nor a CLI fallback (they're routed inline in `_route`).

### B.6 `docs/POSTGRES_TRANSITION.md` runbook update (closes claude MEDIUM#6 + operator friction)

**Two edits.**

#### B.6.a New section: "Provision the daemon-required role"

Insert between the existing "Configure the daemon DB connection" and
"Prepare and verify the daemon DB" sections. Body:

```markdown
## Provision the daemon-required role

The daemon connects to Postgres as the role from your daemon DB URL
(`STRIATUM_DAEMON_DB_URL` / `--postgres-url` / `daemon.toml`). That
role MUST be `striatumd_rw`, and `striatumd_rw` MUST have the
append-only grants the daemon enforces at runtime. The migration
refuses to apply if the role is missing.

Provision it once per Postgres instance — typically as your
admin/superuser role — before the first `striatum daemon doctor
--apply-migrations` run:

\`\`\`sql
-- As a role with CREATEROLE (typically `postgres` or your DB admin):
CREATE ROLE striatumd_rw LOGIN PASSWORD :'pwd';
GRANT CONNECT ON DATABASE striatum_daemon TO striatumd_rw;
GRANT USAGE ON SCHEMA striatumd TO striatumd_rw;
-- The 0007 migration grants table-level privileges and revokes
-- UPDATE/DELETE on append-only tables; do not grant those by hand.
\`\`\`

If `striatum daemon doctor` refuses with **exit code 13
(`daemon_role_missing`)** and a `daemon_role_grants` problem record
naming `striatumd_rw`, run the SQL block above and rerun the doctor.

The grants the 0007 migration enforces (do not hand-edit):

- `SELECT/INSERT/UPDATE/DELETE` on most `striatumd.*` tables.
- `UPDATE/DELETE` revoked on `striatumd.events`, `striatumd.artifacts`,
  `striatumd.audit_log`, `striatumd.audit_segments`. These are
  append-only by doctrine.
- `DELETE` revoked on `striatumd.repo_event_chain_heads`. Head rows
  are UPDATE-only (the handler advances `last_row_hash`).
```

#### B.6.b Rewrite of the "RFC 0048 remaining work" section

The current text (`docs/POSTGRES_TRANSITION.md:248-267`) reads as
though Phase A has not landed. Replace with a tri-section structure:

```markdown
## RFC 0048 status (v1.49.0+)

RFC 0048 ports each `src/striatum/cli/` single-repo handler to
read/write the daemon's per-repo Postgres tables directly. The work
is split across three phases:

### Phase A — ported methods (LANDED in v1.49.0)

Sixteen RPC methods now read and write Postgres natively, via
handlers under `src/striatum/daemon_pg/handlers/`:

| method | handler module |
|---|---|
| `session.register` | `workflow_loop/register_session.py` |
| `work.claim_next` | `workflow_loop/claim_next.py` |
| `work.ack` | `workflow_loop/ack_work.py` |
| `work.complete` | `workflow_loop/complete_job.py` |
| `work.release` | `workflow_loop/release_lease.py` |
| `work.block` | `workflow_loop/block_job.py` |
| `review.verdict` | `workflow_loop/record_verdict.py` |
| `review.submit` | `workflow_loop/submit_review.py` |
| `review.override` | `workflow_loop/override_review_verdict.py` |
| `recovery.stale_leases` | `recovery_evidence/stale_leases.py` |
| `recovery.requeue_stale` | `recovery_evidence/requeue_stale.py` |
| `recovery.cancel_job` | `recovery_evidence/cancel_job.py` |
| `recovery.process_reconcile` | `recovery_evidence/process_reconcile.py` |
| `recovery.resume` | `recovery_evidence/resume_blocker.py` |
| `recovery.auto` | `recovery_evidence/auto_publish_stale_artifacts.py` |
| `evidence.export` | `recovery_evidence/evidence_export.py` |

Per-method routing is greppable via
`striatum daemon doctor --explain`, which lists every registered RPC
method alongside `pg_backed: true/false` and the CLI fallback path
if applicable. Use it during incident triage to confirm a given
method is reaching the PG handler and not the SQLite fallback.

### Phase B — Go core parity (deferred to v2.0)

Implementing the same handler surface in `go/pkg/rpc/` /
`go/pkg/apply/` so `--core go` services single-repo verbs natively.
No CLI surface change for operators; the doctor's `--explain` view
shows the Python daemon's routing only.

### Phase C — SQLite fallback removal (deferred to v2.0)

After Phase B, the `CLI_ROUTES`-backed fallback for the remaining
~30 methods is retired. The `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` test-harness escape becomes a no-op and is
deleted from the docs surface.
```

The existing exit-code table grows by one row:

| Code | Meaning | Operator remediation |
|---:|---|---|
| 13 | `daemon_role_missing` (RFC 0048 V1.5). | Provision `striatumd_rw` per the "Provision the daemon-required role" section above. |

---

## 3. Acceptance checklist (V1.5 landing gate)

The following must hold before V1.5 ships:

### Track A acceptance

- A.1 — `pytest tests/daemon_rpc/test_fail_closed_routing.py -v`
  green: every Phase A method, with handler monkeypatched to raise,
  returns an `RpcError` and asserts `striatum.api.invoke` was never
  called.
- A.2 — `pytest tests/daemon_pg/handlers/test_event_hash_chain_concurrent.py -v`
  green: concurrent overlapping write tests produce a single
  contiguous audit chain.
- A.3 — `pytest tests/daemon_rpc/test_accept_loop.py -v` green:
  end-to-end `daemon.hello` → `status` through the bound Unix
  socket; the dogfood-057 substrate-mismatch failure mode is
  closed (no `STRIATUM_DAEMON_REQUIRED=0` workaround needed for
  the daemon to be reachable).
- A.4 — `pytest tests/daemon_pg/test_role_grants.py -v` green:
  `striatumd_rw` cannot UPDATE/DELETE `events`, `artifacts`, the
  two audit tables, and cannot DELETE chain heads.

### Track B acceptance

- B.1 — Every per-method test file has a `test_parity_with_sqlite`
  case; `RFC0048_PARITY` env gate removed from the source tree
  (`grep -r 'RFC0048_PARITY' tests/` returns zero hits).
- B.2 — Every PG write handler has 6 denial + 1
  replay-after-success parameterized case; `grep -rn 'DENIAL_MATRIX'
  tests/daemon_pg/handlers/ | wc -l` ≥ 14 (12 write handlers +
  2 read-handlers running the auth-only subset).
- B.3 — `src/striatum/daemon_pg/sql/0006_event_chain_columns.sql`
  exists; `tests/daemon_pg/handlers/test_event_hash_chain.py` and
  `test_migration_0006_backfill.py` green.
- B.4 — `grep -rn 'InvalidTransitionError.*requires Track A'
  src/striatum/` returns zero hits; `recovery.resume --complete`
  and `recovery.auto` live mode tests green.
- B.5 — `striatum daemon doctor --explain` returns the 16 Phase A
  methods with `pg_backed=true`; `tests/daemon_rpc/test_doctor_explain.py`
  green.
- B.6 — `docs/POSTGRES_TRANSITION.md` includes the
  "Provision the daemon-required role" section and the rewritten
  "RFC 0048 status" tri-section; `grep 'PROPOSED' docs/POSTGRES_TRANSITION.md`
  returns no RFC 0048 hits (Phase A is LANDED, not PROPOSED).

### Cross-track acceptance (operator-facing)

- `make test` green with `STRIATUM_TEST_POSTGRES_URL` set.
- A clean `striatum daemon start` from a freshly provisioned
  Postgres reaches `DaemonRpcRouter._route` via the Unix socket
  on the first CLI invocation, without `STRIATUM_DAEMON_REQUIRED=0`
  or `STRIATUM_TEST_HARNESS=1` set anywhere.
- `striatum daemon doctor --explain` reports
  `totals.pg_backed=16, totals.fallback_active=30` after migrations
  0006 + 0007 apply.

## 4. Out of scope (reiteration)

Per the prompt, this design covers V1.5 only. The following are
explicitly deferred and have no work item in this design:

- RFC 0048 Phase B (Go core parity).
- RFC 0048 Phase C (SQLite removal flip and test-harness escape
  retirement).
- Bundled Postgres distribution, hosted/cloud daemon.
- Multi-tenancy (`tenant_id` enforcement).
- README / TODO / CHANGELOG / SPEC updates (operator-owned after
  the dogfood lands).

## 5. Provenance

- `docs/rfcs/0048-daemon-side-substrate-migration.md` § "V1 Phase A
  landing summary" and § "V1.5 follow-up".
- `docs/dogfood/057/review/build/codex/REVIEW.md` (F1-F4).
- `docs/dogfood/057/review/build/claude/REVIEW.md` (HIGH#1, HIGH#2,
  MEDIUM#3-6).
- `docs/dogfood/057/DESIGN_SYNTHESIS.md` (synthesis amended here).
- `docs/dogfood/057/build/track_a/HANDOFF.md` and `track_b/HANDOFF.md`
  (current state of V1 Phase A code).
- `docs/POSTGRES_TRANSITION.md` (runbook this design rewrites).
- Source files cited inline: `src/striatum/daemon.py`,
  `src/striatum/daemon_rpc/server.py`,
  `src/striatum/daemon_rpc/registry.py`,
  `src/striatum/daemon_pg/handlers/context.py`,
  `src/striatum/daemon_pg/handlers/registry.py`,
  `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`.
