# RFC 0028 V1 Daemon Design

author: operator

Status: design input
Date: 2026-05-11
Target: RFC 0028 V1 acceptance-criteria slice

## Scope

This design covers the first accepted implementation slice for RFC 0028: an
optional local `striatumd` process that can register multiple target
repositories, serve read-only cross-repository status surfaces, expose
daemon-backed read-only CLI and MCP calls, enforce capability-gated mutation
defaults, run resident recovery sweeps, and become the owner of supervised
agent processes.

The slice deliberately does not implement cross-repository workflows,
sealed-mode apply authority, signing keys, remote serving, or a Go rewrite.
It also does not change Striatum's core product boundary: live run state
remains structured runner state, repository artifacts remain durable
provenance, and terminal output remains non-authoritative.

## Domain Model

RFC 0028 should add these terms to the ubiquitous language after acceptance:

| Term | Definition |
|------|------------|
| Striatum daemon | A local long-running deterministic control-plane process that coordinates registered repositories, client authorization, global read views, resident recovery, event streaming, and supervised process ownership. |
| repository tenant | A registered target repository managed by the daemon. Each repository tenant retains its own `.striatum/state.sqlite3`, workflows, runs, artifacts, policies, and provenance mode. |
| operator tenant | A local operator trust zone, initially the current OS user, that can hold client capabilities over one or more repository tenants. V1 does not imply hosted accounts. |
| client capability | A scoped value granting a connected CLI, MCP, web, or plugin client permission for a bounded command family, repository set, and expiry window. |
| daemon registry | The daemon-owned local SQLite store for registered repositories, client records, capability grants, audit entries, scheduler cursors, daemon-owned supervisors, and registry migrations. |
| global dashboard | A daemon-backed read model that summarizes runs, blockers, stale leases, claimable jobs, sessions, and supervisors across all registered repository tenants. |

The daemon is a resident implementation of the existing Striatum bounded
context. It does not become an AI coordinator, does not decide review truth,
does not synthesize artifacts, and does not parse stdout or stderr as
workflow state.

## Storage Recommendation

V1 should implement RFC 0028 Option C: a central daemon registry plus
repo-local run stores.

The daemon registry should live under the user's local data directory, for
example `${XDG_STATE_HOME:-~/.local/state}/striatum/striatumd.sqlite3`, with
the socket under `${XDG_RUNTIME_DIR:-~/.local/state/striatum/run}` when no
platform-specific runtime directory is available. Exact paths should be
centralized in a new path helper so tests can override them through an
environment variable.

This choice is the right V1 tradeoff:

- It preserves the existing repo-local contract: `.striatum/state.sqlite3`
  remains authoritative for runs, jobs, leases, artifacts, verdicts, and
  events in a target repository.
- It lets existing initialized repositories register without data rewrite.
  Registration records the canonical repository path, detected state DB path,
  schema version, and stable repository id; it does not import run rows into
  the daemon registry.
- It gives the daemon one durable place for global concerns that do not
  belong to a single repository: clients, capabilities, audit log,
  scheduler cursors, daemon lifecycle metadata, and daemon-owned supervisor
  ownership.
- It avoids a premature full migration to central state while RFC 0026 and
  RFC 0027 provenance semantics are still settling.

Option A alone is too weak because capability grants, connected clients,
audit entries, and resident scheduling need daemon-owned state. Option B
would weaken the repo-local cleanup and audit story before the daemon has
proven itself. Option D should remain a future RFC after real daemon-mode
load exposes SQLite pain.

## Registry Schema

Create a daemon-specific migration module rather than mixing registry
migrations with `striatum.migrations`, which owns repo-local state. The
registry should use `PRAGMA user_version`, WAL, foreign keys, and the same
newer-than-supported refusal pattern as repo-local state.

Initial registry tables:

```sql
CREATE TABLE daemon_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE repositories (
  repository_id TEXT PRIMARY KEY,
  repo_root TEXT NOT NULL UNIQUE,
  state_db_path TEXT NOT NULL,
  display_name TEXT NOT NULL,
  registered_at TEXT NOT NULL,
  last_seen_at TEXT,
  last_schema_version INTEGER,
  state TEXT NOT NULL CHECK (state IN ('active','missing','disabled')),
  settings_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE clients (
  client_id TEXT PRIMARY KEY,
  client_kind TEXT NOT NULL CHECK (client_kind IN ('cli','mcp','web','plugin','daemon')),
  display_name TEXT NOT NULL,
  token_hash TEXT,
  created_at TEXT NOT NULL,
  expires_at TEXT,
  revoked_at TEXT
);

CREATE TABLE client_capabilities (
  capability_id TEXT PRIMARY KEY,
  client_id TEXT NOT NULL REFERENCES clients(client_id),
  repository_id TEXT REFERENCES repositories(repository_id),
  capability TEXT NOT NULL,
  granted_at TEXT NOT NULL,
  expires_at TEXT,
  revoked_at TEXT
);

CREATE TABLE audit_log (
  audit_id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp TEXT NOT NULL,
  client_id TEXT,
  repository_id TEXT,
  command TEXT NOT NULL,
  authorization_result TEXT NOT NULL CHECK (
    authorization_result IN ('allowed','denied','read_only_allowed','error')
  ),
  transport TEXT NOT NULL,
  request_id TEXT,
  exit_code INTEGER,
  payload_sha256 TEXT
);

CREATE TABLE scheduler_cursors (
  repository_id TEXT NOT NULL REFERENCES repositories(repository_id),
  run_id TEXT NOT NULL,
  cursor_kind TEXT NOT NULL,
  last_sweep_at TEXT,
  last_result_json TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY (repository_id, run_id, cursor_kind)
);

CREATE TABLE daemon_supervisors (
  supervisor_id TEXT PRIMARY KEY,
  repository_id TEXT NOT NULL REFERENCES repositories(repository_id),
  run_id TEXT NOT NULL,
  session_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('starting','attached','detached','lost','stopped')),
  pid INTEGER,
  pid_start_time TEXT,
  transport TEXT NOT NULL CHECK (transport IN ('pipe','pty')),
  scratch_path TEXT NOT NULL,
  stdin_path TEXT,
  started_at TEXT NOT NULL,
  heartbeat_at TEXT,
  ended_at TEXT,
  stop_reason TEXT
);
```

`audit_log` is intentionally metadata-only. It records who called what, where,
whether authorization allowed it, and a request payload hash for idempotency
or debugging. It must not store prompts, terminal output, artifact contents,
MCP tool result prose, or response bodies.

## Repository Registration

`striatum repo add <path>` should resolve the target repository, verify that
`.striatum/state.sqlite3` exists or that `--init` was explicitly requested,
connect through the existing `striatum.db.connect()` path to apply
repo-local migrations, and insert or update one `repositories` row.

Registration must not rewrite repository state. For an existing repository,
the daemon stores:

- a generated `repository_id`;
- canonical `repo_root`;
- `state_db_path`;
- display name, defaulting to the directory name;
- detected repo-local `PRAGMA user_version`;
- state `active`.

If the repo-local schema is newer than the installed daemon supports,
registration fails with the same class of version-skew error used by direct
CLI mode. If a registered path later disappears, daemon reads mark the
repository `missing` and include a doctor problem instead of deleting the
row.

## Daemon Lifecycle

The CLI surface should start small:

```text
striatum daemon start [--foreground] [--socket <path>] [--http <host:port>]
striatum daemon status
striatum daemon stop
striatum daemon reload
striatum repo add <path>
striatum repo list
striatum repo remove <repository-id>
```

