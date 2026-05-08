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

V1 ships **two skill profiles**. Pick the one that matches your
agent CLI; you can also install both side-by-side and let the
agent decide which one to read.

| Agent CLI | Use this profile | Where files land |
|---|---|---|
| Claude Code | `claude_code` | `.claude/skills/striatum-*/SKILL.md` (five files; Claude Code auto-discovers them) |
| Codex CLI | `generic` (V1) | `striatum-STRIATUM_AGENT_GUIDE.md` at the repo root |
| Gemini CLI | `generic` (V1) | `striatum-STRIATUM_AGENT_GUIDE.md` at the repo root |
| Anything else | `generic` | `striatum-STRIATUM_AGENT_GUIDE.md` at the repo root |

`codex` and `gemini` first-class profiles are step 3 of RFC 0015
and remain deferred; until they land, the `generic` profile
covers those agents — same content, single concatenated guide
you can `cat` into a system prompt or feed as a tool input.

### If your agent is Claude Code

```bash
TARGET_REPO=/path/to/your/repo
striatum --repo "$TARGET_REPO" init --with-skills claude_code --json
```

That single command initializes `.striatum/` in the target repo
and writes the RFC 0015 skill bundle to
`.claude/skills/striatum-*/`. The bundle teaches a Claude Code
session how to drive the runner without reading the striatum
source.

### If your agent is Codex / Gemini CLI / anything else

```bash
TARGET_REPO=/path/to/your/repo
striatum --repo "$TARGET_REPO" init --json
striatum --repo "$TARGET_REPO" skills install --profile generic --json
# writes a single striatum-STRIATUM_AGENT_GUIDE.md at the repo root.
# Paste its contents into the agent's system prompt, or load it as
# a tool input — whatever your CLI's "load this doc as context"
# convention is.
```

The generic guide is one Markdown file with the same five-section
structure as the Claude Code bundle (workflow router, scaffold,
claim loop, supervise, recover). Any agent that can read a
Markdown file can drive the runner from it.

### Install both, if you switch CLIs

```bash
striatum --repo "$TARGET_REPO" skills install --profile claude_code --json
striatum --repo "$TARGET_REPO" skills install --profile generic --json
```

The two profiles write to disjoint paths and each carries its own
manifest, so they don't collide.

### Now drive the run

Point your agent at `$TARGET_REPO`. Tell it: *"drive the workflow
at `<path>/workflow.json` using striatum"*. The agent loads the
bundle, registers a session, claims work, and proceeds.
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

### Where will the workflow's output land?

striatum has **no** built-in output directory. The location of
every artifact comes from the workflow file itself: each job
declares `expected_artifacts[].path` and
`write_scope.allowed_paths`. The runner accepts those paths
verbatim.

If you are trying striatum on a real repo and want the runner's
output corralled (so a single `rm -rf` cleans up if you change
your mind), the recommended convention is a top-level
`striatum/` directory — sibling to the gitignored `.striatum/`
state directory but committed:

```text
<your-repo>/
├── .striatum/             # gitignored runtime state
└── striatum/              # committed durable output
    └── <workflow-slug>/
        ├── RUN_SUMMARY.md
        ├── RUN_EVIDENCE.md
        └── ...
```

This is just a convention — your workflow chooses its own
paths. See [WRITING_WORKFLOWS.md § "Recommended output
layout"](WRITING_WORKFLOWS.md#recommended-output-layout) for
the full pattern, and [HOW_TO_HUMAN.md § "Where artifacts
land"](HOW_TO_HUMAN.md#where-artifacts-land) for how to adapt
an existing example fixture.

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
