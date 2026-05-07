# striatum

Local-first orchestration for multiple terminal-based AI coding agents.

`striatum` is a small, repo-local control plane for coordinating AI coding
agents that live in terminals: Codex, Claude Code, Gemini CLI, or any other
model runtime that can be represented as a command. It is built for workflows
where several agents need to draft, review, synthesize, repair, and report on
work without relying on a hosted coordinator or hidden chat transcripts.

The important distinction is this:

- `.striatum/state.sqlite3` is the authoritative live state for runs,
  jobs, sessions, queue messages, leases, blockers, verdicts, artifacts, and
  events.
- Repository files are durable provenance: prompts, findings, ledgers,
  syntheses, decisions, handoffs, markers, and redacted evidence exports.

Marker files, tmux pane state, terminal output, and provider hooks are useful
for humans, but they are not the live message bus.

Striatum was incubated inside Engram because Engram supplied the first real
validation workflow and design pressure. It is now a standalone project. Engram
remains the reference customer and first fixture, but the product boundary is
generic: `striatum` is intended to orchestrate terminal-agent workflows for any
target repository.

## Current Status

The V1 MVP is implemented as a Python CLI with no runtime dependencies outside
the standard library. It can:

- initialize repo-local SQLite state under `.striatum/`;
- validate JSON workflow files and reject YAML;
- snapshot workflows into SQLite so a run is not silently changed by later file
  edits;
- require explicit branch confirmation before work becomes claimable;
- register agent sessions with opaque `session_id` values and human-readable
  `<role>-<lane>-<ordinal>` slugs;
- hand out identity-aware work packets through `claim-next`;
- enforce leases, acknowledgements, heartbeats, release, block, complete, and
  review-verdict transitions;
- validate artifact paths, artifact kinds, write scopes, and required
  artifacts before completion;
- keep `events` and artifact records append-only;
- route review gates through `accept`, `accept_with_findings`,
  `needs_revision`, and `reject`;
- create bounded revision attempts when a workflow declares a cycle;
- stop at a human checkpoint when review feedback has no declared safe route;
- export redacted Markdown evidence snapshots for commit and review while
  leaving `.striatum/` ignored.

V1 includes a minimal local process adapter for configured `process` lanes and
already-claimed work. It does not yet provide long-lived interactive
supervision, but the tested core is now the deterministic state, workflow,
work-packet, artifact, review-gate, and process launch contract.

## What It Is For

`striatum` is for long-running, review-heavy agent workflows where "just
tell three agents to work in tmux panes" stops being enough. It gives the human
and coordinator a stable answer to questions like:

- What run is active, and on which branch was it confirmed?
- Which jobs are claimable, blocked, waiting for review, or waiting for human
  judgment?
- Which agent session owns a lease?
- What artifact was required, where was it written, and what hash did the
  runner record?
- Why is a downstream job still blocked?
- Did a review return `needs_revision`, and did the workflow declare a safe
  cycle for that?
- Can I commit a redacted evidence summary without committing live SQLite
  state or transcripts?

The runner is intentionally conservative. It coordinates work; it does not
decide that an agent is done because a terminal printed a phrase. Agents and
humans move the workflow by calling `striatum` commands.

## Behavior Model

### Local State

`striatum init` creates `.striatum/state.sqlite3`, enables SQLite WAL
mode and foreign keys, and ensures `.striatum/` appears in `.gitignore`.
The state database is local working state, not a repo artifact to commit.

### Workflow Snapshots

Workflow files are JSON objects with schema version
`striatum.workflow.v1`. `run prepare` validates the file, stores a
canonical JSON snapshot and SHA-256 hash in SQLite, and creates a run in
`needs_branch_confirmation`. Later edits to the workflow file do not mutate an
already prepared run.

### Branch Gate

No job is claimable until `branch confirm` records explicit human confirmation
and `run start` starts the run. In V1, `branch confirm` is records-only: it
does not switch, create, merge, push, or commit branches. It reports the
requested branch, the detected current git branch, and a warning when they
differ.

### Sessions And Work Packets

