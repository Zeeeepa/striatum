# P001: Design, Review, Record, And Build agent_runner V1 MVP

Status: draft
Date: 2026-05-06
Scope: `agent_runner` V1 MVP
Primary outcome: a reviewed, tested Python MVP for local terminal-agent
orchestration.

## Mission

You are the `agent_runner` build coordinator. Your job is to take this project
from PRD-level design into a small, tested V1 MVP.

You must follow the Engram-style process:

1. design the implementation,
2. review the design,
3. record findings,
4. synthesize accepted design deltas,
5. build the MVP,
6. get the build reviewed by independent reviewers,
7. record final decisions and known gaps.

Do not jump directly to code. The design and review artifacts are part of the
deliverable.

## Read First

Read these files in order:

1. `README.md`
2. `docs/PRD.md`
3. `docs/DECISION_LOG.md`
4. `docs/UBIQUITOUS_LANGUAGE.md`
5. `docs/PRIOR_ART.md`
6. `docs/ENGRAM_INCUBATION_CONTEXT.md`
7. `docs/SPEC.md`
8. this prompt

If running from inside the Engram repository, also read the Engram context files
listed in `docs/ENGRAM_INCUBATION_CONTEXT.md`.

Treat `docs/DECISION_LOG.md` as binding unless you discover a contradiction
that requires a new decision.

## Non-Negotiable Decisions

Preserve these accepted decisions:

- Product starts PRD-first and remains generic, not Engram-specific.
- Model portability is a core goal.
- Coordinator is hybrid:
  - deterministic coordinator owns control plane;
  - exactly one selected AI coordinator lane provides project-manager chat;
  - synthesis is an explicit job, not default coordinator behavior.
- SQLite is the local state store, append-only event log, and lightweight
  message queue.
- State lives under `.agent_runner/` in the target repo, ignored by default.
- Agents mutate state through the `agent_runner` CLI, not direct SQLite writes.
- Agents identify themselves through registered sessions before claiming work.
- Persistent sessions are preferred until role expiration; fresh context means
  new role instantiation.
- Agent slugs are human-readable `<role>-<model>-<ordinal>`; database identity
  uses opaque stable session ids.
- Workflow graph is mostly DAG-shaped with bounded retry/revision cycles.
- V1 parallelism is declared and write-scope safe; AI-inferred build
  parallelization is deferred.
- Build artifacts are durable and idempotent.
- Coordinator starts/selects a branch only after confirmation and requests
  commits from the human by default. This one-shot prompt is the explicit
  exception for branch setup: create or switch to branch `agent-runner` without
  asking again, then keep commit authority with the human.
- Python is the V1 implementation language.
- Roles are reusable artifacts; context docs are generic; task prompts may
  reference roles.
- Coordinator chat commands like "read prompt foo" are coordinator skills backed
  by deterministic CLI operations.
- Native sub-agents are internal to the parent session unless explicitly
  registered as first-class `agent_runner` sessions.
- Minimum common agent integration contract is process-based.
- Workflow config is JSON. Do not use YAML.
- Decisions, prompts, findings, syntheses, markers, and handoffs are durable
  repo artifacts. Do not capture or publish broad transcripts by default.
- First validation workflow is RFC-ledger cleanup.
- Emulate Engram's Python project discipline where appropriate.
- `agent_runner` is temporarily incubated inside Engram through MVP
  design/build, then split into a standalone project after validation.

## One-Shot Branch Setup

Before editing source or build files, set up the one-shot branch from the
Engram repository root:

1. Run `git status --short` if this directory is a git repo.
2. If the worktree has uncommitted changes you did not make, stop and report
   the dirty files instead of switching branches.
3. If already on `agent-runner`, continue.
4. If branch `agent-runner` exists, run `git switch agent-runner`.
5. Otherwise run `git switch -c agent-runner`.

Branch setup is already confirmed by this one-shot prompt. Design-lane panes
created by the bootstrap harness must not create or switch branches themselves;
the one-shot coordinator owns that step.

