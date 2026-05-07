# Dogfood 001 — Add `--format dot` to `striatum workflow graph`

Status: ready to run
Date scaffolded: 2026-05-07

## Goal

Drive a real, small Striatum-on-Striatum run to discover V1 friction. The
point is **not** the change itself. The point is to drive the
draft → review → apply path with real agent CLIs and capture every piece of
friction as a `harness_improvement_proposal` artifact (RFC 0005).

## The change

Add Graphviz DOT export to `striatum workflow graph`, alongside the existing
Mermaid and JSON formats.

Acceptance:

- `striatum workflow graph --format dot <workflow.json>` emits valid Graphviz
  DOT (`digraph { ... }`).
- Output represents the same data as Mermaid: nodes per workflow job,
  dependency edges, parallel groups (as `subgraph cluster_<group>`), and
  bounded `needs_revision` cycle edges.
- A new test in `tests/test_cli_mvp.py` runs `dot -Tsvg` against the output
  and asserts non-zero exit only when `dot` is installed; otherwise it
  validates by parsing the DOT string.
- `docs/SPEC.md` and `README.md` mention the new format option.

## Before you start

Install agent CLIs you want to use as lanes. The workflow ships with two:

- `claude_code` — `claude` (Claude Code) on PATH
- `codex` — `codex` on PATH

You can run with one lane by deleting the other from `workflow.json` and the
review job that targets it.

Verify Striatum itself:

```bash
make test     # 142 should pass
```

## One-shot env (copy-paste at the start of your session)

```bash
cd ~/git/striatum
RUNNER=.venv/bin/striatum
WORKFLOW=docs/dogfood/001/workflow.json
TARGET_REPO=.                  # Striatum on Striatum
```

## Step-by-step

### 1. Initialize state

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" status --json
```

### 2. Validate and visualize the workflow

```bash
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json | head -40
```

The Mermaid graph is a quick sanity check on the workflow shape. The plan
shows claim waves, review gates, and the declared revision cycle.

### 3. Prepare a run, confirm a real branch, start

```bash
PREP=$("$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json)
RUN_ID=$(printf '%s' "$PREP" | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])')
echo "RUN_ID=$RUN_ID"

"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id "$RUN_ID" \
  --branch striatum/dogfood-001-graph-dot \
  --create \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id "$RUN_ID" --json
```

`--create` actually runs `git checkout -b` (round-5 work). If a branch with
that name already exists, it falls back to a plain checkout.

### 4. Register sessions

```bash
AUTHOR=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" \
  --role author --lane claude_code \
  --capability write --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
echo "AUTHOR=$AUTHOR"

