# STRIATUM ARCHITECTURE REVIEW

**Date:** 2026-05-18
**Reviewer:** Gemini CLI
**Project:** striatum-orchestrator (Striatum)

---

Staleness note, 2026-05-18: this review snapshot includes the retired Python
daemon module before its later deletion. `src/striatum/daemon.py` references
below are historical evidence; Python `daemon_pg` client/admin code remains
transitional cleanup rather than a production daemon alternative.

## 0. Files reviewed

- `README.md`
- `pyproject.toml`
- `docs/SPEC.md`
- `docs/DDD.md`
- `docs/PRD.md`
- `docs/rfcs/0068-go-production-daemon-port.md`
- `docs/rfcs/0069-pg-only-daemon-global-surfaces.md`
- `src/striatum/cli/dispatch.py`
- `src/striatum/daemon.py` (historical snapshot; later deleted)

## 1. Executive summary

- **Audience & Adoption:** Striatum is built for public adoption; its first external target is a team adopting the tool. It expects a heavy orchestration topology: one human principal piloting 8+ concurrent AI operators across 3+ repositories.
- **Enforced Go/PG Architecture:** The project is cutting over to a Go-based production daemon. PostgreSQL is the authoritative state, driven by the absolute necessity of handling concurrent appender contention and providing strict audit-chain row-lock semantics for the 8+ AI operators.
- **Legacy Purge:** The retired Python daemon module is deleted. Legacy SQLite fixtures and Python `daemon_pg` client/admin cleanup remain transitional debt; the definition of "done" relies heavily on deleting that debt and ensuring a clean PyPI install story across macOS and Linux.
- **Escalation & UX:** The Web UI is strictly reserved for the human principal for escalation purposes. AI operators do not use the Web UI; they rely on CLI and MCP.

## 2. What the project is trying to be

**Stated goals:** Striatum is a local-first workflow runner and coordinator tailored for a heavy operator topology (1 human : 8+ AI operators). It is built for a team adopting the tool, explicitly avoiding being a telemetry sink or a hosted service.
**Operating model:** A local Go background daemon (`striatumd`) owns the live state in PostgreSQL, ensuring robust concurrency. Python serves as the CLI, Web UI (human escalation), and client API.
**Documentation:** Docs are ruthlessly targeted: AI operators (primary), future-maintainer cold-start (secondary), and provenance. They are explicitly not written to onboard external open-source contributors. RFCs serve specifically as forward-looking design proposals.

## 3. Current architecture

- **Components:** A Python 3.11+ core orchestrator (CLI, Web UI) and a Go-based production daemon (`striatumd`). Python `daemon_pg` direct-state/admin code is transitional cleanup, not a production daemon alternative.
- **State/Storage:** Authoritative state lives in a system-installed PostgreSQL database. This is not an architectural smell—it is the correct, load-bearing solution to support high-concurrency writes from 8+ simultaneous AI agents without locking contention.
- **Surfaces:**
  - CLI (Workflow authoring via `striatum workflow generate`, not React Flow or hand-edited JSON).
  - MCP wrapper (for AI operator interactions).
  - Web Dashboard (strictly for human-principal escalation).
- **Capability Tokens:** Tokens are actively used to differentiate scopes per operator in practice, preventing privilege escalation.
- **CI Posture:** CI health is currently uncertain; the build matrix is not confidently known-green on the `main` branch.

## 4. Strengths

- **Concurrency-Ready Storage:** Embracing PostgreSQL solves the concurrent appender contention and row-lock semantics required by an 8+ AI operator topology, which embedded databases like SQLite simply cannot handle cleanly.
- **Capability Gating:** Differentiated capability tokens per operator enforce strict access controls and are actively exercised, preventing horizontal privilege escalation between agents.
- **Fail-Closed Dispatch:** `src/striatum/cli/dispatch.py` explicitly catches route failures and exits rather than silently falling back to legacy paths.
- **Domain-Driven Guardrails:** The rigorous DDD vocabulary dictates the workflow state, ensuring AI agents cannot bypass lifecycle constraints.

## 5. Concerns

- **Substrate-Migration Drag (Blocker):** The top day-to-day friction for the maintainer is the lingering drag from the substrate migration. The legacy SQLite code and the deprecated Python `daemon_pg` must be deleted completely to unblock further feature velocity.
- **CI Health (Serious):** The lack of confidence in the CI matrix on `main` is a significant risk for public adoption. A clean, verified installation story (fresh-clone → pip install → adopt → workflow runs) on macOS and Linux is required for the "done" state, and CI must prove this.

## 6. North-star architecture

The target architecture is exactly where the project is heading: a robust Go daemon coordinating state in PostgreSQL to handle high-concurrency AI traffic, with a thin Python CLI and Web UI for the human operator.

The immediate architectural priority is achieving the definition of "done" for the current phase:
1. The complete deletion of legacy SQLite.
2. The complete deletion of Python `daemon_pg`.
3. The collapse of test fixtures.
4. A flawlessly clean PyPI installation story.

## 7. Recommended changes

| Priority | Change | Rationale | Benefit | Risk | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | **Delete Legacy Python Daemon and SQLite** | The substrate-migration drag is the top source of daily friction. Delete `daemon_pg`, the SQLite backend, and collapse all associated test fixtures. | Achieves the definition of "done" for the migration and unblocks the maintainer. | Dropping historical tests if they aren't ported correctly. | 1.5 weeks |
| 2 | **Fix and Enforce CI Matrix** | Ensure the build matrix is known-green on `main`. Assert the exact "fresh-clone → pip install → adopt" flow works on macOS and Linux. | Guarantees the clean PyPI install story necessary for the first external team adoption. | Uncovering hidden platform bugs. | 1 week |

## 8. Functionality I'd add

| Priority | Change | Rationale | Benefit | Risk | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | **Human-Principal Escalation UX** | With 8+ agents running, the human will be constantly triaging blockers. The Web UI needs optimized bulk-decision and escalation workflows. | Directly addresses the human bottleneck in the 1:8+ operator topology. | Scope creep in the UI layer. | 2 weeks |
| 2 | **Single-Command Doctor/Smoke** | Combine diagnostic checks into a single command to verify the Postgres connection, daemon status, and capability scopes. | Reduces operator friction during the critical adoption phase for the external team. | None. | 2 days |

## 9. Execution roadmap

- **Startable today:** Mercilessly delete the remaining `daemon_pg` Python code and SQLite legacy implementations to relieve the substrate-migration drag.
- **Near-term (month):** Stabilize CI. Assert the clean PyPI installation and runtime story across macOS and Linux to prepare for the adopting team.
- **Medium-term (quarter):** Pivot focus entirely to the human-principal escalation UX in the Web UI to ensure one human can comfortably manage the 8+ AI operators.

## 10. Open questions

- **Tombstone Lifecycle:** Once legacy data is fully migrated to Postgres and `state.sqlite3.tombstone` files are left behind, what is the product decision on their lifecycle? Should the daemon eventually auto-delete them?
- **Workflow Generation Boundaries:** `striatum workflow generate` is the accepted authoring path. How complex are these generated workflows expected to get before a human needs to drop down to manual intervention?
