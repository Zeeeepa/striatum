---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/033/design/codex/DESIGN.md", "docs/dogfood/033/design/claude_code/DESIGN.md", "docs/dogfood/033/design/gemini/DESIGN.md"]
---

# RFC 0033 Substrate Implementation Plan

author: designer-codex-gpt-5.5-001

Status: design synthesis
Date: 2026-05-11
Target: RFC 0033 storage substrate rewrite for daemon V2

## Accepted Implementation Scope

| RFC 0033 acceptance criterion | Concrete plan | Owner |
|---|---|---|
| Daemon connects to system Postgres, applies V2 migrations, and reports schema version via `daemon doctor`. | Add `src/striatum/daemon_pg/connection.py`, `config.py`, and `migrations.py`. Resolve URL from `--postgres-url`, `STRIATUM_DAEMON_DB_URL`, then `~/.config/striatum/daemon.toml`. Startup opens a pool, applies pending migrations, and exposes `substrate_version` in the doctor envelope. | `src/striatum/daemon.py`, `src/striatum/daemon_pg/*`, `tests/test_daemon_pg.py` |
| `daemon doctor` refuses unsupported PG versions, missing privileges, or newer schema. | Add a Postgres doctor section with redacted URL, server version, schema version, role grant checks, extension checks, audit status, and connection-pool status. Schema-newer-than-binary exits with code 9. Unsupported or underprivileged DBs refuse daemon start before mutation. | `src/striatum/daemon.py`, `src/striatum/cli/dispatch.py`, `tests/test_daemon_pg_doctor.py` |
| `striatum daemon migrate --from sqlite --to pg` imports V1 registry with byte-equivalent audit anchors. | Implement `src/striatum/daemon_pg/cutover.py`. Dry run validates source registry and reports row counts/final hash. Apply imports registry tables, replays V1 audit rows with `hash_format_version = 1`, verifies final hash and segment anchors, then writes a V1 cutover marker. | `src/striatum/daemon_pg/cutover.py`, `src/striatum/daemon.py`, `tests/test_daemon_pg_cutover.py` |
| After migration the daemon refuses V1 SQLite registry reads. | Teach V1 registry open path to detect `pg_cutover_completed_at` / cutover marker and fail with a documented error pointing to Postgres configuration. `--keep-sqlite-readonly` leaves a tombstone for audit cross-check but blocks V1 writes. | `src/striatum/daemon.py`, `tests/test_daemon_pg_cutover.py` |
| Per-test Postgres teardown leaves no zombie connections or leftover schemas. | Add test utilities that use `initdb`/`pg_ctl` when available and a `STRIATUM_DAEMON_DB_URL` override for CI service Postgres. Prefer database-per-test for migration/cutover; schema-per-test for concurrency. Teardown closes pools, terminates remaining backends for the test DB, stops per-test clusters, and removes temp dirs. | `tests/pg_harness.py`, `tests/test_daemon_pg.py` |
| `daemon doctor` reports substrate version, schema version, audit chain status, and segment manifests. | Port V1 audit verifier into `src/striatum/daemon_pg/audit.py` with hash-format dispatch for imported V1 rows and native V2 rows. Doctor reports `verified`, `degraded_retention_anchor_only`, or `broken`. | `src/striatum/daemon_pg/audit.py`, `tests/test_daemon_pg_audit.py` |
| Supervisor heartbeat, audit append, and capability checks have concurrency tests. | Add overlapping-transaction tests: audit append under `serializable`, scheduler cursor claims with `FOR UPDATE SKIP LOCKED`, and capability revocation racing authorization. Supervisor heartbeat table itself is RFC 0031-owned; this dogfood tests scheduler heartbeat/cursor paths and leaves daemon-owned supervisor rows deferred. | `src/striatum/daemon_pg/store.py`, `tests/test_daemon_pg_concurrency.py` |
| Docs updated for substrate, env var, migration UX, and system-PG requirement. | Update the named docs only after implementation: SPEC, MCP, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, HOW_TO_HUMAN, and RFC 0033 status block. Do not update docs to claim RFC 0030/0031/0032 behavior before those RFCs land. | docs listed in the prompt, plus `docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md` |

