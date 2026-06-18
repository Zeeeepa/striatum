# STRIATUM ARCHITECTURE REVIEW

**Date:** 2026-05-17  
**Reviewer:** Gemini  
**Project:** striatum-orchestrator (Striatum)

---

## 0. Files reviewed

- `README.md`
- `pyproject.toml`
- `Makefile`
- `docs/SPEC.md`
- `docs/DDD.md`
- `docs/PRD.md`
- `tests/test_cli_mvp.py`
- `tests/` (Directory structure and scale)
- `src/striatum/` (Directory structure and module boundaries)

## 1. Executive summary

- **Clear Domain Model:** Striatum effectively employs Domain-Driven Design (DDD). The vocabulary dictates the workflow state, ensuring AI agents cannot bypass lifecycle constraints.
- **Robust Provenance:** State is tracked in a Postgres DB using a hash-chained event log, ensuring audit-quality provenance and preventing silent modifications.
- **Strict Boundaries:** Mutations occur strictly via daemon RPCs. Agents are relegated to client surfaces (CLI, MCP), preventing raw file-system or DB manipulation for state management.
- **Complex Substrate for Local-First:** The migration from SQLite to a system-installed PostgreSQL instance contradicts the frictionless nature of a local-first laptop tool.
- **Heavy Test Posture:** Comprehensive test coverage exists (~32k lines of tests vs. ~58k lines of Python), but the integration tests (e.g., `test_cli_mvp.py`) are monolithic and unwieldy.

## 2. What the project is trying to be

**Stated goals:** Striatum aims to be a local-first workflow runner and coordinator for terminal-based AI coding agents. It explicitly avoids being a hosted service, a telemetry sink, or a continuous integration server. Its core value proposition is reviewer independence, provider portability (no hardcoded vendor SDKs), and audit-quality provenance via a hash-chained event log.
**Operating model:** A local background daemon (`striatumd`) owns the live state in PostgreSQL, while the target repository simply holds durable artifacts (code, Markdown findings). AI agents interact with the daemon via capability-gated RPC methods (CLI or MCP).
**Actual code:** The implementation strictly honors this model. Agents must acquire leases and submit explicit packets (e.g., `accept_with_findings`) rather than loosely writing "LGTM" into files. The project strictly refuses outbound network calls or SaaS dependencies.

## 3. Current architecture

- **Components:** A Python 3.11+ core orchestrator, a local Web UI (server-rendered Jinja2 + JS islands), and a Go-based PTY supervisor helper.
- **Runtime:** A local daemon communicating over Unix sockets or TCP loopback.
- **State/Storage:** Authoritative state lives in a system-installed PostgreSQL database. The state is divided between daemon-global (registry) and per-repo schemas (`runs`, `jobs`, `events`). The older SQLite repo-local implementation has been formally tombstoned (RFC 0043/0048).
- **Surfaces:** CLI, an MCP-like wrapper, and a Web Dashboard.
- **Test Posture:** Highly integration-focused. The test suite is massive, representing over a third of the codebase by line count. `test_cli_mvp.py` alone is over 4,000 lines. The tests exercise real daemon/Postgres interactions.
- **Release Posture:** Python wheel packaging accompanied by Go cross-compilation for the supervisor helpers, managed via a monolithic `Makefile`.

## 4. Strengths

- **Domain-Driven Guardrails:** The rigid enforcement of DDD principles is a triumph. By modeling `verdicts`, `blockers`, and `events` as the only write surfaces, Striatum prevents the classic failure mode of AI wrappers: agents improvising the state machine via loose shell commands.
- **Hash-Chained Provenance:** Appending state changes to the `events` table with `previous_hash` / `row_hash` linkage offers tamper-evident auditing. This is critical when AI outputs form the chain of trust.
- **Capability-Gated RPCs:** Decoupling the interface from the execution substrate via Daemon RPCs (`read`, `write`, `review`, `apply` capabilities) effectively sandboxes the AI agent from the orchestrator's internals.

## 5. Concerns

