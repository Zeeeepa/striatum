---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: author-author-002
run_id: run_273309e0432e92c869ab06acfad30ece
date: 2026-06-22
title: "RFC 0128 P0 cross-repo write-scope guardrail re-land — apply summary (#575)"
inputs: ["docs/operator/artifacts/rfc0128-p0-crossrepo-guardrail/review/REVIEW.md", "docs/operator/artifacts/rfc0128-p0-crossrepo-guardrail/DRAFT.md"]
---

# Apply Summary: RFC 0128 P0 cross-repo write-scope guardrail re-land (issue #575)

The reviewer returned **`accept`** with no defects and no required revisions —
its "Notes" section is explicitly non-blocking and lists nothing to discharge.
This apply job therefore introduced **no new source changes**; it re-confirmed
the change is complete and green against current `main` and authored this
synthesis. The deliverable is the single commit already on the run branch
`striatum/rfc0128-p0-crossrepo-guardrail` at HEAD `98012b0f`, one commit ahead
of `main` (`9bd90a6c`).

## Files changed

Source diff `9bd90a6c..98012b0f` (`main..HEAD`), 5 files, **702 insertions / 11
deletions**:

| File | Change | Purpose |
| --- | --- | --- |
| `go/pkg/workflowauthoring/crossrepo.go` | **new**, +276 | RFC 0128 P0 (D196) guardrail: `RefuseCrossRepoWriteScope` (exit-7 contract), `pathReachesOutsideRepoRoot`, `ForeignRepoReachWarnings`, and the named-but-unread `DeferredSecondaryReposManifestKey`. |
| `go/pkg/workflowauthoring/crossrepo_test.go` | **new**, +218 | Unit coverage for the guardrail, path-escape detection, foreign-reach warnings, the declined `secondary_repos` manifest, panic-safety, and determinism. |
| `go/pkg/workflowauthoring/workflow.go` | +35 / −11 | Split parsing from validation so the guardrail can run before structural `Validate`. |
| `go/cmd/striatum/main.go` | +39 | Wire the guardrail into `runWorkflowValidate` (exit 7 ahead of exit 8) plus non-fatal foreign-reach warnings. |
| `go/cmd/striatum/main_test.go` | +145 | CLI-level coverage of the exit-7 refusal, the foreign-prompt-slug warning, and the preserved lint ordering. |

(The branch's commit also carries the prior `draft` job's
`docs/operator/artifacts/rfc0128-p0-crossrepo-guardrail/DRAFT.md` handoff
artifact, +143; that is an operator artifact, not product source.)

## How the `main.go` / `workflow.go` drift was resolved

The two library files were not re-landed as a stale verbatim copy of the
quarantine base; they were adapted to the **current** shape of `main`:

- **`workflow.go`** — the parse logic was factored out of `Load`. A new
  `LoadUnvalidated(path)` does the JSON-shape + duplicate-key parse **without**
  `Validate`; `Load` now calls `LoadUnvalidated` then `Validate` (behavior
  unchanged for existing callers); and a new `LoadFileUnvalidated(repoRoot,
  path)` pairs path resolution with the unvalidated parse. This is what lets the
  guardrail inspect the raw declared write-scope before structural validation
  would otherwise reject (or accept) the workflow.
- **`main.go` / `runWorkflowValidate`** — on current `main` this function has
  grown two lint stages that did **not** exist at the quarantine base
  (`RefuseAutonomousSharedCheckoutRepoWrite` and
  `verifier.EvaluateAllowlistTemplate`). Rather than overwrite the function, the
  guardrail was inserted into its current shape:
  1. `LoadFile` → `LoadFileUnvalidated` (parse first, no structural validation);
  2. `RefuseCrossRepoWriteScope` → **exit 7**, code `cross_repo_write_scope`;
  3. explicit `Validate` → **exit 8**, code `workflow_invalid`;
  4. the pre-existing lints (retired one-shot agent refusals,
     `RefuseAutonomousSharedCheckoutRepoWrite`,
     `verifier.EvaluateAllowlistTemplate`) — all still run **after** the
     validating step, so their ordering is preserved;
  5. `ForeignRepoReachWarnings` → non-fatal warnings surfaced on stderr and in
     the JSON `data.warnings` array.

  The `"path/filepath"` import already present on `main` was reused
  (`filepath.Base(repoRoot)`), not re-added. The pre-existing validate-lint
  tests stayed green, confirming the re-wiring did not regress ordering.

