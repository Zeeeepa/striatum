# Port Go Service And Security Layer

You are the Go service/security implementation lane for the RFC 0078 local
web/service cutover. Stay within the job write scope.

Read the route ledger if present, the plan, RFC 0070, RFC 0078,
`contracts/daemon_methods.json`, `go/cmd/striatumd/`, `go/pkg/rpc/`,
`go/pkg/reads/`, `go/pkg/mutations/`, and the Python service/web entry points.

Implement or advance the smallest coherent Go service/security slice that:

- serves only local/loopback operator surfaces;
- routes retained reads and mutations through daemon RPC handlers;
- keeps mutation behavior gated by capability and explicit allow-mutations
  policy where that policy still applies;
- preserves security headers and CSP expectations for HTML/static responses;
- refuses non-loopback or unauthorized access in tests;
- does not introduce hosted service, telemetry, transcript capture, external
  persistence, or direct database writes from the web layer.

Produce
`docs/operator/artifacts/rfc-0078-go-web-service-cutover/service/HANDOFF.md`
with author line:

`author: operator [self-declared: service-porter-codex-gpt-5-002]`

The handoff must list files changed, retained routes covered, validation
commands/results, unresolved blockers, and any routes intentionally deferred to
another lane.
