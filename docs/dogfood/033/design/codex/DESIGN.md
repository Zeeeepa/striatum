# RFC 0033 Storage Substrate Implementation Design

author: designer-codex-gpt-5.5-001

Status: draft
Date: 2026-05-11
Target: RFC 0033 storage substrate rewrite for daemon V2

## Scope

This design implements RFC 0033 as a daemon-owned Postgres substrate. It
migrates only daemon state: repository registry rows, client tokens,
capability grants, audit rows, audit segment manifests, scheduler cursors,
daemon metadata, and the request/session substrate needed by RFC 0030.
Repo-local `.striatum/state.sqlite3` remains the authoritative store for V1
run state and is not redesigned here.

Explicitly deferred:

- Bundled, embedded, or Dockerized Postgres distribution. V2 requires system
  Postgres; packaging alternatives need a follow-up RFC.
- Python-to-Go substrate port design. The schema and migrations must be
  language-agnostic, but the Go daemon implementation is D084 follow-up work.
- Any repo-local state DB migration beyond narrow daemon pointers that later
  RFCs may require. RFC 0033 does not replace `.striatum/state.sqlite3`.

## Postgres Schema Layout

Create a daemon-owned package, `src/striatum/daemon_pg/`, with schema DDL in
versioned SQL migrations and Python accessors wrapping `psycopg` 3. Keep V1
SQLite daemon code in place during transition; new daemon V2 code connects via
`STRIATUM_DAEMON_DB_URL`, config file, or `--postgres-url`.

Use UUID text columns for externally referenced ids and `bigint generated
always as identity` for audit sequence ids where ordering matters. Store JSON
payloads as `jsonb`, timestamps as `timestamptz`, hashes as lowercase hex
text constrained to `^[0-9a-f]{64}$`, and enum-like states as `text CHECK`
constraints rather than Postgres enum types so forward migrations stay simple.

Core tables:

```sql
daemon_meta(
  key text primary key,
  value text not null,
  updated_at timestamptz not null default now()
)

schema_meta(
  singleton boolean primary key default true check (singleton),
  schema_version integer not null,
  applied_at timestamptz not null,
  daemon_version text not null
)

repositories(
  repository_id text primary key,
  repo_identity text not null,
  repo_root text not null,
  state_db_path text not null,
  display_name text not null,
  registered_at timestamptz not null,
  removed_at timestamptz,
  last_seen_at timestamptz,
  last_schema_version integer not null,
  state text not null check (state in ('active','missing','disabled','removed')),
  settings_json jsonb not null default '{}'::jsonb
)
```

Add partial unique indexes on `repositories(repo_identity)` and
`repositories(repo_root)` where state is `active`, `missing`, or `disabled`.
This preserves the V1 duplicate-registration behavior while avoiding id reuse
after `repo remove`.

Client and capability tables:

```sql
clients(
  client_id text primary key,
  client_kind text not null,
  display_name text not null,
  token_id text not null unique,
  token_hash text not null,
  token_salt text not null,
  created_at timestamptz not null,
  expires_at timestamptz,
  revoked_at timestamptz,
  last_used_at timestamptz
)

client_capabilities(
  capability_id text primary key,
  client_id text not null references clients(client_id),
  repository_id text references repositories(repository_id),
  capability text not null,
  granted_at timestamptz not null,
  expires_at timestamptz,
  revoked_at timestamptz,
  revoked_reason text
)
```

V1 supports only `read` and `admin`; V2 must allow RFC 0030 and RFC 0032
capabilities (`read`, `write`, `review`, `claim`, `apply`, `admin`,
`recovery`) without another table rebuild. Validate capability vocabulary in
Python and in a migration-owned reference table only if we need DB-side
introspection later.

Audit tables:

```sql
audit_log(
  audit_id bigint generated always as identity primary key,
  timestamp timestamptz not null default now(),
  schema_version integer not null,
  hash_format_version integer not null,
  daemon_version text not null,
  client_id text references clients(client_id),
  repository_id text references repositories(repository_id),
  command text not null,
  authorization_result text not null,
  denial_reason text,
  transport text not null,
  request_id text,
  exit_code integer,
  payload_sha256 text not null,
  previous_hash text,
  row_hash text not null unique,
  segment_id bigint not null references audit_segments(segment_id)
)

audit_segments(
  segment_id bigint generated always as identity primary key,
  opened_at timestamptz not null,
  closed_at timestamptz,
  first_audit_id bigint,
  last_audit_id bigint,
  first_hash text,
  last_hash text,
  previous_segment_id bigint,
  previous_segment_last_hash text,
  next_segment_first_previous_hash text,
  retention_state text not null default 'active',
  state text not null check (state in ('open','closed','purged'))
)

audit_chain_head(
  singleton boolean primary key default true check (singleton),
  last_audit_id bigint references audit_log(audit_id),
  last_hash text,
  updated_at timestamptz not null default now()
)
```

Scheduler and request state:

```sql
scheduler_cursors(
  repository_id text not null references repositories(repository_id),
  run_id text not null,
  cursor_kind text not null,
  last_sweep_at timestamptz,
  next_sweep_after timestamptz,
  last_result_json jsonb,
  state text not null check (state in ('active','in_progress','sweep_degraded','removed')),
  primary key(repository_id, run_id, cursor_kind)
)

rpc_request_log(
  request_id text primary key,
  client_id text references clients(client_id),
  repository_id text references repositories(repository_id),
  method text not null,
  params_hash text not null,
  response_hash text,
  decision text not null check (decision in ('allowed','denied','completed','failed')),
  audit_id bigint references audit_log(audit_id),
  envelope_version integer not null,
  daemon_version text not null,
  schema_version integer not null,
  started_at timestamptz not null,
  finished_at timestamptz,
  expires_at timestamptz not null
)

client_sessions(
  client_session_id text primary key,
  client_id text references clients(client_id),
  transport text not null,
  envelope_version integer not null,
  methods_etag text not null,
  opened_at timestamptz not null,
  last_seen_at timestamptz,
  closed_at timestamptz
)
```

Do not create daemon-owned supervisor or sealed-apply tables in the RFC 0033
implementation slice unless they are inert compatibility placeholders needed
by RFC 0030 tests. RFC 0031 owns their final semantics.

Differences from V1 SQLite:

- Postgres stores daemon state centrally under an operator-installed server,
  not in `striatumd.sqlite3`.
- Hash-chain append behavior moves from SQLite triggers plus file ownership to
  Postgres triggers plus role privileges.
- `schema_version` is explicit daemon metadata and copied into every audit row;
  SQLite `PRAGMA user_version` has no Postgres equivalent.
- `hash_format_version` lets imported V1 audit rows keep byte-equivalent hash
  anchors while new V2 rows include the substrate schema fields in their
  canonical payload.
- Request log and client-session tables are new for daemon RPC; V1 had direct
  registry reads and no real daemon RPC server.
- Capabilities no longer use a SQLite `CHECK` frozen to `read/admin`; the V2
  daemon must accept the RFC 0030/0032 vocabulary.

## Migration Runner

Add `src/striatum/daemon_pg/migrations/` with files named
`0001_baseline.sql`, `0002_...sql`, and a Python registry in
`src/striatum/daemon_pg/migrations.py`.

Migration runner contract:

- `LATEST_DAEMON_DB_VERSION` is derived from the highest registered file.
- Startup opens a Postgres connection, takes an advisory lock such as
  `pg_advisory_lock(hashtext('striatum_daemon_migrations'))`, reads
  `schema_meta.schema_version`, and refuses with exit code 9 when the DB
  version is newer than the binary.
- Pending migrations run in strict order. Each migration runs in its own
  transaction, records a row in `schema_migrations(version, label, sha256,
  applied_at, daemon_version)`, and updates `schema_meta`.
- There are no down migrations. Every migration file must include a comment
  block naming the export/restore strategy for backing out operationally.
- The daemon records the effective schema version on every `audit_log` and
  `rpc_request_log` row at write time.

This mirrors the repo-local SQLite refusal behavior without pretending
Postgres has `PRAGMA user_version`. The shared user-facing rule remains:
older binaries refuse newer storage with exit code 9.

## Audit Chain Mapping

Audit rows are inserted only through a database function, for example
`striatum_append_audit(...)`, owned by a migration/admin role. The daemon role
gets `EXECUTE` on the function and `SELECT` on audit tables, not direct
`UPDATE` or `DELETE`.

