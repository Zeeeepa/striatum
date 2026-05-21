# Narrow Recovery Evidence Skip

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in `tests/daemon_pg/handlers/recovery_evidence/conftest.py` and
your handoff artifact directory.

Do the bounded follow-up named by the Track 2 regression review:

- remove the broad module-level legacy SQLite skip if present;
- move skipping to only the fixture(s) or paths that truly need the retired
  repo-local SQLite fixture;
- keep active PostgreSQL-only recovery evidence tests runnable;
- run the nearest focused recovery evidence tests and note the exact result.

Do not reopen production SQLite behavior.
