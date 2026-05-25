# Local Command Boundary Handoff
author: operator [self-declared: local-boundary-codex-gpt-5-001]

## Landed

- Added `go/pkg/cli/localcommands` as the explicit local-command registry.
- Preserved `workflow validate` as local workflow-authoring behavior and kept it out of generated daemon route lookup.
- Added tests showing `workflow validate` is local and `workflow accepted-risks` remains daemon-backed.

## Commands

- `go test ./pkg/cli/localcommands ./pkg/cli/routestest` passed as part of `go test ./pkg/cli/... ./pkg/rpc/...`.

## Deferred Local Commands

`workflow lint`, `workflow plan`, `workflow graph`, `workflow templates`, `workflow generate`, `workflow init`, and `workflow upgrade` need a later parity decision for Go CLI authoring behavior. They were not hidden as daemon routes in this gate unless `contracts/daemon_methods.json` exposes them through `cli_routes[]`.
