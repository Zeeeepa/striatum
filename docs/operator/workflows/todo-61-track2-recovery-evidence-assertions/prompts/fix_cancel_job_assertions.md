# Fix Recovery Cancel Job Assertions

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py`
and your handoff artifact directory.

The recovery-evidence shard now runs and exposes a stale assertion in
`test_cancelable_states`. The test still expects the retired SQLite-era subset
`pending`, `claimed`, and `blocked`, while the current daemon/PostgreSQL
handler contract in
`src/striatum/daemon_pg/handlers/recovery_evidence/cancel_job.py` defines the
cancelable states as `blocked`, `queued`, `claimed`, `running`, `stale_lease`,
and `waiting_human`.

Update the test so it verifies the current PG handler contract without hiding
coverage. Keep this as an evidence/contract test; do not change production
handler code unless the test reveals a true handler defect, and do not reopen
production SQLite behavior.

Run the nearest focused test and the recovery-evidence shard if feasible. Note
the exact results in your handoff.