`start` creates the registry if needed, acquires a daemon lock, opens the
Unix socket with owner-only permissions, starts scheduler loops, and records
`daemon_meta.started_at`, `version`, `pid`, and protocol version. A second
start against a live daemon reports the existing process. A stale lock or
dead pid is recovered after verifying that no socket peer responds.

`stop` sends a structured shutdown request over the socket. The daemon stops
accepting new requests, drains in-flight requests for a short bounded window,
marks daemon-owned supervisors `detached` unless explicitly configured to
terminate them, closes sockets, and exits. It should not kill agent
processes by default in V1 because current supervision semantics leave
operator inspection in charge when process state is uncertain.

`reload` rereads registry rows, client capabilities, and scheduler settings
without restarting active request handlers. It must not reload workflow
snapshots from disk for already prepared runs; existing repo-local snapshot
semantics remain authoritative.

Crash recovery on startup:

- recover stale daemon lock files after pid validation;
- scan `daemon_supervisors` and matching repo-local `process_supervisors`
  rows for attached pids;
- verify each pid and `pid_start_time`;
- mark missing processes `lost`;
- re-open PTY/pipe handles only when the underlying mechanism supports it;
- resume scheduler cursors from registry state;
- leave repo-write stale leases under D036 manual-inspection policy.

Version skew must be explicit. Every daemon response includes daemon version,
protocol version, and supported repo-local schema range. A newer CLI may fall
back to direct mode for read-only commands only when the user has not forced
daemon mode. A CLI older than the daemon should be refused for mutating calls
if the protocol major version differs. Repo-local databases whose
`user_version` is higher than either side supports remain refused.

## Transports And Authorization

Unix-domain socket is the default transport. Local file permissions are the
first boundary: create the socket directory `0700` and the socket with
owner-only access. CLI clients use this path by default.

Loopback HTTP is optional for the web UI and simple integrations. It must
reuse the existing localhost refusal rule from `striatum serve`, keep
constant-time bearer-token comparison, and treat non-loopback serving as out
of scope for this RFC.

MCP should support two modes:

- stdio bridge: a small process speaks MCP framing to the client and forwards
  requests to the daemon socket;
- socket-native MCP: future local clients connect directly when useful.

An optional event stream can be SSE over loopback HTTP or a socket
subscription over the Unix socket. V1 only needs read events for dashboards;
it should stream event metadata and state summaries, not transcripts.

All transports map to one authorization function:

```text
authorize(client_id, repository_id | all, command, command_kind) -> allowed | denied
```

Command kinds should start with:

- `global_read`;
- `repo_read`;
- `repo_mutation`;
- `session_mutation`;
- `supervisor_mutation`;
- `recovery_mutation`;
- `admin`.

Read-only capabilities can be granted by default to local CLI clients over
the owner-only Unix socket. MCP and HTTP clients default to read-only. Any
mutation through MCP, HTTP, or plugin clients requires an explicit
capability row. New mutating commands must default to denied, mirroring the
current `SERVICE_READ_TOP_COMMANDS` whitelist.

## CLI Client Mode

The first CLI verbs to become daemon clients should be read-only:

- `status`;
- `doctor`;
- `dashboard --once`;
- `dashboard --all`;
- `list` views where they are read-only;
- `supervise status`;
- `supervise list`;
- `recovery stale-leases` in inspection mode.

These are low-risk because they already gather state without changing
workflow truth. `dashboard --all` should require daemon mode because direct
mode has no registry to enumerate repositories.

The first daemon-backed mutations should be daemon administration and
registration:

- `daemon start/status/stop/reload`;
- `repo add/list/remove`;
- `recovery sweep --all` or daemon-internal scheduler invocation.

Existing workflow mutations should stay direct first:

- `init`;
- `workflow validate/plan/graph`;
- `run prepare/start`;
- `register-session`;
- `claim-next`;
- `ack`;
- `heartbeat`;
- `release`;
- `send`;
- `block`;
- `publish-artifact`;
- `submit-review`;
- `complete`;
- `verdict`;
- `override-verdict`;
- `decision record`;
- `worktree create/release`;
- `evidence export`.

