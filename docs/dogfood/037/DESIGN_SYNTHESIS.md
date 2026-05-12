---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/037/design/codex/DESIGN.md", "docs/dogfood/037/design/claude_code/DESIGN.md", "docs/dogfood/037/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# Design Synthesis: RFC 0035 Multi-Repo Test Harness

Status: implementation plan
Date: 2026-05-12
Target: RFC 0035 multi-repo test harness for cross-repo workflows

## Accepted Implementation Scope

Implement RFC 0035 as test infrastructure only. The harness lives under `tests/`, exercises the existing daemon PostgreSQL migrations, cross-repo lifecycle helpers, daemon RPC method registry, MCP authorization surface, metadata-only audit helpers, and per-repo SQLite state, and adds no public operator-facing API. The current source has mock-heavy coverage for RFC 0032; this plan turns that into a real two-repository integration harness while keeping `.striatum/state.sqlite3` authoritative for each participant repository's live workflow state.

| RFC 0035 acceptance criterion | Concrete plan | Owner |
|---|---|---|
| `tests/_harness/` exists with `MultiRepoHarness`, daemon helpers, repo helpers, PG reset helpers, and MCP client helpers. | Add `tests/_harness/{__init__.py,multi_repo.py,daemon.py,repos.py,pg.py,mcp.py,tokens.py,audit.py,scope.py}`. Keep all helpers test-only and reuse production migrations, validators, lifecycle helpers, capability authorization, and audit hashing. | `tests/_harness/` |
| `MultiRepoHarness` boots a daemon plus N registered repositories; per-class fixture works; per-function reset works. | Implement class-scoped `multi_repo_harness` and function-scoped `clean_daemon_db`. Startup creates an ephemeral PG database, applies daemon migrations, starts the daemon with isolated runtime paths, initializes N target repos, registers them, and verifies schema/socket/repo ids. | `tests/_harness/multi_repo.py`, `tests/conftest.py` |
| Each end-to-end test file from RFC 0035 §4-§8 lands with the listed cases passing. | Add the five e2e modules named in RFC 0035 with the case lists below. Tests assert both daemon DB rows and per-repo SQLite rows when behavior crosses the boundary. | `tests/test_cross_repo_prepare_e2e.py`, `tests/test_cross_repo_lifecycle_e2e.py`, `tests/test_cross_repo_crash_recovery_e2e.py`, `tests/test_mcp_capability_scope_e2e.py`, `tests/test_per_repo_write_scope_e2e.py` |
| Harness smoke test passes. | Add start/register/reset/reregister/stop/start-again coverage before relying on the harness for cross-repo assertions. | `tests/test_multi_repo_harness.py` |
| `make test-multi-repo` runs harness tests against system PG. | Add a Make target that selects the `multi_repo` marker and the new e2e files. Add `pg-test` if it is still absent at implementation time. | `Makefile`, `pyproject.toml` marker |
| CI runs harness tests on Linux+PG and macOS+PG; cleanly skips on macOS-no-PG. | PG jobs install daemon PG extras and set `STRIATUM_TEST_POSTGRES_URL`. Non-PG jobs import the modules and skip with a clear reason, not silent deselection. | CI config plus `tests/_harness/pg.py` |
| Harness adds less than 60 seconds to local `make test`. | Use class-scoped daemon startup, per-function table reset, short health polling, and focused test cases. Avoid sleep-based crash tests. | `tests/_harness/daemon.py`, test modules |
| Existing single-repo tests continue unchanged. | Do not modify existing unit-test fixture shape. The new `tests/conftest.py` only adds fixtures and markers. | `tests/conftest.py` |
| Single-repo fixtures remain recommended for non-cross-repo behavior. | Document the harness as RFC 0032/RFC 0035 integration coverage only; ordinary tests continue to use pure dicts, fake runners, and temp single repos. | `docs/SPEC.md` or developer docs |
| Docs update TODO, SPEC if needed, and CHANGELOG. | Move TODO item 19 to most-done/done according to landed scope, add a test-infrastructure note, and avoid presenting the harness as product surface. | `docs/TODO.md`, `docs/SPEC.md`, `CHANGELOG.md` |

## Deferred Scope

