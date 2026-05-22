# Operator/Adoption Audit Findings

author: operator-auditor-gemini-001
status: open
date: 2026-05-22

## Summary

The audit evaluated whether operators and first adopters can understand setup, workflow selection, MCP/CLI transition, recovery, and status surfaces without private project memory. The current documentation is exceptionally thorough for setup and storage transitions, but has minor gaps in day-to-day "watching" ergonomics and the conceptual boundary between repo-local and private memory.

## Findings

### AUD-001: Lack of explicit "How to watch" guide for tmux/PTY sessions

severity: medium
category: operator_ergonomics
status: open
claim: While tmux-based supervision exists, there is no operator-facing guide on how to attach to and watch these sessions.
evidence:
- `docs/CLI_REFERENCE.md`: Mentions `striatum supervise` but lacks attachment commands.
- `docs/HOW_TO_HUMAN.md`: Retains manual operator reference but doesn't mention how to watch AI operator sessions in tmux.
- `docs/issues/28/SPEC.md`: Mentions session naming `striatum-{run_id}-{lane_id}`, but this is in internal issue docs, not operator guides.
impact: New operators cannot easily inspect stalling or busy agent sessions, relying solely on the high-level `dashboard` or `status` outputs.
recommended_action: Add a "Watching the Agent" section to `docs/USING_STRIATUM.md` or `docs/HOW_TO_HUMAN.md` explaining the tmux naming convention and providing the `tmux attach -t <name>` command.
follow_up: docs fix

### AUD-002: "Private Project Memory" definition is implicit

severity: low
category: docs_drift
status: open
claim: The term "private project memory" is used in audit objectives and system prompts but is not defined or managed in any product-facing documentation.
evidence:
- `docs/rfcs/0076-three-lane-code-and-doc-audit-workflow.md`: Uses the term in the auditor primary question.
- `docs/CONTEXT_HYGIENE.md`: Discusses signal-to-noise and signal curation but does not name "private project memory" as a distinct concept for users to maintain.
impact: Operators may be confused about what context should be committed to the repo (e.g., `AGENTS.md`) versus what should be kept in private, machine-local memory.
recommended_action: Add a definition of "private project memory" to `docs/CONTEXT_HYGIENE.md` or `docs/UBIQUITOUS_LANGUAGE.md`, contrasting it with team-shared repo docs.
follow_up: docs fix

### AUD-003: Dashboard/Status output interpretation requires experience

severity: medium
category: operator_ergonomics
status: open
claim: The triage path from a stalled run to a specific recovery verb is not always a direct map in the documentation.
evidence:
- `docs/CLI_REFERENCE.md`: Lists 10+ inspection and recovery verbs.
- `docs/HOW_TO_HUMAN.md`: Provides an escalation playbook but lacks a concise triage table for common dashboard "stuck" states (e.g., "What to do if the dashboard shows 'stale lease'").
impact: First-time operators may find the variety of recovery options (`auto`, `stale-leases`, `requeue-stale`, `process-reconcile`, `resume`) overwhelming when a run stalls.
recommended_action: Add a "Recovery Triage Table" to `docs/HOW_TO_HUMAN.md` that maps specific dashboard/status indicators to recommended recovery verbs.
follow_up: docs fix

### AUD-004: Suggested Starter Workflow path is opaque

severity: low
category: operator_ergonomics
status: open
claim: `striatum adopt` reports a "suggested starter workflow path," but the logic for this selection is not transparent to the user.
evidence:
- `docs/USING_STRIATUM.md`: Mentions `adopt` "reports a suggested starter workflow path."
- `docs/CLI_REFERENCE.md`: Repeats the claim.
impact: Users may follow the suggestion without understanding if it fits their specific repo type or goal, potentially missing better-fitting templates in `WORKFLOW_TYPES.md`.
recommended_action: Ensure `adopt` output explicitly links to `docs/WORKFLOW_TYPES.md` or provides a one-line rationale for the suggestion (e.g., "Empty repo detected: suggesting 'review' starter").
follow_up: source work

### AUD-005: Postgres Role Provisioning assumes system-admin proximity

severity: medium
category: operator_ergonomics
status: open
claim: The PostgreSQL setup guide assumes the operator can execute `psql` as a superuser or via `sudo`.
evidence:
- `docs/POSTGRES_TRANSITION.md` § "Provision the daemon-required role": Uses `sudo -u postgres psql`.
impact: First-time adopters on macOS (using Postgres.app) or those using managed/hosted Postgres instances may struggle to apply the required grants.
recommended_action: Add an "Alternative Platform Setup" note to `docs/POSTGRES_TRANSITION.md` or `docs/GETTING_STARTED.md` covering common non-Linux Postgres setup patterns.
follow_up: docs fix

## Observations

- **Exit Codes:** The use of exit codes 11 (`daemon_unreachable`) and 12 (`repo_not_migrated`) is an excellent ergonomic choice that provides clear remediation paths.
- **Doctor Surface:** `striatum daemon doctor` is a high-signal tool that significantly reduces day-zero friction.
- **Context Hygiene:** `docs/CONTEXT_HYGIENE.md` is a unique and valuable contribution to operator documentation, addressing the "taste" aspect of agent orchestration.
