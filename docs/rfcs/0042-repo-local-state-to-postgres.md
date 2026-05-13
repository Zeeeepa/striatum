# RFC 0042: Repo-Local State to Postgres

Status: proposed
Date: 2026-05-13
Context:
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D006, D007, D028, D082, D086, D087, D088, D093),
[`RFC 0028`](0028-long-running-daemon-and-multi-repository-control-plane.md),
[`RFC 0030`](0030-daemon-rpc-server-and-version-skew-protocol.md),
[`RFC 0033`](0033-storage-substrate-rewrite-for-daemon-v2.md),
[`RFC 0039`](0039-go-daemon-core.md),
[`docs/dogfood/042/track_c/DESIGN_SYNTHESIS.md`](../dogfood/042/track_c/DESIGN_SYNTHESIS.md)

## Problem

Striatum's live workflow state is still siloed in each target
repository at `.striatum/state.sqlite3`. That was the right V1 shape:
D006 chose SQLite as the local coordination layer, D007 put it inside
the target repository, and D028 kept durable repository artifacts
curated while avoiding transcript capture.

Those decisions now conflict with the accepted daemon-first product
shape. D082 makes the daemon the long-term product surface. D086 and
RFC 0033 move daemon-owned state to system Postgres. D087 and RFC 0030
define daemon RPC and capability-bound method routing. D088 moves
supervision and sealed-apply authority behind the daemon. RFC 0032 and
RFC 0035 rely on multi-repository coordination that must reason across
repositories without opening N independent SQLite files.

The current split leaves the daemon authoritative for registry, audit,
capabilities, supervision, and cross-repo coordination, while ordinary
run state remains outside the daemon in per-repo SQLite files. That
creates three problems:

- Cross-repo workflows require joins and transaction ordering across
  daemon Postgres and several repo-local SQLite databases.
- Capability checks and audit rows can be bypassed by direct CLI code
  paths that still mutate `.striatum/state.sqlite3`.
- RFC 0039's Go daemon could become a registry gateway while the Python
  CLI continues to own workflow-state mutation, creating split-brain
  semantics during the daemon transition.

RFC 0042 closes that split. The daemon-owned Postgres database becomes
the authoritative live state store for workflow state, keyed by
`repository_id`. Repository files remain durable provenance, and
`.striatum/` becomes operational scratch only.

## Goals

- Move every repo-local workflow-state table from
  `.striatum/state.sqlite3` into the daemon-owned Postgres schema.
- Key all migrated rows by `repository_id` so one daemon DB can hold
  many registered target repositories without weakening D083's
  single-user trust boundary.
- Preserve primary identifiers, artifact hashes, event ordering, and
  existing append-only semantics during migration.
- Add an explicit, auditable migration verb:
  `striatum daemon migrate-repo-local --from sqlite --to pg --repo
  <path> [--keep-sqlite-readonly] [--dry-run]`.
- Make the daemon mandatory for all workflow-state reads and writes.
  The CLI refuses cleanly without a daemon instead of falling back to
  direct SQLite.
- Revise RFC 0039 so the Go daemon is the day-one gateway for all
  repo-local workflow operations, not only registry or daemon-owned
  operations.
- Preserve RFC 0033 audit-chain integrity and add migration evidence
  without storing transcripts, request bodies, artifact contents, or
  free-text workflow prose in the daemon audit log.

## Non-Goals

- No hosted mode, cross-machine coordination, or remote transport beyond
  the existing owner-local daemon boundary.
- No user-level auth changes, repository ACLs, shared-workstation
  hardening, or multi-OS-user semantics. D083 is unchanged.
- No bundled, embedded, or Dockerized Postgres distribution. RFC 0033's
  system-Postgres requirement remains.
- No OS-keyring or capability-token lifecycle redesign.
- No transcript capture and no daemon persistence of request/response
  bodies.
- No stronger sealed-apply guarantee than RFC 0031 already defines.
- No export-from-Postgres-back-to-SQLite live downgrade verb.
- No automatic migration on daemon start in V1.
- No read-only direct-Postgres path for CI poll loops in V1.

## Proposal

