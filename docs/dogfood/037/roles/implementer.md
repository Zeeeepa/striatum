# Implementer Role (Dogfood 037)

You implement only the design scope accepted by the threat_model design review. Stay inside the job write scope, update tests for behavior changes, and keep docs aligned with actual runner behavior.

Use sub-agents aggressively per the implement prompt's delegation criteria. RFC 0035 V1 is medium scope (harness skeleton + 5 e2e files + CI wiring + docs). Parallelism is most useful for: per-helper module (`multi_repo.py`, `daemon.py`, `repos.py`, `pg.py`, `mcp.py`), per-e2e-test-file, harness smoke test, `make test-multi-repo` recipe + skip path, and doc surfaces.

The scope is developer/test infrastructure — no public API surface, no production code paths. The harness uses the same daemon binary, same migrations, same RPC envelope, same capability vocabulary as production; it does NOT introduce a parallel code path.

**Operational note on long-running `make test`:** Per the dogfood-038 OPERATOR_REPORT intervention #5 friction pattern, a lease can expire while codex is mid-`make test` if the test run takes longer than ~30 minutes. To avoid that pattern, run the harness tests as a focused invocation (`pytest tests/test_multi_repo_harness.py tests/test_cross_repo_*_e2e.py` rather than the full `make test`) before doing the wider `make install/lint/typecheck/test/smoke` final verification. If the lease still expires, the operator handles surgical recovery; do not retry destructively.

Devil's-advocate and security reviews are post-implementation per operator decision (commit 9d95487). Your acceptance bar is the threat_model build review (claude_code, fresh, repo-level) plus `make install`, `make lint`, `make typecheck`, `make test`, `make smoke`, and `make test-multi-repo`.

Per D089/D091: the OPERATOR_REPORT.md is the operator's responsibility — written incrementally per intervention, not only at the end. Your BUILD_HANDOFF.md documents what shipped, what's deferred (Go-client testing surface; two-repos-with-worktree-isolated-lanes example workflow), and what remains for follow-up RFCs.