An agent registers a session for a run, role, and lane before claiming work.
`claim-next` matches pending work by run, role, lane, and freshness rules. A
successful claim creates a lease and returns a work packet containing:

- run, branch, session, lease, and job identifiers;
- role definition path and context-doc references;
- task prompt reference and inputs;
- write scope and forbidden paths;
- expected artifacts;
- ready-to-use commands for ack, heartbeat, publish, block, verdict, and
  complete;
- adapter constraints such as network, transcript, and repo-scope policy, plus
  whether V1 can enforce them or only record them as advisory.

Fresh-session jobs cannot be claimed by a session that has already received a
work packet in the run. Review fixtures use fresh sessions for independent
review by default.

### Leases And Recovery

Work is leased, not merely assigned. Agents should `ack` after accepting the
packet and `heartbeat` during long work. Lease expiry is lazy: normal CLI
mutations expire stale leases rather than a background daemon. Expired
review-only work may be requeued; repo-write work is treated more cautiously
and becomes stale or blocked for coordinator/human inspection.

### Artifacts

Artifacts are curated repo outputs, not broad transcripts. `publish-artifact`
checks:

- the caller owns the active lease;
- the artifact file exists;
- the path is repo-relative;
- the path stays outside `.striatum/`;
- the path is inside the job write scope;
- the artifact kind is allowed;
- the logical name is not being reused for different content.

Transcript artifacts are rejected by default. Completion and review verdicts
verify required artifacts by logical name, kind, and path.

### Review Gates

Review jobs use structured verdicts:

- `accept` completes the review and may unblock downstream work.
- `accept_with_findings` also completes the review and may unblock downstream
  work, while preserving the findings artifact.
- `needs_revision` follows a declared bounded cycle when one exists. If no
  safe cycle exists and the workflow declares a human-checkpoint policy, the
  runner opens a human checkpoint instead.
- `reject` fails the review job and can fail the run.

`submit-review` is the common shortcut for review jobs: it publishes the review
artifact and records the verdict in one validated command.

### Introspection

Use `status`, `why`, and `doctor` when a run becomes hard to reason about:

- `status --json` reports runs, job counts, open blockers, human checkpoints,
  non-accepting review verdicts, claimable work, blocked downstream jobs, and
  deterministic next actions.
- `why <id> --json` explains runs, jobs, queue messages, blockers, artifacts,
  verdicts, and sessions.
- `doctor --json` checks common state inconsistencies such as active jobs
  without active leases or completed review dependencies without accepting
  verdicts. Pass `--verbose` to additionally emit a `problem_records` list
  with stable `check` names, affected ids, and small context maps for tooling
  that wants to act on specific failure kinds without parsing strings.

`evidence export` writes a redacted Markdown snapshot that can be committed for
review. It redacts free-text blocker descriptions and verdict rationales and
does not include the SQLite database or transcripts.

## Installation

From a checkout of this repository:

```bash
make install
.venv/bin/striatum --help
```

For quick development without installing the console script:

```bash
PYTHONPATH=src python3 -m striatum.cli --help
```

Run the tests with:

```bash
make test
```

## Usage Guide

The examples below assume you are in the Striatum checkout and want to
operate on some target repository. Set these once:

```bash
RUNNER=.venv/bin/striatum
TARGET_REPO=/path/to/target/repo
WORKFLOW=examples/rfc-ledger-cleanup/workflow.json
```

Point `TARGET_REPO` at the repository you want to orchestrate and adapt the
workflow's artifact paths to that repo. The generic fixture writes under
`docs/reviews/rfc-ledger/` in the target repo, so use a scratch target if you
only want to smoke-test the runner.

### 1. Initialize Runner State

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" status --json
"$RUNNER" --repo "$TARGET_REPO" doctor --json
```

This creates `.striatum/state.sqlite3` under the target repo and adds
`.striatum/` to that repo's `.gitignore`.

### 2. Validate A Workflow

```bash
"$RUNNER" --repo "$TARGET_REPO" workflow validate \
  "$WORKFLOW" \
  --json
