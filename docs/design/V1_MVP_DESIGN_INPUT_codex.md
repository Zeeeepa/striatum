# V1 MVP Design Input: Codex

## 1. Lane verdict

Verdict: `accept_with_conditions`.

The V1 MVP is buildable if it stays tightly scoped around a deterministic
SQLite control plane, a JSON workflow loader, and a CLI mutation surface. The
most important implementation choice is to keep queue ownership simple:
SQLite is authoritative, all state transitions happen through `agent_runner`,
and every claim/ack/complete path checks an active lease owned by the calling
session.

Non-negotiable design conditions:

- Do not let agents write SQLite directly. Read-only inspection is acceptable;
  mutation goes through CLI commands that enforce transitions, leases, artifact
  paths, and event emission.
- Do not treat tmux panes, marker files, terminal text, or provider hooks as
  authoritative state.
- Do not infer model/provider semantics from names like Codex, Claude, or
  Gemini. A lane is an explicit local command configuration plus metadata.
- Do not auto-reclaim stale write-capable jobs into a second writer. Expired
  leases on repo-write jobs should become `stale_lease` and require coordinator
  or human inspection before requeue.
- Do not capture or publish broad transcripts by default. Events store compact
  state facts; artifacts store curated outputs.

## 2. MVP boundary

In scope:

- `agent_runner init` creates `.agent_runner/`, initializes SQLite, enables
  WAL, and ensures repo-local state is ignored or documented safely.
- One repo-local SQLite database at `.agent_runner/state.sqlite3`.
- JSON workflow validation and loading. YAML must be rejected by extension,
  media type if supplied, and parse path.
- Run creation with branch confirmation gating before any workflow jobs become
  claimable.
- Session registration with opaque `session_id` plus human-readable slug
  shaped like `<role>-<model>-<ordinal>`.
- Identity-aware `claim-next` that returns a structured work packet.
- Queue semantics for claim, ack, release, heartbeat, block, verdict, and
  completion.
- Append-only event log and durable artifact references.
- CLI read/status commands: status, why, events, queue, sessions, doctor.
- Process/tmux adapter boundary as configuration and process metadata only.
  The MVP can launch commands later, but state semantics must not depend on
  a specific terminal implementation.
- RFC-ledger cleanup fixture represented as a JSON workflow and covered by
  tests using fake sessions. Tests do not run external model CLIs.

Out of scope:

- Slack, web dashboard, TUI, MCP, plugin marketplaces, hosted services,
  telemetry, external persistence, and cloud APIs.
- AI-inferred parallelization.
- Autonomous commits. The coordinator may request a human commit.
- Full transcript capture. Optional debug logs, if ever added, stay under the
  ignored `.agent_runner/` tree and are not repo-published artifacts.
- Multiple-machine coordination. SQLite is local to one worktree.

## 3. Proposed SQLite schema and indexes

Use boring SQLite with:

- `PRAGMA foreign_keys = ON`
- `PRAGMA journal_mode = WAL`
- `PRAGMA busy_timeout = 5000`
- Python-managed RFC3339 UTC timestamps.
- Short `BEGIN IMMEDIATE` transactions for every mutation command.

Recommended tables:

```sql
CREATE TABLE schema_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE workflow_snapshots (
  workflow_snapshot_id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  workflow_version TEXT,
  source_path TEXT,
  content_sha256 TEXT NOT NULL,
  workflow_json TEXT NOT NULL,
  loaded_at TEXT NOT NULL
);

CREATE TABLE runs (
  run_id TEXT PRIMARY KEY,
  workflow_snapshot_id TEXT NOT NULL REFERENCES workflow_snapshots(workflow_snapshot_id),
  repo_root TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN (
    'new','needs_branch_confirmation','ready','running','blocked',
    'completed','failed','canceled'
  )),
  branch_name TEXT,
  branch_base TEXT,
  branch_confirmed_at TEXT,
  branch_confirmed_by TEXT,
  created_at TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT,
  stop_reason TEXT,
  UNIQUE (workflow_snapshot_id, repo_root, created_at)
);

CREATE TABLE sessions (
  session_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  role_id TEXT NOT NULL,
  lane_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  ordinal INTEGER NOT NULL,
  capabilities_json TEXT NOT NULL DEFAULT '[]',
  parent_session_id TEXT REFERENCES sessions(session_id),
  first_class INTEGER NOT NULL DEFAULT 1 CHECK (first_class IN (0,1)),
  fresh_context INTEGER NOT NULL DEFAULT 0 CHECK (fresh_context IN (0,1)),
  state TEXT NOT NULL CHECK (state IN ('active','expired','stopped','lost')),
  registered_at TEXT NOT NULL,
  last_heartbeat_at TEXT,
  expires_at TEXT,
  UNIQUE (run_id, slug),
  UNIQUE (run_id, role_id, lane_id, ordinal)
);

CREATE TABLE jobs (
  job_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  workflow_job_id TEXT NOT NULL,
  title TEXT NOT NULL,
  job_type TEXT NOT NULL CHECK (job_type IN (
    'draft','review','ledger','synthesis','build','test',
    'human_checkpoint','generic'
  )),
  role_id TEXT NOT NULL,
  lane_selector_json TEXT NOT NULL DEFAULT '{}',
  capability_requirements_json TEXT NOT NULL DEFAULT '[]',
  state TEXT NOT NULL CHECK (state IN (
    'blocked','ready','queued','claimed','running','stale_lease',
    'waiting_human','completed','failed','canceled','skipped'
  )),
  attempt INTEGER NOT NULL DEFAULT 1,
  max_attempts INTEGER NOT NULL DEFAULT 1,
  fresh_session_required INTEGER NOT NULL DEFAULT 0 CHECK (fresh_session_required IN (0,1)),
  write_scope_json TEXT NOT NULL DEFAULT '{}',
  expected_artifacts_json TEXT NOT NULL DEFAULT '[]',
  idempotency_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  ready_at TEXT,
  started_at TEXT,
  completed_at TEXT,
  current_message_id TEXT,
  current_lease_id TEXT,
  UNIQUE (run_id, workflow_job_id, attempt),
  UNIQUE (run_id, idempotency_key)
);

CREATE TABLE job_dependencies (
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  depends_on_job_id TEXT NOT NULL REFERENCES jobs(job_id),
  gate_json TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY (job_id, depends_on_job_id)
);

CREATE TABLE queue_messages (
  message_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT REFERENCES jobs(job_id),
  kind TEXT NOT NULL CHECK (kind IN (
    'work','coordinator_message','agent_message','blocker',
    'human_checkpoint','commit_request'
  )),
  state TEXT NOT NULL CHECK (state IN (
    'pending','claimed','acked','blocked','completed','released',
    'expired','canceled','dead'
  )),
  priority INTEGER NOT NULL DEFAULT 0,
  target_session_id TEXT REFERENCES sessions(session_id),
  target_role_id TEXT,
  target_lane_id TEXT,
  dedupe_key TEXT,
  payload_json TEXT NOT NULL DEFAULT '{}',
  visible_after TEXT,
  claim_count INTEGER NOT NULL DEFAULT 0,
  max_claims INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  claimed_at TEXT,
  acked_at TEXT,
  completed_at TEXT,
  current_lease_id TEXT
);

CREATE TABLE leases (
  lease_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  resource_type TEXT NOT NULL CHECK (resource_type IN ('queue_message','job')),
  resource_id TEXT NOT NULL,
  owner_session_id TEXT NOT NULL REFERENCES sessions(session_id),
  state TEXT NOT NULL CHECK (state IN ('active','released','expired')),
  acquired_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  last_heartbeat_at TEXT,
  released_at TEXT,
  release_reason TEXT
);

CREATE TABLE work_packets (
  packet_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  message_id TEXT NOT NULL REFERENCES queue_messages(message_id),
  lease_id TEXT NOT NULL REFERENCES leases(lease_id),
  session_id TEXT NOT NULL REFERENCES sessions(session_id),
  packet_json TEXT NOT NULL,
  packet_sha256 TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (message_id, lease_id)
);

CREATE TABLE artifacts (
  artifact_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT REFERENCES jobs(job_id),
  session_id TEXT REFERENCES sessions(session_id),
  logical_name TEXT NOT NULL,
  artifact_kind TEXT NOT NULL CHECK (artifact_kind IN (
    'prompt','finding','findings_ledger','synthesis','marker',
    'handoff','decision','patch_summary','test_report','other'
  )),
  repo_path TEXT NOT NULL,
  content_sha256 TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  publish_mode TEXT NOT NULL CHECK (publish_mode IN (
    'create','replace_same_job','append_version'
  )),
  created_at TEXT NOT NULL,
  UNIQUE (run_id, job_id, logical_name),
  UNIQUE (run_id, repo_path, content_sha256)
);

CREATE TABLE verdicts (
  verdict_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  session_id TEXT NOT NULL REFERENCES sessions(session_id),
  verdict TEXT NOT NULL CHECK (verdict IN (
    'accept','accept_with_findings','needs_revision','reject'
  )),
  rationale TEXT,
  findings_artifact_id TEXT REFERENCES artifacts(artifact_id),
  created_at TEXT NOT NULL,
  UNIQUE (job_id, session_id, verdict)
);

CREATE TABLE blockers (
  blocker_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT REFERENCES jobs(job_id),
  session_id TEXT REFERENCES sessions(session_id),
  severity TEXT NOT NULL CHECK (severity IN ('info','warning','blocked','human_checkpoint')),
  blocker_kind TEXT NOT NULL,
  description TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('open','resolved','canceled')),
  created_at TEXT NOT NULL,
  resolved_at TEXT
);

CREATE TABLE command_requests (
  request_id TEXT PRIMARY KEY,
  run_id TEXT REFERENCES runs(run_id),
  session_id TEXT REFERENCES sessions(session_id),
  command_name TEXT NOT NULL,
  payload_sha256 TEXT NOT NULL,
  response_json TEXT,
  state TEXT NOT NULL CHECK (state IN ('started','completed','failed')),
  created_at TEXT NOT NULL,
  completed_at TEXT,
  UNIQUE (request_id, payload_sha256)
);

CREATE TABLE events (
  event_id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT REFERENCES runs(run_id),
  event_type TEXT NOT NULL,
  actor_session_id TEXT REFERENCES sessions(session_id),
  job_id TEXT REFERENCES jobs(job_id),
  message_id TEXT REFERENCES queue_messages(message_id),
  artifact_id TEXT REFERENCES artifacts(artifact_id),
  lease_id TEXT REFERENCES leases(lease_id),
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
```

