# Track C Design: Repo-local State → Postgres (RFC 0042 V1)

author: designer-claude-opus-003

## Framing

This design draft proposes RFC 0042 V1 acceptance criteria for moving the
contents of repo-local `.striatum/state.sqlite3` into the daemon-owned
PostgreSQL DB, keyed by `repo_id`. It supersedes:

- **D006** — SQLite as v1 live coordination layer.
- **D007** — Repo-local state under `.striatum/`.
- **D028** — Local-only writes via the CLI (the artifact-publish half stays;
  the "CLI writes directly to repo-local SQLite without a daemon" half goes).

D006/D007 chose SQLite under `.striatum/` because v1 was a short-lived CLI
with a single writer per repo. With the daemon-first product positioning
(D082), the language port to Go (D084 → RFC 0039), and the daemon-owned
substrate already moved to Postgres for daemon-side state (RFC 0033), the
last reason to keep two storage substrates evaporates: there is now a
running Postgres instance that already owns the audit chain, capability
tokens, scheduler cursors, and supervisor metadata. Splitting "live run
state" off into a separate SQLite file per repo produces:

- two migration models to keep in sync;
- two audit-chain conventions to reconcile;
- two concurrency stories (SQLite WAL + single writer in `.striatum/` vs.
  MVCC in `striatumd.*`);
- a recovery surface where the daemon can be healthy but a repo's SQLite
  file is corrupt, missing, or held by a stale process — failure modes
  that no longer exist if there is one substrate.

This design does **not** alter D083. The trust boundary remains
"one OS user per machine, daemon socket owner-only, capability tokens
scoped per repository, not per OS user." "Multi-tenancy" here means
schema-level per-repo row isolation (every repo-local table carries
`repo_id`); it is **not** user-level auth.

Out of scope: cross-machine semantics, hosted mode, per-user auth,
OS-keyring integration. Out of scope for V1 also: bundled / Dockerized
Postgres distribution (deferred per RFC 0033 §8).

## Trust Boundary Restatement (Why This Is The Hard Part)

RFC 0042 is not primarily a schema migration. It is a **trust boundary
shift**. Today's repo-local model has two writers with overlapping
authority:

1. The CLI process invoked by the operator, which opens
   `.striatum/state.sqlite3` directly and mutates rows via SQL.
2. The daemon process, which owns its own Postgres registry but does
   not gate repo-local state mutations.

This was fine when (a) the daemon was optional and (b) repo-local state
was the source of truth. Neither holds anymore: RFC 0030/0031/0032
require a daemon for supervised lanes, cross-repo runs, MCP capability
gating, and apply receipts. The repo-local SQLite is the lone surface
where the CLI can still bypass the daemon's mediation, capability check,
and audit-chain append.

RFC 0042 closes that surface. After V1:

- **Daemon is mandatory** for every repo-local mutation. `striatum run`,
  `striatum ack`, `striatum heartbeat`, `striatum publish-artifact`,
  `striatum verdict`, `striatum complete`, `striatum recovery *`, etc.
  all route through `daemon_rpc` over the Unix socket.
- **CLI without daemon refuses cleanly** with a named remediation hint.
  No silent fallback to SQLite-direct mode. No "best-effort" writes.
  Exit code 11 (new): `daemon_unavailable`.
- **`.striatum/` becomes operational scratch only**: pidfiles, FIFOs for
  supervised-lane stdin, supervisor stdout/stderr log files, work-packet
  drop directories for the wrapper scripts. No authoritative rows, no
  state machine state, no audit anchors.

The threat-model framing: the operator (single trusted OS user, per
D083) is not the adversary. The adversary surface that grows from a
two-writer model is *AI-agent code paths inside the CLI* and *operator
mistakes*. An AI lane that has shell access can open `.striatum/state.sqlite3`
directly today and mutate state without the daemon noticing. After V1 it
cannot — there is nothing authoritative to mutate, and the daemon owns
the only writeable substrate for run-state via capability-gated RPC.

This is the same containment direction RFC 0031 (sealed apply) and RFC
0032 (MCP capability tokens) took. RFC 0042 finishes it for the last
unowned surface.

## Per-Repo-ID Keying

### `repo_id` definition

A repo's identity is the same value already in
`striatumd.repositories.repository_id` from RFC 0033 baseline migration.
It is daemon-issued at first registration of a repo (currently via
`striatum daemon repo add` or implicit auto-registration on first
daemon-mediated CLI), opaque to operators, and stable across:

- rename / move of the repo working directory;
- recreation of `.striatum/` (e.g. operator `rm -rf .striatum/`);
- restart of the daemon;
- migration between daemon-binary versions.

