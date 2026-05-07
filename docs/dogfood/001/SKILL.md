---
name: striatum-dogfood-001
description: Run the Striatum dogfood-001 workflow that adds Graphviz DOT export to workflow graph. Use when Codex is asked to start, drive, monitor, recover, or explain the scaffolded dogfood-001 Striatum-on-Striatum run, including registering sessions, starting the supervised author lane, claiming work, watching the dashboard, capturing harness friction, exporting evidence, and stopping supervisors.
---

# Striatum Dogfood 001

Use this skill to start and drive the scaffolded dogfood-001 workflow from the
Striatum repository root. This workflow is intentionally a real
Striatum-on-Striatum run: the product change is small, but the main purpose is
to discover runner friction and capture it as durable artifacts.

## Ground Rules

- Work from the repository root: `/Users/halbritt/git/striatum`.
- Treat `.striatum/state.sqlite3` as live local state. Do not edit SQLite
  directly and do not commit `.striatum/`.
- Starting the run does not launch an interactive orchestrator chat. It creates
  claimable work in Striatum state. Use CLI commands to drive state.
- `supervise start` launches the configured author lane process. `claim-next`
  auto-delivers the work packet to an attached supervisor when possible.
- Stdout and stderr for supervised agents are intentionally discarded. Watch
  with `dashboard`, `status`, `why`, and produced artifacts.
- Before creating the dogfood branch, inspect `git status --short --branch`.
  If unrelated local changes exist, report them and ask whether to commit,
  stash, or continue with a dirty tree.

## Quick Start

Set these variables in the shell that will drive the run:

```bash
cd /Users/halbritt/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/001/workflow.json
TARGET_REPO=.
```

Verify the runner and workflow:

```bash
make test
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -40
```

Prepare, confirm the branch, and start the run:

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-001-graph-dot \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

Register the author and reviewer sessions:

```bash
AUTHOR=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" \
  --role author --lane claude_code \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
echo "AUTHOR=$AUTHOR"

REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" \
  --role reviewer --lane codex \
  --capability review --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
echo "REVIEWER=$REVIEWER"
```

Start the author supervisor and claim the first packet:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise start \
  --session-id "$AUTHOR" --json

"$RUNNER" --repo "$TARGET_REPO" claim-next \
  --session-id "$AUTHOR" --json
```

Watch the run from another terminal when useful:

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

After the author completes, claim reviewer work:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next \
  --session-id "$REVIEWER" --json
```

## What The Agents Should Do

The author packet points at `docs/dogfood/001/prompts/draft.md`. The author
should read the packet, read the prompt and referenced files, implement
`workflow graph --format dot`, publish `docs/dogfood/001/DRAFT_HANDOFF.md`,
and complete the job using the exact commands in the work packet.

The reviewer packet points at `docs/dogfood/001/prompts/review.md`. The
reviewer should inspect the change and handoff, publish
`docs/dogfood/001/review/FINDING.md`, and use `submit-review` with one of
`accept`, `accept_with_findings`, `needs_revision`, or `reject`.

If the review accepts and the apply job becomes claimable, send it to the
author lane with `claim-next --session-id "$AUTHOR"`.

## Capture Harness Friction

Whenever something is awkward, surprising, missing, or broken, create a
`harness_improvement_proposal` artifact immediately. Start from the template:

```bash
mkdir -p docs/dogfood/001/findings
cp docs/dogfood/001/HARNESS_PROPOSAL_TEMPLATE.md \
   docs/dogfood/001/findings/HARNESS-001.md
```

Edit the artifact with the concrete command, observed behavior, expected
behavior, and proposed fix. Then publish it from the active job's session,
job id, and lease id:

```bash
"$RUNNER" --repo "$TARGET_REPO" publish-artifact \
  --session-id "$SESSION_ID" \
  --job-id "$JOB_ID" \
  --lease-id "$LEASE_ID" \
  --kind harness_improvement_proposal \
  --logical-name harness_001 \
  --path docs/dogfood/001/findings/HARNESS-001.md \
  --json
```

Use `striatum status --run-id "$RUN_ID" --json` or `striatum why <id> --json`
to recover ids if needed.

## Finish The Run

At the end, export durable evidence and a run summary:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/001/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/001/RUN_SUMMARY.md --json
```

Stop the author supervisor cleanly:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise stop \
  --session-id "$AUTHOR" \
  --reason "dogfood 001 done" --json
```

Run verification before reporting success:

```bash
make lint
make typecheck
make test
git status --short --branch
```

## Recovery

If the supervisor path breaks, inspect it before resetting:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise status --session-id "$AUTHOR" --json
"$RUNNER" --repo "$TARGET_REPO" doctor --run-id "$RUN_ID" --verbose --json
"$RUNNER" --repo "$TARGET_REPO" status --run-id "$RUN_ID" --json
```

If the run is stuck and the operator wants a full reset:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise stop \
  --session-id "$AUTHOR" --reason reset --json 2>/dev/null || true
rm -rf .striatum/
git checkout main
git branch -D striatum/dogfood-001-graph-dot 2>/dev/null || true
```

Only reset after confirming that losing local runner state and the dogfood
branch is acceptable.