Required indexes:

```sql
CREATE INDEX idx_runs_state ON runs(state);
CREATE INDEX idx_sessions_run_state ON sessions(run_id, state);
CREATE INDEX idx_sessions_last_heartbeat ON sessions(run_id, last_heartbeat_at);

CREATE INDEX idx_jobs_run_state ON jobs(run_id, state, role_id);
CREATE INDEX idx_jobs_workflow ON jobs(run_id, workflow_job_id);

CREATE INDEX idx_job_dependencies_dep ON job_dependencies(depends_on_job_id);

CREATE INDEX idx_queue_claimable ON queue_messages(
  run_id, state, target_session_id, target_role_id, target_lane_id,
  visible_after, priority, created_at
);
CREATE INDEX idx_queue_job ON queue_messages(job_id);
CREATE UNIQUE INDEX uq_active_work_message_per_job
  ON queue_messages(job_id)
  WHERE kind = 'work' AND state IN ('pending','claimed','acked');
CREATE UNIQUE INDEX uq_queue_dedupe_active
  ON queue_messages(run_id, dedupe_key)
  WHERE dedupe_key IS NOT NULL AND state IN ('pending','claimed','acked');

CREATE UNIQUE INDEX uq_active_resource_lease
  ON leases(resource_type, resource_id)
  WHERE state = 'active';
CREATE INDEX idx_leases_expiry ON leases(run_id, state, expires_at);
CREATE INDEX idx_leases_owner ON leases(owner_session_id, state);

CREATE INDEX idx_packets_job ON work_packets(job_id, created_at);
CREATE INDEX idx_artifacts_job ON artifacts(job_id, artifact_kind);
CREATE INDEX idx_verdicts_job ON verdicts(job_id, verdict);
CREATE INDEX idx_blockers_open ON blockers(run_id, state, severity);
CREATE INDEX idx_events_run_time ON events(run_id, event_id);
CREATE INDEX idx_events_job ON events(job_id, event_id);
```

Append-only protection:

```sql
CREATE TRIGGER events_no_update
BEFORE UPDATE ON events
BEGIN
  SELECT RAISE(ABORT, 'events are append-only');
END;

CREATE TRIGGER events_no_delete
BEFORE DELETE ON events
BEGIN
  SELECT RAISE(ABORT, 'events are append-only');
END;

CREATE TRIGGER artifacts_no_delete
BEFORE DELETE ON artifacts
BEGIN
  SELECT RAISE(ABORT, 'artifact records are append-only');
END;
```

SQLite cannot fully prevent hostile direct writes by a local user with file
access. The MVP should still enforce what it can with foreign keys, CHECKs,
partial unique indexes, append-only triggers, and `doctor` consistency checks.
The product contract remains: direct SQL mutation is unsupported.

## 4. Queue, lease, ack, release, heartbeat, blocker, verdict, completion semantics

Core transaction rule:

- Every mutation opens `BEGIN IMMEDIATE`, validates session/run/job/message
  state, performs the state transition, inserts one or more events, and commits.
