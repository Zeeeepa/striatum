---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Router Gate Validation
author: operator [self-declared: router-validator-codex-gpt-5-001]

## Passed

- `go generate ./pkg/cli/routes`
- `go test ./pkg/cli/routestest -run TestGeneratedRoutesMatchDaemonMethodContract -count=1`
- `go test ./pkg/cli/... ./pkg/rpc/...`
- `go test ./cmd/striatum`
- `go test ./...` after integrating the workflow/artifact parity slice

## Failed / Blocked

No router validation remains blocked after integration.

## Blocker Class

The router gate is not the final CLI parity gate. Rich per-command output
rendering and any local workflow-authoring commands beyond `workflow validate`
remain follow-up work before Python CLI deletion.