After read surfaces are stable, the daemon can forward selected mutations
through the same dispatcher path as `striatum.api.invoke`, but only after
capability checks and audit logging. Direct CLI mode remains the compatibility
fallback for existing workflows throughout V1.

Auto-detection should be conservative:

1. If `--daemon` is passed, require a live compatible daemon or fail.
2. If `--no-daemon` is passed, use direct mode.
3. For read-only commands, try the default daemon socket when present and
   compatible; otherwise fall back to direct mode for single-repo reads.
4. For mutations, use direct mode unless the command is daemon-only or the
   user explicitly opts into daemon client mode.

Every daemon-backed CLI result should include enough metadata under `--json`
for tests to assert `mode: "daemon"` and `repository_id`.

## Global Dashboard

The global dashboard is a read model over registered repositories, not a new
state store. For each active repository, the daemon opens the repo-local DB
read-only where possible, applies pending migrations only through normal
connect paths when permitted, and queries:

- runs in `ready`, `running`, `blocked`, `needs_branch_confirmation`, or
  paused states;
- open blockers and human checkpoints;
- claimable pending work messages;
- stale leases from the existing stale-lease inspection logic;
- active sessions and lane-attestation status;
- active or lost supervisors;
- repo-local doctor problem counts.

The compact terminal view should group by repository tenant, then run. It
should show missing or schema-incompatible repositories as dashboard rows
instead of aborting the whole global view.

## MCP Surface

V1 MCP resources should be read-only by default:

- `striatum://repos`;
- `striatum://repo/<repository_id>/status`;
- `striatum://repo/<repository_id>/doctor`;
- `striatum://runs`;
- `striatum://run/<repository_id>/<run_id>`;
- `striatum://job/<repository_id>/<job_id>`;
- `striatum://blockers`;
- `striatum://supervisors`;

V1 MCP tools should start with:

- `repo_list` (read);
- `status` (read);
- `doctor` (read);
- `dashboard` (read);
- `why` (read);
- `repo_add` (admin, disabled unless capability grants it);
- `recovery_sweep` (recovery mutation, disabled unless capability grants it).

The existing single-repo `mcp.py` can become a compatibility bridge. The
daemon MCP implementation should reuse its JSON-RPC framing code but replace
single-repo `invoke()` dispatch with daemon request dispatch. Raw
`striatum/invoke` should remain disabled by default in daemon MCP because it
is too easy to bypass command classification; if retained for development,
it requires an explicit admin capability.

Mutation-capable MCP clients receive only the tools their capability grants.
For example, a client with `repo:abc:recovery` can call `recovery_sweep` for
that repository but cannot publish artifacts or start runs. A client with
only `global_read` sees no mutation tools in `tools/list`.

## Supervision Migration

Current supervision is repo-local and CLI-operated: `supervisor.py` writes
`process_supervisors`, starts a process with `start_new_session=True`, sends
packets through `.striatum/scratch/<supervisor_id>/stdin.pipe`, and sends
stdout/stderr to `DEVNULL`. That contract should remain the semantic base.

V1 daemon-resident supervision should make the daemon the process parent and
control surface while preserving repo-local visibility:

- `striatum supervise start` in daemon mode asks the daemon to start the
  lane command.
- The daemon validates the repo, run, session, lane adapter, and command
  against the immutable workflow snapshot.
- The daemon inserts or mirrors the repo-local `process_supervisors` row so
  existing status, doctor, lane attestation, and evidence surfaces continue
  to work.
- The daemon records daemon-owned details in `daemon_supervisors`.
- Packet delivery still writes newline-delimited work packet JSON for pipe
  mode.
- stdout and stderr remain `DEVNULL` unless a future product decision
  accepts transcript capture.

