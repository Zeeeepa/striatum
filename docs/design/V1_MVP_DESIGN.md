# agent_runner V1 MVP Design

Status: synthesized design
Date: 2026-05-06

This design synthesizes the required model-lane inputs:

- `docs/design/V1_MVP_DESIGN_INPUT_claude.md`
- `docs/design/V1_MVP_DESIGN_INPUT_codex.md`
- `docs/design/V1_MVP_DESIGN_INPUT_gemini.md`

## 1. MVP Boundary

V1 is a local Python CLI MVP for repo-local terminal-agent orchestration. The
control plane is deterministic. Agents mutate state through `agent_runner`
commands. SQLite is authoritative for live workflow state; repo artifacts are
durable provenance only.

In scope:

- `agent_runner init` creates `.agent_runner/`, initializes SQLite, and ensures
  `.agent_runner/` is ignored by git.
- JSON workflow validation and loading. YAML is rejected.
- Run preparation and branch confirmation before jobs become claimable.
- Registered sessions with opaque `session_id` and human-readable
  `<role>-<lane>-<ordinal>` slugs.
- Identity-aware `claim-next` returning a structured work packet.
- Queue transitions for `ack`, `release`, `heartbeat`, `block`, `complete`,
  and `verdict`.
- Append-only event log and artifact references.
- `status`, `status --json`, `why`, and `doctor`.
- RFC-ledger cleanup fixture represented as JSON and exercised by tests without
  live model calls.

Out of scope:

- Slack, web, TUI, MCP, plugin marketplaces, external persistence, hosted
  services, telemetry, and cloud APIs.
- AI-inferred parallelization.
- Automatic commits, pushes, rebases, or merges.
- Broad transcript capture.
- Cross-machine coordination.
- Background queue daemons. Lease expiry is lazy and checked by CLI commands.
- Production-grade PTY supervision. V1 defines the process/tmux adapter
  boundary but the tested MVP is the state, workflow, and packet contract.

## 2. Work Packet

`claim-next` returns one JSON work packet. The packet is also stored in SQLite
as immutable packet JSON plus a SHA-256 hash.

Required shape:

```json
{
  "packet_version": "agent-runner.work-packet.v1",
  "packet_id": "wp_...",
  "run": {
    "run_id": "run_...",
    "workflow_id": "rfc-ledger-cleanup",
    "repo_root": "/repo",
    "branch": {"name": "agent-runner/rfc-ledger-cleanup", "confirmed": true}
  },
  "session": {
    "session_id": "sess_...",
    "slug": "reviewer-codex-1",
    "role_id": "reviewer",
    "lane_id": "codex",
    "capabilities": ["review"]
  },
  "lease": {
    "lease_id": "lease_...",
    "message_id": "msg_...",
    "expires_at": "2026-05-06T06:30:00Z",
    "heartbeat_after_seconds": 300
  },
  "job": {
    "job_id": "job_...",
    "workflow_job_id": "review_codex",
    "attempt": 1,
    "type": "review",
    "title": "Review RFC ledger draft",
    "objective": "Write an adversarial review.",
    "fresh_session_required": true
  },
  "role": {
    "role_id": "reviewer",
    "definition_path": "roles/reviewer.md",
    "inline_summary": null
  },
  "context": {
    "docs": [{"path": "README.md", "required": true}],
    "content_mode": "references"
  },
  "task_prompt": {"path": "prompts/review.md", "inline_text": null},
  "inputs": [{"kind": "artifact", "path": "docs/reviews/rfc-ledger/draft.md"}],
  "write_scope": {
    "mode": "review_only_artifact",
    "allowed_paths": ["docs/reviews/rfc-ledger/"],
    "forbidden_paths": [".agent_runner/"],
    "repo_write": false
  },
  "expected_artifacts": [
    {
      "logical_name": "review",
      "kind": "finding",
      "path": "docs/reviews/rfc-ledger/review_codex.md",
      "required": true
    }
  ],
  "commands": {
    "ack": "agent_runner ack --session-id ...",
    "heartbeat": "agent_runner heartbeat --session-id ...",
    "publish_artifact": "agent_runner publish-artifact --session-id ...",
    "block": "agent_runner block --session-id ...",
    "verdict": "agent_runner verdict --session-id ...",
    "complete": "agent_runner complete --session-id ..."
  },
  "artifact_policy": {
    "publish_transcripts": false,
    "curated_artifacts_only": true
  }
}
```