`repository_id` is **not** the repo root path, the git remote URL, or
any operator-visible string. It is daemon-internal. The daemon's
`repositories.repo_identity` column (already present) is the operator-
facing canonical identity (currently the repo root absolute path
canonicalized; future RFCs may make it git-remote-derived). The two are
1:1 within a daemon instance; we expose `repo_id` only in audit rows
and structured CLI output, never as a parameter the operator types.

### Composite keys

Every repo-local table gains `repo_id text NOT NULL REFERENCES
striatumd.repositories(repository_id)` as its **leftmost key column**.

Concrete table list (full inventory of what `.striatum/state.sqlite3`
holds today, derived from `src/striatum/schema.py` plus migrations
v2–v10):

| Old SQLite table (PK) | New PG table | New composite key |
|---|---|---|
| `workflow_snapshots` (workflow_snapshot_id) | `striatumd.workflow_snapshots` | `(repo_id, workflow_snapshot_id)` |
| `runs` (run_id) | `striatumd.runs` | `(repo_id, run_id)` |
| `sessions` (session_id) | `striatumd.sessions` | `(repo_id, session_id)` |
| `jobs` (job_id) | `striatumd.jobs` | `(repo_id, job_id)` |
| `job_dependencies` (job_id, depends_on_job_id) | `striatumd.job_dependencies` | `(repo_id, job_id, depends_on_job_id)` |
| `queue_messages` (message_id) | `striatumd.queue_messages` | `(repo_id, message_id)` |
| `leases` (lease_id) | `striatumd.leases` | `(repo_id, lease_id)` |
| `work_packets` (packet_id) | `striatumd.work_packets` | `(repo_id, packet_id)` |
| `artifacts` (artifact_id) | `striatumd.artifacts_metadata` | `(repo_id, artifact_id)` |
| `verdicts` (verdict_id) | `striatumd.verdicts` | `(repo_id, verdict_id)` |
| `blockers` (blocker_id) | `striatumd.blockers` | `(repo_id, blocker_id)` |
| `command_requests` (request_id) | `striatumd.command_requests` | `(repo_id, request_id)` |
| `process_executions` (execution_id) | `striatumd.process_executions` | `(repo_id, execution_id)` |
| `events` (event_id) | `striatumd.events` | `(repo_id, event_id)` |
| `job_worktrees` (worktree_id) | `striatumd.job_worktrees` | `(repo_id, worktree_id)` |
| `process_supervisors` (pointer_id) | `striatumd.process_supervisor_pointers` | `(repo_id, pointer_id)` |

`schema_meta` (the SQLite version pragma) does **not** migrate; the PG
substrate already has `striatumd.schema_migrations`. The
`process_supervisor_pointers` daemon-side table from RFC 0031 stays
keyed as it is — that one was already daemon-owned, not repo-local.

**Decisions in this design:**

- All `repo_id`-keyed foreign keys are `ON DELETE RESTRICT`. A repo
  removal goes through the explicit `striatum daemon repo remove` flow,
  which either refuses (if active rows exist) or runs the documented
  purge sequence under audit. We do not cascade-delete repo state silently.
- Cross-table FKs *within* a repo (e.g. `jobs.run_id → runs.run_id`)
  become composite (`(repo_id, run_id)`) so cross-repo references are
  structurally impossible.
- Existing UNIQUE constraints / partial indexes that were per-`run_id`
  (e.g. unique active worktree per `job_id`) become per-`(repo_id, job_id)`.
- Indexes leading with `repo_id` cover the common per-repo dashboard
  query shape (`WHERE repo_id = $1 AND state = ...`) without forcing
  full-substrate scans.

### Why composite keys instead of a `repo_id` column on a global-PK table

Two reasons:

1. **Containment of cross-repo bugs.** If `(repo_id, run_id)` is the PK,
   a code path that forgets to filter by `repo_id` produces a SQL error
   (FK violation, ambiguous match) rather than silently returning a
   different repo's row. With a single global PK + a separate `repo_id`
   column, a missing `WHERE repo_id = ?` returns the wrong row.
2. **Audit trail cleanliness.** Every audit row already carries
   `repository_id` (RFC 0033 baseline). The composite PK keeps the
   substrate's authoritative identity consistent across the audit log
   and the operational tables, which simplifies the "show me everything
   for repo X" query operators run during incident response.

## CLI Behavior When No Daemon Is Running

This is the most operator-facing change. Current CLI behavior is:

```
$ striatum ack --session-id sess_X --message-id msg_Y --lease-id lease_Z
# opens .striatum/state.sqlite3, runs the SQL, exits
```

After V1, the CLI must talk to a daemon. The decision tree on every
CLI invocation that mutates run state:

```
1. Resolve daemon socket path from STRIATUM_DAEMON_SOCKET env, or
   ~/.config/striatum/daemon.toml, or default ~/.striatum/daemon.sock.
2. Attempt connect + envelope-v1 handshake.
3. If connected: route the verb as an RPC call (audit append happens
   on the daemon side, capability gating happens on the daemon side).
4. If not connected:
   a. Distinguish "no daemon running" from "wrong version" from
      "permission denied".
   b. Emit a structured error with a named remediation hint.
   c. Exit non-zero with the right exit code class.
   d. Do NOT touch any SQLite or any repo-local file.
```

### Refuse-cleanly contract

The CLI prints a structured error block, both human-readable and
JSON-emittable under `--json`:

```
error: daemon_unavailable
reason: no Striatum daemon is running on /home/halbritt/.striatum/daemon.sock
remediation:
  start the daemon:  striatum daemon start
  verify status:     striatum daemon status
  check the socket:  STRIATUM_DAEMON_SOCKET=<path> (current: <path>)
docs: docs/HOW_TO_HUMAN.md § "Daemon required for state mutations"
exit code: 11
```

Exit code allocation (extends the existing `striatum.errors` table):

| Code | Class | Meaning |
|---|---|---|
| 11 | `daemon_unavailable` | Cannot connect to daemon socket (process not running, socket path wrong, perms wrong). |
| 12 | `daemon_version_skew` | Connected, handshake rejected by version-skew rules. |
| 13 | `daemon_unauthorized` | Connected, capability check failed for the requested verb. |
| 14 | `repo_not_registered` | Daemon healthy but this repo is not in `striatumd.repositories` yet — points operator at `striatum daemon repo add`. |
| 15 | `repo_unmigrated` | Daemon healthy, repo registered, but its `.striatum/state.sqlite3` has not been cut over yet — points operator at `striatum daemon migrate-repo-local`. |

**No silent fallback.** The CLI never decides on the operator's behalf
to "just write to the SQLite file this once." That code path is deleted
in the V1 implementer's commit.

### Read verbs vs. write verbs

A subtle but important distinction. After V1:

- **Mutation verbs** (`ack`, `complete`, `verdict`, `block`,
  `heartbeat`, `publish-artifact`, `recovery *`, `run start`, `run *`,
  `worktree *`) require a daemon. No fallback. Exit 11 if unavailable.
- **Read-only verbs that the operator runs at the terminal**
  (`status`, `dashboard --once`, `audit show`, `why`, `doctor`) also
  require a daemon — they read from the canonical PG substrate, which
  is the daemon's. Same exit 11 behavior.
- **The exception** is `striatum daemon start` and `striatum daemon
  doctor` themselves, plus the `striatum init` shape that *registers*
  a repo. These can run pre-connection — they are the recovery surface
  for "I have no daemon."

We considered keeping a read-only SQLite reader for `dashboard --once`
in CI scripts but rejected it: post-V1 the SQLite file is empty
operational scratch; there is nothing to read.

### Daemon-down recovery story

When a developer kills the daemon (Ctrl-C their `daemon start`
foreground, or the daemon crashed), the recovery story is:

1. CLI tells the operator: "daemon_unavailable; start it with
   `striatum daemon start`."
2. Operator restarts the daemon.
3. Daemon verifies all `state = 'active'` rows in
   `striatumd.repositories` — every repo it expects to manage. If
   a repo's `repo_root` has a `.striatum/state.sqlite3` from a previous
   install that hasn't been migrated yet, daemon marks the repo
   `state = 'needs_migration'` and refuses RPC mutations for that
   `repo_id` until the operator runs `daemon migrate-repo-local`.
4. Existing supervised processes (which keep running across daemon
   restarts, per RFC 0031) reconnect to the daemon via supervisor
   pointer rows; their leases are restored.
5. Operator re-runs the previously-failing CLI verb; it succeeds.

No data is lost in a daemon crash because nothing the daemon was
mutating was ever on the SQLite side post-V1.

### The `--no-daemon` flag

Today's `--no-daemon` flag (which forces SQLite-direct mode for a
handful of read-only verbs) is **removed** in V1. The implementer's
deletion list should grep for `--no-daemon` and remove the flag, its
parser entry, and all branches keyed on it. The verb `striatum daemon
migrate-repo-local` is the *only* CLI surface that touches SQLite, and
even it does so via the daemon process — the CLI just sends the RPC,
the daemon binary opens and reads the SQLite file (per the rationale
in §"Migration Verb").

## RFC 0039 Scope Expansion: Go Daemon As Gateway For All Repo-Local Ops From Day 1

The original RFC 0039 scope (Implementation Plan §, Steps 3–6) said:
"The Go daemon eventually handles cross-repo runs, supervised
processes, apply receipts, MCP — basically the same set of verbs the
Python daemon does." It did **not** require the Go daemon to handle
**all** repo-local state from day 1, because at RFC 0039 drafting time
the repo-local SQLite was still the operator's writeable substrate.