| Deferred item | Why deferred | Landing place |
|---|---|---|
| Go-client testing surface | D084 requires the daemon protocol to survive a future Go core, but RFC 0035 is Python test infrastructure over the current daemon seams. | Future Go-core/client RFC |
| Two-repos-with-worktree-isolated-lanes example workflow | Useful operator onboarding, but it is an example workflow, not required for the harness acceptance criteria. | Follow-up examples RFC or TODO item |
| Docker-based ephemeral PostgreSQL | RFC 0033 chose system Postgres. Docker changes packaging and lifecycle responsibility. | Separate hardening RFC |
| Windows daemon harness | RFC 0030 V2 and RFC 0035 scope Linux/macOS local daemon paths; Windows daemon mode remains deferred. | Future Windows daemon support RFC |
| Cross-machine testing | D083 keeps daemon V2 single-user, single-machine and local-only. | Out of scope unless product boundary changes |
| Performance/load testing | The harness proves functional behavior and a wall-clock budget, not throughput. | Separate performance hardening effort |

## Harness Module Layout

Use the RFC 0035 tree with three small additions from the Claude design where they prevent duplication:

```text
tests/
  _harness/
    __init__.py
    multi_repo.py
    daemon.py
    repos.py
    pg.py
    mcp.py
    tokens.py
    audit.py
    scope.py
  test_multi_repo_harness.py
  test_cross_repo_prepare_e2e.py
  test_cross_repo_lifecycle_e2e.py
  test_cross_repo_crash_recovery_e2e.py
  test_mcp_capability_scope_e2e.py
  test_per_repo_write_scope_e2e.py
```

`multi_repo.py` composes the harness. `daemon.py` owns subprocess lifecycle, SIGTERM/SIGKILL, restart, socket/runtime paths, and optional deterministic pause hooks. `pg.py` owns base URL resolution, ephemeral database creation/drop, migration application, and table reset. `repos.py` initializes target repositories and writes workflow fixtures. `mcp.py` wraps `DaemonRpcServer`/MCP calls through the existing PG-backed seam. `tokens.py` issues, revokes, and expires test tokens, preferring production admin/token helpers and using direct DB setup only when no product helper exists. `audit.py` inspects rows and verifies hash-chain continuity using production hash helpers. `scope.py` holds path/symlink/write-scope helpers for the per-repo tests.

## `MultiRepoHarness` API

Implement this exact constructor:

```python
class MultiRepoHarness:
    def __init__(self, daemon_pg_url: str, repo_count: int = 2, scratch_dir: Path) -> None: ...
```

Expose these attributes and methods:

```python
repos: list[RepoDescriptor]
daemon_pg_url: str
daemon: DaemonProcess
socket_path: Path
admin_token: str | None

def start(self) -> None: ...
def stop(self) -> None: ...
def reset_daemon_db(self) -> None: ...
def reset_repo_local(self, repo_index: int) -> None: ...
def register_all(self) -> list[str]: ...
def pg_conn(self): ...
def issue_token(self, capabilities: list[str], repo_id: str | None = None, expires_in: int | None = 3600) -> str: ...
def revoke_token(self, token: str) -> None: ...
def expire_token(self, token: str) -> None: ...
def mcp_client(self, token: str) -> McpClient: ...
def audit_rows(self, *, transport: str | None = None) -> list[dict[str, object]]: ...
def assert_audit_chain(self) -> None: ...
def daemon_db_query(self, sql: str, args: Sequence[object] = ()) -> list[dict[str, object]]: ...
def repo_sqlite_query(self, repo_index: int, sql: str, args: Sequence[object] = ()) -> list[dict[str, object]]: ...
def prepare_cross_repo_run(self, workflow: dict[str, object]) -> dict[str, object]: ...
def start_cross_repo_run(self, cross_repo_run_id: str) -> dict[str, object]: ...
def cancel_cross_repo_run(self, cross_repo_run_id: str) -> dict[str, object]: ...
def describe_cross_repo_run(self, cross_repo_run_id: str) -> dict[str, object]: ...
def kill_daemon(self, signal: signal.Signals = signal.SIGKILL) -> None: ...
def restart_daemon(self) -> None: ...
def install_pause_hook(self, stage: str) -> PauseHook: ...
def simulate_repo_unreachable(self, repo_index: int) -> ContextManager[None]: ...
```