### 1. State Location And Schema

All migrated tables live in the existing daemon Postgres schema,
`striatumd`, beside `striatumd.repositories`,
`striatumd.audit_log`, and `striatumd.capability_tokens`. There is no
per-repository Postgres schema. Isolation is row-level through
`repository_id` because Striatum remains a single-user, single-machine
tool; "multi-tenant" in this RFC means many registered target
repositories, not many users.

SQL uses `repository_id` consistently. Documentation and CLI help may
use `repo_id` as shorthand, but migrations and code use
`repository_id`.

The eighteen repo-local application tables migrate or are accounted for
as follows. `schema_meta` is the nineteenth repo-local table, but it is
not workflow state and does not migrate; it is used only for source
schema inspection and the cutover sentinel.

| Old SQLite table | New Postgres table | New primary key |
| --- | --- | --- |
| `workflow_snapshots` | `striatumd.workflow_snapshots` | `(repository_id, workflow_snapshot_id)` |
| `runs` | `striatumd.runs` | `(repository_id, run_id)` |
| `sessions` | `striatumd.sessions` | `(repository_id, session_id)` |
| `jobs` | `striatumd.jobs` | `(repository_id, job_id)` |
| `job_dependencies` | `striatumd.job_dependencies` | `(repository_id, job_id, depends_on_job_id)` |
| `queue_messages` | `striatumd.queue_messages` | `(repository_id, message_id)` |
| `leases` | `striatumd.leases` | `(repository_id, lease_id)` |
| `work_packets` | `striatumd.work_packets` | `(repository_id, packet_id)` |
| `artifacts` | `striatumd.artifacts` | `(repository_id, artifact_id)` |
| `verdicts` | `striatumd.verdicts` | `(repository_id, verdict_id)` |
| `blockers` | `striatumd.blockers` | `(repository_id, blocker_id)` |
| `command_requests` | `striatumd.command_requests` | `(repository_id, request_id)` |
| `process_executions` | `striatumd.process_executions` | `(repository_id, process_id)` |
| `events` | `striatumd.events` | `(repository_id, event_id)` |
| `job_worktrees` | `striatumd.job_worktrees` | `(repository_id, worktree_id)` |
| `process_supervisors` | `striatumd.process_supervisors` | `(repository_id, supervisor_id)` |
| `process_supervisor_pointers` | `striatumd.process_supervisor_pointers` | `(repository_id, supervisor_id)` |

Daemon Postgres already has `striatumd.schema_migrations`.
`process_supervisor_pointers` keeps its daemon-shaped meaning, but it
must still be repository-scoped because it is repo-local today.
Cross-repo tables remain daemon-global, but participant references
become composite `(repository_id, run_id)` references into
`striatumd.runs`.

For every migrated table:

- `repository_id text NOT NULL REFERENCES
  striatumd.repositories(repository_id) ON DELETE RESTRICT` is the
  leftmost primary-key column.
- All intra-workflow foreign keys become composite and include
  `repository_id`.
- Active uniqueness constraints and partial indexes become
  repository-scoped.
- Hot-path indexes lead with `repository_id`.
- SQLite integer booleans become `boolean`, timestamp text becomes
  `timestamptz`, JSON text becomes `jsonb`, and counters become
  `bigint` where appropriate.
- `events` and `artifacts` retain append-only behavior through
  Postgres roles and triggers that deny runtime `UPDATE` and `DELETE`.
- Existing ids are preserved verbatim; the migration wraps them in a
  repository key rather than rewriting them.

### 2. Migration Tracking

RFC 0042 adds:

```sql
CREATE TABLE striatumd.repo_state_migrations (
  repository_id text PRIMARY KEY REFERENCES striatumd.repositories(repository_id),
  source_sqlite_path text NOT NULL,
  source_sqlite_sha256 text NOT NULL,
  source_schema_version integer NOT NULL,
  source_row_counts jsonb NOT NULL,
  imported_row_counts jsonb NOT NULL,
  source_events_rollup_sha256 text NOT NULL,
  migration_started_at timestamptz NOT NULL,
  migration_completed_at timestamptz NOT NULL,
  migration_daemon_version text NOT NULL,
  rollback_kept_readonly boolean NOT NULL DEFAULT false,
  cutover_marker_sha256 text NOT NULL
);
```