```

The fixture workflow is:

```text
draft -> parallel reviews -> findings ledger -> synthesis -> final review
```

The validator checks required top-level fields, role/lane references, artifact
paths, dependency edges, bounded cycles, declared parallelism, and lane
constraints. YAML files are rejected.

For authoring reviews, export the workflow graph before preparing a run:

```bash
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW" \
  --format json \
  --json
```

The default graph output is a Mermaid `flowchart TD` that can be pasted into
Markdown renderers that support Mermaid. The JSON format returns the same
validated graph data used by the dry-run planner.

### 3. Prepare A Run

```bash
"$RUNNER" --repo "$TARGET_REPO" run prepare \
  --workflow "$WORKFLOW" \
  --json
```

Copy the returned `run_id` for later commands. The run is now prepared but not
claimable.

### 4. Confirm The Branch And Start

Confirm the branch that the human has chosen for this run:

```bash
"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id <run_id> \
  --branch striatum/rfc-ledger-cleanup \
  --json
```

Then start the run:

```bash
"$RUNNER" --repo "$TARGET_REPO" run start \
  --run-id <run_id> \
  --json
```

The default behavior is records-only: Striatum writes the chosen branch into
state, runs an advisory `git branch --show-current` check, and emits a
`warning` field if the working tree is on a different branch. Three opt-in
flags give actual git enforcement instead:

- `--create`: run `git checkout -b <branch>` from the target repo. If the
  branch already exists, fall back to `git checkout <branch>` so the call is
  idempotent. If git refuses (for example, a dirty working tree blocking
  checkout), the runner exits with code 8 and does NOT record the
  confirmation. The response field `created` is `true` only when the
  checkout actually created a new branch.
- `--use-current`: ignore `--branch` as a target; read the current git
  branch and record THAT. Useful when the human already created the branch
  manually. If `--branch` is also given and disagrees with the current
  branch, Striatum exits with code 8 — that disagreement is a user mistake
  worth catching.
- `--strict`: require that the current git branch matches `--branch`
  exactly before recording. If they differ, exit with code 8 and do not
  record. This is the safe default for CI and other automation that should
  refuse to run if the working tree is misaligned.

The default (no flags) is unchanged from earlier releases for backwards
compatibility. The response includes a `mode` field
(`"records_only" | "create" | "use_current" | "strict"`) and a `created`
boolean alongside the existing fields.

### 5. Register A Session

Each agent or human acting as a role needs a session:

```bash
"$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id <run_id> \
  --role author \
  --lane codex \
  --capability write \
  --json
```

Copy the returned `session_id`. The display slug will look like
`author-codex-1`.

### 6. Claim And Acknowledge Work

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next \
  --session-id <session_id> \
  --json
```

If work is available, the response contains a `packet` with `job_id`,
`message_id`, `lease_id`, expected artifacts, write scope, task prompt, and the
commands the agent should use.

Expected artifact metadata includes a privacy-safe lowercase byline such as
`author: author-codex-gpt-5.5-001`. Put that exact line near the top of
workflow-authored Markdown artifacts; do not derive bylines from workflow job
titles.

After reading the packet and accepting the job:

```bash
"$RUNNER" --repo "$TARGET_REPO" ack \
  --session-id <session_id> \
  --message-id <message_id> \
  --lease-id <lease_id> \
  --json
```

For long jobs:

```bash
"$RUNNER" --repo "$TARGET_REPO" heartbeat \
  --session-id <session_id> \
  --lease-id <lease_id> \
  --extend-seconds 1800 \
  --json
```

### 7. Publish Artifacts And Complete Non-Review Work

Write the artifact required by the work packet, then publish it:

```bash
"$RUNNER" --repo "$TARGET_REPO" publish-artifact \
  --session-id <session_id> \
  --job-id <job_id> \
  --lease-id <lease_id> \
  --kind handoff \
  --logical-name draft \
  --path docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md \
  --json
```

Complete the job after all required artifacts are published:

```bash
"$RUNNER" --repo "$TARGET_REPO" complete \
  --session-id <session_id> \
  --job-id <job_id> \
  --lease-id <lease_id> \
  --summary "Draft artifact published." \
  --json
```

Completion may enqueue downstream jobs when dependencies are satisfied.

### 8. Submit Review Work

