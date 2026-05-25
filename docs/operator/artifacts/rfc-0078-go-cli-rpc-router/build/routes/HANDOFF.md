# Generated Route Metadata Handoff
author: operator [self-declared: router-builder-codex-gpt-5-001]

## Landed

- Added `go/pkg/cli/routergen`, a Go generator that reads `contracts/daemon_methods.json`.
- Added generated route metadata in `go/pkg/cli/routes/routes_generated.go`.
- Route records include command, subcommand, daemon method, params group, required capability, scope mode, and deprecation state.
- Added `go/pkg/cli/routestest` freshness coverage comparing generated routes back to `contracts/daemon_methods.json`.

## Commands

- `go generate ./pkg/cli/routes`
- `go test ./pkg/cli/routestest -run TestGeneratedRoutesMatchDaemonMethodContract -count=1` passed.
- `go test ./pkg/cli/... ./pkg/rpc/...` passed.

## Notes

`workflow validate` is intentionally absent from generated daemon routes and covered by a local-boundary test.
