# PostgreSQL Transition Runbook

This runbook walks an operator from a pre-D094 install through the
post-D094 PostgreSQL-first state model: prerequisites, daemon
configuration, daemon doctor, retired SQLite import handling, repository
registration, verification, and the documented refusal codes you will hit if
a step is skipped.

Read [HOW_TO_HUMAN.md](how-to-human.md) for the broader operator
playbook; read [SPEC.md § State Store](../reference/spec.md#state-store) and
[SPEC.md § CLI](../reference/spec.md#cli) for the implementation contract.

## What changed

Two related changes shape the current product:

1. **RFC 0033** put daemon-global state (registry, capabilities,
   audit chain, scheduler cursors, RPC request log) on operator-
   installed system PostgreSQL.
2. **D094 / RFC 0043** supersedes the V1 carve-out that kept
   per-repository workflow state in `.striatum/retired-local-state`.
   Repo-local workflow tables (`runs`, `jobs`, `sessions`,
   `queue_messages`, `leases`, `work_packets`, `artifacts`,
   `verdicts`, `blockers`, `command_requests`, `process_executions`,
   `events`, `job_worktrees`, `process_supervisors`,
   `process_supervisor_pointers`) move into the same daemon-owned
   Postgres under a `repository_id` scope. The daemon becomes a
   hard prerequisite for every Striatum verb; `--no-daemon` is
   retired. `.striatum/` survives as operational scratch only.

3. **RFC 0048 (V1.49.0 → V1.55.0)** finished the substrate port on
   the daemon side: every mutation, recovery, and read handler gained
   a native PG handler in `src/striatum/daemon_pg/handlers/`.
   D107 / RFC 0068 and D109 then moved the production default to the
   Go daemon; D111 retired the Python daemon selector, so `--core python`
   and `STRIATUM_DAEMON_CORE=python` no longer select a production daemon.
   The CLI dispatch routes mapped verbs through the daemon's Unix socket;
   mapped methods fail closed instead of falling back to SQLite when the
   daemon is unreachable. **Schema v6**
   (migration 0006) promotes the per-event chain hash anchors out
   of `payload_json._event_chain` into dedicated `previous_hash` /
   `row_hash` columns and adds a `striatumd.repo_event_chain_heads`
   pointer so the chain head reads in O(1). See § "RFC 0048
   status" below for phase-by-phase detail.

## Prerequisites

- A working PostgreSQL service the operator can connect to (system
  package, managed service, or a developer install — Striatum does
  not manage the Postgres lifecycle). Bundled and Dockerized
  distributions are deferred (RFC 0033 §8, inherited by RFC 0043).
- A role that can `CREATE SCHEMA` and `CREATE TABLE`; the daemon
  owns subsequent forward-only migrations and role grants.
- Striatum's Go binaries installed from a release archive or a contributor
  checkout with `make install`.
- For an existing pre-D094 repo: writable SQLite imports are retired.
  Archive or remove `.striatum/retired-local-state` before registering the
  repository with `striatum repo add --init`.

## Provision the daemon-required role

The daemon's `audit_log` / `events` / `artifacts` tables are append-only
by contract (RFC 0033 §3): the role the daemon connects as must hold
`SELECT, INSERT` on those tables but **not** `UPDATE` or `DELETE`. If
the daemon connects as the database owner, ownership grants implicit
UPDATE/DELETE and `daemon doctor` refuses with status `unsafe_privileges`
and message `"daemon role must not have UPDATE or DELETE on
striatumd.audit_log"`. Fresh installs that use the database owner as
the connecting role will hit this; the dedicated daemon role is the
remediation.

Create the role once, before configuring the daemon DB connection:

```bash
# Replace <yourpass> with a strong password before pasting.
sudo -u postgres psql -d striatum_daemon <<'SQL'
CREATE ROLE striatumd_rw WITH LOGIN PASSWORD '<yourpass>';
GRANT CONNECT ON DATABASE striatum_daemon TO striatumd_rw;
GRANT USAGE ON SCHEMA striatumd TO striatumd_rw;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA striatumd TO striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.audit_log FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;
REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;
ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO striatumd_rw;
ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO striatumd_rw;
-- CREATE on db + schema is needed for forward daemon migrations and local
-- doctor repair/provisioning flows.
GRANT CREATE ON DATABASE striatum_daemon TO striatumd_rw;
GRANT CREATE ON SCHEMA striatumd TO striatumd_rw;
SQL
```

Connect the daemon as `striatumd_rw` rather than the database owner.
The next section ("Configure the daemon DB connection") shows the three
surfaces for that connection string.

### Non-Linux and no-`sudo` provisioning

The `sudo -u postgres psql ...` example is the common Debian/Fedora
shape, not a Striatum requirement. Use any admin connection that can
create roles, grant privileges, and run migrations.

Common alternatives:

- **macOS Postgres.app:** open the Postgres.app SQL shell or run its
  bundled `psql` from a terminal, for example
  `/Applications/Postgres.app/Contents/Versions/latest/bin/psql -d striatum_daemon`.
  Connect as the app-created admin role, run the same SQL, then configure
  `STRIATUM_DAEMON_DB_URL` for `striatumd_rw`.
- **Homebrew PostgreSQL:** use `psql -d striatum_daemon` as the macOS
  user that owns the local cluster, or `psql -U <admin-role> -d
  striatum_daemon` if you created a separate admin role.
- **Managed or locked-down local PostgreSQL:** ask the database admin to
  run the emitted `repair_sql` / `manual_sql`, then set the daemon runtime
  URL to the dedicated `striatumd_rw` role. Do not run the daemon as the
  owner role just to avoid grant setup; doctor will reject unsafe
  append-only privileges.

`daemon doctor` can now apply the common local repair path directly:

```bash
striatum daemon doctor \
  --postgres-url "$STRIATUM_DAEMON_DB_URL" \
  --apply-migrations \
  --provision-rw-role \
  --repair-grants \
  --json
```

If the current database role cannot create roles or grant privileges,
the JSON output includes `repair_sql` / `manual_sql` with the statements
to run from an admin `psql` session. Connect the daemon as
`striatumd_rw` after provisioning; running as the database owner still
fails the append-only privilege check.

### RFC 0110 L0 credential rotation: grant the owner ADMIN OPTION on the runtime role (PG 16, GH #169)

RFC 0110 §9.1 rotates the runtime role's password to a fresh RAM-only value
at **every daemon boot**, over the owner (`STRIATUM_OWNER_DB_URL`) connection:
`ALTER ROLE striatumd_rw PASSWORD '<random>'`. This is what makes a leaked
runtime DSN string die at the next restart. The rotation is only attempted in
the **two-role** posture (owner ≠ runtime, i.e. `STRIATUM_OWNER_DB_URL` is set
and differs from the runtime URL); the single-role / PEER posture skips it and
surfaces a `rotation_skipped_single_role` doctor finding.

**Prerequisite (PostgreSQL 16+).** A non-superuser owner role may only
`ALTER ROLE … PASSWORD` another role when it was granted that role **with admin
option**. (PostgreSQL 16 removed the old "any `CREATEROLE` role can alter any
non-superuser role" behavior; admin membership is now required.) Grant it once,
as a superuser, before adopting the two-role L0 posture:

```bash
sudo -u postgres psql -d striatum_daemon <<'SQL'
-- Let the owner role rotate striatumd_rw's password without inheriting its
-- privileges or being able to SET ROLE to it (admin the role, do not become it).
GRANT striatumd_rw TO <owner-role> WITH ADMIN OPTION, INHERIT FALSE, SET FALSE;
SQL
```

If the owner connects via PEER as your OS user (e.g.
`postgres://halbritt@/striatum_daemon?host=/var/run/postgresql`), `<owner-role>`
is that OS-user role. A **superuser** owner needs no grant (superusers can alter
any role); a **single-role** deployment needs no grant (rotation is skipped).

Without the grant, boot fails closed (RFC 0110 §9.2) with
`daemon_pg_owner_bootstrap_failed: … rotate runtime password` (SQLSTATE 42501),
and — because boot rotates before it serves — the daemon will not start. Apply
the grant as a superuser, then restart.

### Applying daemon migrations as the owner role (GH #22)

When a Striatum upgrade adds a new daemon migration, the runtime role
(`striatumd_rw`) cannot apply it because the migration may `ALTER` existing
tables that are owned by the operator (peer-auth socket installs) or a
DB owner role. The runtime role intentionally lacks `ALTER`/owner privileges
so the append-only contract on `striatumd.audit_log` / `events` / `artifacts`
holds at run-time.

Pass `--as-owner <owner-url>` to point the migration connection at the owner
role while keeping the runtime privilege summary on the runtime role:

```bash
striatum daemon doctor \
  --postgres-url "$STRIATUM_DAEMON_DB_URL" \
  --apply-migrations \
  --as-owner 'postgresql:///striatum_daemon' \
  --repair-grants \
  --json
```

`postgresql:///striatum_daemon` is the local peer-auth socket form: on a
typical Linux PostgreSQL install, the Unix user that owns the
`striatum_daemon` database can connect over the socket with no password
when peer auth routes them to a role with `CREATE`/owner rights. The
URL is also the right shape for an `ident` map; pass any libpq URL the
owner can authenticate against (for example
`postgresql://postgres@127.0.0.1/striatum_daemon`).

What the flag does:

- `--apply-migrations` is executed on the owner connection so `ALTER`
  statements succeed.
- `--repair-grants` (and `--provision-rw-role`, when combined) also run
  on the owner connection, because they issue `GRANT`/`REVOKE`/`CREATE ROLE`
  statements that the runtime role is not authorized to issue.
- The `unsafe_privileges` guardrail still evaluates the *runtime* connection
  resolved from `--postgres-url` / `STRIATUM_DAEMON_DB_URL`. The owner
  connection is never treated as the runtime role, so the append-only
  posture for `striatumd_rw` is preserved.
- If the owner URL is unreachable, doctor returns
  `status: "as_owner_unreachable"` with the redacted owner URL and the
  scrubbed error, and does not run `apply_migrations()`.

When `daemon status` refuses with `pending migrations require database
owner/admin privileges`, this is the supported remediation. The
`STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK` environment variable that older
docs mention is a test-harness affordance only and is not a supported
operator path. Production deployments must never set it.

## Configure the daemon DB connection

Choose one of three surfaces. They are checked in this order:

1. `--postgres-url <url>` — explicit per-invocation override.
2. `STRIATUM_DAEMON_DB_URL` environment variable.
3. `~/.config/striatum/daemon.toml` daemon config file (Linux/macOS
   XDG; Application Support on macOS).

The connection string is standard libpq. For the dedicated daemon role
provisioned above:
`postgres://striatumd_rw:<yourpass>@127.0.0.1:5432/striatum_daemon`.
Unix-socket connections work too (`postgres:///striatum_daemon`) when
peer auth or an ident map routes the connecting OS user to
`striatumd_rw`. The daemon will create its schema on first start; no
manual DDL is required beyond the role provisioning above.

## Prepare and verify the daemon DB

```bash
striatum daemon doctor --postgres-url "$STRIATUM_DAEMON_DB_URL" --apply-migrations --json
```

`daemon doctor` runs even when the daemon is not yet running
(it touches configuration and the database directly). With
`--apply-migrations` it brings the daemon-owned schema forward to
the version this runner expects. Without it, doctor reports the
required version and exits cleanly so operators can review before
applying.

The doctor output names the daemon socket path, the resolved
Postgres URL, the on-disk schema version, the audit-chain head,
and any role-grant issues. Fix what doctor reports before starting
the daemon.

## Start the daemon

```bash
striatum daemon service install --manager auto --json
striatum daemon service start --manager auto --json
```

For foreground debugging, use `striatum daemon start --json`.

## Dev-local Postgres profile

Production-local setup uses your system PostgreSQL service plus the
dedicated runtime role above. For disposable contributor smoke tests,
the repo also includes `examples/dev-postgres/docker-compose.yml`:

```bash
cd examples/dev-postgres
docker compose up -d
export STRIATUM_DAEMON_DB_URL=postgresql://striatum_admin:striatum_dev_password@127.0.0.1:5432/striatum_daemon
striatum daemon doctor --apply-migrations --provision-rw-role --repair-grants --json
```

Do not treat the compose profile as the production path; it exists only
to make local development reproducible without editing a system
PostgreSQL cluster.

`striatum daemon start` (also exposed as the `striatumd` console
script) is the supported foreground entry point. The startup
envelope prints the bound socket path and the resolved Postgres
URL. CLI workflow-state verbs route through this daemon; there is no
production SQLite fallback.

If the daemon is unreachable when a CLI verb runs, the verb refuses
with **exit code 11 (`daemon_unreachable`)** and stderr names the
socket path it tried plus the platform-specific remediation
(`systemctl --user start striatumd` on Linux with the bundled unit;
`launchctl bootstrap` hint on macOS; an explicit foreground
`striatumd` reminder; Postgres install hints if the daemon was
never started). `striatum --help`, `striatum --version`, and
`striatum daemon doctor` keep working without a daemon because they
touch no state.

## Register the target repo

```bash
striatum repo add /path/to/target --init --json
striatum --repo /path/to/target skills install --profile claude_code --json
```

`repo add --init` registers the repo into daemon PostgreSQL, creates
`.striatum/scratch`, and adds `.striatum/` to `.gitignore`. `skills
install` writes agent-side assets and does not create workflow state.
Writable SQLite import windows are closed; if a legacy
`.striatum/retired-local-state` exists, archive or remove it before
registering.

The first `daemon start` bootstraps a single admin token and writes an
owner-only runtime `client-token` file under the daemon's runtime
directory. Treat that file as degraded compared to an OS keyring.

## Retired SQLite import commands

`striatum daemon migrate --from sqlite --to pg` and
`striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>` are
retired compatibility spellings. They remain parseable so old automation gets
a clear error, but they refuse with exit code 12 before importing or opening
SQLite migration code. Current Striatum does not migrate legacy SQLite files
as an operator workflow. Archive or remove legacy SQLite files, then register
the target repository with `striatum repo add --init`.

## Verify registration and retired SQLite state

```bash
# Repo-local workflow state now reads from Postgres under the registered
# repository id. New runs and jobs are queryable through the normal CLI
# surfaces.
striatum daemon doctor \
  --repo /path/to/target \
  --authority \
  --json
striatum --repo /path/to/target status --json
striatum --repo /path/to/target doctor --verbose --json
striatum --repo /path/to/target list runs --json
```

`daemon doctor --repo <path> --authority --json` emits
`striatum.repo_cutover_report.v1`. The report confirms repository
registration, absence or archival of the live source
`.striatum/retired-local-state`, event-chain anchor health for PostgreSQL state,
and the bounded migration/test SQLite exception notes. It is verify-only: it
uses Postgres queries and raw file/tombstone/sentinel stat/hash checks, and it
does not open SQLite as a database or resume finalization.
`daemon doctor --repo <path> --authority --json` mirrors the same
verify-only repository report inside the doctor output and summarizes
repository cutover health in `striatum.authority_report.v1`.

An unregistered repo, including a pre-D094 repo that still has legacy SQLite
state, refuses CLI verbs with **exit code 12 (`repo_not_migrated`)**. The
stderr message and JSON hint tell the operator to archive/remove legacy SQLite
files and register with `repo add --init`.

`striatum daemon doctor --json` reports one substrate (Postgres),
one schema version, and one audit chain after successful registration.
Existing dogfood scaffolds under `docs/dogfood/<NNN>/`
are frozen historical artifacts; their `.striatum/retired-local-state`
references describe V1 behavior and are not migrated by this
command.

## Documented exit codes

The exit codes RFC 0043 reserves for daemon-required behavior:

| Code | Meaning | Operator remediation |
|---:|---|---|
| 10 | Daemon RPC transport, handshake, or version-skew refusal (RFC 0030). | Reconcile client and daemon versions; rerun `daemon doctor`. |
| 11 | `daemon_unreachable`. | Start the daemon (`striatum daemon start` or the systemd / launchd unit). Check the socket path printed in stderr. |
| 12 | `repo_not_migrated`. | Archive/remove legacy SQLite files if present, then register with `striatum repo add --init`. |

See [CLI_REFERENCE.md § Stable exit codes](../reference/cli-reference.md#stable-exit-codes)
for the full closed list.

## Rollback and inspection limits

- **Archive before registration is the safe rollback.** Current Striatum does
  not perform operator SQLite imports. If a target repo still has legacy
  `.striatum/retired-local-state`, archive it before registration so the old local
  mirror remains available for manual inspection.
- **Legacy tombstones are read-only historical remnants.** If an older
  Striatum version already created `.striatum/retired-local-state.tombstone`, no
  current Striatum verb opens it. SQLite tooling can inspect it directly; you
  can delete it manually when you no longer want the local mirror.
- **There is no "un-migrate" command.** Once Postgres-side registration
  exists, deleting it should be treated as data deletion. Normal recovery is
  restoring the Postgres database from backup or PITR and restoring the target
  repo from the matching Git snapshot.
- **The tombstone is not a live mirror.** It is the pre-cutover
  snapshot. Workflow state mutated after registration lives in
  Postgres only.

## RFC 0048 status (substrate port)

RFC 0043 V1.6 shipped the schema flip, the historical migration command, and
the `STRIATUM_DAEMON_REQUIRED` enforcement. RFC 0048 closes the
remaining substrate-port work on the daemon side:

- **Phase A (v1.49.0)** — 16 single-repo mutation handlers ported to
  PG-backed daemon handlers under
  `src/striatum/daemon_pg/handlers/{workflow_loop,recovery_evidence}/`.
  `DaemonRpcRouter._route` initially resolved the PG handler before
  transition-era SQLite dispatch; current mapped CLI verbs fail closed
  instead of falling back.
- **Phase B (v1.50.0–v1.54.0 + follow-up)** — Go parity/support
  fixtures: 12 read handlers (`go/pkg/reads/`) and selected mutation
  plumbing (`go/pkg/mutations/`) were implemented as developer-harness
  counterparts. D107 / RFC 0068 later promoted this to production daemon
  port work; D109 made Go the default daemon core after active
  contract-method parity, and D111 retired the Python daemon selector.
- **Phase C (v1.51.0–v1.52.0)** — CLI dispatch routes ~30 verbs
  through the Unix-socket daemon RPC; the daemon bootstraps an
  admin client into `striatumd.clients` (Postgres) and writes its
  runtime token to the platform runtime token path, normally
  `$XDG_RUNTIME_DIR/striatum/client-token` on Linux when set,
  `~/.cache/striatum/runtime/client-token` as the Linux fallback, or
  `~/Library/Caches/striatum/runtime/client-token` on macOS.
- **V1.5 hardening (v1.55.0)** — capability-denial test matrix
  across the ported handlers (F2), audit-chain row-lock on
  `audit_chain_head` (F3), append-only role-grant tests for
  `striatumd_rw` (F4), parity rig (`assert_payload_parity`,
  HIGH#1), inline-helper wiring for recovery completion paths
  (HIGH#2), and **schema migration 0006** (events
  `previous_hash` + `row_hash` columns + `repo_event_chain_heads`
  pointer).
- **Mapped CLI fail-closed (follow-up)** — mapped daemon RPC verbs
  no longer fall back to SQLite when the daemon is unreachable;
  CLI-local bootstrap and admin surfaces remain explicit out-of-band
  helpers, not SQLite fallback routes.

For ported methods, `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` no longer takes effect — the CLI dispatch
hook checks `daemon_pg.handlers.registry.resolve_pg_handler(method)`
and routes through the daemon's PG handler when one is registered.
Use `striatum daemon doctor --explain --json | jq '.data.explain'`
to see the per-method routing.

### Substrate path for ported methods (Schema v6)

A ported single-repo RPC verb now flows through:

1. Capability token authorization (PG `striatumd.clients` +
   `client_capabilities` rows).
2. `DaemonRpcRouter._route` → `resolve_pg_handler(method)` →
   handler in `daemon_pg.handlers.*`.
3. Per-handler row locks (`FOR UPDATE` on `jobs`, `runs`, `leases`,
   `queue_messages`) plus the parent `striatumd.repositories` row
   lock that serializes the per-repository event chain.
4. `striatumd.events` insert with `previous_hash` / `row_hash`
   columns populated directly (migration 0006); the legacy
   `_event_chain` payload-side anchor is gone from new writes and
   has been backfilled out of existing rows.
5. `striatumd.audit_log` append-only chain (`audit_chain_head`
   singleton locked `FOR UPDATE` so concurrent appenders
   serialize) records the request, response, and authorization
   decision.

## See also

- [LANE_SANDBOX.md](lane-sandbox.md) — isolate supervised lanes from this
  daemon-owned PostgreSQL with a dedicated PG-less lane OS user (RFC 0096 §2 /
  #87), so a lane cannot reach the DB via peer auth and bypass the artifact API.
- [GETTING_STARTED.md](../tutorials/getting-started.md) — the first-15-minutes
  walkthrough; includes the Postgres prerequisite.
- [HOW_TO_HUMAN.md](how-to-human.md) — the operator's playbook
  for the full CLI surface.
- [CLI_REFERENCE.md](../reference/cli-reference.md) — every verb plus stable
  exit codes.
- [SPEC.md § State Store](../reference/spec.md#state-store) and
  [SPEC.md § CLI](../reference/spec.md#cli) — the implementation contract.
- [DECISION_LOG.md § D094](../decisions/decision-log.md) — the decision that
  superseded the V1 carve-out.
- [rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md](../rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md)
  — the full RFC including acceptance criteria and the historical migration
  body.
- [rfcs/0048-daemon-side-substrate-migration.md](../rfcs/0048-daemon-side-substrate-migration.md)
  — the RFC that tracked and completed the daemon-side handler port.
