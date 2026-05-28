---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: operator

# GH #28 Handoff

## Scope Closed

1. Go template catalog parity: Added `agy_default` harness profile fragment to `catalog.json` and supported `"agy"` & `"antigravity"` tool families in `catalog.go`.
2. Go validator parity: Updated `workflowauthoring.Validate` to validate that the optional `model` property on a lane is a non-empty string.
3. Tmux-based supervision: Wrapped command spawning inside a headless tmux session and ensured I/O stream capturing functions seamlessly in the Go supervisor helper.
4. Interactive MCP loop: Implemented `striatum.work.await_packet` RPC handler with long-polling capability in `mcp/` and `mutations/`.
5. Python MCP loop wrapper: Added `src/striatum/skills/mcp_loop_wrapper.py` for non-native CLI tools to talk MCP.
6. Stateful Escalation Inbox: Created PG migration 0011 implementing the `striatumd.escalation_inbox` table, and model/trigger/RPC support. Fixed Go list sessions query.
7. Verification tests: Verified Go tests (`make daemon-go-test`), Python tests (`make test`), and smoke tests pass.

## Files Changed

- `go/pkg/workflowtemplates/catalog.json`
- `go/pkg/workflowtemplates/catalog.go`
- `go/pkg/workflowauthoring/workflow.go`
- `go/pkg/supervisor/helper.go`
- `go/pkg/supervisor/pty.go`
- `go/pkg/mcp/`
- `go/pkg/mutations/`
- `go/pkg/reads/listings.go`
- `src/striatum/skills/mcp_loop_wrapper.py`
- `go/pkg/db/sql/0011_escalation_inbox.sql`
- various Python and Go test files

## Residual Risk

- Python tests take a while to complete in CI/local runs; any flaky tmux-based tests might need longer timeouts.