Recommended shape:

1. The function runs in a `serializable` transaction.
2. It selects the current open segment row `FOR UPDATE`.
3. It selects the previous audit row by max `audit_id` `FOR SHARE` or reads an
   `audit_chain_head` singleton row `FOR UPDATE`.
4. It canonicalizes the audit payload fields in the exact V1 field order plus
   `schema_version` and `daemon_version`.
5. It computes `row_hash = sha256(previous_hash || canonical_payload)`.
6. It inserts the row, updates open-segment first/last fields, updates
   `audit_chain_head`, and returns `audit_id` and `row_hash`.

Use `pgcrypto.digest(..., 'sha256')` if available. If requiring `pgcrypto`
creates installation friction, compute the canonical hash in Python and have
a trigger independently recompute and compare it when `pgcrypto` is present;
the acceptance criterion should prefer DB-side verification.

Append-only enforcement:

- `striatum_daemon_rw` has `INSERT` only where direct inserts are needed,
  `SELECT` on audit tables, `EXECUTE` on append functions, and no
  `UPDATE`/`DELETE` on audit tables.
- `striatum_daemon_migrator` owns migrations and can rotate/close segments.
- Triggers still reject `UPDATE` and `DELETE` on `audit_log`; closed or purged
  `audit_segments` cannot be changed except through migration/admin functions.
- Doctor verifies the role actually cannot update/delete by inspecting
  `information_schema.role_table_grants` and, in integration tests, by trying
  a rollbacked forbidden update.

Segment manifests preserve V1 continuity: segment N+1's first row has
`previous_hash` equal to segment N's `last_hash`. Retention may purge old row
ranges, but purged segment rows retain anchors and counts so doctor can report
which ranges are anchor-only rather than row-verified.

`daemon doctor` verification:

- Walk retained `audit_segments` in order and check first/last ids, counts,
  first/last hashes, previous-segment continuity, and open/closed state rules.
- Recompute retained row hashes from canonical payloads.
- Report `verified`, `degraded_retention_anchor_only`, or `broken` rather than
  collapsing retention gaps into a pass.

## V1 SQLite To V2 Postgres Cutover

CLI surface:

```text
striatum daemon migrate --from sqlite --to pg --dry-run
striatum daemon migrate --from sqlite --to pg
striatum daemon migrate --from sqlite --to pg --keep-sqlite-readonly
```

Inputs:

- Source SQLite path defaults to current V1 daemon registry discovery
  (`STRIATUM_DAEMON_REGISTRY` or platform default).
- Destination URL uses normal Postgres config resolution.
- The command refuses if Postgres schema is newer than the binary, if the
  source registry has already been migrated, or if destination tables are
  non-empty without an explicit `--resume-from-checkpoint` follow-up flag.

Dry run:

- Opens SQLite read-only and Postgres with migration privileges.
- Reports source registry version, destination schema version, row counts per
  table, active/removed repo counts, active/revoked token counts, audit row
  count, segment count, and the source final audit hash.
- Recomputes the V1 audit chain from SQLite rows and fails before import if
  the source is already broken.

Apply:

1. Apply Postgres migrations.
2. Insert repositories, clients, client capabilities, daemon metadata, and
   scheduler cursors with stable ids. Preserve V1 `repository_id` values by
   converting them to `repo_<integer>` only if the V2 id grammar requires text;
   otherwise use textified V1 ids and document it.
3. Import audit segments.
4. Replay audit rows through a special migration function that preserves V1
   `previous_hash` and `row_hash` exactly while adding `schema_version`.
   Because adding fields would change the hash, store V1 rows with
   `hash_format_version = 1` and V2 rows with `hash_format_version = 2`.
   Doctor then recomputes each row with the proper format. This is necessary
   for byte-equivalent V1 hash anchors.
5. Verify imported final hash equals the V1 final hash and every manifest
   anchor matches.
6. Write a SQLite cutover marker in V1 `daemon_meta`, for example
   `pg_cutover_completed_at`, `pg_cutover_db_fingerprint`, and
   `pg_cutover_final_hash`.
7. After the marker exists, V1 daemon-registry read paths refuse with a clear
   message pointing at Postgres daemon mode. With `--keep-sqlite-readonly`,
   the file remains as a tombstone for audit cross-check; writes are still
   blocked.

