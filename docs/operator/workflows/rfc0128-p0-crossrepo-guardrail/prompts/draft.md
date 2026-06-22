# Task: re-land RFC 0128 P0 — cross-repo write-scope guardrail (issue #575)

RFC 0128 (cross-repo run boundary) is **accepted (D196)** but its P0 guardrail is
**not on `main`**. A complete, tested implementation was stranded on a quarantine
branch and is now stale. Re-apply it cleanly against current `main`. This lands
via the daemon, **not a PR**.

## Reference implementation — the spec; read it first
The stranded impl is `origin/striatum/quarantine-rfc-0128-wt-929` (commit `d88eb729`).
Read each file and diff it against main:
- `git show origin/striatum/quarantine-rfc-0128-wt-929:go/pkg/workflowauthoring/crossrepo.go`
- `git show origin/striatum/quarantine-rfc-0128-wt-929:go/pkg/workflowauthoring/crossrepo_test.go`
- `git show origin/striatum/quarantine-rfc-0128-wt-929:go/pkg/workflowauthoring/workflow.go`
- `git show origin/striatum/quarantine-rfc-0128-wt-929:go/cmd/striatum/main.go`
- `git show origin/striatum/quarantine-rfc-0128-wt-929:go/cmd/striatum/main_test.go`
- `git diff origin/main origin/striatum/quarantine-rfc-0128-wt-929 -- <file>`

## Deliverable — write these source files in your worktree
1. `go/pkg/workflowauthoring/crossrepo.go` — port `RefuseCrossRepoWriteScope`
   (structured **exit-7** refusal for any `write_scope.allowed_paths` entry that
   resolves outside the registered repo root: absolute or parent-escaping paths;
   interior `..` that nets back inside must NOT be a false refusal), the
   foreign-repo-slug prompt **warning**, and `DeferredSecondaryReposManifestKey`
   (declined surface).
2. `go/pkg/workflowauthoring/crossrepo_test.go` — port the tests, incl.
   `TestSecondaryReposManifestIsNotHonored`.
3. `go/pkg/workflowauthoring/workflow.go` + `go/cmd/striatum/main.go` — re-wire the
   validate path and CLI. **Resolve the drift**: main's CLI surface moved ~331
   commits since the quarantine base, so adapt to current code rather than copying
   verbatim. Preserve the exit-7 validate contract and its ordering.
4. `go/cmd/striatum/main_test.go` — port the CLI tests, adapted to current main.

## Acceptance — run these, they must pass
- `cd go && go build ./... && go vet ./...` clean.
- `cd go && go test ./pkg/workflowauthoring/... ./cmd/striatum/...` green
  (ported tests + `TestSecondaryReposManifestIsNotHonored` pass).
- Cross-repo reach is **refused** (exit-7, actionable message), never silently narrowed.

## Handoff artifact
Write the `draft` handoff artifact: files changed, exactly how you resolved the
`main.go`/`workflow.go` drift vs the quarantine version, and the verbatim
build/vet/test commands + their outcomes.

Stay inside `write_scope`. Do **not** hand-merge the quarantine branch — re-implement against current `main`.
