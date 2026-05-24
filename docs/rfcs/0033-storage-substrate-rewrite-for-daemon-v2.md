# RFC 0033: Storage Substrate Rewrite for Daemon V2

Status: accepted (V2)
Date: 2026-05-11
Supersession note: D094 / RFC 0043 extends this substrate decision to
per-repository workflow state. Daemon-owned PostgreSQL is now the sole
live-state substrate; D113 retired writable SQLite import commands, so legacy
SQLite survives only as guarded migration-fixture material, tombstones, or test
fixtures.
Context:
[`RFC 0028`](0028-long-running-daemon-and-multi-repository-control-plane.md),
[`RFC 0030`](0030-daemon-rpc-server-and-version-skew-protocol.md) (proposed),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D082, D084, D086, D006, D007),
[`docs/SPEC.md`](../SPEC.md) § "State Store",
`src/striatum/daemon.py`,
`src/striatum/migrations.py`

RFC 0033 lands first in the daemon V2 follow-up sequence because RFC 0030's
wire protocol, schema migrations, and audit-chain format all key off the
substrate choice.

Implemented in dogfood-033 (V2 scaffold: system Postgres requirement,
`src/striatum/daemon_pg/` package, original `daemon migrate --from sqlite
--to pg` cutover, audit-chain mapping with byte-equivalent V1 anchors). The
operator-facing cutover spelling is now a retired compatibility refusal per
D113.

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

At the time RFC 0033 landed, repo-local `.striatum/retired-local-state` stayed and
the substrate rewrite was for daemon-owned global state only. D094/RFC 0043
later superseded that carve-out and moved per-repository workflow state into
daemon-owned PostgreSQL.

## Goals

- Pick a single concrete substrate for daemon V2 state.
- Define a schema-versioning model that survives a future Python-to-Go port
  of the daemon core (D084).
- Define the audit-chain format on the new substrate so that segment
  manifests, hash anchors, and retention semantics survive intact.
- Specify the then-current migration from the V1 daemon registry SQLite to the
  new substrate, including operator UX for in-place upgrade. This is
  historical: D113 later retired writable SQLite import commands from
  operator paths.
- Specify test infrastructure: how does the test suite spin the new
  substrate up and down deterministically?
- Preserve the then-current repo-local workflow model for the RFC 0033
  slice. This goal is historical; D094/RFC 0043 later replaced repo-local
  SQLite as production workflow authority.

## Non-Goals

- Replacing repo-local `.striatum/retired-local-state` in the RFC 0033 slice. That
  later happened explicitly through D094/RFC 0043.
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

#### Option C: System PostgreSQL

The daemon connects to an operator-installed PostgreSQL service through a
configured connection URL. The daemon owns its schema, migrations, roles,
and audit semantics, but it does not manage the PostgreSQL process
lifecycle.

Benefits:
- Real MVCC; concurrent readers and writers.
- Streaming replication, logical decoding, listen/notify all available
  for daemon-mediated events.
- SQL features (triggers, foreign keys, partial indexes, generated
  columns) we already use in V1 plus more.
- Mature Go driver (`pgx`).

Downsides:
- Requires operators to install or provide a system Postgres before daemon
  V2 can start.
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

Adopt **Option C (system PostgreSQL)** for daemon V2, with the following
guardrails:

- The daemon connects to an operator-installed system PostgreSQL via a
  configurable URL (`STRIATUM_DAEMON_DB_URL`,
  `~/.config/striatum/daemon.toml`, or a `--postgres-url` flag).
- `striatum daemon doctor` verifies that the URL is reachable, the
  Postgres major version is supported, and the daemon role has the
  expected privileges; refuses to start otherwise.
- The daemon does NOT manage the Postgres lifecycle. Operators run
  Postgres via the package manager / service manager they already use.
  This keeps the daemon a single process and avoids nesting Postgres
  supervision inside daemon supervision.
- Schema migrations are daemon-owned and forward-only (RFC 0033 §3). The
  original V1 SQLite registry "export to PG" path in §4 is retained as
  historical context; current command spellings refuse before opening SQLite.
- Bundled PostgreSQL is **deferred** to a follow-up RFC. The two
  plausible bundling paths are a Dockerized distribution (daemon +
  Postgres in one compose) and an embedded-binary distribution (daemon
  ships its own `pg_ctl`-managed Postgres). Both add packaging /
  distribution surface area that we can avoid in V2 by requiring system
  Postgres.

