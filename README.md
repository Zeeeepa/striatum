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

`v1.25.0`. RFCs 0001–0026, RFC 0028 V1, RFC 0030 V2 foundation, RFC
0031 V2 foundation, RFC 0032 V2, RFC 0033 V2, and RFC 0034 V1 are
accepted (some at `accepted (V1)` or later staged statuses); RFC 0027
is partially implemented through fail-closed mode and apply-receipt
foundation surfacing; RFC 0035 is proposed (multi-repo test harness).
Per-version release notes live in
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
WORKFLOW=examples/code-change-flow/workflow.json   # or choose a type in docs/WORKFLOW_TYPES.md

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

## Optional Multi-Repo Registry And Sweep

```bash
striatumd --max-sweeps 1 --json              # foreground sweep process
striatum repo add /path/to/target --json
striatum dashboard --all --json
striatum --daemon --repo /path/to/target status --json
```

RFC 0028 V1 is local-first and optional. It adds registry-backed
multi-repository read visibility, daemon audit metadata, resources-only
daemon MCP, and a foreground recovery sweep process. It does not ship a
daemon RPC server in V1: CLI and MCP clients open the owner-only registry
SQLite directly under token/capability checks. Direct repo-local CLI mode
remains the default. `dashboard --all` is registry-backed and requires a
daemon token; run `repo add` or `daemon start` first to bootstrap the local
admin token, or pass an explicit token through the client surface.
`repo add` registers an already-initialized repository by default; use
`repo add --init` only when you intentionally want registration to
initialize the target. V1 does not add daemon-owned supervision, MCP
mutation tools, sealed apply/signing, hosted service semantics, audit
retention/rotation, or stronger provenance than the selected workflow
`provenance_mode` already provides.

RFC 0033 V2 accepts system PostgreSQL as the daemon-owned storage
substrate for daemon-global registry, audit, capability, scheduler, and
RPC-session state. Configure it with `STRIATUM_DAEMON_DB_URL`, daemon
config, or an explicit `--postgres-url` client surface. The cutover
command is `striatum daemon migrate --from sqlite --to pg` (`--dry-run`
inspects first; `--keep-sqlite-readonly` keeps the V1 file as an audit
tombstone). Repo-local `.striatum/state.sqlite3` remains the
authoritative workflow state store.

RFC 0030 and RFC 0031 add the daemon V2 RPC/supervision/apply
foundation on top of that substrate: envelope-v1 JSON, version
handshake, method registry, request/audit helpers, daemon supervisor
metadata, repo-local supervisor pointers, and apply receipts. RFC 0032
adds the V2 cross-repo schema/migration and daemon MCP mutation
capability-gating foundation; the full two-repo daemon end-to-end test
harness remains deferred. Hosted service semantics and
malicious-local-operator-resistant sealed apply remain out of scope.

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
| [docs/WORKFLOW_TYPES.md](docs/WORKFLOW_TYPES.md) | Which workflow shape and lane set to choose; current starters, examples, defaults, and chooser roadmap. |
| [docs/WRITING_WORKFLOWS.md](docs/WRITING_WORKFLOWS.md) | How to author your own `workflow.json`. |
| [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) | Flat list of every CLI verb and stable exit codes. |
| [docs/SPEC.md](docs/SPEC.md) | The implementation contract; the source of truth when this page disagrees with the runner. |
| [docs/INDEX.md](docs/INDEX.md) | Every doc in `docs/` with a one-line summary. |

## License

Apache-2.0. See [`LICENSE`](LICENSE).