`start()` is not re-entrant while running; calling it twice should fail clearly. Calling `start()` after `stop()` with a new scratch/database allocation must work. The startup order is ephemeral PG database, migrations, daemon process, target repo init, repo registration, then a verification query for schema version and registered repo count.

## Fixture Scope

Default to a class-scoped fixture:

```python
@pytest.fixture(scope="class")
def multi_repo_harness(tmp_path_factory): ...
```

Use the function-scoped escape hatch when a test needs clean daemon state:

```python
@pytest.fixture
def clean_daemon_db(multi_repo_harness): ...
```

The smoke, prepare, lifecycle, MCP capability, and write-scope modules use the class-scoped fixture. Individual tests that mutate capability tokens, audit rows, or cross-repo run rows use `clean_daemon_db` and then call `harness.register_all()` explicitly. Crash-recovery tests use class scope but should run in isolated classes or fresh harness instances because they kill/restart daemon state.

## Per-Test DB Reset Semantics

`reset_daemon_db()` truncates daemon-owned data tables and preserves schema/migration/method metadata. It must not re-register repositories automatically.

Preserve:

```text
striatumd.schema_meta
striatumd.schema_migrations
striatumd.rpc_methods
```

Truncate and restart identity, using discovered table names where practical and explicit exclusions for the preserved tables:

```text
striatumd.repositories
striatumd.clients
striatumd.client_capabilities
striatumd.client_sessions
striatumd.rpc_request_log
striatumd.audit_log
striatumd.audit_segments
striatumd.audit_chain_head
striatumd.audit_repositories
striatumd.scheduler_cursors
striatumd.cross_repo_runs
striatumd.cross_repo_run_repositories
striatumd.cross_repo_cycle_counters
striatumd.process_supervisors
striatumd.apply_receipts
```

After truncating audit tables, re-seed the audit chain to the production fresh-install empty-chain state so the next audit row has a well-defined `previous_hash`. `test_multi_repo_harness.py` must assert this reset behavior.

Repo-local SQLite is not reset by `reset_daemon_db()`. `reset_repo_local(repo_index)` is available for tests that reuse a repo and need local `runs`, jobs, queues, leases, artifacts, verdicts, blockers, events, and supervisor pointers cleared while preserving repo-local schema metadata. Published test artifacts under the harness-created paths are removed by the same helper.

## Five E2E Test Files

### `tests/test_cross_repo_prepare_e2e.py`

- `test_prepare_valid_two_repo_workflow_creates_daemon_and_local_rows`: valid workflow validates, creates one `cross_repo_runs` row, creates two participant rows, and each repo-local `runs.cross_repo_run_id` points to the same id.
- `test_prepare_unknown_repository_id_refuses_without_writes`: workflow references an unregistered `repo_id`; daemon-backed prepare refuses and leaves no daemon run or repo-local run rows.
- `test_prepare_repo_local_failure_rolls_back_or_aborts_without_false_prepared`: simulate repo B SQLite failure; assert no `prepared` daemon row exists with a missing participant and no orphan local state is hidden.
- `test_validate_catches_shape_errors_before_prepare`: validator rejects undeclared aliases, missing `primary_repository`, duplicate repo ids, and missing job `repository` before daemon-backed registration checks.

### `tests/test_cross_repo_lifecycle_e2e.py`

- `test_prepare_start_summary_cancel_two_repo_run`: prepare, start, describe/summary, and cancel across two repos; daemon state and both local runs converge.
- `test_mixed_participant_state_is_reflected_in_describe_and_dashboard`: one participant terminal and one still running is reported with both aliases and repository ids.
- `test_cancel_mid_run_cascades_or_blocks_reachable_participants`: cancel attempts every non-terminal participant and records blocked participants without orphaning daemon state.
- `test_cross_repo_dashboard_includes_all_participants`: dashboard/status for the cross-repo id includes both participating repositories.
- `test_cross_repo_cycle_counter_increments_globally_once`: a `cross_repo_cycle: true` needs-revision path increments the daemon cycle counter once per global cycle, not once per repo.

### `tests/test_cross_repo_crash_recovery_e2e.py`

