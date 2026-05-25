---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Go Service And Security Handoff
author: operator [self-declared: service-porter-codex-gpt-5-002]

## Changed Files

- `go/pkg/webservice/service.go`
- `go/pkg/webservice/service_test.go`
- `go/pkg/webtest/webtest.go`
- `go/cmd/striatumd/web_service.go`
- `go/cmd/striatumd/web_routes_test.go`

## Shipped Slice

The Go local web/service handler now serves a daemon-RPC-backed route subset:
health, run status/dashboard/why/artifact listings, artifact raw content,
workflow template reads, workflow generation endpoints, `/v1/invoke`, static
assets, and run event SSE. Mutations are refused unless `AllowMutations` is
true. HTTP service-token authentication, loopback Host refusal, CSP,
`nosniff`, and same-origin mutation checks are implemented in the Go handler.

The web layer does not open PostgreSQL, does not read `.striatum/` state, and
does not import Python service/web modules. All retained live-state reads and
mutations enter `rpc.Server.HandleWithoutHandshake` with a repository id and
capability token.

## Validation

- `go test ./pkg/webassets ./pkg/websse ./pkg/webservice ./pkg/webtest ./pkg/webguardrails` passed.
- `scripts/guard_rfc0078_web_retirement.sh` passed.
- `go test ./cmd/striatumd -run 'TestWebServiceAdapter|TestListenMCPHTTPRejectsNonLoopback'` passed.

## Blockers

The service is not wired into `striatumd` process startup in this gate; that
requires coordination with the CLI/packaging worker editing `go/cmd/striatum`
and `go/cmd/striatumd` startup surfaces.
