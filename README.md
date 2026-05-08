# striatum

Local-first orchestration for multiple terminal-based AI coding agents.

`striatum` is a small, repo-local control plane for coordinating AI coding
agents that live in terminals: Codex, Claude Code, Gemini CLI, or any other
model runtime that can be represented as a command. It is built for workflows
where several agents need to draft, review, synthesize, repair, and report on
work without relying on a hosted coordinator or hidden chat transcripts.

The important distinction is this:

- `.striatum/state.sqlite3` is the authoritative live state for runs, jobs,
  sessions, queue messages, leases, blockers, verdicts, artifacts, and events.
- Repository files are durable provenance: prompts, findings, ledgers,
  syntheses, decisions, handoffs, markers, and redacted evidence exports.

Marker files, tmux pane state, terminal output, and provider hooks are useful
for humans, but they are not the live message bus.

Striatum was incubated inside Engram, which supplied the first validation
workflow and design pressure. It is now a standalone project. Engram remains
the reference customer; the product boundary is generic.

## Current Status

The V1 MVP is implemented as a Python CLI with no runtime dependencies outside
the standard library. Today's surface includes:

- repo-local SQLite state (`.striatum/state.sqlite3`) with a forward-only
  migration system (`PRAGMA user_version`); a database newer than this install
  exits with code 9;
- JSON workflow validation (YAML rejected), workflow snapshots, dry-run
  planning, Mermaid/JSON graph export, and `workflow init` starter trees;
- branch confirmation that defaults to records-only and offers `--create`,
  `--use-current`, and `--strict` to actually drive git;
- agent sessions with opaque `session_id` plus `<role>-<lane>-<ordinal>` slugs;
- `claim-next` work packets with required artifacts, write scope, adapter
  constraints, and the exact follow-up commands the agent should run;
- leases, ack, heartbeat, release, block, complete, verdict, and
  `submit-review`, with declared-cycle revision routing and
  `human_checkpoint` fallback;
- artifact publishing with path/scope/kind validation, default-deny evidence
  redaction, and per-kind front-matter schemas for `decision`, `finding`,
  `findings_ledger`, and `synthesis` artifacts;
- owner decisions recorded as durable Markdown via `decision record`, no
  active lease required;
- introspection through `status`, `why`, and `doctor --verbose`
  (structured `problem_records`), plus `evidence export` for redacted
  Markdown snapshots;
- a compact terminal `dashboard` over the same SQLite state;
- stale-lease recovery (`recovery stale-leases`, `recovery requeue-stale`)
  that distinguishes review-only from repo-write work;
- a single-shot local process adapter (`adapter run`) and long-lived
  supervised sessions (`supervise start | send | stop | status | list`,
  RFC 0009);
- opt-in per-job git worktree isolation (RFC 0008) for parallel repo-write
  jobs, with `worktree create | release | list`;
- a four-level adapter constraint model (`enforced`, `advisory_strict`,
  `advisory`, `unsupported`) where the process adapter scrubs proxy env vars
  and sets `STRIATUM_NETWORK_POLICY` / `STRIATUM_REPO_SCOPE` sentinels for
  cooperating agents;
- a minimal local Python API (`striatum.api.invoke`) and a stdio JSON-RPC
  MCP-like wrapper with `Content-Length` framing.

## What It Is For

`striatum` is for long-running, review-heavy agent workflows where "just tell
three agents to work in tmux panes" stops being enough. It gives the human and
coordinator a stable answer to questions like:

- What run is active, on which branch was it confirmed, and which jobs are
  claimable, blocked, in review, or waiting on a human?
- Which session owns a lease? What artifact was required, where was it
  written, what hash was recorded?
- Did a review return `needs_revision`, and did the workflow declare a safe
  cycle for that?
- Can I commit a redacted evidence summary without committing live SQLite
  state or transcripts?

The runner is intentionally conservative. It coordinates work; it does not
decide that an agent is done because a terminal printed a phrase. Agents and
humans move the workflow by calling `striatum` commands.

## Behavior Model

### Local State

`striatum init` creates `.striatum/state.sqlite3`, enables SQLite WAL and
foreign keys, applies pending migrations under `PRAGMA user_version`, and adds
`.striatum/` to `.gitignore`. The state database is local working state, not a
repo artifact.

### Workflow Snapshots

Workflow files are JSON objects with schema version `striatum.workflow.v1`.
`run prepare` validates the file, stores a canonical JSON snapshot plus
SHA-256 hash, and creates a run in `needs_branch_confirmation`. Later edits to
the workflow file do not mutate an already prepared run.

### Branch Gate

