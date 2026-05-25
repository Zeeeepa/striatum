# Add Route Tests

You are the route-test lane for the RFC 0078 local web/service cutover. Stay
within the job write scope.

Read the plan, route ledger if present, service/security handoff if present,
static/template handoff if present, SSE handoff if present, current Go tests,
and Python route tests or fixtures that still describe current behavior.

Add route-level Go tests or parity fixtures for retained and retired web/service
routes. Prefer tests that exercise HTTP handlers without requiring hosted
services or external persistence.

Coverage should include:

- retained read routes and artifact/static responses;
- mutation refusal when mutations are disabled or capability is missing;
- loopback/non-loopback policy;
- CSP and important security headers;
- SSE response headers, event framing, and client cleanup;
- retired routes returning an explicit refusal or 404/410 as appropriate;
- route behavior backed by daemon RPC/PostgreSQL authority.

Produce
`docs/operator/artifacts/rfc-0078-go-web-service-cutover/tests/HANDOFF.md`
with author line:

`author: operator [self-declared: route-tester-codex-gpt-5-002]`

List changed files, test names, validation command results, untested retained
routes, and any blockers requiring source changes by another lane.
