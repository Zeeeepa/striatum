---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Route Tests Handoff
author: operator [self-declared: route-tester-codex-gpt-5-002]

## Changed Files

- `go/pkg/webservice/service_test.go`
- `go/pkg/websse/sse_test.go`
- `go/pkg/webtest/webtest.go`
- `go/pkg/webguardrails/guardrails_test.go`
- `go/cmd/striatumd/web_routes_test.go`

## Tests Added

- `TestHealthAndSecurityHeaders`
- `TestReadRouteUsesDaemonRPC`
- `TestMutationRefusedWhenDisabled`
- `TestMutationAllowedWhenEnabled`
- `TestNonLoopbackHostRefused`
- `TestRetiredRoutesReturnGone`
- `TestStaticAssetServedFromGoEmbed`
- `TestArtifactRawUsesDaemonContent`
- `TestSincePrefersLastEventID`
- `TestWriteEventFramesData`
- `TestStreamWritesTerminalEventAndReturns`
- `TestGoWebCutoverDoesNotImportPythonWebService`
- `TestRetiredRouteNamesRemainDocumentedInGuardScript`
- `TestWebServiceAdapterServesHealth`
- `TestWebServiceAdapterEnforcesServiceBearer`
- `TestWebServiceAdapterSSEUsesDaemonRunEvents`

## Validation

- `go test ./pkg/webassets ./pkg/websse ./pkg/webservice ./pkg/webtest ./pkg/webguardrails` passed.
- `scripts/guard_rfc0078_web_retirement.sh` passed.
- `go test ./cmd/striatumd -run 'TestWebServiceAdapter|TestListenMCPHTTPRejectsNonLoopback'` passed.
- `go test ./...` failed in the current parallel-worktree state outside this
  gate: `go/pkg/reads` workflow-authoring handler tests reject a fixture lane
  command as empty, and `go/pkg/workflowgenerate` rejects a fixture with
  non-list `context_docs`.

## Untested Or Blocked Routes

Full HTML page parity for `/doctor`, `/escalations*`, `/cross-repo`,
`/view*`, `/workflows*`, and run action POST routes is blocked. Those routes
need either Go page/action ports or explicit retirement decisions before the
Python web package can be deleted.