RFC 0042 changes that premise. With repo-local state living in the
daemon's PG, the Go daemon needs to be the single gateway for it from
Step 3 onward — there is no longer a "I write to SQLite if the daemon's
not ready yet" path.

Concrete RFC 0039 scope revisions RFC 0042 must encode:

### Revision 1: Step 3 read-only verb table expansion

RFC 0039 Step 3 ("Read-only daemon (CLI integration)") was originally
specced as serving `status`, `dashboard`, `audit show` against the
daemon's *own* PG schema. After RFC 0042, Step 3 must also serve
read-only repo-local verbs (`run show`, `job show`, `session list`,
`why <job-id>`, `dashboard --run-id ...`). These read from the same
PG schema, just from the new `striatumd.runs`/`striatumd.jobs`/etc.
tables instead of from a repo's SQLite. No protocol change — the
envelope-v1 method table grows to include the read methods.

### Revision 2: Step 4 mutation verb table expansion

Step 4 was specced for cross-repo prepare/start/cancel + MCP
`tools/call` + capability token lifecycle. After RFC 0042, Step 4 must
also include the full repo-local mutation verb table: `run.start`,
`job.ack`, `job.heartbeat`, `job.publish_artifact`, `job.verdict`,
`job.complete`, `job.block`, `recovery.*`, `worktree.*`. These are the
verbs the Python CLI translates from operator commands into envelope-v1
RPC calls.

Each verb requires:

- A capability table entry. The capability matrix is:

  | Verb | Capability |
  |---|---|
  | `run.*` | `write` |
  | `job.ack` | `claim` |
  | `job.heartbeat` | `claim` (must match the lease holder) |
  | `job.publish_artifact` | `write` |
  | `job.verdict` | `review` |
  | `job.complete` | `claim` (must match the lease holder) |
  | `job.block` | `claim` |
  | `recovery.*` | `recovery` |
  | `worktree.*` | `apply` (touches working tree) |

- A `repo_id`-scoped audit append, the same audit hash chain we
  already maintain.
- A capability check that the requesting client's `client_capabilities`
  row covers the verb's required capability **for this `repo_id`**.

### Revision 3: Step 5 supervised processes — scratch dir lives on

Step 5 (supervised processes, PTY, FIFO packet delivery) is **not**
materially affected by RFC 0042 because `.striatum/` is still the right
location for:

- supervisor stdout/stderr log files (FIFO redirects, rotation),
- the FIFO pipe path for packet delivery to supervised wrappers,
- the daemon's pidfile and socket file (well-known locations under
  `.striatum/`).

What changes is that the *supervisor pointer rows* and *liveness
checkpoints* live in `striatumd.process_supervisor_pointers` (already
RFC 0031), and the *queue messages* / *leases* / *jobs* that the
wrapper coordinates against move from `.striatum/state.sqlite3` to
`striatumd.{queue_messages,leases,jobs}`. The wrapper itself talks to
the daemon over the same socket using a supervisor-scoped capability
token, not by opening SQLite.

### Revision 4: Step 6 distribution — no SQLite migration tool ships separately

Step 6 currently says "ship the four-platform Go binary." RFC 0042
adds: the Go binary must contain the SQLite-read code for the migration
verb (CGO-free SQLite read library; `modernc/sqlite` or `mattn/go-sqlite3`
in CGO mode — implementer choice). Migration is a one-shot per-repo
operation but it must be in the binary so the operator never has to
keep an old Python install around.

### Revision 5: Acceptance criterion delta

RFC 0039's acceptance criteria need one new line under §Acceptance Criteria:

> "Go daemon handles all repo-local state operations via the
> `striatumd.{runs,jobs,sessions,queue_messages,leases,events,
> blockers,verdicts,artifacts_metadata,process_executions,
> job_worktrees,...}` tables from the same PG substrate. No repo-local
> SQLite file is opened by the daemon process at any time except the
> one-shot per-repo migration verb."

This is the cleanest scope expansion because it does not split the
daemon binary into a "daemon-local" and "daemon-repo-local" half — they
are the same binary, the same PG connection pool, the same audit
chain. The operator does not see two daemons.

## New Schemas

### `repo_id`-aware versions of the existing tables

I will not reproduce every column here (the RFC body and the migration
SQL will). The shape rule is:

```sql
CREATE TABLE striatumd.runs (
  repo_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  run_id text NOT NULL,
  workflow_snapshot_id text NOT NULL,
  repo_root text NOT NULL,  -- redundant w/ repositories.repo_root; kept for debug, FK-checked
  state text NOT NULL CHECK (...),
  -- ... same columns as SQLite v1 ...
  PRIMARY KEY (repo_id, run_id),
  FOREIGN KEY (repo_id, workflow_snapshot_id)
    REFERENCES striatumd.workflow_snapshots(repo_id, workflow_snapshot_id)
);

CREATE INDEX runs_repo_state ON striatumd.runs (repo_id, state)
  WHERE state IN ('ready', 'running', 'blocked');
```

Each table follows the same shape: composite PK leading with `repo_id`,
all intra-table FKs use the composite. Partial indexes on hot-path
states use `(repo_id, state)` ordering.

### New table: `repo_state_migrations`

This tracks which repos have completed the SQLite→PG cutover.

```sql
CREATE TABLE striatumd.repo_state_migrations (
  repo_id text PRIMARY KEY REFERENCES striatumd.repositories(repository_id),
  source_sqlite_path text NOT NULL,
  source_sqlite_sha256 text NOT NULL,
  source_row_count_json jsonb NOT NULL,  -- per-table counts
  imported_row_count_json jsonb NOT NULL,  -- per-table counts on PG side
  source_audit_chain_final_hash text,  -- if the SQLite had any audit-shaped rows
  migration_started_at timestamptz NOT NULL,
  migration_completed_at timestamptz NOT NULL,
  migration_daemon_version text NOT NULL,
  rollback_kept_readonly boolean NOT NULL DEFAULT false,
  cutover_marker_sha256 text NOT NULL  -- hash written into the SQLite file as a tombstone
);
```

This table is the operator's "did this repo migrate? when? from what?"
audit record. Removing a repo via `daemon repo remove --purge` archives
this row to `striatumd.repo_state_migrations_history` (out of scope to
detail here).

### Audit-row schema extension

RFC 0033 baseline `striatumd.audit_log` already has `repository_id`. We
keep that column. We add **one** new boolean column to
`striatumd.audit_log`:

```sql
ALTER TABLE striatumd.audit_log
  ADD COLUMN repo_local_state_touched boolean NOT NULL DEFAULT false;
```

A new migration `0005_repo_local_state.sql` adds this and the
`repo_state_migrations` table. The audit-chain hash payload format
version bumps from `hash_format_version = 1` to `2`: the new version
includes the bool in its hash payload. The migration replays existing
rows with `hash_format_version = 1` semantics (no change to historical
hashes), and new rows go in with `hash_format_version = 2`. The audit
hash chain stays continuous across the format-version bump (the
previous-hash linkage in the new row uses the last hash from the v1
era unchanged).

This is the same versioning shape RFC 0033 specified (`hash_format_version`
on every row, schema-version recorded). We are extending it, not
breaking it.

## Migration Verb: `striatum daemon migrate-repo-local`

### Signature

```
striatum daemon migrate-repo-local
  --from sqlite                      [required, only mode in V1]
  --to pg                            [required, only mode in V1]
  --repo <path>                      [required, absolute path to repo root]
  [--keep-sqlite-readonly]           [default off]
  [--dry-run]                        [default off]
  [--force-replace-pg]               [hidden; required if repo already migrated]
  [--json]
```

### Semantics

The CLI sends a single RPC `repo.migrate_local_state` to the daemon
(capability `admin`). The daemon does the work; the CLI streams a
progress log back over the RPC socket and emits structured output.

**Daemon-side flow:**

1. **Preconditions.**
   - Verify the operator is the daemon-owning OS user (D083 boundary).
   - Verify the repo path is canonicalized and matches a registered
     `striatumd.repositories.repo_root`, OR auto-register it (with a
     fresh `repository_id`) if `--register-if-missing` is set
     (default-true; the operator should never need to think about
     this).
   - Verify the destination tables for this `repo_id` are empty.
     Refuse if rows exist for this `repo_id` unless `--force-replace-pg`
     is set; refuse even with `--force-replace-pg` if active leases
     exist (running supervised processes).
   - Verify the source SQLite file exists, is readable, and
     `PRAGMA integrity_check` passes.
   - Compute `source_sqlite_sha256` for the migration audit row.

2. **Schema preflight.**
   - Read the SQLite's `PRAGMA user_version`. Map to the migration set
     in `src/striatum/migrations.py`. Refuse if newer than the daemon
     binary knows about (mirrors the existing exit-code-9 SchemaVersionError
     pattern).

3. **Audit-chain handoff.**
   - The SQLite file has no daemon-shape audit log today (audit is
     daemon-side and already in PG). But the run state has
     `state_transitions` / `events` rows that constitute the
     run's own causal chain. The migration verb does **not** re-hash
     these; it copies them verbatim with original timestamps. The
     `repo_state_migrations.source_audit_chain_final_hash` is computed
     as `sha256(canonical_csv_of_events_rows)` and recorded for
     post-migration verification (the operator can re-run the hash
     against the PG side and confirm equality).

