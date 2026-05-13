---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/042/track_c/design/codex/DESIGN.md", "docs/dogfood/042/track_c/design/claude_code/DESIGN.md", "docs/dogfood/042/track_c/design/gemini/DESIGN.md"]
---

author: synthesizer-codex-1

# Track C Synthesis: Repo-local State To Daemon Postgres (RFC 0042 V1)

This synthesis reconciles the three Track C design inputs into a single
concrete plan that Codex can author `docs/rfcs/0042-repo-local-state-to-postgres.md`
against. It is a chosen plan, not a menu.

## 1. Product-Boundary Statement

RFC 0042 V1 moves the authoritative live workflow state currently stored
in each target repository at `.striatum/state.sqlite3` into the
daemon-owned Postgres database, keyed by `repository_id`. The daemon
becomes mandatory for every workflow mutation and every live-state read.
Repository files remain durable provenance; `.striatum/` becomes
operational scratch only. The Python and Go daemon cores share one
substrate from day one.

This synthesis supersedes the SQLite-as-live-substrate half of D006,
the repo-local-storage-location half of D007, and the "CLI writes
directly to repo-local SQLite without a daemon" half of D028. The
artifact-publish half of D028, and D006's "repository files are durable
provenance, not the live bus" half, are preserved unchanged. D082, D086,
D087, D088, and D093 stay as written. D083 (single-user trust boundary)
is explicitly preserved: "multi-tenancy" in this RFC means schema-level
row isolation keyed by `repository_id`, not user-level auth, not
shared-workstation hardening, not hosted mode.

## 2. Schema Changes

### 2.1 Table location and naming

All migrated tables live in the existing `striatumd` Postgres schema
alongside `striatumd.repositories`, `striatumd.audit_log`,
`striatumd.capability_tokens`, etc. There is no per-repository Postgres
schema. Per-repo isolation is row-level via `repository_id` because the
operator boundary is single-user and cross-repo operator queries are
cheap and frequent.

SQL columns and tables use `repository_id` consistently (matches the
existing RFC 0033 baseline). The RFC body and the operator-facing CLI
may use `repo_id` as a shorthand alias in documentation, but no SQL
column or migration uses that spelling.

### 2.2 Migrated tables

The following sixteen tables move from `.striatum/state.sqlite3` into
`striatumd.*`. The list is the full inventory of repo-local live-state
tables in `src/striatum/schema.py` plus migrations v2-v13 in
`src/striatum/migrations.py`. `schema_meta` does not migrate; the
Postgres substrate already has `striatumd.schema_migrations`.

| Old SQLite table | New Postgres table | New composite primary key |
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
| `process_executions` | `striatumd.process_executions` | `(repository_id, execution_id)` |
| `events` | `striatumd.events` | `(repository_id, event_id)` |
| `job_worktrees` | `striatumd.job_worktrees` | `(repository_id, worktree_id)` |
| `process_supervisors` | `striatumd.process_supervisors` | `(repository_id, supervisor_id)` |

`striatumd.process_supervisor_pointers` (added by repo-local migration
v13 but already daemon-shaped) keeps its existing daemon-side definition
and gains `(repository_id, run_id, session_id)` in its primary key if it
does not already carry repository scope explicitly. Cross-repo
coordination tables (`cross_repo_runs`, `cross_repo_run_repositories`,
`cross_repo_cycle_counters`) stay daemon-global. Their per-participant
`local_run_id` columns become `(repository_id, run_id)` pairs with
composite foreign keys into `striatumd.runs`.

### 2.3 Column shape rules

For every migrated table:

- `repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id)`
  is the leftmost primary-key column.
- All intra-table foreign keys become composite. Example:
  `jobs(repository_id, run_id)` references
  `runs(repository_id, run_id)`. A code path that forgets to filter by
  `repository_id` raises an SQL error rather than silently returning the
  wrong repo's row.
- All `repository_id` foreign keys use `ON DELETE RESTRICT`. Repo
  removal goes through the explicit `striatum daemon repo remove` flow
  under audit, never via cascade.