No job is claimable until `branch confirm` records explicit human confirmation
and `run start` starts the run. By default `branch confirm` is records-only:
it writes the chosen branch and warns when the working tree is on a different
branch. Three opt-in flags promote the gate from advisory to git-enforcing:

- `--create`: run `git checkout -b <branch>`, falling back to
  `git checkout <branch>` if it already exists; on failure exits 8 and does
  NOT record. The response field `created` is true only when a new branch was
  created.
- `--use-current`: ignore `--branch` as a target and record the actual current
  branch. If `--branch` is given and disagrees, exit 8.
- `--strict`: require the current branch to match `--branch` exactly before
  recording; otherwise exit 8. Safe default for CI.

The response includes a `mode` field
(`records_only | create | use_current | strict`).

### Sessions And Work Packets

An agent registers a session for a run, role, and lane before claiming work.
`claim-next` matches pending work by run, role, lane, and freshness rules. A
successful claim creates a lease and returns a work packet containing run,
branch, session, lease, and job identifiers; role and context-doc references;
task prompt and inputs; write scope and forbidden paths; expected artifacts
(with privacy-safe author byline); follow-up commands for ack/heartbeat/
publish/block/verdict/complete; adapter constraints with declared and actual
enforcement levels; and, when the lane opts in, a `worktree_required: true`
flag plus the `worktree create` invocation.

Fresh-session jobs cannot be claimed by a session that has already received a
work packet in the run. Review fixtures use fresh sessions for independent
review by default.

### Leases And Recovery

Work is leased, not merely assigned. Agents `ack` after accepting a packet and
`heartbeat` during long work. Lease expiry is lazy: normal CLI mutations
expire stale leases rather than a background daemon. `recovery stale-leases`
reports expired-lease context and distinguishes repo-write from review-only
work. `recovery requeue-stale --run-id ... --job-id ...` requeues expired
non-repo-write work and refuses repo-write jobs so abandoned write work still
requires manual operator inspection.

### Artifacts

Artifacts are curated repo outputs. `publish-artifact` checks lease ownership,
file existence, repo-relative path, that the path stays outside `.striatum/`
and inside the job's write scope, that the artifact kind is allowed, and that
a logical name is not being reused for different content. Transcript artifacts
are rejected by default. Completion and review verdicts verify required
artifacts by logical name, kind, and path.

When the artifact kind has a registered front-matter schema (`decision`,
`finding`, `findings_ledger`, `synthesis`) and the file starts with a `---`
front-matter block, the publisher validates it against the schema. Files
without a front-matter block are still accepted; the publisher never rewrites
artifact files.

`decision record` writes a durable Markdown decision artifact (kind
`decision`, schema `striatum.decision.v1`) without claiming work or holding a
lease, with outcomes `accepted`, `rejected`, or `accepted_with_follow_up`.

### Review Gates

Review jobs use structured verdicts:

- `accept` completes the review and may unblock downstream work.
- `accept_with_findings` also completes the review and may unblock downstream
  work, while preserving the findings artifact.
- `needs_revision` follows a declared bounded cycle when one exists. If no
  safe cycle exists and the workflow declares a human-checkpoint policy, the
  runner opens a human checkpoint instead.
- `reject` fails the review job and can fail the run.

`submit-review` is the common shortcut: it publishes the review artifact and
records the verdict in one validated command.

### Worktree Isolation

Lanes may opt into per-job filesystem isolation by setting
`worktree_isolation: "per_job"` (default `"off"`). When opted in, work packets
for repo-write jobs in that lane include `worktree_required: true` and the
`striatum worktree create` invocation. The runner does not auto-create a
worktree on claim; the agent calls `worktree create` itself. `publish-artifact`
reads files from the active per-job worktree but records the artifact's
logical repo-relative path so artifacts remain valid main-branch provenance.
Lazy lease expiry marks the worktree row `abandoned` for operator inspection
without removing the directory. See RFC 0008.

### Process Supervision

`adapter run` is single-shot: the child exits with the configured command, and
the next packet must spawn a fresh process. RFC 0009 adds `striatum supervise
start | send | stop | status | list` for long-lived agent CLIs that span
multiple work packets. `supervise start` forks the lane command in a new
session and creates a per-supervisor named pipe at
`.striatum/scratch/<supervisor_id>/stdin.pipe`; `supervise send` delivers a
stored work packet as a newline-terminated JSON line through that pipe. Stdout
and stderr go to `DEVNULL` (no transcripts). Lease expiry transitions an
attached supervisor to `lost` without auto-killing the OS process; operator
inspection is required, mirroring the repo-write stale-lease policy.

