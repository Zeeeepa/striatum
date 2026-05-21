# Review Repo State-Path Projection Fix

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review only the F1 revision. Verify:

- `striatum repo list --json` no longer exposes stale
  `.striatum/state.sqlite3` paths for old rows,
- doctor and MCP projections remain consistent,
- no production SQLite or Python-daemon authority path was reopened,
- focused tests cover the regression.

Use `needs_revision` only for a remaining F1 correctness or coverage problem.
