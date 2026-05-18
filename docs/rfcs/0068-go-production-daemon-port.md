# RFC 0068: Go Production Daemon Port

Status: accepted
Date: 2026-05-17
Context: [RFC 0030](0030-daemon-rpc-server-and-version-skew-protocol.md), [RFC 0033](0033-storage-substrate-rewrite-for-daemon-v2.md), [RFC 0039](0039-go-daemon-core.md), [RFC 0043](0043-postgres-as-sole-substrate-and-daemon-required-runtime.md), [RFC 0059](0059-eradicate-legacy-sqlite-fallbacks.md), [DECISION_LOG.md](../DECISION_LOG.md)

## Problem

D105 encoded a Python-primary production daemon constraint during the
remediation sprint. That kept the product focused while the Python/PostgreSQL
path stabilized, but it is not the desired product direction.

The operator decision is now explicit: port the production daemon and daemon
runtime responsibilities to Go, retire the Python daemon once parity is
reached, keep the Python CLI/web layers where they remain useful, and eliminate
SQLite from production and compatibility paths.

## Goals

- Supersede D105 with D107: Go becomes the intended production daemon core.
- Keep Python only as CLI/web client code and transitional migration fixture
  support; the Python daemon is no longer selectable and `striatumd` no longer
  points at the legacy Python daemon module.
- Keep the Python CLI acceptable as a client of the Go daemon.
- Preserve the RFC 0030 envelope, capability, request-id, version-skew, audit,
  and method-registry semantics.
- Preserve the RFC 0033/RFC 0043 PostgreSQL substrate and daemon-required
  runtime.
- Remove SQLite from all production, service, MCP, dogfood, and operator-helper
  paths. SQLite may remain only as an explicit one-way import fixture until all
  legacy imports are removed.

## Non-Goals

- Rewrite the Python CLI in this RFC.
- Introduce hosted services, telemetry, external persistence, or cloud APIs.
- Keep a permanent dual-core product where Python and Go diverge in behavior.
- Keep repo-local SQLite as a supported compatibility mode.

## Proposal

The Go daemon port lands through independent, testable slices:

1. **Core gate and freshness.** The Go daemon must refuse to serve when its
   embedded migrations, method contract, generated registry, or packaged binary
   lag the Python/source contract. `--describe` and `daemon.hello` expose the
   core, version, supported schema, methods etag, and migration hash set.
2. **Go daemon method parity.** Replace `not_implemented` placeholders for all
   production daemon methods with Go handlers or explicitly remove the method
   from production surfaces until it is implemented.
3. **Go-owned global surfaces.** Move daemon startup, health, audit, sweep,
   dashboard-all, daemon MCP resources, and repository registration to Go over
   PostgreSQL.
4. **Client and service boundary.** Keep the Python CLI/web service as clients:
   no direct PostgreSQL repo resolution, no `striatum.api.invoke` production
   run authority for daemon-mapped reads or mutations, and no Python daemon
   fallback. The `striatumd` console script is a Go-daemon launcher shim.
5. **SQLite eradication.** Delete or port remaining SQLite-backed service,
   dogfood, adapter, byline, inbox, recovery, corpus, and local API helpers.
   Migration fixtures must be named as one-way import fixtures and isolated
   from production modules.
6. **Retirement.** Once the Go conformance suite passes, the explicit
   fail-closed retirement ledger is resolved, and the import-window fixtures
   are quarantined, remove the legacy Python daemon module and any
   Python-daemon-only production code.

## Acceptance Criteria

- D107 is recorded and D105 is superseded.
- `striatum daemon start` launches the Go daemon by default after active
  contract-method parity; D111 retires the Python core selector, so
  `--core python` and `STRIATUM_DAEMON_CORE=python` are no longer supported.
- The Go daemon supports the current PostgreSQL schema version and refuses stale
  packaged binaries with a rebuild/remediation hint.
- The Go daemon serves every production method in
  `contracts/daemon_methods.json` or hides unsupported methods from production
  clients.
- Production MCP discovery hides local workflow-file authoring methods. Removed
  dogfood composite names audit as `method_unknown`; any hidden registered
  methods still reauthorize and fail closed when called directly.
- CLI, web, MCP, and service tests pass against the Go daemon without direct
  SQLite opens.
- Production daemon/client paths do not open repo-local SQLite or the legacy
  daemon registry; remaining `sqlite3` / `striatum.db` imports are named
  migration, fixture, or transitional compatibility exceptions guarded by
  architecture tests.
