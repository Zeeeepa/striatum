# Codex Design Prompt

Produce `docs/dogfood/037/design/codex/DESIGN.md`.

Design an implementation plan for RFC 0035: the multi-repo test harness for cross-repo workflows. The harness exercises RFC 0032 V2 (shipped v1.24.0, cross-repo workflows + MCP mutation capabilities), RFC 0030 (daemon RPC), RFC 0031 (daemon-owned supervision + sealed-apply), and RFC 0033 V2 (system Postgres substrate) end-to-end at the test level. Do not redesign any of those — the harness wraps them.

Your plan must cover:

**Harness module layout:**

```
tests/_harness/
  __init__.py
  multi_repo.py          # MultiRepoHarness fixture
  daemon.py              # ephemeral daemon subprocess helpers
  repos.py               # per-repo init + register helpers
  pg.py                  # ephemeral PG database + reset helpers
  mcp.py                 # MCP client helpers for capability tests
tests/test_multi_repo_harness.py   # smoke test for the harness itself
tests/test_cross_repo_prepare_e2e.py
tests/test_cross_repo_lifecycle_e2e.py
tests/test_cross_repo_crash_recovery_e2e.py
tests/test_mcp_capability_scope_e2e.py
tests/test_per_repo_write_scope_e2e.py
```

**`MultiRepoHarness` API:**

- `__init__(daemon_pg_url, repo_count=2, scratch_dir)`
- `.start()` — create ephemeral PG database; apply daemon migrations; boot daemon subprocess with ephemeral Unix socket; init N target repos under `scratch_dir/repo-{0..N-1}/`; register all repos with the daemon
- `.stop()` — SIGTERM daemon → wait → drop ephemeral PG DB → rm scratch dir
- `.reset_daemon_db()` — TRUNCATE every daemon DB table except schema-version row; does NOT re-register repos (explicit `harness.register_all()` for that)
- `.register_all()` — re-register all participating repos
- Token issuance helper: `harness.issue_token(capability, repo_id=None, expires_in=3600)` returning a capability token
- MCP client helper: `harness.mcp_client(token)` returning a small client object with `tools_list()` + `tools_call(name, args)` methods
- Audit-row inspection helper: `harness.audit_rows(transport=None)` returning the daemon DB audit rows (with hash chain) for assertions
- Daemon-DB row inspection: `harness.daemon_db_query(sql, args)` for cross-repo run row + per-repo row checks
- Per-repo SQLite inspection: `harness.repo_sqlite_query(repo_index, sql, args)` for asserting per-repo state

**Per-test database reset semantics:**

- `multi_repo_harness` fixture: per-class scope by default
- `clean_daemon_db` fixture: per-function escape hatch that calls `harness.reset_daemon_db()` before the test
- Tests that need fresh repo registration explicitly call `harness.register_all()`

**The five e2e test files' case matrices:**

- `test_cross_repo_prepare_e2e.py` — well-formed cross-repo prepare; malformed `repositories` block (unknown repo_id) refused; daemon-DB write succeeds but per-repo SQLite fails → full rollback; validator catches at submit time vs runtime.
- `test_cross_repo_lifecycle_e2e.py` — prepare → start → summary → cancel across two repos; mixed state (one repo done, one running); cancel mid-run cascades; dashboard --run-id shows both; cross-repo cycle iteration accounting (max_iterations global).
- `test_cross_repo_crash_recovery_e2e.py` — SIGKILL mid-prepare → reconciliation rolls back daemon row; SIGKILL mid-start → reconciliation completes or fails; SIGKILL mid-cancel → cascade finishes on restart; one repo's `.striatum/` chmod 000 mid-run → pause + human checkpoint.
- `test_mcp_capability_scope_e2e.py` — write token scoped to repo A succeeds against repo A (audit allowed); same token against repo B refused with `capability_missing` (audit denied); read-only token sees only read tools in `tools/list`; unknown method → `method_unknown` audit; revoked token → `token_revoked`; expired token → `token_expired`; audit chain continuity across all of the above.
- `test_per_repo_write_scope_e2e.py` — job targeting repo B publishes artifact to repo B path → success; expected_artifacts crossing repo boundaries → validator refuses at submit; runtime attempt to write repo A path from job targeting repo B → `write_scope_violation`; repo_write job targeting unregistered repo → validator refuses.

**Harness smoke test:**

`tests/test_multi_repo_harness.py` verifies the harness itself: start + register 2 repos + stop; `reset_daemon_db()` clears every table except schema-version; ephemeral PG DB dropped on stop; scratch dir removed on stop; Unix socket deleted on stop; double-start works (no port/socket collisions).

**CI integration:**

- `make test-multi-repo` runs only harness-backed tests
- `make pg-test` (already exists per RFC 0033) ensures local PG is available
- Existing `make test` includes harness tests by default if PG is available; skip with clear message if not
- CI matrix: Linux+PG full coverage; macOS+PG full coverage; macOS-no-PG skips with message

**Wall-clock budget:** harness adds < 60 seconds to local `make test`. Per-class fixture scope amortizes daemon startup across multiple tests; per-class teardown drops the ephemeral DB.

**Concrete touch points:**

- New `tests/_harness/` module
- Five new e2e test files
- `tests/test_multi_repo_harness.py` smoke test
- `tests/conftest.py` integration (skip-when-no-PG decorator, ephemeral-PG fixture wiring)
- `Makefile` — `test-multi-repo` target
- Documentation updates: SPEC (or TODO) noting the harness exists; CLI_REFERENCE cross-reference; CHANGELOG entry under Unreleased.

**Explicitly deferred:**

- Go-client testing surface (RFC 0035 §Open Questions; D084 future)
- Two-repos-with-worktree-isolated-lanes example workflow under `examples/` (follow-up)
- Docker-based ephemeral Postgres (separate hardening RFC)
- Windows daemon harness (per RFC 0030 V2 scope)
- Cross-machine multi-tenant testing (D083 scope)
- Performance / load testing

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