- Every mutation requiring work ownership must pass `--session-id` and
  `--lease-id`, except `claim-next`, which creates the lease.
- Mutations should accept `--request-id`; repeated requests with the same
  request id and payload hash return the prior response.

Claim:

- `claim-next` first expires old active leases in the same run.
- It selects a `pending` work message matching the session's role, lane,
  capabilities, target session, visibility time, and run state.
- It uses a single transaction and the active lease unique index to prevent
  duplicate claims.
- It creates a `queue_message` lease and a `job` lease for work messages.
- It moves message `pending -> claimed` and job `queued/ready -> claimed`.
- It creates a stable `work_packets` row and returns `packet_json`.
- If no work is available, return exit 0 with `{"status":"no_work"}`.

Ack:

- `ack` validates that the caller owns an active unexpired lease on the
  message.
- It moves message `claimed -> acked` and job `claimed -> running`.
- It records `acked_at` and inserts `message.acked` and `job.started` events.
- Repeated ack by the same active lease is idempotent.

Heartbeat:

- `heartbeat` validates session identity and active lease ownership.
- It updates `sessions.last_heartbeat_at`, active lease `last_heartbeat_at`,
  and `expires_at`.
- It emits a compact `lease.heartbeat` event. Do not emit terminal output.
- Heartbeat should be required before half the lease duration elapses.

Release:

- `release` validates ownership or a coordinator override.
- For read-only/review-only work, release can move message `claimed/acked ->
  pending` if `claim_count < max_claims`; otherwise it moves to `dead`.
- For repo-write work, release without completion should move job to
  `blocked` unless the releasing session explicitly marks the worktree clean
  and the coordinator allows requeue.
- Release marks active leases `released`.
- Release is valid for "cannot perform", "wrong role", "need fresh context",
  and controlled retry. It should not be used to hide a failed job.

Lease expiry:

- Expiry is lazy: `claim-next`, `status`, and `doctor` can detect stale leases.
- For no-write and review-only artifact jobs, expired leases may be reclaimed
  automatically by moving message to `pending` and job to `ready`.
- For repo-write jobs, expired active leases move job to `stale_lease`, message
  to `blocked`, and create a blocker. This avoids two agents modifying the same
  branch after one session silently dies.
- A late `complete` from an expired lease must fail with invalid lease.

Blocker:

- `block` validates an active job lease unless called by coordinator.
- It writes a `blockers` row and a `blocker` queue message for the coordinator
  or human checkpoint.
- It transitions job to `blocked` or `waiting_human` based on severity.
- It releases active leases. Resuming requires coordinator action.

Verdict:

- `verdict` is valid for review jobs and review gates.
- It requires an active job lease and, when configured, a findings artifact.
- It inserts one `verdicts` row and a `verdict.recorded` event.
- For MVP, `verdict` should atomically complete the review job. This avoids a
  review verdict being recorded while the job remains running.
- Gate evaluation is deterministic:
  - `accept` and `accept_with_findings` may unlock downstream jobs.
  - `needs_revision` follows a declared bounded revision edge if attempts
    remain; otherwise it creates a human checkpoint.
  - `reject` stops the configured branch of the workflow or the run.

Completion:

- `complete` validates active job lease ownership.
- It verifies required artifacts are published and within allowed paths.
- It transitions job `running -> completed`, queue message `acked -> completed`,
  releases leases, records a completion event, and enqueues newly-unblocked
  downstream jobs.
- Repeated completion with the same request id returns the original response.
- Completion from a different session, stale lease, or wrong attempt fails.

Invalid transitions:

- `ack` before `claim` fails.
- `complete` before `ack` fails.
- `heartbeat` on a completed/released/expired lease fails.
- `verdict` from a non-review job fails.
- `publish-artifact` outside write scope fails.
- Direct duplicate active work messages for one job are rejected by the partial
  unique index.

## 5. Mutation and read/status CLI commands

All agent-facing mutation commands should emit JSON by default or with
`--json`; human read commands may pretty-print by default.

Mutation commands:

```text
agent_runner init [--repo PATH] [--state-dir .agent_runner]
agent_runner workflow validate WORKFLOW.json [--json]
agent_runner run prepare --workflow WORKFLOW.json [--repo PATH] [--json]
agent_runner branch confirm --run-id RUN --branch NAME [--create|--use-current] [--json]
agent_runner run start --run-id RUN [--json]

agent_runner register-session --run-id RUN --role ROLE --lane LANE \
  [--capability CAP]... [--fresh] [--parent-session-id SESSION] [--json]

agent_runner claim-next --session-id SESSION [--lease-seconds N] [--json]
agent_runner ack --session-id SESSION --message-id MSG --lease-id LEASE [--request-id ID]
agent_runner heartbeat --session-id SESSION --lease-id LEASE [--extend-seconds N]
agent_runner release --session-id SESSION --message-id MSG --lease-id LEASE \
  --reason TEXT [--requeue]
agent_runner send --session-id SESSION --kind agent_message --to coordinator \
  --body-json JSON
agent_runner block --session-id SESSION --job-id JOB --lease-id LEASE \
  --kind KIND --severity blocked|human_checkpoint --description TEXT
agent_runner publish-artifact --session-id SESSION --job-id JOB --lease-id LEASE \
  --kind KIND --logical-name NAME --path REPO_PATH [--request-id ID]
agent_runner complete --session-id SESSION --job-id JOB --lease-id LEASE \
  [--summary TEXT] [--request-id ID]
agent_runner verdict --session-id SESSION --job-id JOB --lease-id LEASE \
  --verdict accept|accept_with_findings|needs_revision|reject \
  [--findings-artifact-id ARTIFACT] [--rationale TEXT] [--request-id ID]
```

Read/status commands:

```text
agent_runner status [--run-id RUN] [--json]
agent_runner queue list --run-id RUN [--state pending|claimed|acked|blocked] [--json]
agent_runner sessions list --run-id RUN [--json]
agent_runner events --run-id RUN [--since-event-id ID] [--json]
agent_runner why JOB_OR_MESSAGE_ID [--json]
agent_runner doctor [--run-id RUN] [--json]
```

Exit behavior:

- `0`: command succeeded, including `claim-next` with no work.
- `2`: CLI usage or JSON validation error.
- `3`: referenced run/session/job/message/artifact not found.
- `4`: invalid state transition.
- `5`: lease conflict, stale lease, or ownership mismatch.
- `6`: artifact/write-scope violation.
- `7`: branch confirmation required.
- `8`: workflow config rejected.

`why` should explain dependency blockers, open blockers, lease owner/expiry,
missing verdicts, missing artifacts, and branch confirmation state.

`doctor` should detect direct-write symptoms where feasible: active job without
active lease, active queue message without active lease, multiple terminal
events for the same job attempt, missing completion events, artifact file hash
mismatch, YAML workflow files, and branch mismatch.

## 6. Work packet and JSON workflow config shape

Work packet shape:

```json
{
  "packet_version": "agent-runner.work-packet.v1",
  "packet_id": "wp_...",
  "run": {
    "run_id": "run_...",
    "workflow_id": "rfc-ledger-cleanup",
    "repo_root": "/repo",
    "branch": {
      "name": "agent-runner/rfc-ledger-cleanup",
      "confirmed": true
    }
  },
  "session": {
    "session_id": "sess_...",
    "slug": "reviewer-codex-1",
    "role_id": "reviewer",
    "lane_id": "codex_gpt_5_5",
    "capabilities": ["review", "markdown_artifact"]
  },
  "lease": {
    "lease_id": "lease_...",
    "message_id": "msg_...",
    "expires_at": "2026-05-06T06:30:00Z",
    "heartbeat_after_seconds": 300
  },
  "job": {
    "job_id": "job_...",
    "workflow_job_id": "review",
    "attempt": 1,
    "title": "Review RFC ledger draft",
    "type": "review",
    "fresh_session_required": true,
    "objective": "Review the draft for missed RFC status cleanup and unsafe decisions."
  },
  "role": {
    "role_id": "reviewer",
    "definition_path": "roles/reviewer.md",
    "inline_summary": "Adversarial reviewer. Do not edit source artifacts."
  },
  "context": {
    "docs": [
      {"path": "README.md", "required": true},
      {"path": "docs/rfcs/README.md", "required": true}
    ],
    "content_mode": "references"
  },
  "task_prompt": {
    "path": "prompts/rfc_ledger_review.md",
    "inline_text": null
  },
  "inputs": [
    {
      "kind": "artifact",
      "from_job_id": "draft",
      "path": "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md"
    }
  ],
  "write_scope": {
    "mode": "review_only_artifact",
    "allowed_paths": ["docs/reviews/rfc-ledger/"],
    "forbidden_paths": [".agent_runner/", "docs/SPEC.md"],
    "repo_write": false
  },
  "expected_artifacts": [
    {
      "logical_name": "review",
      "kind": "finding",
      "path": "docs/reviews/rfc-ledger/RFC_LEDGER_REVIEW_codex.md",
      "required": true
    }
  ],
  "commands": {
    "ack": "agent_runner ack --session-id sess_... --message-id msg_... --lease-id lease_...",
    "heartbeat": "agent_runner heartbeat --session-id sess_... --lease-id lease_...",
    "publish_artifact": "agent_runner publish-artifact --session-id sess_... --job-id job_... --lease-id lease_...",
    "block": "agent_runner block --session-id sess_... --job-id job_... --lease-id lease_...",
    "verdict": "agent_runner verdict --session-id sess_... --job-id job_... --lease-id lease_...",
    "complete": "agent_runner complete --session-id sess_... --job-id job_... --lease-id lease_..."
  },
  "stop_conditions": [
    "Do not edit files outside write_scope.",
    "Block if required input artifacts are missing."
  ],
  "artifact_policy": {
    "publish_transcripts": false,
    "curated_artifacts_only": true
  }
}
```

