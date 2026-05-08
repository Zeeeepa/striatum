# Getting Started

This guide walks you from a fresh target repository to a running
striatum workflow in about 15 minutes. It picks one path and follows
it; for alternatives, see [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) and
[HOW_TO_AGENT.md](HOW_TO_AGENT.md).

## Are you the operator, or are you setting up an agent?

striatum has two natural entry points and they answer different
questions. Pick one before you start.

- **I am driving the workflow myself.** You will run `striatum`
  commands by hand: register a session, claim work, publish
  artifacts, complete jobs. Continue with the **Operator path**
  below; then read [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md).
- **I am setting up a coding agent (Claude Code, Codex, Gemini)
  to drive the workflow.** You will install the runner, install
  the *skill bundle* (RFC 0015), and hand the agent a target repo
  with a workflow file in it. The agent reads the bundle and
  drives the run. Continue with the **Agent path** below; then
  read [HOW_TO_AGENT.md](HOW_TO_AGENT.md).

You can switch later. The runner is the same in both cases; the
only difference is who calls the CLI verbs.

## Prerequisites

- Python 3.11+ on `PATH`.
- A target repository (the one you want striatum to orchestrate).
  This is *not* the striatum source repository unless you are
  dogfooding striatum on itself.
- `git` available.

## Install striatum

From a checkout of the striatum repository:

```bash
make install
.venv/bin/striatum --help
```

For development without installing the console script:

```bash
PYTHONPATH=src python3 -m striatum.cli --help
```

For the rest of this guide, `striatum` refers to either invocation.

## Operator path

You will run striatum yourself.

```bash
TARGET_REPO=/path/to/your/repo
WORKFLOW=examples/code-change-flow/workflow.json   # or your own

striatum --repo "$TARGET_REPO" init --json
striatum --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
striatum --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json
# copy the run_id from the response
striatum --repo "$TARGET_REPO" run start --run-id <run_id> --json
striatum --repo "$TARGET_REPO" dashboard --run-id <run_id> --once
```

The dashboard prints a single frame to stdout: run state, job
counts, claimable work, recent events. From here you register a
session and claim work — see
[HOW_TO_HUMAN.md § "Register a session"](HOW_TO_HUMAN.md#register-a-session).

## Agent path

You will install the runner once, install the skill bundle, and
hand the agent your target repo. The agent does the rest.

```bash
TARGET_REPO=/path/to/your/repo
striatum --repo "$TARGET_REPO" init --with-skills claude_code --json
```

That single command does two things:

1. Initializes `.striatum/` in the target repo.
2. Writes the RFC 0015 agent skill bundle into the target repo at
   `.claude/skills/striatum-*/`. The bundle teaches a Claude Code
   session how to drive the runner without reading the striatum
   source.

For agents that don't have a skill convention, use the generic
profile:

```bash
striatum --repo "$TARGET_REPO" skills install --profile generic --json
# writes a single striatum-STRIATUM_AGENT_GUIDE.md you can paste
# into a system prompt
```

Now point your agent at `$TARGET_REPO`. Tell it: *"drive the
workflow at `<path>/workflow.json` using striatum"*. The agent
loads the bundle, registers a session, claims work, and proceeds.
For the long-form companion to the bundle, see
[HOW_TO_AGENT.md](HOW_TO_AGENT.md).

## What's in `.striatum/`?

After `init`, the target repo contains:

```text
.striatum/
  state.sqlite3       # live state — runs, jobs, leases, events
  scratch/            # per-supervisor named pipes (RFC 0009)
  bin/                # optional; e.g., claude-supervised-wrapper.sh
```

`.striatum/` is added to `.gitignore`. Repository files outside
`.striatum/` (artifacts, decisions, evidence exports) are durable
provenance and should be committed normally.

## Where to next

- **[HOW_TO_HUMAN.md](HOW_TO_HUMAN.md)** — every CLI verb the
  operator path uses, in the order you will use them, with
  examples.
- **[HOW_TO_AGENT.md](HOW_TO_AGENT.md)** — long-form companion to
  the skill bundle.
- **[WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md)** — author your
  own `workflow.json` from scratch.
- **[CLI_REFERENCE.md](CLI_REFERENCE.md)** — every CLI verb,
  flat list, with stable exit codes.
- **[SPEC.md](SPEC.md)** — the implementation contract for the
  current V1 surface.
- **[INDEX.md](INDEX.md)** — every doc in `docs/` with a one-line
  summary.

## How to contribute

The striatum source tree's contributor rules live in
[`AGENTS.md`](../AGENTS.md) at the repository root. The Makefile
targets `install`, `lint`, `typecheck`, `test`, and `smoke` are
the supported entry points; pull requests are expected to keep
all four green.
