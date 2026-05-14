# PostgreSQL Transition Runbook

This runbook walks an operator from a pre-D094 install through the
post-D094 PostgreSQL-first state model: prerequisites, daemon
configuration, daemon doctor, per-repo migration, verification, and
the documented refusal codes you will hit if a step is skipped.

Read [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) for the broader operator
playbook; read [SPEC.md § State Store](SPEC.md#state-store) and
[SPEC.md § CLI](SPEC.md#cli) for the implementation contract.

## What changed

Two related changes shape the current product:

1. **RFC 0033** put daemon-global state (registry, capabilities,
   audit chain, scheduler cursors, RPC request log) on operator-
   installed system PostgreSQL.
2. **D094 / RFC 0043** supersedes the V1 carve-out that kept
   per-repository workflow state in `.striatum/state.sqlite3`.
   Repo-local workflow tables (`runs`, `jobs`, `sessions`,
   `queue_messages`, `leases`, `work_packets`, `artifacts`,
   `verdicts`, `blockers`, `command_requests`, `process_executions`,
   `events`, `job_worktrees`, `process_supervisors`,
   `process_supervisor_pointers`) move into the same daemon-owned
   Postgres under a `repository_id` scope. The daemon becomes a
   hard prerequisite for every Striatum verb; `--no-daemon` is
   retired. `.striatum/` survives as operational scratch only.

**Remaining work — RFC 0048.** The daemon RPC server's single-repo
business-logic handlers (mutations, recovery, evidence, worktree,
run-summary) still delegate to the historical SQLite-backed CLI
dispatch path under the `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` escape. RFC 0048 ports each handler to
read/write the per-repo Postgres tables directly and removes the
escape. Until those phases land, an operator can still hit
SQLite-backed code paths in test harnesses; see § "RFC 0048
remaining work" below.

## Prerequisites

- A working PostgreSQL service the operator can connect to (system
  package, managed service, or a developer install — Striatum does
  not manage the Postgres lifecycle). Bundled and Dockerized
  distributions are deferred (RFC 0033 §8, inherited by RFC 0043).
- A role that can `CREATE SCHEMA` and `CREATE TABLE`; the daemon
  owns subsequent forward-only migrations and role grants.
- Python 3.11+ and a Striatum install (`pip install
  striatum-orchestrator` or a contributor checkout with
  `make install`).
- For an existing repo: a `.striatum/state.sqlite3` at the highest
  V1 schema version the runner supports. Run `striatum migrate` (if
  needed) before invoking the per-repo migration command.

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
-- CREATE on db + schema is needed for striatum daemon migrate-repo-local
-- (it runs CREATE TABLE IF NOT EXISTS on every invocation, even when the
-- schema is at HEAD).
GRANT CREATE ON DATABASE striatum_daemon TO striatumd_rw;
GRANT CREATE ON SCHEMA striatumd TO striatumd_rw;
SQL
```

Connect the daemon as `striatumd_rw` rather than the database owner.
The next section ("Configure the daemon DB connection") shows the three
surfaces for that connection string.

A future `daemon doctor --provision-rw-role` (RFC 0048 V1.5 follow-up)
will automate this step; until then it is a one-time manual operator
action per Postgres install.

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
striatum daemon start --json &
```

`striatum daemon start` (also exposed as the `striatumd` console
script) is the supported foreground entry point. The startup
envelope prints the bound socket path and the resolved Postgres
URL. CLI verbs route through this daemon for every mutation; there
is no SQLite fallback.

If the daemon is unreachable when a CLI verb runs, the verb refuses
with **exit code 11 (`daemon_unreachable`)** and stderr names the
socket path it tried plus the platform-specific remediation
(`systemctl --user start striatumd` on Linux with the bundled unit;
`launchctl bootstrap` hint on macOS; an explicit `striatumd
--foreground` reminder; Postgres install hints if the daemon was
never started). `striatum --help`, `striatum --version`, and
`striatum daemon doctor` keep working without a daemon because they
touch no state.

## Register the target repo

```bash
striatum repo add /path/to/target --json
```

`repo add` canonicalizes the repository root, derives a
realpath/inode-based identity, and refuses symlink or
path-traversal ambiguity. `--init` is required when no `.striatum/`
exists. `--no-migrate` refuses registration when repo-local
migrations would be needed.

The first `repo add` (or `daemon start`) bootstraps a single admin
token and writes a `0600` runtime fallback file under the
daemon's runtime directory. Treat that file as degraded compared
to an OS keyring.

## Migrate the repo-local workflow state

For repos that already have a populated `.striatum/state.sqlite3`,
the per-repo migration command moves their workflow state into the
daemon-owned Postgres under a `repository_id` scope.

```bash
# 1) Inspect what would be migrated — writes nothing.
striatum daemon migrate-repo-local \
  --from sqlite --to pg \
  --repo /path/to/target \
  --postgres-url "$STRIATUM_DAEMON_DB_URL" \
  --dry-run \
  --json

# 2) Apply the migration. The default keeps the SQLite file as a
#    read-only tombstone for inspection (you can delete it later).
striatum daemon migrate-repo-local \
  --from sqlite --to pg \
  --repo /path/to/target \
  --json
```

V1.6 supports only `--from sqlite --to pg`; both flags are
required. `--repo` defaults to the top-level `--repo` if you set
it on the parent invocation. Without `--postgres-url`, the command
uses the same surfaces as the daemon (`STRIATUM_DAEMON_DB_URL`,
daemon config).

The full-migration command runs inside a single Postgres
`SERIALIZABLE` transaction. It re-anchors the audit chain in
Postgres and verifies it is byte-equivalent to the SQLite original
before commit. After commit it writes the migration sentinel
(`.striatum/state.sqlite3.migrated`) and then finalizes the SQLite
file according to the flags you passed (see next section).

Idempotent re-runs against an already-migrated repo report
`already migrated` with the checkpoint marker timestamp and exit 0.
If a crash interrupts the SQLite finalization, the next invocation
resumes the recorded action — same tombstone behavior, same flags
— from the sentinel; the SQLite is not opened for live state.

If the daemon is unreachable, the command refuses with exit code
11. If the on-disk SQLite schema is older than the runner
supports, the command refuses with exit code 8 and points you at
`striatum migrate`.

## Tombstone vs delete

The migration command finalizes the SQLite file with one of two
behaviors:

- **Tombstone (default, `--keep-sqlite-readonly`).** Renames
  `.striatum/state.sqlite3` to `.striatum/state.sqlite3.tombstone`
  and `chmod`s it to `0444`. No Striatum verb opens the tombstone;
  it remains for operator inspection until you remove it. This is
  the safe path.
- **Delete (`--no-keep-sqlite-readonly --confirm-delete`).** Both
  flags are required. The migration deletes the original file
  after the Postgres commit. Use this when the source data is
  redundant and inspection is not needed.

There is no default destructive path. `--confirm-delete` is the
explicit operator choice for irreversible cleanup; the migration
refuses to delete without it.

## Verify the migration

```bash
# Repo-local workflow state now reads from Postgres. Pre-existing
# runs and jobs remain queryable through the normal CLI surfaces.
striatum --repo /path/to/target status --json
striatum --repo /path/to/target doctor --verbose --json
striatum --repo /path/to/target list runs --json
```

A repo that has not been migrated refuses CLI verbs with **exit
code 12 (`repo_not_migrated`)**. The stderr message points at
`striatum daemon migrate-repo-local --from sqlite --to pg --repo
<path>` with the resolved repo path; the JSON envelope carries a
structured `hint` with the same command shape.

`striatum daemon doctor --json` reports one substrate (Postgres),
one schema version, and one audit chain after a successful
migration. Existing dogfood scaffolds under `docs/dogfood/<NNN>/`
are frozen historical artifacts; their `.striatum/state.sqlite3`
references describe V1 behavior and are not migrated by this
command.

## Documented exit codes

The exit codes RFC 0043 reserves for daemon-required behavior:

| Code | Meaning | Operator remediation |
|---:|---|---|
| 10 | Daemon RPC transport, handshake, or version-skew refusal (RFC 0030). | Reconcile client and daemon versions; rerun `daemon doctor`. |
| 11 | `daemon_unreachable`. | Start the daemon (`striatum daemon start` or the systemd / launchd unit). Check the socket path printed in stderr. |
| 12 | `repo_not_migrated`. | Run `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>` (dry-run first if you want a preview). |

See [CLI_REFERENCE.md § Stable exit codes](CLI_REFERENCE.md#stable-exit-codes)
for the full closed list.

## Rollback and inspection limits

- **Pre-commit dry-run is the safe rollback.** Use `--dry-run` to
  inspect counts and audit-chain anchors before committing to a
  full migration.
- **Tombstone is read-only.** No Striatum verb opens
  `.striatum/state.sqlite3.tombstone`. SQLite tooling can still
  inspect it directly; it remains owner-readable for as long as
  you keep it. You can delete it manually when you no longer want
  the local mirror.
- **There is no "un-migrate" command.** Once the Postgres-side
  transaction commits and the SQLite is tombstoned or deleted, the
  authoritative state is in Postgres. Recovery from a corrupted
  migration uses standard Postgres point-in-time recovery
  (operator-owned), or restoring the tombstone if you kept it.
- **The tombstone is not a live mirror.** It is the pre-cutover
  snapshot. Workflow state mutated after the migration lives in
  Postgres only.

## RFC 0048 remaining work

RFC 0043 V1.6 shipped the schema flip, the migration command, and
the `STRIATUM_DAEMON_REQUIRED` enforcement; it did not finish the
substrate port on the daemon side. RFC 0048 (proposed, V2.0 phase)
covers:

- Porting each `src/striatum/cli/` single-repo handler to read and
  write the daemon's per-repo Postgres tables directly instead of
  delegating to `striatum.api.invoke` and `striatum.db.connect`.
- Implementing the same mutation surface in the Go daemon
  (RFC 0039) so `--core go` services single-repo verbs natively.
- Removing the `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`
  escape once test fixtures move to Postgres.

Until those phases land, any verb that still routes through the
SQLite-backed handler will hit the test-harness escape; production
operators should leave `STRIATUM_DAEMON_REQUIRED` unset (the default
enforces the post-D094 behavior), and treat the escape as a known
gap rather than a configuration option.

## See also

- [GETTING_STARTED.md](GETTING_STARTED.md) — the first-15-minutes
  walkthrough; includes the Postgres prerequisite.
- [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) — the operator's playbook
  for the full CLI surface.
- [CLI_REFERENCE.md](CLI_REFERENCE.md) — every verb plus stable
  exit codes.
- [SPEC.md § State Store](SPEC.md#state-store) and
  [SPEC.md § CLI](SPEC.md#cli) — the implementation contract.
- [DECISION_LOG.md § D094](DECISION_LOG.md) — the decision that
  superseded the V1 carve-out.
- [rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md](rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md)
  — the full RFC including acceptance criteria and the migration
  body.
- [rfcs/0048-daemon-side-substrate-migration.md](rfcs/0048-daemon-side-substrate-migration.md)
  — the V2.0 RFC describing the remaining handler-port work.