Repo-local `.striatum/state.sqlite3` is untouched. Existing direct CLI mode
continues to operate on repo-local state during the transition.

## Daemon Doctor

`striatum daemon doctor` gains a Postgres section. It should run before daemon
startup mutates state and should be callable as an operator preflight:

- Connection: URL resolved, server reachable, database name shown without
  password or token material.
- Version: `server_version_num` meets the V2 floor. Recommend PG 14+ unless
  implementation discovers a concrete PG 15+ feature dependency.
- Extensions: `pgcrypto` present if DB-side hash computation is required.
- Role: current user is not superuser for normal daemon runtime; has required
  `CONNECT`, `USAGE`, `SELECT`, `INSERT`/function `EXECUTE`, and lacks
  `UPDATE`/`DELETE` on audit tables.
- Schema: current daemon DB version, latest binary-supported version, migration
  dirty/clean state, and refusal when DB is newer than binary.
- Audit: chain status, retained-row verification status, segment manifest
  status, final anchor hash.
- Runtime: connection pool settings, request-log retention size, active cursor
  count, degraded cursor count.

Error messages should include platform hints without turning doctor into an
installer:

- macOS/Homebrew: `brew install postgresql@16` and `brew services start
  postgresql@16`.
- Debian/Ubuntu: `sudo apt install postgresql` and `sudo systemctl enable
  --now postgresql`.
- Arch: `sudo pacman -S postgresql`, `sudo -iu postgres initdb -D
  /var/lib/postgres/data`, then `sudo systemctl enable --now postgresql`.
- FreeBSD/pkg: `sudo pkg install postgresql16-server postgresql16-client` and
  the platform service command.
- WSL: use the Linux distro package manager inside WSL; do not point Windows
  native paths at Linux repo roots.

## Test Harness

Use two levels of tests:

- Unit tests for SQL rendering, migration ordering, URL redaction, doctor
  parsing, and V1 audit canonicalization run without Postgres.
- Integration tests require Postgres and are skipped with an explicit reason
  when no server or `pg_ctl` is available.

Preferred local harness:

1. Allocate a temporary data directory.
2. Run `initdb` and start a per-test cluster on a random Unix socket directory
   or localhost port with `pg_ctl`.
3. Create a unique database or schema per test.
4. Set `STRIATUM_DAEMON_DB_URL` for the test process.
5. Terminate connection pools, run `pg_terminate_backend` for the test DB,
   stop `pg_ctl`, and remove the temp directory.

CI may instead provide a shared Postgres service. In that mode, use
schema-per-test isolation (`striatum_test_<uuid>`) plus `search_path`, and drop
the schema with `cascade` at teardown. DB-per-test is closer to production
privilege checks but slower; use it for migration/cutover tests and
schema-per-test for concurrency tests.

Required integration coverage:

- Fresh migration applies to empty Postgres and records schema version.
- Newer DB version refusal exits with code 9.
- V1 SQLite registry import preserves repository/client/capability rows.
- V1 audit chain import has byte-equivalent final hash and segment anchors.
- Post-cutover V1 registry reads refuse and `--keep-sqlite-readonly` blocks
  writes while preserving read-only cross-check data.
- Doctor fails for unsupported PG version, missing `pgcrypto` when required,
  excessive privileges, missing privileges, broken chain, and bad segment
  manifests.
- Teardown leaves no zombie connections and no schemas/databases matching the
  test prefix.

## Concurrency Design

Run daemon transactions at `read committed` by default and raise to
`serializable` only for audit append and cross-row invariants. This avoids
making every status read participate in serializable retry loops.

Audit append:

- Use a single append function that locks `audit_chain_head` or the open
  segment row. Keep the transaction small: authorize, append audit, commit.
- Retry serialization failures with bounded jitter. Duplicate `request_id`
  returns the existing request-log row rather than appending a second audit row
  for the same decision.

Capability lookup:

- Use indexed reads on `clients(token_id)` and
  `client_capabilities(client_id, repository_id, capability)`.
- Token `last_used_at` updates are non-critical and should not sit in the same
  serializable transaction as audit append. Either update after commit or use a
  small independent transaction.
