# Read Parameter Adapters Handoff
author: operator [self-declared: read-params-codex-gpt-5-001]

## Landed

- Added `go/pkg/cli/readparams` backed by a shared typed flag/positional parser.
- Read route params preserve repository scoping through `repository_id`, resolving `--repo` through daemon `repo.resolve` where needed.
- Common fields such as `run_id`, `state`, `kind`, `format`, `limit`, `path`, and `out` are parsed without duplicating daemon business logic.

## Commands

- `go test ./pkg/cli/params ./pkg/cli/readparams ./pkg/cli/dispatch` passed as part of `go test ./pkg/cli/... ./pkg/rpc/...`.

## Deferred

Per-command rich text output and exact Python formatting remain outside this bounded router gate.
