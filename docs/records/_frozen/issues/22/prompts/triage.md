# Triage -- GH #22 scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/22/SPEC.md`
2. `src/striatum/daemon_pg/connection.py` -- the `doctor()` function, the
   `connect_and_migrate()` entry point, the `unsafe_privileges` refusal,
   and the `STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK` test-harness escape.
3. `src/striatum/daemon_pg/client_admin.py` -- the hint string emitted on
   privilege refusal (around line 301), `daemon_stop()` (line ~347) which
   routes through `_connect_pg()` → `connect_and_migrate()`.
4. `src/striatum/daemon_pg/config.py` -- the existing `--postgres-url` /
   `STRIATUM_DAEMON_DB_URL` resolution.
5. `src/striatum/daemon_pg/migrations.py` -- migration application + the
   advisory lock pattern; understand what `apply_migrations(conn)` requires
   from the connection's role.
6. `src/striatum/daemon_pg/roles.py` -- the intentional gap between owner
   and runtime role grants.
7. `src/striatum/cli/dispatch.py` -- the `daemon` subcommand dispatch
   around line 1426, especially the `doctor` and `stop` paths.
8. `docs/POSTGRES_TRANSITION.md` -- the operator runbook for repository
   registration and migrations.
9. `docs/issues/23/SPEC.md` -- the sibling pidfile issue; `daemon stop`
   recovery composes across the two.

## Output

Write `docs/issues/22/SCOPE.md` with `striatum.synthesis.v1` front matter
and the exact `author:` line from the work packet. Include:

- the exact files in scope for the fix (CLI dispatch, doctor/connection,
  client_admin, hint strings, tests, any docs that name the workaround
  pattern);
- the exact files out of scope (do NOT touch other RFC migrations,
  legacy SQLite paths, supervisor code, repo_local migrations);
- an acceptance checklist with one numbered check per bullet under
  "Acceptance / Definition of done" in `docs/issues/22/SPEC.md`;
- the chosen approach among the proposals (new verb vs. doctor flag vs.
  hybrid) with justification rooted in the existing CLI shape;
- verification commands the implementer should run after the change
  (lint, typecheck, the relevant test target, a manual `daemon doctor`
  invocation with the proposed flag);
- risks and likely conflicts with parallel issue workflows (especially
  #23, since `daemon stop` recovery touches both).
