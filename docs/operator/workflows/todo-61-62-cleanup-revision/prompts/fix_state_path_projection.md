# Fix Repo State-Path Projection Divergence

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Fix only regression-review F1 from
`docs/operator/artifacts/todo-61-62-cleanup/review/regression/REVIEW.md`:
`striatum repo list --json` and related Python repository projections must not
return stale `.striatum/state.sqlite3` file paths for older rows when
PostgreSQL live state is authoritative and `.striatum/` is operational
scratch.

Expected implementation shape:

- centralize or reuse normalization in `src/striatum/daemon_pg/repositories.py`,
- normalize stale `state_db_path` values ending in `state.sqlite3` to the
  `.striatum/` directory for repo list/resolve outputs,
- keep actual database storage and migration history intact,
- add focused tests that cover stale-row output normalization,
- update the command authority matrix only if the check id or projection
  meaning needs a user-visible note.

Do not implement Track 2/Track 3 follow-ups from the review and do not decide
TODO 55, 56, 59, or 60.
