---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
---

# Operator Ergonomics Finding
author: operator [self-declared: ergonomics-reviewer-codex-gpt-5-001]

verdict: needs_revision

## Finding

The router gives operators understandable daemon routing and stable high-level failure classes, including daemon unreachable, repository not registered, version skew, and capability refusal. Common global options are explicit.

The remaining ergonomics gap is output parity: newly routed daemon commands emit generic JSON. This is acceptable for a router gate but not enough to declare full Python CLI parity.

## File References

- `go/pkg/cli/dispatch/dispatch.go`
- `go/pkg/cli/rpcclient/client.go`
- `go/cmd/striatum/main.go`

## Required Fixes

No blocker for the generated-router authority gate. A later RFC 0078 CLI parity gate should add per-command text renderers or explicitly retire legacy text shapes.

## Re-test

- `go test ./pkg/cli/... ./pkg/rpc/...`
