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
OUTPUT_DIR=striatum/rfc-ledger    # see "Where artifacts land" below
```

Point `TARGET_REPO` at the repository you want to orchestrate.

`OUTPUT_DIR` is **not** a runner setting — striatum has no
"output directory" flag. The runner accepts each artifact's path
verbatim from the *workflow file* (every job declares its
`expected_artifacts[].path` and `write_scope.allowed_paths`). The
shell variable below is just a convenience for the example
commands; in your own workflow you choose where artifacts land
when you author it.

For first-contact use, pick an output directory that is easy to
delete if you change your mind (see
[Where artifacts land](#where-artifacts-land)). The example
workflow `examples/rfc-ledger-cleanup/workflow.json` declares
`docs/reviews/rfc-ledger/` as its output root; if you don't want
that path created in your target repo, point `WORKFLOW` at a
different fixture or copy the example into a scratch tree first.

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
# Claude Code: writes five SKILL.md files under .claude/skills/striatum-*/
"$RUNNER" --repo "$TARGET_REPO" init --with-skills claude_code --json

# Codex CLI: writes five flat files under .codex/agents/striatum-*.md
"$RUNNER" --repo "$TARGET_REPO" init --with-skills codex --json

# Gemini CLI: writes one striatum-STRIATUM_GEMINI_GUIDE.md at the repo root
"$RUNNER" --repo "$TARGET_REPO" init --with-skills gemini --json

# Anything else: writes one striatum-STRIATUM_AGENT_GUIDE.md at the repo root
"$RUNNER" --repo "$TARGET_REPO" init --with-skills generic --json

# All four at once (deterministic order, disjoint paths):
"$RUNNER" --repo "$TARGET_REPO" init --with-skills all --json
```

V1.2 ships four skill profiles plus an `all` fan-out:
`claude_code` (auto-discovered by Claude Code), `codex` (flat
agent docs at `.codex/agents/`), `gemini` (single guide; will
graduate to a five-file shape once Gemini CLI's skill-discovery
convention stabilizes), and `generic` (single Markdown guide for
any other CLI). All profiles are byte-identical on re-install;
operator edits are preserved unless you pass `--force`.

To also scaffold the seven canonical human-facing DDD documents
(RFC 0021) into the target repo's `docs/`:

```bash
# Combined first-time setup (recommended): agent skills + DDD docs.
"$RUNNER" --repo "$TARGET_REPO" init \
  --with-skills claude_code \
  --with-ddd-layout \
  --json

# Preview what would be written (RFC 0021 V1.5):
"$RUNNER" --repo "$TARGET_REPO" init \
  --with-ddd-layout --ddd-layout-dry-run --json

# Force-overwrite existing regular-file targets, recording
# prior_sha256 for audit (RFC 0021 V1.5):
"$RUNNER" --repo "$TARGET_REPO" init \
  --with-ddd-layout --ddd-layout-force --json
```

`--with-ddd-layout` writes `docs/SPEC.md`, `docs/PRD.md`,
`docs/DECISION_LOG.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
`docs/DDD.md`, `docs/rfcs/README.md`, and
`docs/rfcs/0001-template.md`. Existing files are reported as
`skipped` with `reason: "exists"` (or `would_skip` under
`--ddd-layout-dry-run`). Non-regular-file targets (directories,
broken symlinks) are reported as `error` and are not touched
even with `--ddd-layout-force`.

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
  --path "$OUTPUT_DIR/RFC_LEDGER_DRAFT.md" \
  --json

"$RUNNER" complete \
  --session-id <session_id> --job-id <job_id> --lease-id <lease_id> \
  --summary "Draft artifact published." --json
```

`--path` must match the workflow's declared
`expected_artifacts[].path` for the job exactly. The publisher
refuses any path outside `write_scope.allowed_paths` with exit
code 6.

Completion may enqueue downstream jobs once dependencies are
satisfied.

## Submit review work

