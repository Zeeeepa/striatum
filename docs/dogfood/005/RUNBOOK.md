# Dogfood 005 — Process Adapter Completion Guarantees

Status: scaffolded
Date: 2026-05-08
Targets: RFC 0014 V1 — close
[issue #1](https://github.com/halbritt/striatum/issues/1)
("Process adapters can exit or hang without completing claimed
jobs").

## Goal

Use Striatum to research the current adapter code path, synthesize
the locked V1 design, review it, record human acceptance, ship the
combined three-step build slice (post-exit validation + envelope,
`--timeout-seconds`, `recovery process-reconcile` + doctor checks),
and review the build.

## Workflow Shape

```text
research_current_adapter_path
  -> synthesize_v1_design
  -> review_v1_design
  -> human acceptance decision
  -> implement_v1
  -> review_v1_build
```

5 jobs, `max_active_jobs: 1`, two cycles for `needs_revision`
verdicts.

## Why This Run Exists

[Issue #1](https://github.com/halbritt/striatum/issues/1) documented
an engram dogfood run where three reviewer lanes all failed
silently in different ways: Claude hung, Codex exited 0, Gemini
exited 1; none produced artifacts or verdicts; the operator had to
publish artifacts and verdicts manually. The
`process_executions` row for the killed Claude process stayed
`state="running"` because the manual kill bypassed Striatum's
bookkeeping.

RFC 0014 documented the root cause and the three-step fix. This
run lands the V1 build slice.

## One-Shot Environment

```bash
cd ~/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/005/workflow.json
TARGET_REPO=.
```

## Initialize And Inspect

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -80
```

Expect `valid: true` and no warnings (the wrapper from RFC 0010 V2
ships under `.striatum/bin/`).

## Prepare And Start

The workflow uses `branch.mode: "auto"`, so `run prepare` creates
the branch and transitions the run to `ready` atomically.

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

## Register Sessions

```bash
register() {
  "$RUNNER" --repo "$TARGET_REPO" register-session \
    --run-id "$RUN_ID" --role "$1" --lane "$2" \
    --capability "$3" --json \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])'
}

RESEARCHER=$(register researcher codex write)
DESIGNER=$(register designer codex write)
DESIGN_REVIEWER=$(register reviewer claude_code review)
IMPLEMENTER=$(register implementer codex write)
BUILD_REVIEWER=$(register reviewer claude_code review)
```

## Drive The Run

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$RESEARCHER" --json
# ack, do the work, publish CURRENT_ADAPTER.md, complete

"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGNER" --json
# publish DESIGN_SYNTHESIS.md, complete

"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGN_REVIEWER" --json
# publish DESIGN_REVIEW.md, submit verdict
```

## Human Acceptance Decision

If design review accepts:

```bash
mkdir -p docs/dogfood/005/decisions
"$RUNNER" --repo "$TARGET_REPO" decision record \
  --run-id "$RUN_ID" \
  --path docs/dogfood/005/decisions/V1_ACCEPTANCE.md \
  --outcome accepted_with_follow_up \
  --title "Accept RFC 0014 V1 build slice" \
  --follow-up "Ship post-exit validation + envelope, --timeout-seconds, and recovery process-reconcile + doctor checks per the synthesis." \
  --json
```

## Build And Review

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$IMPLEMENTER" --json
# implement V1 (3 steps), publish BUILD_HANDOFF.md, complete

"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$BUILD_REVIEWER" --json
# publish BUILD_REVIEW.md, submit verdict
```

## Capture Harness Friction

```bash
mkdir -p docs/dogfood/005/findings
cp docs/dogfood/005/HARNESS_PROPOSAL_TEMPLATE.md \
   docs/dogfood/005/findings/HARNESS-001.md
# edit, then publish from the active job
```

## Finish

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/005/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/005/RUN_SUMMARY.md --json
```

```bash
make lint
make typecheck
make test
git status --short --branch
```

## Reset

After confirming local runner state and the dogfood branch can be
discarded:

```bash
rm -rf .striatum/
git checkout main
git branch -D striatum/dogfood-005-process-adapter-completion 2>/dev/null || true
```
