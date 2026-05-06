# V1 MVP Design Input: Claude

Lane: Claude Opus design lane for `agent_runner` P001.
Date: 2026-05-06.
Posture: design notes only, no code, no source/spec/decision/language edits.

This file is one of three required design inputs (Claude / Codex / Gemini) per
D032. Synthesis must cite this document.

## 1. Lane verdict

The PRD and decision log already pin the right product shape. Concrete
recommendations follow for the schemas, command surface, queue semantics,
session lifecycle, branch/artifact policy, and the RFC-ledger fixture.

Top-line risks the synthesis must guard:

- Coordinator drift into synthesis. The selected AI coordinator lane will be
  asked to "just write it" and will comply unless the CLI itself denies it.
- Marker / SQLite duality. Engram's tmux harness uses marker files as the live
  message bus. `agent_runner` must not. Markers and other repo artifacts are
  published outputs from SQLite, never read as control-plane state.
- Persistent-session contamination across reviews. Persistent sessions are
  preferred by D011 only while the role is active; reviews that need
  independent judgment must default to fresh role instantiation.
- Sub-agent leakage. Claude Code, Codex, and Gemini all spawn child processes
  internally. The mutation contract must bind those to the parent session id.
- Workflow config sprawl. Without a strict JSON schema and a validator that
  refuses YAML, workflows grow unbounded.

Verdict: approve the product boundary as defined; ship the MVP at the scope
described in section 2; reject any expansion that reintroduces hosted
services, transcript capture, plugin discovery, or automatic git
commits/pushes.

## 2. MVP boundary

In scope (Claude lane recommendation):

- SQLite state store under `.agent_runner/agent_runner.sqlite3` (WAL, foreign
  keys, append-only `events`, queue tables, lease columns, verdicts,
  artifacts, human checkpoints).
- Mutation CLI: `register-session`, `claim-next`, `ack`, `release`,
  `heartbeat`, `block`, `complete`, `verdict`, `publish-artifact`, `send`,
  `read-prompt`. All require `--session` except `register-session`.
- Admin CLI: `init`, `run start|resume|abort`, `status`, `why`, `doctor`,
  `workflow validate`, `branch confirm`, `checkpoint resolve`.
- JSON workflow config + JSON Schema validator. YAML refused at parse and at
  validate.
- Work packet assembly from role + context docs + task prompt + inputs +
  expected artifacts + write scope + completion protocol.
- Process/PTY adapter (single class) that launches lane commands. Tmux is a
  thin wrapper that attaches the same process to a tmux window for visibility,
  not a separate runtime.
- Branch confirmation gate at run start. No auto-commit ever.
- Artifact publisher with content-addressed idempotence and write-scope checks.
- RFC-ledger workflow fixture as the validation harness, exercisable without
  live model calls (mock sessions).
- Test strategy as in section 11.

Out of MVP (must remain explicitly deferred in `docs/SPEC.md`):

- TUI, web dashboard, Slack, MCP server, REST API.
- AI-inferred build parallelization. Parallelism stays declared in JSON.
- Auto-commit, auto-push, auto-merge, auto-rebase.
- Cross-machine coordination, hosted state, telemetry, plugin marketplace,
  lane discovery, lane upgrade flows.
- Provider-specific protocol handling in core (slash commands, MCP, hooks
  belong inside lane command templates the operator declares, not core
  semantics).
- Transcript capture. Optional debug capture only under
  `.agent_runner/diagnostics/<session_id>.log`, gitignored, off by default.
- Coordinator-as-author. The AI coordinator must not be able to publish
  artifacts for jobs other than coordinator-owned jobs.

## 3. User flows and coordinator responsibilities

User flow A — bootstrap a repo:

1. `agent_runner init` creates `.agent_runner/`, initializes SQLite, ensures
   `.gitignore` covers `.agent_runner/` and `.agent_runner/diagnostics/`.
2. `agent_runner doctor` reports schema version, WAL state, and any orphaned
   leases.

User flow B — start a workflow run:

1. Operator: `agent_runner run start --workflow examples/workflows/rfc_ledger_cleanup/workflow.json --coordinator-lane claude_opus`.
2. The deterministic coordinator validates JSON, registers workflow/run rows,
   loads roles/lanes/context refs, and stages all jobs in `state=staged`.
3. If `branch.policy == confirm_required`, the runner emits a
   `branch_proposal` human checkpoint and stops. No git mutation yet.
4. Operator: `agent_runner branch confirm --run R --branch feature/...`. The
   runner performs the `git switch -c` and transitions the run to `running`.
5. The runner launches the AI coordinator session under the selected lane,
   piping a coordinator system prompt that names allowed coordinator skills.
6. Worker sessions are launched per workflow job either eagerly (declared in
   workflow) or on demand by the coordinator via deterministic dispatch.

User flow C — agent loop:

1. Worker process: `register-session --run R --role reviewer --lane gemini_pro_3_1`.
2. Worker: `claim-next --session SID` returns a JSON work packet or `204` (no
   work, optionally `--max-wait` blocks up to N seconds via condition wait).
3. Worker: `ack` to extend lease, `heartbeat` to keep the session alive, and
   eventually `complete` (with artifacts) or `verdict` (for review jobs) or
   `block`.
4. Process exits cleanly when role lifecycle ends (`release` followed by
   process exit), or `expire` if heartbeats lapse.

User flow D — coordinator chat:

1. Operator chats with the AI coordinator inside its terminal.
2. Operator says `read prompt P024` (or "what's blocked?", "what's next?").
3. Coordinator invokes coordinator skills, which are ordinary CLI subcommands
   under a coordinator-only allow list. Skills resolve prompts, assemble
   packets (without writing), propose branches, surface blockers, and
   request human checkpoints. They never write artifacts and never bypass
   gates.
4. Operator confirms or denies any state-changing action. Confirmation is
   itself a CLI invocation by the human (`checkpoint resolve`,
   `branch confirm`, `enqueue-job --confirm`).

User flow E — review/synthesis:

1. Reviewers run as fresh sessions, write `review` artifacts, and post a
   `verdict`.
2. On `accept`/`accept_with_findings`, the gate proceeds.
3. On `needs_revision`, the gate enters a bounded revision cycle (max
   iterations declared in workflow).
4. On `reject`, the gate raises a human checkpoint and stops downstream
   work. Any subsequent revision must come back through the same reviewer
   role for a re-review (per D029 / multi-agent review loop guidance).

Coordinator non-responsibilities (must enforce in CLI, not just prompt):

- No `publish-artifact` for non-coordinator jobs.
- No `complete` or `verdict` for jobs the coordinator session does not own.
- No git operations.
- No direct SQLite writes (this is a global rule for all sessions, not just
  the coordinator).
- No transcript capture commands.
- No spawning unregistered first-class sessions; native sub-agents inside the
  coordinator's CLI inherit the parent's identity per D021.

## 4. Workflow graph and review/synthesis gates

Graph shape (D014): mostly DAG, bounded cycles allowed.

Node kinds:

- `draft` — produces a primary artifact.
- `review` — emits a verdict and a review artifact. Defaults to
  `fresh_session_required=true`.
- `synthesis` — produces a synthesized artifact from upstream artifacts and
  verdicts. Always declared, never coordinator-default.
- `findings_ledger` — records verdicts + findings as a structured artifact.
- `human_checkpoint` — pauses the run, requires `checkpoint resolve`.
- `commit_request` — emits a message asking the human for commit intent. Does
  not run git.
- `revise` — bounded child of a `synthesis` or `draft` node, triggered by a
  `needs_revision` verdict.

Gate rules:

- Review gate evaluates verdicts on the joined set of upstream review
  packets. The gate is a deterministic function of:
  - all reviewer verdict records present,
  - none of them `reject`,
  - if any `needs_revision`, iteration count below `revision_cycle.max_iterations`,
  - same-model re-review present after each revision when the same model
    previously rejected.
- A second `reject` from the same reviewer role/lane terminates the cycle to
  a human checkpoint per D029.
- Parallel `review` jobs in the same `parallel_group` must have disjoint
  `write_scope.allow_paths` and unique `expected_artifacts.path` per D015.
- Branch gate is a one-shot human checkpoint at run start; nothing in the
  workflow runs `git switch` directly.

Bounded cycles: every cycle declares `max_iterations` and is rejected by the
validator if missing. Stop conditions are first-class workflow nodes, not
implicit.

## 5. Work packet and JSON workflow config shape

### 5.1 Work packet (JSON returned by `claim-next`)