```bash
"$RUNNER" submit-review \
  --session-id <review_session_id> \
  --job-id <review_job_id> \
  --lease-id <review_lease_id> \
  --path "$OUTPUT_DIR/codex/RFC_LEDGER_REVIEW.md" \
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
"$RUNNER" run summary --run-id <run_id> --path "$OUTPUT_DIR/RUN_SUMMARY.md"
"$RUNNER" evidence export \
  --run-id <run_id> --path "$OUTPUT_DIR/RUN_EVIDENCE.md" --json
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

For unattended runs across **multiple** registered repositories,
prefer `striatum daemon start` (RFC 0028 V1) which sweeps recovery
across every active run in every registered repo from one
foreground process — see "Daemon / multi-repo coordination"
below.

For unattended runs against a **single** repo, `recovery watch`
is a foreground daemon that wraps `recovery auto` in a sleep
loop. One pidfile per run
(`.striatum/scratch/recovery-watch-<run_id>.pid`); `SIGTERM` /
`SIGINT` shuts it down cleanly. Exits when the run reaches a
terminal state by default.

```bash
"$RUNNER" --repo "$TARGET_REPO" recovery watch \
  --run-id <run_id> \
  --interval-seconds 60 \
  --json | tee "$OUTPUT_DIR/watch.jsonl"
```

Add `--max-sweeps N` to cap iterations (useful for tests and
probes), `--no-exit-on-terminal` to keep looping past terminal,
or any of the same overrides as `recovery auto`
(`--autonomous-review-requeue`, `--checkpoint-timeout`, etc.).
A pidfile collision with an alive watcher exits 4 with a
documented message; stale pidfiles (dead PIDs) are overwritten
cleanly.

## Daemon / multi-repo coordination (RFC 0028 V1)

`striatum daemon` adds optional registry-backed coordination
across multiple repositories without changing direct
repo-local CLI behavior. In V1 the daemon is a foreground
sweep process plus a shared owner-only registry SQLite; CLI
and MCP clients open the registry directly under token /
capability checks. There is **no daemon RPC server** in V1
and the Unix socket bound by `striatumd` is a lifecycle
marker, not a request router.

Start the daemon and register two target repos:

```bash
# Foreground sweep process (also exposed as `striatumd`).
"$RUNNER" daemon start --json &

# Register repos. The first `repo add` (or `daemon start`)
# bootstraps one admin token and writes a 0600 fallback file
# under the runtime directory; treat that file as degraded
# storage compared with an OS keyring.
"$RUNNER" repo add /path/to/repo-a --json
"$RUNNER" repo add /path/to/repo-b --json   # repeat per repo

