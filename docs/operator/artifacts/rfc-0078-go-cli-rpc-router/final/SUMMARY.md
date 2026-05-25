---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Go CLI RPC Router Gate Summary
author: operator [self-declared: router-closer-codex-gpt-5-001]

## Landed

- Generated Go CLI route metadata from `contracts/daemon_methods.json`.
- Freshness tests fail when generated route metadata drifts from the contract.
- Go CLI daemon-backed routes dispatch through the existing daemon RPC envelope over the Unix socket.
- Runtime socket/token resolution follows daemon runtime conventions and supports explicit CLI/env overrides.
- Read and mutation parameter adapters convert CLI flags/positionals into daemon RPC params without duplicating daemon method authority.
- `workflow validate` remains an explicit local workflow-authoring exception.

## Validation

Passed:

- `go generate ./pkg/cli/routes`
- `go test ./pkg/cli/routestest -run TestGeneratedRoutesMatchDaemonMethodContract -count=1`
- `go test ./pkg/cli/... ./pkg/rpc/...`
- `go test ./cmd/striatum`
- `go test ./...` after integrating the parallel workflow/artifact parity
  slice

## Local Exceptions

`workflow validate` is local because it validates workflow files before daemon state may exist. Other workflow-authoring commands not present in `cli_routes[]` remain deferred to the workflow/artifact parity gate.

## Remaining Unported / Deferred

- Rich Python CLI text rendering for daemon routes: next CLI parity/output gate.
- Local workflow-authoring commands beyond `workflow validate`: workflow/artifact parity gate and later CLI parity/output slices.
- Rich text output parity for daemon-routed commands.

## Recommended Next Gate

Continue with the remaining Python deletion blockers: rich CLI output parity,
full workflow authoring parity, and removal or retirement of Python
source/tests/scripts named by the Python-trace guardrail.
