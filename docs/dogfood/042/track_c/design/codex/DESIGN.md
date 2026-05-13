# Track C Design: Repo-Local State To Daemon Postgres

author: designer-codex-gpt-5.5-003
date: 2026-05-13
status: design input

## Summary

RFC 0042 V1 should make one explicit product-boundary change: the
authoritative live workflow state currently stored in each target
repository at `.striatum/state.sqlite3` moves into the daemon DB, keyed by
`repo_id` / `repository_id`. Repository files remain durable provenance,
not the live message bus. `.striatum/` remains in the target repository, but
only as operational scratch for FIFOs, pidfiles, supervisor stdout logs,
temporary adapter files, and read-only migration tombstones.

This is not a small storage refactor. It supersedes D006, D007, and D028 for
workflow state location and mutation routing:

- D006's SQLite-backed local message bus becomes a daemon-Postgres-backed
  message bus.
- D007's repo-local `.striatum/state.sqlite3` storage location becomes a
  migration source and optional read-only audit tombstone.
- D028's "local-only writes through CLI" becomes "local-only writes through
  daemon-mediated CLI/RPC"; direct DB mutation remains forbidden.

D083 stays intact. The trust boundary is still one OS user on one machine.
"Multi-tenancy" means schema-level per-repo isolation inside the daemon DB:
rows are keyed by `repo_id`, capability checks are repository-scoped, and
cross-repo workflows name explicit participant repositories. It does not mean
hosted mode, cross-machine semantics, per-user auth, remote storage, or a
shared-workstation access model.

## Design Posture

The current accepted docs say daemon Postgres owns registry, audit,
capability, scheduler, RPC-session, daemon supervisor metadata, apply
receipts, and cross-repo coordination state only. Repo-local SQLite remains
the workflow truth in RFC 0030, RFC 0031, and RFC 0033. RFC 0042 should
therefore name itself as the RFC that intentionally revisits that line.

The resulting architecture is:

- The daemon DB is authoritative for `workflow_snapshots`, `runs`,
  `sessions`, `jobs`, `job_dependencies`, `queue_messages`, `leases`,
  `work_packets`, `artifacts`, `verdicts`, `blockers`,
  `command_requests`, `process_executions`, `events`, `job_worktrees`, and
  `process_supervisor_pointers`.
- `striatumd.repositories` remains the foreign-key anchor for registered
  target repositories.
- Every workflow-state table either has `repository_id` as a leading key or
  references a parent key that includes `repository_id`.
- Direct repo-local CLI mode stops being a compatibility path for migrated
  repositories. If the daemon is unavailable, mutating and live-state read
  verbs refuse with a named remediation instead of silently falling back to
  `.striatum/state.sqlite3`.
- The future Go daemon from RFC 0039 becomes the gateway for all repo-local
  operations from day one of the rewrite, not only daemon-owned state.

## Schema Shape

RFC 0042 should add a daemon migration series for workflow-state tables under
the existing `striatumd` schema. The migration should not create one schema
per repository. Per-repo isolation is row-level within one daemon-owned schema
because the operator boundary is single-user and the product needs cheap
cross-repo queries.

Recommended table shapes:

- `striatumd.workflow_snapshots(repository_id, workflow_snapshot_id, ...)`
  with primary key `(repository_id, workflow_snapshot_id)`.
- `striatumd.runs(repository_id, run_id, ...)` with primary key
  `(repository_id, run_id)` and `repository_id` referencing
  `striatumd.repositories(repository_id)`.
- Child tables use composite foreign keys. For example,
  `jobs(repository_id, job_id, run_id, ...)` references
  `runs(repository_id, run_id)`, and `leases(repository_id, lease_id, run_id,
  owner_session_id, ...)` references sessions and runs through the same
  repository key.
