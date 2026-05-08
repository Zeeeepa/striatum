---
name: striatum-dogfood-005
description: Drive RFC 0014 V1 (process adapter completion guarantees) end-to-end. Research the current adapter path, synthesize the locked V1 design (envelope schema, blocker reasons, timeout flag, reconcile subcommand, doctor checks), review it, record human acceptance, implement the combined three-step build slice, review the build. Closes issue #1.
---

# Striatum Dogfood 005

Use this skill from `~/git/striatum` to start or drive dogfood-005.
Target: RFC 0014 V1 — post-exit output validation with a privacy-safe
diagnostic envelope, configurable `--timeout-seconds`, and
`recovery process-reconcile` + doctor checks. Closes
[issue #1](https://github.com/halbritt/striatum/issues/1).

## Ground Rules

- Do not edit `.striatum/state.sqlite3` directly.
- D028 stands: the diagnostic envelope contains zero child
  stdout/stderr — only metadata Striatum already collects plus
  output-validation deltas (missing artifact paths, missing verdict).
- The synthesis pins all wire-format contracts (envelope schema,
  blocker reason strings, event types, CLI flags). The implementer
  follows them verbatim.
- Capture friction as `harness_improvement_proposal` artifacts.
- The implementation job blocks until a human acceptance decision
  is recorded under `docs/dogfood/005/decisions/`.

## Quick Start

```bash
cd ~/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/005/workflow.json
TARGET_REPO=.
```

The workflow declares `branch.mode: "auto"`, so `run prepare`
creates the branch and transitions to `ready` in one step:

```bash
make test
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"
"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

Register sessions:

- `RESEARCHER` (researcher / codex)
- `DESIGNER` (designer / codex)
- `DESIGN_REVIEWER` (reviewer / claude_code)
- `IMPLEMENTER` (implementer / codex)
- `BUILD_REVIEWER` (reviewer / claude_code)

Claim sequentially per the workflow plan
(max_active_jobs: 1).

After the design review accepts:

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

Watch:

```bash
"$RUNNER" --repo "$TARGET_REPO" dashboard --run-id "$RUN_ID"
```

Finish:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/005/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/005/RUN_SUMMARY.md --json
```

Use `docs/dogfood/005/RUNBOOK.md` for the full procedure including
session registration commands, friction capture, and reset.
