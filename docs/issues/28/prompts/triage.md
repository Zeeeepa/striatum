# Triage -- GH #28 scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/28/SPEC.md`
2. `go/pkg/workflowtemplates/catalog.json` & `go/pkg/workflowtemplates/catalog.go`
3. `go/pkg/workflowauthoring/workflow.go`
4. `go/pkg/supervisor/helper.go` & `pty.go`
5. MCP router / daemon server code under `go/pkg/mcp/` or `go/pkg/rpc/`
6. Existing Postgres migrations under `src/striatum/daemon_pg/sql/`

## Output

Write `docs/issues/28/SCOPE.md` with `striatum.synthesis.v1` front matter
and the exact `author:` line from the work packet. Include:

- The chosen approach for tmux-based command spawning and stream capturing.
- The chosen approach for the long-polling `await_packet` RPC endpoint in the Go daemon.
- The chosen approach for the Postgres migration `0011_escalation_inbox.sql` and model/trigger/RPC support.
- The exact files in scope.
- The exact files out of scope.
- An acceptance checklist with one numbered check per bullet in `docs/issues/28/SPEC.md`.
- Verification commands (tests, manual tmux sessions attach, database checks).
- Risks and mitigations (e.g. tmux capture buffers, long-poll timeouts, connection pool exhaustion).
