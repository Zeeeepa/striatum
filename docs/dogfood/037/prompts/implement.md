# Implement Prompt

Implementation is blocked until `review_design_threat` returns an accepting verdict. Do not start implementation from RFC 0035 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/037/DESIGN_SYNTHESIS.md` and the resolved threat-model review findings. Stay inside the workflow write scope.

Expected behavior:

**RFC 0035 V1 slice:**

- `tests/_harness/` module skeleton per the synthesis (`__init__.py`, `multi_repo.py`, `daemon.py`, `repos.py`, `pg.py`, `mcp.py`)
- `MultiRepoHarness` fixture with `start`/`stop`/`reset_daemon_db`/`register_all` lifecycle
- Ephemeral Postgres database creation against the existing system PG + RFC 0033 migration apply
- Daemon subprocess boot with ephemeral Unix socket
- Per-test repo init + register helpers
- Per-test DB reset (TRUNCATE all daemon DB tables except `schema_version`)
- Token issuance helper + MCP client helper + audit-row inspection helper + daemon-DB query helper + per-repo SQLite query helper
- `tests/test_multi_repo_harness.py` smoke test
- Five end-to-end test files: `test_cross_repo_prepare_e2e.py`, `test_cross_repo_lifecycle_e2e.py`, `test_cross_repo_crash_recovery_e2e.py`, `test_mcp_capability_scope_e2e.py`, `test_per_repo_write_scope_e2e.py`
- `Makefile` `test-multi-repo` target + skip-with-message when PG is unavailable
- `tests/conftest.py` integration (ephemeral-PG fixture wiring, skip-when-no-PG decorator)
- Documentation updates: TODO Open item 19 marked done with this dogfood as the landing point; SPEC or HOW_TO_HUMAN note pointing at the harness; CHANGELOG Unreleased entry

Do NOT:

- introduce a Go-client testing surface (deferred per RFC 0035 §Open Questions and D084 future);
- author the `examples/` two-repos-with-worktree-isolated-lanes workflow (deferred);
- bundle Postgres into Docker for the harness (separate hardening RFC);
- add Windows daemon support to the harness (out of scope per RFC 0030 V2);
- introduce cross-machine multi-tenant testing (out of scope per D083);
- add load/performance tests (separate effort);
- introduce a parallel production-code path (use the same daemon binary, same migrations, same RPC envelope, same capability vocabulary, same audit chain helper);
- add devil's-advocate / security review jobs to this dogfood's workflow (deferred per operator decision in commit 9d95487).

**Test coverage requirements:**

- Harness smoke test (start + register + reset + stop; back-to-back harness instances)
- Prepare e2e cases per the synthesis
- Lifecycle e2e cases per the synthesis (including cross-repo cycle iteration accounting)
- Crash recovery e2e cases per the synthesis (SIGKILL mid-prepare/start/cancel; one-repo-unreachable pause)
- MCP capability scope e2e cases (scope mismatch, unknown method, revoked token, expired token, `tools/list` filtering, audit chain continuity)
- Per-repo write-scope e2e cases (validator-time refusal, runtime refusal, repo-targeting checks)
- Adversarial cases per the synthesis (hostile MCP client, audit tamper attempt, operator-confirmation bypass)

## Maximize sub-agent usage where it helps

Per the harness profile, native sub-agent delegation is **encouraged**. Spawn sub-agents in parallel for work that's independent enough to parallelize:

- one sub-agent per `tests/_harness/` helper module (`multi_repo.py`, `daemon.py`, `repos.py`, `pg.py`, `mcp.py`)
- one sub-agent per e2e test file (5 total)
- one sub-agent for the harness smoke test
- one sub-agent for the `Makefile` target + conftest integration
- one sub-agent for docs (TODO mark-done, SPEC/HOW_TO_HUMAN cross-reference, CHANGELOG entry)
- exploratory sub-agents to read the existing `src/striatum/cross_repo.py`, `src/striatum/workflow.py`, `src/striatum/daemon_rpc/`, `src/striatum/mcp.py`, `src/striatum/daemon_pg/`, the unit tests from dogfood-035, and the existing `tests/conftest.py` to identify integration points

Do NOT delegate (parent session owns these):

- the BUILD_HANDOFF.md authorship
- the integration step where sub-agents' outputs are reconciled
- `make install`/`lint`/`typecheck`/`test`/`smoke`/`test-multi-repo` invocations
- final commit-shape and scope discipline

**Operational note on long-running test runs:** Per the dogfood-038 OPERATOR_REPORT intervention #5 friction pattern, a lease can expire while codex is mid-`make test` if the test run takes longer than ~30 minutes. To avoid that pattern, run the harness tests as a focused invocation first (`pytest tests/test_multi_repo_harness.py tests/test_cross_repo_*_e2e.py tests/test_mcp_capability_scope_e2e.py tests/test_per_repo_write_scope_e2e.py`) then do the wider `make install/lint/typecheck/test/smoke/test-multi-repo` final verification. If the lease still expires, the operator handles surgical recovery; do not retry destructively.

## Verification

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke`, and `make test-multi-repo` after all changes are in place.

## Handoff

Produce `docs/dogfood/037/BUILD_HANDOFF.md` summarizing changes, new modules, tests added/passing, deferred items with pointers, and any human-decision items the threat-model review did not pre-resolve. If sub-agents were used, briefly note which sub-tasks were delegated.

The byline must be `author: implementer-codex-gpt-5.5-001` (or whatever the work packet supplies) — plain Markdown line, lowercase `author:`, no decoration.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
