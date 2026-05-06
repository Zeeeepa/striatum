# agent_runner

Local-first orchestration for multiple terminal-based AI coding agents.

`agent_runner` is a small, repo-local control plane for coordinating AI coding
agents that live in terminals: Codex, Claude Code, Gemini CLI, or any other
model runtime that can be represented as a command. It is built for workflows
where several agents need to draft, review, synthesize, repair, and report on
work without relying on a hosted coordinator or hidden chat transcripts.

The important distinction is this:

- `.agent_runner/state.sqlite3` is the authoritative live state for runs,
  jobs, sessions, queue messages, leases, blockers, verdicts, artifacts, and
  events.
- Repository files are durable provenance: prompts, findings, ledgers,
  syntheses, decisions, handoffs, markers, and redacted evidence exports.

Marker files, tmux pane state, terminal output, and provider hooks are useful
for humans, but they are not the live message bus.

This directory is temporarily incubated inside Engram because Engram supplied
the first real validation workflow and design pressure. The product boundary is
generic: `agent_runner` is intended to orchestrate terminal-agent workflows for
any repository. Engram is the reference customer and first fixture, not the
product scope. After MVP validation, this directory should split into a
standalone repository.

## Current Status

The V1 MVP is implemented as a Python CLI with no runtime dependencies outside
the standard library. It can:

- initialize repo-local SQLite state under `.agent_runner/`;
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
  leaving `.agent_runner/` ignored.

V1 deliberately does not launch or supervise production model processes yet.
The generic process/tmux adapter boundary is designed, and a temporary tmux
bootstrap script exists, but the tested core is the deterministic state,
workflow, work-packet, artifact, and review-gate contract.

## What It Is For

`agent_runner` is for long-running, review-heavy agent workflows where "just
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
humans move the workflow by calling `agent_runner` commands.

## Behavior Model

### Local State

`agent_runner init` creates `.agent_runner/state.sqlite3`, enables SQLite WAL
mode and foreign keys, and ensures `.agent_runner/` appears in `.gitignore`.
The state database is local working state, not a repo artifact to commit.

### Workflow Snapshots

Workflow files are JSON objects with schema version
`agent-runner.workflow.v1`. `run prepare` validates the file, stores a
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
- the path stays outside `.agent_runner/`;
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
  verdicts.

`evidence export` writes a redacted Markdown snapshot that can be committed for
review. It redacts free-text blocker descriptions and verdict rationales and
does not include the SQLite database or transcripts.

## Installation

From this incubation directory:

```bash
cd agent-runner
make install
.venv/bin/agent_runner --help
```

For quick development without installing the console script:

```bash
cd agent-runner
PYTHONPATH=src python3 -m agent_runner.cli --help
```

Run the tests with:

```bash
cd agent-runner
make test
```

## Usage Guide

The examples below assume you are in this `agent-runner` directory and want to
operate on some target repository. Set these once:

```bash
RUNNER=.venv/bin/agent_runner
TARGET_REPO=/path/to/target/repo
WORKFLOW=examples/rfc-ledger-cleanup/workflow.json
```

During incubation, the Engram repo happens to be the parent directory, so
`TARGET_REPO=..` works for the checked-in fixture. For another project, point
`TARGET_REPO` at that project and adapt the workflow's artifact paths to that
repo. The fixture writes under `docs/reviews/rfc-ledger/` in the target repo,
so use a scratch target if you only want to smoke-test the runner.

### 1. Initialize Runner State

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" status --json
"$RUNNER" --repo "$TARGET_REPO" doctor --json
```

This creates `.agent_runner/state.sqlite3` under the target repo and adds
`.agent_runner/` to that repo's `.gitignore`.

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
  --branch agent-runner/rfc-ledger-cleanup \
  --json
```

Then start the run:

```bash
"$RUNNER" --repo "$TARGET_REPO" run start \
  --run-id <run_id> \
  --json
```

Remember: V1 branch confirmation records intent and reports mismatches. Use git
yourself if you need to create or switch branches.

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

### 9. Report A Blocker

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

### 10. Inspect And Export Recovery Evidence

```bash
"$RUNNER" --repo "$TARGET_REPO" status --run-id <run_id> --json
"$RUNNER" --repo "$TARGET_REPO" why <blocker_or_job_or_artifact_id> --json
"$RUNNER" --repo "$TARGET_REPO" doctor --run-id <run_id> --json
```

To publish a redacted run snapshot:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id <run_id> \
  --path docs/reviews/rfc-ledger/RUN_EVIDENCE.md \
  --json
```

The export path must be inside the repository and outside `.agent_runner/`.

## Writing Workflows

Start from `examples/rfc-ledger-cleanup/workflow.json`.

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
  }
}
```

V1 records these constraints in work packets. It can enforce transcript-off for
the V1 process adapter, but network and repo-scope restrictions are advisory
unless the surrounding launcher/sandbox actually enforces them.

## Bootstrap Tmux Harness

The temporary design bootstrap runner is still available during Engram
incubation:

```bash
agent-runner/scripts/agent_runner_tmux_design.sh start
tmux attach -t agent-runner-design
```

Use `start-pipe` or `AGENT_RUNNER_RUN_MODE=pipe` when the local model CLIs are
ready to accept prompts on stdin. The harness starts Claude, Codex, and Gemini
design-input lanes plus a synthesis handoff pane.

This script is not the product control plane and should not be treated as
generic runner behavior. It exists to bootstrap the MVP design/build process
until the generic runner grows a real process/tmux adapter.

## Command Reference

Core lifecycle:

```text
agent_runner init
agent_runner workflow validate
agent_runner run prepare
agent_runner branch confirm
agent_runner run start
```

Agent/session work loop:

```text
agent_runner register-session
agent_runner claim-next
agent_runner ack
agent_runner heartbeat
agent_runner release
agent_runner send
agent_runner block
agent_runner publish-artifact
agent_runner complete
agent_runner verdict
agent_runner submit-review
```

Inspection and recovery:

```text
agent_runner status
agent_runner why
agent_runner doctor
agent_runner evidence export
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
- `8`: workflow config rejected.

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

Execution prompts:

- [prompts/P001_design_review_build_v1_mvp.md](prompts/P001_design_review_build_v1_mvp.md)
- [prompts/P002_validate_agent_runner_with_rfc_0014.md](prompts/P002_validate_agent_runner_with_rfc_0014.md)
- [prompts/P003_implement_rfc_0014_dogfood_fixes.md](prompts/P003_implement_rfc_0014_dogfood_fixes.md)
- [prompts/P004_rerun_rfc_0014_dogfood.md](prompts/P004_rerun_rfc_0014_dogfood.md)