### Introspection

Use `status`, `why`, `doctor`, and `dashboard` when a run becomes hard to
reason about:

- `status --json` reports runs, job counts, open blockers, human checkpoints,
  non-accepting verdicts, claimable work, blocked downstream jobs, and
  deterministic next actions.
- `why <id> --json` explains runs, jobs, queue messages, blockers, artifacts,
  verdicts, sessions, and processes.
- `doctor --json` flags state inconsistencies. `--verbose` adds a
  `problem_records` list with stable `check` names, affected ids, and small
  context maps for tooling that wants to act on specific failure kinds without
  parsing strings.
- `dashboard --run-id <id>` renders a compact, dependency-free terminal view
  of the same SQLite state.

`evidence export` writes a redacted Markdown snapshot. Redaction is
default-deny: any field not explicitly classified as `safe` in the per-field
policy registry is replaced with the redaction placeholder. The export does
not include SQLite state or transcripts.

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

The examples below assume you are in the Striatum checkout and want to operate
on some target repository. Set these once:

```bash
RUNNER=.venv/bin/striatum
TARGET_REPO=/path/to/target/repo
WORKFLOW=examples/rfc-ledger-cleanup/workflow.json
```

Point `TARGET_REPO` at the repository you want to orchestrate. The generic
fixture writes under `docs/reviews/rfc-ledger/` in the target repo, so use a
scratch target if you only want to smoke-test the runner.

### 1. Initialize Runner State

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" status --json
"$RUNNER" --repo "$TARGET_REPO" doctor --json
```

This creates `.striatum/state.sqlite3` under the target repo and adds
`.striatum/` to that repo's `.gitignore`.

### 2. Validate Or Scaffold A Workflow

```bash
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW" --format json --json
```

`workflow validate` checks required fields, role/lane references, artifact
paths, dependency edges, bounded cycles, declared parallelism, and lane
constraints. YAML files are rejected. `workflow plan` returns a dry-run plan;
`workflow graph` exports Mermaid `flowchart TD` (default), JSON, or
Graphviz DOT (`--format dot`, pipe through `dot -Tsvg` to render).

To scaffold a new workflow tree:

```bash
"$RUNNER" workflow init --style review path/to/new-flow
```

`--style` accepts `minimal`, `review` (default), or `code-change`. The
generated tree includes `workflow.json` plus `roles/` and `prompts/` stubs and
validates cleanly. The command refuses to overwrite an existing path.

### 3. Prepare A Run

```bash
"$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json
```

Copy the returned `run_id` for later commands. The run is now prepared but
not claimable.

### 4. Confirm The Branch And Start

```bash
"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id <run_id> \
  --branch striatum/rfc-ledger-cleanup \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id <run_id> --json
```

Add `--create`, `--use-current`, or `--strict` to drive git instead of just
recording (see Behavior Model > Branch Gate).

### 5. Register A Session

```bash
"$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id <run_id> \
  --role author \
  --lane codex \
  --capability write \
  --json
```

Copy the returned `session_id`. The display slug looks like `author-codex-1`.

### 6. Claim And Acknowledge Work

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id <session_id> --json
```

If work is available, the response includes a `packet` with `job_id`,
`message_id`, `lease_id`, expected artifacts, write scope, task prompt, and
the commands to use. Expected artifact metadata includes a privacy-safe
lowercase byline such as `author: author-codex-gpt-5.5-001`. Put that exact
line near the top of any workflow-authored Markdown artifact.

If the work packet contains `worktree_required: true`, run the suggested
`striatum worktree create` command before publishing.

After reading the packet:

```bash
"$RUNNER" ack \
  --session-id <session_id> --message-id <message_id> --lease-id <lease_id> \
  --json

"$RUNNER" heartbeat \
  --session-id <session_id> --lease-id <lease_id> --extend-seconds 1800 --json
```

### 7. Publish Artifacts And Complete Non-Review Work

```bash
"$RUNNER" publish-artifact \
  --session-id <session_id> --job-id <job_id> --lease-id <lease_id> \
  --kind handoff \
  --logical-name draft \
  --path docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md \
  --json

"$RUNNER" complete \
  --session-id <session_id> --job-id <job_id> --lease-id <lease_id> \
  --summary "Draft artifact published." --json
```

Completion may enqueue downstream jobs once dependencies are satisfied.

### 8. Submit Review Work

```bash
"$RUNNER" submit-review \
  --session-id <review_session_id> \
  --job-id <review_job_id> \
  --lease-id <review_lease_id> \
  --path docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md \
  --verdict accept_with_findings --json
```