- All previously-active uniqueness constraints become repository-scoped.
  Concretely:
  - `runs`: `UNIQUE(repository_id, slug)` (was `UNIQUE(slug)`).
  - `jobs`: `UNIQUE(repository_id, run_id, workflow_job_id, attempt)`.
  - `queue_messages` active-claimability index: partial unique on
    `(repository_id, run_id, workflow_job_id)` where
    `state IN ('ready','claimed')`.
  - `leases`: partial unique on
    `(repository_id, resource_type, resource_id)` where `state = 'active'`.
  - `work_packets`: `UNIQUE(repository_id, message_id, lease_id)`.
  - `job_worktrees` active-worktree uniqueness becomes per
    `(repository_id, job_id)`.
  - `process_supervisors` active-supervisor uniqueness becomes per
    `(repository_id, session_id)`.
- All hot-path indexes lead with `repository_id`. For example,
  `runs_repo_state` partial index on `(repository_id, state)` where
  `state IN ('ready','running','blocked')`.
- Datatype upgrades from the SQLite shape: `INTEGER` 0/1 booleans
  become `boolean`; ISO8601 text timestamps become `timestamptz`;
  JSON-as-text columns become `jsonb`; `event_id` becomes `bigserial`
  scoped to one daemon DB (the canonical key is
  `(repository_id, event_id)`); `size_bytes` and similar counters
  become `bigint`.
- Append-only semantics on `events` and `artifacts` are enforced under
  Postgres roles plus row-level triggers refusing `UPDATE`/`DELETE` for
  daemon-runtime clients. The migration role retains those grants.
- Primary identifiers (`run_id`, `job_id`, `session_id`, `message_id`,
  `lease_id`, `packet_id`, `artifact_id`, `verdict_id`, `blocker_id`,
  `request_id`, `execution_id`, `worktree_id`, `supervisor_id`,
  `pointer_id`) are preserved verbatim across the migration. The
  migration adds `repository_id` around them; it never rewrites them.

### 2.4 New table: `striatumd.repo_state_migrations`

```sql
CREATE TABLE striatumd.repo_state_migrations (
  repository_id           text PRIMARY KEY REFERENCES striatumd.repositories(repository_id),
  source_sqlite_path      text        NOT NULL,
  source_sqlite_sha256    text        NOT NULL,
  source_schema_version   integer     NOT NULL,
  source_row_counts       jsonb       NOT NULL,
  imported_row_counts     jsonb       NOT NULL,
  source_events_rollup_sha256 text    NOT NULL,
  migration_started_at    timestamptz NOT NULL,
  migration_completed_at  timestamptz NOT NULL,
  migration_daemon_version text       NOT NULL,
  rollback_kept_readonly  boolean     NOT NULL DEFAULT false,
  cutover_marker_sha256   text        NOT NULL
);
```

This row is the operator's "did this repo migrate, when, from what,
into how many rows" audit record. A companion
`striatumd.repo_state_migrations_history` table archives prior rows when
a repo is purged or re-imported via `--force-replace-pg`.

### 2.5 Audit-chain extension

`striatumd.audit_log` (RFC 0033 baseline, already carries
`repository_id`) gains one column:

```sql
ALTER TABLE striatumd.audit_log
  ADD COLUMN repo_local_state_touched boolean NOT NULL DEFAULT false;
```

The hash payload format version bumps from `1` to `2` and the new bool
joins the canonical payload. Historical rows keep `hash_format_version = 1`
semantics so prior hashes do not change; the previous-hash link from the
first v2 row to the last v1 row is unchanged, so the chain stays
continuous across the bump. This mirrors how RFC 0033 already versions
the audit hash payload.

### 2.6 Daemon-side migration packaging

The schema changes ship as a new SQL migration file
`src/striatum/daemon_pg/sql/0005_repo_local_state.sql` and are applied
by the existing daemon migration loader. The Go daemon (RFC 0039)
embeds the same SQL file via `embed.FS` so both daemon cores apply the
identical schema. RFC 0042 does not introduce a language-specific
migration runner.

## 3. New CLI Verb: `striatum daemon migrate-repo-local`

### 3.1 Exact shape

```
striatum daemon migrate-repo-local
    --from sqlite                 (required; only mode in V1)
    --to pg                       (required; only mode in V1)
    --repo <path>                 (required; absolute or repo-relative)
   [--keep-sqlite-readonly]       (default: off)
   [--dry-run]                    (default: off)
   [--force-replace-pg]           (hidden; required to re-import an already-migrated repo)
   [--json]                       (machine-readable output)
```

