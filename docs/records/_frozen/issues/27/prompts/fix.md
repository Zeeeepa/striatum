# Implement -- GH #27

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/27/SPEC.md`
- `docs/issues/27/SCOPE.md`
- the source modules named in `SCOPE.md`

## Deliverables

Per `docs/issues/27/SPEC.md` "Acceptance / Definition of done":

1. New PG migration (named in SCOPE) that refines the artifacts append-only trigger to allow updates touching ONLY `(blob_key, blob_sha256, blob_content_type)`.
2. `roles.py` grants column-level UPDATE on those three columns to `striatumd_rw`.
3. `artifact.backfill_blob` works as `striatumd_rw` without trigger disabling.
4. Negative test: UPDATE on any non-blob column still refused with `P0001`.
5. Positive test: UPDATE on only the three blob columns succeeds.
6. `make smoke` and `make pg-test` still pass.

## Constraints

- Stay inside `write_scope.allowed_paths`.
- The migration must be idempotent (`CREATE OR REPLACE FUNCTION`).
- `LATEST_DAEMON_DB_VERSION` in `src/striatum/daemon_pg/migrations.py`
  must be bumped to the new migration's number and the new
  `PgMigration(...)` row added to `MIGRATIONS`.
- The Go daemon's `LATEST_DAEMON_DB_VERSION` (or equivalent — check
  `go/pkg/db/migrations.go`) must also be bumped so the daemon
  binary contract matches.
- Use the exact `author:` line from the work packet in the handoff.

## Handoff

Write `docs/issues/27/build/HANDOFF.md` with `striatum.handoff.v1` front matter. Cite each definition-of-done bullet closed, files changed, tests run, and residual risk.