4. **Row copy, table by table, in FK order.**

   ```
   1. workflow_snapshots
   2. runs
   3. sessions
   4. jobs + job_dependencies
   5. work_packets
   6. queue_messages
   7. leases
   8. artifacts
   9. verdicts
   10. blockers
   11. command_requests
   12. process_executions
   13. events
   14. job_worktrees
   ```

   Each row gets `repo_id = <resolved_repo_id>` prepended. Each row is
   inserted under a single transaction per table (so a mid-table
   failure rolls back that table without leaving partial rows; the next
   re-run of the verb retries from the failed table).

5. **Audit append.**
   - Append one daemon-audit row per migrated table to
     `striatumd.audit_log` with `method = 'repo.migrate_local_state'`,
     `decision = 'allow'`, `params_sha256 = sha256(<table_name>)`, and
     row count in metadata.
   - Then append one summary row with the per-table totals.

6. **Cutover marker.**
   - Write `repo_state_migrations` row.
   - Write a sentinel into the SQLite file: a `cutover_marker` row in
     `schema_meta` with `key = 'cutover_completed_at'`, `value =
     <ISO8601>`, plus `cutover_marker_sha256 = <hash of cutover row contents>`.
   - If `--keep-sqlite-readonly`, additionally `chmod 0400` the file
     and rename to `.striatum/state.sqlite3.pre-cutover.<timestamp>`.
     If not, **delete** the file. Either way, future daemon starts
     check for the rename pattern or the sentinel row and refuse to
     start direct-SQLite mode.

7. **Repository state update.**
   - Mark `striatumd.repositories.state = 'active'`, set
     `last_seen_at = now()`, set
     `settings_json->>'repo_local_state_storage' = 'pg'`.

### Dry-run

`--dry-run` reads the SQLite, computes per-table counts, computes
the source hash, validates the schema is migratable, and emits the JSON
report without writing to PG. No audit row appended. No marker written.

### Idempotency

Re-running the verb on an already-migrated repo refuses with exit code
14 (`repo_already_migrated`) and the remediation hint to pass
`--force-replace-pg` (which requires no active leases and is documented
as "wipe the PG side for this repo and re-import"). The history row
is preserved.

### First-daemon-start auto-migration: deferred

The task prompt hints at "migrated per-repo on first daemon start with
the new flow." We **defer** auto-migration to a separate operator step
and document this clearly. Reasoning:

- Auto-migration on daemon start makes the daemon-start failure mode
  ambiguous ("did the daemon fail to start, or did the migration fail?").
- Operators who upgrade across the V1 release boundary need an
  *explicit* step that they can read about in `docs/HOW_TO_HUMAN.md`
  and run when they are ready. A surprise migration on daemon start
  produces panic.
- The daemon on V1+ start does detect unmigrated repos in its
  `striatumd.repositories` table and marks them `state = 'needs_migration'`,
  refusing RPC mutations against them with exit code 15 and the
  remediation hint. This is the discoverability path; it is not the
  surprise path.

If reviewer feedback prefers auto-migration on daemon start, we have
the building blocks for that (the verb is daemon-side; the daemon
calling its own RPC entry point internally is straightforward). I
recommend deferring it to RFC 0042 V2 or a follow-up RFC.

## `.striatum/` After V1: Operational Scratch Only

The post-V1 contents of `.striatum/`:

- `daemon.sock` — Unix socket. **Daemon-owned.** Created and removed
  by `daemon start` / `daemon stop`.
- `daemon.pid` — Pidfile. Daemon-owned.
- `bin/` — Wrapper scripts installed by `striatum skills install`
  (`codex-supervised-wrapper.sh`, `claude-supervised-wrapper.sh`,
  `gemini-supervised-wrapper.sh`). These are stable across runs.
- `supervisor/<session-id>/` — Per-supervised-process scratch directory:
  - `stdin.fifo` — Named pipe for packet delivery.
  - `stdout.log` / `stderr.log` — Captured child output (rotation
    pending follow-up RFC).
  - `agent.pid` — Child agent process PID.
  - `lifecycle.state` — Plain-text current state ("starting",
    "running", "exiting"); not authoritative, just a debugging aid.
- `tmp/` — Scratch dir for the CLI to drop transient files (worktree
  scaffolding stage-2 outputs, etc.). Cleaned by `striatum doctor --clean`.
- **No** `state.sqlite3`.
- **No** marker files representing authoritative state.

`.gitignore` rules continue to ignore `.striatum/` (per current
convention). The post-cutover SQLite tombstone (if
`--keep-sqlite-readonly`) sits in `.striatum/` and is also gitignored.

