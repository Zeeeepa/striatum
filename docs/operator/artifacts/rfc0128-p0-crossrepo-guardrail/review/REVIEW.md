---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept
severity: info
author: reviewer-reviewer-001
run_id: run_273309e0432e92c869ab06acfad30ece
date: 2026-06-22
---

# Review: RFC 0128 P0 cross-repo write-scope guardrail re-land (issue #575)

**Verdict: `accept`.** The change re-lands the RFC 0128 P0 (D196) cross-repo
write-scope guardrail cleanly against current `main`, satisfies every stated
obligation, and is green on a clean build / vet / test that I ran myself in an
isolated worktree at the run-branch HEAD (`98012b0f`). No defects found; no
revision required.

I did **not** trust the author's handoff claims — I re-derived every check
below from the source on the run branch and from binary-level fixtures I
constructed independently.

## What I reviewed

- Source diff `main..striatum/rfc0128-p0-crossrepo-guardrail`:
  - `go/pkg/workflowauthoring/crossrepo.go` (new, +276)
  - `go/pkg/workflowauthoring/crossrepo_test.go` (new, +218)
  - `go/pkg/workflowauthoring/workflow.go` (+35 / −11)
  - `go/cmd/striatum/main.go` (+39)
  - `go/cmd/striatum/main_test.go` (+145)
- The author's `DRAFT.md` handoff (claims cross-checked against source, not taken on trust).

## Independent verification

All commands run by me in a detached worktree at `98012b0f`, working dir `go/`:

| Check | Result |
| --- | --- |
| `go build ./...` | exit 0, clean |
| `go vet ./...` | exit 0, clean |
| `go test ./pkg/workflowauthoring/... ./cmd/striatum/...` | both packages `ok` |
| `gofmt -l` on all 5 changed files | clean (no files listed) |

Targeted RFC 0128 obligation tests, run with `-v`, all PASS:
`TestRefuseCrossRepoWriteScopeAcceptsInRepoPaths`,
`TestRefuseCrossRepoWriteScopeRefusesEscapingPaths` (absolute, parent_escape,
bare_parent, nested_escape, absolute_no_trim),
`TestRefuseCrossRepoWriteScopeIsDeterministicAcrossJobs`,
`TestRefuseCrossRepoWriteScopeIsPanicSafeOnMalformedJobs`,
`TestPathReachesOutsideRepoRoot`,
`TestForeignRepoReachWarningsFlagsForeignSlugAndPaths`,
`TestForeignRepoReachWarningsIgnoresInRepoReferences`,
`TestForeignRepoReachWarningsDoesNotFlagRegisteredRepo`,
`TestSecondaryReposManifestIsNotHonored`,
`TestWorkflowValidateRefusesCrossRepoWriteScope`,
`TestWorkflowValidateWarnsOnForeignPromptSlug`. The pre-existing validate-lint
tests (`...RefusesAutonomousSharedCheckoutRepoWrite`, same-model, print-lane,
codex-exec, JSON envelope) also remain green — the re-wiring did not regress
the existing lint ordering.

## Obligation-by-obligation

1. **Exit-7 refusal of escaping write-scope, ordered before structural
   `Validate` (exit 8).** Confirmed by code inspection of
   `runWorkflowValidate` (`LoadFileUnvalidated` → `RefuseCrossRepoWriteScope`
   exit 7 → `Validate` exit 8 → existing lints → `ForeignRepoReachWarnings`)
   **and** by a binary fixture the author's tests don't cover: a workflow that
   is *both* cross-repo-escaping *and* structurally invalid (missing
   coordinator/lanes/roles) returns **exit 7** with code
   `cross_repo_write_scope` — the refusal genuinely wins the exit-code race
   over `Validate`, not merely fires on an otherwise-valid workflow. Absolute
   paths and parent-escaping relative paths are refused; the structured error
   names the offending job + path and points at decomposition rather than
   silent narrowing.
2. **Interior `..` that nets back inside is NOT falsely refused.** Confirmed:
   `pathReachesOutsideRepoRoot("a/../b")` is false, and an end-to-end binary run
   of a workflow with `allowed_paths: ["a/../b"]` is *not* refused as cross-repo
   — it falls through to structural `Validate`, which rejects it as malformed at
   **exit 8** (`workflow_invalid: invalid write_scope allowed_path`), exactly as
   the design intends. Cross-repo reach is refused, never silently narrowed.
3. **Foreign-repo-slug prompt warning present and non-fatal.**
   `ForeignRepoReachWarnings` flags GitHub owner/repo refs, absolute paths, and
   parent-escaping path tokens in `task_prompt.inline` / `objective` / `title`,
   while a conservative in-repo-top-dir + file-extension allowlist keeps false
   positives near zero on prose citing in-repo paths (verified by
   `...IgnoresInRepoReferences` and `...DoesNotFlagRegisteredRepo`). At the CLI,
   a foreign slug yields exit 0 with `warning:` on stderr and a
   `foreign_repo_reach` entry in `data.warnings` (human + JSON paths both
   asserted).
4. **`secondary_repos` manifest declined.** `DeferredSecondaryReposManifestKey`
   has a name but no reader; `TestSecondaryReposManifestIsNotHonored` proves a
   workflow declaring it gains zero cross-repo write capability — an escaping
   job is still refused with the manifest present, and an in-repo job validates
   identically with or without it.
5. **CLI/validate wiring adapted to current `main`, not a stale verbatim
   copy.** The handoff documents the real drift: the two library files ported
   verbatim (helpers still exist), but `runWorkflowValidate` on current `main`
   has grown two lint stages absent at the quarantine base
   (`RefuseAutonomousSharedCheckoutRepoWrite`, `verifier.EvaluateAllowlistTemplate`).
   The guardrail was inserted into the *current* function shape, preserving the
   required ordering (cross-repo refusal ahead of `Validate`; every lint that
   ran after the validating `LoadFile` still runs after `Validate`). The
   `"path/filepath"` import already present on `main` was correctly not
   re-added. This is a genuine adaptation, confirmed by the diff and by the
   pre-existing lint tests staying green.

## Notes (non-blocking, no action required)

- The change is well-scoped to P0: dispatch-time `scope_violation` terminal
  state, read-only artifact federation, decomposition ergonomics, and the
  first-class multi-repo manifest are explicitly deferred and not implemented
  here, which is correct per RFC 0128 / D196.
- All edits stayed inside the author's declared `write_scope`
  (`go/pkg/workflowauthoring/`, `go/cmd/striatum/`, and the artifact dir).

The work is correct, complete for P0, and ready to land.