## Bootstrap Orchestration

Use the `agent_runner` bootstrap harness for the required design-input fan-out:

```bash
agent-runner/scripts/agent_runner_tmux_design.sh start
```

Use `start-pipe` instead when the local model CLIs are ready to accept prompts
on stdin:

```bash
agent-runner/scripts/agent_runner_tmux_design.sh start-pipe
```

This harness creates Claude, Codex, and Gemini design-input panes plus a
synthesis handoff pane. The watched completion artifacts are:

```text
agent-runner/docs/design/V1_MVP_DESIGN_INPUT_claude.md
agent-runner/docs/design/V1_MVP_DESIGN_INPUT_codex.md
agent-runner/docs/design/V1_MVP_DESIGN_INPUT_gemini.md
```

If you are already running inside one of those pane assignments, obey the pane
assignment and write only the lane-specific artifact. If you are the one-shot
coordinator, do not proceed to synthesis until all three artifacts exist.

Do not treat that bootstrap harness as product architecture. The `agent_runner`
product must still design and implement its own generic tmux/PTY adapter through
the reviewed MVP design.

## Design Team

This one-shot requires design input from all three frontier model lanes before
synthesis or implementation:

- Claude lane
- Codex lane
- Gemini lane

Use separate model sessions or native sub-agents wired to those models when
available. If a model lane is unavailable, stop and record the blocker instead
of silently substituting one model wearing multiple hats. The first MVP design
should not proceed to synthesis/build without three-lane design input.

Each lane must produce its own design note under:

```text
docs/design/V1_MVP_DESIGN_INPUT_claude.md
docs/design/V1_MVP_DESIGN_INPUT_codex.md
docs/design/V1_MVP_DESIGN_INPUT_gemini.md
```

The synthesis must cite all three input files and explain accepted, modified,
deferred, and rejected recommendations.

Required design roles:

- Product / workflow designer:
  - clarifies MVP boundary;
  - defines user flows;
  - protects against product sprawl.
- State / schema designer:
  - designs SQLite tables, indexes, transitions, leases, and queue semantics.
- CLI / adapter designer:
  - designs command surface, exit behavior, JSON output, tmux/process adapter
    boundaries, and promptable agent commands.
- Workflow / artifact designer:
  - designs JSON workflow shape, work packet schema, role/context/prompt
    assembly, and artifact publishing policy.
- Adversarial reviewer:
  - attacks portability, idempotence, safety, concurrency, context
    contamination, and hidden provider assumptions.

The parent agent remains accountable for all native sub-agent output, but the
three required design inputs must still come from the three model lanes.

## Required Design Artifacts

Create or update:

```text
docs/design/V1_MVP_DESIGN.md
docs/design/V1_MVP_DESIGN_INPUT_claude.md
docs/design/V1_MVP_DESIGN_INPUT_codex.md
docs/design/V1_MVP_DESIGN_INPUT_gemini.md
docs/reviews/v1/V1_MVP_DESIGN_REVIEW.md
docs/reviews/v1/V1_MVP_FINDINGS_LEDGER.md
docs/reviews/v1/V1_MVP_SYNTHESIS.md
```

The design must resolve at least:

- work packet schema;
- SQLite schema;
- mutation command set and argument shapes;
- read/status command set;
- JSON workflow config shape;
- role/context/task prompt assembly;
- persistent session and fresh-role behavior;
- queue claim, lease, ack, release, heartbeat, blocker, verdict, and completion
  semantics;
- event log semantics;
- artifact publisher behavior;
- one-shot branch setup behavior and the general confirmation-gated branch
  behavior after MVP;
- RFC-ledger validation workflow fixture;
- test strategy.

If the design creates new product or architecture decisions, record them in
`docs/DECISION_LOG.md`.

If the design introduces new terms or sharpens existing terms, update
`docs/UBIQUITOUS_LANGUAGE.md`.

After design synthesis, update `docs/SPEC.md` so it becomes an implementation
contract rather than a placeholder.

## Build Scope

