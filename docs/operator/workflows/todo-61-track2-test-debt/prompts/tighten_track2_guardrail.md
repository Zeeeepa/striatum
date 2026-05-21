# Tighten Track 2 Guardrail

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in `tests/architecture/test_legacy_sqlite_quarantine.py` and your
handoff artifact directory.

After the first Track 2 implementation batch, update the architecture guardrail
so the completed batch remains enforced:

- fail if the primary batch files still import `striatum.legacy_sqlite`,
  `striatum.db`, or `striatum.migrations`, unless the import is in an
  explicitly quarantined historical fixture test named in the guardrail;
- fail if those files use a broad module-level skip to hide the deleted legacy
  SQLite package;
- keep residual legacy SQLite fixture imports outside this batch visible as
  future work rather than silently allowing everything under `tests/`;
- do not decide or encode policy for TODO 55, 56, 59, or 60.

Run the architecture guardrail test and note the result in the handoff.
