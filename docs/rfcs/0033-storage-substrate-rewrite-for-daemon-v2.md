# RFC 0033: Storage Substrate Rewrite for Daemon V2

Status: proposed
Date: 2026-05-11
Context:
[`RFC 0028`](0028-long-running-daemon-and-multi-repository-control-plane.md),
[`RFC 0030`](0030-daemon-rpc-server-and-version-skew-protocol.md) (proposed),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D082, D084, D086, D006, D007),
[`docs/SPEC.md`](../SPEC.md) § "Local SQLite",
`src/striatum/daemon.py`,
`src/striatum/migrations.py`

RFC 0033 lands first in the daemon V2 follow-up sequence because RFC 0030's
wire protocol, schema migrations, and audit-chain format all key off the
substrate choice.

## Problem

D086 commits to a non-SQLite substrate for daemon-owned state in V2. RFC
0028 V1 shipped daemon registry storage as a second SQLite file because the
acceptance-criteria slice was narrow (registry plus foreground sweep). With
D082 (daemon-first product) and D084 (Go core planned), the daemon-owned
state surface grows:

- supervised process ownership and liveness checkpoints (RFC 0031);
- capability tokens, audit hash chain, audit segment manifests, and
  retention/rotation;
- daemon RPC request log, version handshake state, client session state
  (RFC 0030);
- cross-repository scheduler cursors, transaction coordination state
  (RFC 0032);
- daemon-mediated mutation queue and outbox.

Each of those grows reads and writes against the registry. The daemon also
becomes a long-running process that must survive crashes, hot-restarts, and
eventual port to Go. SQLite is fine for a repo-local short-lived CLI; it is
not the right substrate for a daemon-first product with multi-tenant
queries, streaming projections, and resident process supervision.

Repo-local `.striatum/state.sqlite3` stays. The substrate rewrite is for
daemon-owned state only.

## Goals

- Pick a single concrete substrate for daemon V2 state.
- Define a schema-versioning model that survives a future Python-to-Go port
  of the daemon core (D084).
- Define the audit-chain format on the new substrate so that segment
  manifests, hash anchors, and retention semantics survive intact.
- Specify migration from the V1 daemon registry SQLite to the new
  substrate, including operator UX for in-place upgrade.
- Specify test infrastructure: how does the test suite spin the new
  substrate up and down deterministically?
- Preserve the repo-local SQLite model. The substrate rewrite must not
  bleed into `.striatum/state.sqlite3` semantics.

## Non-Goals

- Replacing repo-local `.striatum/state.sqlite3`. That stays SQLite under
  D006/D007 unless a future RFC explicitly proposes change.
- Introducing a hosted or networked storage tier. Local-first is preserved.
- Defining the daemon RPC wire protocol. That lives in RFC 0030 and
  references this RFC for the persistence model.
- Defining sealed-mode containment. That lives in RFC 0031.

## Proposal

### 1. Substrate options

#### Option A: Stay on SQLite

Keep the V1 daemon registry SQLite shape and accept the limits. Rejected
by D086, but documented here because reviewers will (and should) ask
whether D086 was premature.

Benefits:
- No rewrite cost; modern SQLite Go drivers (`modernc/sqlite`,
  `mattn/go-sqlite3`) are stable.
- Single-file durability; trivial backup/export.
- All V1 V2 tests stay in place.

Downsides:
- WAL plus single-writer contention bites under daemon-mediated
  high-frequency mutations (supervisor heartbeats, audit append, request
  log).
- Streaming projections require polling; no native CDC.
- Schema evolution under live readers is awkward.
- Cross-repository transactions span one file; large cross-repo workflows
  serialize.

#### Option B: libSQL / Turso (embedded)

libSQL is a SQLite fork with built-in replication, durable async writes,
and better concurrency. Embedded mode is a drop-in for SQLite at the SQL
level.

Benefits:
- SQL compatibility minimizes V1→V2 migration risk; existing migrations
  largely port.
- Embedded mode keeps the daemon a single process.
- Optional future replication enables hot standby without changing the
  daemon API.