Assembly order is deterministic: control preamble, role definition, generic
context docs, task prompt, prior artifacts/messages, expected artifacts, write
scope, and completion protocol.

## 3. SQLite Schema

Database path: `.agent_runner/state.sqlite3`.

Connection defaults:

- `PRAGMA foreign_keys = ON`
- `PRAGMA journal_mode = WAL`
- `PRAGMA busy_timeout = 5000`
- Short `BEGIN IMMEDIATE` transactions for mutation commands.

Tables:

- `schema_meta(key, value)`.
- `workflow_snapshots(workflow_snapshot_id, workflow_id, workflow_version,
  source_path, content_sha256, workflow_json, loaded_at)`.
- `runs(run_id, workflow_snapshot_id, repo_root, state, branch_name,
  branch_base, branch_confirmed_at, branch_confirmed_by, created_at,
  started_at, completed_at, stop_reason)`.
- `sessions(session_id, run_id, role_id, lane_id, slug, ordinal,
  capabilities_json, parent_session_id, first_class, fresh_context, state,
  registered_at, last_heartbeat_at, expires_at)`.
- `jobs(job_id, run_id, workflow_job_id, title, job_type, role_id,
  lane_selector_json, capability_requirements_json, state, attempt,
  max_attempts, fresh_session_required, write_scope_json,
  expected_artifacts_json, idempotency_key, created_at, ready_at, started_at,
  completed_at, current_message_id, current_lease_id)`.
- `job_dependencies(job_id, depends_on_job_id, gate_json)`.
- `queue_messages(message_id, run_id, job_id, kind, state, priority,
  target_session_id, target_role_id, target_lane_id, dedupe_key, payload_json,
  visible_after, claim_count, max_claims, created_at, updated_at, claimed_at,
  acked_at, completed_at, current_lease_id)`.
- `leases(lease_id, run_id, resource_type, resource_id, owner_session_id,
  state, acquired_at, expires_at, last_heartbeat_at, released_at,
  release_reason)`.
- `work_packets(packet_id, run_id, job_id, message_id, lease_id, session_id,
  packet_json, packet_sha256, created_at)`.
- `artifacts(artifact_id, run_id, job_id, session_id, logical_name,
  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
  created_at)`.
- `verdicts(verdict_id, run_id, job_id, session_id, verdict, rationale,
  findings_artifact_id, created_at)`.
- `blockers(blocker_id, run_id, job_id, session_id, severity, blocker_kind,
  description, state, created_at, resolved_at)`.
- `command_requests(request_id, run_id, session_id, command_name,
  payload_sha256, response_json, state, created_at, completed_at)`.
- `events(event_id, run_id, event_type, actor_session_id, job_id, message_id,
  artifact_id, lease_id, payload_json, created_at)`.

Required invariants:

- `events` is append-only by trigger.
- `artifacts` records are append-only by trigger.
- Sessions are unique by `(run_id, slug)` and `(run_id, role_id, lane_id,
  ordinal)`.
- One active lease per resource via a partial unique index on
  `(resource_type, resource_id)` where `state='active'`.
- One active work message per job via a partial unique index where
  `kind='work'` and state is `pending`, `claimed`, or `acked`.
- Artifact paths must be repo-relative, outside `.agent_runner/`, and inside
  the packet write scope.

Direct SQLite mutation by agents is unsupported. The CLI owns writes and
invariants. SQLite constraints and `doctor` catch common corruption, but a
local user with file access remains able to mutate the database outside the
contract.

## 4. Queue And Lease Semantics

Every mutating command validates session identity, opens a transaction, checks
state, applies the transition, inserts one or more events, and commits.

`claim-next`:

- Lazily expires active leases before selecting work.
- Selects the oldest pending work message matching the session run, role, lane,
  capabilities, and freshness policy.
