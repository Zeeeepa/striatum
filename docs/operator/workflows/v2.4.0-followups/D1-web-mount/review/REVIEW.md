---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---
# Finding: accept_with_findings

Scope reviewed: `startMCPHTTPServer`, `newDaemonHTTPHandler`, and the mux test.

Interrogation `intg_14d47299b76ef42361afd3f19bef2597` confirms `isMCPPath`
routes exact `/mcp` plus `/mcp/` children to MCP, so `/mcp/sse` and
`/mcp/messages` remain MCP while `/mcpsomething` and `/v1/...` go web-side.

Interrogation also confirms loopback binding and each handler's host gate are
preserved, and MCP keeps the same `mcp.NewHTTPHandler` authorizer path.

Finding: the mounted web service fails open if the daemon runtime token file is
unreadable. `readRuntimeTokenFile` returns empty, `ServiceToken` is empty, and
the implementer confirmed loopback clients can reach web GET routes without a
bearer; no GET origin/CORS gate blocks that access.

Impact is limited to degraded startup state and loopback-host clients, but on a
multi-user host it can expose run state, interrogation bodies, and raw artifacts
to a local user who cannot read the 0600 token.

Recommendation: fail closed for web mounting when the runtime token is empty,
or inject an unguessable deny token so web requests 401 while MCP remains
available.