- Active uniqueness constraints become repository-scoped. Examples:
  `UNIQUE(repository_id, run_id, workflow_job_id, attempt)`,
  `UNIQUE(repository_id, run_id, slug)`, `UNIQUE(repository_id, message_id,
  lease_id)`, and active lease uniqueness on `(repository_id, resource_type,
  resource_id)` where `state = 'active'`.
- Append-only tables preserve append-only semantics. `events` and
  `artifacts` keep no-update/no-delete enforcement, now under Postgres roles
  and triggers. `events.event_id` can be identity per daemon DB, but all
  indexes and API queries must include `repository_id`.
- `events` should carry `repository_id` directly even when `run_id` is null,
  so daemon-visible repository events are not forced through a run row.
- `process_supervisor_pointers` should remain the compatible repo-scoped
  pointer table, but it now lives beside the daemon-owned supervisor metadata
  instead of in SQLite. The pointer still references `(repository_id, run_id,
  session_id)`.
- Cross-repo tables from RFC 0032 stay daemon-global coordination tables:
  `cross_repo_runs`, `cross_repo_run_repositories`, and
  `cross_repo_cycle_counters`. Participant local run references become
  `(repository_id, local_run_id)` pairs with foreign keys into
  `runs(repository_id, run_id)`.

The design should normalize naming. Existing daemon tables use
`repository_id`; workflow JSON and prompts sometimes say `repo_id`. The RFC
can define `repo_id` as the user-facing alias, but SQL should consistently
use `repository_id` because that is the existing daemon registry column.

## Migration Verb

Add:

```bash
striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--keep-sqlite-readonly] [--dry-run]
```

Required behavior:

- Resolve `<path>` to an active `striatumd.repositories` row. If the target
  repository is not registered, refuse with a remediation that names
  `striatum repo add --init <path>` or the current registration verb.
- Inspect `.striatum/state.sqlite3`, its `PRAGMA user_version`, and the
  current schema. Refuse unsupported future SQLite versions with the same
  spirit as exit code 9.
- `--dry-run` reports table counts, source schema version, destination
  workflow-state schema version, source final event/artifact counts, and
  whether the destination already contains rows for that `repository_id`.
  It writes nothing.
- The real migration runs in a bounded transaction plan: import into
  staging tables, validate foreign-key and count parity, then promote into
  the live workflow-state tables for that `repository_id`.
- The destination must be empty for that `repository_id` unless a future
  recovery RFC defines resumable import. V1 should refuse partial imports
  rather than merge.
- Preserve primary identifiers. Existing `run_id`, `job_id`, `session_id`,
  `message_id`, `lease_id`, `packet_id`, `artifact_id`, `verdict_id`,
  `blocker_id`, `request_id`, `process_id`, and `worktree_id` values should
  not be rewritten; `repository_id` is added around them.
- Record a daemon audit checkpoint for the migration with metadata-only
  hashes: source database fingerprint, source schema version, table counts,
  imported row counts, destination schema version, and whether the SQLite
  tombstone was kept. Do not log artifact contents, prompts, verdict
  rationale, blocker descriptions, or packet bodies.
- With `--keep-sqlite-readonly`, leave `.striatum/state.sqlite3` on disk as a
  read-only audit tombstone and record a source marker in daemon metadata.
  Without it, V1 may either leave the file renamed to a tombstone path or
  refuse deletion; the important point is that it is not live state.

The first daemon start after RFC 0042 may offer guided migration, but it
should not silently import and continue. The operator should see an explicit
state such as `repo_local_state_unmigrated` with the exact migration command.
Silent import hides the most important product-boundary change.

## CLI And Daemon Behavior

After migration, all live-state verbs route through the daemon RPC method
registry. This includes `run prepare`, `run start`, `register-session`,
`claim-next`, `ack`, `heartbeat`, `publish-artifact`, `verdict`,
`submit-review`, `complete`, `block`, recovery verbs, dashboard/status/why,
evidence export, run summary, worktree helpers, and supervisor pointer
operations.