For a review job, the shortest path is `submit-review`:

```bash
"$RUNNER" --repo "$TARGET_REPO" submit-review \
  --session-id <review_session_id> \
  --job-id <review_job_id> \
  --lease-id <review_lease_id> \
  --path docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md \
  --verdict accept_with_findings \
  --json
```

Use `--verdict accept`, `accept_with_findings`, `needs_revision`, or `reject`.
For unusual flows, you can still call `publish-artifact` and `verdict`
separately.

### 9. Record Owner Decisions

Owner choices that affect a run can be written as durable decision artifacts
without claiming work or holding an active lease:

```bash
"$RUNNER" --repo "$TARGET_REPO" decision record \
  --run-id <run_id> \
  --path docs/decisions/owner-choice.md \
  --outcome accepted_with_follow_up \
  --title "Keep decisions as durable artifacts" \
  --follow-up "Review fuller decision schemas later." \
  --json
```

The generated Markdown includes machine-checkable front matter and is recorded
as artifact kind `decision`.

### 10. Report A Blocker

If an agent cannot proceed:

```bash
"$RUNNER" --repo "$TARGET_REPO" block \
  --session-id <session_id> \
  --job-id <job_id> \
  --lease-id <lease_id> \
  --kind missing_input \
  --severity human_checkpoint \
  --description "Need human decision before continuing." \
  --json
```

Use `--severity blocked` for normal blockers and `human_checkpoint` when the
run needs explicit human judgment.

### 11. Inspect And Export Recovery Evidence

```bash
"$RUNNER" --repo "$TARGET_REPO" status --run-id <run_id> --json
"$RUNNER" --repo "$TARGET_REPO" why <blocker_or_job_or_artifact_id> --json
"$RUNNER" --repo "$TARGET_REPO" doctor --run-id <run_id> --json
```

### Dashboard

For a compact at-a-glance view of a run, use the dashboard. It is a
dependency-free terminal renderer over the same SQLite state that `status`
exposes:

```bash
"$RUNNER" --repo "$TARGET_REPO" dashboard --run-id <run_id>
```

The dashboard refreshes every 2 seconds by default and clears the screen
between frames. It shows run state and branch, job counts by state, verdict
counts, open blockers (including human checkpoints), claimable work grouped by
role/lane, deterministic next actions, and the most recent events. Use
`Ctrl-C` to quit.

Useful flags:

- `--refresh <seconds>`: change the refresh cadence.
- `--once`: render a single frame to stdout and exit. Handy in scripts and CI
  assertions where a redrawing TUI is not what you want.

To publish a redacted run snapshot:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id <run_id> \
  --path docs/reviews/rfc-ledger/RUN_EVIDENCE.md \
  --json
```

The export path must be inside the repository and outside `.striatum/`.

## Writing Workflows

Start from `examples/rfc-ledger-cleanup/workflow.json`. For a smaller generic
docs-only fixture, see `examples/docs-review-flow/workflow.json`.

Required top-level fields:

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

Common job fields:

- `id`, `type`, `title`, `role_id`, and optional `lane_id`;
- `objective` and `task_prompt`;
- `inputs`;
- `write_scope` with `allowed_paths` and `forbidden_paths`;
- `expected_artifacts` with `logical_name`, `kind`, `path`, and `required`;
- `fresh_session_required` when independent context matters;
- `parallel_group` only when declared parallel work has unique artifacts or
  disjoint write scopes.

Lane configs may declare adapter constraints:

```json
{
  "constraints": {
    "network": "forbidden",
    "transcripts": "off",
    "repo_scope": "local_only"
  },
  "required_enforcement": {
    "network": "advisory",
    "transcripts": "enforced"
  }
}
```

V1 records requested constraints, required enforcement levels, actual adapter
enforcement, and satisfaction status in work packets. Workflow validation
rejects a lane when `required_enforcement` asks for a stronger guarantee than
the adapter can provide. The local process adapter enforces transcript-off by
default; network and repo-scope restrictions remain advisory unless a
surrounding launcher or sandbox enforces them.

## Bootstrap Tmux Harness

The temporary design bootstrap runner remains available for historical design
fixture work:

```bash
scripts/striatum_tmux_design.sh start
tmux attach -t striatum-design
```

Use `start-pipe` or `STRIATUM_RUN_MODE=pipe` when the local model CLIs are
ready to accept prompts on stdin. The harness starts Claude, Codex, and Gemini
design-input lanes plus a synthesis handoff pane.

This script is not the product control plane and should not be treated as
generic runner behavior. It exists to bootstrap the MVP design/build process
until the generic runner can represent that workflow end to end.

The runner now has a minimal generic process adapter through `adapter run`.
It launches configured local `process` lane commands for already-claimed work,
passes the work packet on stdin by default, records process metadata/events in
SQLite, and does not capture transcripts.

## Command Reference

Core lifecycle:

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
```

