## 2026-05-29T08:11:52Z

Address and resolve all PostgreSQL integration test failures and mock setup desynchronizations reported during Victory Audit. All tests must pass cleanly when executed under a live PostgreSQL database environment.

### Target Tasks:

1. **Fix `go/pkg/lanehealth/integration_test.go`**:
   - Resolve column mismatches and satisfy PostgreSQL NOT NULL constraints in the `INSERT` statements.
   - For `process_supervisors` insert (around lines 60-68):
     - Replace `last_heartbeat_at` with the actual column name `heartbeat_at`.
     - In the insert statement, also specify values for NOT NULL columns `adapter`, `command_json`, `cwd`, and `scratch_path`.
     - Set them to: `adapter` = `'codex'`, `command_json` = `'[]'::jsonb`, `cwd` = `'/tmp'`, `scratch_path` = `'/tmp/scratch'`.
   - For `process_supervisor_pointers` insert (around lines 71-79):
     - Replace `last_heartbeat_at` with the actual column name `updated_at`.
     - Ensure matching NOT NULL columns are provided.
   - For `daemon_supervisors` insert (around lines 81-89):
     - Replace `last_heartbeat_at` with `heartbeat_at` and `registered_at` with `started_at` (actual column names).
     - In the insert statement, also specify values for NOT NULL columns: `repo_supervisor_id` = `'sup_lh_001'`, `daemon_instance_id` = `'inst'`, `command_json` = `'[]'::jsonb`, `command_sha256` = `'sha'`, and `cwd` = `'/tmp'`.

2. **Fix `go/pkg/mutations/artifact_integration_test.go`**:
   - In `TestPublishArtifactUsesLaneAttestedAuthorLine`, the session (`sess_1`) is evaluated as unattested because it lacks pointer and daemon supervisor table rows, causing the author line verification check to fail.
   - Insert accompanying `process_supervisor_pointers` and `daemon_supervisors` rows right after the `process_supervisors` insert statement (around lines 94-101) to satisfy `lanehealth.Checker` attestation:
     - For `process_supervisor_pointers`:
       ```sql
       INSERT INTO striatumd.process_supervisor_pointers (
         repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
         pid, pid_start_time, state, updated_at, metadata_json
       ) VALUES ('repo_artifact', 'sup_1', 'dsup_1', 'run_1', 'sess_1', 4242, '', 'attached', $1, '{}'::jsonb)
       ```
     - For `daemon_supervisors`:
       ```sql
       INSERT INTO striatumd.daemon_supervisors (
         daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
         daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
         pid_start_time, state, started_at, heartbeat_at
       ) VALUES ('dsup_1', 'repo_artifact', 'run_1', 'sess_1', 'sup_1', 'inst', 'codex', '[]'::jsonb, 'sha', '/tmp', 4242, '', 'attached', $1, $1)
       ```

3. **Fix `go/pkg/mutations/interrogation_test.go`**:
   - In `intgAttest` (around lines 76-87), insert accompanying `process_supervisor_pointers` and `daemon_supervisors` rows so that the target session is classified as attested, preventing the interrogation check from failing.
   - Use these exact SQL statements:
     ```go
     dsupID := "dsup_" + sessionID
     supID := "sup_" + sessionID
     if err := runner.Exec(ctx, `
         INSERT INTO striatumd.process_supervisor_pointers (
           repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
           pid, pid_start_time, state, updated_at, metadata_json
         ) VALUES ($1,$2,$3,$4,$5,4242,'','attached',$6,'{}'::jsonb)`,
         repoID, supID, dsupID, runID, sessionID, now); err != nil {
         t.Fatalf("attest pointer for %s: %v", sessionID, err)
     }
     if err := runner.Exec(ctx, `
         INSERT INTO striatumd.daemon_supervisors (
           daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
           daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
           pid_start_time, state, started_at, heartbeat_at
         ) VALUES ($1,$2,$3,$4,$5,'inst',$6,'[]'::jsonb,'sha','/tmp',4242,'','attached',$7,$7)`,
         dsupID, repoID, runID, sessionID, supID, lane, now); err != nil {
         t.Fatalf("attest daemon supervisor for %s: %v", sessionID, err)
     }
     ```

### Verification Requirement:
- You MUST run the entire Go test suite against a live PostgreSQL database:
  ```bash
  STRIATUM_PG_TEST_URL="postgres:///postgres" go test -p 1 -race ./...
  ```
- Verify that ALL tests pass cleanly with zero failures and zero race conditions under live Postgres database.