- **System Postgres for a Local Tool (Blocker / Smell):** `docs/SPEC.md` states Postgres 14+ is required as a system install. For a local-first, single-operator laptop tool, requiring a background database engine is massive friction. The docs mention the D094 cutover from SQLite; however, abandoning an embedded database for a system dependency directly harms the "day-zero usage" goal.
- **Dual-Language Complexity (Serious):** The core is Python, but `Makefile` and `docs/SPEC.md` reveal a Go-based PTY supervisor helper (`go/cmd/striatumd`). Cross-compiling Go binaries and staging them into Python wheels (as seen in `daemon-go-release`) introduces disproportionate build complexity for a single-developer homelab project.
- **Monolithic Integration Tests (Smell):** `tests/test_cli_mvp.py` is over 4,000 lines long. While end-to-end coverage is valuable, monolithic test files create a massive tax on the sole maintainer when refactoring core flows.

## 6. North-star architecture

Given the constraints—single operator, homelab/laptop runtime, demo-stage maturity—the current architecture is artificially inflated by the system PostgreSQL dependency and the Python/Go split. 

If building greenfield under these exact constraints: I would have built the entire system as a **single statically-compiled binary** (Go or Rust). The state store would be an embedded database (SQLite or DuckDB). An embedded DB natively supports the "local-first, zero-config" mandate. The single binary would embed the daemon, the CLI, and the supervisor logic, eliminating the need for Unix socket IPC between different language runtimes and sidestepping the wheel packaging and cross-compilation dance entirely. No system DB, no Python virtual environments, just a downloaded binary that initializes its own state and runs.

## 7. Recommended changes

| Priority | Change | Rationale | Benefit | Risk | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | **Revert to Embedded Storage (SQLite/DuckDB)** | System PostgreSQL introduces massive adoption friction for a laptop tool. Embedded DBs support robust concurrent writers (via WAL) without a background server. | Drastically simplifies day-zero installation and runtime orchestration. | Re-writing the migrations and RPC wrappers back from PG. | 2 weeks |
| 2 | **Consolidate PTY Helper into Python** | The Go helper introduces an entire second build pipeline for a single developer. Python's native `pty` and `subprocess` can handle the supervision needs. | Unifies the build pipeline; simplifies debugging. | Slight performance/concurrency hit in PTY streaming. | 1 week |
| 3 | **Fracture Monolithic Tests** | `test_cli_mvp.py` is an unnavigable behemoth. Break it down by domain aggregate (`test_runs.py`, `test_sessions.py`, `test_artifacts.py`). | Vastly improved maintainer velocity and error locality. | Git history loss on those specific test lines. | 3 days |

## 8. Functionality I'd add

| Priority | Change | Rationale | Benefit | Risk | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | **Event Log Rotation/Archival** | The `events` table is append-only. On a long-lived project, this will bloat the local DB. | Prevents disk space exhaustion on local machines. | Deleting chained hashes requires a root-hash rollover mechanism. | 1 week |
| 2 | **Single-File Executable Packaging** | If staying on Python, package the orchestrator using PyInstaller or PyOxidizer. | Lowers adoption friction, mirroring the UX of tools like Claude Code or Gemini CLI. | Bundled wheels can be brittle across OS variants. | 4 days |

## 9. Execution roadmap

- **Startable today:** Break up `test_cli_mvp.py` and other monolithic test files to improve immediate developer ergonomics.
- **Near-term (month):** Implement Python-native PTY supervision to deprecate and delete the Go helper directory, shrinking the build matrix.
- **Medium-term (quarter):** Begin the architectural reversion back to SQLite. The daemon can remain, but it should manage an embedded SQLite file rather than connecting to a system Postgres instance.
- **Long-term:** Package the entire Python environment into a self-contained executable to finalize the zero-dependency local-first vision.

## 10. Open questions

- **Why Postgres?** The decision log (D094) references abandoning SQLite for Postgres, but the operational trade-offs for a strictly local, non-distributed tool are staggering. Was SQLite locking genuinely a bottleneck for single-operator parallel agents?
- **Web UI Utilization:** The project ships a Jinja2/JS local web dashboard. Is the human principal actually using this, or do operators primarily stay in the terminal/MCP tools?
- **Event Log Growth:** Without log rotation, what is the expected disk footprint of the `events` table for a moderately active repository over a year?