The verb is a thin CLI shim around a single daemon RPC method,
`repo.migrate_local_state`, gated by the `admin` capability. The CLI
process never opens the SQLite file. The daemon process opens the
SQLite file (Python core via `sqlite3`; Go core via `modernc/sqlite`
CGO-free) and performs the import inside the same process that holds
the destination Postgres connection. Operators run one binary; there
is no separate migration tool.

### 3.2 Required behavior

1. **Path canonicalization.** Resolve `--repo` against the operator
   cwd, canonicalize, refuse if the path is not a directory.
2. **Repository registration.** Look up
   `striatumd.repositories.repo_root`. If absent, refuse with exit code
   14 (`repo_not_registered`) and name the remediation
   (`striatum daemon repo add <path>`). V1 does not auto-register.
3. **OS-owner check.** Refuse unless the operator is the daemon-owning
   OS user (D083 boundary).
4. **Destination preflight.** Refuse if any row exists in any migrated
   table for this `repository_id` unless `--force-replace-pg` is set;
   refuse even with `--force-replace-pg` if any active lease for this
   `repository_id` exists.
5. **Source preflight.** Open the SQLite file read-only. Refuse if the
   file is missing, unreadable, `PRAGMA integrity_check` fails, or
   `PRAGMA user_version` is newer than the migration set the daemon
   knows about (mirrors exit code 9 SchemaVersionError semantics in
   `src/striatum/migrations.py`).
6. **Source fingerprint.** Compute `source_sqlite_sha256` of the file
   bytes plus a `source_events_rollup_sha256` over the canonical CSV of
   the `events` rows in `event_id` order.
7. **Dry-run.** With `--dry-run`, emit a JSON report containing
   per-table row counts, source schema version, source fingerprint,
   destination readiness, daemon version, and the exact remediation
   command. No write to Postgres, no audit row, no marker, no SQLite
   mutation.
8. **Real import.** Inside one Postgres advisory lock keyed by
   `repository_id`, import rows into staging tables in foreign-key
   order, validate per-table row-count parity plus all composite FKs,
   then promote the staging rows into the live workflow-state tables
   in one bounded transaction per table. A mid-table failure rolls back
   that table and aborts; the operator re-runs the verb to retry. The
   destination must end with byte-identical primary identifiers and
   artifact hashes.
9. **Order.** The import order is `workflow_snapshots`, `runs`,
   `sessions`, `jobs`, `job_dependencies`, `work_packets`,
   `queue_messages`, `leases`, `artifacts`, `verdicts`, `blockers`,
   `command_requests`, `process_executions`, `events`, `job_worktrees`,
   `process_supervisors`. Cross-repo participant references resolve
   last via `striatumd.cross_repo_run_repositories` updates.
10. **Audit append.** Append one daemon-audit row per migrated table
    with `method = 'repo.migrate_local_state'`, `decision = 'allow'`,
    `repo_local_state_touched = true`, `params_sha256 = sha256(table_name)`,
    and metadata containing per-table imported counts. Append one
    summary row at the end with the per-table totals and the source
    fingerprint. All rows link via `previous_hash` in the v2 format.
11. **Cutover marker.** Insert one `striatumd.repo_state_migrations`
    row. Write a tombstone sentinel into the source SQLite file (a
    `schema_meta` row with `key = 'cutover_completed_at'`,
    `value = <ISO8601>`) so a stray Python process opening the file
    can refuse to mutate it. Compute `cutover_marker_sha256` over the
    sentinel row contents and record it in the migration row.
12. **Tombstone disposition.** With `--keep-sqlite-readonly`, rename
    the file to `.striatum/state.sqlite3.pre-cutover.<timestamp>`,
    `chmod 0400`, and record `rollback_kept_readonly = true`. Without
    the flag, delete the file. Either way the file is no longer live
    state.
13. **Repository state update.** Set
    `striatumd.repositories.settings_json->>'repo_local_state_storage' = 'pg'`
    and refresh `last_seen_at = now()` for the migrated repository.
14. **Concurrency.** While `repo_state_migrations.migration_completed_at`
    is null for this `repository_id`, the daemon refuses all other
    mutating RPC methods for that `repository_id` with exit code 19
    (`migration_in_progress`). The advisory lock prevents two
    migrations of the same repo from racing.

### 3.3 Auto-migration on daemon start: deferred

The first daemon start after RFC 0042 does not silently import. The
daemon detects unmigrated repositories (a `.striatum/state.sqlite3`
without the cutover sentinel) and marks them `state = 'needs_migration'`,
returning exit code 15 (`repo_unmigrated`) to any RPC method that
targets them and naming the migration command in the error envelope.
Operator-invoked migration only in V1; a follow-up RFC may add an
opt-in convenience wrapper once the refusal states are proven in
production.