## Deferred Scope

Bundled, embedded-binary, and Dockerized Postgres distribution is deferred because RFC 0033 chooses system Postgres for V2. It lands in a packaging follow-up RFC.

Python to Go substrate port is deferred because D084 asks for Go-compatible protocol and schema choices, not a Go implementation in this dogfood. It lands with the future Go-core RFC.

Repo-local `.striatum/state.sqlite3` replacement is deferred because D006/D007 still make it authoritative for run state. RFC 0033 migrates daemon-owned registry state only.

Signing-key migration and apply-receipt custody are deferred because RFC 0031 owns sealed-apply authority. RFC 0033 only provides audit/request persistence for future apply calls.

Cross-repo coordinator state, `cross_repo_runs`, and MCP mutation capability expansion are deferred because RFC 0032 owns cross-repo workflow semantics. This substrate may leave extension points, but it must not ship future-only tables as accepted behavior.

Daemon-owned `process_supervisors` is deferred to RFC 0031. This dogfood may test scheduler/liveness-style update patterns, but it should not migrate supervisor ownership.

## Schema Decision

Use one Postgres schema named `striatumd`. Keep table states as text with `CHECK` constraints rather than Postgres enums so future migrations can widen values without enum DDL. Use `timestamptz`, `jsonb`, lowercase hex text for SHA-256 values, text public ids, and identity `bigint` only for sequence-sensitive audit and segment ids.

Baseline migration `0001_baseline.sql` creates:

| Table | Purpose |
|---|---|
| `schema_meta(key text primary key, value text not null)` | Stores `substrate_version`, `instance_id`, `bootstrapped_at`, `pg_major_at_bootstrap`. |
| `schema_migrations(version int primary key, label text, sha256 text, applied_at timestamptz, daemon_version text)` | Forward-only migration ledger. |
| `daemon_meta(key text primary key, value text, updated_at timestamptz)` | Daemon settings and cutover metadata not tied to one repo. |
| `repositories(repository_id text primary key, repo_identity, repo_root, state_db_path, display_name, registered_at, removed_at, last_seen_at, last_schema_version, state, settings_json jsonb)` | Port of V1 registry repository records. Partial unique indexes protect active `repo_identity` and `repo_root`; ids are never reused. |
| `clients(client_id text primary key, client_kind, display_name, token_id unique, token_hash, token_salt, created_at, expires_at, revoked_at, last_used_at)` | Client/token metadata without token secrets. |
| `client_capabilities(capability_id text primary key, client_id, repository_id nullable, capability, granted_at, expires_at, revoked_at, revoked_reason)` | Capability grants. V2 schema accepts `read`, `write`, `review`, `claim`, `apply`, `admin`, and `recovery`, but migration grants only V1 `read`/`admin` values. |
| `audit_segments(segment_id bigint identity primary key, opened_at, closed_at, first_audit_id, last_audit_id, first_hash, last_hash, previous_segment_id, previous_segment_last_hash, next_segment_first_previous_hash, retention_state, state)` | Segment manifests and retention anchors. |
| `audit_log(audit_id bigint identity primary key, ts, schema_version, hash_format_version, daemon_version, client_id, repository_id, method, decision, denial_reason, transport, request_id, exit_code, params_sha256, previous_hash, row_hash unique, segment_id)` | Metadata-only hash-chained daemon audit. No bodies, token secrets, transcripts, or tracebacks. |
| `audit_chain_head(singleton bool primary key, last_audit_id, last_hash, updated_at)` | Narrow lock target for serial audit append. |
| `scheduler_cursors(repository_id, run_id, cursor_kind, last_sweep_at, next_sweep_after, last_result_json jsonb, state, primary key(repository_id, run_id, cursor_kind))` | Multi-repo sweep cursors. |
| `rpc_request_log(request_id text primary key, client_id, repository_id, method, params_hash, response_hash, decision, audit_id, envelope_version, daemon_version, schema_version, started_at, finished_at, expires_at)` | RFC 0030-ready request observability with retention. This is not the integrity-bearing audit chain. |
| `client_sessions(client_session_id text primary key, client_id, transport, envelope_version, methods_etag, opened_at, last_seen_at, closed_at)` | Version-handshake/session bookkeeping for RFC 0030. |

