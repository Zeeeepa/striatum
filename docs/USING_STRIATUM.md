# Using Striatum — Day-zero guide

This is the doc to read first. It covers what Striatum is, the two
named roles (AI operator + human principal), what you need
installed, how to start a workflow, and what your role looks like
when something escalates. Plan ~10 minutes.

If you want a quick install summary instead, see the
[`README.md`](../README.md). For the long-form playbooks, see
[`HOW_TO_HUMAN.md`](HOW_TO_HUMAN.md) and
[`HOW_TO_AGENT.md`](HOW_TO_AGENT.md).

## What is Striatum?

Striatum is a local workflow runner for terminal-based AI coding
agents. It coordinates draft → review → repair → synthesize loops
across multiple agents (Codex, Claude Code, Gemini CLI, or any
runtime you can wrap as a command), records every state transition
in a daemon-owned PostgreSQL audit chain, and never touches a hosted
service.

The runner does not decide that an agent is done because a terminal
printed a phrase. Agents and humans move work by calling `striatum`
CLI verbs.

## The two roles

Striatum runs with two named roles. RFC 0053 fixes the model.

**AI operator** — the default driver.

- Claims work, publishes artifacts, advances state through CLI
  verbs.
- Same surface that humans have; bounded by *function*, not
  *interface*. Nothing prevents a human from calling any verb;
  doing so for routine work is outside the role.
- Long-form companion to the operator skill bundle:
  [`HOW_TO_AGENT.md`](HOW_TO_AGENT.md).

**Human principal** — escalation only.

- Resolves blockers the AI judges itself stuck on (`escalation`
  artifacts or declared blocker classes like `human_checkpoint`).
- Watches the inbox; investigates when something paged; signs off
  on the change.
- Long-form playbook:
  [`HOW_TO_HUMAN.md`](HOW_TO_HUMAN.md).

Routine work belongs to the operator. If you find yourself
running CLI verbs to push a healthy run forward, the operator
isn't doing its job — that's harness friction worth filing.

## Prerequisites

- **Python 3.11+.** Striatum is published to PyPI as
  `striatum-orchestrator`. The package name is unrelated to a
  separate `striatum` package on PyPI.
- **PostgreSQL 14+** running locally. The daemon is a hard
  prerequisite; SQLite is no longer the live substrate (D094 /
  RFC 0043). See [`POSTGRES_TRANSITION.md`](POSTGRES_TRANSITION.md)
  for the install runbook including the `striatumd_rw` role
  provisioning.
- **An agent runtime** — Claude Code, Codex, Gemini CLI, or any
  CLI tool that takes a session prompt and writes a deliverable.
  Striatum provides a skill bundle for the first three;
  `--profile generic` writes a paste-into-system-prompt guide for
  anything else.
- **Optionally**, a target repository you want to orchestrate.
  Striatum can register multiple target repos with the same
  daemon.

## Day-zero setup

```bash
# 1. Install.
pip install striatum-orchestrator

# 2. Verify and repair the local Postgres role/grants.
striatum daemon doctor \
  --apply-migrations \
  --provision-rw-role \
  --repair-grants \
  --json

# 3. Install/start a user service, or use foreground daemon start.
striatum daemon service install --manager auto --json
striatum daemon service start --manager auto --json
# OR: striatum daemon start --json &

# 4. Adopt a target repo: init scratch, install skills/plugins,
#    scaffold DDD docs, and migrate/register the repo in Postgres.
TARGET_REPO=/path/to/your/repo
striatum --repo "$TARGET_REPO" adopt --profile claude_code --json

# 5. Smoke-test the first-run path end to end.
striatum --repo "$TARGET_REPO" doctor --first-run --json
```

What `adopt` does:

- Creates `.striatum/` next to the target repo (runtime scratch).
- Writes the operator skill bundle to
  `.claude/skills/striatum-*/` (or `~/.codex/skills/` for codex,
  etc.).
- Writes the agent-CLI plugin bundle for the selected profile when one
  exists.
- Scaffolds the seven canonical DDD docs under `docs/` (per
  RFC 0021) — `SPEC.md`, `PRD.md`, `DECISION_LOG.md`,
  `UBIQUITOUS_LANGUAGE.md`, `DDD.md`, `rfcs/README.md`,
  `rfcs/0001-template.md`. Existing files are preserved.