```json
{
  "packet_id": "01J...uuidv7",
  "run_id": "01J...uuidv7",
  "job_id": "review_codex",
  "session_id": "01J...uuidv7",
  "workflow": { "id": "rfc_ledger_cleanup", "version": "1.0.0", "sha": "abc..." },
  "role": {
    "id": "reviewer",
    "definition_path": "examples/workflows/rfc_ledger_cleanup/roles/reviewer.md",
    "definition_sha": "..."
  },
  "context_docs": [
    { "id": "process_loop", "path": "docs/process/multi-agent-review-loop.md", "sha": "..." }
  ],
  "task_prompt": {
    "id": "review_rfc",
    "path": "examples/workflows/rfc_ledger_cleanup/prompts/review.md",
    "sha": "..."
  },
  "inputs": [
    { "kind": "rfc_draft", "artifact_id": "...", "path": "docs/rfc/01J.../draft.md", "sha": "..." }
  ],
  "expected_artifacts": [
    { "kind": "review", "path": "docs/reviews/01J.../review_codex.md", "required": true, "schema": "review.v1" }
  ],
  "write_scope": {
    "allow_paths": ["docs/reviews/01J.../"],
    "deny_paths": ["src/", "scripts/", "docs/rfc/"],
    "readonly_globs": ["**/*.lock", "**/.agent_runner/**"]
  },
  "completion_protocol": {
    "required_calls": ["publish-artifact", "verdict", "complete"],
    "verdict_required": true,
    "verdict_values": ["accept", "accept_with_findings", "needs_revision", "reject"]
  },
  "stop_conditions": [
    { "kind": "lease_exceeded" },
    { "kind": "write_scope_violation" },
    { "kind": "max_attempts", "value": 2 }
  ],
  "fresh_session_required": true,
  "session_policy": { "kind": "fresh_role" },
  "timeouts": {
    "lease_seconds": 1800,
    "heartbeat_interval_seconds": 60,
    "max_wallclock_seconds": 7200
  },
  "retries": { "max_attempts": 2, "current_attempt": 1 },
  "cycle": { "id": "review_revise", "iteration": 1, "max_iterations": 2 },
  "prior_messages": [],
  "assembled_at": "2026-05-06T...",
  "assembler_version": "agent_runner/0.1.0",
  "packet_sha": "..."
}
```

Notes:

- `prior_messages` is capped (default 32) and contains only structured
  messages (blockers, handoffs, verdicts) addressed to the role/session, not
  free-form transcripts.
- `packet_sha` is recorded so completion records can reference an immutable
  packet body even if the underlying assembled rows change.
- All paths are repo-relative and use the resolved `{run_id}` token.

### 5.2 JSON workflow config

Top-level shape (illustrative subset for RFC-ledger):

```json
{
  "$schema": "https://schemas.agent_runner.dev/workflow.v1.json",
  "format": "agent_runner.workflow.v1",
  "workflow": "rfc_ledger_cleanup",
  "version": "1.0.0",
  "branch": {
    "policy": "confirm_required",
    "default": "feature/rfc-ledger-{run_id}"
  },
  "roles": {
    "drafter": {
      "definition": "roles/drafter.md",
      "default_lane": "claude_opus",
      "default_session_policy": { "kind": "persistent_until_role_expires" }
    },
    "reviewer": {
      "definition": "roles/reviewer.md",
      "default_session_policy": { "kind": "fresh_role" }
    },
    "ledger_author": { "definition": "roles/ledger_author.md", "default_lane": "codex_gpt5_5" },
    "synthesizer":   { "definition": "roles/synthesizer.md",   "default_lane": "claude_opus" },
    "coordinator":   { "definition": "roles/coordinator.md",   "default_lane": "claude_opus",
                       "command_allowlist": ["read-prompt","status","why","propose-branch",
                                             "request-human-checkpoint","enqueue-job"] }
  },
  "lanes": {
    "claude_opus": {
      "adapter": "process_pty",
      "command": ["claude","--model","opus","--dangerously-skip-permissions"],
      "capabilities": ["chat","stdin_prompt"]
    },
    "codex_gpt5_5": {
      "adapter": "process_pty",
      "command": ["codex","-a","never","exec","--model","gpt-5.5",
                  "--sandbox","danger-full-access","-"],
      "capabilities": ["stdin_prompt"]
    },
    "gemini_pro_3_1": {
      "adapter": "process_pty",
      "command": ["gemini","--model","gemini-3.1-pro-preview","--yolo"],
      "capabilities": ["stdin_prompt"]
    }
  },
  "context_docs": [
    { "id": "process_loop", "path": "docs/process/multi-agent-review-loop.md" }
  ],
  "jobs": [
    {
      "id": "draft_rfc", "kind": "draft", "role": "drafter",
      "task_prompt": "prompts/draft.md", "context": ["process_loop"],
      "expected_artifacts": [
        { "kind": "rfc_draft", "path": "docs/rfc/{run_id}/draft.md", "required": true }
      ],
      "write_scope": { "allow_paths": ["docs/rfc/{run_id}/"] },
      "next": ["review_gemini","review_codex","review_claude"]
    },
    {
      "id": "review_gemini", "kind": "review", "role": "reviewer",
      "lane": "gemini_pro_3_1", "parallel_group": "rfc_reviews",
      "fresh_session_required": true, "verdict_required": true,
      "task_prompt": "prompts/review.md",
      "expected_artifacts": [
        { "kind": "review", "path": "docs/reviews/{run_id}/review_gemini.md", "required": true }
      ],
      "write_scope": { "allow_paths": ["docs/reviews/{run_id}/"] }
    },
    {
      "id": "findings_ledger", "kind": "findings_ledger", "role": "ledger_author",
      "fresh_session_required": true,
      "depends_on": ["review_gemini","review_codex","review_claude"],
      "task_prompt": "prompts/findings_ledger.md",
      "expected_artifacts": [
        { "kind": "findings_ledger", "path": "docs/reviews/{run_id}/findings.md", "required": true }
      ],
      "next": ["synthesis"]
    },
    {
      "id": "synthesis", "kind": "synthesis", "role": "synthesizer",
      "fresh_session_required": true, "task_prompt": "prompts/synthesis.md",
      "expected_artifacts": [
        { "kind": "synthesis", "path": "docs/reviews/{run_id}/synthesis.md", "required": true }
      ],
      "revision_cycle": {
        "id": "synth_revise",
        "trigger_verdicts": ["needs_revision"],
        "max_iterations": 2
      },
      "next": ["final_review"]
    },
    {
      "id": "final_review", "kind": "review", "role": "reviewer",
      "lane": "claude_opus", "fresh_session_required": true,
      "verdict_required": true, "stop_on_reject": true,
      "task_prompt": "prompts/final_review.md",
      "expected_artifacts": [
        { "kind": "review", "path": "docs/reviews/{run_id}/final_review.md", "required": true }
      ]
    }
  ],
  "stop_conditions": [
    { "kind": "review_reject", "scope": "final_review", "action": "human_checkpoint" }
  ]
}
```