- `test_restart_after_crash_mid_prepare_aborts_or_completes_without_orphans`: deterministic pause after daemon DB write but before local rows, SIGKILL, restart, reconcile to `aborted` or completed prepared state per implemented contract, with no hidden orphan rows.
- `test_restart_after_crash_mid_start_completes_or_blocks_structured`: pause after first participant start, SIGKILL, restart, then converge to `running` or structured `blocked` with primary-repo human checkpoint.
- `test_restart_after_crash_mid_cancel_finishes_or_blocks`: pause during cancel cascade, SIGKILL, restart, then finish cancel or mark blocked participants explicitly.
- `test_unreachable_participant_mid_run_creates_primary_checkpoint`: chmod or otherwise make repo B `.striatum/` inaccessible, trigger a daemon touch, and assert daemon paused/blocked state plus a human checkpoint in the primary repo. Cleanup must restore permissions in `finally`.

Crash tests require deterministic hooks, not timing sleeps. If a hook must touch production code, it must be guarded by `STRIATUM_TEST_PAUSE_AT` and inert outside tests; otherwise use a harness runner around the existing `CrossRepoLocalRunner` seam without inventing alternate product behavior.

### `tests/test_mcp_capability_scope_e2e.py`

- `test_repo_scoped_write_token_allows_repo_a_write_and_audits_allowed`: repo A scoped `write` token can call a write tool against repo A and records `decision='allowed'`.
- `test_repo_scoped_write_token_denied_against_repo_b_and_audits_scope_mismatch`: same token against repo B is denied. Choose `capability_scope_mismatch` as the accepted vocabulary for scoped-elsewhere tokens and update docs if they still say only `capability_missing`.
- `test_read_only_token_lists_only_read_tools`: `tools/list` hides write/review/claim/apply/admin/recovery tools from a read-only token.
- `test_read_only_token_cannot_call_write_tool`: `tools/call` re-authorizes and denies write with `capability_missing`.
- `test_unknown_method_denied_and_audited`: unknown method returns `method_unknown` and writes a denied audit row.
- `test_revoked_token_denied_and_audited`: revoked token returns `token_revoked`.
- `test_expired_token_denied_and_audited`: expired token returns `token_expired`.
- `test_audit_chain_continuous_across_allowed_and_denied_calls`: mixed allow/deny sequence keeps `previous_hash`/`row_hash` continuous.

### `tests/test_per_repo_write_scope_e2e.py`

- `test_repo_b_job_publishes_artifact_inside_repo_b_scope`: a job targeting repo B publishes under repo B allowed paths successfully.
- `test_expected_artifact_escape_into_repo_a_rejected_before_prepare`: `../` or symlink path crossing into repo A is rejected by validation or daemon prepare before runtime.
- `test_runtime_publish_from_repo_b_into_repo_a_refused`: malicious runtime publish attempt through the normal `publish-artifact` path refuses with `write_scope_violation` and audits denial if routed through MCP/RPC.
- `test_job_targeting_undeclared_repository_alias_rejected_by_validator`: validator catches undeclared aliases; runtime path is unreachable.

## Harness Smoke Test

`tests/test_multi_repo_harness.py` owns:

- `test_start_registers_two_repos_with_distinct_sqlite_state`
- `test_reset_daemon_db_preserves_schema_metadata_and_clears_data`
- `test_register_all_after_reset_restores_repository_rows`
- `test_stop_drops_ephemeral_database_removes_scratch_and_socket`
- `test_start_stop_start_has_no_socket_or_database_collision`
- `test_reset_reseeds_empty_audit_chain`

## CI Integration

Add a marker:

```toml
markers = [
  "multi_repo: requires system PostgreSQL and the daemon multi-repo harness",
]
```

Add Make targets:

```make
.PHONY: pg-test test-multi-repo

pg-test: $(VENV)/.installed
	$(PYTHON) -m pytest tests/test_daemon_pg.py -q

test-multi-repo: $(VENV)/.installed
	$(PYTHON) -m pytest -m multi_repo \
		tests/test_multi_repo_harness.py \
		tests/test_cross_repo_prepare_e2e.py \
		tests/test_cross_repo_lifecycle_e2e.py \
		tests/test_cross_repo_crash_recovery_e2e.py \
		tests/test_mcp_capability_scope_e2e.py \
		tests/test_per_repo_write_scope_e2e.py
```

`tests/_harness/pg.py` resolves a base URL from `STRIATUM_TEST_POSTGRES_URL`, then `STRIATUM_DAEMON_DB_URL`, then a local default only if `psycopg` imports and the server is reachable. Missing PG skips with a message naming the requirement and `make pg-test`. CI PG jobs run the target on Linux and macOS. Non-PG jobs import and skip cleanly.