## 4. `.striatum/` After V1: Operational Scratch Only

After the verb runs against a repo, `.striatum/` carries no
authoritative state. Allowed contents:

- `daemon.sock` and `daemon.pid` (when the daemon runs against this
  repo, created and removed by `daemon start`/`daemon stop`).
- `bin/` wrapper scripts installed by `striatum skills install`.
- `supervisor/<session-id>/` per-supervised-process scratch:
  `stdin.fifo`, `stdout.log`, `stderr.log`, `agent.pid`,
  `lifecycle.state` (plain-text debug aid only, never authoritative).
- `tmp/` scratch for transient CLI files (worktree scaffolding, etc.).
- `state.sqlite3.pre-cutover.<timestamp>` read-only tombstone when
  `--keep-sqlite-readonly` was used.

Forbidden after V1:

- A live `state.sqlite3` file with mutable schema. If a daemon start
  encounters one without the cutover sentinel and the repo is not
  marked `needs_migration`, it refuses to start for that repo and
  points the operator at the migration verb.
- Marker files representing authoritative state. The two existing
  marker shapes (`.striatum/cross_repo/*.json` if any, and any
  pre-RFC-0042 status markers) are migrated into Postgres rows or
  removed.

`striatum init` is revised so that it never creates `state.sqlite3`,
never seeds workflow-state rows, and only scaffolds the scratch
subdirectories listed above. `daemon doctor` verifies that
`state.sqlite3` is absent or carries the cutover sentinel, that
`daemon.sock` is owner-only `0600`, that `daemon.pid` matches a running
process, and that wrapper scripts match the bundled hash.

## 5. CLI Behavior Without A Daemon

The CLI never silently falls back to direct SQLite. The `--no-daemon`
flag is removed. The decision tree on every state-touching invocation:

```
1. Resolve daemon socket: $STRIATUM_DAEMON_SOCKET, then
   ~/.config/striatum/daemon.toml, then ~/.striatum/daemon.sock.
2. Connect and run the envelope-v1 handshake.
3. On success: route the verb as an RPC call. The daemon owns
   capability gating and audit append.
4. On failure: emit a structured, JSON-emittable error block and exit
   with the named exit code. Never touch any SQLite file or any
   repo-local mutable artifact.
```

Exit-code allocation (extends `src/striatum/errors.py`):

| Code | Class | Meaning |
| --- | --- | --- |
| 11 | `daemon_unavailable` | Cannot connect to daemon socket (process not running, wrong path, permissions). |
| 12 | `daemon_version_skew` | Handshake refused by version-skew rules. |
| 13 | `daemon_unauthorized` | Capability check failed for the requested verb / repo scope. |
| 14 | `repo_not_registered` | Daemon healthy, repository not in `striatumd.repositories`. Remediation names `striatum daemon repo add`. |
| 15 | `repo_unmigrated` | Daemon healthy, repository registered, `.striatum/state.sqlite3` present without cutover sentinel. Remediation names `striatum daemon migrate-repo-local`. |
| 16 | `pg_unavailable` | Daemon reached, Postgres unreachable. |
| 17 | `pg_repo_drift_detected` | Cutover sentinel and Postgres `repo_state_migrations` row disagree. Remediation names doctor + force-replace flow. |
| 18 | `source_sqlite_corrupt` | Migration preflight `PRAGMA integrity_check` failed. |
| 19 | `migration_in_progress` | Mutating RPC denied while migration for this repo is in flight. |

Each refusal emits both a human-readable block and a `--json` envelope
naming `error`, `reason`, `remediation`, and `docs` (relative path into
`docs/HOW_TO_HUMAN.md`). The contract is "refuse cleanly with a named
remediation," not "best effort." The only CLI verbs that may run before
the handshake succeeds are `striatum daemon start`, `striatum daemon
doctor`, and `striatum daemon repo add` (the registration on-ramp).

Read verbs follow the same rule: `status`, `dashboard`, `dashboard --once`,
`audit show`, `why`, and `run summary` all route through the daemon
after V1 because the daemon owns the only authoritative substrate.
A follow-up RFC may introduce a read-only direct-Postgres path for CI
scripts; V1 does not.