- Fails if the run is waiting on branch confirmation.
- Moves message `pending -> claimed` and job `queued/ready -> claimed`.
- Creates an active job lease and a stable work packet.
- Returns `{"status":"no_work"}` with exit 0 when no work is available.

`ack`:

- Requires caller ownership of an active unexpired lease.
- Moves message `claimed -> acked` and job `claimed -> running`.
- Is idempotent for the same active lease.

`heartbeat`:

- Requires caller ownership of an active lease.
- Updates session and lease heartbeat timestamps and extends the lease.

`release`:

- Requires active lease ownership.
- For review-only/no-write jobs, may requeue if attempts remain.
- For repo-write jobs, moves job to `stale_lease` or `blocked` unless the
  coordinator explicitly requeues after inspection.

`block`:

- Records a blocker and event.
- Moves job to `blocked` or `waiting_human`.
- Releases active leases.

`verdict`:

- Valid for review jobs.
- Records `accept`, `accept_with_findings`, `needs_revision`, or `reject`.
- In V1, verdict atomically completes the review job after required artifacts
  are registered.

`complete`:

- Requires active lease ownership.
- Verifies all required artifacts are registered.
- Moves job `running -> completed`, message `acked -> completed`, releases
  leases, emits events, and enqueues newly unblocked downstream jobs.

Lease expiry:

- Expiry is lazy, not daemon-driven.
- Expired review-only work can be requeued.
- Expired repo-write work becomes `stale_lease` and requires coordinator or
  human inspection.

## 5. CLI Surface

Mutation commands:

```text
agent_runner init [--repo PATH]
agent_runner workflow validate WORKFLOW.json [--json]
agent_runner run prepare --workflow WORKFLOW.json [--repo PATH] [--json]
agent_runner branch confirm --run-id RUN --branch NAME [--create|--use-current] [--json]
agent_runner run start --run-id RUN [--json]
agent_runner register-session --run-id RUN --role ROLE --lane LANE [--capability CAP]... [--fresh] [--json]
agent_runner claim-next --session-id SESSION [--lease-seconds N] [--json]
agent_runner ack --session-id SESSION --message-id MSG --lease-id LEASE [--json]
agent_runner heartbeat --session-id SESSION --lease-id LEASE [--extend-seconds N] [--json]
agent_runner release --session-id SESSION --message-id MSG --lease-id LEASE --reason TEXT [--requeue] [--json]
agent_runner send --session-id SESSION --kind KIND --body-json JSON [--json]
agent_runner block --session-id SESSION --job-id JOB --lease-id LEASE --kind KIND --severity blocked|human_checkpoint --description TEXT [--json]
agent_runner publish-artifact --session-id SESSION --job-id JOB --lease-id LEASE --kind KIND --logical-name NAME --path REPO_PATH [--json]
agent_runner complete --session-id SESSION --job-id JOB --lease-id LEASE [--summary TEXT] [--json]
agent_runner verdict --session-id SESSION --job-id JOB --lease-id LEASE --verdict accept|accept_with_findings|needs_revision|reject [--findings-artifact-id ARTIFACT] [--json]
```

Read commands:

```text
agent_runner status [--run-id RUN] [--json]
agent_runner why JOB_OR_MESSAGE_ID [--json]
agent_runner doctor [--run-id RUN] [--json]
```

Exit codes:

- `0`: success, including no work.
- `2`: usage or JSON validation error.
- `3`: missing run/session/job/message/artifact.
- `4`: invalid transition.
- `5`: stale lease or ownership mismatch.
- `6`: artifact/write-scope violation.
- `7`: branch confirmation required.
- `8`: workflow config rejected.

## 6. Workflow Config

Workflow config is JSON with
`schema_version: "agent-runner.workflow.v1"`. YAML is rejected by extension
and parser path.

Required top-level keys:

- `schema_version`
- `workflow_id`
- `workflow_version`
- `name`
- `branch`
- `coordinator`
- `lanes`
- `roles`
- `context_docs`
- `parallelism`
- `jobs`
- `edges`
- `cycles`

