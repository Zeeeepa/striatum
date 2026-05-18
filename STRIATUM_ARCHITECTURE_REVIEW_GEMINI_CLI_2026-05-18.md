# STRIATUM ARCHITECTURE REVIEW
author: reviewer-gemini-cli-001

**Date:** 2026-05-18
**Reviewer:** Gemini CLI
**Project:** striatum-orchestrator (Striatum)

---

## Operator disposition

This is retained as an external critique artifact, not as current product
direction. The recommendations to return to embedded SQLite/DuckDB or rewrite
the Python CLI/web layer into Go are rejected for the current roadmap by
D094, D107, and D117. The accepted overlap is narrower: delete the remaining
legacy Python daemon/SQLite fixture debt and improve first-run diagnostics.
Use `STRIATUM_REMEDIATION_PLAN_GEMINI_CLI_2026-05-18.md` and `docs/TODO.md`
as the actionable backlog, not the rejected storage/runtime recommendations
below.

## 0. Files reviewed

- `README.md`
- `pyproject.toml`
- `docs/SPEC.md`
- `docs/DDD.md`
- `docs/PRD.md`
- `docs/rfcs/0068-go-production-daemon-port.md`
- `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`
- `src/striatum/cli/dispatch.py`
- `src/striatum/daemon.py`

## 1. Executive summary

- **Clear Domain Boundaries:** Striatum effectively employs Domain-Driven Design (DDD). The vocabulary dictates the workflow state, ensuring AI agents cannot bypass lifecycle constraints.
- **Enforced Go/PG Architecture:** The project has successfully cut over to a Go-based production daemon and PostgreSQL authoritative state. The CLI actively fails closed if daemon routing fails, rather than silently falling back.
- **Legacy Quarantine:** The old Python daemon and SQLite codebase have been successfully relegated to test-harness compatibility and migration fixtures, guarded by environment tripwires.
- **Robust Provenance:** State is tracked in a Postgres DB using a hash-chained event log, ensuring audit-quality provenance.
- **Complex Substrate for Local-First:** The strict requirement of a system-installed PostgreSQL instance remains a high-friction constraint for a local-first laptop tool, despite the excellent isolation it provides.

## 2. What the project is trying to be

**Stated goals:** Striatum aims to be a local-first workflow runner and coordinator for terminal-based AI coding agents. It explicitly avoids being a hosted service, a telemetry sink, or a continuous integration server. Its core value proposition is reviewer independence, provider portability, and audit-quality provenance.
**Operating model:** A local Go background daemon (`striatumd`) owns the live state in PostgreSQL, while the target repository simply holds durable artifacts. Python serves as the CLI, Web UI, and client API.
**Actual code:** The implementation strictly honors this model. RFC 0068 and RFC 0069 explicitly name Go as the production daemon core and mandate PostgreSQL for all global and repo-scoped state.

## 3. Current architecture

- **Components:** A Python 3.11+ core orchestrator (CLI/Web clients) and a Go-based production daemon (`striatumd`).
- **Runtime:** A local Go daemon communicating over Unix sockets, taking precedence over the deprecated Python daemon.
- **State/Storage:** Authoritative state lives in a system-installed PostgreSQL database. The older SQLite implementation is formally quarantined.
- **Surfaces:** CLI, an MCP-like wrapper, and a Web Dashboard.
- **Test Posture:** Highly integration-focused.
- **Release Posture:** Python wheel packaging accompanied by Go cross-compilation for the daemon binary.

## 4. Strengths

- **Fail-Closed Dispatch:** `src/striatum/cli/dispatch.py` now explicitly catches route failures and exits rather than silently falling back to SQLite. This enforces the Go/PG architectural boundary.
- **Legacy Quarantine:** `src/striatum/daemon.py` uses `STRIATUM_SQLITE_CONNECT_TRIPWIRE` to prevent accidental production use of the SQLite registry, physically isolating the legacy substrate without losing historical test coverage.
- **Hash-Chained Provenance:** Appending state changes to the `events` table with `previous_hash` / `row_hash` linkage offers tamper-evident auditing.