- The Python daemon can be deleted without losing production behavior once the
  remaining legacy harness, sealed-apply, and import-window tasks are done.

## Implementation Notes

- `make daemon-go-conformance` is now the Go daemon CI/release gate. It builds
  and tests the Go daemon, then runs the PostgreSQL multi-repo harness with
  `CORE=go`, including Go daemon smoke, audit, mutation-registry, and
  supervisor smoke coverage. CI runs that gate on Linux where the PostgreSQL
  service is available.
- The multi-repo harness participant runner writes prepare/start/cancel and
  human-checkpoint state to daemon-owned PostgreSQL tables instead of creating
  or querying `.striatum/state.sqlite3` in target repositories.
- Go `run.prepare` uses the Go workflow-authoring loader for source-path
  resolution before inserting workflow snapshot rows, so traversal refusal and
  JSON-only workflow-source validation no longer depend on Python-daemon
  behavior.
- Go `workflow.upgrade --add-phases` now ports the Python V1-to-V1.1
  phase-inference path, including preview/apply behavior, synthesis-job
  insertion, cross-phase edge rewriting, and the PostgreSQL non-terminal-run
  guard.
- Go `workflow.generate --shape multi_phase` now ports the Python generator's
  V1.1 output path, including ordered phases, per-track job remapping,
  `phase_synthesis` gates, and cross-phase synthesis-to-entry edges.
- As of 2026-05-18, Go handler coverage reports zero missing or generic
  `not_implemented` handlers for active contract methods. The remaining
  Python-daemon retirement blockers are executable in
  `go/cmd/striatumd/handler_coverage_test.go`: `apply.reviewed_patch`
  must keep returning a named fail-closed RPC error until it is ported or
  removed by product decision. D110 removed `daemon.migrate_repo_local`,
  `dogfood.publish_on_behalf`, and `dogfood.surgical_recovery` from the
  production method contract.
- Python/Go production MCP `tools/list` now hides local workflow-file
  authoring methods. Daemon MCP `resources/list` and `resources/read` use
  PostgreSQL-backed repository visibility and read projections whenever a
  daemon PostgreSQL connection is present.
- SQLite remnants allowed under this RFC are one-way migration/test fixtures
  only. Production daemon, service, MCP, and operator-helper paths must not
  reopen repo-local SQLite or the legacy daemon registry.
- `striatum daemon start` now always launches the Go daemon. `--core go`
  remains a deprecated no-op compatibility flag; the Python daemon is not
  selectable by CLI flag or environment variable. The `striatumd` console
  script now routes through a launcher shim that delegates to the same Go
  startup path without importing `striatum.daemon`.
- Runtime path/token helpers have moved to `striatum.daemon_runtime`, and
  PostgreSQL repository-registration helpers used by day-zero and daemon RPC
  live in `striatum.daemon_pg.repositories`; remaining imports from
  `striatum.daemon` are legacy daemon, migration, or compatibility debt.
- SQLite-era repository identity and daemon audit-chain validation used by
  one-way migration fixtures now live in `striatum.daemon_pg.sqlite_compat`
  instead of importing `striatum.daemon`.

## Retirement Gate

The Python daemon can be deleted after this ledger reaches zero or every
remaining row has been removed from production discovery and the daemon method
contract:

| Method | Current Go behavior | Retirement action |
|---|---|---|
| `apply.reviewed_patch` | Fails closed with `sealed_key_missing` / `apply_gate_unsatisfied`. | Decide sealed-apply authority or remove the mutation from the production contract. |

Removed from the production contract by D110: `daemon.migrate_repo_local`,
`dogfood.publish_on_behalf`, and `dogfood.surgical_recovery`. The local
`striatum daemon migrate-repo-local` helper remains an explicit one-way
migration fixture until the SQLite import window closes.

`make daemon-go-conformance`, `go test ./cmd/striatumd`, and
`tests/architecture/test_authority_guardrails.py` are the executable cutover
checks for this ledger.

## Resolved Questions

- D109 resolved the daemon-core default, and D111 completed the selector
  retirement: `striatum daemon start` launches Go, `--core go` is a
  deprecated no-op, and Python is no longer selectable as a daemon core.

## Open Questions

- Should historical SQLite import fixtures be retained indefinitely for
  migration tests, or removed after a final deprecation window?

## Domain Modeling

This RFC changes an implementation boundary, not the workflow model. The daemon
core is a runtime implementation of the existing daemon aggregate authority.
Workflow state, method semantics, and audit events remain the same model
described in [`docs/DDD.md § Adding to the model`](../DDD.md#adding-to-the-model).
