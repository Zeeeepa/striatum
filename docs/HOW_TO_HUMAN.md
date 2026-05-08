# How To Drive striatum (Human Operator)

This is the operator's playbook. You run `striatum` commands; the
runner manages state. For the agent-facing equivalent, see
[HOW_TO_AGENT.md](HOW_TO_AGENT.md).

The examples assume you are in a striatum checkout and want to
orchestrate some target repository. Set these once:

```bash
RUNNER=.venv/bin/striatum
TARGET_REPO=/path/to/target/repo
WORKFLOW=examples/rfc-ledger-cleanup/workflow.json
```

Point `TARGET_REPO` at the repository you want to orchestrate. The
generic fixture writes under `docs/reviews/rfc-ledger/` in the
target repo, so use a scratch target if you only want to smoke-test
the runner.

## Initialize

```bash
"$RUNNER" --repo "$TARGET_REPO" init --json
"$RUNNER" --repo "$TARGET_REPO" status --json
"$RUNNER" --repo "$TARGET_REPO" doctor --json
```

This creates `.striatum/state.sqlite3` under the target repo and
adds `.striatum/` to that repo's `.gitignore`.

To also drop a self-contained agent skill bundle that teaches a
Striatum-aware agent how to drive the runner (RFC 0015 V1):

```bash
"$RUNNER" --repo "$TARGET_REPO" init --with-skills claude_code --json
# or, against an existing initialized repo:
"$RUNNER" --repo "$TARGET_REPO" skills install --profile claude_code --json
"$RUNNER" --repo "$TARGET_REPO" skills install --profile generic --json
```

The Claude Code profile writes five `SKILL.md` files under
`.claude/skills/striatum-*/`. The generic profile writes a single
`striatum-STRIATUM_AGENT_GUIDE.md` for any agent CLI without a
skill-discovery convention. Both profiles are byte-identical on
re-install; operator edits are preserved unless you pass `--force`.

## Author or validate a workflow

```bash
"$RUNNER" --repo "$TARGET_REPO" workflow validate "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow plan "$WORKFLOW" --json
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW"
"$RUNNER" --repo "$TARGET_REPO" workflow graph "$WORKFLOW" --format json --json
```

`workflow validate` checks required fields, role/lane references,
artifact paths, dependency edges, bounded cycles, declared
parallelism, and lane constraints. YAML files are rejected.
`workflow plan` returns a dry-run plan; `workflow graph` exports
Mermaid `flowchart TD` (default), JSON, or Graphviz DOT
(`--format dot`, pipe through `dot -Tsvg` to render).

To scaffold a new workflow tree:

```bash
"$RUNNER" workflow init --style review path/to/new-flow
```

`--style` accepts `minimal`, `review` (default), or `code-change`.
The generated tree includes `workflow.json` plus `roles/` and
`prompts/` stubs and validates cleanly. The command refuses to
overwrite an existing path.

For deeper authoring guidance, see
[WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md).

## Prepare a run

```bash
"$RUNNER" --repo "$TARGET_REPO" run prepare --workflow "$WORKFLOW" --json
```

Copy the returned `run_id` for later commands. The run is now
prepared but not claimable.

## Confirm the branch and start

```bash
"$RUNNER" --repo "$TARGET_REPO" branch confirm \
  --run-id <run_id> \
  --branch striatum/rfc-ledger-cleanup \
  --json

"$RUNNER" --repo "$TARGET_REPO" run start --run-id <run_id> --json
```

