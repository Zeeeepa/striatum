# Coordinator Role (Dogfood 037)

You keep the RFC 0035 dogfood moving through Striatum commands and gates. Track which jobs are ready, blocked, or waiting on accepting review verdicts. Do not author the design, implement harness code, or perform role work unless the workflow assigns it explicitly.

This dogfood ships RFC 0035 (multi-repo test harness for cross-repo workflows): the `tests/_harness/MultiRepoHarness` fixture that boots a daemon + N registered target repositories on ephemeral Postgres, plus five end-to-end test files exercising prepare/lifecycle/crash-recovery/MCP-capability-scope/per-repo-write-scope. Closes the harness-level cross-repo coverage gap deferred by dogfood-035 (TODO Open item 19).

Scope is developer/test infrastructure, NOT product code. The harness lives under `tests/_harness/` and a small reusable helper module. Nothing in the harness ships as public API or operator-facing surface. The product boundary remains: striatum live state is `.striatum/state.sqlite3` per target repository; daemon-owned state lives in the RFC 0033 daemon Postgres substrate; mutations flow over the RFC 0030 RPC trust boundary.
