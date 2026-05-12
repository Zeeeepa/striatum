author: designer-codex-gpt-5.5-001

# RFC 0035 Multi-Repo Harness Design

Status: implementation design
Date: 2026-05-12
Target: RFC 0035 multi-repo end-to-end test harness

## Design Boundary

This is developer/test infrastructure only. The harness should live under
`tests/`, should not become a supported public Python API, and should not
change product semantics while being built. It exercises the accepted RFC
0030, RFC 0031, RFC 0032, and RFC 0033 slices against real temporary
repositories and a real daemon PostgreSQL schema.

The important product invariant remains unchanged: each participant target
repository owns its live workflow state in `.striatum/state.sqlite3`; daemon
PostgreSQL owns daemon-global registry, audit, capability, RPC-session,
supervision/apply metadata, and cross-repo run metadata. The harness may query
both stores for assertions, but it must not make tests pass by editing either
store directly except through explicit reset/setup helpers.

The current source shape matters for the implementation handoff:

- there is no `tests/conftest.py` yet, so RFC 0035's fixture wiring means
  creating that file rather than extending an existing one;
- the `Makefile` has no `pg-test` target today, so implementation must add it
  or revise the doc claim while adding `test-multi-repo`;
- `striatum.daemon.run_daemon_foreground()` currently binds a Unix socket as a
  daemon lifecycle marker and runs sweeps; the RPC router exists as
  `striatum.daemon_rpc.server.DaemonRpcRouter`, but there is not yet a full
  socket accept loop to send daemon RPC envelopes over that socket. Harness
  tests should therefore separate "daemon subprocess lifecycle is live" from
  "RPC/MCP route behavior is exercised through the existing Python router and
  MCP server seams" unless the implementer explicitly adds a test-only accept
  loop in harness code.

## Harness Module Layout

Add the RFC 0035 module layout exactly:

```text
tests/
  _harness/
    __init__.py
    multi_repo.py
    daemon.py
    repos.py
    pg.py
    mcp.py
  test_multi_repo_harness.py
  test_cross_repo_prepare_e2e.py
  test_cross_repo_lifecycle_e2e.py
  test_cross_repo_crash_recovery_e2e.py
  test_mcp_capability_scope_e2e.py
  test_per_repo_write_scope_e2e.py
```

`tests/_harness/__init__.py` should export only the small intended test
surface: `MultiRepoHarness`, `requires_postgres`, and any typed result objects
that keep tests readable. Keep helper internals in their leaf modules.

`tests/_harness/pg.py` owns all PostgreSQL setup. It should resolve a base URL
from `STRIATUM_TEST_POSTGRES_URL`, then `STRIATUM_DAEMON_DB_URL`, then a local
default such as `postgresql:///postgres` if `psycopg` can import. It creates a
unique database name per harness instance, connects to that database, applies
`striatum.daemon_pg.migrations.apply_migrations()`, and exposes:

- `postgres_available() -> tuple[bool, str]` for skip decisions;
- `create_ephemeral_database(base_url, name_prefix) -> EphemeralPostgres`;
- `drop_ephemeral_database(ephemeral)`;
- `truncate_daemon_tables(conn)`.

`truncate_daemon_tables()` should use PostgreSQL metadata to discover tables in
schema `striatumd`, exclude `schema_meta`, `schema_migrations`,
`audit_segments`, and `audit_chain_head` unless the test explicitly requests a
full reset, and execute a single `TRUNCATE ... RESTART IDENTITY CASCADE`. The
schema-version row must survive; audit-chain smoke assertions that need a
fresh chain should use a full ephemeral database, not a reset shortcut.

`tests/_harness/daemon.py` owns subprocess lifecycle. It should start the
daemon with:

```text
python -m striatum.cli daemon start --postgres-url <ephemeral-url>
```