`workflow init [--style minimal|review|code-change] <path>` writes a starter
workflow tree (`workflow.json` plus `roles/` and `prompts/` stubs). The
`review` default mirrors the `examples/code-change-flow/` shape with
placeholder paths and validates cleanly with `striatum workflow validate`.
`minimal` writes a single author job and `code-change` adds a one-shot
`needs_revision` cycle. The command refuses to overwrite an existing path.

Agent/session work loop:

```text
striatum register-session
striatum claim-next
striatum ack
striatum heartbeat
striatum release
striatum send
striatum block
striatum publish-artifact
striatum complete
striatum verdict
striatum submit-review
striatum decision record
```

Inspection and recovery:

```text
striatum status
striatum why
striatum doctor
striatum evidence export
<<<<<<< HEAD
striatum recovery stale-leases
striatum recovery requeue-stale
striatum adapter run
```

Per-job worktree isolation (opt-in per lane via `worktree_isolation: per_job`):

```text
striatum worktree create
striatum worktree release
striatum worktree list
```

Long-lived process supervision (RFC 0009; see `docs/SPEC.md` Process
Supervision):

```text
striatum supervise start
striatum supervise send
striatum supervise stop
striatum supervise status
striatum supervise list
=======
striatum dashboard
>>>>>>> aacbeef (Add compact terminal dashboard over SQLite state)
```

Stable exit codes:

- `0`: success, including `claim-next` with `no_work`;
- `2`: CLI usage error;
- `3`: missing run, session, job, message, blocker, artifact, verdict, or
  session target;
- `4`: invalid state transition;
- `5`: lease expiry or ownership mismatch;
- `6`: artifact or write-scope violation;
- `7`: branch confirmation required before work can be claimed;
- `8`: workflow config rejected;
- `9`: local SQLite schema is newer than this Striatum install supports.

## Documentation Map

Start with:

1. [docs/README.md](docs/README.md)
2. [docs/PRD.md](docs/PRD.md)
3. [docs/DECISION_LOG.md](docs/DECISION_LOG.md)
4. [docs/UBIQUITOUS_LANGUAGE.md](docs/UBIQUITOUS_LANGUAGE.md)
5. [docs/PRIOR_ART.md](docs/PRIOR_ART.md)
6. [docs/rfcs/](docs/rfcs/)
7. [docs/SPEC.md](docs/SPEC.md)
8. [docs/INTERVIEW_LOG.md](docs/INTERVIEW_LOG.md)
9. [docs/ENGRAM_INCUBATION_CONTEXT.md](docs/ENGRAM_INCUBATION_CONTEXT.md)

Historical incubation prompts:

These prompts are retained as provenance from the Engram incubation and dogfood
validation period. They are useful templates and history, not current
standalone execution plans.

- [prompts/README.md](prompts/README.md)
- [prompts/P001_design_review_build_v1_mvp.md](prompts/P001_design_review_build_v1_mvp.md)
- [prompts/P002_validate_striatum_with_rfc_0014.md](prompts/P002_validate_striatum_with_rfc_0014.md)
- [prompts/P003_implement_rfc_0014_dogfood_fixes.md](prompts/P003_implement_rfc_0014_dogfood_fixes.md)
- [prompts/P004_rerun_rfc_0014_dogfood.md](prompts/P004_rerun_rfc_0014_dogfood.md)