Workflow config shape must be JSON:

```json
{
  "schema_version": "agent-runner.workflow.v1",
  "workflow_id": "rfc-ledger-cleanup",
  "workflow_version": "2026-05-06",
  "name": "RFC Ledger Cleanup",
  "branch": {
    "mode": "confirm",
    "suggested_name": "agent-runner/rfc-ledger-cleanup",
    "allow_dirty": false
  },
  "coordinator": {
    "role_id": "coordinator",
    "lane_id": "codex_gpt_5_5"
  },
  "lanes": {
    "codex_gpt_5_5": {
      "adapter": "process",
      "display_model": "Codex GPT-5.5",
      "command": ["codex", "exec", "--model", "gpt-5.5", "-"],
      "capabilities": ["write", "review", "synthesis"]
    },
    "gemini_pro_3_1": {
      "adapter": "process",
      "display_model": "Gemini Pro 3.1",
      "command": ["gemini", "--model", "gemini-3.1-pro-preview"],
      "capabilities": ["review"]
    }
  },
  "roles": {
    "author": {"definition_path": "roles/author.md"},
    "reviewer": {"definition_path": "roles/reviewer.md"},
    "ledger": {"definition_path": "roles/findings-ledger.md"},
    "synthesizer": {"definition_path": "roles/synthesizer.md"}
  },
  "context_docs": [
    {"path": "README.md", "required": true},
    {"path": "docs/process/multi-agent-review-loop.md", "required": true}
  ],
  "session_policy": {
    "default": "persistent_by_role",
    "fresh_context_jobs": ["review_codex", "review_gemini", "final_review"]
  },
  "parallelism": {
    "mode": "declared",
    "max_active_jobs": 3,
    "require_disjoint_write_scopes": true
  },
  "jobs": [
    {
      "id": "draft",
      "type": "draft",
      "role_id": "author",
      "lane_id": "codex_gpt_5_5",
      "prompt_path": "prompts/fixtures/rfc_ledger_draft.md",
      "needs": [],
      "write_scope": {
        "mode": "repo_write",
        "allowed_paths": ["docs/reviews/rfc-ledger/"]
      },
      "expected_artifacts": [
        {
          "logical_name": "draft",
          "kind": "handoff",
          "path": "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
          "required": true
        }
      ]
    }
  ],
  "edges": [
    {"from": "draft", "to": "review_codex", "on": "completed"},
    {"from": "draft", "to": "review_gemini", "on": "completed"}
  ],
  "cycles": [
    {
      "from": "final_review",
      "to": "synthesis",
      "on_verdict": "needs_revision",
      "max_iterations": 1
    }
  ]
}
```

Workflow validation should require:

- Top-level `schema_version`.
- No YAML parsing path.
- All job IDs unique.
- Edges reference known jobs.
- Cycles have `max_iterations`.
- Parallel jobs have disjoint repo-write scopes or are review-only with unique
  artifact paths.
- Expected artifact paths are repo-relative and do not escape the repo.
- Lane commands are explicit arrays. Core code must not synthesize provider
  flags.

## 7. Role/context/task prompt assembly and session policy

Prompt assembly should be deterministic and auditable:

1. Control preamble: local-first constraints, run id, session id, branch,
   write-scope rules, and CLI completion protocol.
2. Role definition: reusable role file or workflow inline role.
3. Generic context docs: repo/process docs, in declared order.
4. Task prompt: job-specific objective and acceptance criteria.
5. Inputs: prior artifacts, verdicts, blockers, and handoffs.
6. Completion protocol: exact CLI commands and required artifacts.

