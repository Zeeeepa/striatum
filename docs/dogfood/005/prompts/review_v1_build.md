# Review V1 Build

Use fresh context, repo-level access. Inspect the changed source,
tests, docs, fixture (if any), and `BUILD_HANDOFF.md`.

Focus on:

- diagnostic envelope contents — runtime check that no child
  stdout/stderr leaks in;
- blocker rows: each failure path inserts a row with the synthesis-
  pinned reason and an envelope payload that matches the schema;
- `--timeout-seconds` actually SIGTERMs the child and falls back
  to SIGKILL after 5s; `process_executions.state` becomes
  `'timed_out'`;
- `recovery process-reconcile` correctly identifies dead PIDs
  (`os.kill(pid, 0)` raises `ProcessLookupError`) and re-runs
  output validation;
- the two doctor checks fire correctly under the conditions named;
- `striatum status --run-id` JSON envelope grew the
  `process_health` key without breaking existing callers;
- schema impact: if `process_executions.state` got new values,
  there's a migration; if not, the column was already accepting
  string values without a CHECK;
- D028 (no transcripts) and D020 (no hosted services) preserved;
- `make lint`, `make typecheck`, `make test` clean on the
  implementer's branch;
- the issue #1 reproduction shape from RFC 0014 actually works
  (a fixture or test that runs an adapter that exits 0 without
  publishing the artifact reaches the blocked state with the
  pinned reason).

Write `docs/dogfood/005/review/build/BUILD_REVIEW.md` as a
`finding` artifact and submit a structured verdict. Use
`needs_revision` for issues that must be fixed before the run
can finish.
