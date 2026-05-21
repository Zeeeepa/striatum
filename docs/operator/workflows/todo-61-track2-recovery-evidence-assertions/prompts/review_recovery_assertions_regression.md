# Review Recovery Assertion Cleanup Regression

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review the recovery-evidence assertion cleanup for regression risk. Verify:

- the two previously failing tests now assert the current daemon/PostgreSQL
  contract rather than a retired SQLite-era subset;
- `tests/daemon_pg/handlers/recovery_evidence -q` passes or any remaining
  failure is clearly outside this bounded scope;
- no active PostgreSQL recovery-evidence coverage was hidden behind new broad
  skips or deselection;
- focused tests named in the handoffs pass.

Use `needs_revision` only for a real regression, hidden-test problem, or
unfixed stale assertion in this bounded scope.
