---
name: striatum-dogfood-003
description: Run the Striatum dogfood-003 workflow for researching Codex, Claude Code, and Gemini CLI tool harness features, using native subagents for independent research where useful, then moving RFC 0010 through design synthesis, independent review, human acceptance, implementation, build review, evidence export, and supervisor cleanup.
---

# Striatum Dogfood 003

Use this skill from `/Users/halbritt/git/striatum` to start or drive
dogfood-003. This run researches tool-specific harness profiles and uses the
run itself to test the generic instruction: use native subagents for useful
independent research, while the parent Striatum session remains accountable.
RFC 0010 already includes concrete profile candidates and prior research
notes, so research jobs should verify and refresh those inputs rather than
starting cold.

## Ground Rules

- Do not edit `.striatum/` or SQLite directly.
- Starting a run creates claimable workflow state; it does not create an
  interactive orchestrator chat.
- Prefer official tool docs listed in `docs/dogfood/003/SOURCES.md`.
- Treat the workflow's `harness_profiles` map as an RFC 0010 fixture. It may
  be ignored by the current runner until the implementation job lands profile
  validation and packet exposure.
- Do not use direct `claude -p` as if it were a long-lived supervised lane;
  RFC 0010 says supervised Claude Code lanes need a wrapper.
- Capture runner, prompt, adapter, or delegation friction as
  `harness_improvement_proposal` artifacts.
- Do not let implementation proceed until a human acceptance decision has
  been recorded under `docs/dogfood/003/decisions/`.

## Quick Start

```bash
cd /Users/halbritt/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/003/workflow.json
TARGET_REPO=.
```

Verify:

```bash
make test
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -80
```

Prepare and start:

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-003-tool-harness-profiles \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

Register sessions for the jobs you will drive. Use the full command block in
`docs/dogfood/003/RUNBOOK.md` if you want every lane. The minimum variables
to remember are:

- `CODEX_RESEARCHER`
- `CLAUDE_RESEARCHER`
- `GEMINI_RESEARCHER`
- `DESIGNER`
- `DESIGN_REVIEWER`
- `IMPLEMENTER`
- `BUILD_REVIEWER`

Claim in this order:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$CODEX_RESEARCHER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$CLAUDE_RESEARCHER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$GEMINI_RESEARCHER" --json
```

Then, after the three research jobs complete:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGNER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGN_REVIEWER" --json
```

After design review accepts, record the human decision:

```bash
mkdir -p docs/dogfood/003/decisions
"$RUNNER" --repo "$TARGET_REPO" decision record \
  --run-id "$RUN_ID" \
  --path docs/dogfood/003/decisions/RFC_0010_ACCEPTANCE.md \
  --outcome accepted_with_follow_up \
  --title "Accept RFC 0010 first implementation slice" \
  --follow-up "Implement the reviewed V1 harness profile slice and defer provider-specific automation beyond work-packet guidance." \
  --json
```

Then build and review:

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
  --path docs/dogfood/003/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/003/RUN_SUMMARY.md --json
```

Use `docs/dogfood/003/RUNBOOK.md` for the full supervisor, friction capture,
and reset procedures.
