# Triage -- GH #27 scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/27/SPEC.md`
2. `src/striatum/daemon_pg/sql/0001_baseline.sql` — find the
   `striatumd.refuse_repo_append_only_change` function definition and
   the `artifacts_no_update` trigger registration.
3. `src/striatum/daemon_pg/sql/0009_blob_storage.sql` — confirms the
   blob columns added to `striatumd.artifacts`.
4. `src/striatum/daemon_pg/roles.py` — the role-grant manifest.
   Specifically the `GRANT SELECT, INSERT` on `striatumd.artifacts` and
   the column-grant pattern.
5. `go/pkg/mutations/artifact_backfill_blob.go` — the consumer that
   needs the trigger refinement to land.
6. Existing migrations under `src/striatum/daemon_pg/sql/0002...0009`
   to understand the migration pattern (single SQL file, idempotent
   guards where possible).

## Output

Write `docs/issues/27/SCOPE.md` with `striatum.synthesis.v1` front matter
and the exact `author:` line from the work packet. Include:

- the chosen approach (column-aware trigger vs. allow-list function)
  with justification;
- the exact new migration filename and number (`0010_<short_slug>.sql`);
- the exact files in scope (the new migration, `roles.py` grant
  update, tests under `tests/test_daemon_pg.py` or similar);
- the exact files out of scope (do NOT touch unrelated migrations,
  do NOT widen the audit_log or events triggers — only `artifacts`);
- an acceptance checklist with one numbered check per bullet in
  `docs/issues/27/SPEC.md`;
- verification commands (`make pg-test`, a manual UPDATE round-trip
  proving both the positive and negative cases);
- risks: schema migrations are forward-only; a malformed trigger
  function could lock out all writes until rolled back manually.
  Recommend the migration include an idempotent `CREATE OR REPLACE
  FUNCTION` so a re-run is safe.
