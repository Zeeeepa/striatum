# agent_runner Specification

Status: implementation contract
Date: 2026-05-06

This specification binds the V1 MVP described in
`docs/design/V1_MVP_DESIGN.md` and synthesized in
`docs/reviews/v1/V1_MVP_SYNTHESIS.md`.

## Product Boundary

`agent_runner` V1 is a local Python CLI for orchestrating terminal-agent
workflow state inside one repository. It does not provide hosted services,
external persistence, telemetry, Slack, web, TUI, MCP, plugin marketplaces, or
automatic commits.

The authoritative live state is SQLite under `.agent_runner/state.sqlite3`.
Repository artifacts are durable provenance only. Marker files, tmux panes,
terminal output, and provider hooks are never live control-plane state.

## State Store

`agent_runner init` creates `.agent_runner/`, initializes SQLite, enables WAL,
enforces foreign keys, and ensures `.agent_runner/` is ignored by git.

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

`events` and artifact records are append-only. Mutations use short
`BEGIN IMMEDIATE` transactions and emit structured events.

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

The V1 schema version is `agent-runner.workflow.v1`.

The validator enforces unique job ids, resolved role/lane references, valid
edges, bounded cycles, repo-relative artifact paths, and declared parallelism
with disjoint write scopes or review-only unique artifact paths.

Lane configs may declare adapter constraints for network access, transcript
handling, and repository scope. The validator accepts only known constraint
names and values, and work packets expose both the requested constraint and the
adapter's recorded enforcement level.

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

`publish-artifact` validates file existence, repo-relative path, write scope,
artifact kind, and content hash. Transcript artifacts are rejected by default.

`complete` and review `verdict` commands verify all required artifacts before
terminal job transition.

`submit-review` composes the common review path: it publishes the review
artifact, records the verdict, applies review-gate behavior, and returns the
artifact, verdict, blocker, run, and downstream state.

`evidence export` writes a redacted Markdown snapshot of run, job, blocker,
verdict, artifact, status, doctor, and downstream-blocking state. Export paths
must stay inside the repository and outside `.agent_runner/`; SQLite state is
not committed. Free-text fields that may contain agent or user prose, including
blocker descriptions and verdict rationales, are redacted in the export.
Workflow job titles are omitted by default; job and artifact authorship is
reported through stable identity metadata: role id, lane id, declared model
display name, and workflow job id.

Work packets expose an exact `Author:` line built from the same identity tuple
for agents to place in durable Markdown artifacts. The artifact publisher
records and validates artifact references; it does not rewrite artifact files
to insert headers.

## Branches And Commits

Workflow startup is confirmation-gated:

1. `run prepare` validates and snapshots workflow JSON and leaves the run in
   `needs_branch_confirmation`.
2. `branch confirm` records explicit human confirmation and optionally creates
   or selects a branch.
3. `run start` makes eligible root jobs claimable.

No job is claimable before branch confirmation. V1 does not commit, push,
merge, or rebase.

`branch confirm --json` discloses that branch confirmation is records-only in
V1, includes the requested branch and detected current git branch, and warns
when they differ.

## CLI

Required commands:

```text
agent_runner init
agent_runner workflow validate
agent_runner run prepare
agent_runner branch confirm
agent_runner run start
agent_runner register-session
agent_runner claim-next
agent_runner ack
agent_runner heartbeat
agent_runner release
agent_runner send
agent_runner block
agent_runner publish-artifact
agent_runner submit-review
agent_runner complete
agent_runner verdict
agent_runner evidence export
agent_runner status
agent_runner why
agent_runner doctor
```

Human read commands can pretty-print. `--json` returns stable machine-readable
JSON. Mutation commands support JSON output for agent use.

`status --json` keeps aggregate run and job counts and also reports open
blockers, human checkpoints, latest non-accepting review verdicts, claimable
jobs grouped by role and lane, blocked downstream jobs, and deterministic
`next_actions`.

`why <id> --json` resolves run, job, queue message, blocker, artifact, verdict,
and session ids. Blocker introspection includes owning context, related verdict
when present, blocked downstream jobs, and next actions.

## Adapter Boundary

The minimum integration contract is process-based: command array, cwd, env,
stdin, stdout, stderr, exit code, and optional PTY/tmux wrapping. Provider
features live in lane command configuration. Core scheduling does not parse
terminal output or infer behavior from provider names.

## First Validation Fixture

The first fixture is RFC-ledger cleanup:

```text
draft -> parallel reviews -> findings ledger -> synthesis -> final review
```

Tests exercise it with fake sessions and no live model calls.

## Verification

The required check is:

```bash
make test
```

The smoke sequence is:

```bash
agent_runner init
agent_runner status --json
agent_runner doctor
```