Validator rules:

- All job ids are unique.
- All role and lane references resolve.
- All edges reference known jobs.
- Cycles declare `max_iterations`.
- Expected artifact paths are repo-relative and do not escape the repo.
- Parallel jobs either have disjoint repo-write scopes or are review-only with
  unique artifact paths.
- Lane commands are explicit arrays. Core code must not synthesize provider
  flags or infer behavior from model names.

## 7. Sessions And Fresh Roles

Default session behavior is persistent by `(run_id, role_id, lane_id)` while a
role remains active. Jobs may set `fresh_session_required=true`; those jobs
must be claimed by a newly registered session. Review jobs default to fresh
sessions in fixtures and recommended workflows.

Native sub-agents spawned by an agent CLI are internal to the parent session.
They inherit the parent session's accountability unless explicitly registered
with `--parent-session-id` as first-class sessions.

## 8. Artifact And Event Policy

Published artifacts are curated repo outputs: prompts, findings, findings
ledgers, syntheses, decisions, markers, handoffs, and test reports. Markers are
artifacts only, never live control state.

`publish-artifact` validates file existence, path scope, artifact kind,
content hash, and publish mode. Transcript artifacts are rejected by default.

Events are structured state facts. Payloads should contain ids, states, paths,
hashes, counts, and compact summaries, not terminal transcripts.

## 9. Branch Confirmation

Workflow startup is gated:

1. `run prepare` validates and snapshots workflow JSON, inspects git state,
   proposes a branch, and leaves the run in `needs_branch_confirmation`.
2. `branch confirm` records explicit human confirmation and optionally creates
   or selects the branch.
3. `run start` makes eligible root jobs claimable only after confirmation.

No job can be claimed while branch confirmation is missing. The MVP does not
commit, push, merge, or rebase.

## 10. RFC-Ledger Fixture

The V1 fixture represents:

```text
draft -> parallel reviews -> findings ledger -> synthesis -> final review
```

Parallel reviews are allowed only because they are review-only jobs with unique
artifact paths. The fixture demonstrates reusable roles, generic context docs,
task prompts, expected artifacts, bounded `needs_revision` cycles, durable
findings/synthesis artifacts, and no transcript capture.

Tests use fake sessions and do not run external model CLIs.

## 11. Test Strategy

Required tests:

- `init` creates `.agent_runner/`, schema, WAL state, and ignore entry.
- Workflow validator accepts the JSON fixture and rejects YAML.
- Branch confirmation blocks claims before confirmation.
- Session registration creates opaque ids and unique slugs.
- Fresh-session jobs reject reused sessions.
- `claim-next` does not duplicate work under two-connection contention.
- `ack`, `heartbeat`, `release`, `block`, `complete`, and `verdict` enforce
  transitions.
- Expired review-only leases can requeue; expired repo-write leases become
  blocked/stale.
- Artifact publishing enforces write scope and rejects transcripts by default.
- Required artifacts are enforced before completion/verdict.
- Event log is append-only.
- `status --json`, `why --json`, and `doctor --json` produce stable JSON.
- RFC-ledger fixture runs end to end with fake sessions.

The primary verification command is `make test`.

## 12. Design Synthesis Decisions

Accepted:

- Claude and Codex's full CLI/SQLite/control-plane shape.
- Gemini's warning against background lease daemons and complex automatic
  recovery.
- Review roles default to fresh sessions.
- Markers remain artifacts only.

Modified:

- Gemini's recommendation to drop `ack`, `heartbeat`, and `verdict` is not
  adopted because P001 makes those commands MVP requirements. They are kept
  with minimal lazy semantics.
- Parallelism is kept only for declared review-only/disjoint-write jobs.

Deferred:

- Real PTY/tmux supervision beyond a process-adapter boundary.
- Coordinator chat UX beyond deterministic CLI skills.
- Dashboards, Slack, MCP, plugin discovery, telemetry, hosted services, and
  transcript capture.

Rejected:

- YAML workflow configuration.
- Auto-commit or auto-push.
- Treating marker files, terminal text, or provider hooks as live state.