Store the assembled packet JSON in SQLite and optionally in ignored local state
under `.agent_runner/packets/`. Do not publish packets as repo artifacts unless
the workflow explicitly classifies a prompt/handoff as durable.

Persistent session policy:

- Default is persistent by `(run_id, role_id, lane_id)` while the role remains
  active.
- A session may claim multiple jobs if role, lane, capabilities, and freshness
  policy match.
- `fresh_session_required` excludes existing sessions and requires a new
  `register-session --fresh` identity.
- Fresh context means a new first-class session, not clearing a database row.
- Native sub-agents created inside an agent CLI remain internal to the parent
  session. The parent owns write scope, artifacts, completion, and audit.
- If a native sub-agent needs independent queue claims or artifact ownership,
  it must be explicitly registered as its own first-class session.

Hidden provider assumption guard:

- Lane ID and display model are metadata.
- Launch command, environment allowlist, cwd, stdin behavior, and PTY mode live
  in JSON config.
- The core scheduler matches roles/capabilities, not provider names.

## 8. Artifact publishing and event log policy

Artifacts:

- Repo-published artifacts are curated outputs: decisions, prompts, findings,
  findings ledgers, syntheses, markers, handoffs, test reports, and compact
  summaries.
- `publish-artifact` validates that the file exists, path is repo-relative,
  path is inside the job write scope, path is not under `.agent_runner/`, kind
  is allowed by workflow policy, and content hash matches what is recorded.
- Replacement is allowed only by declared mode:
  - `create`: fail if path exists.
  - `replace_same_job`: allow the same job attempt to replace its own artifact.
  - `append_version`: require a new path or versioned logical name.
- Transcript artifacts are rejected unless the workflow explicitly sets
  `allow_transcripts: true`; default is false.

Events:

- `events` is append-only and structured.
- Events record state facts, not terminal transcripts.
- Payloads should contain IDs, statuses, counts, paths, hashes, and compact
  summaries.
- Process stdout/stderr capture is out of scope for repo artifacts. If adapter
  debug logs are needed later, store them only under ignored local state and
  reference redacted summaries from durable artifacts.

Minimum event types:

```text
run.created
run.branch_confirmed
run.started
session.registered
session.heartbeat
queue.message_enqueued
queue.claimed
queue.acked
lease.heartbeat
lease.released
lease.expired
job.started
job.blocked
job.completed
verdict.recorded
artifact.published
workflow.gate_evaluated
human_checkpoint.requested
run.completed
run.failed
```

## 9. Branch confirmation behavior

Run startup should be a two-step gate:

1. `run prepare --workflow WORKFLOW.json` validates workflow, snapshots config,
   inspects git state, proposes a branch, and leaves run state as
   `needs_branch_confirmation`.
2. `branch confirm --run-id RUN --branch NAME --create` or `--use-current`
   records explicit confirmation and performs the git branch action if needed.

No workflow job is claimable until:

- branch confirmation exists,
- current branch matches the confirmed branch,
- dirty worktree policy is satisfied,
- run state is `ready` or `running`.

Dirty worktree handling:

- Default `allow_dirty` is false for repo-write workflows.
- Review-only workflows may allow dirty state if artifact paths are disjoint.
- If dirty state exists, CLI reports concise path status and asks for explicit
  confirmation or blocks with a human checkpoint.

Commit behavior:

- The coordinator can enqueue a `commit_request` message after successful run
  completion.
- The MVP should not commit automatically by default.
- Commit requests are artifacts/messages, not hidden git actions.

## 10. RFC-ledger validation fixture

The first validation fixture should represent this workflow:

```text
draft -> review -> findings ledger -> synthesis -> final review
```

Recommended fixture expansion:

```text
draft
  -> review_codex
  -> review_gemini
  -> findings_ledger
  -> synthesis
  -> final_review
```

`review_codex` and `review_gemini` can run in declared parallelism because they
are review-only jobs with unique artifact paths. Tests can use fake sessions;
they do not need to launch Codex or Gemini.

Fixture artifacts:

- `docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md`
- `docs/reviews/rfc-ledger/RFC_LEDGER_REVIEW_codex.md`
- `docs/reviews/rfc-ledger/RFC_LEDGER_REVIEW_gemini.md`
- `docs/reviews/rfc-ledger/RFC_LEDGER_FINDINGS_LEDGER.md`
- `docs/reviews/rfc-ledger/RFC_LEDGER_SYNTHESIS.md`
- `docs/reviews/rfc-ledger/RFC_LEDGER_FINAL_REVIEW.md`

Fixture should demonstrate:

- reusable roles: author, reviewer, ledger recorder, synthesizer;
- generic context docs: README, RFC index, process docs;
- task prompts as job inputs;
- expected artifacts and path validation;
- review-only parallelism with unique artifact paths;
- final review verdicts;
- bounded revision loop from `final_review.needs_revision -> synthesis` with
  `max_iterations: 1`;
- durable findings and synthesis artifacts;
- no transcript capture.

Test execution should simulate:

1. Load fixture JSON.
2. Prepare run and confirm branch in a temporary git repo.
3. Register sessions.
4. Claim/ack/complete draft.
5. Claim/ack/verdict parallel reviews.
6. Complete findings ledger and synthesis.
7. Record final review verdict.
8. Verify events, artifacts, dependencies, and final run status.

## 11. Test strategy

Use deterministic unit and integration tests with temporary directories and
SQLite databases. Do not call live LLMs or external model CLIs.

Core tests:

- `init` creates `.agent_runner/`, SQLite schema, WAL mode, and ignore handling.
- Workflow validator accepts JSON fixture and rejects YAML.
- Branch prepare/confirm blocks job claims until confirmed.
- Session registration produces stable opaque IDs and unique slugs.
- Fresh-session jobs cannot be claimed by an existing persistent session.
- `claim-next` returns no duplicate work under two-connection contention.
- Active lease unique index rejects duplicate claims.
- `ack`, `heartbeat`, `release`, `block`, `complete`, and `verdict` enforce
  valid transitions.
- Late completion after lease expiry fails.
- Expired review-only lease requeues safely.
- Expired repo-write lease creates `stale_lease` and blocker.
- `publish-artifact` rejects paths outside repo, outside write scope, and
  transcript kind by default.
- Required artifacts are enforced before completion/verdict.
- Event log is append-only.
- Artifact records are append-only.
- Command request idempotency returns prior response for repeated request IDs.
- `status --json`, `queue list --json`, `events --json`, and `why --json`
  return stable machine-readable structures.
- `doctor` detects active jobs without active leases, artifact hash mismatch,
  and branch mismatch.
- RFC-ledger fixture runs end-to-end with fake sessions.

Concurrency tests:

- Use two SQLite connections and `BEGIN IMMEDIATE`.
- Avoid sleep-based race tests. Use deterministic transaction ordering and
  barriers where threads are needed.
- Confirm one claimant wins and the other returns `no_work` or lease conflict.
- Confirm busy timeout behavior is bounded and returns a clear error.

CLI smoke:

```bash
agent_runner init
agent_runner workflow validate examples/rfc-ledger-cleanup.json --json
agent_runner run prepare --workflow examples/rfc-ledger-cleanup.json --json
agent_runner status --json
agent_runner doctor --json
```

Package checks should eventually be:

```bash
make test
```

If the Makefile does not exist yet during implementation, create it with a
single test target rather than inventing a separate undocumented command.

## 12. Risks, blockers, and recommendations

Risks:

- Direct SQLite writes cannot be perfectly blocked in a local file database.
  Mitigation: document read-only SQL only, enforce constraints/triggers, and
  make `doctor` detect common corruption.
- Stale write leases are dangerous on a shared branch. Mitigation: do not
  auto-reclaim repo-write jobs; require inspection or explicit requeue.
- Artifact collisions can make idempotence fake. Mitigation: logical artifact
  uniqueness plus content hashes and declared publish modes.
- Persistent sessions can contaminate independent review. Mitigation: workflow
  `fresh_session_required` and fresh role instantiation for adversarial review.
- Hidden provider assumptions can creep into role names and workflow examples.
  Mitigation: every lane command is explicit config; core uses capabilities.
- SQLite contention is manageable only if transactions are short. Mitigation:
  do not hold DB transactions while launching agents, reading files, or writing
  artifacts.
- Workflow cycles can become unbounded. Mitigation: require `max_iterations`
  and stop with a human checkpoint when exhausted.
- Branch confirmation can be bypassed accidentally if jobs are enqueued too
  early. Mitigation: run state `needs_branch_confirmation` blocks claims even
  if queue messages already exist.

Recommendations:

- Implement schema and transitions before adapters. The fake-session RFC-ledger
  fixture should pass before any tmux/process launch work.
- Keep state changes boring and explicit. A small transition module is more
  valuable than a broad abstraction.
- Make JSON output stable from the first CLI implementation; agents will paste
  these commands into prompts.
- Treat `.agent_runner/` as ignored operational state and repo artifacts as
  curated provenance. Do not blur the two.
- Add a decision only if synthesis changes accepted product architecture. The
  recommendations above fit within the current decision log.