REVIEWER=$("$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id "$RUN_ID" \
  --role reviewer --lane codex \
  --capability review --json | python3 -c \
  'import json,sys; print(json.load(sys.stdin)["data"]["session_id"])')
echo "REVIEWER=$REVIEWER"
```

### 5. Start the author supervisor

This is the round-5 auto-delivery path that has only been exercised by tests.
Real agent CLI behavior under a named-pipe stdin is the most likely
friction point in this session.

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise start \
  --session-id "$AUTHOR" --json
```

### 6. Watch from a second terminal

```bash
.venv/bin/striatum --repo . dashboard --run-id "$RUN_ID"
```

Press Ctrl-C to quit the dashboard.

### 7. Drive the workflow

The agent in the supervised lane reads work packets from its stdin pipe
when you call `claim-next`. The packet's `task_prompt.path` points at the
prompt under `docs/dogfood/001/prompts/`. The agent should:

1. Read the prompt and the referenced source files.
2. Make the change in `src/striatum/workflow.py`, `src/striatum/cli/parser.py`,
   `tests/test_cli_mvp.py`, `docs/SPEC.md`, `README.md`.
3. Write the draft commit description to the expected artifact path.
4. Call `striatum publish-artifact` with kind `handoff`.
5. Call `striatum complete`.

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$AUTHOR" --json
```

After the author finishes, the reviewer claims:

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id "$REVIEWER" --json
```

The reviewer publishes a `finding` artifact and submits a verdict via
`submit-review`.

### 8. Capture friction as you go

Whenever something is awkward, surprising, missing, or broken, file it
**immediately** as a harness improvement proposal. Use
`HARNESS_PROPOSAL_TEMPLATE.md` as the starting point. The runner will
validate `striatum.harness_improvement_proposal.v1` front matter when you
publish.

```bash
cp docs/dogfood/001/HARNESS_PROPOSAL_TEMPLATE.md \
   docs/dogfood/001/findings/HARNESS-001.md
# edit it
"$RUNNER" --repo "$TARGET_REPO" publish-artifact \
  --session-id "$AUTHOR" \
  --job-id "$AUTHOR_JOB_ID" \
  --lease-id "$AUTHOR_LEASE_ID" \
  --kind harness_improvement_proposal \
  --logical-name harness_001 \
  --path docs/dogfood/001/findings/HARNESS-001.md \
  --json
```

The artifact_kind validator (round-7 work) will catch typos in the kind name
and the front-matter validator will catch malformed `target` values, so
typos surface as a clean ArtifactError instead of a silent record.

### 9. Export evidence at the end

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id "$RUN_ID" \
  --path docs/dogfood/001/EVIDENCE.md --json

"$RUNNER" --repo "$TARGET_REPO" run summary \
  --run-id "$RUN_ID" \
  --path docs/dogfood/001/RUN_SUMMARY.md --json
```

Commit both. The redacted snapshot is the durable artifact of this session.

### 10. Stop cleanly

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise stop \
  --session-id "$AUTHOR" --reason "dogfood 001 done"
```

## Things I expect to break (capture as harness proposals)

These are surfaces that have only been exercised by fixtures/tests, never
by a real agent. Each is a candidate for a harness_improvement_proposal.

1. **Supervisor pipe vs TTY-required CLIs.** Does `claude -p` actually read
   from a named pipe? Some agent CLIs require a TTY for input. RFC 0009
   already calls PTY support an open question; file the exact symptom if
   it bites.

2. **Packet → agent context handoff.** The packet hands the agent
   `task_prompt.path`; the agent has to open and read that file itself.
   Awkward? Consider whether packets should inline the prompt body for
   short prompts.

3. **Dashboard under real load.** Refresh-loop reliability when SQLite is
   being written by a real concurrent process, not a test stub.

4. **Author byline accuracy.** When the agent writes the draft Markdown, is
   the recorded `author:` line right (role-model-ordinal)? Is the
   `display_model` in the lane config matching the agent's actual model?

5. **Checkpoint flow ergonomics.** If the author hits a question requiring
   human judgment, can it call `striatum block --severity human_checkpoint`
   cleanly, and can you then `decision record` + `checkpoint resolve
   --decision-id` to unblock?

6. **`workflow init` round-trip.** This dogfood scaffold was hand-written;
   try `striatum workflow init --style code-change docs/dogfood/002/` for
   the next session and see if the generated tree is actually usable.

## After the session

- Tag the snapshot: `git tag dogfood-001 -m "first V1 dogfood"`.
- Promote any high-signal harness proposals into RFCs (or TODO items).
- Scaffold dogfood-002 with whatever the next minimum-useful target is.
- If the supervisor pipe broke for a real CLI, file RFC 0010 (PTY supervisor).

## Reset cheat-sheet

If something gets stuck and you want to start over:

```bash
"$RUNNER" --repo "$TARGET_REPO" supervise stop --session-id "$AUTHOR" \
  --reason reset 2>/dev/null || true
rm -rf .striatum/
git checkout main
git branch -D striatum/dogfood-001-graph-dot 2>/dev/null || true
```

The state DB is local; nuking `.striatum/` is safe.