The `daemon doctor` verb verifies:

- `daemon.sock` exists and is owner-only (0600).
- `daemon.pid` matches a running process.
- No `state.sqlite3` exists, or if it does, it has the cutover sentinel
  row.
- Wrapper scripts in `bin/` are executable and match the bundled
  version.

If `state.sqlite3` exists without the sentinel, daemon refuses to start
with exit code 9 (schema-version-conflict shape; remediation:
`striatum daemon migrate-repo-local --repo <path>`). This is the
upgrade-time discoverability gate.

## Audit-Chain Integrity Across Migration

Two distinct chains:

1. **Daemon audit log** (`striatumd.audit_log` from RFC 0033). Already
   exists. RFC 0042 adds rows during migration (one per migrated table
   plus the summary), each with a `previous_hash` linkage to the prior
   row, each with `hash_format_version = 2` (the bumped version),
   each with `repository_id = <the repo being migrated>`. **This chain
   is preserved.**

2. **Per-repo run causal chain** (the `events` table in repo-local
   SQLite). This is not a hash chain today, but it is causally ordered
   by `event_id` autoincrement + `created_at`. The migration verb
   copies rows in `event_id` order, preserving original timestamps. We
   add a `migration_provenance` JSONB field to migrated event rows
   (column added by `0005_repo_local_state.sql`) containing:

   ```json
   {
     "source_sqlite_sha256": "<hash>",
     "source_event_id": <int>,
     "migration_audit_id": <int>
   }
   ```

   This lets `daemon doctor` cross-link any imported event back to the
   audit row that imported it. After V1, new events from RPC verbs
   write `migration_provenance = NULL`.

3. **End-to-end integrity check** (operator-runnable):

   ```
   striatum daemon doctor --repo <path> --check-migration
   ```

   Computes the source-SQLite-side rollup hash from
   `repo_state_migrations.source_audit_chain_final_hash`, recomputes
   the equivalent hash on the PG side, and reports equality. If the
   `--keep-sqlite-readonly` tombstone is present, runs the same
   computation on the tombstone to give a third independent check.

## Failure Modes And Their Handling

| Failure | Detection | Operator-visible result | Recovery |
|---|---|---|---|
| Daemon not running | CLI socket connect EOF | Exit 11 with `daemon_unavailable` remediation | `striatum daemon start` |
| Daemon running, wrong version | Envelope-v1 handshake refuses | Exit 12 with version-skew message | upgrade CLI or daemon |
| Daemon running, PG unreachable | Daemon-side errors | RPC returns `pg_unavailable`; CLI exits 16 | check PG service |
| Daemon running, repo not registered | `repo.migrate_local_state` precondition fails | Exit 14 | `striatum daemon repo add` |
| Daemon running, repo unmigrated, operator runs `striatum ack` | Daemon refuses, returns `repo_unmigrated` | Exit 15 | `striatum daemon migrate-repo-local` |
| Migration mid-way crashes (process killed) | SQLite tombstone not yet written | Exit non-zero, partial PG rows present | Re-run with `--force-replace-pg` |
| Migration succeeds, operator manual-rolls-back | SQLite tombstone exists, operator deletes daemon's PG rows | Exit 17 `pg_repo_drift_detected` on next CLI call | full re-import via `--force-replace-pg`, or use `--keep-sqlite-readonly` tombstone for forensics |
| SQLite file is corrupt | `PRAGMA integrity_check` fails in preflight | Migration refuses; exit 18 | restore from backup / git-recover; no migration possible |
| Concurrent CLI writes during migration | RPC mutation refused while migration row in `repo_state_migrations` is in-progress | Exit 19 `migration_in_progress` | wait for migration to finish |

## Test Plan (V1 Acceptance)

Concrete acceptance tests the RFC body should require:

1. **Schema migration applies clean.** `daemon migrate` (the
   RFC 0033 verb that goes V1-daemon-SQLite-registry → V2-daemon-PG)
   running against a fresh PG database produces the
   `0005_repo_local_state.sql` schema with all the new tables
   present and the bool column on `audit_log`.

2. **Repo cutover round-trips a real workflow.** Build a SQLite
   `.striatum/state.sqlite3` representing a finished dogfood run
   (use the existing fixtures under `tests/data/`). Run
   `daemon migrate-repo-local --dry-run`; verify per-table counts
   match a hand-counted expectation. Run without `--dry-run`;
   verify rows present in PG with correct `repo_id` prefix; verify
   `striatum dashboard --run-id <id>` reads the run from PG.

3. **`--keep-sqlite-readonly` produces a readable tombstone.** After
   migration, the renamed SQLite file is 0400, contains the cutover
   sentinel row, and `daemon doctor --check-migration` succeeds.