"$RUNNER" repo list --json
```

`repo add` is admin-gated. It canonicalizes the repository
root, refuses symlink/path-traversal ambiguity (including
symlinked parent components or state-database symlink
escapes), derives a realpath/inode-based repository identity,
and refuses active path re-occupation by a different
identity. If `.striatum/state.sqlite3` is absent the command
refuses unless `--init` is passed; `--no-migrate` refuses
registration when repo-local migrations would be needed.

`repo remove <path>` is idempotent, revokes live repo-scoped
capabilities, preserves audit rows, and never reuses
`repository_id` (re-adding allocates a fresh id).

Read across registered repos with `--daemon`:

```bash
"$RUNNER" --daemon status --json
"$RUNNER" --daemon doctor --json
"$RUNNER" --daemon why <job-or-blocker-id> --json
"$RUNNER" --daemon dashboard --all
```

`--daemon` (or `STRIATUM_DAEMON=1`) opens the registry
SQLite under a read token. V1 read surfaces supported under
`--daemon`: `status`, `doctor`, `why`, `dashboard --all`. The
CLI refuses (does not silently fall back to direct mode) on
forced-daemon verbs that are not registry-backed.
`--no-daemon` forces direct repo-local mode.

`dashboard --all` is registry-backed even without `--daemon`
because it fans out across the registry; it requires the same
`read` token bootstrapped by `repo add` / `daemon start`.

Audit shape:

- Audit rows are metadata-only: command, authorization
  result, client/repository ids when known, payload hash,
  and a continuous hash chain across retained rows.
- Closed segment manifests are SQL-guarded against
  daemon-API rewrites and checked by `daemon doctor`.
- Audit deliberately excludes transcripts, request/response
  bodies, artifact text, blocker prose, token secrets, and
  tracebacks. It is per-machine daemon evidence, not
  transcript evidence or authorship proof.

What V1 daemon mode does **not** do:

- It does not host a daemon RPC server. CLI and MCP clients
  open the registry SQLite directly.
- It does not own workflow mutations. Ordinary
  `register-session`, `claim-next`, `publish-artifact`,
  `verdict`, `complete`, and `recovery` calls continue to
  mutate repo-local SQLite directly.
- It does not own supervised processes. `striatum supervise
  start` still spawns lane processes under repo-local
  bookkeeping.
- It does not ship Windows daemon support, sealed-apply
  authority, signing keys, mutation MCP tools, hosted
  semantics, cross-repository workflows, or remote serving.

## Cross-repo workflow foundation

RFC 0032 adds the V2 foundation for local cross-repo workflows on the
daemon PostgreSQL substrate. A cross-repo workflow declares at least two
registered repositories:

```json
{
  "repositories": {
    "primary": {"repo_id": "repo_primary"},
    "consumer": {"repo_id": "repo_consumer"}
  },
  "primary_repository": "primary"
}
```

Every job in that workflow declares its `repository` alias explicitly.
Artifact paths and write scopes are interpreted relative to that job's
target repo. The daemon DB owns the `cross_repo_run_id`; each
participant repo keeps its own repo-local run state and records a
`runs.cross_repo_run_id` back-reference.

Operator inspection commands:

```bash
"$RUNNER" cross-repo list --json
"$RUNNER" cross-repo describe <cross_repo_run_id> --json
"$RUNNER" cross-repo why <cross_repo_run_id> --json
```

If a participant repository disappears mid-run, the daemon pauses the
cross-repo run and the operator must re-register the same repository id
or cancel the run. Cross-repo coordination is best-effort local
orchestration, not atomic file mutation across repositories.

Daemon MCP mutation tools follow least privilege. A read-only token sees
only read tools. Mutation grants (`write`, `review`, `claim`, `apply`,
`recovery`, or `admin`) must be granted deliberately, should usually be
short-lived, and are re-checked on every `tools/call`. Prompt-injected
tool arguments cannot escalate beyond the token's grants.
- It is not a replacement for `recovery watch` against a
  single repo, only for multi-repo sweeping.

### Daemon V2 storage substrate (RFC 0033)

RFC 0033 accepts system PostgreSQL as the daemon-owned storage
substrate for V2 daemon-global state. This is a daemon storage
cutover only: repo-local `.striatum/state.sqlite3` remains the
live run state for each target repository, and ordinary workflow
mutations continue to use the repo-local path until RFC 0030
ships daemon RPC routing.

The operator provides PostgreSQL. Striatum connects through
`STRIATUM_DAEMON_DB_URL`, daemon config, or an explicit
`--postgres-url` client surface; the daemon owns schema
migrations and roles, but it does not install, start, stop, or
upgrade PostgreSQL. Bundled, embedded, and Dockerized Postgres
distributions are deferred.

Cut over a V1 daemon registry with:

```bash
# Inspect what would be imported.
"$RUNNER" daemon migrate --from sqlite --to pg --dry-run --json

# Import the V1 registry into the V2 daemon DB.
"$RUNNER" daemon migrate --from sqlite --to pg --json
```

The migration writes the V2 schema, imports registered
repositories, clients, capabilities, scheduler cursors, and
metadata-only audit rows, verifies hash continuity, and writes a
cutover marker. Once the marker exists, V1 registry reads are
refused and operators are pointed at the V2 daemon DB. Add
`--keep-sqlite-readonly` when you want to retain the old registry
file as an audit tombstone while blocking V1 writes.

RFC 0030/0031 add the daemon V2 RPC and supervision/apply foundation on
top of this storage substrate. The daemon RPC envelope is versioned JSON;
`daemon.hello` / `daemon.welcome` negotiate envelope and framing, and
`daemon.describe` publishes the capability-bound method registry. Version
or framing incompatibility refuses with exit code 10 and does not silently
fall back to direct mode.

Daemon-owned supervision is represented by daemon DB supervisor rows plus
repo-local supervisor pointers. The compatibility `striatum supervise`
verbs still work in direct mode, and future daemon-routed `supervise.*`
calls use the same packet/FIFO and lane-attestation invariants.

Sealed apply is intentionally fail-closed. `apply.reviewed_patch` requires
daemon apply authority and a loadable daemon signing key, and it records
apply receipts in daemon-owned state. The receipt is an AI guardrail: it
does not prove model-token authorship or resistance to a malicious local
operator with filesystem or database access.

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
  `fancy` uses Unicode box-drawing characters (`┌`, `┐`, `└`,
  `┘`, `─`, `│`); falls back to `layered` ASCII when the
  per-slot width drops below 14 columns.
- `--graph-orient {tb,lr}`: top-to-bottom (default) or
  left-to-right. LR arranges layers as columns instead of
  rows; useful for long workflow chains. Falls back to TB when
  too many layers don't fit horizontally.
- `--graph-no-cycles`: suppress dashed `~~>` back-edges for
  revision cycles (or `╌╌▶` in fancy mode).

For a one-shot snapshot outside the dashboard, use
`striatum run graph --run-id <id> --format ascii`; it reuses the
same renderer and accepts the same `--graph-style` and
`--graph-orient` flags.

To publish a redacted run snapshot:

```bash
"$RUNNER" --repo "$TARGET_REPO" evidence export \
  --run-id <run_id> \
  --path "$OUTPUT_DIR/RUN_EVIDENCE.md" \
  --json
