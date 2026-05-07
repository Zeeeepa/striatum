# striatum Specification

Status: implementation contract
Date: 2026-05-06

This specification binds the V1 MVP described in
`docs/design/V1_MVP_DESIGN.md` and synthesized in
`docs/reviews/v1/V1_MVP_SYNTHESIS.md`.

## Product Boundary

`striatum` V1 is a local Python CLI for orchestrating terminal-agent
workflow state inside one repository. It does not provide hosted services,
external persistence, telemetry, Slack, web, TUI, a long-running MCP server,
plugin marketplaces, or automatic commits.

The authoritative live state is SQLite under `.striatum/state.sqlite3`.
Repository artifacts are durable provenance only. Marker files, tmux panes,
terminal output, and provider hooks are never live control-plane state.

## State Store

`striatum init` creates `.striatum/`, initializes SQLite, enables WAL,
enforces foreign keys, and ensures `.striatum/` is ignored by git.

The schema includes:

- `schema_meta`
- `workflow_snapshots`
- `runs`
- `sessions`
- `jobs`
- `job_dependencies`
- `queue_messages`
- `leases`
- `work_packets`
- `artifacts`
- `verdicts`
- `blockers`
- `command_requests`
- `events`
- `job_worktrees`

`events` and artifact records are append-only. Mutations use short
`BEGIN IMMEDIATE` transactions and emit structured events.

Schema upgrades use SQLite's `PRAGMA user_version` together with a registered
migration list in `striatum.migrations`. `striatum init` and every connect to
an existing database apply pending migrations in version order inside a single
`BEGIN IMMEDIATE` transaction. A database whose `user_version` is higher than
the runner supports is refused with exit code 9.

## Workflow Config

Workflow config is JSON only. The validator rejects `.yaml` and `.yml` files
and rejects non-object JSON roots.

Required workflow fields:

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

The V1 schema version is `striatum.workflow.v1`.

The validator enforces unique job ids, resolved role/lane references, valid
edges, bounded cycles, repo-relative artifact paths, and declared parallelism
with disjoint write scopes or review-only unique artifact paths.

Lane configs may declare adapter constraints for network access, transcript
handling, and repository scope. The validator accepts only known constraint
names and values, and work packets expose both the requested constraint and the
adapter's recorded enforcement level. Lanes may also declare
`required_enforcement` for any declared constraint. Validation rejects a lane
when the adapter can only provide a weaker level than the workflow requires.

Workflows may declare `review_revision_policy` for root review
`needs_revision` verdicts. V1 supports the explicit
`root_review_needs_revision: "human_checkpoint"` policy for RFC-style workflows
that intentionally pause for human judgment instead of entering a revision
loop. `root_review_needs_revision: "declared_cycle"` is accepted only when each
root review job declares a matching `needs_revision` cycle.

## Sessions

Agents must call `register-session` before claiming work. Database identity is
an opaque `session_id`; human display uses `<role>-<lane>-<ordinal>` slugs.

Sessions match work by run, role, lane, and capabilities. Jobs can require
fresh sessions. Native sub-agents spawned inside an agent CLI inherit the
parent session unless explicitly registered as first-class sessions.

## Work Queue

`claim-next` lazily expires active leases, then atomically claims the oldest
eligible pending work message. It returns a structured work packet and stores
the packet JSON plus hash.

Required transition commands:

- `ack`
- `heartbeat`
- `release`
- `block`
- `complete`
- `verdict`
- `publish-artifact`
- `send`

Expired review-only leases can be requeued when attempts remain. Expired
repo-write leases become stale or blocked and require coordinator or human
inspection before requeue.

## Artifacts

Published artifacts are curated outputs: prompts, findings, ledgers,
syntheses, decisions, handoffs, markers, and test reports.