PTY support should be designed but not required for the first read-only
acceptance tests. The daemon should model `transport = 'pipe' | 'pty'` now
so PTY support can land without a schema break. Pipe remains the V1 default
because it matches current supervised lane wrappers and avoids terminal
transcript capture. PTY mode is necessary later for agent CLIs that require
interactive terminal behavior, but the daemon must still not infer workflow
state from screen output.

Re-attach after daemon restart has two levels:

- Pipe supervisors can be reclassified as alive/lost and can keep their
  existing FIFO if it still exists. If the daemon no longer owns an open
  writer descriptor, the next delivery must verify write behavior and mark
  the supervisor lost on failure.
- PTY supervisors should only claim re-attachment when the daemon has a
  durable PTY/session handle strategy. Until then, restart can mark them
  `detached` with a clear doctor problem.

Lane attestation must continue to use RFC 0026's stricter V1 rule:
attached-only, live pid, matching Linux process start-time token when
available, and command equality against the workflow snapshot. The daemon
does not upgrade this to model-token authorship proof.

## Resident Recovery Scheduler

The daemon should replace per-run `recovery watch` processes with a scheduler
loop that calls the existing one-shot `run_auto_sweep()` per active run. The
scheduler is resident, but recovery policy remains the same:

- lazy lease expiry still runs inside normal mutation paths and daemon sweeps;
- review-only stale work may be requeued only when policy permits and retry
  budgets allow it;
- repo-write stale leases remain stale or blocked for operator inspection;
- process reconciliation runs only when the workflow policy or daemon
  scheduler settings permit it;
- human-checkpoint timeout escalation uses the existing policy envelope and
  hooks.

This preserves D036's safety policy while removing the operational need for
one watcher command per run. Scheduler results are summarized in
`scheduler_cursors.last_result_json` and emitted as structured daemon events
without storing transcripts.

## Audit Log

Every daemon request should append one `audit_log` row after authorization is
computed. Required fields:

- `timestamp`;
- `client_id`;
- `repository_id`, nullable for global/admin calls;
- `command`, as a normalized command name such as `status`, `doctor`,
  `dashboard`, `repo.add`, or `recovery.sweep`;
- `authorization_result`;
- `transport`;
- `request_id`, when supplied;
- `exit_code`, when dispatch completed;
- `payload_sha256`, optional and computed over normalized request metadata.

Denied calls are audited. Read calls are audited too because the acceptance
criteria require every client request, and because read surfaces can reveal
workflow state. The log must not store request bodies that contain prompts,
artifact text, rationales, blocker descriptions, terminal output, or MCP
resource contents.

## Compatibility Risks

Existing examples and dogfood workflows assume direct CLI mode. V1 must keep
that path untouched unless the user opts into daemon mode. Tests should assert
that a repo with no daemon still supports the current command loop.

The service code currently has a single global `--allow-mutations` flag and a
read-command whitelist. Daemon mode should preserve the whitelist idea but
replace the global flag with per-client capabilities. Any code path that
forwards raw argv must classify before dispatch.

The current MCP wrapper exposes mutation tools in `tools/list` for a
single-repo stdio server. Daemon MCP should not copy that default. It should
advertise only tools granted to the current client.

Supervisor migration can break lane attestation if repo-local
`process_supervisors` rows and daemon `daemon_supervisors` rows diverge. The
daemon should treat repo-local rows as the compatibility source for existing
status and RFC 0026 helpers, with registry rows as ownership metadata.

Recovery sweeps across repositories can amplify a bad policy setting. The
daemon should bound sweeps per tick, preserve per-workflow retry limits, and
include dry-run tests before enabling autonomous review requeue globally.

Direct and daemon modes can diverge if they do not share dispatch semantics.
For V1, daemon-backed mutations should route through the same command
functions used by direct CLI mode after capability checks, not a parallel
implementation.

## What The Daemon Does Not Prove

Daemon mode does not prove model-token authorship. RFC 0026 lane attestation
proves only that the runner has a live process binding for a declared lane at
the relevant moment.

