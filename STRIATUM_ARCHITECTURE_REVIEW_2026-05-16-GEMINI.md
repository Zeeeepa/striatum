# Striatum Architecture Assessment Report
**Date:** 2026-05-16
**Target:** `~/git/striatum/`
**Reviewer:** Expert Systems Architect

---

## 1. What the project is trying to be

### Product Goals
Striatum aims to be a local-first, vendor-agnostic workflow orchestrator for terminal-based AI coding agents. It provides a deterministic control plane that coordinates loops of drafting, reviewing, repairing, and synthesizing code without relying on a hosted service. 

### Core Principles
- **Multi-Lane Reviews:** Splitting implementation and review responsibilities across different models (e.g., Codex implementing, Claude reviewing) to prevent "co-blindness," where a model accepts its own logical flaws.
- **Audit-Quality Provenance:** Every action, decision, and system event is durably recorded with a SHA-256 hash-chain (similar to a local ledger) ensuring tamper-evident history.
- **Provider Portability:** The runner manages the state and control plane; the model adapters (via MCP or process wrappers) manage the generation. The runner has no direct dependencies on model vendor SDKs.
- **Local-First & Private:** All telemetry, transcripts, and event states live either in the target repository (as durable artifacts) or in a local, daemon-owned PostgreSQL database. 

### Domain Model
The core domain heavily leverages Domain-Driven Design (DDD) principles:
- **Workflows & Jobs:** Directed Acyclic Graphs (DAGs) representing phases of work.
- **Lanes & Sessions:** Specific agent roles bound to specific model capabilities.
- **Artifacts:** Strictly schema-validated markdown files (e.g., `decision`, `finding`, `synthesis`) containing front-matter schemas.
- **Verdicts & Blockers:** Formal state-transition markers that require either AI resolution or human escalation.

### Intended Operating Model
Striatum operates on a clear dual-role paradigm (RFC 0053):
1. **AI Operator:** The default driver that claims work, publishes artifacts, and advances state.
2. **Human Principal:** Acts strictly as an escalation point to resolve stuck states, approve high-risk operations, or unblock routing stalemates.

---

## 2. Current Architecture

### Major Components
- **CLI (`striatum`):** A Python-based command-line interface that agents and humans use to interact with the system.
- **Daemon (`striatumd`):** A background service acting as the central authority for state mutations. It is currently undergoing a substrate transition with logic split between Python (`daemon_pg/`) and Go (`go/pkg/`).
- **PostgreSQL Database:** The authoritative live state layer per repository (formerly SQLite).
- **Process Supervisor:** A subsystem that wraps agent CLIs (like Claude Code or Gemini CLI), manages their lifecycles via FIFOs and pidfiles, and feeds them context.
- **Web UI / Dashboard:** A local dashboard for human introspection and workflow management.

### Runtime/Control-Plane Architecture
The architecture is structured as a closed-loop control plane. Agents operate inside a `process_adapter` sandbox. They cannot write to the system state directly; instead, they execute `striatum` CLI verbs. The CLI translates these verbs into Unix socket RPC calls to `striatumd`, which then validates the request, records it in the PostgreSQL event log with a locked `FOR UPDATE` audit chain, and mutates the state.

### State/Storage Model
- **Live State:** PostgreSQL manages active runs, leases, sessions, and the event ledger.
- **Durable State:** The target repository itself stores generated artifacts (e.g., findings, handoffs) and workflow configuration (`workflow.json`). `.striatum/` is used as ephemeral scratch space (FIFOs, pidfiles).

### Boundaries
- **CLI to Daemon:** Unix socket RPC using custom JSON payloads (transitioning to a formalized RPC matrix).
- **Daemon to DB:** Direct PostgreSQL queries.
- **Agent to Repo:** Scoped file-system access managed by the orchestrator's constraints.

### Test and Release Posture
The project maintains a rigorous, high-coverage testing posture. It features over 1,200 tests combining unit checks, end-to-end multi-repo test harnesses (using ephemeral PostgreSQL instances), and capability-denial matrices. Releases are automated via GitHub Actions spanning Python and Go matrices.

---

## 3. What is strong

### Good Architectural Decisions
- **Moving to PostgreSQL:** Deprecating the file-based SQLite database in favor of a daemon-managed PostgreSQL instance eliminates database lock contention and race conditions during high-concurrency multi-lane executions.
- **SHA-256 Audit Chaining:** Enforcing a cryptographic ledger of events guarantees the integrity of AI actions, which is critical for trust in autonomous orchestration.
- **Strict Schema Enforcement:** Forcing agents to emit artifacts with strict `v1` front-matter schemas ensures that the control plane can deterministically parse AI output without relying on flaky LLM text-extraction.

### Design Principles Worth Preserving
- **The "Co-Blindness" Defense:** The explicit architectural decision to force cross-model reviews is brilliant and practically solves a major limitation of current LLMs.
- **Human as Escalation Only:** By formalizing the AI as the operator and the human as the principal, the architecture successfully shifts the burden of orchestration away from the developer.

---

## 4. Architectural Concerns

### Current Risks & Complexity Hotspots
- **Split Brain Daemon Substrate:** The daemon's logic is currently bifurcated between Python (`daemon_pg/`) and Go (`go/pkg/`). This transition phase causes severe authority ambiguity. A command might be handled by Go, fallback to Python PG handlers, or worse, fallback to legacy SQLite pathways.
- **Process Supervision Fragility:** The current supervisor relies on standard Unix FIFOs and piped stdin/stdout. Many interactive terminal tools refuse to operate without a full pseudo-terminal (PTY), causing stalls and requiring frequent "operator-on-behalf" manual overrides. 