- Revocation uses `SELECT ... FOR UPDATE` on the client/capability rows it
  changes so a request authorizing after revocation sees the revoked state.

Scheduler cursors:

- Sweep workers claim work with `FOR UPDATE SKIP LOCKED` over active cursors.
- Cursor state transitions are short transactions. The sweep itself runs
  outside the cursor lock and writes final state afterward.

Supervisor heartbeats, when RFC 0031 moves them into daemon DB:

- Use an upsert keyed by `supervisor_id` or a narrow update on a single row.
- Do not serialize heartbeat updates behind audit append. Heartbeat audit, if
  needed, should be sampled or metadata-only per existing D085 limits.

Deadlock rule: every transaction that touches multiple families must lock in
this order: client/capability rows, repository row, scheduler/supervisor row,
audit segment/head. Keep repo-local SQLite writes outside Postgres
transactions except for explicit RFC 0032 reconciliation flows.

## Documentation Deltas

`docs/SPEC.md`:

- Add daemon V2 Postgres substrate section.
- State system Postgres is required for daemon V2 and repo-local SQLite is
  unchanged.
- Document daemon DB migration refusal parity with exit code 9.
- Document audit rows carrying substrate schema version.

`docs/MCP.md`:

- Update daemon MCP notes from V1 resources-only direct registry access to V2
  daemon RPC/capability model once RFC 0030 lands.
- Name that MCP audit records go to daemon Postgres and never include request
  bodies or token secrets.

`docs/UBIQUITOUS_LANGUAGE.md`:

- Add daemon DB, daemon DB migration, substrate version, system Postgres, and
  bundled distribution as RFC 0033 terms.
- Preserve the distinction between daemon DB and repo-local state DB.

`docs/CLI_REFERENCE.md`:

- Add `daemon migrate --from sqlite --to pg [--dry-run]
  [--keep-sqlite-readonly]`.
- Add `--postgres-url` and `STRIATUM_DAEMON_DB_URL` precedence.
- Document exit code 9 for newer daemon DB schema, plus doctor failures for
  unreachable/unsupported Postgres.

`docs/HOW_TO_HUMAN.md`:

- Add first-time daemon V2 setup: install Postgres, create role/database, set
  URL, run doctor, run migrate, start daemon.
- Make clear that direct repo-local workflows can still run without daemon V2
  during the transition.

## Operator Onboarding

First-time setup should be copy-pastable but conservative:

```bash
# Example local-only role/database setup as the postgres admin user.
createuser striatum_daemon
createdb --owner striatum_daemon striatumd

export STRIATUM_DAEMON_DB_URL='postgresql://striatum_daemon@localhost/striatumd?sslmode=disable'
striatum daemon doctor
striatum daemon migrate --from sqlite --to pg --dry-run
striatum daemon migrate --from sqlite --to pg --keep-sqlite-readonly
striatum daemon start
```

For password-authenticated installs:

```sql
create role striatum_daemon login password '<choose-local-password>';
create database striatumd owner striatum_daemon;
```

Then:

```bash
export STRIATUM_DAEMON_DB_URL='postgresql://striatum_daemon:<password>@localhost:5432/striatumd?sslmode=disable'
```

Doctor must redact the password when echoing the URL. It should also explain
that the daemon does not start, stop, initialize, upgrade, or back up the
Postgres service itself. Operators own normal Postgres backups; if the daemon
DB is wiped, daemon audit history cannot be reconstructed from repo-local
state.

## Risks

The system-Postgres requirement is the largest product risk. We should not
hide it: `daemon doctor` needs precise, platform-specific failure messages and
the docs need a short path from "I have never installed Postgres" to a working
local DB.

Audit hash compatibility is the main implementation trap. Imported V1 rows
cannot both add new hashed fields and keep byte-equivalent anchors unless the
hash format is versioned. The migration must preserve V1 hash semantics for
imported rows and switch to V2 hashing only for new rows.

Privilege checks can produce false confidence if tests run as a superuser.
Integration tests need a non-superuser daemon role and a separate migrator
role, otherwise append-only enforcement is not being tested.

Postgres transactions must not wrap repo-local SQLite writes casually. The
daemon DB coordinates daemon state; repo-local run state remains a separate
store until RFC 0032 explicitly defines cross-store reconciliation.
