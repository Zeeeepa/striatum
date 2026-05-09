# striatum

Local-first orchestration for multiple terminal-based AI coding agents.

`striatum` is a small, repo-local control plane for coordinating AI
coding agents that live in terminals: Codex, Claude Code, Gemini
CLI, or any other model runtime that can be represented as a
command. It is built for workflows where several agents need to
draft, review, synthesize, repair, and report on work without
relying on a hosted coordinator or hidden chat transcripts.

The important distinction is this:
`.striatum/state.sqlite3` is the authoritative live state for
runs, jobs, sessions, queue messages, leases, blockers, verdicts,
artifacts, and events. Repository files (prompts, findings,
ledgers, syntheses, decisions, handoffs, redacted evidence
exports) are durable provenance, not the live message bus. Marker
files, tmux pane state, terminal output, and provider hooks are
useful for humans, but they do not advance state.

## Status

`v1.11.0`. RFCs 0001–0022 are all `accepted` (some at
`accepted (V1)` or `accepted (V1+V1.5)`); the implementation
slices for each are on main. Per-version release notes live in
[`CHANGELOG.md`](CHANGELOG.md). The package is published to
PyPI as `striatum-orchestrator` (the bare `striatum` name on
PyPI is unrelated); the Python module name is still
`striatum`.

## Install

From PyPI:

```bash
pip install striatum-orchestrator
striatum --help
```

From a checkout of this repository:

```bash
make install
.venv/bin/striatum --help
```

For development without installing the console script:

```bash
PYTHONPATH=src python3 -m striatum.cli --help
```

Run the tests with:

```bash
make test
```

## Quick Start (Human Operator)

You will run `striatum` commands by hand.

```bash
TARGET_REPO=/path/to/your/repo
WORKFLOW=examples/code-change-flow/workflow.json   # or your own

striatum --repo "$TARGET_REPO" init --json
striatum --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
striatum --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json
striatum --repo "$TARGET_REPO" run start --run-id <run_id> --json
striatum --repo "$TARGET_REPO" dashboard --run-id <run_id> --once
```

From here you register a session and claim work. The full
operator playbook is [`docs/HOW_TO_HUMAN.md`](docs/HOW_TO_HUMAN.md).

## Quick Start (Coding Agent)

You will install the runner, install the *agent skill bundle*
(RFC 0015), and hand the agent a target repo with a workflow
file in it. The agent does the rest. For first-time setup,
combine `--with-skills` and `--with-ddd-layout` (RFC 0021) so
the target repo gets both the agent-facing skills and the
human-facing DDD doc layout in one command:

```bash
TARGET_REPO=/path/to/your/repo
striatum --repo "$TARGET_REPO" init \
  --with-skills claude_code \
  --with-ddd-layout \
  --json
# now point your Claude Code session at $TARGET_REPO and tell it:
#   "drive the workflow at <path>/workflow.json using striatum"
```

The single command above initializes `.striatum/`, writes the
agent skill bundle to `.claude/skills/striatum-*/`, and
scaffolds the seven canonical human-facing DDD documents
(`docs/SPEC.md`, `docs/PRD.md`, `docs/DECISION_LOG.md`,
`docs/UBIQUITOUS_LANGUAGE.md`, `docs/DDD.md`,
`docs/rfcs/README.md`, `docs/rfcs/0001-template.md`).
Existing files are preserved; pass `--ddd-layout-dry-run` to
preview, `--ddd-layout-force` to overwrite (records
`prior_sha256` for audit). For agents without a skill
convention, `striatum skills install --profile generic`
writes a single `STRIATUM_AGENT_GUIDE.md` you can paste into a
system prompt. The long-form companion to the bundle is
[`docs/HOW_TO_AGENT.md`](docs/HOW_TO_AGENT.md).

## Web UI

```bash
striatum --repo "$TARGET_REPO" serve --web
# bound port is in the startup envelope; pass --port for a fixed one
```

The web UI (RFC 0022) is server-rendered Jinja2: real HTML
pages at `/`, `/run/<id>`, `/run/<id>/job/<id>`,
`/run/<id>/artifact/<id>`, `/doctor`. The dependency graph
renders as inline SVG with state-colored nodes that
click-navigate. Light + dark mode follow
`prefers-color-scheme`. Localhost-only by default; mutations
gated behind `--allow-mutations`.

## What It Is For

striatum is for long-running, review-heavy agent workflows where
"just tell three agents to work in tmux panes" stops being
enough. It gives the human and coordinator a stable answer to
questions like:

- What run is active, on which branch was it confirmed, and
  which jobs are claimable, blocked, in review, or waiting on a
  human?
- Which session owns a lease? What artifact was required, where
  was it written, what hash was recorded?
- Did a review return `needs_revision`, and did the workflow
  declare a safe cycle for that?
- Can I commit a redacted evidence summary without committing
  live SQLite state or transcripts?

The runner is intentionally conservative. It coordinates work; it
does not decide that an agent is done because a terminal printed
a phrase. Agents and humans move the workflow by calling
`striatum` commands.

striatum is a domain-driven workflow runner: the vocabulary in
[`docs/UBIQUITOUS_LANGUAGE.md`](docs/UBIQUITOUS_LANGUAGE.md) is the
*model*, not just documentation; the CLI verbs are the only legal
mutations. [`docs/DDD.md`](docs/DDD.md) explains why the
vocabulary is load-bearing instead of bookkeeping.

## Documentation Map

| File | When to read |
|---|---|
| [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) | First 15 minutes; forks human-operator vs. coding-agent setup. |
| [docs/HOW_TO_HUMAN.md](docs/HOW_TO_HUMAN.md) | The operator's long-form playbook; every CLI verb in the order you use it. |
| [docs/HOW_TO_AGENT.md](docs/HOW_TO_AGENT.md) | Long-form companion to the RFC 0015 agent skill bundle. |
| [docs/WRITING_WORKFLOWS.md](docs/WRITING_WORKFLOWS.md) | How to author your own `workflow.json`. |
| [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) | Flat list of every CLI verb and stable exit codes. |
| [docs/SPEC.md](docs/SPEC.md) | The implementation contract; the source of truth when this page disagrees with the runner. |
| [docs/INDEX.md](docs/INDEX.md) | Every doc in `docs/` with a one-line summary. |

## License

Apache-2.0. See [`LICENSE`](LICENSE).
