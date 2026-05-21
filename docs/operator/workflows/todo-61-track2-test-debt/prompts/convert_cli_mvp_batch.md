# Convert CLI MVP Batch

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in `tests/test_cli_mvp.py` and your handoff artifact directory.

Reduce TODO 61 Track 2 legacy SQLite test debt in this file:

- remove or quarantine direct imports of `striatum.legacy_sqlite`,
  `striatum.db`, and `striatum.migrations` where practical;
- prefer daemon/PostgreSQL harness helpers for live workflow behavior;
- if a test is purely historical SQLite fixture coverage, keep that explicit
  and bounded rather than restoring production SQLite authority;
- do not add broad module-level skips that hide unrelated tests;
- run focused tests for this file or clearly explain blockers.

Do not edit service fallback policy, architecture guardrails, or other test
files from this job.