4. **CLI refuses cleanly without daemon.** `pkill -f daemon`; run
   `striatum ack ...`. Assert exit code 11, structured error block,
   remediation hint mentions `daemon start`.

5. **CLI refuses cleanly for unmigrated repo.** Daemon running; repo
   added to `striatumd.repositories` but not migrated; run
   `striatum ack ...`. Assert exit code 15, remediation hint mentions
   `daemon migrate-repo-local`.

6. **Audit chain unbroken after migration.** Compute audit chain
   integrity per `daemon doctor` before and after migration; the
   chain extends with one row per migrated table plus one summary,
   each linking via `previous_hash` to the prior row.

7. **Migration verb is idempotent.** Re-running on a migrated repo
   refuses with exit 14. Re-running with `--force-replace-pg`
   replaces. The `repo_state_migrations` history preserves the prior
   row (move to history table).

8. **Concurrent supervisor with migration is refused.** Start a
   supervised process; while its lease is held, run the migration
   verb. Assert refusal with exit 19, message mentions the held lease.

9. **Cross-repo isolation.** Migrate two repos. Assert PG queries
   constrained by `WHERE repo_id = <other>` never return rows from
   the first. Drop the second repo's PG rows; assert the first repo's
   data unchanged.

10. **Go-daemon parity.** Run all of the above with
    `daemon_core="go"` via the RFC 0035 multi-repo harness. Per the
    RFC 0039 scope expansion specified here, the Go daemon must
    implement all repo-local verbs from day 1 of the rewrite — i.e.
    Step 4 of RFC 0039 Implementation Plan ships with the full
    mutation verb table.

## Open Questions For Synthesis

1. **Should we keep the `--keep-sqlite-readonly` tombstone as the
   default?** Arguments for: forensic rollback path. Against: extra
   file in `.striatum/`, possible operator confusion. Recommendation:
   keep it off-by-default for V1, document it as the recommended
   incident-response option. Reviewer / synthesis is welcome to flip.

2. **Auto-migrate on daemon start?** Deferred above. Recommend the
   synthesis stay with explicit migration in V1.

3. **`repo_id` schema visibility to operators.** Should
   `striatum daemon repo list` show `repo_id`? Recommendation: yes
   (helps the operator correlate audit rows with repo roots), but
   the operator never types it — only paths.

4. **Per-table vs. per-repo migration transactionality.** Today
   each table is its own transaction. Alternative: single big
   serializable transaction covering all 14 tables. Pro: stronger
   atomicity. Con: lock duration. Recommendation: per-table for V1
   (matches the existing pattern), revisit if operators report
   in-the-middle failures.

5. **Should we keep the `striatum.db` (SQLite-direct CLI library)
   module at all post-V1?** It is currently the public SQLite-side
   API. Recommendation: keep it as a daemon-internal-only module
   used by the migration verb. Public CLI does not import it. The
   implementer's diff for V1 removes the SQLite-direct imports from
   every CLI verb path.

6. **What about the dashboard's `--once` script use case?** Today
   it reads SQLite directly so CI / shell scripts can poll without
   needing a daemon. Post-V1 it requires the daemon. Recommendation:
   document the change in `docs/HOW_TO_HUMAN.md`, ship a parallel
   `striatum dashboard --once --connect-timeout-seconds N` to make
   CI scripts robust to daemon-still-starting transient.

## Summary For The Synthesizer

If you accept this design:

- **One substrate**: daemon-owned PG; SQLite goes away as a writeable
  surface.
- **One trust boundary**: daemon-mandatory, refuse-cleanly CLI, no
  silent fallback; exit codes 11/14/15 with named remediation.
- **One identity model**: `repo_id` as composite-key prefix on every
  formerly-repo-local table; FKs to `striatumd.repositories`.
- **One migration verb**: `daemon migrate-repo-local --repo <path>`,
  audit-chain-preserving, dry-run-capable, with tombstone for rollback.
- **One RFC 0039 expansion**: Go daemon Step 3 + Step 4 grow to cover
  all repo-local read + mutation verbs from day 1; Step 6 binary
  contains the migration tool.
- **One supersession**: D006 + D007 + D028's SQLite-direct half. D083
  unchanged. Single-user trust boundary stays.
- **One deferred**: auto-migrate on daemon start. Operator runs the
  verb explicitly in V1; daemon refuses with exit 15 + remediation
  when the operator hits an unmigrated repo.

This is the conservative, threat-model-defensible shape: it removes the
last unmediated mutation surface from the system, makes the failure
modes diagnosable (named exit codes, named remediations), and aligns
the Go daemon's scope to "single substrate gateway from day 1" without
forcing a parallel implementation path.