`--verdict` accepts `accept`, `accept_with_findings`, `needs_revision`, or
`reject`. For unusual flows, call `publish-artifact` and `verdict` separately.

### 9. Record Owner Decisions

```bash
"$RUNNER" decision record \
  --run-id <run_id> \
  --path docs/decisions/owner-choice.md \
  --outcome accepted_with_follow_up \
  --title "Keep decisions as durable artifacts" \
  --follow-up "Review fuller decision schemas later." \
  --json
```

The generated Markdown includes machine-checkable front matter and is recorded
as artifact kind `decision`. `decision record` does not require an active
lease.

### 10. Report A Blocker

```bash
"$RUNNER" block \
  --session-id <session_id> --job-id <job_id> --lease-id <lease_id> \
  --kind missing_input \
  --severity human_checkpoint \
  --description "Need human decision before continuing." \
  --json
```

Use `--severity blocked` for normal blockers and `human_checkpoint` when the
run needs explicit human judgment.

To resolve a `human_checkpoint` blocker explicitly once the operator has
decided, use `striatum checkpoint resolve`:

```bash
# Continue: closes the blocker and returns the affected job to the queue.
"$RUNNER" --repo "$TARGET_REPO" checkpoint resolve \
  --blocker-id <blocker_id> \
  --action continue \
  --decision-id <decision_id> \
  --json

# Cancel: closes the blocker and cancels the affected job.
"$RUNNER" --repo "$TARGET_REPO" checkpoint resolve \
  --blocker-id <blocker_id> \
  --action cancel \
  --json
```

`--decision-id` is optional but recommended. When present, it must reference
an existing run-level decision artifact recorded with `striatum decision
record`; the resolution event payload then links back to that artifact for
audit.

### 11. Inspect, Watch, And Export Recovery Evidence

```bash
"$RUNNER" status --run-id <run_id> --json
"$RUNNER" why <blocker_or_job_or_artifact_id> --json
"$RUNNER" doctor --run-id <run_id> --verbose --json
"$RUNNER" dashboard --run-id <run_id>           # live; --once for one frame
"$RUNNER" run summary --run-id <run_id> --path docs/reviews/RUN_SUMMARY.md
"$RUNNER" evidence export \
  --run-id <run_id> --path docs/reviews/rfc-ledger/RUN_EVIDENCE.md --json
```

To explicitly cancel a non-terminal job (and optionally its blocked-only-
through-this dependents), use `striatum recovery cancel-job`:

```bash
"$RUNNER" --repo "$TARGET_REPO" recovery cancel-job \
  --run-id <run_id> \
  --job-id <job_id> \
  --reason "operator chose to abandon this work" \
  --cascade \
  --json
```

Without `--cascade` the command refuses with exit code 4 if the job has
blocked dependents whose only path was through it; rerun with `--cascade` to
cancel them transitively in the same transaction. Terminal-state jobs
(`completed`, `failed`, `canceled`, `skipped`) cannot be canceled.

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

Start from `examples/rfc-ledger-cleanup/workflow.json`. For smaller fixtures,
see `examples/docs-review-flow/`, `examples/code-change-flow/`,
`examples/failed-review-revision-cycle/`, `examples/human-checkpoint-flow/`,
and `examples/adapter-unavailable-flow/`.

Required top-level fields: `schema_version`, `workflow_id`,
`workflow_version`, `name`, `branch`, `coordinator`, `lanes`, `roles`,
`context_docs`, `parallelism`, `jobs`, `edges`, `cycles`.

Common job fields: `id`, `type`, `title`, `role_id`, optional `lane_id`,
`objective`, `task_prompt`, `inputs`, `write_scope` (`allowed_paths`,
`forbidden_paths`), `expected_artifacts` (`logical_name`, `kind`, `path`,
`required`), `fresh_session_required`, and `parallel_group`.

Lane configs may declare adapter constraints:

```json
{
  "constraints": {
    "network": "forbidden",
    "transcripts": "off",
    "repo_scope": "local_only"
  },
  "required_enforcement": {
    "network": "advisory_strict",
    "transcripts": "enforced"
  }
}
```

V1 records the requested constraint, the required enforcement level, the
adapter's actual enforcement (`enforced`, `advisory_strict`, `advisory`, or
`unsupported`), and satisfaction status in work packets. Validation rejects a
lane when `required_enforcement` asks for a level the adapter cannot provide.
The local process adapter enforces transcript-off, scrubs proxy env vars when
network is forbidden, and sets `STRIATUM_NETWORK_POLICY` and
`STRIATUM_REPO_SCOPE` sentinels so cooperating agents can honor the policy.

