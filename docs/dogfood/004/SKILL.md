---
name: striatum-dogfood-004
description: Run the Striatum dogfood-004 workflow for authoring the Claude Code supervised wrapper script (.striatum/bin/claude-supervised-wrapper.sh) — research named-pipe stdin behavior, synthesize a wrapper design, review it, record human acceptance, implement the wrapper plus a verification test, and review the build.
---

# Striatum Dogfood 004

Use this skill from `~/git/striatum` to start or drive dogfood-004. The
target is the V2 step of `docs/dogfood/003/findings/HARNESS-001.md`:
ship a wrapper that lets Claude Code run as an RFC 0009 supervised lane
(reading newline-delimited JSON packets from stdin), and verify its
behavior under `os.mkfifo`-backed supervisor pipes.

## Ground Rules

- Do not edit `.striatum/state.sqlite3` or any SQLite directly.
- The wrapper itself lives under `.striatum/bin/`; that path is
  carved out of the implementer's `forbidden_paths` so the wrapper
  can be created. It is the only `.striatum/` write the workflow
  permits.
- Pipe behavior under `os.mkfifo` is the riskiest unknown. The
  research and design jobs should land a verification approach
  before the implementer ships the script.
- Capture friction as `harness_improvement_proposal` artifacts —
  this run is itself the V2 follow-up of one such proposal.
- Implementation must block until a human acceptance decision is
  recorded under `docs/dogfood/004/decisions/`.

## Quick Start

```bash
cd ~/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/004/workflow.json
TARGET_REPO=.
```

Verify:

```bash
make test
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
```

The validate output includes the V1.5 lint warning naming the missing
wrapper path. That warning is expected — landing the wrapper is the
goal of the run.

Prepare and start. The workflow declares `branch.mode: "auto"`, so
`run prepare` creates the branch and transitions the run to `ready`
in one step:

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

Register sessions for the lanes you'll drive. Minimum:

- `RESEARCHER`
- `DESIGNER`
- `DESIGN_REVIEWER`
- `IMPLEMENTER`
- `BUILD_REVIEWER`

Claim in order:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$RESEARCHER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGNER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGN_REVIEWER" --json
```

After design review accepts, record the human decision:

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

Then implement and review:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$IMPLEMENTER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$BUILD_REVIEWER" --json
```

Watch:

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

Finish:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/004/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/004/RUN_SUMMARY.md --json
```

Use `docs/dogfood/004/RUNBOOK.md` for the full registration commands,
friction-capture template, and reset procedures.