Behavior when no daemon is running:

- For migrated repositories, live-state reads and mutations refuse with a
  stable error such as `daemon_required` and remediation:
  `start the daemon with STRIATUM_DAEMON_DB_URL configured, then retry`.
- For unmigrated repositories, verbs that would mutate workflow state refuse
  with `repo_local_state_unmigrated` and print the
  `migrate-repo-local --from sqlite --to pg --repo <path>` command.
- `--no-daemon` becomes read-only/export-only for migrated repositories, if
  it remains at all. It must never mutate a tombstone SQLite database.
- `striatum init` may still create `.striatum/` and `.striatum/scratch/`, but
  it must not initialize authoritative workflow state there after the RFC
  lands.

The RPC invariants from RFC 0030 should stay unchanged: envelope-v1,
`daemon.hello` / `daemon.welcome`, `daemon.describe`, stable
`methods_etag`, route-bound capability checks, request log, audit row per
allowed or denied request, no token secrets in logs, and no silent fallback on
version mismatch.

## RFC 0039 Scope Revision

RFC 0039 currently scopes the Go daemon as an alternative implementation of
the daemon-owned state surfaces. RFC 0042 should revise that implementation
plan before the Go rewrite lands:

- The Go daemon must implement workflow-state RPC routes from its first
  useful end-to-end milestone, not after a Python-only transition.
- The Go daemon's DB package must include the workflow-state tables and
  migrations, not just registry/audit/capability/supervisor/apply tables.
- The Python CLI remains the operator-facing client, but it never opens
  `.striatum/state.sqlite3` for live workflow state once a repository is
  migrated.
- The RFC 0035 harness should parameterize daemon core and run the migrated
  workflow-state suite against both Python and Go cores during coexistence.

This revision avoids a split-brain period where the Go daemon is "daemon
first" for registry state while the Python CLI still owns repo-local workflow
state.

## Audit And Provenance

There are two logs with different jobs:

- `daemon audit` records metadata-only authorization and mutation decisions
  through the RFC 0030/RFC 0033 hash chain.
- `events` records workflow-domain events, now keyed by `repository_id`.

RFC 0042 should preserve both. Migration should append daemon audit rows for
the import decision and workflow events for repository-level migration
milestones, without copying transcript-like content into audit. Evidence
export remains the redacted, durable way to publish selected run evidence
from live state. Repository artifacts remain durable provenance and do not
become the live bus.

The audit chain cannot be "the same chain" as the old repo-local SQLite
events table because `events` is not currently hash-chained. The acceptance
bar should instead be:

- all imported `events` rows survive with the same order within each run;
- all imported `artifacts` rows preserve content hashes and repo-relative
  paths;
- the daemon audit chain records a migration checkpoint with source and
  destination hashes/counts; and
- `daemon doctor` verifies the checkpoint against the destination rows.

## Acceptance Criteria

1. Daemon DB migrations create repo-keyed workflow-state tables for every
   current repo-local live-state table, including migration-added tables
   (`job_worktrees` and `process_supervisor_pointers`).
2. Every workflow-state row is isolated by `repository_id`, either directly
   or through a composite foreign key whose parent includes `repository_id`.
3. `striatumd.repositories(repository_id)` is the foreign-key anchor for all
   migrated workflow state.
4. Active uniqueness, claimability, queue, lease, event, artifact, verdict,
   blocker, process, worktree, and supervisor-pointer indexes are
   repository-scoped.
5. `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>
   --dry-run` reports source counts and destination readiness without writes.
6. The non-dry-run migration imports an existing `.striatum/state.sqlite3`
   into daemon Postgres with row-count parity, preserved identifiers,
   preserved artifact hashes, preserved event ordering per run, and
   validated foreign keys.
7. `--keep-sqlite-readonly` leaves the source SQLite file as a read-only
   audit tombstone and prevents later direct-mode mutation against it.
