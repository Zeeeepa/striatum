# Embed Static Assets And Templates

You are the static/template embedding lane for the RFC 0078 local web/service
cutover. Stay within the job write scope.

Read the plan, route ledger if present, `src/striatum/web/templates/`,
`src/striatum/web/static/`, `src/striatum/web/static_assets.py`, and current Go
service code.

Implement or advance a Go-owned asset/template embedding slice for retained
routes. The result must avoid Python package-data loading at runtime and must
preserve local operator behavior for CSS, JavaScript, templates, static build
assets, cache behavior where relevant, and CSP-compatible script/style usage.

Produce
`docs/operator/artifacts/rfc-0078-go-web-service-cutover/static/HANDOFF.md`
with author line:

`author: operator [self-declared: static-porter-codex-gpt-5-002]`

The handoff must include:

- which static/template files are embedded or intentionally deferred;
- how the Go package exposes assets/templates to the service layer;
- security/CSP implications;
- validation commands/results;
- any route-retirement recommendation caused by template/static complexity.