## Dogfood 001 Usage

`docs/dogfood/001/` is the first Striatum-on-Striatum dogfood scaffold. It
drives a small code change, adding Graphviz DOT export to `workflow graph`,
but the real purpose is to exercise the runner with real agent CLIs and
capture harness friction as durable `harness_improvement_proposal` artifacts.

For a human-run session, start with the runbook:

```bash
less docs/dogfood/001/RUNBOOK.md
```

For an agent handoff, give the agent the repo-local skill:

```text
Use the skill at /path/to/striatum/docs/dogfood/001/SKILL.md to start and drive dogfood-001.
```

Starting a dogfood run does not launch an interactive orchestrator chat. The
runner creates SQLite workflow state, makes jobs claimable, and can supervise
an agent process. Humans drive the run with `striatum` commands and watch it
through `dashboard`, `status`, `why`, and the artifacts the agents publish.

The shortest happy path is:

```bash
cd /path/to/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/001/workflow.json
TARGET_REPO=.

"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -40
```

Then follow the runbook to prepare the run, confirm
`striatum/dogfood-001-graph-dot`, register author and reviewer sessions,
start the author supervisor, claim packets, export evidence, and stop the
supervisor cleanly. If any step is awkward or surprising, capture it
immediately with `docs/dogfood/001/HARNESS_PROPOSAL_TEMPLATE.md`.

## Dogfood 003 Usage

`docs/dogfood/003/` scaffolds the RFC 0010 tool-harness-profile dogfood run:
verify the existing Codex, Claude Code, and Gemini CLI research, synthesize an
implementation design from the concrete profile candidates, review it, record
human acceptance, implement the first slice, and review the build.

For a human-run session:

```bash
less docs/dogfood/003/RUNBOOK.md
```

For an agent handoff:

```text
Use the skill at /path/to/striatum/docs/dogfood/003/SKILL.md to start and drive dogfood-003.
```

The workflow intentionally asks research agents to try native subagents or
equivalent delegation for independent research subtasks while keeping the
parent Striatum session accountable for final artifacts and state changes.
It also carries RFC 0010's proposed `harness_profiles` map as a fixture for
the implementation job to validate and expose in work packets.

## Bootstrap Tmux Harness

The temporary design bootstrap runner remains available for historical design
fixture work:

```bash
scripts/striatum_tmux_design.sh start
tmux attach -t striatum-design
```

This script is not the product control plane. It exists to bootstrap MVP
design/build work until the generic process adapter (`adapter run`) and
supervised sessions (`supervise`) cover that workflow end to end.

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

Agent / session work loop:

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

Worktree (opt-in per lane via `worktree_isolation: per_job`):

```text
striatum worktree create
striatum worktree release
striatum worktree list
```

Supervisor (RFC 0009):

```text
striatum supervise start
striatum supervise send
striatum supervise stop
striatum supervise status
striatum supervise list
```

Dashboard:

```text
striatum dashboard
```

List (read-only enumeration):

```text
striatum list runs
striatum list sessions
striatum list jobs
striatum list artifacts
striatum list workflows
```

Inspection and recovery:

```text
striatum status
striatum why
striatum doctor
striatum evidence export
striatum run graph
striatum recovery stale-leases
striatum recovery requeue-stale
striatum recovery cancel-job
striatum checkpoint resolve
```

`run graph --run-id <id> [--format mermaid|json]` renders the workflow graph
for an existing run with each node colored by current job state (Mermaid
`classDef`/`class` lines), or as machine-readable JSON with `current_state`,
`attempt`, and a `latest_verdict` block on review nodes.

Adapter:

```text
striatum adapter run
```

Stable exit codes:

- `0`: success, including `claim-next` with `no_work`;
- `1`: generic / unhandled runtime error;
- `2`: CLI usage error (argparse);
- `3`: missing run, session, job, message, blocker, artifact, verdict, or
  session target;
- `4`: invalid state transition;
- `5`: lease expiry or ownership mismatch;
- `6`: artifact or write-scope violation;
- `7`: branch confirmation required before work can be claimed;
- `8`: workflow config rejected (also raised by `branch confirm` when a
  requested git operation cannot be performed);
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
8. [docs/MCP.md](docs/MCP.md)
9. [docs/INTERVIEW_LOG.md](docs/INTERVIEW_LOG.md)
10. [docs/ENGRAM_INCUBATION_CONTEXT.md](docs/ENGRAM_INCUBATION_CONTEXT.md)

Historical incubation prompts live under `prompts/`. They are retained as
provenance from the Engram incubation and dogfood validation period; they are
not current standalone execution plans.