Rationale:

- MVCC eliminates the single-writer pain that would dominate Options A
  and B as daemon-mediated mutation traffic grows.
- `LISTEN`/`NOTIFY` and logical decoding are the right primitives for
  daemon→client event streaming (web UI live updates, recovery resume,
  MCP subscribe).
- SQL keeps doctor/dashboard/why introspection ergonomic; we are not
  prepared to author the query tooling Option D/E demand.
- Go ecosystem (`pgx`, `pgxpool`) is mature.
- Requiring system Postgres keeps the daemon's distribution simple: one
  Python wheel today, one Go binary tomorrow. Operators on workstations
  already have Postgres available via Homebrew / apt / pacman / WSL.
  Operators who don't can install it once.
- Bundled / Dockerized distribution is a real product question, but it
  is orthogonal to the substrate decision. Deferring it lets RFC 0030
  ship without the daemon installer growing a Postgres lifecycle
  manager.

The five options are documented above as the accepted decision record for
why daemon V2 uses system PostgreSQL instead of SQLite, libSQL, embedded
Postgres, a key-value store, or a custom event log.

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

### 4. V1 registry -> V2 daemon DB cutover

D113 later retired this operator-facing SQLite import surface. The command
shapes below document the original RFC 0033 cutover behavior; current Striatum
keeps the spellings parseable only to return a clear exit-code-12 refusal
before importing SQLite migration code.

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

Repo-local `.striatum/retired-local-state` is untouched.

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
- Integration tests assert: schema migration applies forward and audit
  chain verification passes after migration. Historical SQLite-import
  byte-equivalence coverage is fixture-only.

### 8. Packaging and distribution

- The Striatum distribution / Go daemon does **not** bundle Postgres.
  The package stays small (no `postgres/` subdir). Operators install
  Postgres via their platform package manager (Homebrew, apt, pacman,
  pkg, ...) or use a system service. Python may remain as CLI/web
  client code, not as a production daemon-core requirement.
- `daemon doctor` documents the minimum supported Postgres major
  version and refuses to start against an unsupported version.
- The Go production daemon inherits the same "system Postgres
  required" stance unless a follow-up RFC changes it.
- Bundled / Dockerized distribution is documented as a deferred
  follow-up RFC. Two paths are plausible:
  - **Dockerized**: ship a `docker compose` that brings up daemon +
    Postgres + (optionally) MCP server as one unit. Easiest for
    onboarding new users; adds Docker as a hard dependency.
  - **Embedded binary**: ship a `pg_ctl`-managed Postgres alongside
    the daemon binary, similar to how some desktop apps embed
    Postgres. Avoids Docker; grows the distribution footprint by
    30–50 MiB and pushes Postgres lifecycle into the daemon.

  Both are real product choices that benefit from their own RFC; V2
  ships with system Postgres only.

### 9. Provenance and trust implications

- The daemon owns the Postgres schema and runtime credentials, not the
  Postgres service lifecycle. Daemon process compromise therefore implies
  daemon DB compromise. Same blast radius as V1 registry SQLite, but the
  daemon is now a longer-running, larger target.
- Audit append-only is enforced by Postgres roles, which is stronger than
  V1's SQL triggers (operator with raw write access to the SQLite file
  can still mutate). It is not stronger than an operator who can read
  the daemon DB role credentials, which lives under the same OS
  permissions as the daemon socket.
- Sealed-mode authority (RFC 0031) does not gain new powers from this
  substrate. Substrate provenance does not equal source provenance.

## Compatibility and Migration

- The V1 SQLite registry import path is retired for operators. Current
  `daemon migrate` spellings are compatibility refusals; any SQLite import
  exercise is fixture-only.
- Repo-local `.striatum/retired-local-state` was unaffected by RFC 0033 itself.
  Current production workflow state now lives in daemon-owned PostgreSQL per
  D094/RFC 0043; V1 SQLite survives only as migration source, tombstone, or
  fixture material.
- Direct CLI mode (`--no-daemon`) was unaffected by RFC 0033. It is now
  retired for production use.

## Downsides and risks

- System Postgres is a real first-time-setup ask. Operators new to
  Striatum need to install Postgres (or use an existing service) before
  `daemon start` works. The deferred bundled / Dockerized distribution
  RFC addresses onboarding friction; V2 documents the system-PG
  requirement clearly and provides a `daemon doctor` story that points
  at platform-specific install instructions.