## 6. RFC 0039 Scope Revision (Go Daemon As Day-One Gateway)

RFC 0042 amends RFC 0039 so the Go rewrite is daemon-first for
workflow-state from its first useful milestone, not after a
Python-only transition. Concrete revisions, edited into
`docs/rfcs/0039-go-daemon-core.md` when RFC 0042 lands:

- **Step 2 (Postgres substrate).** The Go daemon's `pkg/db` embeds the
  full SQL migration tree including the new
  `0005_repo_local_state.sql`. The Go daemon does not have a
  "registry-only" mode; from the first build, its DB layer covers all
  migrated workflow-state tables.
- **Step 3 (read-only daemon).** The Go daemon's read-only RPC surface
  serves both daemon-owned reads (status, dashboard, audit show) and
  repo-local reads (`run show`, `job show`, `session list`, `why`,
  `dashboard --run-id`). Both read from the same Postgres schema.
- **Step 4 (mutating verbs).** The Go daemon's mutation table includes
  the full repo-local workflow-state verb set on its first end-to-end
  milestone: `run.prepare`, `run.start`, `session.register`,
  `job.claim_next`, `job.ack`, `job.heartbeat`, `job.publish_artifact`,
  `job.verdict`, `job.submit_review`, `job.complete`, `job.block`,
  `recovery.*`, `worktree.*`, `supervisor_pointer.*`. Each entry carries
  the same repo-scoped capability matrix the Python daemon uses
  (`write`, `claim`, `review`, `apply`, `recovery`, `admin`).
- **Step 5 (supervised processes).** Unchanged shape: `.striatum/`
  remains scratch for FIFOs and supervisor stdout logs. Supervisor
  pointer rows and lease heartbeats live in `striatumd.*`. The
  supervised wrapper talks to the daemon over the same socket using a
  supervisor-scoped capability token; it does not open SQLite.
- **Step 6 (distribution).** The Go binary includes a SQLite reader
  used by the `repo.migrate_local_state` RPC. Operators never need to
  keep a Python install around to migrate; the Go daemon performs the
  one-shot per-repo cutover itself.
- **Acceptance criteria.** RFC 0039 gains the line: "The Go daemon
  reads and writes the migrated workflow-state tables
  (`striatumd.{workflow_snapshots, runs, sessions, jobs,
  job_dependencies, queue_messages, leases, work_packets, artifacts,
  verdicts, blockers, command_requests, process_executions, events,
  job_worktrees, process_supervisors}`) from the same Postgres substrate.
  No repo-local SQLite file is opened by the daemon process at any
  time except inside the one-shot `repo.migrate_local_state` RPC."
- **Test harness.** The RFC 0035 multi-repo harness extends to run the
  full post-migration claim/ack/heartbeat/publish/verdict/complete
  lifecycle against both `daemon_core="python"` and `daemon_core="go"`.
  Phase 1 of the Go rewrite does not ship until both cores pass the
  same suite.

This revision avoids a split-brain period where the Go daemon owns
registry state while the Python CLI still opens repo-local SQLite.

## 7. Migration Ordering And Rollback

### 7.1 Per-repo ordering

The migration verb's table-by-table import order follows foreign-key
dependency: parent rows before child rows, as enumerated in §3.2 step 9.
Each table imports inside its own transaction; a failure leaves the
prior table's rows committed but the current table's rows rolled back.
The verb may be re-run; it restarts from the first table whose imported
count does not match the source. The advisory lock prevents concurrent
imports of the same repo.

### 7.2 Cross-repo ordering

Cross-repo coordination rows (`cross_repo_runs`,
`cross_repo_run_repositories`, `cross_repo_cycle_counters`) live in the
daemon-global schema and are not migrated per-repo. The migration verb
updates `cross_repo_run_repositories.local_run_id` references for the
migrated repository so existing cross-repo runs keep resolving. If a
participant repository is unmigrated, cross-repo verbs that touch it
return exit code 15 (`repo_unmigrated`).

### 7.3 Rollback

Rollback is intentionally a break-glass operator step in V1, not an
automatic verb. The options:

- **Forensic inspection.** With `--keep-sqlite-readonly`, the
  pre-cutover SQLite file remains at
  `.striatum/state.sqlite3.pre-cutover.<timestamp>` mode `0400` and is
  readable by any SQLite tool. `striatum daemon doctor --repo <path>
  --check-migration` cross-validates the tombstone against the
  Postgres rows.