- Migrates/registers the repo with the daemon-owned Postgres substrate
  and reports a suggested starter workflow path.

If the target repo follows the recommended layout in
[`CONSUMER_REPO_LAYOUT.md`](CONSUMER_REPO_LAYOUT.md), your
workflow file lives at `striatum/workflows/<name>.json` and
artifacts land under `striatum/<workflow-name>/`.

## Your first run

```bash
WORKFLOW="$TARGET_REPO/striatum/workflows/code-change.json"
# Or use a starter from the repo:
WORKFLOW=examples/code-change-flow/workflow.json

# Validate the workflow JSON against the schema.
striatum --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json

# Prepare a run (records the workflow snapshot + creates the runs row).
striatum --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json
# -> note the run_id from the envelope

# Confirm a working branch (optional — required for code-change workflows).
striatum --repo "$TARGET_REPO" branch confirm \
  --run-id <run_id> --branch striatum/example --json

# Start the run.
striatum --repo "$TARGET_REPO" run start --run-id <run_id> --json

# Watch progress.
striatum --repo "$TARGET_REPO" dashboard --run-id <run_id> --once
# OR for a live terminal view:
striatum --repo "$TARGET_REPO" dashboard --run-id <run_id>
```

Now hand the agent the run. With Claude Code, open a session in
the target repo and tell it:

> Drive the workflow at `striatum/workflows/code-change.json`
> using striatum.

The operator skill bundle teaches the agent the verbs it needs
(`claim-next`, `ack`, `complete`, `publish-artifact`, `verdict`,
`submit-review`, `recovery requeue-stale`). The agent
self-supervises through the loop until the run completes or hits a
blocker it can't resolve.

## Your role as principal

The AI operator is the default driver. You're escalation-only.
What that looks like in practice:

```bash
# 1. Check the inbox for escalations + open blockers.
striatum --repo "$TARGET_REPO" inbox --json

# 2. For a stuck run, look at the dashboard and ask `why`.
striatum --repo "$TARGET_REPO" status --run-id <run_id> --json
striatum --repo "$TARGET_REPO" why <target_id> --run-id <run_id>

# 3. Resolve a checkpoint or recover stale work.
striatum --repo "$TARGET_REPO" checkpoint resolve \
  --blocker-id <id> --action continue --json
# OR
striatum --repo "$TARGET_REPO" recovery requeue-stale \
  --run-id <run_id> --job-id <job_id> --force \
  --justification "<reason>" --json
```

The principal's playbook —
[`HOW_TO_HUMAN.md`](HOW_TO_HUMAN.md) — covers the full set of
recovery verbs with the documented reasons each is appropriate.
The short version: don't reach for these by default. If the
operator AI is repeatedly hitting the same blocker, that's
harness friction worth filing in `HARNESS_FRICTION_PATTERNS.md`.

## Where to go next

- **Long-form playbooks** —
  [`HOW_TO_HUMAN.md`](HOW_TO_HUMAN.md) for the human principal,
  [`HOW_TO_AGENT.md`](HOW_TO_AGENT.md) for the AI operator (and
  the long-form companion to the skill bundle).
- **The implementation contract** — [`SPEC.md`](SPEC.md). When
  this doc disagrees with the runner, the SPEC wins.
- **Every CLI verb + stable exit codes** —
  [`CLI_REFERENCE.md`](CLI_REFERENCE.md).
- **Workflow shapes and lane sets** —
  [`WORKFLOW_TYPES.md`](WORKFLOW_TYPES.md).
- **Authoring a workflow.json** —
  [`WRITING_WORKFLOWS.md`](WRITING_WORKFLOWS.md).
- **System architecture (Mermaid)** —
  [`README.md`](../README.md) §"At a glance".
- **Postgres prerequisites + per-repo migration** —
  [`POSTGRES_TRANSITION.md`](POSTGRES_TRANSITION.md).
- **Target-repo layout recommendations** —
  [`CONSUMER_REPO_LAYOUT.md`](CONSUMER_REPO_LAYOUT.md).
- **Every doc in `docs/`** — [`INDEX.md`](INDEX.md).
