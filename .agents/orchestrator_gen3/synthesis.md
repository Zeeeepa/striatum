# Synthesis & Implementation Strategy (Gen 3)

We have synthesized the comprehensive codebase exploration and triage reports from all three parallel Explorer subagents. Below is the unified, step-by-step implementation strategy for all six outstanding GitHub issues.

---

## 1. Issue #57: Write-Scope Strictness (Explorer 1 Synthesis)

### Problem
When a job completes (`work.complete`), write-scope guard validation is triggered. The helper `gitTouchedPathsSinceBaseline` in `go/pkg/mutations/write_scope_guard.go` flags any file that was dirty in the baseline but is no longer dirty (i.e. transitions from dirty to clean due to stash or revert) as a touched path. If this file resides outside `allowed_paths`, it raises an unauthorized write-scope violation.

### Solution
Remove the secondary loop in `gitTouchedPathsSinceBaseline` that loops over `baseline` and flags files absent from `currentByPath`.
- **File**: `go/pkg/mutations/write_scope_guard.go`
- **Action**: Delete lines 129–133:
  ```go
  for path, baselineHash := range baseline {
      if _, ok := currentByPath[path]; !ok && baselineHash != "" {
          touched = append(touched, path)
      }
  }
  ```
- **Rationale**: The first loop (lines 124–128) already captures all newly added files and all modified files (dirty or differing hash). Removing this secondary loop ensures that files which transitioned from dirty to clean (restored to baseline state) are not considered "touched" and thus do not trigger false write-scope violations.

---

## 2. Issue #58: Duplicate Artifact Publication in `submit-review` (Explorer 2 Synthesis)

### Problem
`HandleSubmitReview` in `go/pkg/mutations/review.go` invokes `publishArtifact`. If a finding artifact is already published under the same logical name or path, the database returns a PostgreSQL `unique_violation` (code `23505`). The transaction rolls back and crashes the request instead of proceeding with the verdict.

### Solution
Catch the unique key violation, query the existing artifact ID using the unique fields, log a user-friendly message, and proceed with recording the verdict.
- **File**: `go/pkg/mutations/review.go`
- **Action**: In `HandleSubmitReview`, wrap `publishArtifact` in a try/catch-like error check for unique violations.
  - If a unique violation error occurs (using `errors.As(err, &pgErr) && pgErr.Code == "23505"`), look up the existing artifact's ID.
  - The lookup can query `striatumd.artifacts` using `repository_id`, `run_id`, `job_id`, and `logical_name`.
  - Log a user-friendly message (e.g. `"Artifact already published under logical name %s, proceeding with verdict recording."`).
  - Use the retrieved existing `artifact_id` to call `recordVerdict`.
  - Ensure the database transaction is NOT rolled back (or perform the lookup and verdict record in the same transaction block).

---

## 3. Issue #59: Strict Front-Matter List Formatting (Explorer 1 Synthesis)

### Problem
Front-matter parsing in `pkg/artifactcontracts/contracts.go` rejects any leading spaces or tabs, preventing standard YAML formatting for multi-line lists (such as `inputs`). Additionally, schema failures or invalid formats trigger a silent exit-code 6 omission (exiting with 1 instead of 6), and syntax errors do not report precise line numbers.

### Solution
1. **YAML Parsing**: Update `ParseFrontMatterBlock` in `go/pkg/artifactcontracts/contracts.go` to use standard `yaml.Unmarshal` instead of manual line splits.
   - Standard `yaml.Unmarshal([]byte(block), &result)` handles multi-line lists natively.
   - If a duplicate key error is encountered, translate it from YAML's default `"already defined"` to `"declared more than once"` to keep compatibility with existing tests.
2. **Line Number Reporting**: Parse the line number from the YAML error (e.g. extracts `X` from `line X:`) and adjust it (+1) relative to the original Markdown file to return precise line-number syntax errors.
3. **Exit Code Mapping**: In `go/pkg/cli/rpcclient/client.go` inside `exitCode()`, add a explicit case for `"artifact_error"` (or check if it is `artifact_error` and return `6`):
   ```go
   case "artifact_error":
       return 6
   ```

---

## 4. Issue #60: Rigid Session Lifetime Enforcement (Explorer 2 Synthesis)

### Problem
Starting a new session when an active session already exists on the same lane for a run causes manual unregister blocks because the old session still owns active leases, preventing the new session from claiming jobs.

### Solution
Implement automated supersession in `HandleRegisterSession` inside `go/pkg/mutations/lifecycle.go`:
- When registering a new session on `(repository_id, run_id, role_id, lane_id)`, query if there is already an active session.
- If an active session exists, automatically:
  1. Update its state to `'closed'` in `striatumd.sessions`, logging a `'session.closed'` transition.
  2. Query all active leases owned by the old session.
  3. Release these leases (transition state to `'released'` in `striatumd.leases`, logging `'lease.released'`).
  4. Reset any corresponding jobs to `'queued'` and queue messages to `'pending'` in the database so they can be immediately claimed by the new session.

---

## 5. Issues #49 & #54: PTY Supervision, Rebridge & Re-queueing (Explorer 3 Synthesis)

### Issue #49: Re-queued Packet Resume
- **Problem**: When a job is re-queued after checkpoint resolution (e.g. `checkpoint.resolve` with `continue`), the session attempts to claim it in `HandleClaimNext`. However, if `fresh_session_required` is `true`, the `NOT EXISTS` query blocks the claim because a work packet for this same run and session ID already exists in `striatumd.work_packets`.
- **Solution**: In `go/pkg/mutations/claim.go`, relax the `NOT EXISTS` query by appending `AND wp.job_id != qm.job_id`:
  ```sql
  AND (
    j.fresh_session_required = false
    OR NOT EXISTS (
      SELECT 1 FROM striatumd.work_packets wp
       WHERE wp.repository_id = qm.repository_id
         AND wp.run_id = qm.run_id
         AND wp.session_id = $4
         AND wp.job_id != qm.job_id
    )
  )
  ```
  This permits a session to reclaim and resume its own job while still preventing it from claiming other different jobs in the same run.

### Issue #54: Supervision Rebridge & Status Details
- **Problem**: In `HandleSuperviseRebridge`, the background delivery bridge process (`striatum-supervisor-helper`) is launched and recorded under `helper_pid` in pointer metadata. If the helper dies, the bridge fails silently. Standard `lanehealth.Check` and status projections only probe the main PTY process PID, classifying the lane as healthy.
- **Solution**:
  - Update `lanehealth.Check` and `Facts` to parse `helper_pid` and `helper_pid_start_time` from pointer metadata.
  - Signal-probe (`syscall.Kill(helperPID, 0)`) the helper process.
  - If the helper process is dead, transition `f.DeliveryDegraded = true` with reason `"helper_process_gone"`.
  - Ensure standard status projections in `go/pkg/reads/supervision.go` reflect this degraded status under lane liveness and attestation details.

---

## 6. Verification Plan
- For each issue, regression unit/integration tests must be implemented.
- The full Go test suite (`go test -race ./...`) must compile and pass cleanly.
- The automated retired vocabulary grep gate check must be verified.