The environment must set `PYTHONPATH=src`, `STRIATUM_DAEMON_RUNTIME_DIR` to a
short path under `scratch_dir/runtime`, and `STRIATUM_DAEMON_REGISTRY` to
`scratch_dir/daemon/striatumd.sqlite3`. Keep Unix socket paths short because
macOS has a low AF_UNIX path-length limit. `DaemonProcess.start()` waits until
the pid file and socket exist and until `daemon doctor --postgres-url` reports
PostgreSQL `ok`. `stop()` sends SIGTERM, waits, escalates to SIGKILL only on
timeout, and asserts socket/pid cleanup. `kill()` is reserved for crash tests.

`tests/_harness/repos.py` owns target repositories. It should create
`scratch_dir/repo-0`, `repo-1`, and so on, run `git init`, set minimal local
git identity, create a README commit if a test needs a non-empty tree, and run
`striatum --repo <repo> init --json`. Registration should use the daemon
PostgreSQL path when available and return the daemon `repository_id` values.
For the current code, `repo add` still flows through the V1 registry helper;
the harness should hide that compatibility detail and expose repository ids as
strings because daemon PG tables use text ids.

`tests/_harness/mcp.py` owns MCP-oriented helpers over existing seams:
`DaemonRpcServer(pg_conn=...)` for `tools/list` and `tools/call`, plus a tiny
client object with `tools_list()` and `tools_call(name, args)`. It should not
run a transcript-capturing stdio MCP subprocess for these tests.

`tests/_harness/multi_repo.py` composes the above and is the only object tests
should import directly.

## `MultiRepoHarness` API

Implement the requested constructor:

```python
MultiRepoHarness(daemon_pg_url: str, repo_count: int = 2, scratch_dir: Path)
```

The object should expose:

- `repos`: ordered repo descriptors with `index`, `alias`, `path`, and
  `repository_id`;
- `daemon_pg_url`: the ephemeral database URL, not the base URL;
- `daemon`: the daemon subprocess wrapper;
- `pg_conn()`: context manager for assertions;
- `start()`, `stop()`, `reset_daemon_db()`, `register_all()`;
- `issue_token(capability, repo_id=None, expires_in=3600)`;
- `mcp_client(token)`;
- `audit_rows(transport=None)`;
- `daemon_db_query(sql, args=())`;
- `repo_sqlite_query(repo_index, sql, args=())`;
- `restart_daemon()` and `kill_daemon(signal=SIGKILL)` for crash tests.

`start()` should be idempotent only in the sense that calling it after `stop()`
works. Calling it twice while running should raise an assertion failure with a
clear message. Startup order:

1. create the ephemeral PostgreSQL database from the configured base URL;
2. apply daemon PostgreSQL migrations;
3. start the daemon subprocess with an ephemeral runtime dir and socket;
4. create and initialize N target repositories;
5. register all repositories with the daemon;
6. open a short assertion connection and verify schema version, registered
   repository count, and socket existence.

`stop()` should be best-effort but strict for leaks:

1. terminate the daemon subprocess and wait for socket cleanup;
2. close any held connections;
3. drop the ephemeral PostgreSQL database from the base connection;
4. remove `scratch_dir`;
5. leave enough failure detail in assertion messages to diagnose leaked
   processes or active database sessions.

`reset_daemon_db()` is an explicit data reset, not a repo reset. It truncates
daemon-owned data tables while preserving schema metadata. It does not
re-register repositories because that would hide the fact that registration is
daemon state. Tests that need registered repositories after reset call
`register_all()`.

`register_all()` should be idempotent from a test perspective: if reset cleared
registration rows, insert/register them again; if rows already exist for the
same repo paths, return the existing ids. It must not silently rebind an alias
to a different repository id.

`issue_token()` should insert through the same token helper used by product
code when available. If no CLI/API helper exists, implement the harness helper
by reusing `striatum.daemon._hash_token()`, inserting into
`striatumd.clients` and `striatumd.client_capabilities`, and returning
`<token_id>.<secret>`. This is acceptable only as test setup, and the helper
should live in `_harness`, not production code.

## Pytest Fixtures And Skips

Create `tests/conftest.py` with the new fixtures. Keep existing tests
unaffected:

```python
@pytest.fixture(scope="class")
def multi_repo_harness(tmp_path_factory):
    ok, reason = postgres_available()
    if not ok:
        pytest.skip(f"multi-repo harness requires PostgreSQL: {reason}")
    harness = MultiRepoHarness(
        daemon_pg_url=resolved_base_url(),
        repo_count=2,
        scratch_dir=tmp_path_factory.mktemp("multi_repo"),
    )
    try:
        harness.start()
        yield harness
    finally:
        harness.stop()

@pytest.fixture
def clean_daemon_db(multi_repo_harness):
    multi_repo_harness.reset_daemon_db()
    yield
```

Also add a marker in `pyproject.toml`:

```toml
markers = [
  "multi_repo: requires system PostgreSQL and daemon harness",
]
```

Every new harness-backed test module should set
`pytestmark = pytest.mark.multi_repo`. Skip reason text should name the missing
requirement and a local command such as `make pg-test` once that target exists.

## Workflow Fixture Strategy

Do not hand-author large JSON blobs in every e2e test. Add helper builders in
`tests/_harness/repos.py` or `tests/_harness/multi_repo.py`:

- `two_repo_workflow(repo_ids, *, artifact_root="docs/out")`;
- `workflow_with_cross_repo_cycle(repo_ids, max_iterations=1)`;
- `workflow_with_bad_repository(repo_ids)`;
- `workflow_with_bad_artifact_scope(repo_ids)`.

Each builder should return plain workflow dictionaries and optionally write
them to a repo-local temporary file. The generated workflow should use generic
roles, manual/process-safe lanes, explicit `primary_repository`, explicit
per-job `repository`, per-repo `parallelism.per_repo_max_active_jobs`, and
paths under `docs/` so validator behavior is obvious.

## E2E Test Matrices

`tests/test_multi_repo_harness.py` should prove the harness is trustworthy
before relying on it:

- start registers two repositories and creates distinct repo-local SQLite
  files;
- `reset_daemon_db()` clears daemon data tables while preserving
  `schema_meta.substrate_version`;
- `register_all()` after reset restores repository rows with stable aliases;
- `stop()` drops the ephemeral database, removes scratch dir, and leaves no
  Unix socket;
- start/stop/start works with a new scratch dir and no socket collision.

`tests/test_cross_repo_prepare_e2e.py` should cover:

- well-formed workflow validates, then `prepare_cross_repo_run()` or the
  daemon route creates one `cross_repo_runs` row and two participant rows,
  and each repo-local `runs.cross_repo_run_id` points to the same id;
- unknown `repo_id` in `repositories` is refused before any daemon DB or
  repo-local write;
- simulated local SQLite failure on repo B causes the daemon row to become
  `aborted` or the transaction to roll back according to the implementation
  path chosen, but never leaves a `prepared` cross-repo row with a missing
  participant;
- validator-level failures are asserted with `validate_workflow()` where the
  current contract is shape-only, and daemon-backed registration failures are
  asserted at prepare time.

`tests/test_cross_repo_lifecycle_e2e.py` should cover:

- prepare -> start -> summary/describe -> cancel across two repos;
- mixed state where one participant is terminal and the other is running is
  reflected in `describe_cross_repo_run()` and dashboard/summary aggregation;
- cancel mid-run cascades to all reachable participants and records blocked
  participants explicitly;
- dashboard for the cross-repo id includes both aliases and repository ids;
- cross-repo cycle counter rows increment once per global cycle, not once per
  participant repo.

`tests/test_cross_repo_crash_recovery_e2e.py` should use explicit crash hooks
rather than timing sleeps. If production code has no injectable barrier, the
first implementation should add a harness-only local runner fake backed by real
repo SQLite writes for deterministic failure points. Cases:

- SIGKILL/restart while daemon DB row is `preparing` and local rows are missing
  reconciles to `aborted`;
- SIGKILL/restart after prepare but before every participant is `running`
  either completes the transition or records a structured blocked state;
- SIGKILL/restart during cancel finishes the cascade or records blocked
  participants without orphaning daemon state;
- making one participant `.striatum/` inaccessible mid-run causes a primary
  repo human-checkpoint blocker and a daemon `blocked`/paused state.

`tests/test_mcp_capability_scope_e2e.py` should assert both behavior and
audit:

- repo A scoped `write` token can call a write tool against repo A and writes
  an audit row with `transport='mcp'` and `decision='allowed'`;
- the same token against repo B is denied. Current capability code returns
  `capability_scope_mismatch` for some scoped-elsewhere cases, while RFC 0035
  names `capability_missing`; the implementer should either align code/docs or
  assert the accepted vocabulary after a decision;
- read-only token only sees read tools in `tools/list` and cannot call a write
  tool;
- unknown method records `method_unknown`;
- revoked token records `token_revoked`;
- expired token records `token_expired`;
- audit rows form a valid hash chain across the whole test.

`tests/test_per_repo_write_scope_e2e.py` should cover:

- job targeting repo B publishes an artifact under repo B's allowed path;
- an expected artifact path that attempts to escape or cross into repo A is
  rejected by workflow validation or daemon prepare before runtime;
- runtime publication from a repo B job into repo A is refused as a write-scope
  violation through the normal `publish-artifact` path;
- a job targeting an undeclared repository alias is rejected by the validator.

## CI And Make Targets

Add:

```make
.PHONY: pg-test test-multi-repo

pg-test: $(VENV)/.installed
	$(PYTHON) -m pytest tests/test_daemon_pg.py -q

test-multi-repo: $(VENV)/.installed
	$(PYTHON) -m pytest -m multi_repo tests/test_multi_repo_harness.py tests/test_cross_repo_*_e2e.py tests/test_mcp_capability_scope_e2e.py tests/test_per_repo_write_scope_e2e.py
```

`make test` can continue to run plain `pytest`; the multi-repo modules will
self-skip when PostgreSQL is unavailable. CI should install
`.[dev,daemon-pg]` for PG jobs, set `STRIATUM_TEST_POSTGRES_URL` to a
maintenance database, and let the harness create/drop per-run databases.
macOS-no-PG should still run the test files and report skips, not deselect
them silently.

The wall-clock target is under 60 seconds added locally. The main levers are
class-scoped daemon startup, cheap data resets, short daemon health polling,
and using focused test files rather than full workflow runs when a unit seam
already proves scheduler behavior.

## Documentation Updates

This implementation should update:

- `docs/TODO.md`: move Open item 19 from open to most-done or done depending
  on whether all RFC 0035 acceptance criteria land;
- `docs/SPEC.md`: mention that the test suite now has a harness for the
  deferred RFC 0032 end-to-end coverage, without making it a user-facing
  product contract;
- `docs/CLI_REFERENCE.md`: only if new or clarified developer commands such as
  `make pg-test` are referenced from docs; avoid implying a new CLI surface;
- `CHANGELOG.md`: add an Unreleased test-infrastructure entry.

If the implementation discovers that `pg-test` should not exist, update RFC
0035/TODO claims rather than letting stale docs say it does.

## Sequencing

1. Land `tests/conftest.py`, `tests/_harness/pg.py`, `daemon.py`,
   `repos.py`, and `multi_repo.py` plus `tests/test_multi_repo_harness.py`.
2. Add `Makefile` targets and pytest marker so the smoke test can run in CI.
3. Land prepare/lifecycle tests, using current `cross_repo.py` helpers and
   repo-local SQLite assertions.
4. Land crash-recovery tests with deterministic harness barriers; avoid
   sleep-based race tests.
5. Land MCP capability-scope tests and settle the
   `capability_missing` versus `capability_scope_mismatch` vocabulary before
   freezing assertions.
6. Land per-repo write-scope tests through normal CLI publish/validation
   paths.
7. Update TODO/SPEC/CHANGELOG and run focused verification:

```text
make test-multi-repo
pytest tests/test_workflow_cross_repo.py tests/test_cross_repo_lifecycle.py tests/test_mcp_mutation_capabilities.py tests/test_daemon_rpc_registry.py
make test
```

## Explicit Deferrals

Do not add Go client tests, a Dockerized PostgreSQL lifecycle, Windows daemon
harness support, cross-machine or multi-tenant scenarios, performance/load
tests, or example workflows under `examples/` as part of this implementation.
Those are separate product decisions or hardening RFCs.