Owner choices can be recorded with `decision record`. The command writes a
durable Markdown artifact with YAML front matter using
`schema_version: striatum.decision.v1`, `artifact_kind: decision`, a stable
`decision_id`, `run_id`, `outcome`, `follow_up_required`, title, owner, and
creation timestamp. It records the file as a run-level artifact of kind
`decision` with no job, session, or active lease requirement, and emits a
`decision.recorded` event. Outcomes are `accepted`, `rejected`, and
`accepted_with_follow_up`; the follow-up outcome requires explicit follow-up
text.

Durable Markdown artifacts should include the work packet's privacy-safe
`author: <role-name>-<model-name>-<ordinal>` line in their title block when
one is provided.

`publish-artifact` validates file existence, repo-relative path, write scope,
artifact kind, and content hash. Transcript artifacts are rejected by default.
Markdown artifacts may include YAML front matter or title-block `author:`
metadata; when they do, the line must exactly match the work packet's lowercase
author line. The publisher still records artifacts rather than rewriting them.

`complete` and review `verdict` commands verify all required artifacts before
terminal job transition.

`submit-review` composes the common review path: it publishes the review
artifact, records the verdict, applies review-gate behavior, and returns the
artifact, verdict, blocker, run, and downstream state.

`evidence export` writes a redacted Markdown snapshot of run, job, blocker,
verdict, artifact, status, doctor, and downstream-blocking state. Export paths
must stay inside the repository and outside `.striatum/`; SQLite state is
not committed. Free-text fields that may contain agent or user prose, including
blocker descriptions and verdict rationales, are redacted in the export.
Workflow job titles are omitted by default; job and artifact authorship is
reported through stable identity metadata: role id, lane id, declared model
display name, and workflow job id.

Evidence redaction is **default-deny**. The export schema is defined by an
explicit per-field policy registry that classifies every emitted field as
`safe`, `redacted`, or `dropped`. Any field added to `evidence_snapshot()`,
`status()`, or `doctor()` that is not registered as `safe` is replaced with
the redaction placeholder. New fields cannot leak agent or user prose into a
committed export without an explicit, reviewable change to the registry.

Work packets expose an exact lowercase `author:` line for agents to place in
durable Markdown artifacts. This byline is distinct from evidence-export
identity metadata: exports keep stable role id, lane id, declared model display
name, and workflow job id; artifact files use the compact
`author: <role-name>-<model-name>-<ordinal>` convention so workflow job titles
or other project-specific prose do not leak into the artifact byline. The
artifact publisher records and validates artifact references; it does not
rewrite artifact files to insert headers.

### Artifact Front Matter Schemas

Durable Markdown artifacts may include an optional YAML-style `---`-delimited
front-matter block at the top of the file. When the artifact kind has a
registered schema and a front-matter block is present, `publish-artifact`
validates the parsed metadata against the schema. Files without a front-matter
block remain accepted as before; the publisher never rewrites artifact files.

Front-matter values are written as `key: <json-value>` lines so the parser is
unambiguous without adding a YAML dependency. Strings must be JSON-quoted,
booleans use `true` and `false`, integers and lists follow JSON syntax, and
nested mappings are not supported.

V1 schemas:

- `striatum.decision.v1` (kind `decision`): required `schema_version`,
  `artifact_kind: decision`, `decision_id`, `run_id`, `owner: human`,
  `outcome` (one of `accepted`, `rejected`, `accepted_with_follow_up`),
  `follow_up_required` (boolean), `title`, `created_at`.
- `striatum.finding.v1` (kind `finding`): required `schema_version`,
  `artifact_kind: finding`, and `verdict_intent` (one of `accept`,
  `accept_with_findings`, `needs_revision`, `reject`); optional `severity`
  (one of `info`, `low`, `medium`, `high`, `critical`) and `tags` (list of
  strings).
- `striatum.findings_ledger.v1` (kind `findings_ledger`): required
  `schema_version`, `artifact_kind: findings_ledger`, and `summary_count`
  (non-negative integer); optional `entries_path`. Ledger entries themselves
  are body content, not structured front matter.
