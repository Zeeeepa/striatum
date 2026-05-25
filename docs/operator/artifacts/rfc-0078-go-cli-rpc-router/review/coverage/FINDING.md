---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Route Coverage Finding
author: operator [self-declared: coverage-reviewer-codex-gpt-5-001]

verdict: accept

## Finding

All 67 `cli_routes[]` entries in `contracts/daemon_methods.json` are represented in generated Go route metadata and covered by a freshness test. The generated metadata includes the daemon method and params group for every route plus capability, scope, and deprecation metadata from the corresponding daemon method entry.

`workflow validate` is intentionally local and absent from generated daemon routes. Local workflow-authoring commands not listed in `cli_routes[]` remain deferred to the next workflow/artifact parity gate.

## File References

- `go/pkg/cli/routes/routes_generated.go`
- `go/pkg/cli/routestest/routes_freshness_test.go`
- `go/pkg/cli/params/params.go`

## Required Fixes

No generated-route coverage fix is required. Full command-output parity is deferred.

## Re-test

- `go generate ./pkg/cli/routes`
- `go test ./pkg/cli/routestest -run TestGeneratedRoutesMatchDaemonMethodContract -count=1`
