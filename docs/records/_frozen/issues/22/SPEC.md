# GH #22 — daemon migration path requires owner role but has no --admin-url; runtime role gets stuck

Source: https://github.com/halbritt/striatum/issues/22

## Summary

When daemon migrations are pending and the tables aren't owned by the runtime role (`striatumd_rw`), every read-side daemon CLI verb refuses, with no documented operator escape:

- `striatum daemon status` → `daemon PostgreSQL status unavailable: pending migrations require database owner/admin privileges; runtime role was refused: must be owner of table runs`
- `striatum daemon doctor` → `daemon_diagnostics.error = "must be owner of table runs"`, `ok: false`
- `striatum daemon stop` → same; calls `connect_and_migrate` first, can't migrate, can't stop the daemon either

This is the expected behavior of the migration code (the runtime role intentionally doesn't have `ALTER` on existing tables — see `daemon_pg/roles.py`). The problem is there's no first-class way to apply migrations as the owner.

## Repro

Local PG setup where daemon tables are owned by the Unix user (peer auth on local socket) and the daemon runs as `striatumd_rw` over TCP:

1. Land a new migration (e.g. 0009 — the RFC 0072 blob storage one in 154fac4).
2. Don't rebuild/restart the daemon — it's still on the prior schema.
3. Run `striatum daemon status`. It refuses because `schema_version=6 < latest_supported=9`, and the runtime role can't ALTER `runs`.
4. Try to recover: `striatum daemon stop` ALSO refuses, with the same message, because it routes through `connect_and_migrate` first.
5. The hint in `client_admin.py:301` says: "run `striatum daemon doctor --apply-migrations --repair-grants` as a database owner/admin" — but there is no flag or env var to actually point the doctor at a different role for migration-time. `--postgres-url` exists, but if your owner role is peer-auth-only (as in a typical Linux PG install), you have to discover the `postgresql:///dbname` socket form yourself.

## Workaround currently required

```
kill -TERM <daemon-pid>   # daemon stop is unusable
STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK=1 \
  striatum daemon doctor --apply-migrations \
    --postgres-url 'postgresql:///striatum_daemon'
```

The env var is documented as test-harness-only ("Production deployments must NEVER set this" — `daemon_pg/connection.py:97-99`). Using it for routine migrations is a documentation smell.

## Acceptance / Definition of done

A solution must satisfy each of:

1. **A documented operator path applies pending migrations as the owner role** without the test-harness env-var. Either a new verb (`striatum daemon migrate --admin-url <url>` / `--peer-auth`) or a doctor flag (`--as-owner <url>`) that bypasses the runtime-role privilege summary for the migration connection only.
2. **`striatum daemon stop` works even when migrations are pending.** Stop only needs the pidfile + SIGTERM (related: #23); it must NOT route through `connect_and_migrate`.
3. **The hint string in `client_admin.py:301` references a real, supported CLI shape** — no env-var workaround.
4. **Tests cover the owner-vs-runtime gap** at unit level (mock `pg.errors.InsufficientPrivilege` on ALTER and assert the operator path resolves it) and at integration level if PG is available.

## Suggested fix (proposals; pick one)

1. **New verb**: `striatum daemon migrate --admin-url <url>`. Connects with owner credentials, applies migrations, exits. No doctor-level privilege check; this is the intended migration path. Audit-chained.
2. **Doctor flag**: `striatum daemon doctor --apply-migrations --as-owner <url>` — separates the migration connection from the privilege-summary connection so the owner can ALTER without tripping the `unsafe_privileges` refusal.
3. **Independent**: make `daemon stop` work without migrating (this is required regardless of 1/2 — see #23).

(3) is the cheapest immediate win and probably mandatory regardless of (1)/(2) — the daemon should always be stoppable.

## Provenance

Hit while diagnosing a local 1.54.0 → 1.55.0 daemon after RFC 0072 migration 0009 landed (commit `154fac4`). Full repro and unblock walkthrough in conversation transcript on 2026-05-18.

Related:

- #23 — daemon pidfile is never written; daemon stop relies on a working migration path because it can't fall back to pidfile-only termination.