A companion `striatumd.repo_state_migrations_history` archives prior
rows when an admin re-imports a repository with the explicit
break-glass path.

The daemon audit log gains:

```sql
ALTER TABLE striatumd.audit_log
  ADD COLUMN repo_local_state_touched boolean NOT NULL DEFAULT false;
```

The audit hash payload format bumps from version 1 to version 2 so the
new boolean is included in canonical hashes. Historical audit rows are
not recomputed. The first v2 row links to the last v1 row through
`previous_hash`, preserving chain continuity.

These changes ship as a daemon Postgres migration, for example
`src/striatum/daemon_pg/sql/0005_repo_local_state.sql`, loaded by the
existing daemon migration runner. The Go daemon embeds and applies the
same SQL file; RFC 0042 does not introduce a separate migration system.

### 3. Migration Verb

The new operator verb is:

```text
striatum daemon migrate-repo-local
    --from sqlite
    --to pg
    --repo <path>
   [--keep-sqlite-readonly]
   [--dry-run]
   [--force-replace-pg]
   [--json]
```

`--from sqlite` and `--to pg` are required and are the only V1 modes.
`--force-replace-pg` is hidden help text and requires admin capability;
it refuses if any active lease exists for the repository.

The CLI is a thin shim over daemon RPC method
`repo.migrate_local_state`, gated by `admin`. The CLI process never
opens SQLite. The daemon opens the source SQLite database read-only,
connects to Postgres, performs the import, and writes audit rows.

Required behavior:

1. Resolve and canonicalize `--repo`; refuse if it is not a directory.
2. Require an existing `striatumd.repositories` row. V1 does not
   auto-register repositories.
3. Enforce the daemon-owning OS user boundary.
4. Refuse if destination rows already exist unless
   `--force-replace-pg` is set.
5. Open SQLite read-only and refuse missing files, unreadable files,
   failed `PRAGMA integrity_check`, or source schema versions newer
   than the daemon supports.
6. Compute `source_sqlite_sha256` and
   `source_events_rollup_sha256` before import.
7. In `--dry-run`, report row counts, schema version, fingerprints,
   destination readiness, daemon version, and remediation commands.
   Dry-run writes nothing, including no audit row and no cutover marker.
8. In a real import, take a Postgres advisory lock keyed by
   `repository_id`, import into staging tables in foreign-key order,
   validate row-count parity and composite foreign keys, then promote
   rows into live tables.
9. Import in this order: `workflow_snapshots`, `runs`, `sessions`,
   `jobs`, `job_dependencies`, `work_packets`, `queue_messages`,
   `leases`, `artifacts`, `verdicts`, `blockers`,
   `command_requests`, `process_executions`, `events`,
   `job_worktrees`, `process_supervisors`,
   `process_supervisor_pointers`.
10. Append one daemon audit row per migrated table plus a summary row,
    all with `repo_local_state_touched = true`.
11. Insert `striatumd.repo_state_migrations`, write a cutover sentinel
    into the source SQLite `schema_meta`, and record
    `cutover_marker_sha256`.
12. With `--keep-sqlite-readonly`, rename the source to
    `.striatum/state.sqlite3.pre-cutover.<timestamp>` and chmod it
    `0400`; otherwise delete the file.
13. Mark the repository settings with
    `repo_local_state_storage = "pg"`.
14. While migration is in progress, refuse other mutating RPCs for that
    repository with `migration_in_progress`.

The first daemon start after RFC 0042 does not silently import state.
If a registered repository has an unmigrated `.striatum/state.sqlite3`,
the daemon marks it as needing migration and state-touching methods
return a named refusal with the migration command.

### 4. `.striatum/` Becomes Scratch

After migration, `.striatum/` is not authoritative state. Allowed
contents are operational scratch: daemon socket and pid files, installed
wrapper scripts, supervised-process FIFOs and logs, temporary files, and
an optional read-only pre-cutover SQLite tombstone.