- **Re-import.** `striatum daemon migrate-repo-local --from sqlite
  --to pg --repo <path> --force-replace-pg` wipes the Postgres rows
  for this `repository_id` (refusing if any active lease exists),
  archives the previous `repo_state_migrations` row into
  `repo_state_migrations_history`, and re-runs the import.
- **Full revert (documented but not scripted).** Stop the daemon,
  restore the SQLite file in place, drop the
  `repo_local_state_storage = 'pg'` setting on the repository row,
  remove the cutover sentinel. Any Postgres-side work performed since
  cutover is discarded. V1 documents this in `docs/HOW_TO_HUMAN.md`
  but ships no automation; the audit row explaining why is left
  intact.

V1 does not offer an export-from-Postgres-back-to-SQLite verb. The
tombstone plus the `repo_state_migrations` audit row are the rollback
surface.

## 8. Audit-Chain Integrity Preservation

Two distinct logs serve two distinct purposes after RFC 0042. The
synthesis preserves both:

1. **Daemon audit log** (`striatumd.audit_log`, RFC 0033). Metadata-only
   authorization and mutation decisions. The migration verb appends one
   row per migrated table plus a summary row, each with
   `previous_hash` linkage to the prior row and
   `hash_format_version = 2`. The chain remains continuous across the
   format-version bump because v1 row hashes are not recomputed and the
   first v2 row links to the last v1 row's hash. Migration audit rows
   carry table name, imported row count, source fingerprint, and
   destination schema version. They never carry artifact contents,
   prompt bodies, verdict rationale, or blocker descriptions.
2. **Per-repo run causal chain** (the `events` table, now
   `striatumd.events`). Not a hash chain today, but causally ordered
   by `event_id` plus `created_at`. The migration verb copies events
   in `event_id` order, preserves original timestamps, and computes
   `source_events_rollup_sha256` over the canonical CSV before import.
   Each imported event row carries a `migration_provenance` JSONB
   field:

   ```json
   {
     "source_sqlite_sha256": "<hash>",
     "source_event_id": <int>,
     "migration_audit_id": <int>
   }
   ```

   Events written after migration via RPC have
   `migration_provenance = NULL`.

`daemon doctor --repo <path> --check-migration` is the operator-facing
integrity verifier. It:

- recomputes `source_events_rollup_sha256` on the Postgres side from
  the imported rows for this `repository_id` and compares it to the
  value in `striatumd.repo_state_migrations`;
- if a `--keep-sqlite-readonly` tombstone is present, recomputes the
  rollup against the tombstone for an independent third check;
- walks the daemon audit chain back from the migration summary row,
  verifying `previous_hash` linkage and the recomputed payload hash;
- reports `pg_repo_drift_detected` (exit 17) if the values disagree.

The migration verb does not attempt to retroactively hash the SQLite
events as if they had been an RFC 0033 audit chain. The
`source_events_rollup_sha256` is sufficient for after-the-fact
tamper-evidence on the imported rows.

## 9. Supersession Framing

RFC 0042 declares the following effects on the decision log. The PR
landing RFC 0042 updates `docs/DECISION_LOG.md` accordingly.

- **D006** is superseded by D093 for the live-coordination-substrate
  half: the v1 SQLite live message bus moves into daemon Postgres.
  D006's "repository files are durable artifacts, not the live bus"
  half is preserved unchanged and restated in the RFC.
- **D007** is superseded by D093 for the storage-location half:
  `.striatum/state.sqlite3` is no longer authoritative state. The
  `.striatum/` directory is preserved as the per-repo scratch root
  (sockets, pidfiles, supervisor FIFOs, wrapper scripts, optional
  read-only tombstone). `striatum init` is amended in the same PR.
- **D028** is superseded by D093 for the "CLI writes directly to
  repo-local SQLite without a daemon" half. The artifact-publish
  policy half (decisions/prompts/findings published; transcripts
  not captured) is preserved unchanged and explicitly restated.