```

The export path must be inside the repository and outside
`.striatum/`.

## Where artifacts land

striatum has no built-in concept of a "default output
directory." Every output path is named in the workflow file:

- Each job's `expected_artifacts[].path` is the *exact*
  repo-relative path the publisher writes to.
- Each job's `write_scope.allowed_paths` is the *set of
  prefixes* the agent may write inside.
- `evidence export` and `run summary` use the path you pass on
  the command line; they have to be inside the repo and outside
  `.striatum/`.

So if you don't like where a workflow's artifacts land, the fix
is in the *workflow file*, not the runner.

### Recommended layout for first-contact use

If you are trying striatum on a real repo and want to keep its
output corralled (so you can `rm -rf` it cleanly without
disturbing the rest of the tree), put everything under a
top-level `striatum/` directory — sibling to the runner's
`.striatum/` state directory but checked in:

```text
<your-repo>/
├── .striatum/                 # gitignored runner state (sqlite, scratch)
│   └── state.sqlite3
├── striatum/                  # checked-in workflow output (parallel name)
│   └── <run-slug>/
│       ├── RUN_SUMMARY.md
│       ├── RUN_EVIDENCE.md
│       ├── <draft>.md         # build / synthesis artifacts
│       ├── <reviewer>/        # one subdir per reviewer lane
│       │   └── <review>.md
│       └── final/
│           └── <final-review>.md
└── workflow.json              # the workflow itself can live anywhere;
                               # the example fixtures put it under
                               # examples/<slug>/ in the runner repo
```

The directory name `striatum/` is just a convention — pick
whatever you like in your workflow's `allowed_paths`. The
parallel naming (`.striatum/` for runtime state, `striatum/` for
durable output) is a clean visual reminder that:

- `.striatum/` is **not** committed (gitignored by `init`); it's
  the runner's working state.
- `striatum/` **is** committed; it's the durable provenance the
  runner produces.

### Adapting an example workflow

To use this layout with the bundled example, edit
`examples/rfc-ledger-cleanup/workflow.json` and change every
`docs/reviews/rfc-ledger/...` path under `expected_artifacts`
and `write_scope.allowed_paths` to `striatum/rfc-ledger/...`.
Then re-run `striatum workflow validate` to confirm the edits
parse cleanly. Better yet, copy the example into your target
tree first so the bundled fixture stays untouched:

```bash
mkdir -p striatum/rfc-ledger
cp -r path/to/striatum/examples/rfc-ledger-cleanup .
sed -i 's|docs/reviews/rfc-ledger|striatum/rfc-ledger|g' \
    rfc-ledger-cleanup/workflow.json
"$RUNNER" --repo "$TARGET_REPO" workflow validate \
    rfc-ledger-cleanup/workflow.json --json
```

For new workflows, see
[WRITING_WORKFLOWS.md § "Recommended output layout"](WRITING_WORKFLOWS.md#recommended-output-layout).

## See also

- **[CLI_REFERENCE.md](CLI_REFERENCE.md)** — every verb in one
  flat list with stable exit codes.
- **[WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md)** — author your
  own `workflow.json`.
- **[SPEC.md](SPEC.md)** — the implementation contract; the
  source of truth when this doc and the runner disagree.
