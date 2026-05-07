---
name: striatum-dogfood-001-v2
description: Run the Striatum dogfood-001 v2 workflow that lands the four HARNESS fixes from dogfood-001 (HARNESS-001 supervised lane diagnostics + idempotent stop, HARNESS-002 editable-install guards, HARNESS-003 reviewer-independence advisory + register-session policy, HARNESS-004 reviewer doc fix). Use when the operator asks to start, drive, monitor, recover, or explain dogfood-001 v2, including registering sessions, claiming work, watching the dashboard, capturing v2-round harness friction, exporting evidence, and stopping supervisors.
---

# Striatum Dogfood 001 v2

Use this skill to start and drive the dogfood-001 v2 workflow from the
Striatum repository root. v2 dogfoods the runner's own remediation work
— the change being driven *is* the four HARNESS fixes from dogfood-001.

## Ground Rules

- Work from the repository root: `/home/halbritt/git/striatum` (or
  wherever the repo was cloned).
- Treat `.striatum/state.sqlite3` as live local state. Do not edit
  SQLite directly and do not commit `.striatum/`.
- Starting the run does not launch an interactive orchestrator chat.
  It creates claimable work in Striatum state. Use CLI commands to
  drive state.
- The author lane is `claude_code`; the reviewer lane is `codex`. If
  HARNESS-001 lands during draft and codex's `exec -` mode actually
  consumes packets, drive the reviewer through real `supervise start`.
  If not, fall back to operator-driven review and capture the symptom
  as a v2-round `harness_improvement_proposal`.
- Stdout and stderr for supervised agents are intentionally discarded.
  Watch with `dashboard`, `status`, `why`, and produced artifacts.
- **Verify the editable install before starting.** This is the
  HARNESS-002 foot-gun:
  ```bash
  .venv/bin/pip show striatum | grep "Editable project location"
  ```
  must point at the canonical repo, not a Claude Code worktree.

## Quick Start

Set these variables in the shell that will drive the run:

```bash
cd /home/halbritt/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/001-v2/workflow.json
TARGET_REPO=.
```

Verify the runner and workflow:

```bash
make test
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW" --format dot
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -40
```

Prepare, confirm the branch, and start the run:

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-001-v2-harness-fixes \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

Register sessions:

```bash
AUTHOR=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role author --lane claude_code \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role reviewer --lane codex \
  --capability review --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
```

Start the author supervisor and claim the first packet:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise start --session-id "$AUTHOR" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$AUTHOR" --json
```

Watch the run from another terminal:

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

After the author completes, claim reviewer work:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$REVIEWER" --json
```

## What The Agents Should Do

The author packet points at `docs/dogfood/001-v2/prompts/draft.md`.
Land the cheap layer of each HARNESS fix per that prompt's per-HARNESS
"in scope" lists. Defer items the prompt marks "out of scope" — write
them up in `DRAFT_HANDOFF.md` instead of silently expanding scope.

The reviewer packet points at `docs/dogfood/001-v2/prompts/review.md`.
Walk each HARNESS proposal's "Proposed change" sub-points and mark
each as landed / partial / deferred against the source code; do not
take the author handoff on faith.

If the review accepts (with or without findings), the apply job
becomes claimable for the author lane.

## Capture Harness Friction

Whenever the v2 round itself is awkward, surprising, missing, or
broken, file a `harness_improvement_proposal` immediately. The split
introduced by HARNESS-004 is intentional: author-side proposals go
under `docs/dogfood/001-v2/findings/`, reviewer-side under
`docs/dogfood/001-v2/review/`. Each path matches the corresponding
write_scope.

```bash
mkdir -p docs/dogfood/001-v2/findings
cp docs/dogfood/001-v2/HARNESS_PROPOSAL_TEMPLATE.md \
   docs/dogfood/001-v2/findings/HARNESS-001.md
# edit it
"$RUNNER" --repo "$TARGET_REPO" publish-artifact \
  --session-id "$AUTHOR" --job-id "$AUTHOR_JOB_ID" \
  --lease-id "$AUTHOR_LEASE_ID" \
  --kind harness_improvement_proposal \
  --logical-name harness_v2_001 \
  --path docs/dogfood/001-v2/findings/HARNESS-001.md \
  --json
```

## Finish The Run

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/001-v2/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/001-v2/RUN_SUMMARY.md --json

"$RUNNER" --repo "$TARGET_REPO" supervise stop \
  --session-id "$AUTHOR" \
  --reason "dogfood 001 v2 done" --json
```

`supervise stop` should now be idempotent against a lost supervisor
(HARNESS-001 fix). If it still returns exit 4, that's a v2-round
finding worth filing.

Run verification before reporting success:

```bash
make lint
make typecheck
make test
git status --short --branch
```

## Recovery

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
git branch -D striatum/dogfood-001-v2-harness-fixes 2>/dev/null || true
```

Only reset after confirming that losing local runner state and the v2
branch is acceptable.
