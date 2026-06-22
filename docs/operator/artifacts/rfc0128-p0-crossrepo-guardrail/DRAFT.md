# RFC 0128 P0 — cross-repo write-scope guardrail re-land (issue #575)

author: author-author-001
artifact_kind: handoff
status: complete
date: 2026-06-22

## Summary

RFC 0128 (cross-repo run boundary, accepted **D196**) defines the single-repo
run invariant: a Striatum run writes exactly its one registered target
repository. The P0 guardrail that enforces this at `workflow validate` time was
implemented and tested earlier but stranded on the stale quarantine branch
`origin/striatum/quarantine-rfc-0128-wt-929` (`d88eb729`) and never reached
`main`. This change re-implements that guardrail cleanly against current `main`
inside the run worktree — re-implemented, **not** hand-merged from the
quarantine branch — preserving the exit-7 validate contract and its ordering.

The guardrail does two things at validate time and never silently narrows scope
(the #280 failure mode):

1. **REFUSES (exit 7)** any lane `write_scope.allowed_paths` entry that resolves
   outside the registered repository root — absolute paths or parent-escaping
   relative paths. Interior `..` that nets back inside the repo (e.g. `a/../b`)
   is **not** a false refusal; it is left to structural `Validate` (exit 8).
2. **WARNS (non-fatal, exit 0)** when a free-text prompt field
   (`task_prompt.inline`, `objective`, `title`) names a foreign org/repo slug or
   an out-of-repo path token, surfacing cross-repo intent before a lane spawns.

The deferred first-class multi-repo manifest is explicitly **declined**: the
`secondary_repos` key has a name (`DeferredSecondaryReposManifestKey`) but no
code reads it, and `TestSecondaryReposManifestIsNotHonored` proves a workflow
declaring it gains zero cross-repo write capability.

## Files Changed

| File | Change | Lines |
| --- | --- | --- |
| `go/pkg/workflowauthoring/crossrepo.go` | **new** — `RefuseCrossRepoWriteScope` (exit-7 refusal), `pathReachesOutsideRepoRoot`, `ForeignRepoReachWarnings` (prompt-slug warnings), `DeferredSecondaryReposManifestKey`, and the structured `CrossRepoWriteScopeError`. | +276 |
| `go/pkg/workflowauthoring/crossrepo_test.go` | **new** — unit tests for refusal/accept/determinism/panic-safety, foreign-reach warnings, and `TestSecondaryReposManifestIsNotHonored`. | +218 |
| `go/pkg/workflowauthoring/workflow.go` | added `LoadFileUnvalidated` + `LoadUnvalidated`; refactored `Load` to parse-then-`Validate` so the guard can run on the unvalidated shape. | +35 / -11 |
| `go/cmd/striatum/main.go` | re-wired `runWorkflowValidate`: unvalidated load → exit-7 cross-repo refusal → structural `Validate` (exit 8) → existing lints → foreign-reach warnings. | +39 |
| `go/cmd/striatum/main_test.go` | added `TestWorkflowValidateRefusesCrossRepoWriteScope` (exit 7), `TestWorkflowValidateWarnsOnForeignPromptSlug`, and the `crossRepoEscapeWorkflow` / `foreignPromptSlugWorkflow` fixtures. | +145 |

`crossrepo.go` reuses the existing `workflowauthoring` helpers `anySlice`,
`stringsFromSlice`, and `stringValue` (in `workflow.go`); no new helper was
needed and they exist unchanged on current `main`.

## Drift Resolution vs the Quarantine Version

The quarantine base is ~331 commits behind `main`. The two library files
(`crossrepo.go`, `crossrepo_test.go`) ported **verbatim** — they depend only on
helpers that still exist. The two surface files required active drift handling:

### `go/pkg/workflowauthoring/workflow.go`
- The quarantine diff split the JSON-parse body out of `Load` into a new
  `LoadUnvalidated` and added `LoadFileUnvalidated`, leaving `Load` =
  `LoadUnvalidated` + `Validate`. Current `main`'s `Load`/`LoadFile` region is
  structurally identical to the quarantine base (same duplicate-key /
  not-valid-JSON guards), so this split applied **cleanly with no semantic
  drift**: I moved the single `Validate(workflow)` call out of the parse body
  and into the new `Load` wrapper, and added `LoadFileUnvalidated` next to
  `LoadFile`. All existing parse diagnostics (`#99` not-valid-JSON, `#114`
  duplicate-key) are preserved unchanged.

### `go/cmd/striatum/main.go` — the real drift
- The quarantine version replaced `LoadFile` with `LoadFileUnvalidated`, then
  ran `RefuseCrossRepoWriteScope` (exit 7) **before** `Validate` (exit 8) to win
  the exit-code race, then the lints, then warnings.
- On current `main`, `runWorkflowValidate` has grown **two extra lint stages**
  that did not exist at the quarantine base:
  `RefuseAutonomousSharedCheckoutRepoWrite` (exit 8) and
  `verifier.EvaluateAllowlistTemplate` (RFC 0141, exit 8), in addition to
  `RefuseRetiredOneShotLanes` and `refuseSameModelLint`.
- **Resolution:** rather than copy the quarantine function body verbatim (which
  would have dropped the two new lints), I inserted the guardrail into the
  *current* function shape, preserving the required ordering:
  `LoadFileUnvalidated` → **`RefuseCrossRepoWriteScope` (exit 7)** →
  **`Validate` (exit 8)** → `RefuseRetiredOneShotLanes` →
  `RefuseAutonomousSharedCheckoutRepoWrite` → `refuseSameModelLint` →
  `verifier.EvaluateAllowlistTemplate` → `ForeignRepoReachWarnings` → output.
  The cross-repo refusal stays ahead of structural `Validate`; every lint that
  already ran after the validating `LoadFile` still runs after `Validate`,
  unchanged.
- The quarantine diff added a `"path/filepath"` import; on current `main` that
  import **already exists** (used elsewhere), so I did not re-add it. This is
  the only line-count difference from the reference diff (main.go +39 here vs
  +40 in quarantine).
- The JSON success envelope now carries `data.warnings` only when warnings
  exist; human output prints `warning: <message>` lines to stderr and still
  prints `valid` to stdout with exit 0 — matching the quarantine contract.

## Acceptance — verbatim commands and outcomes

Run from the per-job worktree, working dir `go/`:

```
$ cd go && go build ./...
(exit 0, no output)

$ cd go && go vet ./...
(exit 0, no output)

$ cd go && go test ./pkg/workflowauthoring/... ./cmd/striatum/...
ok  	github.com/halbritt/striatum/go/pkg/workflowauthoring	0.131s
ok  	github.com/halbritt/striatum/go/cmd/striatum	0.171s
(exit 0)
```

Targeted verification of the RFC 0128 obligations (`go test -run ... -v`):

- `TestRefuseCrossRepoWriteScopeAcceptsInRepoPaths` — PASS
- `TestRefuseCrossRepoWriteScopeRefusesEscapingPaths` (absolute, parent_escape,
  bare_parent, nested_escape, absolute_no_trim) — PASS
- `TestRefuseCrossRepoWriteScopeIsDeterministicAcrossJobs` — PASS
- `TestRefuseCrossRepoWriteScopeIsPanicSafeOnMalformedJobs` — PASS
- `TestPathReachesOutsideRepoRoot` — PASS
- `TestForeignRepoReachWarningsFlagsForeignSlugAndPaths` — PASS
- `TestForeignRepoReachWarningsIgnoresInRepoReferences` — PASS
- `TestForeignRepoReachWarningsDoesNotFlagRegisteredRepo` — PASS
- `TestSecondaryReposManifestIsNotHonored` — PASS
- `TestWorkflowValidateRefusesCrossRepoWriteScope` (exit 7, structured
  `cross_repo_write_scope` envelope naming `../striatum-warmtier/go`) — PASS
- `TestWorkflowValidateWarnsOnForeignPromptSlug` (exit 0 + `foreign_repo_reach`
  warning naming `acme/widget-service` on both stderr and `data.warnings`) — PASS

All pre-existing `cmd/striatum` validate tests (`TestWorkflowValidateJSON`,
`...RefusesSameModelPairing...`, `...RefusesClaudePrintLane`,
`...RefusesCodexExecLane`, `...RefusesAutonomousSharedCheckoutRepoWrite`, …)
remain green, confirming the re-wiring did not regress the existing lint
ordering. `gofmt -l` on all five changed files reports clean.

## Boundary Notes

- Cross-repo reach is **refused, never silently narrowed** — the refusal message
  names the offending job + path and points at decomposition (one coordinated
  single-repo run per repository) rather than scope widening.
- This is P0 only. Dispatch-time `scope_violation` terminal state, read-only
  artifact federation, decomposition ergonomics, and the first-class multi-repo
  manifest remain explicitly out of scope (P1+ / deferred).
- All edits stayed inside the declared `write_scope`:
  `docs/operator/artifacts/rfc0128-p0-crossrepo-guardrail/`,
  `go/pkg/workflowauthoring/`, and `go/cmd/striatum/`.
