# Exploration Report — Issue #58 & Issue #60

This report presents a read-only investigation and detailed implementation strategy for resolving **Issue #58** (Duplicate Artifact Publication in `submit-review`) and **Issue #60** (Rigid Session Lifetime Enforcement) in the Striatum repository.

---

## 1. Issue #58: Duplicate Artifact Publication in `submit-review`

### Problem Statement
When a user or agent invokes the `submit-review` verb, the handler calls `publishArtifact` and `recordVerdict` sequentially in a database transaction. If the finding artifact has already been published during the run (e.g. manually via `artifact.publish`), `publishArtifact` attempts to insert the duplicate row into the `striatumd.artifacts` table.

This violates PostgreSQL unique constraints:
1. `UNIQUE (repository_id, run_id, job_id, logical_name)` — if the same logical name is published under the same job.
2. `UNIQUE (repository_id, run_id, repo_path, content_sha256)` — if the exact same file and content hash have already been published under any job or name in the run.

Consequently, a raw `*pgconn.PgError` with SQL State `23505` (unique_violation) is thrown. The transaction rolls back, crashing the execution rather than proceeding with the verdict.

### Code Locations Involved
- **Handler**: `HandleSubmitReview` in `go/pkg/mutations/review.go` (lines 33–97).
- **Artifact Publisher**: `publishArtifact` in `go/pkg/mutations/artifact.go` (lines 42–215).
- **Postgres Unique Constraints**: Defined in `go/pkg/db/sql/0005_repo_local_workflow_state.sql` (lines 231–232).

### Proposed Fix / Reconcile Strategy
In `HandleSubmitReview`, we should intercept the `23505` unique key violation when calling `publishArtifact`.
1. Use `errors.As(err, &pgErr)` to check if the error code is `"23505"`.
2. Log a user-friendly message using the `"log"` package:
   ```go
   log.Printf("Finding artifact %q has already been published (duplicate constraint triggered). Proceeding with recording the verdict.", pathText)
   ```
3. Look up the existing artifact's ID in the `striatumd.artifacts` table using the unique constraint criteria.
4. Set the `artifact["artifact_id"]` to the existing ID and proceed directly to call `recordVerdict` instead of rolling back or crashing.

---

## 2. Issue #60: Rigid Session Lifetime Enforcement

### Problem Statement
When a new agent session is registered via `session.register` (`register-session` CLI command), it does not clean up or terminate existing active sessions for the same role and lane in the current run. Instead, it increments the session's `ordinal` and marks the new session as `active`. The previous session also remains `active` in the DB.

This causes "rigid session lifetime enforcement" and "manual unregister blocks", where stale active sessions are left dangling. These stale sessions can block workflow jobs because their active leases remain active and are not released, forcing manual operator interventions.

### Code Locations Involved
- **Registration Handler**: `HandleRegisterSession` in `go/pkg/mutations/lifecycle.go` (lines 16–165).
- **Lease Verification**: `activeLeaseFor` in `go/pkg/mutations/mutations.go` (lines 283–301).
- **Session Close Handler**: `HandleCloseSession` in `go/pkg/mutations/lifecycle.go` (lines 194–250).

### Proposed Fix / Reconcile Strategy
Introduce automated logic inside `HandleRegisterSession` (and optionally `resolveOverrideSession` in `go/pkg/mutations/review.go`) to terminate and supersede any existing duplicate active sessions on the same lane and role for the current run.

1. **New Flag/Param**: Support an optional boolean parameter `replace_active` (which can be passed via `--replace-active` flag in CLI/MCP) in `register_session` parameters.
2. **Automated Replacement Logic**:
   - Query all active sessions on the same `(repository_id, run_id, role_id, lane_id)` using `SELECT ... FOR UPDATE`.
   - For each active session found:
     - Query and release all active leases owned by the old session.
     - For each active lease:
       - Update `striatumd.leases` state to `'released'` with reason `'superseded'`.
       - Reset the corresponding job state to `'queued'` and `current_lease_id` to `NULL`.
       - Reset the corresponding `queue_messages` state to `'pending'` and `current_lease_id` to `NULL`.
       - Emit `lease.released` event to event log.
     - Update the old session state in `striatumd.sessions` to `'closed'` with reason `'superseded'`.
     - Emit `session.closed` event with source `'automated'`.
