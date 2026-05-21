# Fix Go Repo State-Path Projection

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Fix only the remaining production Go side of cleanup F1:

- `striatum repo list --json` is daemon-backed by the Go repository service
  and still returns stale `.striatum/state.sqlite3` file paths for older
  repository rows.
- The already-completed Python revision normalized Python projections, but it
  did not change the live Go `repo.list`/`repo.resolve`/already-registered
  `repo.add` output.

Expected implementation shape:

- normalize stale `state_db_path` values ending in `state.sqlite3` to the
  `.striatum/` operational scratch directory at Go response-projection time;
- keep actual database storage and migration history intact;
- cover `repo.list`, `repo.resolve`, and already-registered `repo.add` outputs
  where they expose repository metadata;
- add focused Go tests for stale-row output normalization;
- run the smallest useful Go test target, and note the exact command/result in
  the handoff artifact.

Do not implement Track 2/Track 3 follow-ups from the cleanup review and do not
decide TODO 55, 56, 59, or 60.
