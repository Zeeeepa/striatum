# GH #27 — artifacts_no_update trigger should allow blob_* column updates (RFC 0072 follow-up)

Source: https://github.com/halbritt/striatum/issues/27

## Summary

The append-only trigger `artifacts_no_update` on `striatumd.artifacts` (PG migration 0001) refuses ALL updates via `striatumd.refuse_repo_append_only_change()`. The trigger predates RFC 0072 migration 0009, which added `(blob_key, blob_sha256, blob_content_type)` columns that aren't part of the artifact's immutable identity (`content_sha256`, `repo_path`, `kind`, `byline` are).

This makes the new `artifact.backfill_blob` RPC (commit `363969a`) require an out-of-band `ALTER TABLE striatumd.artifacts DISABLE TRIGGER artifacts_no_update` by the database owner before the backfill, and a re-enable after. Fragile, and creates a window where genuine append-only violations could slip through.

```
# As striatumd_rw:
UPDATE striatumd.artifacts
   SET blob_key = '...', blob_sha256 = '...', blob_content_type = '...'
 WHERE artifact_id = 'art_...';
ERROR:  repo-local append-only rows cannot be updated or deleted (SQLSTATE P0001)
```

## Acceptance / Definition of done

1. A new PG migration (e.g. 0010) refines `striatumd.refuse_repo_append_only_change` (or replaces the `artifacts` trigger with a column-aware variant) so that UPDATEs that touch ONLY `(blob_key, blob_sha256, blob_content_type)` are allowed, and UPDATEs that touch any other column are still refused with the existing error.
2. A column-level `GRANT UPDATE (blob_key, blob_sha256, blob_content_type) ON striatumd.artifacts TO striatumd_rw` is sufficient (no table-wide UPDATE for `striatumd_rw`).
3. `artifact.backfill_blob` works for `striatumd_rw` without trigger disabling.
4. **Negative test**: UPDATE that touches any non-blob column (`content_sha256`, `repo_path`, `artifact_kind`, etc.) is still refused with `P0001`.
5. **Positive test**: UPDATE that touches only the three blob columns succeeds.
6. The `roles.py` grant list is updated to include the new column-level UPDATE so `striatum daemon doctor --provision-rw-role --repair-grants` keeps a freshly-provisioned daemon aligned.
7. `make smoke` and `make pg-test` still pass.

## Suggested fix (proposals; pick one)

1. **Column-aware trigger**: replace `artifacts_no_update` (or the `refuse_repo_append_only_change` function it calls) with a trigger function that compares OLD/NEW row-by-column for all non-blob columns and raises `P0001` only when any of those differs. Smallest change, table-local.
2. **Allow-list function**: keep the generic refuse function but consult a per-table allow-list of mutable columns. More general; useful if other append-only tables need similar exceptions later. Larger surface.

(1) is simpler and sufficient for V1; defer (2) until a second exception emerges.

## Provenance

Diagnosed while shipping `artifact.backfill_blob` in commit `363969a` (RFC 0072 follow-up). The migration of the two SCOPE artifacts for GH #22 / #23 required `ALTER TABLE ... DISABLE TRIGGER` as a one-shot. This issue closes that fragile path.
