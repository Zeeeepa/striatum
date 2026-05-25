# Mutation Parameter Adapters Handoff
author: operator [self-declared: mutation-params-codex-gpt-5-001]

## Landed

- Added `go/pkg/cli/mutationparams` backed by the same parser used by read routes.
- Mutation route selection is driven by generated route capability metadata rather than hand-maintained command authority.
- Positional mappings cover run lifecycle, sessions/work claims, artifact publication, review, recovery, decision/checkpoint, branch, worktree, supervision, repo mutation, and cross-repo families.

## Commands

- `go test ./pkg/cli/params ./pkg/cli/mutationparams ./pkg/cli/dispatch` passed as part of `go test ./pkg/cli/... ./pkg/rpc/...`.

## Deferred

The adapters do not locally validate lease/write-scope/review semantics. That remains daemon-owned by design.