Daemon mode does not prove independent human decision provenance. A single
operator or operator surrogate can still route, prompt, and approve within
the authority they hold.

Daemon mode does not resist adversarial local root, a compromised Striatum
install, direct SQLite tampering by a user with file access, or OS-level
permission bypass.

Daemon mode does not provide sealed source-byte provenance unless RFC 0027
sealed patch authority is actually implemented and active. In `advisory`
mode it remains advisory. In `attested_bylines` mode it improves byline
honesty, not source containment. `sealed_patch` must continue to refuse start
until hard containment and apply authority exist.

## Test Plan

Add focused tests around the V1 acceptance slice:

| Area | Coverage |
|------|----------|
| Registry restart | Start a daemon with a temp registry, register two initialized repos, stop it, restart it, and assert both repository rows and schema versions survive. |
| Existing repo registration | Initialize a repo through current direct mode, add it to the daemon, and assert no repo-local run/artifact rows are rewritten. |
| Multi-repo dashboard | Prepare active runs in two repos and assert `dashboard --all --once --json` reports both, including blockers, claimable jobs, and stale leases. |
| CLI read client mode | With a daemon running, `status`, `doctor`, and dashboard read calls report `mode: daemon`; without a daemon they still work in direct mode for one repo. |
| Read-only MCP | A default MCP client lists and reads multi-repo resources but receives no mutation tools. |
| Mutation refusal | A client without mutation capability is denied for `repo_add`, `recovery_sweep`, and any raw invoke mutation; the denial is audited. |
| Capability grant | A client with a repository-scoped recovery capability can sweep that repo but not another repo. |
| Audit shape | Every allowed and denied request records client id, repository id where applicable, command, authorization result, timestamp, transport, and no response body. |
| Recovery sweep | The resident scheduler runs `run_auto_sweep()` across multiple active runs, requeues only eligible review-only stale work, and leaves repo-write stale work for inspection. |
| Supervisor ownership | Daemon-started supervision creates compatible repo-local `process_supervisors` data and daemon registry ownership data. |
| Re-attach after restart | After daemon restart, a still-live supervised process is detected by pid and start-time token; dead pids are marked lost and surfaced by doctor. |
| Version skew | Older/newer CLI protocol mismatches produce clear errors or direct-mode fallback only where allowed. |
| Direct mode preservation | Existing direct CLI workflow tests pass with no daemon and no registry. |

The test suite should use temporary repositories and a temporary daemon
registry path. It should avoid real agent CLIs by using small process-lane
commands that read stdin and sleep.

## Staged Plan

1. Add daemon registry paths, schema, migrations, and `striatum daemon
   status`/`repo add`/`repo list` without changing existing commands.
2. Add foreground daemon process, Unix socket request protocol, lifecycle
   lock handling, version handshake, and metadata-only audit logging.
3. Implement daemon-backed read-only `status`, `doctor`, and
   `dashboard --all`, with direct-mode fallback for single-repo reads.
4. Add daemon MCP read resources spanning repositories and make read-only the
   default tool/resource posture.
5. Add capability records and enforce mutation denial by default across
   Unix socket, HTTP, and MCP transports.
6. Add resident recovery scheduler that reuses `run_auto_sweep()` and records
   bounded scheduler results in the registry.
7. Move supervision ownership into the daemon for pipe mode while mirroring
   repo-local `process_supervisors` rows for compatibility and RFC 0026
   attestation.
8. Add optional loopback HTTP and event stream on top of the same
   authorization function.
9. Only after the V1 slice is stable, consider daemon-backed workflow
   mutations beyond recovery/supervision. Keep sealed apply, signing keys,
   remote serving, cross-repo workflows, and a Go rewrite in separate RFCs.

This path lands the acceptance criteria without pretending the daemon already
solves the harder provenance questions from RFC 0026 and RFC 0027.
