# Dogfood 001 v2 — Land HARNESS-001/002/003/004 fixes

Status: ready to run
Date scaffolded: 2026-05-07
Source synthesis: `docs/dogfood/001/SYNTHESIS.md`

## Goal

Land the cheap, high-signal layer of each of the four dogfood-001
harness improvement proposals so the next supervised dogfood (002 or
001 v3) can drive a workflow without falling back to operator-driven
recovery. The change being driven is the runner's own remediation
work; the meta-purpose (as always) is to surface any *new* friction
the v2 round introduces.

## Sub-targets (one bundled draft)

- **HARNESS-001** (defaults): SPEC contract subsection, doctor warning
  for orphaned-lease-on-dead-supervisor, `status` next_action,
  idempotent `supervise stop` against a lost supervisor. Working
  long-running supervised lane is **deferred** — that depends on RFC
  0010 (PTY supervisor + protocol skill).
- **HARNESS-002** (defaults): doctor warning for editable install
  outside repo, `init` guard against stale install, Makefile install
  path resolution.
- **HARNESS-003** (spec): SPEC reviewer-independence subsection,
  doctor warning for shared-pid sessions, `register-session
  --force-non-fresh --reason`, byline-missing recording.
- **HARNESS-004** (documentation): fix
  `docs/dogfood/001/roles/reviewer.md` to point at scope-valid path,
  audit other dogfood reviewer role docs.

See `docs/dogfood/001-v2/prompts/draft.md` for the per-HARNESS scope
cuts (in / out) and `docs/dogfood/001-v2/prompts/review.md` for the
gates.

## Before you start

Verify Striatum:

```bash
make test     # 143 should pass before v2; v2 adds ~7 new tests
```

Verify the editable install points at the canonical source — this is
the HARNESS-002 foot-gun, and skipping the check is exactly how
dogfood-001 stumbled into it:

```bash
.venv/bin/pip show striatum | grep "Editable project location"
# must print /home/halbritt/git/striatum (or wherever you cloned to)
```

If the path is anywhere else (e.g. `.claude/worktrees/agent-…/`),
re-install:

```bash
.venv/bin/pip install -e /home/halbritt/git/striatum
```

## One-shot env

```bash
cd /home/halbritt/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/001-v2/workflow.json
TARGET_REPO=.
```

## Step-by-step

### 1. Initialize state and validate

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW" --format dot
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -40
```

(The `--format dot` is a sanity check that dogfood-001's product
change is still working in this checkout.)

### 2. Prepare a run, confirm a real branch, start

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-001-v2-harness-fixes \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

### 3. Register sessions

```bash
AUTHOR=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" \
  --role author --lane claude_code \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')

REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" \
  --role reviewer --lane codex \
  --capability review --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
```

### 4. Drive the work

`docs/dogfood/001-v2/SKILL.md` is the agent-handoff entry point; pass
it to whichever agent is driving and let the skill orchestrate
`claim-next` → `ack` → make the changes → `publish-artifact` →
`complete`.

If HARNESS-001 lands cleanly during draft, try driving the review job
through a real codex `supervise start` — that's the first signal that
*any* supervised lane works. If codex's `exec -` mode also cannot
execute packets, capture the symptom as a v2-round HARNESS proposal
under `docs/dogfood/001-v2/findings/`.

### 5. Watch from a second terminal

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

### 6. Capture friction as you go

Author-side proposals: `docs/dogfood/001-v2/findings/HARNESS-NNN.md`.
Reviewer-side proposals: `docs/dogfood/001-v2/review/HARNESS-NNN.md`.
The split matches the two write_scopes (and is the reason HARNESS-004
exists in the first place).

### 7. Export evidence and stop

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

The `supervise stop` call should be idempotent against a lost
supervisor by the time v2 lands — that's HARNESS-001's idempotency
fix. If it still returns exit 4, that is itself a v2-round finding.

### 8. Final verification

```bash
make lint
make typecheck
make test
git status --short --branch
```

## Things I expect to surface (capture as harness proposals)

The four dogfood-001 findings should be addressed by the time v2
finishes. The candidate *new* friction:

1. **Codex `exec -` lane.** If the reviewer is driven through real
   codex supervision, does `codex exec -` actually consume newline-
   delimited packets and call back via `striatum`? If not, file
   HARNESS-005 with the exact symptom.

2. **Doctor diagnostic UX.** Are the new HARNESS-001/002/003 doctor
   problem records readable as a flat list, or do they need
   grouping/severity? If the operator has to mentally sort five
   warning kinds, doctor needs a verbosity flag or grouping.

3. **Test setup ergonomics for supervisor states.** The
   per-HARNESS-001 test fabricates `process_supervisors` rows by
   hand. If that boilerplate becomes substantial, it suggests a
   `tests/_supervise_helpers.py` should grow.

4. **`init` guard surface.** If the new `init` refusal triggers in
   any unexpected scenario (CI, `make smoke`, fresh clones), capture
   the surface and propose a workaround.

5. **Byline-missing recording downstream.** If the snapshot now
   carries `null` author lines, do `evidence export` and `run
   summary` render the missing case gracefully? If they crash or
   render `None` literally, the renderer needs a small fallback.

## Reset cheat-sheet

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise stop --session-id "$AUTHOR" \
  --reason reset 2>/dev/null || true
rm -rf .striatum/
git checkout main
git branch -D striatum/dogfood-001-v2-harness-fixes 2>/dev/null || true
```

## After the session

- Tag the snapshot: `git tag dogfood-001-v2 -m "harness fixes round 1"`.
- Promote any v2-round harness proposals into RFCs or TODO items.
- If a working supervised lane stayed deferred, scaffold dogfood-002
  to test it once RFC 0010 (PTY) lands.
