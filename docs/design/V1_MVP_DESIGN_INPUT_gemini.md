# V1 MVP Design Input: Gemini

## 1. Lane Verdict

The architectural foundation of `agent_runner`—a deterministic local control plane over SQLite, decoupled from AI logic—is conceptually sound and aligns well with local-first, portable requirements. However, the scope specified for the V1 MVP (leases, multi-agent queues, bounded cycles, heartbeats) is overly ambitious and highly susceptible to distributed state bugs disguised as local CLI complexity.

I recommend reducing the V1 MVP boundary to strictly sequential job execution with explicit manual recovery, dropping complex autonomous queuing mechanics for now. With that scope reduction, I approve the design direction and believe it is safe to proceed to synthesis.

## 2. MVP Boundary

The P001 specification for the MVP currently encompasses session registration, lease management, structured messaging, append-only events, DAG workflows with cycles, and artifact publishing.

**Critique:** Building a resilient, concurrent message queue on top of SQLite accessed via transient CLI commands is difficult. Introducing leases, heartbeats, and automated retry cycles in V1 will pull focus away from the core value proposition: ensuring reliable, auditable agent execution and artifact generation.

**Recommendation:** Narrow the MVP boundary.
- **In Scope:** `init`, `register-session`, `claim-next` (synchronous), `complete` (with verdict/blocker payloads), `publish-artifact`, sequential JSON workflows, append-only event logging.
- **Out of Scope for V1:** Background lease monitors, heartbeat mechanics, automated retry cycles (if a job fails, the run stops), parallel execution logic, and complex workflow DAG resolution (stick to sequential or simple independent DAGs).

## 3. Adversarial Risks and Mitigations

- **Context Contamination (High Risk):** Persistent sessions are dangerous for multi-step tasks. An agent might hallucinate a solution based on a previous, rejected iteration still residing in its CLI history.
  - *Mitigation:* Synthesis and Build jobs must default to `fresh_session_required: true`. Persistent sessions should be strictly reserved for the AI Coordinator (where context accumulation is the feature) or explicit multi-pass review loops where prior context is intentionally carried over.
- **Write-Scope Safety (High Risk):** Same-branch parallel writes risk file corruption and merge conflicts.
  - *Mitigation:* V1 must enforce sequential execution. If parallel jobs are eventually allowed in V1, they must be mathematically proven to have disjoint write scopes or be strictly read-only (e.g., parallel independent reviews).
- **Privacy and Local-First Defaults (Medium Risk):** The SQLite database (`.agent_runner/`) could be accidentally ingested into context windows or committed to remote repositories.
  - *Mitigation:* `agent_runner init` must aggressively append `.agent_runner/` to `.gitignore`, `.geminiignore`, and `.aiderignore`.
- **Hidden Provider Assumptions (Medium Risk):** Assuming agents can parse complex CLI output or maintain interactive PTY states reliably.
  - *Mitigation:* The `agent_runner` binary must support a purely non-interactive, headless mode driven by JSON stdout.

## 4. SQLite Schema and Queue Semantics Critique

**Critique:** Complex queue semantics (leases, dead-letter queues, timeouts) require background processing or complex transactional logic upon every read, which is brittle when the only interface is independent CLI executions.

**Recommendation:**
- Simplify job states to: `pending`, `claimed`, `completed`, `failed`.
- `claim-next` must be atomic and synchronous. When an agent calls `claim-next`, the state immediately transitions to `claimed` using the `session_id`.
- Omit automatic lease expiration in V1. If an agent crashes while a job is `claimed`, human intervention (e.g., `agent_runner release <job_id>`) is required to revert it to `pending`.

## 5. CLI and Adapter Boundary Critique

**Critique:** The proposed mutation command set (`claim-next`, `ack`, `send`, `block`, `complete`, `verdict`, `publish-artifact`) is too granular and risks leaving the database in inconsistent states if an agent fails halfway through a sequence.

**Recommendation:**
- Consolidate commands. `ack` is redundant if `claim-next` synchronously marks the job as claimed.
- Combine `block` and `verdict` into the payload of a `complete` command. An agent completes its lease by submitting an outcome: `success`, `blocked`, or `needs_revision`.
- **MVP Command Set:** `init`, `register-session`, `claim-next`, `complete`, `publish-artifact`, `status`.

