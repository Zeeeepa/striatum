# Fix Recovery Resume Blocker Assertions

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in
`tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py` and your
handoff artifact directory.

The recovery-evidence shard now runs and exposes a stale assertion in
`test_process_adapter_blocker_kinds`. The test still expects the retired
SQLite-era blocker-kind allow-lists. The current daemon/PostgreSQL handler
contract in
`src/striatum/daemon_pg/handlers/recovery_evidence/resume_blocker.py` defines:

- `PROCESS_ADAPTER_BLOCKER_KINDS`: `process_outputs_missing`,
  `process_review_verdict_missing`, `process_exit_nonzero`,
  `process_timeout_exceeded`, and `process_lost_with_outputs_missing`.
- `PROCESS_EXIT_BLOCKER_KINDS`: `process_exit_nonzero` and
  `process_timeout_exceeded`.

Update the test so it verifies the current PG handler contract without hiding
coverage. Keep this as an evidence/contract test; do not change production
handler code unless the test reveals a true handler defect, and do not reopen
production SQLite behavior.

Run the nearest focused test and the recovery-evidence shard if feasible. Note
the exact results in your handoff.
