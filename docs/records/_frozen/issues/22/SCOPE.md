---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/22/SPEC.md", "docs/issues/23/SPEC.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/POSTGRES_TRANSITION.md", "docs/SPEC.md", "docs/INDEX.md", "AGENTS.md"]
---

author: triager-unknown-model-001

# GH #22 - SCOPE

Bound scope for GH #22, "daemon migration path requires owner role but has no
--admin-url; runtime role gets stuck."

The implementation must give operators a supported owner-role path for daemon
PostgreSQL migrations and must make `striatum daemon stop` independent of
successful schema migration. This is a narrow daemon-admin fix, not a broader
substrate or SQLite migration change.

## 1. Issue covered

- GH #22 - daemon migrations need an owner/admin role, but the runtime role is
  intentionally not allowed to `ALTER` existing tables.
- Related recovery behavior from GH #23 only where it affects `daemon stop`.
  The actual daemon pidfile writer is owned by #23.

## 2. Chosen approach

Use the **hybrid** approach:

- Add a doctor owner-connection flag, preferably
  `striatum daemon doctor --apply-migrations --as-owner <url>`, for applying
  pending daemon migrations through an owner/admin connection while preserving
  `--postgres-url` / `STRIATUM_DAEMON_DB_URL` as the runtime-role connection
  used for normal diagnostics.
- Make `striatum daemon stop` read the daemon pidfile and send `SIGTERM`
  without calling `_connect_pg()` / `connect_and_migrate()` first.

This fits the existing CLI shape better than overloading `daemon migrate`.
`daemon doctor` is already the direct PostgreSQL bootstrap/admin surface and
already owns `--apply-migrations`, `--provision-rw-role`, `--repair-grants`,
and `--postgres-url`. The current `daemon migrate` spelling is deliberately a
retired SQLite import compatibility command that refuses before opening
SQLite; turning that name into a new daemon-schema migration verb risks
confusing two unrelated migration stories. If the implementer finds parser
compatibility requires a different flag name, keep the semantics: a separate
owner/admin URL for the migration connection only, with no use of
`STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK`.

## 3. Files in scope

The implementer may edit only the daemon-admin code, parser/dispatch wiring,
operator docs, and focused tests needed for this issue.

- **EDIT** `src/striatum/cli/parser.py` - add the supported doctor flag
  (`--as-owner <url>` or a clearly equivalent owner/admin URL flag). Do not
  repurpose retired SQLite import flags.
- **EDIT** `src/striatum/cli/dispatch.py` - pass the new doctor owner URL
  into the PostgreSQL doctor helper. Keep `daemon stop` dispatch pointed at a
  stop helper that does not require a migrated database.
- **EDIT** `src/striatum/daemon_pg/connection.py` - teach `doctor()` to use a
  separate owner/admin connection for `apply_migrations()` when the new flag is
  supplied, without running the runtime-role `unsafe_privileges` summary
  against that owner connection. Preserve the test-harness env-var as
  test-only, not as an operator path.
- **EDIT** `src/striatum/daemon_pg/client_admin.py` - update the privilege
  refusal hint string to name the real CLI shape, and change `daemon_stop()`
  so it does not call `_connect_pg()` or `connect_and_migrate()` before
  reading the pidfile and sending `SIGTERM`.
- **EDIT** `src/striatum/daemon_pg/config.py` only if the owner/admin URL
  needs shared redaction or config-result wording. Do not change the existing
  runtime URL precedence.
- **EDIT** `src/striatum/daemon_pg/migrations.py` only if a small helper is
  needed around existing `apply_migrations(conn)` behavior. Do not change the
  migration SQL list or advisory lock pattern.
- **EDIT** `src/striatum/daemon_pg/roles.py` only if doctor owner-connection
  flow needs clearer role/grant messaging. Do not weaken the runtime role's
  append-only or non-owner posture.
- **EDIT** `docs/POSTGRES_TRANSITION.md` - document the owner/admin migration
  path, including a peer-auth socket example such as
  `postgresql:///striatum_daemon`, and remove the workaround pattern using
  `STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK`.
- **EDIT** `docs/CLI_REFERENCE.md` if the CLI reference names daemon doctor,
  `--apply-migrations`, `--repair-grants`, `daemon stop`, or the old hint.
- **EDIT** focused tests:
  `tests/test_daemon_pg_lifecycle.py`,
  `tests/test_daemon_pg.py`, `tests/test_daemon_pg_doctor.py`, and
  `tests/cli/test_dispatch_daemon_doctor.py` are the likely unit targets.
  Add an integration-style PostgreSQL test behind the existing PG harness if
  practical.

## 4. Files and directories out of scope

The implementer must not edit:

- `src/striatum/legacy_sqlite/`, `src/striatum/db.py`, or
  `src/striatum/migrations.py` - no repo-local SQLite revival or import
  behavior is part of GH #22.