## 6. Work Packet and JSON Workflow Config Shape

**Critique:** YAML parsing ambiguity is correctly avoided. The JSON schema must remain flat and explicit to prevent agents from misinterpreting nested dependencies.

**Shape Recommendation:**
- **Workflow Config:** A strict DAG representation. Nodes represent jobs, containing `role`, `context_docs`, `prompt`, `write_scope`, and `expected_artifacts`. Edges represent `depends_on`.
- **Work Packet Shape:** The payload returned by `claim-next` must be a self-contained envelope.
```json
{
  "job_id": "job_123",
  "run_id": "run_456",
  "role": "reviewer",
  "context_docs": ["docs/SPEC.md"],
  "task_prompt": "Review the spec for safety...",
  "write_scope": ["docs/reviews/"],
  "stop_conditions": ["If architecture principles are violated, complete with verdict 'reject'"]
}
```

## 7. Session Lifecycle and Context Contamination Controls

As noted in the adversarial risks, persistent sessions are the enemy of idempotency. V1 should adopt a pessimistic stance on memory.

**Policy:**
- The deterministic coordinator tracks identity via `session_id`.
- Workflows must explicitly declare `fresh_session_required: false` to allow state carryover. By default, agents should expect to be restarted or instructed to clear their context between jobs to ensure outputs are derived solely from durable repo artifacts and the task envelope.

## 8. Artifact Publishing, Transcript Policy, and Branch Confirmation

- **Transcripts:** I strongly endorse the decision *not* to capture broad shell transcripts. They cause database bloat and introduce massive privacy risks. Only structured payloads sent via `complete` or explicit `publish-artifact` commands should be recorded.
- **Artifacts:** `publish-artifact` should register a file path in the SQLite event log and ensure it exists within the allowed `write_scope`.
- **Branch Confirmation:** If `agent_runner` attempts to modify git state, it must fail cleanly in headless environments unless passed a `--confirm-branch` flag.

## 9. RFC-Ledger Validation Fixture

The RFC-ledger workflow is an excellent V1 fixture because it can be modeled as a strict sequence, avoiding the need for cyclic DAG support in the MVP.

**Workflow Shape:**
1. `draft_rfc` (Job: fresh session, writes `RFC.md`)
2. `review_rfc` (Job: fresh session, reads `RFC.md`, writes `REVIEW.md`)
3. `synthesize_rfc` (Job: fresh session, reads `RFC.md` and `REVIEW.md`, updates `RFC.md`)

This proves role definition, artifact passing, and sequential queue claiming without needing parallel execution or automatic retry loops.

## 10. Test Strategy

- **Avoid LLM dependencies in CI:** The test suite must be entirely local and deterministic. Do not hit real model APIs.
- **Integration Harness:** Use a bash script that mimics an agent by calling `agent_runner register-session`, `agent_runner claim-next`, and `agent_runner complete` to validate state transitions.
- **State Validation:** Tests must assert that SQLite invariants hold (e.g., an agent cannot claim a job already claimed by another `session_id`).

## 11. Recommendations to Accept, Modify, Defer, or Reject

- **Accept:** Deterministic SQLite control plane, repo-local state, CLI-first control surface, JSON configuration, idempotent artifact generation.
- **Modify:** Reduce the CLI command surface (drop `ack`, merge `block`/`verdict` into `complete`). Simplify queue semantics to manual lease recovery.
- **Defer:** Heartbeats, automated retry cycles (cycles should require human restart in V1), parallel execution, AI-inferred parallelism, TUI/Slack/Web dashboards.
- **Reject:** Capturing full PTY transcripts into the SQLite event log.

## 12. Open Blockers

1. **Coordinator Invocation:** If `agent_runner` is purely a CLI invoked by agents, what initiates the run and routes jobs? There must be a top-level `agent_runner serve` or `run` command that acts as the deterministic coordinator loop, dispatching tasks or waiting for agents to poll. The exact architecture of this run loop needs definition before implementation.
2. **JSON Schema Definition:** The exact schema for the workflow configuration must be defined prior to building the parser to ensure deterministic testing.