Validator must enforce:

- `format` literal `agent_runner.workflow.v1`.
- All declared `lane`/`role` references resolve.
- All `parallel_group` siblings have disjoint `write_scope.allow_paths` and
  distinct `expected_artifacts.path`.
- Every cycle declares `max_iterations`.
- Templating tokens limited to `{run_id}`, `{job_id}`, `{role}`, `{lane}`,
  `{ordinal}`. No shell expansion.
- Reject any input whose extension is `.yaml`/`.yml` or whose first non-blank
  byte is not `{`.

## 6. Session lifecycle and fresh-context behavior

Session record fields (DB):

- `session_id` opaque UUIDv7.
- `slug` `<role>-<lane>-<ordinal>` for tmux/log/dashboard.
- `role_id`, `lane_id`, `run_id`, `parent_session_id` (for first-class
  registered sub-agents), `ordinal`.
- `capabilities` JSON (e.g., `["chat","stdin_prompt","write_tools"]`).
- `command_allowlist` JSON (coordinator and other restricted roles).
- `registered_at`, `last_heartbeat_at`, `terminated_at`, `terminate_reason`.
- `session_policy` enum: `persistent_until_role_expires`, `fresh_role`,
  `single_packet`.

Lifecycle states:

- `registered` → `active` (first ack/heartbeat) → `idle` (no in-flight
  packet) → `terminating` → `terminated`.
- `expired` if heartbeat older than `2 × heartbeat_interval_seconds` and a
  packet is in flight.

`claim-next` predicate (Claude lane recommendation):

- Match by `(run_id, role_id, lane_id, capabilities ⊇ packet.required_capabilities)`.
- Reject the match if the packet has `fresh_session_required=true` AND the
  session has previously claimed any packet for the same role+run. The
  caller must register a new session (new ordinal); `agent_runner run start`
  and the coordinator skill `enqueue-job` are responsible for triggering a
  fresh launch for fresh-required jobs.
- Reject the match if `session.terminated_at` is set.
- Tie-break by oldest packet `created_at` then job priority, then lexical
  job id.

Fresh-context semantics (D029):

- "Fresh context" = new role instantiation = new `session_id` and a new
  process. It does not mean clearing in-process memory; it means launching
  a new agent process.
- Reviews and builds remain persistent only when their role definition
  declares `default_session_policy=persistent_until_role_expires` AND the
  job does not set `fresh_session_required=true`.
- Reviewer role default is `fresh_role` per Claude lane recommendation, to
  preempt contamination across reviews of different drafts.

Native sub-agents (D021):

- Default: parent session is accountable. Sub-agents inherit
  `AGENT_RUNNER_SESSION_ID` from environment, and any CLI mutation they
  make is recorded against the parent.
- Opt-in first-class: a coordinator skill `register-session
  --parent-session-id PSID` registers the child as a separate session with
  its own queue claims, slug, and audit trail. Workflow jobs that require
  this declare `requires_first_class_subagent: true`.
- The CLI rejects mutation calls that present a session id with
  `terminated_at` set, and refuses to start a sub-agent registration whose
  parent is not `active` or `idle`.

