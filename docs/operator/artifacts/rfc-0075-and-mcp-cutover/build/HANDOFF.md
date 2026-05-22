# RFC 0075 / MCP Cutover First Slice Handoff
author: implementer-codex-001

## Summary

Implemented the first structured pre-work MCP slice: `session.report`.
The new daemon method is contract-visible, claim-authorized,
single-repository scoped, available through MCP `tools/list` for claim
tokens, and records metadata-only `session.reported` events. This gives MCP
agents a structured path for `ready`, `heartbeat`, `question`, and
`escalate` reports before a work packet exists, without treating terminal
text or tmux pane output as workflow state.

The agent-loop bootstrap prompt and MCP docs now teach `session.report` as
the pre-`work.await_packet` blocker/question path. Contract-generated Go
registry and daemon method tables were regenerated, and the authority matrix
classifies the new method as Go-only current production authority.

## Changed Files

- `contracts/daemon_methods.json`
- `src/striatum/daemon_rpc/daemon_methods.json`
- `go/pkg/rpc/registry_methods.go`
- `go/pkg/mutations/lifecycle.go`
- `go/pkg/mutations/mutations.go`
- `go/pkg/agentloop/bootstrap.go`
- `docs/MCP.md`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- `docs/architecture/DAEMON_METHOD_TABLES.md`
- focused Go/Python tests under `go/pkg/*` and `tests/`

## Verification

- `python scripts/generate_go_rpc_registry.py`
- `python scripts/generate_daemon_method_tables.py`
- `make daemon-go-build`
- `go test ./pkg/mutations ./pkg/rpc ./pkg/mcp && go test ./pkg/agentloop -run TestBuildBootstrapPromptNamesNativeMCPBoundary`
- `pytest tests/test_mcp_fake_agent_loop_e2e.py -q`
- `pytest tests/test_go_rpc_registry_generation.py tests/test_daemon_method_tables_generation.py tests/test_ui_packaging.py tests/architecture/test_authority_guardrails.py -q`

Note: full `go test ./pkg/agentloop` still has an existing environment
sensitivity in `TestResolveMCPEndpointFromPort`; it reads this live
Striatum runtime endpoint instead of the test port. The changed bootstrap
test passes directly.

## Follow-Ups

- Persist per-session MCP activity timestamps (`last_tools_list_at`,
  `last_await_packet_at`, etc.) instead of only recording `session.reported`
  events.
- Add tmux metadata persistence and status/dashboard liveness projection.
- Add the deadline sweeper and `session.liveness_deadline_missed` /
  recovered events.
- Gate live-interactive lanes fail-closed on missing tmux in a later parity
  slice.