- `striatum.synthesis.v1` (kind `synthesis`): required `schema_version` and
  `artifact_kind: synthesis`; optional `inputs` (list of logical-name
  strings).

Other artifact kinds (`prompt`, `marker`, `handoff`, `patch_summary`,
`test_report`, `other`) remain unschemaed in V1 and pass through without a
front-matter check.

## Branches And Commits

Workflow startup is confirmation-gated:

1. `run prepare` validates and snapshots workflow JSON and leaves the run in
   `needs_branch_confirmation`.
2. `branch confirm` records explicit human confirmation and optionally creates
   or selects a branch.
3. `run start` makes eligible root jobs claimable.

No job is claimable before branch confirmation. V1 does not commit, push,
merge, or rebase.

`branch confirm --json` is records-only by default: it includes the requested
branch and detected current git branch, warns when they differ, and reports
`records_only: true`. Three opt-in flags promote the gate from advisory to
git-enforcing:

- `--create`: run `git checkout -b <branch>` (idempotent fallback to
  `git checkout <branch>` if the branch already exists). If git refuses, the
  runner exits with `WorkflowError` (code 8) and does NOT record the
  confirmation. The response field `created` is `true` only when a new
  branch was created.
- `--use-current`: ignore `--branch` as a target and record the current git
  branch instead. If `--branch` is also given and disagrees with the
  current branch, exit with code 8.
- `--strict`: require that the current git branch matches `--branch`
  exactly before recording. If they differ, exit with code 8 and do not
  record. This is the safe default for CI and other automation.

The response also includes a `mode` field
(`"records_only" | "create" | "use_current" | "strict"`). The default
records-only mode preserves backwards compatibility for existing callers.

## CLI

Required commands:

```text
striatum init
striatum workflow validate
striatum workflow plan
striatum workflow graph
striatum workflow init
striatum run prepare
striatum branch confirm
striatum run start
striatum run summary
striatum register-session
striatum claim-next
striatum ack
striatum heartbeat
striatum release
striatum send
striatum block
striatum publish-artifact
striatum submit-review
striatum complete
striatum verdict
striatum evidence export
striatum decision record
striatum status
striatum why
striatum doctor
striatum recovery stale-leases
striatum recovery requeue-stale
striatum adapter run
striatum worktree create
striatum worktree release
striatum worktree list
```

Human read commands can pretty-print. `--json` returns stable machine-readable
JSON. Mutation commands support JSON output for agent use.

## Local API And MCP Wrapper Boundary

`striatum.api.invoke(args, repo=...)` is the minimal local Python API. It
parses the same command arguments as the CLI, calls the same dispatcher, and
returns the same JSON-style result envelope:

```json
{"ok": true, "data": {}}
```

Errors use the CLI's existing exit-code semantics:

```json
{"ok": false, "error": {"message": "...", "code": 3}}
```

This API is an adapter convenience only. It must not write SQLite directly,
reimplement workflow transitions, bypass artifact validation, or define a
separate command vocabulary.

The minimal local MCP-like wrapper exposes tools over line-delimited stdio
JSON-RPC. Each tool maps to an existing CLI command or `striatum.api.invoke`
call. MCP resources may expose read-only views such as status, `why`, doctor
output, or stored work packets. MCP remains optional and local; the CLI and
SQLite invariants are still the product contract.

`status --json` keeps aggregate run and job counts and also reports open
blockers, human checkpoints, latest non-accepting review verdicts, claimable
jobs grouped by role and lane, blocked downstream jobs, and deterministic
`next_actions`.

`workflow plan --json` validates a workflow and returns a dry-run plan with
claim waves, review gates, declared revision cycles, and graph nodes/edges.

