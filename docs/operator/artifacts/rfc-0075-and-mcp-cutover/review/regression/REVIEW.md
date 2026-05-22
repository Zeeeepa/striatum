# Regression Risk Review: RFC-0075 and MCP Cutover
author: reviewer-gemini-001

## 1. Objective
Review the RFC-0075 (Tmux-Observable MCP Agent Sessions) and RFC-0050 (Go Daemon HTTP/SSE MCP) cutover implementation to identify missing tests, daemon startup/MCP regressions, status-surface breakage, and headless-fixture compatibility risks.

## 2. Key Findings

### 2.1. Implemented functionality
*   **Native Go HTTP/SSE MCP Endpoint:** `POST /mcp` and `/mcp/sse` endpoints are implemented and functional (`go/pkg/mcp/http.go`). The endpoint supports `tools/list` and `tools/call` using daemon JSON-RPC methods.
*   **Fake MCP Agent Work Loop Proof:** `tests/test_mcp_fake_agent_loop_e2e.py` demonstrates a complete fake agent loop using MCP for `work.await_packet`, `work.ack`, `work.heartbeat`, `artifact.publish`, and `work.complete`.
*   **Agent-Loop PTY Supervisor:** `go/pkg/agentloop/loop.go` successfully spawns the agent process in a PTY, injecting the MCP endpoint, token material, and run configuration via a bootstrap prompt. The supervisor manages the agent process but correctly defers MCP tool usage to the agent.
*   **Session Report Implementation:** `session.report` has been introduced in the daemon contract and implemented in Go (`go/pkg/mutations/lifecycle.go`), successfully supporting the "ready", "heartbeat", "question", and "escalate" event kinds and publishing `session.reported` events. Tests confirm capability-gating works.

### 2.2. Regressions & Missing Implementation
*   **Missing Tmux Daemon Supervisor Metadata:** The RFC requires the daemon to record the tmux attach command, pane IDs, and process liveness state as operational metadata. This logic is not present in the current agent loop or daemon implementation. The supervisor spawns a PTY but does not manage a tmux session or capture the attach command.
*   **Missing Liveness Deadline Sweeper & Classifier:** RFC-0075 specifies deadlines for MCP tool discovery, `work.await_packet`, `work.ack`, and `work.heartbeat`. The daemon-side liveness detection, missed deadline classification, and protocol activity timestamp tracking described in the RFC's proposal are missing from the current implementation slice.
*   **Status Surface Breakage Risk:** The operator dashboard and current-brief surfaces have not been updated to display the new tmux attach metadata and stall classifications. There's a risk that operators will have zero visibility into stalled sessions once CLI supervision is fully retired unless these surfaces are implemented.
*   **Missing Stall Tests:** The "fake-agent tests for discovery, await, ack, heartbeat, and question stalls" workstream remains open and lacks test coverage.

## 3. Recommendations
*   **Implement Tmux Supervision Metadata:** Enhance the agent loop to spawn or attach to a tmux session, and add daemon components to capture and report tmux session names, pane IDs, and the `attach` command.
*   **Implement Liveness Sweeper & Activity Tracking:** Add tracking for last protocol activity and a daemon-side sweeper to detect missed MCP activity deadlines (discovery, await, ack, heartbeat) and classify session stalls as outlined in the RFC.
*   **Update Status Surfaces:** Ensure operator dashboard and current-brief surfaces display the new tmux attach metadata and stall classifications.
*   **Implement Stall Condition Tests:** Add fake-agent tests covering the various liveness stall conditions (discovery stall, await stall, ack stall, lease heartbeat stall).