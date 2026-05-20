# Implementation Plan: GH #28 — Unified Interactive Harness, Tmux Supervisor, and Escalation Inbox

This plan details the implementation of a unified interactive agent harness, tmux-based supervision, and a stateful escalation inbox in the Postgres daemon, executed via the Striatum workflow runner under `docs/issues/28/`.

## User Review Required

> [!IMPORTANT]
> **Tmux Supervision & Environment:** Spawning agent lanes in headless `tmux` sessions requires `tmux` installed on the host system. The supervisor will wrap lane commands in a `tmux` multiplexer session.

> [!WARNING]
> **Postgres Schema Evolution:** A new PG migration (`0011_escalation_inbox.sql`) will be added to the daemon. Go-embedded migrations must be updated to match the Python migrations.

## Open Questions

None. The workflow runs will serve as direct execution gates.

---

## Proposed Changes

### 1. Workflow Scaffold

#### [NEW] [SPEC.md](file:///home/halbritt/git/striatum/docs/issues/28/SPEC.md)
- Issue specification capturing the requirements and acceptance criteria.

#### [NEW] [workflow.json](file:///home/halbritt/git/striatum/docs/issues/28/workflow.json)
- The 3-job workflow (triage → fix → verify) that will drive the implementation.

#### [NEW] [triage.md](file:///home/halbritt/git/striatum/docs/issues/28/prompts/triage.md)
- Task prompt for the triage phase.

#### [NEW] [fix.md](file:///home/halbritt/git/striatum/docs/issues/28/prompts/fix.md)
- Task prompt for the fix phase.

#### [NEW] [verify.md](file:///home/halbritt/git/striatum/docs/issues/28/prompts/verify.md)
- Task prompt for the verify phase.

#### [MODIFY] [README.md](file:///home/halbritt/git/striatum/docs/issues/README.md)
- Append GH #28 to the documented issues index.

---

### 2. Implementation Scope (To be detailed in `SCOPE.md` during Triage)

The following components will be modified/created during the **fix** job:

- **Go Validator & Catalog Parity**:
  - Add `agy_default` harness profile fragment to `go/pkg/workflowtemplates/catalog.json`.
  - Update `go/pkg/workflowtemplates/catalog.go` to include `agy` and `antigravity`.
  - Update `go/pkg/workflowauthoring/workflow.go` to check the optional `model` lane property.
- **Tmux-Based Supervision**:
  - Modify `go/pkg/supervisor` to spawn commands inside named headless tmux sessions and multiplex output.
- **Interactive MCP Loop**:
  - Implement `await_packet` RPC endpoint in the Go daemon MCP/RPC handlers.
  - Implement `src/striatum/skills/mcp_loop_wrapper.py` python wrapper.
- **Stateful Escalation Inbox**:
  - Create Postgres migration `0011_escalation_inbox.sql` under python/Go dirs.
  - Connect inbox triggers/procedures.

---

## Verification Plan

### Automated Tests
- Run `make test` and `make daemon-go-test` to verify Go/Python parity.
- Run `pytest` on newly added integration tests for tmux output capture and escalation inbox tables.

### Manual Verification
- Start the prepared run:
  ```bash
  PYTHONPATH=src python3 -m striatum.cli run prepare --workflow docs/issues/28/workflow.json --json
  PYTHONPATH=src python3 -m striatum.cli run start --run-id <run_id> --json
  ```
- Monitor via `striatum dashboard --run-id <run_id>` and attach to spawned tmux sessions (`tmux attach-session -t striatum-...`).
