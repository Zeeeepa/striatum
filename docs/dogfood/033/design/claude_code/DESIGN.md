# RFC 0033 Storage Substrate Rewrite — Claude Code Design

author: designer-claude-opus-001

Date: 2026-05-11
Status: design draft for synthesis

## 0. Scope and explicit non-claims

This document designs the V2 daemon storage substrate per RFC 0033: a system
PostgreSQL instance reached by the daemon over a configurable URL, with V1
SQLite-registry data exported into the new schema by an operator-driven
cutover.

What this substrate does **not** claim:

- It does not prove model-token authorship. Apply receipts (RFC 0031) and
  lane attestation (RFC 0026) keep their own scope; the substrate is the
  durable home for those records but does not authenticate the model that
  produced them.
- It does not provide cryptographic non-repudiation against a malicious
  operator. Per RFC 0031's threat model, in scope are over-eager AI agents
  and operator-mistake footguns; a malicious local-root operator who can
  read the daemon's database role credentials or run `pg_dump` against the
  daemon DB is explicitly out of scope.
- It does not change repo-local provenance guarantees.
  `.striatum/state.sqlite3` keeps the same semantics as today (D006/D007);
  the substrate rewrite is for daemon-owned state only.
- It does not bundle PostgreSQL. RFC 0033 §2 defers Dockerized and
  embedded-binary distribution; V2 requires a system-installed Postgres
  and does not manage its lifecycle.
- It does not design the Python→Go core port (D084). The substrate written
  here is the Python `pgx`-via-`psycopg` substrate; a future RFC will
  re-evaluate when the Go core is in scope.

## 1. Trust boundaries

Three trust zones interact with the new substrate. They are kept distinct
in code and in Postgres role grants.

### 1.1 The daemon process

The daemon is the only writer for daemon-owned state. It connects to
Postgres as the `striatumd_app` role with a connection pool sized for the
daemon's typical concurrency (supervisor heartbeats, audit append, RPC
serving, scheduler ticks). The pool is held in memory in the daemon
process; the role credentials live where the daemon's other secrets live
(OS keyring when available, `0600` file in the daemon runtime directory
otherwise).

The daemon does not parse arbitrary SQL on behalf of clients. Every
mutation is a typed call inside `src/striatum/substrate/` (see §9) that
runs prepared statements against `striatumd_app`. RPC clients (RFC 0030)
never see a connection string and never issue SQL.

### 1.2 Postgres roles

Three roles are provisioned by the schema bootstrap migration:

- `striatumd_owner` — owns every object in the daemon schema. Used only
  by `striatum daemon migrate` (forward migrations and bootstrap).
  Privileges include `CREATE`, `ALTER`, `DROP` against the daemon schema.
