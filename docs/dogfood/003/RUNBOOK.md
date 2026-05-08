# Dogfood 003 - Tool Harness Profiles

Status: scaffolded
Date scaffolded: 2026-05-07
Last aligned with RFC 0010: 2026-05-08

## Goal

Use Striatum itself to take RFC 0010 from refreshed tool research through
design, independent review, human acceptance, implementation, and build
review.

This dogfood run is explicitly about the tool-harness idea: each research job
asks its agent to use native subagents or equivalent delegation for independent
research where useful, while Striatum keeps the parent session accountable for
artifacts and workflow state.

RFC 0010 already contains concrete profile candidates and prior research
under `docs/research/0010-tool-harness-profiles/`. Dogfood-003 should verify
and update those inputs, not rediscover them from scratch.

## Workflow Shape

```text
parallel tool research (Codex, Claude Code, Gemini CLI)
  -> design synthesis
  -> design review
  -> human acceptance decision
  -> implementation
  -> build review
```

The human acceptance decision is recorded with `striatum decision record`
after design review accepts. The implementation prompt is instructed to block
with a human checkpoint if that decision artifact is missing.

The workflow fixture includes RFC 0010's proposed `harness_profiles` map and
lane `harness_profile_id` references. Current Striatum may ignore those fields
until the implementation job lands validation and work-packet exposure; that
is intentional fixture pressure for the RFC 0010 build slice.

## Before You Start

For a concise agent handoff, give the agent `docs/dogfood/003/SKILL.md` and
ask it to use the skill to start or drive dogfood-003.

Install the agent CLIs you want to use as lanes. The workflow declares:

- `codex` - `codex exec --json --ephemeral ... -` with per-job `CODEX_HOME`
- `claude_code` - `.striatum/bin/claude-supervised-wrapper.sh`
- `gemini` - `gemini --prompt - --output-format stream-json --approval-mode auto_edit`

If one lane is unavailable, either edit `workflow.json` to remove that lane
and its research job, or let validation/claiming fail and capture the friction
as a `harness_improvement_proposal`.

The Claude Code wrapper is a proposed RFC 0010/RFC 0009 harness shape. If it
does not exist yet, do not pretend the direct `claude -p` mode is supervised;
run that lane manually or capture the missing wrapper as dogfood evidence.

Verify Striatum:

```bash
make test
```

## One-Shot Environment

```bash
cd ~/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/003/workflow.json
TARGET_REPO=.
```

## Initialize And Inspect

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -80
```

## Prepare, Confirm Branch, Start

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-003-tool-harness-profiles \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

## Register Sessions

```bash
CODEX_RESEARCHER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role researcher --lane codex \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

CLAUDE_RESEARCHER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role researcher --lane claude_code \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

GEMINI_RESEARCHER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role researcher --lane gemini \
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

## Drive Research

For supervised lanes, start supervisors before claiming only when the declared
lane command is actually long-lived or wrapped for RFC 0009. This dogfood run
is allowed to discover that a tool command exits after one packet; capture
that as harness friction rather than hiding it.

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise start --session-id "$CODEX_RESEARCHER" --json
"$RUNNER" --repo "$TARGET_REPO" supervise start --session-id "$CLAUDE_RESEARCHER" --json
"$RUNNER" --repo "$TARGET_REPO" supervise start --session-id "$GEMINI_RESEARCHER" --json
```

Claim the three research jobs:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$CODEX_RESEARCHER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$CLAUDE_RESEARCHER" --json
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$GEMINI_RESEARCHER" --json
```

Each research prompt asks the agent to verify the existing RFC 0010 research
and try native subagents or equivalent delegation for independent research
subtasks. The parent session still owns the final artifact and Striatum
commands.

Watch progress:

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

## Design, Review, And Acceptance

After all three research jobs complete:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGNER" --json
```

After the design synthesis completes:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$DESIGN_REVIEWER" --json
```

If the design review accepts, record human acceptance before implementation.
Dogfood-003 keeps this as an owner decision artifact rather than a first-class
workflow job so it can exercise `decision record` without making hidden native
subagents first-class Striatum actors:

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

## Build And Review

After the decision artifact exists:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$IMPLEMENTER" --json
```

After implementation completes:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$BUILD_REVIEWER" --json
```

## Capture Harness Friction

Whenever the workflow, prompt, supervisor, adapter, or subagent instruction
feels wrong, capture it immediately:

```bash
mkdir -p docs/dogfood/003/findings
cp docs/dogfood/003/HARNESS_PROPOSAL_TEMPLATE.md \
   docs/dogfood/003/findings/HARNESS-001.md
```

Edit the file, then publish it from the active job:

```bash
"$RUNNER" --repo "$TARGET_REPO" publish-artifact \
  --session-id "$SESSION_ID" \
  --job-id "$JOB_ID" \
  --lease-id "$LEASE_ID" \
  --kind harness_improvement_proposal \
  --logical-name harness_001 \
  --path docs/dogfood/003/findings/HARNESS-001.md \
  --json
```

## Finish

Export durable run evidence:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/003/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/003/RUN_SUMMARY.md --json
```

Stop any supervisors you started:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise list --run-id "$RUN_ID" --json
"$RUNNER" --repo "$TARGET_REPO" supervise stop --session-id "$CODEX_RESEARCHER" --reason "dogfood 003 done" --json
"$RUNNER" --repo "$TARGET_REPO" supervise stop --session-id "$CLAUDE_RESEARCHER" --reason "dogfood 003 done" --json
"$RUNNER" --repo "$TARGET_REPO" supervise stop --session-id "$GEMINI_RESEARCHER" --reason "dogfood 003 done" --json
```

Run final checks:

```bash
make lint
make typecheck
make test
git status --short --branch
```

## Reset

Only after confirming local runner state and the dogfood branch can be thrown
away:

```bash
rm -rf .striatum/
git checkout main
git branch -D striatum/dogfood-003-tool-harness-profiles 2>/dev/null || true
```
