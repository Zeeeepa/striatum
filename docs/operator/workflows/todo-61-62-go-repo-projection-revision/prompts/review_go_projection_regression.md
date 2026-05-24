# Review Go Projection Regression Fix

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review only the Go projection revision. Verify:

- daemon-backed `repo.list` no longer exposes stale `.striatum/retired-local-state`
  paths for old rows;
- `repo.resolve` and already-registered `repo.add` use the same projection
  rule where they expose repository metadata;
- focused Go tests cover the stale-row regression;
- the implementation does not rewrite stored database values as part of the
  projection fix.

Use `needs_revision` only for a remaining correctness or coverage problem in
this Go projection scope.
