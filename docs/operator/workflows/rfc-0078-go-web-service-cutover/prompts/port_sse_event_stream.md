# Port SSE Event Stream

You are the SSE implementation lane for the RFC 0078 local web/service cutover.
Stay within the job write scope.

Read the plan, route ledger if present, `docs/MCP.md`, Python service SSE
behavior, Go MCP HTTP/SSE behavior, daemon event reads, and current run-event
RPC surfaces.

Implement or advance Go SSE behavior for retained service routes. The stream
must be backed by daemon-owned run events or accepted read methods, not by
terminal output, pane text, transcripts, filesystem markers, or in-memory-only
Python state.

Cover:

- event format and reconnect behavior;
- authorization/capability handling;
- heartbeat or idle behavior if current behavior has one;
- cancellation/closed-client cleanup;
- route tests or test hooks needed by the route-test lane.

Produce
`docs/operator/artifacts/rfc-0078-go-web-service-cutover/sse/HANDOFF.md`
with author line:

`author: operator [self-declared: sse-porter-codex-gpt-5-002]`

Include changed files, validation commands/results, and any remaining parity
gaps.
