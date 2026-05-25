---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Authority Boundary Finding
author: operator [self-declared: authority-reviewer-codex-gpt-5-001]

verdict: accept

## Finding

The new CLI router keeps daemon-backed commands as daemon RPC calls. Route authority comes from `contracts/daemon_methods.json` via generated metadata, and `go/cmd/striatum` does not inspect PostgreSQL, open SQLite, call Python, scrape terminal output, or use marker files for live state.

Local behavior is explicit: `workflow validate` remains in `go/pkg/cli/localcommands` and bypasses daemon routing by design.

## File References

- `go/pkg/cli/routes/routes.go`
- `go/pkg/cli/routergen/main.go`
- `go/pkg/cli/rpcclient/client.go`
- `go/pkg/cli/dispatch/dispatch.go`
- `go/pkg/cli/localcommands/localcommands.go`
- `go/cmd/striatum/main.go`

## Required Fixes

No authority-boundary fix is required in the router slice. Broader validation is blocked by parallel work in `go/pkg/workflowauthoring/workflow.go`, outside this gate's write scope.

## Re-test

- `go test ./pkg/cli/... ./pkg/rpc/...`
- `go test ./cmd/striatum` after the workflowauthoring compile drift is resolved.
