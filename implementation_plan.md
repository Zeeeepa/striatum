# Implementation Plan: Striatum Go Core Transition & Unified Interactive Harness

This plan details the transition of Striatum towards a single-binary Go core, integrating model configuration, adding the `agy` (Antigravity) tool family, implementing tmux-based supervision for all agents, and enabling interactive MCP-driven execution (RFC 0049) to bypass programmatic billing skews.

## User Review Required

> [!IMPORTANT]
> **Tmux Dependency:** Tmux is now a hard runtime requirement for the Go supervisor helper. The supervisor will spawn all agent loops in named headless tmux sessions (`striatum-{run_id}-{lane_id}`).

> [!WARNING]
> **Schema Evolution:** We are adding `model` directly to the lane configuration in `workflow.json`. This requires updating the workflow validation schema in both Python (for the transition) and Go.

## Open Questions

None at this stage. All major design decisions have been resolved via the `/grill-me` alignment.

---

## Proposed Changes

### 1. Harness & Schema Updates

We need to add the `agy` tool family and allow the `model` property on lanes.

#### [MODIFY] [workflow.py](file:///home/halbritt/git/striatum/src/striatum/workflow.py)
- Extend `HARNESS_PROFILE_TOOL_FAMILIES` to include `"agy"`.
- Update the lane validation schema to accept an optional `model` string property.

#### [MODIFY] [artifact_contracts.py](file:///home/halbritt/git/striatum/src/striatum/artifact_contracts.py)
- If necessary, verify `agy` doesn't conflict with any byline validation rules.

#### [MODIFY] [catalog.json](file:///home/halbritt/git/striatum/src/striatum/workflow_templates/catalog.json)
- Add a new `harness_profile_fragments` entry for `agy_default` wrapping the Antigravity assistant configuration.

---

### 2. Go Validator (Single Binary Foundation)

We will implement the JSON schema validation and semantic rules in Go.

#### [NEW] [validator.go](file:///home/halbritt/git/striatum/go/pkg/workflow/validator.go)
- Implement structural validation using a Go JSON Schema library.
- Implement DAG cycle detection and write-scope overlap checks in Go.

---

### 3. Tmux-Based Go Supervision

We will rewrite the supervisor helper to spawn commands inside tmux.

#### [MODIFY] [pty.go](file:///home/halbritt/git/striatum/go/pkg/supervisor/pty.go) or [helper.go](file:///home/halbritt/git/striatum/go/pkg/supervisor/helper.go)
- Modify the command launcher to spawn target commands using:
  `tmux new-session -d -s striatum-{run_id}-{lane_id} "<command>"`
- Ensure the helper can still capture stdout/stderr streams from the tmux pipe to feed progress logs and heartbeats.

---

### 4. Interactive MCP Loop (RFC 0049)

We will expose the `await_packet` RPC method on the daemon and support it in all lanes.

#### [NEW] [await_packet.go](file:///home/halbritt/git/striatum/go/pkg/mcp/await_packet.go)
- Implement `striatum.work.await_packet()` RPC endpoint in the Go daemon.
- This endpoint will block/long-poll Postgres for claimable jobs matching the calling session's lane.

#### [NEW] [mcp_loop_wrapper.py](file:///home/halbritt/git/striatum/src/striatum/skills/mcp_loop_wrapper.py)
- For tool families that do not natively speak MCP (like generic scripts), we will provide a thin Python wrapper that connects to the MCP socket and processes packets.

---

### 5. Stateful Escalation Inbox

#### [NEW] [escalation_inbox.sql](file:///home/halbritt/git/striatum/src/striatum/daemon_pg/sql/V99__escalation_inbox.sql)
- Create `striatumd.escalation_inbox` table tracking escalation states (`Pending`, `Viewed`, `Resolved`).

---

## Verification Plan

### Automated Tests
- `make daemon-go-test` to verify Go validator and helper changes.
- Add integration tests verifying a mock agent starting in tmux can successfully long-poll the daemon via `await_packet`.

### Manual Verification
- Start a test run using the `agy` lane, verify a headless tmux session `striatum-test-agy` is spawned.
- Attach to the tmux session manually (`tmux attach -t striatum-test-agy`) to verify visibility of the agent interactive loop.