Build a small but real Python MVP. Prefer boring dependencies. Avoid networked
services, hosted orchestration, telemetry, and plugin marketplaces.

Recommended project shape:

```text
pyproject.toml
Makefile
src/agent_runner/
  __init__.py
  cli.py
  db.py
  schema.py
  workflow.py
  packets.py
  artifacts.py
tests/
```

The design team may adjust this shape, but keep it simple.

Minimum MVP behavior should include:

- `agent_runner init`
  - creates `.agent_runner/`;
  - initializes SQLite;
  - ensures ignore handling is documented or applied safely.
- session registration
  - registers role, lane, ordinal/slug, capabilities, and run;
  - returns stable session id and slug.
- queue/work operations
  - enqueue or load workflow jobs;
  - `claim-next` returns an identity-aware work packet;
  - `ack`, `release`, `heartbeat`, `block`, `complete`, and `verdict` mutate
    state only through validated transitions.
- messaging/events
  - writes append-only events;
  - supports structured messages;
  - records blockers and human checkpoints.
- artifacts
  - records artifact references;
  - publishes selected durable Markdown artifacts;
  - does not capture broad transcripts by default.
- status/introspection
  - `status`;
  - `status --json`;
  - `why <job_or_message_id>`;
  - `doctor`.
- workflow config
  - accepts JSON workflow config;
  - rejects YAML;
  - validates declared parallelism/write scopes enough for MVP.
- tests
  - exercise init, registration, queue claim, lease/ack/release, completion,
    blocker, verdict, artifact registration, event log, and JSON status.

Do not build Slack, web dashboard, TUI, or MCP for V1 unless everything above
is already complete and tested. Leave those as documented future work.

## First Validation Fixture

Create an RFC-ledger cleanup workflow fixture. It does not need to fully run
external model CLIs in tests. It should prove that `agent_runner` can represent
the workflow:

```text
draft -> review -> findings ledger -> synthesis -> final review
```

Use JSON.

The fixture should demonstrate:

- reusable role definitions;
- generic context docs;
- task prompts;
- expected artifacts;
- declared review-only parallelism if useful;
- bounded revision/re-review path;
- durable decision/finding artifacts.

## Get The Build Reviewed

After implementation, do not review your own build. Get fresh independent build
reviews from the three model lanes, or stop and record a blocker if any lane is
unavailable. Reviewers may run in parallel if each writes only its own review
artifact.

Create:

```text
docs/reviews/v1/V1_MVP_BUILD_REVIEW_claude.md
docs/reviews/v1/V1_MVP_BUILD_REVIEW_codex.md
docs/reviews/v1/V1_MVP_BUILD_REVIEW_gemini.md
docs/reviews/v1/V1_MVP_BUILD_SYNTHESIS.md
```

Each build reviewer must check:

- accepted decisions are honored;
- SQLite schema supports required queue/event semantics;
- CLI commands enforce state transitions;
- direct SQLite writes are not part of the agent contract;
- workflow config is JSON;
- transcripts are not captured by default;
- persistent/fresh session policy is represented;
- tests cover core behavior;
- generated artifacts are idempotent where practical.

After all build reviews exist, synthesize their findings in
`docs/reviews/v1/V1_MVP_BUILD_SYNTHESIS.md`. If findings are accepted, fix them
and request any needed re-review before final response. The builder/coordinator
may synthesize findings and make fixes; it must not be the sole reviewer of its
own build.

## Verification

Run the project's relevant checks. At minimum:

```bash
make test
```

If `make test` is not available yet, create it or explain why an alternate
command is the MVP test command.

Also run a small CLI smoke sequence against a temporary directory if feasible:

```bash
agent_runner init
agent_runner status --json
agent_runner doctor
```

## Final Response Requirements

When complete, report:

- design artifacts written;
- review artifacts written;
- code modules created;
- tests run and result;
- known gaps intentionally deferred;
- whether branch `agent-runner` is ready for human commit.

Do not claim the MVP is complete if review findings remain unresolved or tests
were not run.
