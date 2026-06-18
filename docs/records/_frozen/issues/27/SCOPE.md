---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/27/SPEC.md", "docs/rfcs/0072-blob-backed-artifact-storage.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/POSTGRES_TRANSITION.md", "docs/SPEC.md", "docs/INDEX.md", "AGENTS.md", "src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql", "src/striatum/daemon_pg/sql/0009_blob_storage.sql", "src/striatum/daemon_pg/roles.py", "go/pkg/mutations/artifact_backfill_blob.go"]
---

author: triager-unknown-model-001

# GH #27 -- Scope

Bound scope for GH #27, "artifacts_no_update trigger should allow blob_*
column updates." The implementation job should let the RFC 0072
`artifact.backfill_blob` path update only blob-reference columns on
`striatumd.artifacts` while preserving append-only protection for every other
artifact column and for the unrelated append-only tables.

## Chosen Approach

Use a **column-aware artifacts trigger**, not a generic allow-list framework.

The existing append-only trigger function
`striatumd.refuse_repo_append_only_change()` is shared by
`striatumd.events` and `striatumd.artifacts`, and the current `events`
invariant still needs unconditional update/delete refusal. The cleanest V1
change is to leave that generic function and the `events_*` / delete triggers
alone, then replace only the `artifacts_no_update` trigger with an
artifact-specific function that compares `OLD` and `NEW` row values after
removing the mutable blob fields. If any non-blob field differs, it raises the
existing `repo-local append-only rows cannot be updated or deleted` exception;
if only `blob_key`, `blob_sha256`, and/or `blob_content_type` differ, it
returns `NEW`.

This is smaller and safer than adding a reusable allow-list registry before a
second append-only exception exists. It also avoids widening the authority of
`audit_log` or `events`, which are not part of GH #27.

## Migration

Add migration:

- `0010_artifact_blob_update_trigger.sql`

The SQL should be forward-only and idempotent where possible:

- `CREATE OR REPLACE FUNCTION striatumd.allow_artifact_blob_reference_update()`
  or an equivalently named table-local trigger function.
- `DROP TRIGGER IF EXISTS artifacts_no_update ON striatumd.artifacts`.
- `CREATE TRIGGER artifacts_no_update BEFORE UPDATE ON striatumd.artifacts FOR EACH ROW EXECUTE FUNCTION ...`.
- A guarded grant block that gives `striatumd_rw` exactly
  `GRANT UPDATE (blob_key, blob_sha256, blob_content_type) ON striatumd.artifacts`
  when that role exists.

Use `IS DISTINCT FROM` semantics, either explicitly per column or by comparing
`to_jsonb(NEW) - ARRAY['blob_key','blob_sha256','blob_content_type']` against
the same expression for `OLD`, so NULL transitions are handled correctly.

## Files In Scope

- `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql` -- new
  migration containing the artifacts-only update trigger replacement and
  guarded column-level grant.
- `src/striatum/daemon_pg/migrations.py` -- bump
  `LATEST_DAEMON_DB_VERSION` to `10` and register the new migration label.
- `go/pkg/db/sql/0010_artifact_blob_update_trigger.sql` -- matching Go-embedded
  SQL copy, because the production Go daemon embeds migrations and rejects a
  source tree with newer Python SQL.
- `go/pkg/db/migrations.go` -- bump `LatestDaemonDBVersion` to `10` and add
  the migration label.
- `src/striatum/daemon_pg/roles.py` -- after revoking table-wide
  `UPDATE, DELETE` on `striatumd.artifacts`, grant
  `UPDATE (blob_key, blob_sha256, blob_content_type)` on that table; update
  `role_repair_sql()` the same way so `daemon doctor --repair-grants` keeps
  fresh installs aligned.
- Focused tests under `tests/daemon_pg/` or `tests/test_daemon_pg.py`; the
  most natural home is a new `tests/daemon_pg/test_migration_0010_artifact_blob_update.py`
  plus updates to `tests/daemon_pg/test_append_only_role_grants.py` and
  `tests/test_daemon_pg.py` for role-repair SQL/version assertions.
- `docs/issues/27/build/HANDOFF.md` and `docs/issues/27/review/REVIEW.md` are
  later workflow artifacts, not part of this triage job.

## Files Out Of Scope

- Do not edit or rewrite historical migrations `0001` through `0009`; add a
  forward migration only.
- Do not widen or replace `audit_log` triggers, `events_no_update`,
  `events_no_delete`, or `artifacts_no_delete`.
- Do not grant table-wide `UPDATE` on `striatumd.artifacts` to
  `striatumd_rw`.