8. Migration appends metadata-only daemon audit rows and a verifiable
   checkpoint. `daemon doctor` verifies that checkpoint.
9. After migration, all live-state CLI verbs route through daemon RPC and
   refuse with `daemon_required` when no compatible daemon is reachable.
10. There is no silent fallback from daemon RPC to direct repo-local SQLite
    after a repository is migrated.
11. `striatum init` and `.striatum/` setup are revised so `.striatum/` is
    operational scratch only, not authoritative state storage.
12. The method registry includes workflow-state routes with repo-scoped
    capability requirements. Tokens scoped to repo A cannot read or mutate
    repo B workflow state.
13. Cross-repo workflows keep explicit participant mappings; local run
    references in cross-repo tables point to `(repository_id, run_id)`.
14. RFC 0039 is updated or paired with RFC 0042 so the Go daemon owns
    workflow-state operations from the start of the rewrite.
15. Documentation updates name the supersession of D006/D007/D028, preserve
    D083, and define `.striatum/` as scratch plus optional tombstones.

## Test Requirements

The test suite should extend the RFC 0035 multi-repo harness rather than
inventing a separate shape.

Required coverage:

- Single-repo migration dry run and real import from a populated SQLite
  fixture that exercises all live-state tables.
- Idempotence/refusal tests: destination already has rows, source SQLite
  version is newer than supported, partial import marker exists, repository
  is unregistered, repository is disabled, and daemon schema is newer than
  the binary.
- Post-migration claim loop: prepare/start/register/claim/ack/heartbeat/
  publish/verdict/complete all operate against daemon Postgres for a
  migrated repository.
- Daemon-required refusal: stop the daemon and verify migrated live-state
  verbs refuse without mutating SQLite.
- Capability scope: repo-scoped token for repo A cannot inspect or mutate
  repo B's runs, jobs, queue, leases, artifacts, or events.
- Cross-repo lifecycle: two migrated repositories participate in a
  cross-repo workflow, with participant local runs represented by
  `(repository_id, run_id)`.
- Crash recovery: kill the daemon during import staging, import promotion,
  claim, publish-artifact, and cross-repo start. Restart reconciliation must
  either finish from a recorded safe point or refuse with a recoverable
  state, never produce orphaned live rows.
- Audit checkpoint verification: daemon doctor detects tampered imported
  row counts, missing imported artifacts, or a mismatched migration
  fingerprint.
- Go/Python parity once RFC 0039 work begins: the same migrated-state
  harness runs against both daemon cores.

## Non-Goals

- Hosted mode, cross-machine coordination, or remote transports beyond the
  existing owner-local daemon surfaces.
- Per-user auth, local multi-user tenancy, repository ACLs, or shared
  workstation hardening.
- OS-keyring redesign or token lifecycle redesign.
- Bundled/Dockerized Postgres.
- Transcript capture or broader request/response body persistence.
- Stronger sealed-apply guarantees than RFC 0031 already defines.
- Rewriting durable artifact formats or making repository artifacts
  authoritative workflow state.

## Open Questions

- Should V1 offer an export-from-Postgres-back-to-SQLite recovery path, or
  is the read-only SQLite tombstone the only rollback aid? Recommendation:
  no live downgrade in V1; document backup/restore and tombstone inspection
  only.
- Should `run_id` remain globally unique enough to use alone in operator UI,
  or should daemon-facing APIs always accept `(repo_id, run_id)`? Recommendation:
  make APIs repository-scoped even if generated IDs are globally unique.
- Should migration be explicitly operator-invoked only, or can daemon start
  offer `--migrate-repo-local` as a convenience wrapper? Recommendation:
  explicit verb first; convenience wrapper later after the refusal states are
  proven.
- RFC 0041 currently gestures at a future RFC 0042 for Engram Phase 1. If
  this work owns RFC 0042, the Engram roadmap should be renumbered before the
  RFC index is updated.