- Operators with multiple unrelated Postgres deployments (one for a
  hobby app, one for Striatum) must keep them isolated. The daemon role
  is named distinctly; the database name is configurable. This is
  documented but not enforced by code.
- The schema-version-in-every-audit-row promise constrains future audit
  redesigns. We accept that cost.
- The Go core port (D084, later D107/RFC 0068) inherits the system-Postgres
  choice. D111 keeps that substrate as the production daemon boundary rather
  than re-evaluating storage at port time.
- Daemon installer complexity stays low because the daemon does not
  manage Postgres. The cost shows up in the operator's machine setup
  instead.

## Benefits

- MVCC eliminates the dominant V1 contention path before it bites.
- `LISTEN`/`NOTIFY` and logical decoding are the right primitives for
  daemon→client event streaming.
- Real append-only enforcement via DB roles.
- SQL keeps dashboard and doctor query ergonomics.
- Mature Go driver story aligns with D084.

## Acceptance Criteria

- A daemon binary can connect to a system Postgres via a configured URL,
  apply V2 schema migrations forward, and report the resulting schema
  version via `daemon doctor`.
- `daemon doctor` refuses to start if the Postgres major version is
  unsupported, the role is missing required privileges, or the daemon
  binary is older than the on-disk schema.
- The original `striatum daemon migrate --from sqlite --to pg` cutover behavior
  is retained only as historical RFC context; current Striatum returns a
  retired-command refusal before opening SQLite migration code.
- Current daemon registry reads are PostgreSQL-only; V1 SQLite registry reads
  are not a production path.
- Per-test Postgres harness teardown leaves no zombie connections and
  no leftover schemas after a full test run.
- `daemon doctor` reports substrate version, schema version, audit chain
  status, and segment manifest verification.
- Supervisor heartbeat, audit append, and capability check write paths
  each have at least one concurrency test that exercises overlapping
  transactions without deadlock under serializable isolation.
- Documentation in `docs/SPEC.md`, `docs/MCP.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CLI_REFERENCE.md`, and
  `docs/HOW_TO_HUMAN.md` is updated to name the substrate, the system-PG
  requirement, the `STRIATUM_DAEMON_DB_URL` env var, and the retired
  `daemon migrate` compatibility refusal.

## Open Questions

- ~~Should the daemon bundle Postgres or always require a system
  Postgres?~~ **Resolved**: V2 requires system Postgres. Bundled /
  Dockerized distribution is deferred to a follow-up RFC.
- What is the supported Postgres major-version range? Pick one minimum
  major version at V2 release (recommendation: PG 14+); bump with each
  major daemon release. Reviewers should challenge the floor.
- What does the daemon-onboarding UX look like for an operator who has
  never installed Postgres? Recommendation: `daemon doctor` emits
  platform-specific install hints (Homebrew, apt, pacman, pkg). The
  bundled / Dockerized follow-up RFC will lower this bar further.
- ~~Does the system-Postgres choice survive the Python→Go port (D084),
  or do we re-evaluate substrate at port time?~~ **Resolved**:
  D107/RFC 0068 and D111 keep system PostgreSQL as the Go daemon's
  production live-state substrate.
- If the operator wipes the daemon DB, what is the recovery story for
  the audit chain? Document that audit cannot be reconstructed from
  repo-local state and that this is the expected outcome.

## Domain Modeling

Terms to add to `docs/UBIQUITOUS_LANGUAGE.md` after acceptance:

- **Daemon DB** — the daemon-owned Postgres instance that holds registry,
  audit, capability, scheduler, RPC-session state, and current per-repository
  workflow state. `.striatum/retired-local-state` is historical migration/fixture
  material, not repo-local run truth.
- **Daemon DB migration** — the daemon-owned forward-only versioned
  migration set applied at startup; refuses to run if the on-disk schema
  is newer than the daemon binary.
- **Substrate version** — the schema-version integer recorded in every
  audit row and surfaced by `daemon doctor`. Distinct from daemon binary
  version and from repo-local state DB version.
- **System Postgres** — the operator-installed PostgreSQL instance the
  daemon connects to via `STRIATUM_DAEMON_DB_URL` or `--postgres-url`.
  V2 requires it; daemon does not manage Postgres lifecycle.
- **Bundled distribution** — a deferred follow-up packaging shape that
  ships the daemon together with Postgres (Dockerized or
  embedded-binary). Out of V2 scope.
