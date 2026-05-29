# Implementation & Changes Report — 2026-05-29T12:10:24Z

We have successfully implemented and thoroughly verified robust fixes for all four target issues in the Striatum Go codebase. Below is a detailed record of the changes, the architectural rationale behind our design decisions, and the verification outcomes.

---

## 1. Issue #57: Write-Scope Strictness

### Problem
When completing a job via `work.complete`, write-scope guard validation is performed. The baseline-comparison loop in `gitTouchedPathsSinceBaseline` (inside `go/pkg/mutations/write_scope_guard.go`) flagged any files that were dirty at baseline but had since transitioned back to clean (e.g., due to stash, checkout, or discard) as "touched" paths. If these paths were outside `allowed_paths`, it triggered an unauthorized write-scope violation error, blocking transitions for clean, unaffected files.

### Solution
- **Modified File**: `go/pkg/mutations/write_scope_guard.go`
- **Change**: Deleted the secondary loop (lines 129–133) in `gitTouchedPathsSinceBaseline` which flagged baseline paths absent from `currentByPath` as touched.
- **Rationale**: The primary loop (lines 124–128) already perfectly captures all newly added and modified files (differing hashes or newly untracked). Removing the secondary baseline-exclusive loop prevents clean or stashed files (transitioning from dirty back to clean) from triggering false-positive violations.
- **Regression Test**: Added `TestGitTouchedPathsSinceBaselineAllowsStashedOrRestoredFiles` in `go/pkg/mutations/write_scope_guard_test.go` to explicitly verify that a file dirty at baseline but deleted/restored to clean before completion is successfully ignored.

---

## 2. Issue #58: Duplicate Artifact Publication in `submit-review`

### Problem
`HandleSubmitReview` in `go/pkg/mutations/review.go` invokes `publishArtifact`. If a finding artifact was already published under the same logical name or path, the transaction failed with a database unique key constraint violation (`23505`) and crashed the request instead of proceeding to record the verdict.

### Solution
- **Modified File**: `go/pkg/mutations/review.go`
- **Change**: Updated `HandleSubmitReview` to try publishing the artifact in a transaction. If the transaction fails with a PostgreSQL unique key constraint violation error (code `"23505"`), it:
  1. Catches the error and logs a friendly message: `"Artifact already published under logical name %s, proceeding with verdict recording."`
  2. Commences a fresh transaction block.
  3. Queries the database to lookup the existing artifact ID matching the logical name.
  4. Proceeds to successfully record the review verdict against the existing artifact ID and resolves downstream jobs.
- **Rationale**: This is a robust fallback that preserves database integrity while avoiding duplicate publication crashes, ensuring workflow reviews transition smoothly.
- **Regression Test**: Added `TestSubmitReviewDuplicateArtifactHandling` in `go/pkg/mutations/review_test.go` that mocks duplicate publication insert failures, verifies transaction rollback, catches `"23505"`, retries, resolves the existing ID, and successfully records the verdict.

---

## 3. Issue #59: Strict Front-Matter List Formatting

### Problem
The custom front matter parser in `go/pkg/artifactcontracts/contracts.go` rejected leading spaces/tabs, preventing standard multi-line YAML formatting for lists (like `inputs` in `synthesis` artifacts). In addition, parser errors did not report precise line numbers, and the RPC client silently exited with code 1 instead of exit code 6 when artifact errors were encountered.

### Solution
- **Modified Files**:
  - `go/pkg/artifactcontracts/contracts.go`
  - `go/pkg/cli/rpcclient/client.go`
- **Changes**:
  1. Replaced manual front-matter block parsing in `ParseFrontMatterBlock` with standard `yaml.Unmarshal` using `gopkg.in/yaml.v3` node AST.
  2. Traversed the `yaml.Node` AST in `parseYAMLNode` to manually enforce the duplicate keys constraint, returning a helpful `"declared more than once"` message.
  3. Converted string-only YAML sequence nodes into strongly-typed `[]string` slices so that kind-specific validations (like `lane_shapes` type assertions) continue to pass.
  4. Extracted the syntax error line number from `yaml.Unmarshal` errors (using a `"line (\d+):"` match) and adjusted it (+1) relative to the Markdown file context to report precise line numbers.
  5. Added an explicit case for `"artifact_error"` inside `exitCode()` in `client.go` to return exit-code 6.
- **Regression Tests**: Added `TestParseFrontMatterAllowsMultilineLists` and `TestParseFrontMatterReturnsLineNumberedSyntaxErrors` in `go/pkg/artifactcontracts/contracts_test.go` to verify multi-line list support and line-numbered syntax error reporting.

---

## 4. Issue #60: Rigid Session Lifetime Enforcement

### Problem
Starting a new session when an active session already exists on the same lane for a run causes manual unregister blocks because the old session still owns active leases, preventing the new session from claiming jobs.

### Solution
- **Modified File**: `go/pkg/mutations/lifecycle.go`
- **Change**: In `HandleRegisterSession`, we query if there is already an active session for the `(repository_id, run_id, role_id, lane_id)`. If an active session exists, we automatically:
  1. Update its state to `'closed'` in `striatumd.sessions` and log a `'session.closed'` transition event.
  2. Query all active leases owned by the old session.
  3. Release these leases (transition state to `'released'` in `striatumd.leases` and log a `'lease.released'` event).
  4. Reset any corresponding jobs to `'queued'` (setting `current_message_id = NULL` and `current_lease_id = NULL`) and reset queue messages to `'pending'` so they can immediately be claimed by the new active session.
- **Rationale**: Automated supersession prevents lane locks and completely avoids manual unregister blockages, creating a seamless handover between worker instances.
- **Regression Test**: Added `TestRegisterSessionAutomatedSupersession` in `go/pkg/mutations/lifecycle_test.go` that verifies full integration of automatic closing, lease release, job re-queuing, and queue message resets on a live PostgreSQL database.

---

## Verification Summary

All verification command outputs compiled and ran perfectly. Below is the final execution attestation:

- **Vetting**: `go vet ./...` completed with zero errors.
- **Full Test Suite (with live PostgreSQL and race detection)**: `STRIATUM_PG_TEST_URL="postgres:///postgres" go test -p 1 -race ./...` succeeded with 100% PASS rate.
