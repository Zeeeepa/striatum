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
- Keep Python as the current implementation only until the Go daemon passes
  the production conformance gate.
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
   fallback.
5. **SQLite eradication.** Delete or port remaining SQLite-backed service,
   dogfood, adapter, byline, inbox, recovery, corpus, and local API helpers.
   Migration fixtures must be named as one-way import fixtures and isolated
   from production modules.
6. **Retirement.** Once the Go conformance suite passes, remove the Python
   daemon entry point and any Python daemon-only production code.

## Acceptance Criteria

- D107 is recorded and D105 is superseded.
- `striatum daemon start` launches the Go daemon by default after the final
  parity phase; before that, Go is selectable and clearly marked as the target
  production core.
- The Go daemon supports the current PostgreSQL schema version and refuses stale
  packaged binaries with a rebuild/remediation hint.
- The Go daemon serves every production method in
  `contracts/daemon_methods.json` or hides unsupported methods from production
  clients.
- CLI, web, MCP, and service tests pass against the Go daemon without direct
  SQLite opens.
- Production source modules no longer import `sqlite3` or `striatum.db`; any
  remaining imports live under migration/fixture packages with guardrail tests.
- The Python daemon can be deleted without losing production behavior.

## Implementation Notes

- `make daemon-go-conformance` is now the Go daemon CI/release gate. It builds
  and tests the Go daemon, then runs the PostgreSQL multi-repo harness with
  `CORE=go`, including Go daemon smoke, audit, mutation-registry, and
  supervisor smoke coverage. CI runs that gate on Linux where the PostgreSQL
  service is available.
- Go `run.prepare` uses the Go workflow-authoring loader for source-path
  resolution before inserting workflow snapshot rows, so traversal refusal and
  JSON-only workflow-source validation no longer depend on Python-daemon
  behavior.
- Go `workflow.upgrade --add-phases` now ports the Python V1-to-V1.1
  phase-inference path, including preview/apply behavior, synthesis-job
  insertion, cross-phase edge rewriting, and the PostgreSQL non-terminal-run
  guard.

## Open Questions

- Should the intermediate default remain Python until all slices pass, or flip
  to Go behind an explicit `STRIATUM_GO_DAEMON_EXPERIMENTAL=1` gate earlier?
- Should historical SQLite import fixtures be retained indefinitely for
  migration tests, or removed after a final deprecation window?

## Domain Modeling

This RFC changes an implementation boundary, not the workflow model. The daemon
core is a runtime implementation of the existing daemon aggregate authority.
Workflow state, method semantics, and audit events remain the same model
described in [`docs/DDD.md § Adding to the model`](../DDD.md#adding-to-the-model).
