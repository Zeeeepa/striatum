---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["gh-27", "postgres", "verification"]
---

author: reviewer-unknown-model-001

# GH #27 Verification Review

Final verdict: `accept_with_findings`.

No license, attribution, telemetry, or compliance issue was found in the
reviewed artifact set. The implementation is confined to in-repository SQL,
role-grant helpers, migration registries, and tests; it does not add
third-party code, external services, transcript capture, or license-bearing
material.

The GH #27 behavior is implemented correctly by source inspection and the
focused PostgreSQL tests pass. The only remaining gaps are verification-depth
gaps: the test suite should add literal coverage for the mixed blob/non-blob
UPDATE case, the `author_line` negative case, and a full
`artifact.backfill_blob` runtime-role path.

## Acceptance Verification

1. **GH27-1, column-aware trigger: accepted.** Migration 0010 installs
   `striatumd.allow_artifact_blob_reference_update()` at
   `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:19`.
   The function removes only `blob_key`, `blob_sha256`, and
   `blob_content_type` from `OLD` and `NEW` before comparing the remaining
   row identity at `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:24`.
   Any non-blob difference raises the existing append-only exception at
   `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:30`.
   The artifacts update trigger is recreated to call the new function at
   `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:37`.

2. **GH27-2, column-level grant only: accepted.** Migration 0010 grants
   column-level UPDATE on exactly the three blob columns at
   `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:42`.
   `repair_role_grants()` first revokes table-wide UPDATE/DELETE for
   append-only tables at `src/striatum/daemon_pg/roles.py:80`, then grants
   back only the blob columns for artifacts at
   `src/striatum/daemon_pg/roles.py:98`. The pasteable
   `role_repair_sql()` mirrors that posture at
   `src/striatum/daemon_pg/roles.py:163` and
   `src/striatum/daemon_pg/roles.py:169`. The information-schema test
   asserts no table-wide UPDATE and exactly the three blob UPDATE columns at
   `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:359`.

3. **GH27-3, `artifact.backfill_blob` works without disabling the trigger:
   accepted with verification gap.** The Go handler updates only
   `blob_key`, `blob_sha256`, and `blob_content_type` at
   `go/pkg/mutations/artifact_backfill_blob.go:179`, which is the precise
   mutation allowed by the new trigger and column grant. The focused SQL
   positive test proves that shape succeeds at
   `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:161`.
   I did not find a full RPC/blob-client test that creates a fresh NULL
   `blob_key` artifact and invokes `artifact.backfill_blob` as
   `striatumd_rw` with the trigger enabled.

4. **GH27-4, non-blob UPDATEs still refused with P0001: accepted with
   verification gap.** The parametrized test covers `content_sha256`,
   `repo_path`, `artifact_kind`, `logical_name`, and `size_bytes` at
   `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:232` and
   asserts SQLSTATE `P0001` at
   `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:258`.
   `author_line` is also a non-blob artifact column in the schema at
   `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql:226` and
   is protected by the JSONB whole-row comparison, but it is not one of the
   literal negative test parameters.

5. **GH27-5, blob-only UPDATE succeeds: accepted.** The positive test updates
   all three blob columns and commits successfully at
   `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:161`. The
   NULL round-trip test confirms value-to-NULL transitions remain allowed at
   `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:195`.

6. **GH27-6, grant repair aligned: accepted.** `repair_role_grants()` records
   `grant_artifacts_blob_columns` after applying the column grant at
   `src/striatum/daemon_pg/roles.py:104`. Unit coverage asserts the applied
   step and SQL text at `tests/daemon_pg/test_roles.py:83`, and
   `role_repair_sql()` coverage asserts the generated grant at
   `tests/test_daemon_pg.py:105`.

7. **GH27-7, regression suite: accepted.** The handoff reports `make lint`,
   `make typecheck`, `make pg-test`, `make smoke`, the focused daemon-PG
   suite, Go tests, and Go build all passing in
   `docs/issues/27/build/HANDOFF.md`. I reran the focused review subset:
   `.venv/bin/python -m pytest tests/daemon_pg/test_migration_0010_artifact_blob_update.py tests/daemon_pg/test_roles.py tests/test_daemon_pg.py -q`
   returned `27 passed`.

## Adversarial Probes

- **Negative, individual non-blob columns:** `content_sha256`, `repo_path`,
  and `artifact_kind` are explicitly covered by the parametrized P0001 test
  at `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:232`.
  `author_line` is protected by the same trigger comparison because it is
  not removed from the JSONB row, but it needs an explicit regression test.
- **Positive, all blob columns:** Covered by
  `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:161`.
- **Mixed blob plus non-blob UPDATE:** Source behavior is correct because the
  trigger compares all non-blob columns after removing only the blob fields,
  but no literal mixed-case test is present.
- **DELETE still refused:** Covered by
  `tests/daemon_pg/test_migration_0010_artifact_blob_update.py:269`; the
  original `artifacts_no_delete` trigger still calls the generic refuse
  function at `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql:462`.
- **`striatumd_rw` end-to-end backfill:** The handler and grants line up, but
  the suite should add a full `artifact.backfill_blob` invocation as the
  runtime role against a fresh artifact row with `blob_key IS NULL`.
- **Migration idempotency:** The migration is structurally re-runnable:
  `CREATE OR REPLACE FUNCTION` at
  `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:19`,
  `DROP TRIGGER IF EXISTS` at
  `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:37`, and
  an idempotent role-existence grant block at
  `src/striatum/daemon_pg/sql/0010_artifact_blob_update_trigger.sql:42`.
  The migration framework skips already-applied migrations while verifying
  hashes at `src/striatum/daemon_pg/migrations.py:64`.

## Findings

- **LOW: Add literal adversarial regression coverage for `author_line` and
  mixed UPDATEs.** Exact remediation: extend
  `tests/daemon_pg/test_migration_0010_artifact_blob_update.py` so the
  negative parametrization includes `author_line`, and add a test that runs a
  single UPDATE setting one blob column plus one immutable column and asserts
  SQLSTATE `P0001`.

- **LOW: Add full `artifact.backfill_blob` runtime-role coverage.** Exact
  remediation: add an integration test that provisions `striatumd_rw`, seeds
  a blob-routed artifact with `blob_key IS NULL`, configures a fake or local
  blob client, invokes `artifact.backfill_blob` through the Go handler/RPC
  path as the runtime role with `artifacts_no_update` enabled, and asserts
  the row is backfilled plus the `artifact.blob_backfilled` event is inserted.
