# ARCHITECTURE REVIEW: STRIATUM

**Date:** 2026-05-20
**Reviewer:** Antigravity (Senior Systems Architect)
**Project:** Striatum
**Status:** Senior Peer Review

---

## 0. Files Reviewed

- `README.md`
- `AGENTS.md`
- `pyproject.toml`
- `Makefile`
- `docs/INDEX.md`
- `docs/SPEC.md`
- `docs/DECISION_LOG.md`
- `docs/UBIQUITOUS_LANGUAGE.md`
- `docs/TODO.md`
- `docs/operator/BRIEF.md`
- `docs/DDD.md`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- `src/striatum/__init__.py`
- `src/striatum/cli/__init__.py`
- `src/striatum/cli/daemon.py`
- `src/striatum/cli/dispatch.py`
- `src/striatum/cli/parser.py`
- `src/striatum/daemon_entrypoint.py`
- `src/striatum/daemon_pg/` (directory structure)
- `src/striatum/daemon_rpc/` (directory structure)
- `src/striatum/artifact_contracts.py`
- `go/pkg/db/audit.go`
- `go/pkg/db/connection.go`
- `go/cmd/striatumd/main.go`

---

## 1. Executive Summary

- **Local-First, Audit-Ready:** Striatum successfully builds a robust, local-first orchestration layer for AI agents that prioritizes provenance and auditability over ease of implementation.
- **Architectural Pivot:** The project is in the middle of a major (and correct) architectural shift from repo-local SQLite to a centralized (but still local) Go-based daemon with PostgreSQL.
- **Strong Boundary Discipline:** The strict "no model SDK" rule in the runner core is a critical design win, ensuring Striatum remains a coordinator, not a client library.
- **Audit Chain Integrity:** The implementation of hash-chained audit logs in PostgreSQL using `FOR UPDATE` row-locks demonstrates high-maturity systems engineering for a "demo-stage" project.
- **Governance via Artifacts:** Using Markdown artifacts as the "durable provenance" while keeping live state in the daemon is a clean separation of concerns.
- **Complexity Debt:** The legacy SQLite code (now quarantined) and the dual Python/Go nature of the current stack create a high "cognitive load" for a single maintainer.
- **Tooling First:** The project invests heavily in "doctor" and "summary" commands, which is essential for a system that orchestrates non-deterministic agents.
- **Recommendation:** Accelerate the deletion of Python-daemon and SQLite legacy code. The "cut-over" is the primary risk to momentum.

---

## 2. What the project is trying to be

### Goals and Principles
Striatum is a **coordinator** for AI coding agents. Its primary goal is to eliminate "reviewer co-blindness" and provide "audit-quality provenance" for agentic workflows.