AI coordinator session is treated as an ordinary registered session whose
role is `coordinator`. It is launched by the deterministic coordinator at
`run start`, not at workflow definition time. It receives a project-manager
prompt (D004/D005) and a `command_allowlist`. It is not assigned worker
packets unless the workflow explicitly enqueues a coordinator job (and then
the job's role must be `coordinator`).

## 7. SQLite/state-store recommendations

PRAGMAs at every connect: `journal_mode=WAL`, `foreign_keys=ON`,
`busy_timeout=5000`, `synchronous=NORMAL`.

Tables (concrete):

- `schema_version (version INTEGER PRIMARY KEY, applied_at TEXT)` — enforce
  monotonic upgrades.
- `runs (run_id PK, workflow_id FK, branch TEXT, started_at, finished_at,
  state CHECK in ('pending','awaiting_branch','running','blocked','succeeded','failed','aborted'),
  owner_user TEXT)`.
- `workflows (workflow_id PK, source_path, sha TEXT, version, raw_json TEXT,
  registered_at)`.
- `roles (role_id PK, run_id FK, name, definition_path, definition_sha,
  default_lane_id, default_session_policy TEXT, command_allowlist JSON)`.
- `lanes (lane_id PK, run_id FK, name, adapter TEXT, command_template JSON,
  capabilities JSON)`.
- `sessions (session_id PK, run_id FK, role_id FK, lane_id FK, slug UNIQUE
  per run, ordinal, parent_session_id NULL, capabilities JSON,
  command_allowlist JSON, session_policy TEXT, registered_at,
  last_heartbeat_at, terminated_at NULL, terminate_reason TEXT NULL)`.
- `jobs (job_id PK, run_id FK, workflow_node_id, kind, role_id FK, lane_id
  NULL, parallel_group TEXT NULL, depends_on JSON, expected_artifacts JSON,
  write_scope JSON, fresh_session_required INTEGER, retries_max INTEGER,
  attempt_count INTEGER DEFAULT 0,
  state CHECK in ('staged','ready','dispatched','blocked','succeeded','failed','skipped'),
  cycle_id TEXT NULL, cycle_iteration INTEGER NULL,
  current_packet_id NULL)`.
- `packets (packet_id PK, job_id FK, session_id NULL, packet_sha,
  body_json TEXT, created_at, claimed_at NULL, lease_expires_at NULL,
  released_at NULL, completed_at NULL,
  state CHECK in ('ready','claimed','released','completed','failed','expired'),
  attempt INTEGER NOT NULL)`.
- `messages (message_id PK INTEGER, run_id FK, kind CHECK in
  ('blocker','finding','verdict','handoff','human_checkpoint','system',
  'coordinator_proposal','decision','commit_request'),
  source_session_id NULL, target_session_id NULL, target_role NULL,
  body_json TEXT, ack_required INTEGER, acked_at NULL, created_at)`.
- `events (event_id INTEGER PK AUTOINCREMENT, run_id FK, ts, actor_session_id
  NULL, kind, ref_table, ref_id, body_json TEXT)` — append-only.
- `verdicts (verdict_id PK, job_id FK, packet_id FK, verdict TEXT CHECK in
  ('accept','accept_with_findings','needs_revision','reject'),
  reviewer_session_id FK, body_json, created_at)`.
- `artifacts (artifact_id PK, run_id FK, kind, repo_path, content_sha,
  registered_by_session_id FK, registered_at,
  supersedes_artifact_id NULL)` — `(run_id, repo_path)` UNIQUE for current
  rows; supersedes lets revisions register without losing provenance.
- `human_checkpoints (checkpoint_id PK, run_id FK, kind, body_json,
  requested_at, resolved_at NULL, resolution TEXT NULL,
  resolved_by_user TEXT NULL)`.
- `coordinator_proposals (proposal_id PK, run_id FK, kind, body_json,
  proposed_at, accepted_at NULL)` — coordinator skill output before
  state mutation.

Indexes:

- `packets(state, job_id, lease_expires_at)`,
- `jobs(state, run_id)`,
- `messages(run_id, kind, created_at)`,
- `events(run_id, ts)`,
- `artifacts(run_id, kind, repo_path)`,
- `sessions(run_id, role_id, lane_id, terminated_at)`.

Triggers / invariants:

- `events` blocks UPDATE/DELETE via a trigger raising
  `RAISE(ABORT, 'events is append-only')`.
- `packets` state transitions enforced by trigger or by application-layer
  CHECK before INSERT INTO events.
- Foreign keys enforced.
- `(run_id, slug)` and `(run_id, role_id, lane_id, ordinal)` UNIQUE on
  `sessions`.
- `(run_id, repo_path)` UNIQUE on `artifacts` for non-superseded rows.

Queue semantics (single SQLite writer per CLI invocation):

- `claim-next`: BEGIN IMMEDIATE; select eligible packet; UPDATE state to
  claimed, set `session_id`, `claimed_at`, `lease_expires_at`,
  `attempt += 1`; insert event `packet_claimed`; COMMIT; return body.
- `ack`: extend `lease_expires_at`, append event `packet_ack`.
- `release`: state→released, clear `session_id`, append event.
- `heartbeat`: update `sessions.last_heartbeat_at`. Does NOT extend lease;
  ack is the lease extender. (Keeps liveness and lease orthogonal.)
- `block`: insert messages row kind=blocker; packet stays claimed; gate
  evaluates and may emit a human_checkpoint.
- `complete`: validate all `required=true` expected artifacts are
  registered; state→completed; append event; trigger gate evaluation.
- `verdict`: insert verdicts row; trigger gate evaluation.
- Lease reaper: invoked at the start of every CLI mutation as a quick
  sweep — release any packets whose `lease_expires_at < now` and whose
  session also missed `last_heartbeat_at + 2*heartbeat_interval`. Append
  `packet_expired` event.

This avoids a daemon for v1. If contention bites, post-MVP can add a tiny
local Unix socket mediator without changing the agent contract.

## 8. CLI and adapter boundary recommendations

CLI shape:

- One entrypoint: `agent_runner <group> <verb> [...]`.
- Two groups: `admin` (operator) and mutations (no group prefix needed for
  agents). Aliases keep operator commands ergonomic
  (`agent_runner status` ≡ `agent_runner admin status`).
- Universal flags: `--json`, `--repo PATH` (defaults to cwd discovery),
  `--quiet`, `--session SID`, `--run RUN_ID`.

Mutation commands (D023 set, refined):

- `register-session --run R --role NAME --lane NAME [--ordinal N]
  [--parent-session-id PSID] [--policy fresh_role|persistent|single_packet]`
- `claim-next --session SID [--max-wait SEC] [--include-cycles]`
- `ack --session SID --packet PID`
- `release --session SID --packet PID --reason TEXT`
- `heartbeat --session SID`
- `block --session SID --packet PID --kind K --body @file.json`
- `complete --session SID --packet PID --body @file.json`
- `verdict --session SID --packet PID --value V --body @file.json`
- `publish-artifact --session SID --packet PID --kind K --path P
  --content @file [--supersede]`
- `send --session SID --kind K (--target-session TID|--target-role TR)
  --body @file.json`
- `read-prompt --session SID --prompt-id ID` (coordinator skill, returns
  resolved prompt + allowed context excerpts for chat)

Admin commands:

- `init [--repo .]`
- `run start --workflow PATH [--coordinator-lane LANE] [--owner USER]`
- `run resume --run R`
- `run abort --run R --reason TEXT`
- `status [--run R] [--json] [--watch]`
- `why <id>` — print event chain + state machine path; never modifies state.
- `doctor` — schema version, WAL state, hung leases, orphan sessions,
  missing markers/artifacts, branch sanity.
- `workflow validate PATH` — JSON Schema + cross-ref validator.
- `branch confirm --run R --branch NAME` — performs the `git switch -c`
  after explicit human invocation.
- `checkpoint resolve --id CID --resolution accept|reject [--note TEXT]`.
- `enqueue-job --run R --job NAME --confirm` — coordinator-proposed work
  insertion, requires explicit `--confirm` flag.

Output:

- Default human text. `--json` returns single-document JSON with stable keys
  (`{"ok":true,"event_id":...,"data":{...}}` on success;
  `{"ok":false,"error":{"code":"...","message":"...","details":{...}}}` on
  failure). Non-zero exit on failure.

Adapter boundary:

- `Adapter` = thin Python class with one method:
  `launch(lane: Lane, env: dict, cwd: str, stdin_prompt: bytes|None,
  pty: bool) -> ProcessHandle`.
- Two concrete adapters in MVP: `ProcessAdapter` (subprocess + optional
  `pty`), `TmuxAdapter` (wraps `ProcessAdapter` to attach the process
  inside a configured tmux window). No vendor-specific behavior.
- The adapter does NOT parse stdout. Workers communicate state back through
  `agent_runner` mutations. This is the cornerstone of model portability:
  swapping a lane only swaps the launch command, not protocol logic.
- Provider features (slash commands, MCP tool servers, hooks) live entirely
  inside lane `command` arrays the operator authors. Core makes no
  assumption about them.

## 9. Artifact publishing and branch confirmation behavior

Artifact policy (D016, D028):

- `publish-artifact` writes the file to its `repo_path`, computes
  `content_sha` (sha256), inserts an `artifacts` row, and records an
  event. Idempotent if the on-disk content already matches the recorded
  sha.
- Write-scope check: the path must canonicalize under one of the packet's
  `write_scope.allow_paths` and outside `deny_paths` and `readonly_globs`.
- Artifact kinds shipped in MVP: `rfc_draft`, `review`, `findings_ledger`,
  `synthesis`, `decision`, `marker`, `commit_request`, `handoff`,
  `human_checkpoint_resolution`.
- Decisions are written to `docs/decisions/D<NNN>.md`. The CLI provides a
  template via `read-prompt --prompt-id decision_template`. The decision
  log itself in `docs/DECISION_LOG.md` is updated by an explicit synthesis
  job (not by ad hoc agents) — this preserves the human-curated index.
- Markers are an ARTIFACT kind, not a control-plane state. They are
  optional: a workflow can declare `expected_artifacts.kind=marker` to
  produce a per-step marker file under
  `docs/reviews/<run_id>/markers/<NN>_<NAME>.md`, but the runner never
  reads them back as state. SQLite is authoritative.
- Transcripts: not captured by default. If an operator turns on diagnostic
  capture for a session, output goes to
  `.agent_runner/diagnostics/<session_id>.log`, gitignored, and never
  registered as an artifact.

Branch confirmation (D017, D026):

- At `run start`, runner inspects `git status --short` and current branch.
  If clean and on the desired branch (per workflow `branch.default` after
  templating), it can transition straight to `running` only when
  `branch.policy=auto_if_clean`. Default policy is `confirm_required`.
- Under `confirm_required`, the runner inserts a `human_checkpoint` row of
  kind `branch_proposal` and sets `runs.state=awaiting_branch`. No git
  operation executes.
- The human runs `agent_runner branch confirm --run R --branch NAME`. The
  runner now executes `git switch -c NAME` (or `git switch NAME` for an
  existing branch confirmed by the human) and resolves the checkpoint.
- If the working tree is dirty, the checkpoint body lists modified paths
  and the runner refuses to switch until clean or until the human accepts
  with `--allow-dirty`.
- No auto-commit, no auto-push. A `commit_request` artifact summarizes
  what the workflow believes is ready and the human commits manually.
  This MUST be enforced by absence of any git commit/push code path in the
  binary; a unit test greps the package for `git commit`/`git push` and
  fails the build if present outside specific exemption comments.

## 10. RFC-ledger validation fixture

Layout under `examples/workflows/rfc_ledger_cleanup/`:

```text
workflow.json
roles/drafter.md
roles/reviewer.md
roles/ledger_author.md
roles/synthesizer.md
roles/coordinator.md
prompts/draft.md
prompts/review.md
prompts/findings_ledger.md
prompts/synthesis.md
prompts/final_review.md
context/process_loop.md
context/project_judgment.md
README.md
```

Job graph: `draft_rfc → {review_gemini, review_codex, review_claude}
(parallel_group=rfc_reviews) → findings_ledger → synthesis →
final_review`, with bounded `synth_revise` cycle (`max_iterations=2`).

Test exercise (no live model calls; mocked sessions):

1. `agent_runner workflow validate examples/.../workflow.json` succeeds.
2. Equivalent YAML file with same content is rejected.
3. `agent_runner run start --workflow ...` puts run into
   `awaiting_branch`; no git mutation occurred.
4. After `branch confirm`, exactly three review packets are `ready` and the
   draft packet was already completed by a fixture pre-populated artifact.
5. Three reviewer sessions register on the three lanes; each `claim-next`
   returns a distinct packet whose `write_scope.allow_paths` is disjoint
   from the others. A fourth session for the same lane gets `204` because
   the packet is already claimed.
6. Each reviewer publishes a review artifact and posts a verdict.
7. Synthesis packet does not become ready until all three verdicts are
   present and none is `reject`. A previously-registered drafter session
   cannot claim the synthesis packet because `fresh_session_required=true`
   refuses session reuse for the same role/run.
8. A reviewer verdict of `needs_revision` triggers a revision cycle. The
   cycle attempts at most `max_iterations` times, after which the gate
   raises a human checkpoint.
9. A `reject` from `final_review` halts downstream work and records a
   human checkpoint with body summarizing the reviewer's findings.
10. `publish-artifact` for an identical content sha is a no-op event;
    different sha without `--supersede` is rejected.
11. `status --json` reflects every state transition; `why <packet_id>`
    returns a chain of events that matches the lifecycle.

The fixture proves the workflow shape; live model calls happen only inside
the actual P001 design pass that produces these artifacts.

## 11. Test strategy

Layers and what each enforces:

1. Schema/model unit tests (`tests/test_schema.py`):
   - workflow JSON Schema valid + invalid corpora.
   - YAML rejection (extension, content sniff, format field absence).
   - work packet round-trip (assemble → JSON → parse).
   - role / lane reference resolution.
   - templating tokens whitelist.

2. SQLite invariant tests (`tests/test_db.py`):
   - WAL/foreign keys/busy timeout on connect.
   - `events` UPDATE/DELETE blocked by trigger.
   - illegal packet transitions raise (`complete` before `claim`,
     `verdict` for non-review jobs, `publish-artifact` outside
     `write_scope`).
   - lease expiry sweeper releases stale packets and emits events.
   - `(run_id, slug)` and `(run_id, repo_path)` uniqueness.

3. CLI command tests (`tests/test_cli.py`):
   - every mutation command returns documented JSON shape.
   - non-zero exit on every state-machine violation.
   - `register-session` rejects unknown role/lane.
   - coordinator session `command_allowlist` denies `publish-artifact`,
     `verdict`, `complete` for non-coordinator jobs.

4. Workflow simulation tests (`tests/test_rfc_ledger.py`):
   - end-to-end fixture run with mocked sessions.
   - branch confirmation gate enforced.
   - parallel group disjoint write scope.
   - revision cycle bounded.
   - `reject` raises human checkpoint and stops work.

5. Concurrency tests (`tests/test_concurrency.py`):
   - two processes call `claim-next` for the same packet;
     exactly one wins, other gets `204`.
   - heartbeat liveness vs lease expiry are orthogonal.

6. Adapter contract tests (`tests/test_adapters.py`):
   - `ProcessAdapter` launches a stub subprocess and does NOT capture
     stdout to any tracked path.
   - `TmuxAdapter` falls back to `ProcessAdapter` when tmux is absent
     (and skips with a clear message in CI).

7. Repo-policy tests (`tests/test_policy.py`):
   - grep the package for `git commit`/`git push` and fail unless inside
     an explicitly exempted module path (currently none).
   - workflow JSON Schema does not mention transcript capture.

8. Smoke (Make target `make smoke`):
   - tmpdir → `agent_runner init` → `status --json` → `doctor`. Asserts
     exit 0 and presence of expected files.

Make targets: `make test`, `make lint`, `make smoke`. CI runs all three.

Coverage target: ≥85% line coverage on `src/agent_runner/`, with full
coverage of state-machine transitions and gate evaluator. Performance is
not an MVP gate; informational `claim-next` micro-bench under
`tests/perf/` is allowed but not required to pass.

## 12. Risks, blockers, and recommendations

Risks:

- R1. Coordinator drift to artifact author. Mitigation: enforce
  `command_allowlist` server-side in the CLI, not just in the prompt. Test
  that a coordinator session cannot publish a non-coordinator artifact or
  emit a verdict.
- R2. Marker-as-bus regression. Mitigation: code review checklist forbids
  reading marker files from core. SQLite is authoritative. Document this
  rule in `docs/SPEC.md` under non-negotiables.
- R3. Persistent-session contamination in reviews. Mitigation: reviewer
  role default is `fresh_role`; `fresh_session_required` is set at the job
  level for any review or synthesis where independence matters.
- R4. Sub-agent unaccountable mutations. Mitigation: every mutation
  requires `--session SID`. Sub-agents inherit the env var. Anonymous
  invocations exit non-zero with a clear error. Tests assert this.
- R5. Workflow JSON sprawl. Mitigation: ship a JSON Schema + validator that
  is stricter than the parser, and version the schema.
- R6. Auto-commit creep. Mitigation: package contains no git
  commit/push code path. Repo-policy test enforces this.
- R7. SQLite contention. Mitigation: WAL + `BEGIN IMMEDIATE` + short
  transactions. Lease reaper runs in-line. Defer a Unix-socket mediator
  to post-MVP.
- R8. Process group vs single PID confusion. Mitigation: the adapter
  tracks process groups (`os.setsid`) so model CLIs that fork internally
  remain bound to the parent session id.
- R9. Bootstrap harness leakage. The Engram tmux runner is an OK temporary
  one-shot helper but its marker pattern must not bleed into
  `agent_runner` core. Mitigation: explicit non-goal in `docs/SPEC.md`.
  The adapter ships from scratch.
- R10. Plugin marketplace temptation. Mitigation: lanes are workflow JSON
  declarations only. No discovery, no registry, no upgrade flow. Document
  this as a hard non-goal.
- R11. Hosted/cloud regression. Any future "lane that calls a hosted API"
  must remain a local CLI invocation. Mitigation: PRD / decision log
  language that adapters launch local processes; the validator can warn
  on lane commands containing `https://`.
- R12. Transcript capture creeping in for "debugging". Mitigation:
  diagnostics are gitignored, off by default, and never registered as
  artifacts.

Blocker (open question, Q030):

- Recommendation: yes, allow Engram's `scripts/phase3_tmux_agents.sh` to be
  reused as bootstrap orchestration for THIS one-shot's three-model design
  pass. No, do not import its marker-as-bus mental model into
  `agent_runner` core. Make this distinction explicit in `docs/SPEC.md`
  before synthesis.

Recommendations to synthesis:

- Adopt the work packet, workflow JSON, SQLite schema, and CLI command set
  in this document as authoritative drafts.
- Lock the coordinator command-allowlist as a CLI-level enforcement, not a
  prompt convention.
- Default reviewer roles to `fresh_role` and require explicit
  `default_session_policy=persistent_until_role_expires` to opt out.
- Ship the JSON Schema and `workflow validate` in MVP; refuse YAML at
  parse time AND at validate time.
- Treat all repo artifacts (markers, decisions, findings, syntheses) as
  one-way published outputs from SQLite.
- Build the RFC-ledger fixture as a hermetic simulation; live-model
  execution against the fixture is post-MVP.
- Ban `git commit`/`git push` in the package and assert it via test.
- Defer everything in section 2's "out of scope" list and label each item
  in `docs/SPEC.md` as `roadmap` rather than `gap`.
