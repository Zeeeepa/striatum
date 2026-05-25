---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Go Web Service Cutover Summary
author: operator [self-declared: web-cutover-closer-codex-gpt-5-002]

## Final Route State

| State | Route families |
|---|---|
| ported | `/v1/health`, `/v1/runs`, `/v1/runs/<run_id>`, `/v1/runs/<run_id>/why`, `/v1/runs/<run_id>/dashboard`, `/v1/runs/<run_id>/events`, `/v1/runs/<run_id>/artifacts`, `/v1/artifacts/<id>/raw`, `/workflow-templates`, `/workflow-templates/<id>`, `/workflows/generate/preview`, `/workflows/generate`, `/v1/invoke`, `/static/*` |
| retired | `/dogfood*`, `/chat*` |
| blocked | `/doctor`, `/escalations*`, `/cross-repo`, `/view*`, `/workflows*`, full `/run/*` HTML/action parity |
| deferred | Process startup wiring for the Go local web service, pending coordination with parallel CLI/packaging edits |

## Files Changed By Lane

- Routes: `docs/operator/artifacts/rfc-0078-go-web-service-cutover/routes/ROUTE_LEDGER.md`
- Service/security: `go/pkg/webservice/`, `go/pkg/webtest/`, `go/cmd/striatumd/web_service.go`, `go/cmd/striatumd/web_routes_test.go`
- Static/templates: `go/pkg/webassets/`, `go/web/`
- SSE: `go/pkg/websse/`
- Guardrails: `go/pkg/webguardrails/`, `scripts/guard_rfc0078_web_retirement.sh`

## Validation

- Passed: `go test ./pkg/webassets ./pkg/websse ./pkg/webservice ./pkg/webtest ./pkg/webguardrails`
- Passed: `scripts/guard_rfc0078_web_retirement.sh`
- Passed: `go test ./cmd/striatumd -run 'TestWebServiceAdapter|TestListenMCPHTTPRejectsNonLoopback'`
- Passed after integration: `go test ./...`

## Deletion Readiness

Python service/web code cannot be deleted now. The Go service/security, static
embedding, SSE, tests, and retired-route guardrails are in place for the
retained subset, but several HTML/operator action routes are still blocked and
the process startup integration is not wired.

## Next Executable Slice

Wire `go/pkg/webservice` into the Go `striatumd` startup surface with a
loopback listen flag and runtime endpoint publication, then port or retire the
blocked HTML/action routes one group at a time: doctor, escalation inbox,
workflow browser/editor, repo view, cross-repo, and run/job action pages.

Live workflow state remains daemon-owned PostgreSQL/RPC. Terminal output, tmux
panes, transcripts, marker files, and `.striatum/` scratch were not used as
authority for this gate.