- Do not change `go/pkg/mutations/artifact_backfill_blob.go` unless a focused
  test exposes a direct SQL parameter bug unrelated to the trigger.
- Do not change daemon RPC contracts, MCP method registration, blob key
  derivation, S3 client behavior, web UI artifact rendering, corpus export,
  historical dogfood migration behavior, or repository adoption flow.
- Do not touch legacy SQLite modules, `.striatum/`, `.venv/`, caches, build
  output, transcripts, or private diagnostics.

## Acceptance Checklist

Each item maps 1:1 to `docs/issues/27/SPEC.md`.

1. **GH27-1 (Trigger permits only blob columns).** Migration 0010 refines the
   artifacts update trigger so updates touching only `blob_key`,
   `blob_sha256`, and `blob_content_type` succeed, while updates touching any
   other artifact column still raise the existing append-only exception.
2. **GH27-2 (Column-level grant only).** `striatumd_rw` has no table-wide
   `UPDATE` on `striatumd.artifacts`, but does have
   `UPDATE` privilege on exactly the three blob columns.
3. **GH27-3 (`artifact.backfill_blob` works).** The Go backfill handler's
   existing `UPDATE striatumd.artifacts SET blob_key = $1, blob_sha256 = $2,
   blob_content_type = $3 ...` succeeds when executed by the runtime role,
   without disabling `artifacts_no_update`.
4. **GH27-4 (Negative non-blob update).** Updating `content_sha256`,
   `repo_path`, `artifact_kind`, or any other non-blob artifact field still
   fails with SQLSTATE `P0001` and the existing message.
5. **GH27-5 (Positive blob-only update).** Updating one, two, or all three
   blob-reference columns succeeds, including NULL-to-value and value-to-NULL
   transitions where allowed by current column nullability.
6. **GH27-6 (Grant repair aligned).** `src/striatum/daemon_pg/roles.py` and
   its `role_repair_sql()` output include the column-level update grant so
   `striatum daemon doctor --provision-rw-role --repair-grants` restores the
   same privilege posture on fresh or repaired installs.
7. **GH27-7 (Regression suite).** `make smoke` and `make pg-test` still pass.

## Verification Commands

Run at minimum:

```bash
make lint
make typecheck
make pg-test
make smoke
```

Targeted tests should include the new migration/role checks, for example:

```bash
pytest tests/daemon_pg/test_migration_0010_artifact_blob_update.py \
  tests/daemon_pg/test_append_only_role_grants.py \
  tests/test_daemon_pg.py
```

Manual PostgreSQL round-trip, adjusted to an ephemeral test database and a
seeded artifact row:

```sql
-- Positive case: blob-reference update succeeds.
UPDATE striatumd.artifacts
   SET blob_key = 'runs/run_1/jobs/job_1/artifacts/scope',
       blob_sha256 = repeat('a', 64),
       blob_content_type = 'text/markdown'
 WHERE repository_id = 'repo_1'
   AND artifact_id = 'art_1';

-- Negative case: immutable artifact identity/content update still fails P0001.
UPDATE striatumd.artifacts
   SET content_sha256 = repeat('b', 64)
 WHERE repository_id = 'repo_1'
   AND artifact_id = 'art_1';
```

Also verify runtime-role privileges explicitly:

```sql
SELECT privilege_type
  FROM information_schema.table_privileges
 WHERE grantee = 'striatumd_rw'
   AND table_schema = 'striatumd'
   AND table_name = 'artifacts';

SELECT privilege_type, column_name
  FROM information_schema.column_privileges
 WHERE grantee = 'striatumd_rw'
   AND table_schema = 'striatumd'
   AND table_name = 'artifacts'
 ORDER BY column_name, privilege_type;
```

The expected result is no table-wide `UPDATE` for `artifacts`, plus
column-level `UPDATE` only on `blob_key`, `blob_sha256`, and
`blob_content_type`.

## Risks

- Daemon PostgreSQL migrations are forward-only. If migration 0010 installs a
  malformed trigger function, it can lock out legitimate artifact writes until
  an owner/admin manually applies a corrective migration or function
  replacement.
- The trigger must be table-local. Accidentally changing
  `striatumd.refuse_repo_append_only_change()` to permit blob columns would
  weaken `events` update protection or make the generic function depend on
  artifact-only column names.
- Privilege and trigger behavior need to agree. A trigger-only fix still leaves
  `striatumd_rw` blocked by revoked table privileges; a grant-only fix weakens
  protection unless the artifacts trigger remains column-aware.
- Include `CREATE OR REPLACE FUNCTION` in the migration so a failed or partial
  local re-run can safely converge before the trigger is recreated.
