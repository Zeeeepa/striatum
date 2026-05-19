---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "medium"
---

author: reviewer-unknown-model-001

# GH #22 Verification Review

Final verdict: `needs_revision`.

No license, attribution, external persistence, telemetry, or compliance issue
was found in the reviewed artifact set. The implementation satisfies the
runtime behavior and operator-documentation requirements, but the test coverage
does not fully meet the issue's acceptance bar for PostgreSQL owner-vs-runtime
integration coverage.

## Acceptance Verification

1. **Documented owner/admin migration path: accepted.**
   The parser exposes `striatum daemon doctor --apply-migrations --as-owner
   <owner-url>` and documents the peer-auth socket example at
   `src/striatum/cli/parser.py:288` and `src/striatum/cli/parser.py:289`.
   Dispatch passes `as_owner_url` into `pg_doctor()` at
   `src/striatum/cli/dispatch.py:1475` through
   `src/striatum/cli/dispatch.py:1480`. Operator docs describe the same
   supported command and peer-auth URL at `docs/POSTGRES_TRANSITION.md:127`
   through `docs/POSTGRES_TRANSITION.md:137`, and CLI reference matches at
   `docs/CLI_REFERENCE.md:199` through `docs/CLI_REFERENCE.md:202`.

2. **`daemon stop` works when migrations are pending: accepted.**
   `daemon_stop()` reads the pidfile and sends SIGTERM before any PostgreSQL
   work at `src/striatum/daemon_pg/client_admin.py:349` through
   `src/striatum/daemon_pg/client_admin.py:359`. The audit path uses plain
   `connect()` rather than `connect_and_migrate()` and catches connection,
   authorization, and audit failures at `src/striatum/daemon_pg/client_admin.py:363`
   through `src/striatum/daemon_pg/client_admin.py:390`, so an unreachable or
   unmigrated PostgreSQL instance cannot block the stop result. The no-pidfile
   result remains `{"stopped": False, "reason": "not_running"}` at
   `src/striatum/daemon_pg/client_admin.py:353` through
   `src/striatum/daemon_pg/client_admin.py:355`, which is acceptable for #22
   because #23 owns pidfile creation/removal.

3. **Hint string names a real supported CLI shape: accepted.**
   The migration-privilege failure hint now names
   `striatum daemon doctor --apply-migrations --as-owner <owner-url>
   --repair-grants` and includes `postgresql:///striatum_daemon` at
   `src/striatum/daemon_pg/client_admin.py:300` through
   `src/striatum/daemon_pg/client_admin.py:305`. The lifecycle test asserts
   `--as-owner`, `--apply-migrations`, and the peer-auth URL appear, and that
   `STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK` does not, at
   `tests/test_daemon_pg_lifecycle.py:194` through
   `tests/test_daemon_pg_lifecycle.py:199`.

4. **Tests cover the owner-vs-runtime gap: needs revision.**
   Unit coverage is present and targeted: the owner connection is required for
   `apply_migrations()` while the runtime privilege summary remains scoped to
   the runtime connection at `tests/test_daemon_pg.py:216` through
   `tests/test_daemon_pg.py:268`; the runtime-only path still surfaces
   `InsufficientPrivilege` at `tests/test_daemon_pg.py:271` through
   `tests/test_daemon_pg.py:300`; and an unreachable owner URL is structured at
   `tests/test_daemon_pg.py:304` through `tests/test_daemon_pg.py:335`.
   However, the handoff states that no dedicated runtime-vs-owner PostgreSQL
   integration test was added. This leaves the SPEC bullet "at integration
   level if PG is available" and SCOPE DoD-8 unfulfilled. `make pg-test` in
   this checkout runs only `tests/test_daemon_pg.py` and passed in 0.11s; it
   did not exercise a real runtime role failing an ALTER-requiring migration
   and an owner URL applying it successfully.

## Adversarial Probes

- **Owner-path does not weaken `unsafe_privileges`: accepted.**
  `doctor()` resolves the runtime connection first, opens a separate owner
  connection only when `as_owner_url` is supplied with an admin action, runs
  admin work through `admin_conn`, then calls `_privilege_summary(conn)` on the
  runtime connection at `src/striatum/daemon_pg/connection.py:90` through
  `src/striatum/daemon_pg/connection.py:119`. The `unsafe_privileges` refusal
  remains at `src/striatum/daemon_pg/connection.py:120` through
  `src/striatum/daemon_pg/connection.py:152`.

- **`daemon stop` with fresh or unreachable PG: accepted.**
  `daemon_stop()` computes and returns the pidfile/SIGTERM result before the
  audit hook at `src/striatum/daemon_pg/client_admin.py:353` through
  `src/striatum/daemon_pg/client_admin.py:360`. `_best_effort_audit_stop()`
  returns if PG is unconfigured, catches connection failure, and catches
  authorization/audit failures at `src/striatum/daemon_pg/client_admin.py:370`
  through `src/striatum/daemon_pg/client_admin.py:390`. This covers both
  intentionally unreachable PG and a zero-migration/fresh PG whose audit tables
  are missing.

- **Hint string shape: accepted.**
  The operator-facing hint and parser/help/docs now agree on `--as-owner` and
  the peer-auth socket example. Evidence: `src/striatum/daemon_pg/client_admin.py:300`,
  `src/striatum/cli/parser.py:289`, `docs/POSTGRES_TRANSITION.md:130`, and
  `docs/CLI_REFERENCE.md:270`.

## Verification Run

- `make lint`: passed.
- `make typecheck`: passed.
- `.venv/bin/python -m pytest tests/test_daemon_pg.py tests/test_daemon_pg_doctor.py tests/test_daemon_pg_lifecycle.py tests/cli/test_dispatch_daemon_doctor.py tests/cli/test_parser_help.py`: 39 passed.
- `make pg-test`: 11 passed.

## Finding

**F1 - Medium - Missing PostgreSQL integration coverage for the owner-vs-runtime
migration gap.**

The SPEC requires coverage of the owner-vs-runtime migration gap at unit level
and integration level if PostgreSQL is available. The implementation added the
unit tests, but the handoff explicitly says a dedicated runtime-only
ephemeral-role integration test was not added. Add a PostgreSQL harness test
that creates/provisions a runtime role unable to apply an ALTER-requiring
daemon migration, verifies the runtime-only `--apply-migrations` path fails
with the owner/ALTER privilege failure, then verifies
`daemon doctor --apply-migrations --as-owner <owner-url>` applies the migration
successfully while the runtime privilege summary still evaluates the runtime
role. If the harness truly cannot support this, the spec/scope needs an
operator-approved relaxation; otherwise this is an unresolved acceptance gap.