- Mature Go driver.

Downsides:
- Newer than SQLite; production track record shorter.
- Still single-writer per database; doesn't eliminate Option A's
  contention story, just buffers it.
- Implies Turso brand commitment in docs.

#### Option C: Embedded PostgreSQL

`postgresql-embedded` or similar ship a Postgres binary that the daemon
launches as a child process or links statically.

Benefits:
- Real MVCC; concurrent readers and writers.
- Streaming replication, logical decoding, listen/notify all available
  for daemon-mediated events.
- SQL features (triggers, foreign keys, partial indexes, generated
  columns) we already use in V1 plus more.
- Mature Go driver (`pgx`).

Downsides:
- Heavyweight on disk; daemon installer ships a Postgres binary or
  depends on a system Postgres.
- Process supervision now nests: daemon → embedded Postgres → its workers.
- Backup/export story is no longer "copy one file."

#### Option D: RocksDB / BoltDB / Pebble (KV)

Key-value embedded stores. BoltDB is Go-native, single-file, single-writer.
RocksDB/Pebble are LSM-based, multi-writer-friendly, higher throughput.

Benefits:
- Native Go ergonomics (BoltDB and Pebble are Go libraries).
- High write throughput suits supervisor heartbeats and audit append.
- No SQL parser; schema is whatever the daemon decides.
- Trivial to embed and version.

Downsides:
- We lose SQL. Ad-hoc queries, joins, and views require code instead of
  SQL.
- Migration tooling does not exist; we author it.
- Audit hash chain and segment manifests require careful design in a KV
  world.
- Dashboard queries get more code-heavy.

#### Option E: Event-sourced append log + indexed projections

Daemon writes an append-only event log (e.g. a single file plus an index)
and maintains derived projections (KV index, SQL view, materialized
snapshot) for query.

Benefits:
- Conceptually aligns with daemon-as-deterministic-control-plane (RFC
  0028 §1) and audit chain (RFC 0028 V1).
- Easy to ship multiple projection shapes simultaneously (SQL view for
  dashboards, KV index for hot reads).
- Streaming consumers (web UI, MCP, recovery) attach to the log directly.
- Crash recovery is replay-from-log.

Downsides:
- We design the storage layer ourselves; failure modes are ours to find.
- Projection consistency under restart is non-trivial.
- Schema migration becomes "rebuild projection."
- Largest rewrite cost; furthest from V1.

### 2. Recommendation

Adopt **Option C (embedded PostgreSQL)** for daemon V2, with the following
guardrails:

- Daemon installer ships a known-good Postgres binary inside its
  distribution, or detects and refuses to start against a system Postgres
  whose major version is unsupported.
- Postgres lives inside `${XDG_STATE_HOME}/striatum/daemon/postgres/`
  (Linux) or platform equivalents; ownership is the daemon-installed OS
  user only.
- Daemon owns Postgres lifecycle: start/stop/reload, log rotation, vacuum,
  and version check. Operators never run `psql` against the daemon DB
  unless they pass an explicit `--unsafe-direct` flag the daemon refuses
  by default.
- Schema is migrated via daemon-owned versioned migrations; the V1 SQLite
  registry has an explicit "export to PG" path during the V1→V2 cutover
  (see §4).

Rationale:

- MVCC eliminates the single-writer pain that would dominate Options A
  and B as daemon-mediated mutation traffic grows.
- `LISTEN`/`NOTIFY` and logical decoding are the right primitives for
  daemon→client event streaming (web UI live updates, recovery resume,
  MCP subscribe).
- SQL keeps doctor/dashboard/why introspection ergonomic; we are not
  prepared to author the query tooling Option D/E demand.
- Go ecosystem (`pgx`, `pgxpool`) is mature.
- The "ship the Postgres binary" tax is real but tractable; the existing
  Python distribution already vendors `striatumd`, claude/gemini/codex
  wrappers, plugin templates, and migration logic.

The five options are documented above so reviewers can challenge the
recommendation in the dogfood-033 design phase.

### 3. Schema and migration model

