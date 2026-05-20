# Implement -- GH #28

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/28/SPEC.md`
- `docs/issues/28/SCOPE.md`
- The source modules named in `SCOPE.md`

## Deliverables

Per `docs/issues/28/SPEC.md` "Acceptance / Definition of done":

1. Go template catalog parity: add `agy_default` harness profile fragment to `catalog.json` and support `"agy"` & `"antigravity"` tool families in `catalog.go`.
2. Go validator parity: update `workflowauthoring.Validate` to validate that the optional `model` property on a lane is a non-empty string.
3. Tmux-based supervision: wrap command spawning inside a headless tmux session and ensure I/O stream capturing functions seamlessly.
4. Interactive MCP loop: implement `striatum.work.await_packet` RPC handler with long-polling capability.
5. Python MCP loop wrapper: add `src/striatum/skills/mcp_loop_wrapper.py` for non-native CLI tools to talk MCP.
6. Stateful Escalation Inbox: create PG migration 0011 implementing the `striatumd.escalation_inbox` table, and model/trigger/RPC support.
7. Verification tests: ensure Go tests (`make daemon-go-test`), Python tests (`make test`), and smoke tests pass.

## Constraints

- Stay inside `write_scope.allowed_paths`.
- Ensure all Go/Python schema validations are in sync.
- Use the exact `author:` line from the work packet in the handoff.

## Handoff

Write `docs/issues/28/build/HANDOFF.md` with `striatum.handoff.v1` front matter. Cite each definition-of-done bullet closed, files changed, tests run, and residual risk.
