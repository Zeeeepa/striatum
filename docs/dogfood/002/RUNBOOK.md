# Dogfood 002 — Land RFC 0011 (session close + run-terminal auto-close)

Status: ready to run
Date scaffolded: 2026-05-08
Source RFC: `docs/rfcs/0011-session-close-and-run-terminal-auto-close.md`

## Goal

Implement RFC 0011 end to end. The change closes the gap that
dogfood-001 surfaced and the v2 SYNTHESIS captured: there is no CLI
surface that transitions a session out of `active`, so the
`active_session_on_terminal_run` doctor warning fires permanently on
every clean-finish run.

The acceptance criterion that demands the most attention: dogfood-002's
*own* run, after the apply job completes, must produce
`doctor ok=true`. That is the in-the-loop validation — RFC 0011 is
shipping when this dogfood's evidence shows clean closure.

## Sub-targets

- Migration version 7: `closed` state value, `closed_at` and
  `close_reason` columns.
- `striatum session close --session-id --reason` CLI: idempotent,
  refuses against an active lease.
- `close_remaining_sessions` helper, threaded into every run-terminal
  transition (complete, submit-review final accept, recovery
  cancel-job).
- `evidence_session_summaries` + `## Sessions` block in the run
  summary.
- Seven RFC-0011 acceptance tests.
- SPEC + CHANGELOG entries; DECISION_LOG entry on apply.

See `docs/dogfood/002/prompts/draft.md` for per-deliverable detail.

## Before you start

```bash
make test     # 151 should pass before 002; 002 adds 7 new tests
.venv/bin/pip show striatum | grep "Editable project location"
# must point at /home/halbritt/git/striatum (or wherever you cloned)
```

If the editable install points elsewhere, re-install:

```bash
.venv/bin/pip install -e /home/halbritt/git/striatum
```

## One-shot env

```bash
cd /home/halbritt/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/002/workflow.json
TARGET_REPO=.
```

## Step-by-step

### 1. Validate

```bash
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -40
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW" --format dot
```

### 2. Prepare a run

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-002-session-close \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

### 3. Register sessions

```bash
AUTHOR=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role author --lane claude_code \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" --role reviewer --lane codex \
  --capability review \
  --force-non-fresh --reason "operator-driven; supervised lane work deferred to a future RFC" \
  --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
```

`--force-non-fresh --reason` is required by HARNESS-003 for any
operator-driven reviewer registration.

### 4. Drive the work

`docs/dogfood/002/SKILL.md` is the agent-handoff entry point. The
draft prompt enumerates the seven acceptance criteria and the seven
prescribed tests; the review prompt walks each one as a gate.

### 5. Watch from a second terminal

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

### 6. Capture friction

Author-side: `docs/dogfood/002/findings/HARNESS-NNN.md`.
Reviewer-side: `docs/dogfood/002/review/HARNESS-NNN.md`.

### 7. Export evidence and stop

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/002/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/002/RUN_SUMMARY.md --json
```

After RFC 0011 lands, no `striatum supervise stop` call is required
to reach `doctor ok=true` — auto-close handles the session
disposition. Stopping the supervised lane is still recommended if one
was started:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise stop \
  --session-id "$AUTHOR" \
  --reason "dogfood 002 done" --json
```

The HARNESS-001 idempotency means this is safe even if the supervisor
already exited.

### 8. Final verification

```bash
make lint
make typecheck
make test
"$RUNNER" --repo . doctor --run-id "$RUN_ID" --verbose
git status --short --branch
```

`doctor` should now return `ok: true` — that's the in-the-loop
validation of the auto-close behavior.

## Things I expect to surface (capture as harness proposals)

1. **Auto-close interaction with existing tests.** The 151 baseline
   tests do not register sessions in `closed` state because no path
   created that state. After auto-close lands, every test that drives
   a run to completion will see closed sessions in `doctor` output.
   If any existing assertion breaks, capture as friction.
2. **Idempotent close vs. event ordering.** The
   `_latest_terminal_supervisor` pattern returns `note` and skips the
   normal path; `close_session` should mirror that. Double-events on
   the second close are a regression worth flagging.
3. **Evidence export schema bump.** Adding a `"sessions"` key to the
   evidence snapshot is technically a schema change. If
   `striatum.evidence.v1` is meant to be stable, the addition should
   land as `v1.1` or similar; if not, document the decision.
4. **Doctor warning rarity.** Once auto-close ships, the
   `active_session_on_terminal_run` warning fires only for genuinely
   anomalous states. If the existing test
   `test_init_status_and_doctor` (or any doctor smoke) depends on a
   clean ok-true after fresh init, it should keep passing — but worth
   double-checking.

## Reset cheat-sheet

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise stop --session-id "$AUTHOR" \
  --reason reset 2>/dev/null || true
rm -rf .striatum/
git checkout main
git branch -D striatum/dogfood-002-session-close 2>/dev/null || true
```

## After the session

- Tag the snapshot: `git tag dogfood-002 -m "land RFC 0011"`.
- Mark RFC 0011 accepted in `docs/rfcs/0011-session-close-and-run-terminal-auto-close.md`
  and the rfcs README index (apply step does this).
- Add the DECISION_LOG entry (apply step does this).
- Promote any new harness proposals into RFCs or TODO items.
- Scaffold dogfood-003 with the next minimum-useful target.