## 5. Concerns

- **System Postgres for a Local Tool (Blocker):** For a single-operator laptop tool, requiring a background PostgreSQL database engine is massive friction. The docs assert this is the path forward, but abandoning an embedded database for a system dependency directly harms the "day-zero usage" goal.
- **Dual-Language Build Matrix (Serious):** Cross-compiling Go binaries and staging them into Python wheels introduces disproportionate build complexity for a single-developer homelab project.
- **Lingering Legacy Code Volume (Smell):** Even though `src/striatum/daemon.py` and other SQLite paths are safely quarantined, they still constitute thousands of lines of dead weight in the active repository that slow down refactoring and searching.

## 6. North-star architecture

Given the constraints - single operator, homelab/laptop runtime, demo-stage maturity - the current architecture is artificially inflated by the system PostgreSQL dependency and the Python/Go split.

If building greenfield under these exact constraints: I would build the entire system as a **single statically-compiled Go binary**. The state store would be an embedded database (SQLite or DuckDB with WAL enabled). An embedded DB natively supports the "local-first, zero-config" mandate. The single binary would embed the daemon, the CLI, and the supervisor logic, eliminating the need for Unix socket IPC between different language runtimes and sidestepping the wheel packaging and cross-compilation dance entirely. No system DB, no Python virtual environments.

## 7. Recommended changes

| Priority | Change | Rationale | Benefit | Risk | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | **Delete Quarantined Legacy Code** | `src/striatum/daemon.py` and `src/striatum/legacy_sqlite/` are heavily guarded but still massive. Convert the final fixtures and `rm -rf` the legacy backend entirely. | Eliminates architectural confusion and reduces maintainer burden. | Loss of historical test coverage if not ported carefully. | 1 week |
| 2 | **Revert to Embedded Storage (SQLite/DuckDB)** | System PostgreSQL introduces massive adoption friction for a laptop tool. Embedded DBs support robust concurrent writers (via WAL) without a background server. | Drastically simplifies day-zero installation and runtime orchestration. | Re-writing the migrations and RPC wrappers back from PG. | 3 weeks |
| 3 | **Unify Language Runtimes** | If Go is the daemon, rewrite the CLI/Web UI in Go (using Bubbletea/Templ) and ship a single binary. | Drops the Python wheel packaging matrix and simplifies IPC. | Huge rewrite effort for the UI. | 4 weeks |

## 8. Functionality I'd add

| Priority | Change | Rationale | Benefit | Risk | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | **Single-Command Doctor/Smoke** | Combine `adopt --first-run-smoke` and `daemon doctor --authority` into a single diagnostic command. | Reduces operator friction when debugging the multi-component stack. | None. | 2 days |
| 2 | **Event Log Rotation** | The `events` table is append-only. Add a `daemon prune` command to archive old workflow runs to disk. | Prevents local disk exhaustion. | Deleting chained hashes requires a root-hash rollover mechanism. | 1 week |

## 9. Execution roadmap

- **Startable today:** Delete the remaining dead-code in `src/striatum/legacy_sqlite/` and `src/striatum/daemon.py` that isn't strictly required for the final migration fixtures.
- **Near-term (month):** Implement the unified single-command diagnostic tool to aid local support.
- **Medium-term (quarter):** Begin exploring a single-binary Go rewrite to eliminate the Python dependency entirely.
- **Long-term:** Migrate the PostgreSQL daemon logic back to an embedded SQLite instance, finalizing the zero-dependency local-first vision.

## 10. Open questions

- **Why Postgres?** The decision log (RFC 0043) references abandoning SQLite for Postgres, but the operational trade-offs for a strictly local, non-distributed tool are staggering. Was SQLite locking genuinely a bottleneck for single-operator parallel agents?
- **Tombstone Lifecycle:** Once legacy data is fully migrated to Postgres and `state.sqlite3.tombstone` files are left behind, what is the product decision on their lifecycle? Should the daemon eventually auto-delete them?
