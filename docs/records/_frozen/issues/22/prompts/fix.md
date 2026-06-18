# Implement -- GH #22

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/22/SPEC.md`
- `docs/issues/22/SCOPE.md` (the triager's bound)
- the source modules named in `SCOPE.md`
- `docs/issues/23/SPEC.md` only as far as is needed to satisfy the
  "daemon stop must work without migrating" acceptance bullet. Do NOT
  implement #23's pidfile fix in this workflow.

## Deliverables

Per `docs/issues/22/SPEC.md` "Acceptance / Definition of done":

1. A first-class operator CLI path that applies pending migrations as the
   owner role, without `STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK`.
2. `striatum daemon stop` must succeed even when migrations are pending;
   it must use the pidfile (or, if absent per #23, fall back to a
   documented manual SIGTERM path). It must NOT call
   `connect_and_migrate`.
3. The hint string in `client_admin.py` must reference the new supported
   shape, not the test-harness env var.
4. Unit tests for the owner-vs-runtime split (mock
   `psycopg.errors.InsufficientPrivilege` and assert the operator path
   recovers). Add an integration smoke if existing PG-backed tests support
   it (look for `tests/daemon_pg/` and the conftest fixtures).

## Constraints

- Stay inside `write_scope.allowed_paths`.
- Do not introduce a separate config file shape; if you need an admin
  URL, accept it as a CLI flag or env var.
- Preserve the existing `unsafe_privileges` safety net for *runtime*
  connections; only the migration connection should bypass it.
- Use the exact `author:` line from the work packet in the handoff.

## Handoff

Write `docs/issues/22/build/HANDOFF.md` with the
`striatum.handoff.v1` front matter. Cite each definition-of-done bullet
closed, files changed, tests run / not run, and residual risk.