## Verbatim build / vet / test results

Run in the per-job worktree at HEAD `98012b0f`, working dir `go/`:

```
$ go build ./... && go vet ./...
exit=0

$ go test ./pkg/workflowauthoring/... ./cmd/striatum/...
ok  	github.com/halbritt/striatum/go/pkg/workflowauthoring	0.049s
ok  	github.com/halbritt/striatum/go/cmd/striatum	0.092s
exit=0

$ gofmt -l go/pkg/workflowauthoring/crossrepo.go go/pkg/workflowauthoring/crossrepo_test.go \
          go/pkg/workflowauthoring/workflow.go go/cmd/striatum/main.go go/cmd/striatum/main_test.go
(no files listed — clean)
```

The packet's explicitly-named test passes:

```
$ go test ./pkg/workflowauthoring/... -run TestSecondaryReposManifestIsNotHonored -v
=== RUN   TestSecondaryReposManifestIsNotHonored
--- PASS: TestSecondaryReposManifestIsNotHonored (0.00s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/workflowauthoring	0.001s
```

All RFC 0128 obligation tests pass (`TestWorkflowValidateRefusesCrossRepoWriteScope`,
`TestWorkflowValidateWarnsOnForeignPromptSlug`,
`TestRefuseCrossRepoWriteScopeAcceptsInRepoPaths`,
`TestRefuseCrossRepoWriteScopeRefusesEscapingPaths` and its five subcases,
`TestRefuseCrossRepoWriteScopeIsDeterministicAcrossJobs`,
`TestRefuseCrossRepoWriteScopeIsPanicSafeOnMalformedJobs`,
`TestPathReachesOutsideRepoRoot`,
`TestForeignRepoReachWarningsFlagsForeignSlugAndPaths`,
`TestForeignRepoReachWarningsIgnoresInRepoReferences`,
`TestForeignRepoReachWarningsDoesNotFlagRegisteredRepo`).

## Exit-7 cross-repo refusal contract — confirmed

The contract holds at three levels:

1. **Source.** `runWorkflowValidate` (`go/cmd/striatum/main.go:851-852`) calls
   `RefuseCrossRepoWriteScope` and returns **exit code 7** with structured code
   `cross_repo_write_scope`, ordered **before** the structural `Validate` (exit
   8) at line 855 — the refusal wins the exit-code race.
2. **Test.** `TestWorkflowValidateRefusesCrossRepoWriteScope` asserts the CLI
   returns exit 7, `ok: false`, `error.code == "cross_repo_write_scope"`, and a
   message naming the offending path — PASS.
3. **Binary.** Building the CLI and running `striatum workflow validate --json`
   on a workflow whose lane declares `allowed_paths: ["../striatum-warmtier/go"]`
   produced, verbatim:

   ```
   {"error":{"code":"cross_repo_write_scope","message":"job \"build\" declares write_scope allowed_path \"../striatum-warmtier/go\", which resolves outside the run's registered repository root. A Striatum run writes exactly one registered repository (RFC 0128 / D196); cross-repo reach is refused, never silently narrowed. For genuine cross-repo work, decompose it into one coordinated single-repo run per repository instead of widening this scope."},"ok":false}
   CLI_EXIT=7
   ```

   Interior `..` that nets back inside the repo (e.g. `a/../b`) is **not** falsely
   refused — it falls through to structural `Validate`. Cross-repo reach is
   refused, never silently narrowed.

## Scope

The change is well-scoped to RFC 0128 **P0** (D196). Dispatch-time
`scope_violation` terminal state, read-only artifact federation, decomposition
ergonomics, and the first-class `secondary_repos` multi-repo manifest are
explicitly **deferred** — `DeferredSecondaryReposManifestKey` has a name but no
reader, and `TestSecondaryReposManifestIsNotHonored` proves declaring it grants
zero cross-repo write capability.

The work is correct, complete for P0, green, and ready to land. References issue
#575.