Do not create `process_supervisors`, `cross_repo_runs`, `mutation_queue`, `supervisor_heartbeats`, or apply-receipt tables in the RFC 0033 implementation. Those are attractive because the substrate can hold them, but shipping them now would blur ownership with RFC 0031 and RFC 0032.

## Migration Plan

Add `src/striatum/daemon_pg/migrations.py` with a registry mirroring repo-local `src/striatum/migrations.py`: strict integer versions, committed SQL files, SHA-256 verification, no down migrations, and refusal when the database version is newer than the binary.

Startup order is:

1. Resolve and redact the Postgres URL.
2. Connect with a migration-capable role.
3. Take `pg_advisory_lock(hashtext('striatum_daemon_migrations'))`.
4. Read `schema_meta.substrate_version`; if absent, apply baseline.
5. Refuse with exit code 9 if DB version is newer than `LATEST_DAEMON_DB_VERSION`.
6. Apply pending migrations in order, one transaction per migration, writing `schema_migrations` and updating `schema_meta`.
7. Drop migration privileges by reconnecting or switching to the daemon app role before serving normal daemon work.

Every audit row and request-log row records the effective `schema_version` at write time. Rollback is export-and-restore only: each migration must document an export path for reconstructing the previous supported schema from a backup, not a `DOWN` script.

## Audit Chain Mapping

Audit append goes through `striatumd.append_audit(...)`. The daemon role receives `EXECUTE` on this function and `SELECT` on audit tables; it does not receive `UPDATE` or `DELETE` on `audit_log` or closed `audit_segments`.

The append function runs inside a short serializable transaction:

1. Lock `audit_chain_head` and the current open segment.
2. Build canonical metadata payload from `(ts, schema_version, hash_format_version, daemon_version, client_id, repository_id, method, decision, denial_reason, transport, request_id, exit_code, params_sha256, previous_hash, segment_id)`.
3. Compute `row_hash = sha256(previous_hash || canonical_payload)`, preferably with `pgcrypto.digest`. If `pgcrypto` is unavailable, the daemon may compute in Python only if a doctor warning labels DB-side recomputation unavailable.
4. Insert `audit_log`, update open segment first/last fields, update `audit_chain_head`, and return `audit_id` and `row_hash`.

Imported V1 rows use `hash_format_version = 1` and preserve V1 `row_hash` exactly. New V2 rows use `hash_format_version = 2` and include substrate schema fields. The first V2 row's `previous_hash` is the last imported V1 row hash.

`daemon doctor` recomputes retained row hashes by hash-format version, verifies row-to-row continuity, verifies segment first/last anchors, verifies cross-segment continuity, and reports retention-gapped segments as degraded rather than clean.

## V1 SQLite To V2 Postgres Cutover

Operator UX:

```text
striatum daemon migrate --from sqlite --to pg --dry-run
striatum daemon migrate --from sqlite --to pg
striatum daemon migrate --from sqlite --to pg --keep-sqlite-readonly
```

Dry run opens V1 SQLite read-only, connects to Postgres, checks source audit integrity, reports table counts, active/removed repositories, live/revoked tokens, segment count, source final hash, destination schema version, and whether the destination is empty or resumable.

Apply flow:

1. Refuse if a V1 daemon process is alive, a scheduler cursor is `in_progress`, the source registry schema is not current, destination schema is newer than the binary, or destination tables are non-empty without an explicit future resume flag.
2. Apply Postgres migrations.
3. Open V1 SQLite with `BEGIN IMMEDIATE`.
4. Import in dependency order: repositories, clients, capabilities, daemon metadata, scheduler cursors, audit segments, audit rows.
5. Import V1 audit rows through a migration-only function that preserves `previous_hash` and `row_hash` with `hash_format_version = 1`.
6. Verify final hash and every retained segment anchor against the V1 source.
7. Write V1 cutover metadata: `pg_cutover_completed_at`, `pg_cutover_db_fingerprint`, and `pg_cutover_final_hash`.
8. Refuse all future V1 registry reads with a documented error pointing to Postgres daemon mode. With `--keep-sqlite-readonly`, leave the SQLite file as an audit tombstone while blocking writes.

Repo-local `.striatum/state.sqlite3` is untouched throughout.

## Concurrency

Default daemon store operations run at `read committed`. Escalate to `serializable` only for audit append and cross-row invariants.

Use `SELECT ... FOR UPDATE SKIP LOCKED` for scheduler cursor claims so parallel sweep workers do not block each other. Cursor transactions must mark a row `in_progress`, commit, run the actual sweep outside the lock, then write final cursor state in a second short transaction.

Capability authorization reads indexed client/capability rows. Revocation locks the affected grant rows with `FOR UPDATE` so any authorization that starts after revocation sees the revoked state. Non-critical `last_used_at` updates run outside audit append transactions.

Deadlock avoidance order is: client/capability rows, repository rows, scheduler cursor rows, audit segment/head rows. Do not hold Postgres locks while doing repo-local SQLite writes except in future RFC 0032 reconciliation code.

## Daemon Doctor Onboarding Story

Minimum supported version: PostgreSQL 14 unless implementation discovers a concrete dependency requiring newer. The doctor prints the detected `server_version_num`, supported floor, database name, username, host/socket, and redacts passwords.

Doctor hints are advisory, not an installer:

| Platform | Hint |
|---|---|
| macOS/Homebrew | `brew install postgresql@16 && brew services start postgresql@16` |
| Debian/Ubuntu | `sudo apt install postgresql && sudo systemctl enable --now postgresql` |
| Arch | `sudo pacman -S postgresql`, then `sudo -iu postgres initdb -D /var/lib/postgres/data`, then `sudo systemctl enable --now postgresql` |
| FreeBSD/pkg | `sudo pkg install postgresql16-server postgresql16-client` and use the platform service command |
| WSL | Install inside the Linux distro; do not point Windows-native paths at Linux repo roots |

Operator setup docs should include:

```bash
createuser striatum_daemon
createdb --owner striatum_daemon striatumd
export STRIATUM_DAEMON_DB_URL='postgresql://striatum_daemon@localhost/striatumd?sslmode=disable'
striatum daemon doctor
striatum daemon migrate --from sqlite --to pg --dry-run
striatum daemon migrate --from sqlite --to pg --keep-sqlite-readonly
striatum daemon start
```

For password auth, docs may show `create role striatum_daemon login password '<local-password>'; create database striatumd owner striatum_daemon;`. The URL must be redacted in every diagnostic.

## Test Harness

Use `tests/pg_harness.py` with two modes:

1. Local cluster mode: locate `initdb` and `pg_ctl`, create a temp data dir, start Postgres on a random local socket or port, create roles/databases, set `STRIATUM_DAEMON_DB_URL`, close pools, stop the cluster, and delete the temp dir.
2. Shared CI mode: use an externally supplied `STRIATUM_DAEMON_DB_URL`, create schema/database-per-test with unique names, and drop with `cascade` at teardown.

Tests that require privilege enforcement must use separate migrator and app roles. Tests must not run append-only assertions as a superuser because that would mask the role guarantee.

The V1-to-V2 integration test builds a temp V1 daemon registry with repositories, clients, capabilities, scheduler cursors, audit rows, and segments; runs dry run; runs apply; verifies imported row counts and final hash; then asserts V1 registry reads refuse after the marker.

## Documentation Deltas

`docs/SPEC.md`: add daemon V2 Postgres substrate, system-PG requirement, daemon DB migration behavior, audit schema-version rows, and explicit non-change to repo-local SQLite.