- Schema migrations live in a daemon-owned versioned migration directory.
  The migration runner is invoked by the daemon at startup and refuses to
  start if the on-disk schema is newer than the daemon binary (mirrors
  D006/D007 / exit code 9 for repo-local SQLite).
- Each migration is a forward-only SQL script with a documented
  rollback-by-export path: down migrations are not supported, but every
  schema change must specify how to export V_N data so an operator can
  restore from a V_N backup against a V_{N+1} daemon.
- The substrate version is exposed via `daemon doctor` and recorded in
  every audit row so an audit log read by a future daemon can match
  schema version to row interpretation.

### 4. V1 registry → V2 daemon DB cutover

The V1 registry SQLite is small (repositories, clients, capabilities,
audit, scheduler cursors, daemon metadata). The cutover is:

1. `striatum daemon migrate --from sqlite --to pg --dry-run` enumerates V1
   registry rows and reports what would be exported.
2. `striatum daemon migrate --from sqlite --to pg` writes the V2 schema,
   imports rows, replays the audit chain into the V2 audit tables,
   verifies hash continuity end-to-end, and writes a checkpoint marker.
3. If the marker is present, V1 SQLite registry reads are refused;
   operators are pointed at the V2 DB.
4. A `--keep-sqlite-readonly` flag retains the V1 file as a tombstone for
   audit cross-check but blocks all V1 writes.

Repo-local `.striatum/state.sqlite3` is untouched.

### 5. Audit chain in the new substrate

V1 audit invariants (hash-chained rows, append-only triggers, segment
manifest verification in `daemon doctor`) are preserved with Postgres
equivalents:

- Hash chain: row trigger computes `row_hash` from `previous_hash || row
  payload`, no row updates or deletes permitted via daemon API.
- Append-only enforcement: Postgres roles. The daemon connects as a
  read-write role that has INSERT but not UPDATE/DELETE on audit tables.
  Migration / vacuum run as a separate role.
- Segment manifests: closed segments record range, count, anchor hash;
  manifest rows are also append-only.
- `daemon doctor` runs the V1 verification logic against the new schema.

### 6. Concurrency primitives

The daemon may not assume single-writer semantics anymore:

- Supervisor heartbeats and recovery scheduler ticks use SELECT ... FOR
  UPDATE SKIP LOCKED for queue claim semantics where appropriate.
- Audit append uses a serializable transaction with `previous_hash` read
  inside the same transaction to preserve chain integrity.
- Long-running daemon operations release transactions promptly; no
  multi-second locks while the daemon is doing anything else.

### 7. Test infrastructure

- Tests spin a per-test Postgres data directory using
  `pg_ctl`-equivalent helpers; teardown deletes the directory.
- A `STRIATUM_DAEMON_DB_URL` env override lets CI run against a shared
  Postgres if desired, with explicit isolation per test.
- Integration tests assert: schema migration applies forward, audit
  chain verification passes after migration, V1 SQLite import is
  byte-equivalent in hash anchors.

### 8. Packaging and distribution

- The Python daemon distribution adds a `postgres/` subdir with the
  bundled binary plus a small launcher script. Total disk footprint
  increases by 30–50 MiB depending on platform.
- The future Go daemon (D084) embeds Postgres the same way or, more
  likely, drops to a static-linked alternative (DuckDB? libSQL after
  more maturity? to be decided by a follow-up RFC if the bundled-binary
  story becomes painful).
- Operators may opt into a system Postgres with
  `--external-postgres-url`; the daemon checks compatibility and refuses
  to start if the version is wrong.

### 9. Provenance and trust implications

- Daemon owns Postgres lifecycle, so daemon process compromise implies DB
  compromise. Same blast radius as V1 (registry SQLite is also
  daemon-owned), but the daemon is now a longer-running, larger target.
- Audit append-only is enforced by Postgres roles, which is stronger than
  V1's SQL triggers (operator with raw write access to the SQLite file
  can still mutate). It is not stronger than an operator who can read
  the daemon DB role credentials, which lives under the same OS
  permissions as the daemon socket.