- **D082**, **D086**, **D087**, **D088** are preserved and reinforced:
  daemon-first product, non-SQLite daemon substrate, envelope-v1 RPC,
  and daemon-owned supervision/sealed-apply authority now extend to
  every former repo-local table. RFC 0042 closes the loophole D086
  named ("V1 repo-local SQLite remains the authoritative run-state
  store during the transition") by retiring that substrate.
- **D083** is explicitly preserved. The trust boundary remains one OS
  user per machine. "Multi-tenancy" in RFC 0042 means schema-level
  row isolation keyed by `repository_id`, capability checks scoped
  per repository, and cross-repo workflows naming explicit participant
  repositories. It is not user-level auth, not shared-workstation
  hardening, not hosted mode, not cross-machine semantics.

The DECISION_LOG entry for D093 (already accepted) is the umbrella
decision; RFC 0042's PR appends a "Supersedes: D006, D007, D028 (in
part); see D093" pointer onto each of those rows.

## 10. Acceptance Criteria Summary

1. Migration `0005_repo_local_state.sql` creates the sixteen
   repo-keyed workflow-state tables, the `repo_state_migrations`
   table, the history table, and the `repo_local_state_touched` audit
   column. The Python and Go daemons both apply it.
2. Every workflow-state row carries `repository_id` directly or via a
   composite foreign key whose parent includes `repository_id`. All
   active uniqueness, claimability, queue, lease, event, artifact,
   verdict, blocker, process, worktree, and supervisor-pointer indexes
   are repository-scoped.
3. `striatum daemon migrate-repo-local --from sqlite --to pg --repo
   <path> --dry-run` reports source counts, source schema version,
   source fingerprint, and destination readiness with no writes.
4. The non-dry-run migration imports an existing
   `.striatum/state.sqlite3` into `striatumd.*` with row-count parity,
   preserved primary identifiers, preserved artifact content hashes,
   preserved event ordering per run, and validated composite foreign
   keys.
5. `--keep-sqlite-readonly` produces a `0400` tombstone at
   `.striatum/state.sqlite3.pre-cutover.<timestamp>` and a verifiable
   `repo_state_migrations` row.
6. The migration appends one daemon audit row per migrated table plus
   a summary, each with `hash_format_version = 2` and continuous
   `previous_hash` linkage. `daemon doctor --check-migration` verifies
   the chain and the events rollup.
7. After migration, every state-touching CLI verb routes through
   daemon RPC and refuses with exit code 11/12/13/14/15/16/17/18/19
   plus a named remediation when the daemon is unavailable,
   incompatible, unauthorized, the repo is unregistered, the repo is
   unmigrated, Postgres is unreachable, drift is detected, the source
   SQLite is corrupt, or a migration is in flight. No silent fallback.
8. `--no-daemon` is removed. `striatum init` no longer creates
   `state.sqlite3` or seeds workflow-state rows.
9. The daemon RPC method registry exposes the full repo-local verb
   set with repo-scoped capability gating. A token scoped to repo A
   cannot read or mutate repo B workflow state.
10. Cross-repo participant rows in `striatumd.cross_repo_run_repositories`
    use `(repository_id, run_id)` composite references into
    `striatumd.runs`.
11. RFC 0039 is amended so the Go daemon implements the full
    workflow-state RPC surface from Step 3 (reads) and Step 4
    (mutations); the RFC 0035 multi-repo harness exercises both cores.
12. The Go daemon binary embeds a SQLite reader sufficient for the
    one-shot `repo.migrate_local_state` RPC; operators do not need a
    parallel Python install to migrate.
13. `docs/DECISION_LOG.md`, `docs/SPEC.md`, `docs/HOW_TO_HUMAN.md`,
    `docs/HOW_TO_AGENT.md`, and `docs/UBIQUITOUS_LANGUAGE.md` are
    updated to name `.striatum/` as scratch-only, list the new exit
    codes, point at the migration verb, restate D083, and explicitly
    cite D006/D007/D028 supersession via D093.

## 11. Out Of Scope For V1

- Hosted mode, cross-machine coordination, remote transports beyond
  the existing owner-local daemon surface.
- Per-user auth, repository ACLs, shared-workstation hardening.
- OS-keyring redesign, token-lifecycle redesign.
- Bundled or Dockerized Postgres distribution.
- Transcript capture or request/response body persistence.
- Stronger sealed-apply guarantees than RFC 0031.
- Export-from-Postgres-back-to-SQLite live downgrade verb.
- Auto-migration on daemon start (deferred to a follow-up).
- A read-only direct-Postgres path for CI poll loops (deferred).

The implementer for Track C is Codex. The single deliverable is
`docs/rfcs/0042-repo-local-state-to-postgres.md` authored against this
synthesis, plus the supporting code, migration, CLI, and documentation
edits the RFC body enumerates.
