---
name: striatum-dogfood-002
description: Run the Striatum dogfood-002 workflow that lands RFC 0011 (explicit `striatum session close` plus run-terminal auto-close of active sessions). Use when the operator asks to start, drive, monitor, recover, or explain dogfood-002, including registering sessions, claiming work, watching the dashboard, capturing friction, exporting evidence, and stopping supervisors.
---

# Striatum Dogfood 002

Use this skill to start and drive the dogfood-002 workflow from the
Striatum repository root. dogfood-002 lands RFC 0011: the runner
finally has a CLI surface for transitioning sessions out of `active`
and an automatic close-on-run-terminal flow that resolves the
permanent `active_session_on_terminal_run` doctor warning.

## Ground Rules

- Work from the repository root (e.g. `/home/halbritt/git/striatum`).
- Treat `.striatum/state.sqlite3` as live local state. Do not edit
  SQLite directly and do not commit `.striatum/`.
- Verify the editable install before starting (HARNESS-002 foot-gun):
  ```bash
  .venv/bin/pip show striatum | grep "Editable project location"
  ```
  must point at the canonical repo, not a Claude Code worktree.
- The author lane is `claude_code`; the reviewer lane is `codex`.
- Operator-driven reviewer registration requires
  `--force-non-fresh --reason "..."` (HARNESS-003).
- Stdout/stderr for any supervised agent are `DEVNULL`; watch via
  `dashboard`, `status`, `why`, and produced artifacts.

## Quick Start

```bash
cd /home/halbritt/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/002/workflow.json
TARGET_REPO=.
```

Verify and prepare:

```bash
make test
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -40

PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-002-session-close --create --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json

AUTHOR=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role author --lane claude_code \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role reviewer --lane codex \
  --capability review \
  --force-non-fresh --reason "operator-driven; supervised lane work deferred" \
  --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
```

Drive:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$AUTHOR" --json
# (do the work per docs/dogfood/002/prompts/draft.md)
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$REVIEWER" --json
# (do the review per docs/dogfood/002/prompts/review.md)
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$AUTHOR" --json
# (apply per docs/dogfood/002/prompts/apply.md)
```

Watch:

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

## What The Agents Should Do

The author packet points at `docs/dogfood/002/prompts/draft.md` —
implement RFC 0011 end to end (migration v7, `session close` CLI,
`close_remaining_sessions` helper at every run-terminal transition,
evidence/run-summary rendering, seven acceptance tests).

The reviewer packet points at `docs/dogfood/002/prompts/review.md`
and walks the seven acceptance criteria as gates.

The apply step promotes RFC 0011 to `accepted`, adds the
`docs/DECISION_LOG.md` entry, and verifies that *this run's own*
`doctor` output is `ok=true` after `complete` triggers auto-close.
That is the in-the-loop validation of the change.

## Capture Harness Friction

Author-side: `docs/dogfood/002/findings/HARNESS-NNN.md`.
Reviewer-side: `docs/dogfood/002/review/HARNESS-NNN.md`.
HARNESS-004 is the reason the two paths are distinct.

```bash
mkdir -p docs/dogfood/002/findings
cp docs/dogfood/002/HARNESS_PROPOSAL_TEMPLATE.md \
   docs/dogfood/002/findings/HARNESS-001.md
# edit it
"$RUNNER" --repo "$TARGET_REPO" publish-artifact \
  --session-id "$AUTHOR" --job-id "$AUTHOR_JOB_ID" \
  --lease-id "$AUTHOR_LEASE_ID" \
  --kind harness_improvement_proposal \
  --logical-name harness_002_001 \
  --path docs/dogfood/002/findings/HARNESS-001.md --json
```

## Finish The Run

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/002/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/002/RUN_SUMMARY.md --json
```

After RFC 0011 lands, `complete` on the apply job triggers auto-close
for every session on the run. `doctor --run-id "$RUN_ID" --json`
should return `ok: true` immediately afterward.

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
git branch -D striatum/dogfood-002-session-close 2>/dev/null || true
```

Only reset after confirming losing local runner state and the
dogfood-002 branch is acceptable.
