# Convert Service Batch

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in `tests/test_service.py` and your handoff artifact directory.

Reduce TODO 61 Track 2 legacy SQLite test debt in this file:

- remove or quarantine direct imports of `striatum.legacy_sqlite`,
  `striatum.db`, and `striatum.migrations` where practical;
- keep production service invocations daemon-routed;
- keep any historical legacy SQLite fixture behavior explicitly named and
  guarded by the existing legacy fixture environment, not production defaults;
- do not add broad module-level skips that hide unrelated tests;
- run focused tests for this file or clearly explain blockers.

Do not edit service fallback policy, architecture guardrails, or other test
files from this job.