`workflow graph <workflow.json>` validates a workflow and exports graph data
for authoring review. The default output is Mermaid `flowchart TD`, including
declared dependency edges, accepting-review gates, bounded revision-cycle
edges, and declared parallel groups. `--format json --json` returns stable
machine-readable graph data with nodes, edges, and cycles.

`why <id> --json` resolves run, job, queue message, blocker, artifact, verdict,
session, and process ids. Blocker introspection includes owning context,
related verdict when present, blocked downstream jobs, human-checkpoint context
when relevant, and next actions.

`run summary` writes a compact durable Markdown note with run id, branch
context (recorded plus current git branch with an explicit `(MISMATCH)`
annotation when they differ), run timing (`created_at`, `started_at`,
`completed_at`, and a wall-clock `duration`), job counts, verdicts grouped by
review job with attempt counts, artifacts annotated with structured author
bylines, blockers, and verification state.

`workflow init [--style minimal|review|code-change] <path>` writes a starter
workflow tree. The generated tree includes `<path>/workflow.json` plus role
and prompt stubs and validates cleanly with `workflow validate`. The default
`review` style mirrors the `examples/code-change-flow/` shape with placeholder
paths; `minimal` writes a single author job with no review; `code-change`
adds a one-shot `needs_revision` cycle. The command refuses to overwrite an
existing path.

`doctor [--verbose]` returns a stable string `problems` list by default. With
`--verbose` the payload also carries a `problem_records` list of structured
records with stable `check` names (e.g. `active_job_without_active_lease`,
`stale_queue_message_claim`, `worktree_path_missing_on_disk`), the affected
`id`, and a small `context` map. The string list is preserved verbatim so
callers that grep `problems` keep working.

`recovery stale-leases --json` applies lazy lease expiry for a run and reports
stale lease recovery context, explicitly distinguishing repo-write work that
requires manual inspection from review-only work that can be reclaimed safely.
`recovery requeue-stale --run-id <id> --job-id <id> --json` is a bounded
operator mutation for expired non-repo-write work only. It restores the job's
work message to `pending` when needed, reports when the work was already
reclaimable, and refuses repo-write jobs so abandoned write work still requires
manual inspection or a future worktree-isolated recovery path.

## Adapter Boundary

The minimum integration contract is process-based: command array, cwd, env,
stdin, stdout, stderr, exit code, and optional PTY/tmux wrapping. Provider
features live in lane command configuration. Core scheduling does not parse
terminal output or infer behavior from provider names.

`adapter run` is the minimal local process adapter. It launches the configured
`process` lane command for an active claimed lease, can pass the stored work
packet on stdin, sets `STRIATUM_*` environment variables, creates a
`.striatum/scratch/<process_id>` scratch directory, and records process
metadata plus lifecycle events in SQLite. Stdout and stderr are suppressed
unless the operator explicitly requests inherited stdio; Striatum does not
capture transcripts.

### Process Supervision

`adapter run` is single-shot: the child exits with the configured command,
and the next work packet must spawn a fresh process. Long-lived agent CLIs
(Codex, Claude Code, Gemini CLI, etc.) need a different shape: one
persistent process that receives multiple work packets across multiple
turns. RFC 0009 introduces a `striatum supervise` command group plus a new
`process_supervisors` table for that flow. The two adapter modes coexist;
`adapter run` is unchanged.

`process_supervisors` is added by migration version 4 and is separate from
`process_executions` so single-shot launches and supervised sessions keep
distinct rows and event streams. State values are
`('starting','attached','detached','lost','stopped')` and a partial unique
index on `session_id` enforces "at most one active supervisor per session"
without blocking historical `stopped` or `lost` rows.

The supervise CLI surface:

- `striatum supervise start --session-id <id>` validates the session is
  active and that its lane uses the `process` adapter, refuses if the
  session already has a supervisor in `('starting','attached','detached')`
  state, creates `.striatum/scratch/<supervisor_id>/stdin.pipe` via
  `os.mkfifo`, forks the lane command with `start_new_session=True`, sends
  stdout/stderr to `DEVNULL` (no transcripts, per D028), and transitions
  the row to `attached` once the child pid is alive. A
  `supervisor.starting` and `supervisor.started` event are recorded.