Add `--create`, `--use-current`, or `--strict` to drive git
instead of just recording (see [SPEC.md § Branches And
Commits](SPEC.md#branches-and-commits)).

## Register a session

```bash
"$RUNNER" --repo "$TARGET_REPO" register-session \
  --run-id <run_id> \
  --role author \
  --lane codex \
  --capability write \
  --json
```

Copy the returned `session_id`. The display slug looks like
`author-codex-1`.

## Claim and acknowledge work

```bash
"$RUNNER" --repo "$TARGET_REPO" claim-next --session-id <session_id> --json
```

If work is available, the response includes a `packet` with
`job_id`, `message_id`, `lease_id`, expected artifacts, write
scope, task prompt, and the commands to use. Expected artifact
metadata includes a privacy-safe lowercase byline such as
`author: author-codex-gpt-5.5-001`. Put that exact line near the
top of any workflow-authored Markdown artifact.

If the work packet contains `worktree_required: true`, run the
suggested `striatum worktree create` command before publishing.

After reading the packet:

```bash
"$RUNNER" ack \
  --session-id <session_id> --message-id <message_id> --lease-id <lease_id> \
  --json

"$RUNNER" heartbeat \
  --session-id <session_id> --lease-id <lease_id> --extend-seconds 1800 --json
```

## Publish artifacts and complete non-review work

```bash
"$RUNNER" publish-artifact \
  --session-id <session_id> --job-id <job_id> --lease-id <lease_id> \
  --kind handoff \
  --logical-name draft \
  --path docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md \
  --json

"$RUNNER" complete \
  --session-id <session_id> --job-id <job_id> --lease-id <lease_id> \
  --summary "Draft artifact published." --json
```

Completion may enqueue downstream jobs once dependencies are
satisfied.

## Submit review work

```bash
"$RUNNER" submit-review \
  --session-id <review_session_id> \
  --job-id <review_job_id> \
  --lease-id <review_lease_id> \
  --path docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md \
  --verdict accept_with_findings --json
```

`--verdict` accepts `accept`, `accept_with_findings`,
`needs_revision`, or `reject`. For unusual flows, call
`publish-artifact` and `verdict` separately.

## Record owner decisions

```bash
"$RUNNER" decision record \
  --run-id <run_id> \
  --path docs/decisions/owner-choice.md \
  --outcome accepted_with_follow_up \
  --title "Keep decisions as durable artifacts" \
  --follow-up "Review fuller decision schemas later." \
  --json
```

The generated Markdown includes machine-checkable front matter and
is recorded as artifact kind `decision`. `decision record` does
not require an active lease.

## Report a blocker

```bash
"$RUNNER" block \
  --session-id <session_id> --job-id <job_id> --lease-id <lease_id> \
  --kind missing_input \
  --severity human_checkpoint \
  --description "Need human decision before continuing." \
  --json
```

Use `--severity blocked` for normal blockers and `human_checkpoint`
when the run needs explicit human judgment.

To resolve a `human_checkpoint` blocker explicitly once the
operator has decided, use `striatum checkpoint resolve`:

```bash
# Continue: closes the blocker and returns the affected job to the queue.
"$RUNNER" --repo "$TARGET_REPO" checkpoint resolve \
  --blocker-id <blocker_id> \
  --action continue \
  --decision-id <decision_id> \
  --json

# Cancel: closes the blocker and cancels the affected job.
"$RUNNER" --repo "$TARGET_REPO" checkpoint resolve \
  --blocker-id <blocker_id> \
  --action cancel \
  --json
```

`--decision-id` is optional but recommended. When present, it must
reference an existing run-level decision artifact recorded with
`striatum decision record`; the resolution event payload then links
back to that artifact for audit.

## Inspect, watch, and export evidence

```bash
"$RUNNER" status --run-id <run_id> --json
"$RUNNER" why <blocker_or_job_or_artifact_id> --json
"$RUNNER" doctor --run-id <run_id> --verbose --json
"$RUNNER" dashboard --run-id <run_id>           # live; --once for one frame
"$RUNNER" run summary --run-id <run_id> --path docs/reviews/RUN_SUMMARY.md
"$RUNNER" evidence export \
  --run-id <run_id> --path docs/reviews/rfc-ledger/RUN_EVIDENCE.md --json
```

To explicitly cancel a non-terminal job (and optionally its
blocked-only-through-this dependents), use `striatum recovery
cancel-job`:

```bash
"$RUNNER" --repo "$TARGET_REPO" recovery cancel-job \
  --run-id <run_id> \
  --job-id <job_id> \
  --reason "operator chose to abandon this work" \
  --cascade \
  --json
```

Without `--cascade` the command refuses with exit code 4 if the
job has blocked dependents whose only path was through it; rerun
with `--cascade` to cancel them transitively in the same
transaction. Terminal-state jobs (`completed`, `failed`,
`canceled`, `skipped`) cannot be canceled.

## Dashboards and graphs

For a compact at-a-glance view of a run, use the dashboard. It is
a dependency-free terminal renderer over the same SQLite state
that `status` exposes:

```bash
"$RUNNER" --repo "$TARGET_REPO" dashboard --run-id <run_id>
```

The dashboard refreshes every 2 seconds by default and clears the
screen between frames. It shows run state and branch, job counts
by state, verdict counts, open blockers (including human
checkpoints), claimable work grouped by role/lane, deterministic
next actions, and the most recent events. Use `Ctrl-C` to quit.

Useful flags:

- `--refresh <seconds>`: change the refresh cadence.
- `--once`: render a single frame to stdout and exit. Handy in
  scripts and CI assertions where a redrawing TUI is not what you
  want.
- `--graph` / `--no-graph`: force the layered dependency-graph
  panel on or off. Default is auto: rendered when the terminal is
  at least 100 columns wide and 30 lines tall and the workflow has
  at least one edge.
- `--graph-only`: hide the rest of the frame and show only the
  graph.
- `--graph-style {auto,layered,list,fancy}`: pick a layout.
  `fancy` falls back to `layered` until the Unicode box-drawing
  follow-up lands.
- `--graph-no-cycles`: suppress dashed `~~>` back-edges for
  revision cycles.

For a one-shot snapshot outside the dashboard, use
`striatum run graph --run-id <id> --format ascii`; it reuses the
same renderer.

To publish a redacted run snapshot:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id <run_id> \
  --path docs/reviews/rfc-ledger/RUN_EVIDENCE.md \
  --json
```

The export path must be inside the repository and outside
`.striatum/`.

## See also

- **[CLI_REFERENCE.md](CLI_REFERENCE.md)** — every verb in one
  flat list with stable exit codes.
- **[WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md)** — author your
  own `workflow.json`.
- **[SPEC.md](SPEC.md)** — the implementation contract; the
  source of truth when this doc and the runner disagree.