As stated in [SPEC.md](file:///home/halbritt/git/striatum/docs/SPEC.md):
> `striatum` is a standalone, local-first workflow runner for terminal-based AI coding agents. It coordinates registered target repositories through a local daemon.

The stated principles include:
1. **Local-First:** No hosted services, no telemetry, no external persistence.
2. **Provider Portability:** No vendor SDKs inside the runner.
3. **Auditability:** Every state transition is hash-chained and logged.
4. **Lane Attestation:** Verification of who (which process) authored an artifact.

### Domain Model
The domain model is built on [DDD.md](file:///home/halbritt/git/striatum/docs/DDD.md) principles:
- **Run:** A single execution of a workflow.
- **Session:** An agent's interactive window into a run.
- **Job:** A discrete unit of work (implement, review, etc.).
- **Lease:** A time-bound claim on a job by a session.
- **Artifact:** A durable output (Markdown, finding, decision).
- **Verdict:** The outcome of a review job (`accept`, `needs_revision`, etc.).

### Operating Model
Striatum assumes two roles ([README.md](file:///home/halbritt/git/striatum/README.md)):
1. **AI Operator:** The default driver that advances the workflow.
2. **Human Principal:** Escalation-only role for resolving blockers or risky decisions.

### Mutually Incompatible Goals
- **Maximized Portability vs. Strict Attestation:** Striatum wants to support any terminal agent (portability), but it also wants to prove the agent process was alive when the work was done (attestation). This creates a friction point where "wrapper" scripts are needed, increasing the barrier to entry for new agents.

---

## 3. Current Architecture

### Components
1. **striatum CLI (Python):** The primary user/agent interface. It acts as a client to the daemon.
2. **striatumd (Go):** The authoritative daemon. It owns the PostgreSQL state and RPC methods.
3. **PostgreSQL:** The single source of truth for live state (runs, sessions, leases, audit logs).
4. **Target Repository:** The "Durable Provenance" store. Artifacts (`.md`) are written here.
5. **.striatum/ (Scratch):** Temporary FIFOs, pidfiles, and supervisor pipes. Never live state.

### Runtime and State
**Stated:** "The authoritative live state is the daemon-owned PostgreSQL instance" ([SPEC.md:29](file:///home/halbritt/git/striatum/docs/SPEC.md#L29)).
**Actual:** The code in `src/striatum/cli/daemon.py` and `src/striatum/cli/dispatch.py` confirms that mutations are routed through the daemon RPC.

**Stated:** "The Go daemon is the production/default daemon" ([operator/BRIEF.md:18](file:///home/halbritt/git/striatum/docs/operator/BRIEF.md#L18)).
**Actual:** `Makefile` defaults `CORE=go` and `striatum daemon start` invokes the Go binary.

### Surfaces
- **CLI:** Complete verb set for claim/publish/review.
- **MCP:** A JSON-RPC bridge for LLM tools.
- **Web UI:** A server-rendered Jinja2 UI (with some React islands) for visual monitoring.

### Test Posture
**Actual:** High density. `pyproject.toml` and `Makefile` show integration tests, multi-repo harnesses, and architecture guardrails. `tests/architecture/test_authority_guardrails.py` is particularly impressive, ensuring no production path touches SQLite.

### Release Posture
**Actual:** Version `1.57.0`. Semantic versioning is strictly followed as per `DECISION_LOG.md`. The project uses GitHub Actions for CI and OIDC-based PyPI publishing.

---

## 4. Strengths

1. **Hash-Chained Audit Log:** The decision to use a serialized hash chain in PostgreSQL ([go/pkg/db/audit.go:153](file:///home/halbritt/git/striatum/go/pkg/db/audit.go#L153)) is a premier systems decision. It moves the project from "script" to "auditable infrastructure."
2. **Role-Based Workflow Separation:** The ability to assign `codex` to implement and `claude` to review ([README.md:197](file:///home/halbritt/git/striatum/README.md#L197)) is the project's unique value proposition. Most agentic frameworks ignore the "co-blindness" problem.
3. **Markdown as Data:** By enforcing schemas on Markdown front matter ([src/striatum/artifact_contracts.py:170](file:///home/halbritt/git/striatum/src/striatum/artifact_contracts.py#L170)), Striatum makes the repo itself queryable without requiring a specialized database for historical provenance.
4. **Boundary Guarding:** The `tests/test_cli_corpus_export.py` ([SPEC.md:725](file:///home/halbritt/git/striatum/docs/SPEC.md#L725)) asserting no accidental imports of external memory systems (Engram) shows a disciplined approach to dependency management.
5. **Authority Mapping:** The `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` is a rare but highly valuable artifact. It maps every CLI command to its daemon RPC method and records its "Go-port" status. This ensures the maintainer has a clear view of the "authority transition" and prevents "ghost" commands that bypass the daemon.

---

## 5. Concerns

### [Rank: Blocker] Dual-Core Maintenance Debt
**Actual:** The project maintains a Python CLI and a Go daemon. While the Python daemon is "retired," much of the business logic (workflow validation, artifact contracts) is still in Python.
**Evidence:** `src/striatum/workflow.py` is over 100,000 bytes. This logic is not yet in the Go core.
**Risk:** Drifts in validation rules between the CLI (Python) and the Daemon (Go) are inevitable until the logic is consolidated or shared.

### [Rank: Serious] Manual Escalation UI
**Actual:** Escalations are handled via "escalation artifacts" ([SPEC.md:665](file:///home/halbritt/git/striatum/docs/SPEC.md#L665)), but the human principal's "inbox" is still largely a collection of files or a "dashboard" frame.
**Evidence:** `docs/HOW_TO_HUMAN.md` still refers to manual CLI verbs for resolving checkpoints.
**Smell:** For a tool that aims to save time, the "human in the loop" experience feels like an afterthought compared to the agentic loop.

### [Rank: Smell] Non-Native PTY Handling in Python
**Actual:** The PTY supervision logic in `src/striatum/daemon_supervisor/` is complex and prone to edge cases in Python's subprocess model.
**Evidence:** `DECISION_LOG.md:D106` shelves interactive Claude lanes partly due to "PTY/MCP stability."
**Mine:** PTY management is notoriously brittle in Python. Moving the *entire* supervisor loop to the Go daemon (which excels at this) is the only sane path.

---

## 6. North-Star Architecture

If built greenfield today with the same constraints:
1. **Single-Binary Go Core:** One `striatum` binary (Go) that acts as both daemon and CLI. No Python dependency in the critical path.
2. **Embedded SQLite with WAL Mode (Revisited):** While the move to PostgreSQL was driven by multi-repo coordination, a local-first tool is a hard sell if the user has to manage a system Postgres. SQLite with a proper WAL-based daemon would satisfy 99% of the single-operator requirements with zero setup friction.
3. **WASM-based Plugin System:** Instead of "wrapper scripts" for agents, use WASM to implement "skills" that can safely execute within the runner's sandbox.
4. **React-only Frontend:** Ditch the Jinja2/React hybrid. A clean, modern SPA served by the Go binary (using `embed`) would be more maintainable.

---

## 7. Recommended Changes

| Priority | Change | Rationale | Benefit | Risk | Effort |
|---|---|---|---|---|---|
| **High** | Consolidate Workflow Logic in Go | Currently duplicated or Python-only. | Eliminates drift; enables full Go-only operation. | Logic-heavy; high regression risk. | 2-3 Weeks |
| **High** | Finalize Python Daemon Deletion | The "quarantine" still exists. | Reduces cognitive load and surface area. | Breaking legacy fixtures. | 1 Week |
| **Medium** | "Human Inbox" Web Surface | Resolving escalations in the CLI is high friction. | Makes the "Human Principal" role actually usable. | UI complexity. | 1 Week |
| **Medium** | Automated Postgres Provisioning | `daemon doctor` is good, but `daemon start` should "just work." | Lowers the barrier to entry significantly. | OS-level permissions. | 3 Days |

---

## 8. Functionality I'd Add

| Priority | Change | Rationale | Benefit | Risk | Effort |
|---|---|---|---|---|---|
| **Medium** | Replay-from-Corpus | The `corpus export` exists, but there's no `corpus import` to reconstruct a run. | Disaster recovery and cross-machine work sharing. | Non-deterministic PTY output. | 1 Week |
| **Low** | Agent Capability Discovery | The runner knows which lanes exist, but doesn't know their "skills." | Dynamic workflow routing (e.g., "find me a lane that can do Go"). | Schema complexity. | 1 Week |

---

## 9. Execution Roadmap

1. **Phase 1 (Today):** Delete remaining legacy SQLite compatibility modules and migration/in-memory fixtures.
2. **Phase 2 (Month 1):** Port `striatum.workflow.v1` validation logic to `go/pkg/workflow`. This is the biggest hurdle.
3. **Phase 3 (Quarter 1):** Implement the "Human Principal" inbox in the Vite-based frontend.
4. **Phase 4 (Long-term):** Move to a single Go binary and deprecate the Python requirement for the CLI.

---

## 10. Open Questions

1. **Binary Distribution:** How are the Go binaries packaged for non-Go developers? (Found the answer in `pyproject.toml:57` - `_daemongo/binaries/`, but is this automated in CI for all platforms?).
2. **Sealed Patch Mode:** `SPEC.md:429` mentions it's reserved. What is the actual technical plan for "hard containment" on a local developer machine?
3. **Multi-Repo Collision:** If two repositories use the same daemon, how is "identity" shared beyond `repository_id`? (e.g., shared agent secrets).

---

**Summary of actual state:** The project is in a high-quality transition state. The foundation (Postgres + Audit Chain) is solid, but the logic remains split across two languages, which is a significant tax on a single maintainer.