- `src/striatum/daemon_pg/sql/` - do not add, rewrite, or reorder daemon
  migrations for this issue.
- `docs/rfcs/`, `docs/dogfood/`, and historical prompts - preserve provenance.
- `docs/issues/23/` - #23 owns pidfile creation/removal and Go daemon
  lifecycle changes.
- Go daemon supervisor/process code except for tests or interfaces strictly
  needed by an existing stop test. #22 should not implement the #23 pidfile
  writer.
- Workflow handlers under `src/striatum/daemon_pg/handlers/`,
  daemon RPC method contracts, MCP surfaces, web UI, corpus/blob storage,
  recovery sweepers, and repo registration/adoption code unless a direct test
  failure proves the doctor/stop surface depends on them.
- `.striatum/`, `.venv/`, caches, build output, transcripts, and private
  diagnostics.

## 5. Acceptance checklist

The verify job should cite each ID below.

- [DoD-1] A documented operator command applies pending daemon PostgreSQL
  migrations through an owner/admin role without setting
  `STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK`. The expected shape is
  `striatum daemon doctor --apply-migrations --as-owner <owner-url> --json`,
  optionally combined with `--postgres-url <runtime-url>` and
  `--repair-grants`.
- [DoD-2] The owner/admin migration connection is used only for migration
  application (and grant repair if the implementer deliberately supports that
  combination). Runtime diagnostics still evaluate the runtime role and still
  refuse `unsafe_privileges` when the daemon is configured to run as an owner.
- [DoD-3] `striatum daemon stop` works when migrations are pending because it
  does not route through `_connect_pg()` or `connect_and_migrate()` before
  reading the pidfile and sending `SIGTERM`.
- [DoD-4] `daemon stop` remains honest when no usable pidfile exists:
  `{"stopped": false, "reason": "not_running"}` or the existing equivalent is
  acceptable for #22. Creating/removing the pidfile is #23.
- [DoD-5] The privilege-refusal hint in
  `src/striatum/daemon_pg/client_admin.py` references the supported owner
  CLI shape and no longer implies that the operator should use a test-harness
  env-var workaround.
- [DoD-6] Unit tests simulate a runtime-role `InsufficientPrivilege` during
  migration and assert the owner/admin doctor path calls `apply_migrations()`
  on the owner connection and returns a successful structured doctor result.
- [DoD-7] Unit tests assert `daemon_stop()` does not call
  `connect_and_migrate()` or require PostgreSQL authorization before using the
  pidfile path.
- [DoD-8] PostgreSQL integration coverage is added when the PG harness is
  available: runtime role cannot apply an `ALTER`-requiring migration, owner
  URL can, and the test skips cleanly under the existing no-PG harness rules
  when PostgreSQL is unavailable.
- [DoD-9] Operator docs and CLI help agree on the exact flag spelling and
  include a peer-auth owner URL example for local Linux installs.

## 6. Verification commands

Run these at minimum:

```bash
make lint
make typecheck
pytest tests/test_daemon_pg_lifecycle.py tests/test_daemon_pg.py tests/test_daemon_pg_doctor.py tests/cli/test_dispatch_daemon_doctor.py tests/cli/test_parser_help.py
```

If PostgreSQL is available, also run the relevant PG harness target:

```bash
make pg-test
```

Manual verification should include the supported owner path, using the final
flag spelling from help output:

```bash
striatum daemon doctor \
  --postgres-url "$STRIATUM_DAEMON_DB_URL" \
  --apply-migrations \
  --as-owner 'postgresql:///striatum_daemon' \
  --repair-grants \
  --json
```

Also verify stop does not depend on a successful migration:

```bash
striatum daemon stop --json
```

## 7. Risks and likely conflicts

- GH #23 is the main parallel conflict. #22 should make stop pidfile-driven
  and Postgres-independent; #23 should make the Go daemon write/remove the
  pidfile and test stale pidfile behavior. Coordinate tests so both issues do
  not each claim ownership of the same pidfile lifecycle implementation.
- Existing `daemon migrate` is intentionally a retired SQLite import command.
  Reusing it for daemon schema migration would require extra documentation and
  parser compatibility work; avoid that unless the operator explicitly
  redirects the approach.
- Owner/admin connections naturally fail the `unsafe_privileges` check because
  owners have implicit table privileges. The implementation must keep the
  bypass scoped to the migration connection, not dilute the runtime-role
  guardrail.
- `--repair-grants` may need owner/admin permissions too. If it is supported
  with `--as-owner`, document which connection performs grant repair; if not,
  return a clear structured error instead of silently using the runtime role.
- Docs currently mention `daemon doctor --apply-migrations --repair-grants`
  as though it is sufficient. Update every current operator doc or hint that
  repeats that workaround-shaped guidance.