Forbidden contents are any live mutable `state.sqlite3` file and any
marker file that claims to be authoritative workflow state. `striatum
init` no longer creates `state.sqlite3` or seeds workflow rows; it only
creates scratch directories and ignore rules. `daemon doctor` verifies
the scratch directory shape, socket permissions, pid liveness, wrapper
hashes, and any migration tombstone.

### 5. CLI Behavior Without A Daemon

The CLI never silently falls back to direct SQLite. The `--no-daemon`
flag is removed for state-touching verbs. On every state-touching
invocation the CLI resolves the daemon socket, runs the RFC 0030
envelope-v1 handshake, and routes the request as daemon RPC. On failure
it emits a structured, JSON-emittable error block and exits without
touching SQLite or mutable repo-local artifacts.

New refusal codes:

| Code | Error | Meaning |
| --- | --- | --- |
| 11 | `daemon_unavailable` | Cannot connect to daemon socket. |
| 12 | `daemon_version_skew` | Handshake refused by version-skew rules. |
| 13 | `daemon_unauthorized` | Capability check failed. |
| 14 | `repo_not_registered` | Repository is not registered with the daemon. |
| 15 | `repo_unmigrated` | Repository still has unmigrated SQLite state. |
| 16 | `pg_unavailable` | Daemon is reachable but Postgres is unreachable. |
| 17 | `pg_repo_drift_detected` | Cutover sentinel and Postgres migration row disagree. |
| 18 | `source_sqlite_corrupt` | Migration preflight found corrupt SQLite source. |
| 19 | `migration_in_progress` | Another migration owns the repository lock. |

Each error includes `error`, `reason`, `remediation`, and `docs`.
`striatum daemon start`, `striatum daemon doctor`, and
`striatum daemon repo add` are the on-ramp verbs that may run before a
repository has migrated.

Read verbs follow the same rule. `status`, `why`, `dashboard`,
`run summary`, evidence export, audit views, and job/session/run
introspection all route through the daemon because the daemon owns the
only authoritative substrate.

The state-touching inventory includes:

- lifecycle verbs: `init`, `run prepare`, `run start`, `run cancel`,
  `run pause`, `run resume`, `run retry-job`, `branch confirm`, and
  `checkpoint resolve`;
- session and queue verbs: `register-session`, `session close`,
  `claim-next`, `ack`, `heartbeat`, `release`, `send`, `block`, and
  `complete`;
- artifact and review verbs: `publish-artifact`, `verdict`,
  `override-verdict`, `submit-review`, `decision record`, evidence
  export, and `run summary`;
- read and introspection verbs: `status`, `why`, `doctor`, `dashboard`,
  `run graph`, and `list runs/sessions/jobs/artifacts/workflows`;
- recovery and process verbs: `recovery stale-leases`,
  `recovery requeue-stale`, `recovery cancel-job`,
  `recovery process-reconcile`, `recovery resume`, `recovery auto`,
  `recovery watch`, `adapter run`, `worktree create/release/list`, and
  `supervise start/send/stop/status/list`;
- wrapper surfaces that call the same dispatcher, including
  `striatum.api.invoke`, local service `POST /v1/invoke`, MCP tools,
  and web dogfood-lifecycle chat tools.

Daemon and registry verbs that currently inspect or initialize
repo-local state (`repo add --init`, `daemon start`, daemon sweep,
daemon-backed reads) switch to Postgres-backed repository state after
migration. The only allowed SQLite opening in V1 is the daemon-owned
read-only source scan inside `repo.migrate_local_state`.

### 6. RFC 0039 Scope Revision

RFC 0042 amends RFC 0039 so the Go daemon handles all workflow-state
operations from its first useful milestone:

- The Go DB layer embeds the full daemon SQL migration tree, including
  the repo-local state migration.
- The read-only milestone serves both daemon-owned reads and former
  repo-local reads from Postgres.
- The mutation milestone includes the full workflow verb table:
  `run.prepare`, `run.start`, `session.register`, `job.claim_next`,
  `job.ack`, `job.heartbeat`, `job.publish_artifact`, `job.verdict`,
  `job.submit_review`, `job.complete`, `job.block`, `recovery.*`,
  `worktree.*`, and `supervisor_pointer.*`.