## Wall-Clock Budget

The added local `make test` cost must stay under 60 seconds when PG is available. The budget depends on class-scoped daemon startup, table truncation instead of database recreation per function, focused tests, short health checks, and no sleep-based crash timing. If the budget is exceeded, reduce test duplication before weakening assertions.

## Determinism And Cleanup Hygiene

Every harness instance uses unique scratch paths, a unique ephemeral database, short Unix socket paths for macOS, and isolated daemon registry/runtime environment variables. `stop()` sends SIGTERM, waits up to five seconds, escalates to SIGKILL only on timeout, closes DB connections, drops the database with force semantics where available, removes scratch, and asserts socket deletion. Permission-damage helpers must register cleanup callbacks so teardown can restore chmod state before `rmtree`.

## Cross-Platform

Support Linux and macOS. Keep Unix socket paths short for macOS. Windows daemon harness support is out of scope for this RFC and must be skipped explicitly rather than half-supported.

## No Parallel Production-Code Path

The harness must use the same daemon binary, daemon PG migrations, repo-local migrations, workflow validator, cross-repo lifecycle helpers, daemon RPC registry, capability vocabulary, MCP authorization path, and audit hash-chain helper as production code. It may query databases for assertions and perform explicit reset/setup, but it must not make tests pass through a separate cross-repo implementation.

One caveat is current source reality: `DaemonRpcServer` and `cross_repo.py` provide usable production seams, while a full daemon socket accept loop may lag. The harness may call those existing seams directly for assertions until the socket router catches up, but it must label that as exercising the production Python seam, not a new daemon protocol.

## Adversarial Test Cases

Fold this closed set into the e2e modules above rather than adding extra files in the first implementation:

- repo-scoped token used against another repo;
- read-only token calling a write tool;
- revoked token;
- expired token;
- unknown method;
- hostile `tools/list` arguments claiming elevated scope;
- spoofed trusted-client headers or arguments;
- symlink/path traversal from one repo into another;
- participant repo unreachable mid-run;
- audit `previous_hash` injection attempt in call params;
- audit-chain continuity after mixed allow/deny calls.

Operator-confirmation-gate bypass is not part of RFC 0035 because the current accepted MCP capability model is capability-token based, not browser confirmation based.

## Documentation Deltas

- `docs/TODO.md`: move Open item 19 to most-done when the harness skeleton and first e2e files land; mark done only when all RFC 0035 acceptance criteria pass.
- `docs/SPEC.md`: add a developer-test note that the deferred RFC 0032 multi-repo e2e coverage now exists under `tests/_harness/`; do not make it a user-facing product contract.
- `CHANGELOG.md`: add an Unreleased test-infrastructure entry for the multi-repo harness and `make test-multi-repo`.
- `docs/rfcs/0035...md`: update status only after implementation/review acceptance, not during the first skeleton commit.

## Staging Plan

1. Land harness skeleton plus smoke test: `tests/_harness/pg.py`, `daemon.py`, `repos.py`, `multi_repo.py`, `tests/conftest.py`, marker, and `tests/test_multi_repo_harness.py`.
2. Land prepare and lifecycle e2e tests, using workflow builders and existing `cross_repo.py` seams.
3. Land crash recovery e2e tests with deterministic pause hooks or the existing runner seam; no sleep races.
4. Land MCP capability-scope e2e tests, including token expiry/revocation and audit-chain continuity.
5. Land per-repo write-scope e2e tests through validator and runtime publish paths.
6. Wire CI and docs: `make test-multi-repo`, PG skip behavior, TODO/SPEC/CHANGELOG updates.

## Human-Decision Questions

- Should production code accept guarded `STRIATUM_TEST_PAUSE_AT` hooks for deterministic crash tests, or should crash tests stay on the existing `CrossRepoLocalRunner` seam until the daemon accept loop matures?
- Should RFC 0035 docs be edited to prefer `capability_scope_mismatch` for scoped-elsewhere tokens, preserving `capability_missing` for missing capability on the same scope?
- When all e2e files land, should TODO item 19 move to done immediately, or remain most-done until live scheduler progression through cross-repo jobs is also covered?
