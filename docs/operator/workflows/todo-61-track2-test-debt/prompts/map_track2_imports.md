# Map Track 2 Imports

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Map the first TODO 61 Track 2 test-debt batch. Produce a concise artifact that:

- lists current `striatum.legacy_sqlite`, `striatum.db`, and
  `striatum.migrations` imports in:
  - `tests/test_cli_mvp.py`
  - `tests/test_service.py`
  - `tests/test_artifact_schemas.py`
  - `tests/test_process_adapter.py`
- identifies module-level skip patterns in those files that are only hiding
  the deleted legacy SQLite package;
- suggests whether each import should be converted to daemon/PostgreSQL
  harness coverage or quarantined as explicit historical fixture coverage;
- names focused tests that each implementation batch should run.

Do not edit source or tests in this job.