- Supervised wrappers talk to the daemon over the same socket and do
  not open SQLite.
- The Go binary includes a SQLite reader used only by the one-shot
  `repo.migrate_local_state` RPC.
- The RFC 0035 multi-repo harness runs the post-migration
  claim/ack/heartbeat/publish/verdict/complete lifecycle against both
  Python and Go daemon cores before the Go daemon ships.

The resulting RFC 0039 acceptance criterion is: the Go daemon reads and
writes all migrated workflow-state tables from the same Postgres
substrate, including `process_supervisor_pointers`, and no repo-local
SQLite file is opened by the daemon except inside
`repo.migrate_local_state`.

### 7. Migration Ordering And Rollback

Per-repository migration follows the foreign-key order named above.
Each table promotion is transactional. If a table fails, that table is
rolled back and the operator can rerun the command; previously imported
tables are validated by count and hash before proceeding.

Cross-repo coordination rows stay daemon-global. The migration updates
participant references so existing cross-repo runs resolve through
`(repository_id, run_id)`. If a cross-repo run includes an unmigrated
participant, cross-repo verbs return `repo_unmigrated`.

Rollback is deliberately break-glass:

- `--keep-sqlite-readonly` leaves a forensic tombstone.
- `--force-replace-pg` re-imports after archiving the previous
  migration row and refusing active leases.
- A full revert is documented but not automated: stop daemon, restore
  SQLite, remove the repository's `repo_local_state_storage = "pg"`
  setting, and accept that Postgres-side work since cutover is
  discarded.

V1 does not ship a Postgres-to-SQLite export command.

### 8. Audit And Event Integrity

RFC 0042 preserves two separate evidence streams:

- `striatumd.audit_log` records metadata-only authorization and
  mutation decisions. Migration appends one row per table plus a
  summary with continuous hash linkage and `hash_format_version = 2`.
  Rows carry table name, imported row count, source fingerprint, and
  destination schema version, but never artifact contents, prompts,
  verdict rationale, blocker descriptions, request bodies, or
  transcripts.
- `striatumd.events` preserves the per-repo causal event stream. The
  migration copies events in `event_id` order, preserves timestamps,
  and records migration provenance:

```json
{
  "source_sqlite_sha256": "<hash>",
  "source_event_id": 123,
  "migration_audit_id": 456
}
```

`daemon doctor --repo <path> --check-migration` recomputes the events
rollup from Postgres, optionally recomputes it from the read-only
tombstone, walks the audit chain through the migration summary row, and
reports `pg_repo_drift_detected` on mismatch.

## Acceptance Criteria

- A daemon Postgres migration creates the repository-keyed workflow
  state tables, `repo_state_migrations`, migration history, and
  `repo_local_state_touched`.
- Every workflow-state row is repository-scoped directly or through a
  composite foreign key. Active uniqueness, queue, lease, event,
  artifact, verdict, blocker, process, worktree, and supervisor indexes
  are repository-scoped.
- `striatum daemon migrate-repo-local --from sqlite --to pg --repo
  <path> --dry-run` reports counts, source schema version, source
  fingerprint, and destination readiness without writes.
- Non-dry-run migration imports an existing `.striatum/state.sqlite3`
  with row-count parity, preserved identifiers, preserved artifact
  content hashes, preserved event ordering, and validated composite
  foreign keys.
- `--keep-sqlite-readonly` creates a `0400` tombstone and a verifiable
  `repo_state_migrations` row.
- Migration appends one audit row per migrated table plus a summary,
  all with continuous hash linkage. `daemon doctor --check-migration`
  verifies the audit chain and events rollup.
- After migration, every state-touching CLI verb routes through daemon
  RPC and refuses with codes 11-19 plus named remediation when required.
  No silent fallback remains.
- `--no-daemon` is removed for state-touching verbs, and `striatum init`
  no longer creates repo-local live state.
- The daemon method registry exposes the full former repo-local verb set
  with repo-scoped capability gating. A token scoped to repository A
  cannot read or mutate repository B workflow state.
