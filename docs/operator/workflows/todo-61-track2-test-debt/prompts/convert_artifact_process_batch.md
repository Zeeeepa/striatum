# Convert Artifact And Process Batch

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in `tests/test_artifact_schemas.py`,
`tests/test_process_adapter.py`, and your handoff artifact directory.

Reduce TODO 61 Track 2 legacy SQLite test debt in these files:

- remove module-level skips that hide all tests only because the legacy SQLite
  package was deleted;
- convert tests that assert current artifact schema or process-adapter behavior
  to daemon/PostgreSQL harnesses where practical;
- quarantine only genuinely historical SQLite fixture assertions;
- do not restore `src/striatum/legacy_sqlite` or add production SQLite
  fallback paths;
- run focused tests for the touched files or clearly explain blockers.

Do not edit service fallback policy, architecture guardrails, or other test
files from this job.