`docs/MCP.md`: after RFC 0030 lands, update daemon MCP from resources-only direct registry access to daemon RPC/capability routing backed by Postgres audit. Until then, avoid claiming MCP mutation availability.

`docs/UBIQUITOUS_LANGUAGE.md`: add daemon DB, daemon DB migration, substrate version, system Postgres, bundled distribution, and keep them distinct from repo-local state DB.

`docs/CLI_REFERENCE.md`: add `daemon migrate --from sqlite --to pg`, `--postgres-url`, `STRIATUM_DAEMON_DB_URL`, exit code 9 for newer daemon DB schema, and doctor errors for unreachable/unsupported Postgres.

`docs/HOW_TO_HUMAN.md`: add first-time setup, role/database creation, doctor preflight, dry-run migration, cutover, and direct-mode compatibility note.

`docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md`: update status after implementation/review and record accepted PG 14+ floor or the final chosen floor.

## Test Matrix

| Case | Expected result |
|---|---|
| Malformed `STRIATUM_DAEMON_DB_URL` | Doctor reports parse failure without leaking secret material; daemon start refuses before mutation. |
| Unreachable host/socket | Doctor reports unreachable; daemon start refuses. |
| Unsupported PG version | Doctor reports detected version and supported floor; daemon start refuses. |
| App role can `UPDATE audit_log` | Doctor fails append-only privilege check. |
| App role missing `EXECUTE` on audit append function | Doctor fails privilege check. |
| DB schema version newer than binary | Exit code 9; no migration attempt. |
| Migration SQL SHA differs from committed registry | Migration refuses. |
| V1 audit chain broken before import | Dry run and apply refuse before Postgres writes. |
| Destination Postgres contains existing rows | Cutover refuses unless a future explicit resume mode exists. |
| Two daemons share one DB | Advisory startup lock / daemon instance lease allows one active daemon; second refuses with clear owner metadata. |
| Audit append serialization conflict | Bounded retry succeeds once or returns deterministic failure without duplicate request ids. |
| Scheduler cursor concurrent claims | `SKIP LOCKED` assigns each cursor once. |
| Capability revoked during request | Requests authorizing after revocation deny and audit `capability_missing` or `token_revoked`. |
| `--keep-sqlite-readonly` tombstone | SQLite remains inspectable but V1 daemon reads/writes refuse through Striatum. |
| Test teardown | No remaining test schemas, databases, clusters, or zombie backends. |

## Staging Plan

This dogfood should land the Python Postgres substrate scaffolding, baseline schema, migration runner, doctor preflight, V1 registry cutover, audit verifier, and tests for the RFC 0033 acceptance criteria.

This dogfood should not land daemon RPC routing defaults from RFC 0030, daemon-owned supervision or sealed apply from RFC 0031, cross-repo workflows or MCP mutation tools from RFC 0032, or bundled Postgres distribution. It may create request-log and client-session tables because RFC 0030 needs them and RFC 0033 explicitly says request-log persistence keys off the substrate choice; it must not claim the RPC server itself is implemented.

## Human-Decision Questions

1. What exact Postgres floor should V2 support? Recommendation: PG 14+ because it is old enough for operator availability and sufficient for JSONB, `SKIP LOCKED`, advisory locks, and role enforcement.
2. Should `pgcrypto` be mandatory? Recommendation: require it for DB-side audit recomputation unless a target platform makes that materially painful; otherwise Python-only hashing weakens the DB-side verification story.
3. Should the daemon require separate migrator and app roles in V2, or allow one owner role with degraded-trust warning? Recommendation: require separate roles for production start, allow owner-only only in tests/dev with an explicit warning.
4. If the daemon DB is wiped, should Striatum offer any reconstruction command? Recommendation: no. Repo-local state can preserve run facts, but daemon audit history cannot be reconstructed and docs should say so plainly.
5. How long is V1 SQLite registry tombstone support kept after cutover? Recommendation: one minor release with `--keep-sqlite-readonly`, then remove V1 registry read paths in a follow-up.