- Cross-repo participant rows use composite `(repository_id, run_id)`
  references into `striatumd.runs`.
- RFC 0039 is amended so the Go daemon implements the full workflow
  read and mutation surface from its initial useful milestones.
- The Go daemon can perform the one-shot SQLite-to-Postgres migration
  without requiring a parallel Python daemon.
- Documentation and decision-log consolidation explicitly name the
  partial supersession of D006, D007, and D028 under the D093 decision
  expected by the Track C synthesis, while preserving D083 and D028's
  no-transcripts artifact policy.

## Implementation Plan

1. Add the daemon Postgres migration and schema tests for
   repository-scoped keys, composite foreign keys, partial indexes,
   append-only triggers, and audit hash payload version 2.
2. Add repository-state data-access helpers in the daemon layer and
   route existing workflow reads through them.
3. Implement `repo.migrate_local_state` with dry-run, source integrity
   checks, staging imports, row-count parity checks, events rollup,
   audit rows, cutover marker, and optional tombstone handling.
4. Add CLI plumbing for `striatum daemon migrate-repo-local` and the
   new refusal codes.
5. Move workflow mutations behind daemon RPC method registrations with
   repository-scoped capability checks.
6. Remove direct SQLite fallback for migrated repositories, then remove
   `--no-daemon` for state-touching verbs.
7. Update `striatum init` and `daemon doctor` so `.striatum/` is
   scratch-only and migration drift is reportable.
8. Extend cross-repo tables and lifecycle helpers to use
   `(repository_id, run_id)` participant references.
9. Extend RFC 0035 harness coverage for migrated single-repo and
   cross-repo lifecycles, including repo-scoped token denial.
10. Amend RFC 0039 implementation milestones so Go and Python daemon
    cores share the same migrated workflow-state substrate from day one.
11. Update product docs, decision-log rows, and RFC index in the
    consolidation phase, not in this RFC-drafting job.

## Supersession Statement

RFC 0042 supersedes D006, D007, and D028 only in part:

- D006's SQLite live-state substrate is superseded by daemon Postgres.
  D006's rule that repository files are durable provenance, not the live
  message bus, remains true.
- D007's `.striatum/state.sqlite3` storage location is superseded.
  `.striatum/` remains the per-repository scratch root.
- D028's direct-CLI-write path into local state is superseded by
  daemon-mediated writes. D028's curated artifact policy and
  no-transcripts default remain unchanged.

D082, D086, D087, and D088 are reinforced. D083 is preserved: this RFC
does not add multi-user, hosted, cross-machine, or shared-workstation
semantics. The Track C synthesis identifies D093 as the decision-log
entry for the umbrella supersession; this RFC references that expected
decision and leaves decision-log reconciliation to the consolidation
job.

## Open Questions

- Should a future RFC add an opt-in automatic migration wrapper on daemon
  start after V1 refusal states have proven stable?
- Should a future RFC add a read-only direct-Postgres mode for CI scripts,
  or should all CI and automation continue to use daemon RPC?
- How long should read-only SQLite tombstones be retained by default when
  `--keep-sqlite-readonly` is used?
- Should `--force-replace-pg` remain hidden forever, or become a
  documented recovery command after migration tooling matures?
- Should a future bundled-Postgres distribution change any operator
  migration ergonomics, while leaving the schema and daemon ownership
  unchanged?

## Domain Modeling

RFC 0042 changes the persistence boundary for existing aggregates rather
than inventing a new workflow concept. `Run`, `Session`, `Job`,
`QueueMessage`, `Lease`, `WorkPacket`, `Artifact`, `Verdict`,
`Blocker`, process metadata, and `Event` remain the same domain
objects. Their repository membership becomes an explicit value in the
daemon DB through `repository_id`.

The new concept is `repo_state_migration`: a daemon-owned audit record
for a one-time state-store cutover of a registered target repository.
It is a value object attached to a repository tenant, not a workflow
job and not durable project provenance. It records source identity,
row-count parity, event rollup, daemon version, and cutover marker so
operators can verify that the live-state boundary moved without losing
workflow history.