- Sealed-mode authority (RFC 0031) does not gain new powers from this
  substrate. Substrate provenance does not equal source provenance.

## Compatibility and Migration

- V1 SQLite registry remains readable until the operator runs `daemon
  migrate`. After migration, V1 reads are refused.
- Repo-local `.striatum/state.sqlite3` is unaffected; existing workflows
  continue to mutate it directly while RFC 0030 routes mutations through
  the daemon RPC. V1 dogfood workflows do not need re-running.
- Direct CLI mode (`--no-daemon`) continues to use repo-local SQLite only
  and is unaffected by this RFC.

## Downsides and risks

- Bundling Postgres adds 30–50 MiB to daemon distribution; users on
  metered installs notice.
- Daemon installer complexity rises: install, upgrade, uninstall must
  manage a Postgres lifecycle.
- Operators who break the bundled Postgres data directory have a worse
  recovery story than "delete `.striatum/state.sqlite3` and re-run
  `daemon migrate`." We need to document recovery explicitly.
- The schema-version-in-every-audit-row promise constrains future audit
  redesigns. We accept that cost.
- The Go core port (D084) inherits the embedded-Postgres choice; if Go
  daemon designers prefer something else, this RFC may need a partner
  RFC re-evaluating substrate at port time. We document that explicitly
  rather than pretend it cannot happen.

## Benefits

- MVCC eliminates the dominant V1 contention path before it bites.
- `LISTEN`/`NOTIFY` and logical decoding are the right primitives for
  daemon→client event streaming.
- Real append-only enforcement via DB roles.
- SQL keeps dashboard and doctor query ergonomics.
- Mature Go driver story aligns with D084.

## Acceptance Criteria

- A daemon binary can start against an embedded Postgres data directory
  it created itself and successfully apply V2 schema migrations.
- `striatum daemon migrate --from sqlite --to pg` imports a V1 daemon
  registry SQLite into the V2 Postgres schema with a byte-equivalent
  audit chain (hash anchors match end-to-end).
- After migration the daemon refuses V1 SQLite registry reads with a
  documented error.
- Per-test Postgres harness teardown leaves no zombie processes and no
  leftover data directories after a full test run.
- `daemon doctor` reports substrate version, schema version, audit chain
  status, and segment manifest verification.
- Supervisor heartbeat, audit append, and capability check write paths
  each have at least one concurrency test that exercises overlapping
  transactions without deadlock under serializable isolation.
- Documentation in `docs/SPEC.md`, `docs/MCP.md`, and
  `docs/UBIQUITOUS_LANGUAGE.md` is updated to name the substrate and
  describe operator UX for `daemon migrate`.

## Open Questions

- Should the daemon bundle Postgres or always require a system Postgres?
  Recommendation is bundle, but reviewers should challenge.
- What is the supported Postgres major-version range? Pick one major
  version at V2 release; bump with each major daemon release.
- Should `--external-postgres-url` be in V2 scope, or deferred? Adding
  it complicates the daemon installer; recommendation defers it to a
  follow-up RFC.
- Does the embedded-Postgres choice survive the Python→Go port (D084),
  or do we re-evaluate substrate at port time?
- If the operator wipes the daemon DB, what is the recovery story for
  the audit chain? Document that audit cannot be reconstructed from
  repo-local state and that this is the expected outcome.

## Domain Modeling

Terms to add to `docs/UBIQUITOUS_LANGUAGE.md` after acceptance:

- **Daemon DB** — the daemon-owned Postgres instance that holds registry,
  audit, capability, scheduler, and RPC-session state for V2. Distinct
  from `.striatum/state.sqlite3`, which is repo-local run state.
- **Daemon DB migration** — the daemon-owned forward-only versioned
  migration set applied at startup; refuses to run if the on-disk schema
  is newer than the daemon binary.
- **Substrate version** — the schema-version integer recorded in every
  audit row and surfaced by `daemon doctor`. Distinct from daemon binary
  version and from repo-local state DB version.
- **External Postgres** — operator-supplied Postgres URL accepted via
  `--external-postgres-url`. Not in V2 scope; documented as a future
  follow-up.
