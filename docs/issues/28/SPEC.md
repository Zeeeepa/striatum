# GH #28 — Unified Interactive Harness, Tmux Supervisor, and Escalation Inbox

Source: https://github.com/halbritt/striatum/issues/28

## Summary

This issue transitions Striatum to support an interactive execution loop (RFC 0049) for all agents (not just Claude Code), wraps supervised process-adapter agent loops inside headless `tmux` sessions for operator observability, and adds a stateful escalation inbox to Postgres.

Additionally, we align the Go daemon's workflow template catalog and validator with the Python side (including `agy` / `antigravity` tool families and the `model` lane property).

## Acceptance / Definition of done

### 1. Go Parity & Harness Updates
- `go/pkg/workflowtemplates/catalog.json` contains the `agy_default` harness profile fragment.
- `go/pkg/workflowtemplates/catalog.go` accepts `"agy"` and `"antigravity"` as valid tool families.
- Go's `workflowauthoring.Validate` checks that the optional `model` property on a lane, if present, is a non-empty string.
- Python and Go tests pass.

### 2. Tmux-Based Go Supervision
- In `go/pkg/supervisor/helper.go` (or `pty.go`), the supervisor spawns the agent command inside a headless `tmux` session named `striatum-{run_id}-{lane_id}`.
- The supervisor captures the stdout/stderr stream from the tmux session (e.g., using `tmux pipe-pane` or similar, or redirecting to a shared FIFO/file) so heartbeat monitoring, transcript logging, and progress logs function exactly as before.
- Heartbeat stalls and exits are still caught cleanly by the supervisor.

### 3. Interactive MCP Loop (RFC 0049)
- Expose the RPC endpoint `striatum.work.await_packet(session_id)` in the Go daemon.
- This endpoint should support long-polling: if no claimable job exists, block/long-poll (with timeout/context cancellation) until a job is queued for the lane, then return the work packet.
- Provide a Python-side wrapper `src/striatum/skills/mcp_loop_wrapper.py` that connects to the MCP socket and processes packets, allowing non-native CLIs to run in interactive mode.

### 4. Stateful Escalation Inbox
- Implement `striatumd.escalation_inbox` table in Postgres (via migration `0011_escalation_inbox.sql`).
- The table must track escalations, their state (`pending`, `viewed`, `resolved`), and links to the escalation artifacts.
- Provide corresponding Go and Python models, RPC methods, or triggers to populate/update the inbox when an escalation blocker/artifact is created or resolved.

## Suggested implementation path

1. **Go Validator Parity**: Update `catalog.go`, `catalog.json`, and `workflow.go` to support `agy`/`antigravity` and validate the `model` field.
2. **Tmux Supervision**: Modify `go/pkg/supervisor` command execution to use `tmux new-session -d` and ensure I/O pipes are captured.
3. **Escalation Schema & RPCs**: Add migration 0011 and wire the DB models.
4. **Await Packet**: Implement the long-polling RPC under `go/pkg/mcp/` or the RPC method handlers.

## Provenance

Addresses the remaining requirements from the interactive agent loop design and tmux-based supervisor logging requested by the principal.