- `striatumd_app` — the daemon's connect-time role. Has `INSERT, SELECT,
  UPDATE, DELETE` on mutable tables, `INSERT, SELECT` only on audit and
  audit-segment tables, and `SELECT` on read-only views. No DDL.
- `striatumd_readonly` — `SELECT` only across the schema. Reserved for
  future doctor-from-second-process and dashboard-from-third-party (not
  used in V2 client code; provisioned now so `daemon doctor` can warn if
  the operator has dropped it).

Append-only enforcement on audit and segment tables is the role-grant
itself: `striatumd_app` has no `UPDATE` or `DELETE` privilege on those
tables. This is stronger than V1's SQL trigger because a daemon-process
compromise that runs arbitrary statements against `striatumd_app` still
cannot rewrite audit history — Postgres refuses at the role layer. The
trigger is kept as belt-and-suspenders so direct connections by other
roles cannot accidentally violate the invariant either, but the role
grant is the primary defense.

What this does **not** defend against (per §0 and RFC 0031 threat model):

- An operator with `pg_dump` access or the superuser role can rewrite
  anything they want.
- An operator who reads `~/.config/striatum/daemon.toml` and connects to
  Postgres directly with the `striatumd_owner` role can drop tables.

These are operator-zone actions and the substrate does not claim to
prevent them. The substrate **does** make them visible: `daemon doctor`
records the schema state on every run and the audit chain breaks on any
out-of-band write, so reviewers can notice.

### 1.3 The operator

The operator owns both the daemon process and the Postgres instance the
daemon points at. The operator is responsible for:

- installing Postgres at a supported major version (§11);
- providing a connection URL pointing at a database the daemon may
  bootstrap (`STRIATUM_DAEMON_DB_URL`, `~/.config/striatum/daemon.toml`,
  or `--postgres-url`);
- ensuring the daemon role can `CREATE SCHEMA` on first run, or
  pre-creating the schema and granting privileges manually for operators
  who run a managed Postgres they cannot bootstrap (§5.5).

The daemon does **not** manage Postgres lifecycle, restart Postgres, run
`pg_ctl`, or write to PG configuration files. Trust boundary: the daemon
is a client of Postgres; the operator is the administrator of Postgres.
This keeps the daemon a single process and matches the RFC 0033 stance.

## 2. Schema layout

The daemon DB lives in one Postgres schema, `striatumd`, owned by
`striatumd_owner`. The schema is bootstrapped on first daemon startup
(or on `striatum daemon migrate` if the operator runs that explicitly
first).

### 2.1 Schema-version metadata

```sql
CREATE TABLE striatumd.schema_meta (
  key         TEXT PRIMARY KEY,
  value       TEXT NOT NULL
);
-- bootstrap rows:
--   substrate_version : integer string, e.g. '1'
--   instance_id       : opaque 12-hex tag emitted in audit author lines
--   bootstrapped_at   : RFC3339Z timestamp
--   pg_major_at_bootstrap : '14', '15', '16', '17'
```

`substrate_version` is the V2 equivalent of `PRAGMA user_version` on the
V1 registry. Every migration writes the new version into this row in the
same transaction that applies DDL.

### 2.2 Registry tables (V1 carry-over, ported)

The V1 registry tables (`repositories`, `clients`, `client_capabilities`,
`scheduler_cursors`) port forward with minor shape adjustments:

```sql
CREATE TABLE striatumd.repositories (
  repository_id        BIGSERIAL PRIMARY KEY,
  repo_identity        TEXT NOT NULL,
  repo_root            TEXT NOT NULL,
  state_db_path        TEXT NOT NULL,
  display_name         TEXT NOT NULL,
  registered_at        TIMESTAMPTZ NOT NULL,
  removed_at           TIMESTAMPTZ,
  last_seen_at         TIMESTAMPTZ,
  last_schema_version  INTEGER NOT NULL,
  state                TEXT NOT NULL CHECK (state IN ('active','missing','disabled','removed')),
  settings_json        JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE UNIQUE INDEX uq_active_repo_identity
  ON striatumd.repositories(repo_identity)
  WHERE state IN ('active','missing','disabled');
CREATE UNIQUE INDEX uq_active_repo_root
  ON striatumd.repositories(repo_root)
  WHERE state IN ('active','missing','disabled');

CREATE TABLE striatumd.clients (
  client_id     TEXT PRIMARY KEY,
  client_kind   TEXT NOT NULL,
  display_name  TEXT NOT NULL,
  token_id      TEXT NOT NULL UNIQUE,
  token_hash    TEXT NOT NULL,
  token_salt    TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL,
  expires_at    TIMESTAMPTZ,
  revoked_at    TIMESTAMPTZ,
  last_used_at  TIMESTAMPTZ
);

CREATE TABLE striatumd.client_capabilities (
  capability_id   TEXT PRIMARY KEY,
  client_id       TEXT NOT NULL REFERENCES striatumd.clients(client_id),
  repository_id   BIGINT REFERENCES striatumd.repositories(repository_id),
  capability      TEXT NOT NULL CHECK (capability IN ('read','write','review','claim','apply','admin','recovery')),
  scope           TEXT NOT NULL DEFAULT 'repo' CHECK (scope IN ('repo','daemon')),
  granted_at      TIMESTAMPTZ NOT NULL,
  expires_at      TIMESTAMPTZ,
  revoked_at      TIMESTAMPTZ,
  revoked_reason  TEXT
);
CREATE INDEX idx_client_capabilities_client
  ON striatumd.client_capabilities(client_id) WHERE revoked_at IS NULL;

CREATE TABLE striatumd.scheduler_cursors (
  repository_id    BIGINT NOT NULL REFERENCES striatumd.repositories(repository_id),
  run_id           TEXT NOT NULL,
  cursor_kind      TEXT NOT NULL,
  last_sweep_at    TIMESTAMPTZ,
  next_sweep_after TIMESTAMPTZ,
  last_result_json JSONB,
  state            TEXT NOT NULL CHECK (state IN ('active','in_progress','sweep_degraded','removed')),
  PRIMARY KEY(repository_id, run_id, cursor_kind)
);
```

The capability vocabulary widens from V1's `{read, admin}` to the RFC
0030/0032 set `{read, write, review, claim, apply, admin, recovery}`. The
V1 `read` and `admin` tokens migrate unchanged; the new capabilities are
not granted retroactively by the cutover (operators add them explicitly
after migration).

### 2.3 Audit chain and segment manifests

```sql
CREATE TABLE striatumd.audit_log (
  audit_id              BIGSERIAL PRIMARY KEY,
  ts                    TIMESTAMPTZ NOT NULL,
  client_id             TEXT,
  token_hash            TEXT,
  repository_id         BIGINT,
  method                TEXT NOT NULL,
  decision              TEXT NOT NULL CHECK (decision IN ('allowed','denied','revoked')),
  denial_reason         TEXT,
  transport             TEXT NOT NULL,
  request_id            TEXT,
  exit_code             INTEGER,
  params_sha256         TEXT NOT NULL,
  previous_hash         TEXT,
  row_hash              TEXT NOT NULL,
  segment_id            BIGINT NOT NULL,
  substrate_version     INTEGER NOT NULL
);

CREATE TABLE striatumd.audit_segments (
  segment_id                        BIGSERIAL PRIMARY KEY,
  opened_at                         TIMESTAMPTZ NOT NULL,
  closed_at                         TIMESTAMPTZ,
  first_audit_id                    BIGINT,
  last_audit_id                     BIGINT,
  first_hash                        TEXT,
  last_hash                         TEXT,
  previous_segment_id               BIGINT,
  previous_segment_last_hash        TEXT,
  next_segment_first_previous_hash  TEXT,
  retention_state                   TEXT NOT NULL DEFAULT 'active',
  state                             TEXT NOT NULL CHECK (state IN ('open','closed','purged'))
);
```

Two invariants are enforced by Postgres directly:

1. `striatumd_app` has only `INSERT, SELECT` on both tables. No update or
   delete path exists for the daemon connection role.
2. A row-level `BEFORE UPDATE / BEFORE DELETE` trigger raises an
   exception. This is redundant under the role grant but defends against
   any future role expansion that accidentally grants `UPDATE`.

`substrate_version` is denormalized into every audit row. A future
substrate that reads an older row knows which schema-version's row layout
to interpret it under. This matches RFC 0033 §3.

`params_sha256` is the only payload-tied field on the audit row. The
request body itself is in the request log (§2.5), not the audit chain.

### 2.4 `process_supervisors` (RFC 0031)

```sql
CREATE TABLE striatumd.process_supervisors (
  supervisor_id      TEXT PRIMARY KEY,
  session_id         TEXT NOT NULL,
  run_id             TEXT NOT NULL,
  repository_id      BIGINT NOT NULL REFERENCES striatumd.repositories(repository_id),
  adapter            TEXT NOT NULL,
  command_json       JSONB NOT NULL,
  cwd                TEXT NOT NULL,
  scratch_path       TEXT NOT NULL,
  stdin_pipe_path    TEXT,
  pid                INTEGER,
  pid_start_time     TEXT,
  state              TEXT NOT NULL CHECK (state IN ('starting','attached','detached','stopped','lost')),
  started_at         TIMESTAMPTZ NOT NULL,
  attached_at        TIMESTAMPTZ,
  heartbeat_at       TIMESTAMPTZ,
  ended_at           TIMESTAMPTZ,
  stop_reason        TEXT
);
CREATE INDEX idx_supervisors_alive
  ON striatumd.process_supervisors(state, heartbeat_at)
  WHERE state IN ('starting','attached','detached');
```

The daemon is the only writer. Repo-local SQLite keeps a
`process_supervisor_pointers` table per RFC 0031 §1; that table is
**repo-local** and lives in `.striatum/state.sqlite3`, not in the daemon
DB. The pointer is updated by the daemon through its existing repo-local
write path when supervisor state transitions land.

### 2.5 Request log (RFC 0030 §6)

```sql
CREATE TABLE striatumd.request_log (
  request_log_id   BIGSERIAL PRIMARY KEY,
  request_id       TEXT NOT NULL,
  ts_started       TIMESTAMPTZ NOT NULL,
  ts_completed     TIMESTAMPTZ,
  client_id        TEXT,
  repository_id    BIGINT,
  method           TEXT NOT NULL,
  envelope_version INTEGER NOT NULL,
  transport        TEXT NOT NULL,
  daemon_version   TEXT NOT NULL,
  substrate_version INTEGER NOT NULL,
  params_redacted  JSONB,
  response_status  TEXT,
  response_summary JSONB,
  duration_ms      INTEGER
);
CREATE INDEX idx_request_log_ts ON striatumd.request_log(ts_started);
CREATE INDEX idx_request_log_request_id ON striatumd.request_log(request_id);
```

The request log records every RPC envelope (per RFC 0030). It is not
append-only enforced because retention/rotation deletes rows on schedule.
The audit chain stays the integrity-bearing record; the request log is
operator-observability.

`capability_token` secrets are never written. The `client_id` is
recorded; the secret value is not.

### 2.6 Cross-repo coordination (RFC 0032)

```sql
CREATE TABLE striatumd.cross_repo_runs (
  cross_repo_run_id    TEXT PRIMARY KEY,
  workflow_id          TEXT NOT NULL,
  primary_repository_id BIGINT NOT NULL REFERENCES striatumd.repositories(repository_id),
  participating_json   JSONB NOT NULL,
  state                TEXT NOT NULL CHECK (state IN ('preparing','started','completed','aborted')),
  prepared_at          TIMESTAMPTZ NOT NULL,
  started_at           TIMESTAMPTZ,
  ended_at             TIMESTAMPTZ
);
```

The cross-repo run id is namespaced separately from per-repo `run_id`
values per RFC 0032 §1. `state = 'preparing'` is the recovery anchor
RFC 0032 §2 describes; on daemon restart the substrate is read to detect
inconsistent cross-repo runs.

### 2.7 Reference: tables that stay repo-local

Repo-local `.striatum/state.sqlite3` keeps:

- `runs`, `jobs`, `sessions`, `work_packets`, `leases`, `events`,
  `blockers`, `verdicts`, `artifacts`, `process_executions`,
  `job_worktrees`, `process_supervisor_pointers` (new in RFC 0031);
- everything the repo-local schema currently holds for run mechanics.

**Nothing else migrates.** The substrate rewrite is daemon-scoped: it
touches the registry SQLite that lives under `~/.local/state/striatum/`
(or platform equivalent), not the per-repo state DB.

## 3. Forward-only migration model

### 3.1 Migration runner

A new `src/striatum/substrate/migrations.py` mirrors the shape of the
existing `src/striatum/migrations.py` for repo-local SQLite, with three
deliberate differences:

- Each migration is a SQL script committed under
  `src/striatum/substrate/migrations/V###__label.sql` plus an optional
  matching Python hook for data movement (data hooks are kept tiny;
  schema lives in SQL).
- The runner is **forward-only**. There is no symmetric `down` method.
  Rolling back is an export-and-restore exercise (§3.3).
- The runner refuses to run if the on-disk schema version is greater
  than the binary's `LATEST_SUBSTRATE_VERSION`. The refusal exits with
  exit code 9, matching the V1 repo-local convention.

```python
# src/striatum/substrate/migrations.py
LATEST_SUBSTRATE_VERSION: int = 1  # bumped per landed migration

def apply_migrations(conn: psycopg.Connection) -> None:
    current = read_substrate_version(conn)
    if current > LATEST_SUBSTRATE_VERSION:
        raise SchemaVersionError(
            f"daemon DB substrate_version={current} is newer than "
            f"this binary supports ({LATEST_SUBSTRATE_VERSION}); "
            "upgrade the daemon binary or rerun against an older DB."
        )
    pending = [m for m in MIGRATIONS if m.version > current]
    if not pending:
        return
    # forward-only; one transaction per migration.
    for migration in pending:
        with conn.transaction():
            migration.apply(conn)
            conn.execute(
                "UPDATE striatumd.schema_meta SET value = %s "
                "WHERE key = 'substrate_version'",
                (str(migration.version),),
            )
```

### 3.2 Migration shape

Every migration declares:

- `version: int` — strictly increasing, no gaps.
- `label: str` — short slug used in filenames and logs.
- `requires_role: 'owner' | 'app'` — almost always `owner`. Encoded so
  the runner refuses to run a migration that touches DDL with the wrong
  role.
- `apply(conn)` — typically `conn.execute()` of a SQL file plus optional
  Python data movement.

Migration V1 is the baseline that creates the entire schema described in
§2 plus inserts the three bootstrap rows in `schema_meta`. Subsequent
migrations are additive (new tables, new columns, new indices); column
removals and CHECK widening follow the same rebuild-table pattern the
V1 repo-local migrations use, never via destructive in-place changes.

### 3.3 Rollback by export, not by `DOWN`

RFC 0033 §3 commits to "no down migrations." This design implements that
literally: every migration ships with a `docs/SUBSTRATE_MIGRATIONS.md`
entry that names:

- the new tables/columns the migration introduces;
- the export shape an operator can extract from a V_{N+1} daemon to
  reconstruct a V_N-readable backup;
- the `striatum daemon db export --substrate-version N` invocation that
  produces it.

The daemon also writes a pre-migration export to
`~/.local/state/striatum/exports/pre-vN.sql.gz` automatically before
applying any pending migrations (overwritable with `--no-pre-export`
for operators who manage their own backups). The export is the
restoration path: an operator who needs to revert installs the older
daemon binary, drops the schema, restores the export, and starts.

The pre-export is a `pg_dump --schema=striatumd` of the daemon DB.
Bytes are not interpreted by the daemon — Postgres owns the format.

### 3.4 Exit-code-9 refusal on schema-newer-than-binary

The substrate `apply_migrations` call runs at daemon startup. If the
on-disk `substrate_version` exceeds `LATEST_SUBSTRATE_VERSION`:

- the daemon refuses to start;
- `daemon doctor` (run via the older binary, or via the operator's CLI
  client connecting to a non-running daemon — see §8) reports the
  mismatch with the exact integers and the suggested next step;
- exit code 9 is documented in `docs/CLI_REFERENCE.md` alongside the
  existing repo-local schema-newer exit-code-9 convention.

There is no silent downgrade and no "run anyway" flag. The operator
either upgrades the daemon binary, downgrades the DB by restoring an
export, or chooses to run direct-mode CLI (which uses repo-local SQLite
and is independent of the daemon DB).

## 4. Audit chain on Postgres

The V1 audit chain shape (hash-chained rows + closed-segment manifests)
ports verbatim onto Postgres. The integrity model is the same; the
**enforcement** model strengthens.

### 4.1 Hash chain semantics

`row_hash = sha256(canonical_json({ts, client_id, repository_id, method,
decision, denial_reason, transport, request_id, exit_code,
params_sha256, previous_hash, segment_id, substrate_version}))`.

The canonicalization is the existing `json_dumps` (`sort_keys=True`,
no whitespace) so V1 row hashes are byte-equivalent to V2 row hashes for
the same logical content. This is the **byte-equivalent hash anchor**
RFC 0033 §4 acceptance criterion: a row imported from V1 SQLite
re-computes to the same `row_hash` it had in V1.

`segment_id` and `substrate_version` were both implicit in V1 (segment
present, substrate=1-but-implicit). The cutover (§5) records the V1
rows with their original `segment_id` values and with
`substrate_version = 0` (a sentinel meaning "imported from V1
SQLite"). New rows after cutover record `substrate_version = 1` (or
later).

The first new V2 row's `previous_hash` is the last V1 row's `row_hash`.
End-to-end chain continuity holds across the cutover.

### 4.2 Segment manifests

Closed segments record `(first_audit_id, last_audit_id, first_hash,
last_hash, previous_segment_id, previous_segment_last_hash,
next_segment_first_previous_hash)`. The V1 segments port unchanged. The
cutover **closes** the last open V1 segment as part of the export, then
opens a new V2 segment whose `previous_segment_last_hash` matches that
closed segment's `last_hash`.

`daemon doctor` (§8) walks the segment chain from segment 1 forward,
verifying:

- segment chain (`previous_segment_last_hash` agrees with the previous
  segment's `last_hash`);
- row chain inside each segment (every row's `previous_hash` matches the
  prior row's `row_hash`, and the segment's `first_hash`/`last_hash`
  match the rows actually present);
- boundary continuity (the first row of segment N+1 has
  `previous_hash == last row of segment N`).

The verification logic ports from `_audit_chain_records` and
`_audit_segment_records` in V1 `daemon.py` (lines 1019-1088). Verifier
code lives in `src/striatum/substrate/audit.py` and is called by both
`daemon doctor` and the post-cutover verification step (§5.4).

### 4.3 Append-only enforcement by role

`striatumd_owner` grants on `audit_log` and `audit_segments`:

```sql
GRANT INSERT, SELECT ON striatumd.audit_log TO striatumd_app;
GRANT INSERT, SELECT ON striatumd.audit_segments TO striatumd_app;
-- explicitly DO NOT grant UPDATE or DELETE.
```

A daemon-process bug that issues `UPDATE audit_log SET ...` against
`striatumd_app` fails at the role-grant layer:
`ERROR: permission denied for table audit_log`. The exception is
caught and surfaced by `daemon doctor` as a bug indicator (this should
never happen in correct daemon code).

For belt-and-suspenders, BEFORE UPDATE/DELETE triggers also raise. A
hypothetical role expansion that accidentally grants `UPDATE` would still
be refused at the trigger layer, with the trigger error pointing at the
role-grant invariant.

`audit_segments` updates are permitted only on rows where `state =
'open'` (closing a segment is a state transition the daemon must run);
this is enforced by a `BEFORE UPDATE` trigger consistent with the V1
trigger.

## 5. V1 SQLite registry → V2 Postgres cutover

The migration is a one-time, operator-driven event. Once cut over, V1
SQLite registry reads are refused.

### 5.1 CLI shape

```text
striatum daemon migrate --from sqlite --to pg \
  --postgres-url $STRIATUM_DAEMON_DB_URL \
  --dry-run             # enumerates rows, runs checks, writes no DB rows
striatum daemon migrate --from sqlite --to pg \
  --postgres-url $STRIATUM_DAEMON_DB_URL
striatum daemon migrate --from sqlite --to pg \
  --postgres-url $STRIATUM_DAEMON_DB_URL --keep-sqlite-readonly
```

### 5.2 Refuse-during-active-supervisors

The migrate command **refuses** to run while:

- A V1 daemon is running (PID file present, process alive on that PID).
  This is the simple check that catches most accidents.
- Any V1 `scheduler_cursors` row has `state = 'in_progress'`, indicating
  a sweep is mid-flight.
- (RFC 0031, post-supervisor migration) Any
  `process_supervisor_pointers` row in any registered repo-local SQLite
  shows `state IN ('starting','attached')`. The migrate command queries
  the V1 registry for `repositories.state = 'active'` repos, opens each
  one's `.striatum/state.sqlite3`, and checks. If any supervisor is
  alive, the command refuses.

The operator UX is **stop daemon, run migrate, start daemon**, not
"hot-swap mid-flight." This is explicit in the documentation and
enforced by code. The rationale:

- We do not have a mechanism to coordinate a substrate switch with an
  active daemon process that already holds connections to the V1
  registry SQLite.
- The cutover takes seconds for a normal-sized registry (V1 registry
  rarely exceeds 10k audit rows). The user-facing pause is acceptable.
- "Hot-swap" would require dual-write windows or active failover
  primitives we are explicitly choosing not to design. RFC 0033 §4 says
  "checkpoint marker, then refuse"; this design honors that.

The error message on refusal names the specific blocking condition and
the recovery command:

```text
$ striatum daemon migrate --from sqlite --to pg ...
error: daemon is running (pid 4124); stop the daemon before migrating.
       run: striatum daemon stop
exit code: 14
```

### 5.3 Cutover steps

When prerequisites pass, the migrate command:

1. **Bootstrap V2 schema.** Connects to Postgres as `striatumd_owner`,
   creates the `striatumd` schema if missing, runs
   `apply_migrations()` to V_LATEST. Records
   `schema_meta.bootstrapped_at = utc_now()`,
   `pg_major_at_bootstrap = server_major_version`.

2. **Lock V1.** Opens the V1 registry SQLite with `BEGIN IMMEDIATE`,
   reads everything, holds the lock until step 5. (V1 daemon is
   already stopped per §5.2; this lock catches concurrent CLI direct
   reads.)

3. **Import.** Inside a single V2 transaction, copies rows table-by-
   table in dependency order: `repositories` → `clients` →
   `client_capabilities` → `scheduler_cursors` → `audit_segments` →
   `audit_log`. `BIGSERIAL` columns are written with their original V1
   integer ids using `OVERRIDING SYSTEM VALUE`; the sequence is then
   advanced past the imported max.

4. **Verify chain end-to-end.** Calls the audit verifier (§4.2) over the
   imported rows. If verification fails, the V2 transaction rolls back
   and the V1 file is untouched. The verifier reports the first broken
   row id and exits non-zero (exit code 15: cutover verification
   failed).

5. **Close V1 segment, open V2 segment.** Closes the V1 last-open
   segment with its current `last_hash`; opens a new V2 segment whose
   `previous_segment_last_hash` equals that close hash. The V1
   `audit_segments.next_segment_first_previous_hash` row is filled in
   so the V1 file remains chain-consistent for any future export
   verification.

6. **Write checkpoint marker.** Writes
   `~/.local/state/striatum/cutover.json` with:
   - the timestamp;
   - V1 SQLite path and inode;
   - V1 last-row hash;
   - V2 first-row's `previous_hash` (must equal V1 last-row hash);
   - operator note (optional, supplied via `--note "..."`);
   - schema version of V1 registry at cutover (must be the latest V1
     value; see §5.4);
   - Postgres major version + database name.
   File is `0600`.

7. **Mark V1 read-refused.** Renames the V1 registry SQLite from
   `striatumd.sqlite3` to `striatumd.sqlite3.migrated-YYYYMMDDTHHMMSSZ`
   unless `--keep-sqlite-readonly` was passed. If that flag is set,
   leaves the file in place with `chmod 0400` and emits a deprecation
   notice ("V1 SQLite is tombstoned but readable for audit cross-check;
   it will be unlinked on the V2.1 upgrade").

8. **Release V1 lock and commit V2 transaction.**

9. **Print summary.** Reports rows imported per table, first/last audit
   IDs, the byte-equivalent `last_hash` check, and the path to the
   checkpoint marker.

### 5.4 Refuse-cutover on V1-not-current

If the V1 registry's `PRAGMA user_version` differs from
`REGISTRY_VERSION` (V1's latest), the cutover refuses and asks the
operator to upgrade the V1 daemon first. Reason: cutover assumes the
canonical V1 row shape. We do not maintain a back-catalog of V1 schemas
that all map onto the V2 schema; we maintain only the current one.

### 5.5 Operators on managed Postgres

Some operators (rare for V2 but worth documenting now) run a Postgres
they cannot bootstrap with `CREATE SCHEMA` privileges. For those:

- The cutover supports `--no-bootstrap-schema`. When set, the operator
  must have already created the `striatumd` schema and granted
  `striatumd_app` the privileges from §1.2.
- `daemon doctor` includes a "manual-bootstrap" check that re-verifies
  the privileges match the spec; missing privileges produce a
  documented warning rather than crashing the daemon.

This is documented in `docs/HOW_TO_HUMAN.md` as the advanced-operator
path. V2 default is "daemon role can bootstrap."

### 5.6 Refusing V1 reads after cutover

After the cutover marker is present, `connect_registry()` (the V1
function in `daemon.py`) refuses to open the V1 SQLite at all. The
substitute path is the V2 substrate module; the V1 import is kept only
for the `striatum daemon migrate` command itself (which obviously must
read V1 to import it).

A clear error directs operators to the V2 path:

```text
$ striatum daemon status
error: V1 daemon registry has been migrated. Use the V2 daemon
       (STRIATUM_DAEMON_DB_URL set or daemon.toml present).
       Cutover marker: ~/.local/state/striatum/cutover.json
exit code: 13
```

## 6. Interplay with RFC 0030 (RPC) and RFC 0031 (supervisors)

### 6.1 RFC 0030 — audit + request log

RFC 0030 §5 audit rows and RFC 0030 §6 request log rows live in this
substrate. Every RPC call writes both:

- one audit row in `striatumd.audit_log` (decision-time, chained);
- one request-log row in `striatumd.request_log` (operator-observability,
  rotatable).

Both writes happen in the same daemon-side transaction so a crash mid-
write does not leave the audit chain inconsistent with the request log.
The two tables share `request_id` and `client_id` for join-time
correlation.

### 6.2 RFC 0031 — daemon-owned supervisors

RFC 0031 §1 introduces `process_supervisors` in the daemon DB and a thin
`process_supervisor_pointers` table in repo-local SQLite. The daemon-side
row is authoritative; the repo-local pointer exists only so repo-local
code can detect "does this session have a supervisor?" without
contacting the daemon.

When the daemon-side row state changes, the daemon writes the new state
to the daemon DB **first**, then issues a pointer-update call against
the repo-local SQLite. If the second write fails (repo missing on disk,
DB locked beyond retry), the daemon DB row is the source of truth and
the pointer is reconciled on the next daemon-doctor pass.

This ordering is the explicit invariant the substrate guarantees: the
daemon DB is the authoritative record; repo-local pointers are derived
projections.

### 6.3 Sealed-apply receipts

RFC 0031 §4 specifies apply receipts. They live in `striatumd.audit_log`
plus a Markdown evidence artifact in the run's evidence path. No new
substrate table; receipts are audit-chain rows with method
`apply.reviewed_patch` and a structured params payload whose
`params_sha256` digest the Markdown artifact references.

The signing key id is recorded in the audit row's `params_sha256` material
(via a JSON field), not as a separate column — keeping the chain shape
stable across new feature additions.

## 7. `striatum daemon migrate` and in-flight daemon interaction

§5 covered the cutover from V1→V2. This section covers the broader
interaction between `daemon migrate` and a running daemon for all future
migrations (V_N → V_{N+1}).

### 7.1 Forward migrations during startup

The daemon **applies forward migrations during its own startup** when
`substrate_version < LATEST_SUBSTRATE_VERSION`. The migration runs as
`striatumd_owner` (the daemon configures both roles at startup, uses
`owner` for migrations only, then drops to `app`).

No separate `daemon migrate` command is needed for routine forward
migrations. The startup migration:

- writes the pre-migration export (§3.3) before applying;
- applies one transaction per migration;
- logs the migrations applied and the resulting version;
- starts the daemon's RPC server only after migration completes
  successfully.

### 7.2 `daemon migrate` as explicit command

`striatum daemon migrate` (without `--from`/`--to` flags) is an
operator-explicit "do the migration but do not start the daemon" path.
Useful for:

- pre-deploy checks: run migrate on an upgraded binary before flipping
  the service-manager unit to the new version;
- export-and-rollback rehearsals.

The command exits 0 on success with a summary, exits 9 on schema-newer-
than-binary, exits 14 on refuse-during-active-daemon, exits 15 on
verification failure.

### 7.3 Refuse-during-active-daemon for forward migrations

The same "refuse if daemon running" rule applies to all forward
migrations, not just the V1→V2 cutover. Reason: forward migrations are
DDL; a running daemon holds long-lived `striatumd_app` connections that
may hold MVCC snapshots inconsistent with the DDL. Stopping the daemon
first is the safe sequence.

In a future RFC, "rolling forward without daemon stop" may become
worthwhile (with extension-version coordination and named statements).
V2 does not attempt it; the cost is one daemon-stop per substrate
upgrade.

### 7.4 Two daemons against the same DB

The daemon **refuses to start** if another daemon is already running
against the same database. Detection:

- The daemon takes an advisory lock at startup:
  `SELECT pg_try_advisory_lock(hashtext('striatumd:singleton'))`.
  If the lock is held by another session, the daemon refuses with exit
  code 16 ("another striatumd appears to be running against this
  database; advisory lock is held by pid X").
- The lock is released when the daemon's connection closes.

This protects against accidental double-start (two `striatumd` invocations
on the same host) and against partial-coordination cases (one host with
two service-manager units).

`daemon doctor` reports the lock state ("singleton lock held by this
daemon at startup") as a positive check.

## 8. Failure modes and what `daemon doctor` catches

### 8.1 Operator wipes the daemon DB

If the operator runs `DROP SCHEMA striatumd CASCADE` (or `dropdb`),
the next daemon startup finds no `schema_meta` row. The daemon emits:

```text
error: daemon DB has no striatumd schema. If you intended to wipe the
       daemon DB, the audit chain cannot be reconstructed from
       repo-local state — those records are gone. To start fresh, run:
         striatum daemon bootstrap --acknowledge-wipe
exit code: 17
```

The `--acknowledge-wipe` flag (RFC 0033 §9) initializes a fresh schema
and records a `schema_meta.wipe_history` JSONB array entry with the
timestamp and an operator-supplied reason. The audit chain in the
restarted daemon begins with `previous_hash = NULL`; reviewers can see
the discontinuity. The substrate **does not** reconstruct the chain;
the operator-acknowledged wipe is recorded as a one-way event.

This matches RFC 0033's "audit cannot be reconstructed from repo-local
state" open question.

### 8.2 Operator points the daemon at the wrong Postgres role

If `STRIATUM_DAEMON_DB_URL` resolves to a Postgres instance the daemon
can connect to but as the wrong role (no `striatumd` schema, no
privileges):

- `daemon doctor` (run via the V2 CLI client when no daemon is up:
  invokes the same substrate routines directly) reports:
  - "schema striatumd not present and role cannot CREATE SCHEMA"
  - "role striatumd_app missing required privileges on audit_log"
- `daemon start` refuses with exit code 18 ("daemon role lacks required
  privileges; see daemon doctor").

If `STRIATUM_DAEMON_DB_URL` is the right database but the daemon was
**previously bootstrapped against a different database** (the
`schema_meta.bootstrapped_at` exists but does not match the cutover
marker), `daemon doctor` warns and the daemon refuses with exit code 19
("daemon DB has different bootstrap timestamp than recorded cutover
marker; this is the wrong database or the marker is stale").

### 8.3 Operator points daemons at the same DB from two hosts

The advisory-lock check (§7.4) catches this when both daemons can reach
the DB. Both daemons attempting `pg_try_advisory_lock` — the second
fails and exits 16.

If one host runs a daemon and the other host runs only the CLI, the CLI
goes through the daemon's RPC and the topology stays single-writer.

### 8.4 Postgres major-version mismatch

`schema_meta.pg_major_at_bootstrap` records the major version at
bootstrap. On every startup the daemon compares the running PG major
against the recorded one:

- Same major → no concern;
- Newer major (operator did a PG major upgrade) → emit a doctor
  warning and update `pg_major_at_bootstrap` after running the V_LATEST
  re-verification pass;
- Older major (operator downgraded PG) → refuse with exit code 20.

The daemon binary itself declares its `MIN_SUPPORTED_PG_MAJOR` (start
at 14 per RFC 0033 §11). PG version below that floor refuses with exit
code 21 with platform-specific install hints (§11).

### 8.5 Audit chain integrity failure

The daemon doctor walks the chain on every `doctor` invocation. Any
break is reported with the audit_id of the first failure and the
expected vs actual hash. The daemon does **not** auto-repair; this is
out-of-band manual operator intervention with a fresh export.

### 8.6 Cutover marker present but Postgres unreachable

`~/.local/state/striatum/cutover.json` exists but the daemon DB is not
reachable: the V1 SQLite is **also** refused (per §5.6). The daemon
refuses to start with exit code 22 ("cutover marker present but PG
unreachable; restore PG connectivity or remove the marker after
restoring V1 SQLite from backup").

### 8.7 What `daemon doctor` reports

The V2 `daemon doctor` (the V1 verification logic ported to substrate
calls) produces:

- substrate version (current schema_meta) vs binary supported version;
- audit chain integrity (rows + segment manifests);
- role privileges check (the three roles, expected GRANTs);
- singleton-lock status;
- PG major version + recorded bootstrap PG major;
- registered repos and per-repo `.striatum/state.sqlite3` reachability;
- cutover marker status and consistency with bootstrap row;
- request log size and rotation state;
- (RFC 0031) supervisor-pointer consistency: walk every registered
  repo's pointer rows and check the daemon-side row exists and agrees;
- (RFC 0032) cross-repo runs in `state = 'preparing'` for longer than
  a documented threshold (e.g. 5 minutes) — these are recovery
  candidates.

`daemon doctor` runs **without** the daemon up (against a non-running
daemon process) by directly opening the substrate module. This is the
recovery diagnostic path; running it against a live daemon goes through
the daemon RPC.

## 9. Concrete touch points in `src/striatum/`

### 9.1 New modules

- `src/striatum/substrate/` — new package. Modules:
  - `substrate/__init__.py` — public API (`connect()`, `apply_migrations()`,
    role-bootstrap, version checks).
  - `substrate/migrations.py` — the forward-only runner described in §3.
  - `substrate/migrations/V001__baseline.sql` — DDL for §2.
  - `substrate/migrations/V001__baseline_data.py` — bootstrap `schema_meta`
    rows.
  - `substrate/registry.py` — registry tables (repositories, clients,
    capabilities, scheduler cursors) replacing the V1 functions in
    `daemon.py`.
  - `substrate/audit.py` — append, hash chain, segment management, doctor
    verifiers.
  - `substrate/request_log.py` — RFC 0030 request log writer and rotation.
  - `substrate/supervisors.py` — RFC 0031 `process_supervisors` table
    accessors.
  - `substrate/cross_repo.py` — RFC 0032 cross-repo run records.
  - `substrate/cutover.py` — V1 SQLite → V2 PG import (§5).
  - `substrate/doctor.py` — V2 daemon-doctor records (§8.7).
- `src/striatum/substrate/migrations/` — one SQL file per migration.
- `src/striatum/cli/daemon_migrate.py` — operator CLI surface for
  `striatum daemon migrate` (§5, §7.2).
- `src/striatum/cli/daemon_bootstrap.py` — operator CLI surface for
  `daemon bootstrap --acknowledge-wipe` (§8.1).
- `tests/substrate/` — new test directory:
  - `test_substrate_migrations.py`
  - `test_substrate_audit_chain.py`
  - `test_cutover_v1_to_v2.py`
  - `test_substrate_doctor.py`
  - `test_substrate_singleton_lock.py`
  - `test_substrate_role_privileges.py`
  - `tests/substrate/_pg_harness.py` — per-test PG data directory.

### 9.2 Modules that change

- `src/striatum/daemon.py` — kept for V1 SQLite support only during the
  cutover transition. After cutover, every call delegates to
  `substrate.*`. The `connect_registry()` function gains a cutover-marker
  check that refuses V1 SQLite reads (§5.6). The V1 doctor logic stays
  callable by the cutover code path but is wrapped by the new
  `substrate.doctor` for live V2 operation.
- `src/striatum/migrations.py` — unchanged. It owns the **repo-local**
  SQLite schema for `.striatum/state.sqlite3`. This is deliberately
  not affected by the substrate rewrite.
- `src/striatum/db.py` — unchanged. Repo-local SQLite stays the same.
- `src/striatum/schema.py` — unchanged. Repo-local baseline schema.
- `src/striatum/cli/dispatch.py` — adds the `daemon migrate` and
  `daemon bootstrap` routes; existing daemon routes stay (they now
  route through the substrate module post-cutover).
- `src/striatum/recovery/auto.py` — unchanged. Recovery operates against
  repo-local SQLite; daemon-side coordination (scheduler cursors) is
  now in PG via `substrate.registry`.
- `src/striatum/mcp.py` — gains a substrate-vs-V1 branch for the
  registry read paths. Post-cutover, MCP resource reads go through
  `substrate.registry`.
- `src/striatum/errors.py` — new exit codes 14–22 documented; the
  existing `SchemaVersionError` continues to surface exit code 9 from
  the substrate runner.
- `pyproject.toml` — adds `psycopg[binary]>=3.1` to dependencies. The
  binary build ships a libpq so operators do not need a separate
  install; this is OK by RFC 0033 §8 because Postgres-the-server is
  not bundled (only the client library to talk to it).

### 9.3 Configuration sources

The daemon resolves its Postgres URL from, in order:

1. `--postgres-url` CLI flag (highest priority);
2. `STRIATUM_DAEMON_DB_URL` environment variable;
3. `~/.config/striatum/daemon.toml`'s `postgres_url` field.

The TOML file is `0600` and may also carry connection-pool sizing
(`pool_min`, `pool_max`), the `striatumd_app` and `striatumd_owner`
role names (defaults are documented), and PG-version-floor override
(off by default; only for operators who knowingly run an older PG and
accept the support warning).

The `daemon doctor` step prints the resolved URL with the password
redacted to confirm to the operator that the right URL is in use.

## 10. Test harness shape

### 10.1 Per-test Postgres data directory

A `tests/substrate/_pg_harness.py` module provides a fixture that:

- creates a per-test `tmp_path/pgdata/` and runs `initdb` (skipped when
  `STRIATUM_TEST_PG_URL` is set, see §10.3);
- starts Postgres on a free local port with `unix_socket_directories =
  tmp_path/pgsockets`, listening on the Unix socket only;
- creates the test database and the three roles per §1.2;
- yields a connection URL;
- on teardown stops the postmaster and deletes `pgdata/`.

The harness uses the system `pg_ctl` / `initdb` binaries. Tests skip if
they are not in `PATH`; this is documented as a test-suite prerequisite
and surfaced by `make doctor` (a new tiny Make target).

### 10.2 Integration assertions

The acceptance criteria from RFC 0033 are codified as tests:

- `test_substrate_migrations_apply_forward` — V0 → V1 → V2 chain runs
  cleanly; `LATEST_SUBSTRATE_VERSION+1` simulated DB refuses with exit
  code 9.
- `test_cutover_byte_equivalent_hashes` — fixture V1 SQLite with audit
  rows; cutover; assert every imported row's `row_hash` is unchanged
  and the first new V2 row's `previous_hash` equals the V1 last-row
  hash.
- `test_cutover_refuse_active_daemon` — V1 daemon running; cutover
  refuses with exit 14.
- `test_cutover_refuse_active_supervisor` — V1 supervisor in `attached`
  state; cutover refuses with exit 14.
- `test_substrate_v1_read_refused_after_cutover` — V1 file untouched
  after `--keep-sqlite-readonly`; V1 read paths refuse with exit 13.
- `test_singleton_advisory_lock` — second daemon refuses with exit 16.
- `test_audit_append_role_refuses_update` — connect as
  `striatumd_app`, attempt `UPDATE audit_log`; assert permission-denied
  exception.
- `test_pg_major_floor` — start against PG older than the floor;
  refuse exit 21.
- `test_doctor_reports_segment_manifest` — open and closed segments
  recorded correctly; doctor reports clean.
- `test_concurrency_supervisor_heartbeat` — two concurrent transactions
  updating different supervisors do not deadlock under default
  isolation.
- `test_concurrency_audit_append` — two parallel audit appends serialize
  correctly under SERIALIZABLE; one retries on conflict and both
  succeed eventually.

### 10.3 CI and developer ergonomics

- `make smoke` adds a substrate-bootstrap-and-cutover smoke that does
  not require a system Postgres: it skips if `pg_ctl` is unavailable.
- `STRIATUM_TEST_PG_URL` lets CI point tests at a shared throwaway
  Postgres; tests still isolate by creating per-test schemas, dropping
  them on teardown, and asserting no leftover schemas/zombie
  connections.
- A `tests/conftest.py` fixture skips substrate tests with a clear
  message when the binary or env are missing, rather than producing
  hard failures.

## 11. Operator-onboarding UX for system Postgres

The operator-first-time experience is the riskiest UX in V2. The design
goal is "the daemon doctor knows what to tell you, by platform."

### 11.1 Hints from `daemon doctor`

When PG is unreachable, `daemon doctor` detects the host platform and
emits an install hint:

- **macOS** (Homebrew available): `brew install postgresql@16 && brew
  services start postgresql@16`
- **Debian/Ubuntu**: `sudo apt install postgresql && sudo systemctl
  enable --now postgresql`
- **Fedora/RHEL**: `sudo dnf install postgresql-server && sudo
  postgresql-setup --initdb && sudo systemctl enable --now postgresql`
- **Arch**: `sudo pacman -S postgresql` plus `initdb` instructions.
- **WSL**: same as the Debian path; document the `wsl --shutdown` step
  if the operator changes PG service state.
- **Unknown**: a generic link to upstream Postgres install docs and the
  spec for required role privileges.

These hints are not magic — they are static strings keyed on
`platform.system()` and `/etc/os-release`. The hint includes the next
command in the sequence:

```text
$ daemon doctor
striatum daemon doctor

problems:
  - postgres unreachable at $STRIATUM_DAEMON_DB_URL
  - schema striatumd not present
hints:
  - install Postgres 16: `brew install postgresql@16`
  - start service: `brew services start postgresql@16`
  - create db: `createdb striatumd`
  - set STRIATUM_DAEMON_DB_URL=postgres://localhost/striatumd
  - run: striatum daemon migrate (or daemon start to bootstrap)
```

### 11.2 First-run bootstrap

If the URL resolves to a reachable PG and the daemon role can `CREATE
SCHEMA`, the first `striatum daemon start` bootstraps. There is no
separate "install" step. This is the V2 onboarding default.

For operators who pre-create the schema manually (managed Postgres),
they run `striatum daemon migrate --no-bootstrap-schema` once. Both
paths converge on the same `schema_meta` content.

### 11.3 Documentation that must update

The implementation should update (per RFC 0033 §11):

- `docs/SPEC.md` — substrate section; system Postgres requirement;
  `STRIATUM_DAEMON_DB_URL`; cutover UX; exit codes 9, 13–22.
- `docs/MCP.md` — daemon MCP now backed by substrate, capability set
  widened.
- `docs/UBIQUITOUS_LANGUAGE.md` — substrate vocabulary from RFC 0033
  §"Domain Modeling".
- `docs/CLI_REFERENCE.md` — `daemon migrate`, `daemon bootstrap`,
  `daemon doctor` expanded fields.
- `docs/HOW_TO_HUMAN.md` — first-run install walkthrough per platform.
- `docs/SUBSTRATE_MIGRATIONS.md` — new; records export shapes per §3.3.
- `CHANGELOG.md` — V2 substrate, exit codes, cutover instructions,
  deprecations (env-var token fallback was retired in dogfood-031;
  V1 SQLite registry is retired in this RFC).

## 12. Open questions for synthesis

These are points where the synthesis pass should pick concretely; this
design records the lean rather than asserting.

- **psycopg vs asyncpg.** This design uses synchronous `psycopg[binary]`
  3.1+. The daemon is multi-threaded today and not async-native; moving
  to async is an orthogonal concern. Synthesis may take the Go-port
  argument (D084) to mean "design as if async is coming soon"; this
  design says "stay sync, port if Go pulls async with it." Reviewers
  should push back if async-native is required by the RPC server work
  (RFC 0030).
- **Pool sizing defaults.** Suggested defaults: `pool_min = 2`,
  `pool_max = 16` for the V2 single-user workstation. Synthesis should
  validate against expected supervisor count and RPC fan-out.
- **PG floor major.** RFC 0033 OQ recommends PG 14. This design adopts
  14 explicitly. If a single dev machine is stuck on PG 13 in 2026 the
  install hint covers it; revisit if reviewers push for an earlier
  floor.
- **Cutover marker location.** `~/.local/state/striatum/cutover.json`
  (XDG state on Linux) and `~/Library/Application Support/striatum/`
  on macOS, mirroring V1 registry path resolution. Synthesis confirms
  no operator-confusion risk.
- **Sequence advancement for BIGSERIAL on import.** This design uses
  `OVERRIDING SYSTEM VALUE` plus `setval(...)` after import. Synthesis
  validates the PG idiom is the same across PG 14–17.
- **Wipe-acknowledgment durability.** §8.1 records the wipe in the
  freshly-bootstrapped `schema_meta.wipe_history`. Reviewers may want
  a paranoid mode that requires the operator to keep the prior
  cutover marker; we propose recording it but not requiring it.
- **Single-process Postgres.** §1.3 says the daemon does not manage PG.
  If a future RFC adds a bundled distribution, this section may need a
  partner update; out of scope for V2.
