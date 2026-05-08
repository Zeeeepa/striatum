# Dogfood 004 — Claude Code Supervised Wrapper

Status: scaffolded
Date scaffolded: 2026-05-08
Targets: V2 step of `docs/dogfood/003/findings/HARNESS-001.md` —
author `.striatum/bin/claude-supervised-wrapper.sh` so workflows that
declare a supervised Claude Code lane (per RFC 0009) actually run.

## Goal

Use Striatum itself to research Claude Code's stdin behavior under
named-pipe (`os.mkfifo`) supervisor pipes, design the wrapper, review
that design, record human acceptance, ship the wrapper plus a
verification test, and review the build.

## Workflow Shape

```text
research_pipe_behavior
  -> synthesize_wrapper_design
  -> review_wrapper_design
  -> human acceptance decision
  -> implement_wrapper
  -> review_wrapper_build
```

The implementer prompt is instructed to block with a human checkpoint
if the acceptance decision artifact is missing.

The implementer's `write_scope` carves `.striatum/bin/` out of the
otherwise-forbidden `.striatum/` tree. That carve-out is the only
permitted `.striatum/` write in the workflow; `.striatum/state.sqlite3`
remains forbidden.

## Why This Run Exists

`docs/dogfood/003/findings/HARNESS-001.md` recorded the missing wrapper
as a high-severity friction. RFC 0010 V1.5 added a workflow-validate
lint warning for the missing path; this run lands the wrapper itself.

The known unknown is whether Claude Code's `claude --print` /
`--input-format stream-json` reads named-pipe stdin in a way that
preserves per-line packet semantics under back-pressure. The research
and design jobs should produce a verification harness (a small Python
test that drives `os.mkfifo` and asserts the wrapper accepts each
packet as a discrete user turn) before the implementer commits to a
specific `claude` invocation form.

## Before You Start

Install the agent CLIs you want to use as lanes:

- `codex` — `codex exec --json --ephemeral ... -` with per-job
  `CODEX_HOME`.
- `claude` — Claude Code CLI; the wrapper this run produces will be
  the supervised lane command. While the wrapper is being authored,
  the dogfood-004 RUNBOOK (this file) tells the operator to drive
  the `claude_code` lane interactively rather than via `striatum
  supervise start` until the implementer ships the wrapper.

If `claude` is not installed, the research, design, and review jobs
can still run on the codex lane; only the implementer's verification
test needs an actual `claude` binary.

Verify Striatum:

```bash
make test
```

## One-Shot Environment

```bash
cd ~/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/004/workflow.json
TARGET_REPO=.
```

## Initialize And Inspect

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -80
```

Expect the validate output to include exactly one lint warning naming
the missing `.striatum/bin/claude-supervised-wrapper.sh` path. That
warning will go away when the implementer ships the wrapper.

## Prepare, Confirm Branch, Start

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-004-claude-supervised-wrapper \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

## Register Sessions

```bash
RESEARCHER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role researcher --lane codex \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

DESIGNER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role designer --lane codex \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

DESIGN_REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role reviewer --lane claude_code \
  --capability review --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

IMPLEMENTER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role implementer --lane codex \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

BUILD_REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role reviewer --lane claude_code \
  --capability review --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
```

## Drive The Run

Single-job-per-step (`max_active_jobs: 1`); claim sequentially.

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$RESEARCHER" --json
# do the work, publish PIPE_BEHAVIOR.md, complete

"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGNER" --json
# publish DESIGN_SYNTHESIS.md, complete

"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGN_REVIEWER" --json
# publish DESIGN_REVIEW.md, submit verdict
```

## Human Acceptance Decision

If the design review accepts, record human acceptance before
implementation:

```bash
mkdir -p docs/dogfood/004/decisions
"$RUNNER" --repo "$TARGET_REPO" decision record \
  --run-id "$RUN_ID" \
  --path docs/dogfood/004/decisions/WRAPPER_ACCEPTANCE.md \
  --outcome accepted_with_follow_up \
  --title "Accept wrapper design and proceed to implementation" \
  --follow-up "Ship .striatum/bin/claude-supervised-wrapper.sh and the named-pipe verification test." \
  --json
```

## Build And Review

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$IMPLEMENTER" --json
# author the wrapper + verification test, publish BUILD_HANDOFF.md, complete

"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$BUILD_REVIEWER" --json
# publish BUILD_REVIEW.md, submit verdict
```

## Capture Harness Friction

```bash
mkdir -p docs/dogfood/004/findings
cp docs/dogfood/004/HARNESS_PROPOSAL_TEMPLATE.md \
   docs/dogfood/004/findings/HARNESS-001.md
# edit, then publish from the active job
```

## Finish

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/004/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/004/RUN_SUMMARY.md --json
```

Run final checks:

```bash
make lint
make typecheck
make test
git status --short --branch
```

## Reset

Only after confirming local runner state and the dogfood branch can be
discarded:

```bash
rm -rf .striatum/
git checkout main
git branch -D striatum/dogfood-004-claude-supervised-wrapper 2>/dev/null || true
```
