# Plan TODO 62 PG-Only Guardrail Cleanup

Produce the expected synthesis artifact only. Do not edit source in this job.

Focus on daemon-global PostgreSQL-only surfaces and guardrail residuals. The
artifact must:

- distinguish legacy SQLite file refusal/diagnostics from live SQLite
  authority;
- identify stale registry or `.striatum/state.sqlite3` references that still
  need wording or test cleanup;
- list files and tests that should remain as migration-refusal coverage;
- define the smallest cleanup batch with disjoint write scope.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