- `striatum supervise send --session-id <id> --packet-id <id>` looks up
  the stored work packet, writes its `packet_json` plus a trailing newline
  to the supervisor's named pipe, refreshes `heartbeat_at`, and records a
  `supervisor.packet_delivered` event with the byte count. The agent reads
  packets line-by-line from stdin; reactions remain CLI-driven (publish,
  ack, complete, verdict) so the supervisor never parses agent stdout.
- `striatum supervise stop --session-id <id> --reason <text>` sends
  `SIGTERM`, waits up to five seconds, falls back to `SIGKILL` if the
  process is still present, removes the FIFO, marks the row `stopped`,
  and records `supervisor.stopped`.
- `striatum supervise status --session-id <id>` probes liveness via
  `os.kill(pid, 0)`. An active row whose pid is gone is transitioned to
  `lost` with a `supervisor.lost` event before returning. Status itself
  never starts or kills processes.
- `striatum supervise list --run-id <id> [--state <state>]` lists rows
  for a run, optionally filtered by state.

Recovery: when a session's lease expires, `expire_leases` marks any
`attached` supervisor for that session as `lost` and records
`supervisor.lease_expired_with_supervisor`. The OS process is not
auto-killed; operator inspection is required, mirroring D036's stale-lease
policy for repo-write work.

Doctor: `striatum doctor` flags supervisors in `('starting','attached',
'detached')` whose pid is gone, and `attached` supervisors whose
`stdin_pipe_path` no longer exists on disk.

### Worktree Isolation

Lanes may opt into per-job filesystem isolation by setting
`worktree_isolation: "per_job"`. The default is `"off"`, which keeps current
single-worktree behavior. When a lane is configured for `per_job` isolation,
work packets for repo-write jobs in that lane include
`worktree_required: true` and a `commands.worktree_create` invocation. The
runner does not auto-create worktrees on claim; the agent must call
`striatum worktree create` itself.

`striatum worktree create --session-id ... --job-id ... --lease-id ...`
validates the active lease, requires the lane to declare `per_job` isolation,
requires the job to be repo-write, and rejects requests when an active
worktree already exists for the job. It runs
`git worktree add --detach .striatum/worktrees/<worktree_id> <base_branch>`
based on the run's confirmed branch and records a row in the new
`job_worktrees` table with state `active`. `striatum worktree release
--worktree-id <id>` runs `git worktree remove --force` and marks the row
`removed`; releasing an already-terminal row is a no-op. `striatum worktree
list [--run-id <id>]` returns the rows verbatim plus each job's
`workflow_job_id`.

`publish-artifact` continues to validate write scope and content against the
logical repo-relative path, but when an active worktree exists for the job it
reads the file from `<worktree_path>/<logical_path>` and records the
artifact's `repo_path` as the logical path. Artifacts remain durable
provenance for the main branch regardless of which worktree the work
happened in.

Lazy lease expiry preserves the worktree directory for operator inspection.
The `job_worktrees` row is marked `abandoned` and an event is emitted, but
`git worktree remove` is not run. `striatum doctor` flags active worktrees
whose lease is no longer active and active worktrees whose path no longer
exists on disk.

## First Validation Fixture

The first fixture is RFC-ledger cleanup:

```text
draft -> parallel reviews -> findings ledger -> synthesis -> final review
```

Tests exercise it with fake sessions and no live model calls.

A smaller generic docs-only workflow fixture also lives at
`examples/docs-review-flow/workflow.json`. It covers draft, review, and apply
steps without Engram-specific paths or live model requirements.

## Verification

The required check is:

```bash
make test
```

The smoke sequence is:

```bash
striatum init
striatum status --json
striatum doctor
```