### Coupling, Duplication, and Migration Debt
- **SQLite Transition Debt:** The system still retains legacy SQLite fallback code paths. While `PRAGMA user_version` tombstoning exists, the sheer presence of the fallback logic weakens the absolute authority of the Postgres daemon.
- **Web Service Direct DB Access:** The local web dashboard directly queries the database instead of routing all requests exclusively through the daemon's RPC layer, breaking the single-source-of-truth paradigm.

### Places Where Docs and Implementation Disagree
- Several pieces of documentation (e.g., parts of `README.md` and `GETTING_STARTED.md`) still reference `.striatum/state.sqlite3` as the authoritative state, which directly contradicts the D094/RFC 0043 implementation that enforces PostgreSQL. (Tracked as GH #15).

---

## 5. What you would do differently greenfield

### Preferred Architecture & Technology Choices
- **Unified Go Daemon:** Greenfield, the `striatumd` daemon should be written entirely in Go from day one. Go's native concurrency model, lightweight goroutines, and zero-dependency binary distribution make it vastly superior to Python for a local orchestrator daemon. Python would be retained exclusively for the lightweight CLI client.
- **gRPC / Protobuf:** Instead of bespoke JSON-RPC over Unix sockets, I would define the contract using Protobuf and communicate via gRPC. This provides free client generation, strong typing, and eliminates the "command authority matrix" ambiguity currently plaguing the project.
- **Strict PTY Supervision:** Greenfield process adapters would use native PTY allocation (e.g., via Go's `creack/pty`) rather than FIFOs, ensuring flawless interaction with complex terminal applications.

---

## 6. Recommended changes to the current project

**Ordered by priority:**

1. **Phase 1: Eradicate SQLite Fallbacks (High Priority)**
   - **Rationale:** The presence of fallback code masks daemon failures and creates state-split risks.
   - **Action:** Delete the production SQLite fallback pathways for all core workflow verbs. Force a hard failure if the daemon RPC is unreachable.
   - **Difficulty:** Low to Medium (mostly deletion and test updates).

2. **Phase 2 & 3: Resolve Daemon Core Strategy (High Priority)**
   - **Rationale:** Maintaining business logic in both Python and Go is unsustainable. 
   - **Action:** Make a definitive architectural decision (RFC/Decision Log). If Go is the future, halt Python `daemon_pg` feature development and port the remaining handlers. Generate a single source-of-truth RPC contract file.
   - **Difficulty:** High.

3. **Enforce Reviewer Diversity (Medium Priority)**
   - **Rationale:** Dogfooding has repeatedly shown that pairing the same model (e.g., Codex implementing and Codex reviewing) leads to cycle-exhaustion due to shared blind spots.
   - **Action:** Add a hard workflow validation rule refusing identical model pairings for `implement` and `review` lanes unless an `--allow-same-model-pairing` flag is explicitly provided.
   - **Difficulty:** Low.

4. **Phase 6: PTY Process Supervision (Medium Priority)**
   - **Rationale:** Agent stalling due to non-interactive terminal restrictions requires human intervention, breaking autonomy.
   - **Action:** Upgrade the process supervisor to allocate true PTYs. 
   - **Difficulty:** Medium.

---

## 7. Functionality you would add

### Product Capabilities
- **Auto-Finalize from Front Matter (RFC 0051):** Allow the system to automatically complete a job if an agent crashes but successfully writes a valid schema-compliant artifact to disk. This provides immense resilience against brittle LLM CLI wrappers.

### Operator/Developer Experience
- **Real Escalation Inbox:** A dedicated terminal and web view for the Human Principal that aggregates all `waiting_human` states, blockers, and required decisions across all local repositories into a single queue.

### Observability & Workflow
- **Optional Git/PR Integration:** Once an artifact/code change is synthesized and approved by the workflow, provide an optional, plugin-based capability to automatically stage, commit, and push to a remote branch or open a Pull Request.

---

## 8. Suggested execution roadmap

### Clear First Step (Immediate)
- **Architecture Remediation Phase 1:** Hard-close the legacy SQLite fallback routes. Update all remaining documentation to reflect PostgreSQL as the sole authoritative state substrate (resolving GH #15).

### Near-Term (Next 2-4 Weeks)
- **Unify the Daemon Contract:** Complete Phase 2 and 3 of the remediation plan. Adopt a single API contract definition to bind the CLI, Python handlers, and Go handlers, eliminating authority drift.
- **Validator Risk Linting:** Implement the same-model reviewer refusal logic to permanently cure the "Codex/Codex co-blindness" loop.

### Medium-Term (1-3 Months)
- **Robust Supervision & Resilience:** Ship RFC 0051 (Auto-finalize from frontmatter) and upgrade the process adapter to use native PTYs.
- **Daemon-First Web Service:** Rearchitect the web dashboard to communicate exclusively via the daemon RPC, removing its direct database connections.

### Long-Term (3-6 Months)
- **Corpus V2 Integration:** Finalize the Engram memory integration (RFC 0052/0057) to allow AI agents to draw on historical run provenance for long-term project context.
- **Human Principal Inbox:** Build the cross-repo escalation inbox to polish the dual-role operating model. 
