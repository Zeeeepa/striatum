# How to Act as Human Principal

Per [RFC 0053](rfcs/0053-human-principal-and-terminology-truing.md)
and [D103](DECISION_LOG.md), the human principal's only role is to
**resolve unresolvable blockers or decisions**. Routine workflow
execution — claim, ack, publish, verdict, complete — is the AI
operator's job, covered in [HOW_TO_AGENT.md](HOW_TO_AGENT.md).
Use this doc when an escalation has surfaced and you need to look
at it.

The full operator-by-hand walkthrough is retained at the bottom of
this page as **reference**: skip to the [Manual operator
reference](#manual-operator-reference) section only if you are
specifically driving the runner by hand (debugging, demo, or the
rare case where no AI operator is in the loop). For normal use you
will not read past the escalation playbook.

## Escalation playbook

### What you'll see

The AI operator escalates to you in one of two shapes:

1. **A declared blocker.** A blocker `kind` from the closed set
   the runner cannot auto-resolve: `ambiguous_goal`,
   `missing_authority`, `contradicting_decisions`,
   `no_available_reviewer_lane`,
   `committee_stalemate` (RFC 0052), `override_required`.
2. **An AI-self-declared escalation.** An `escalation` artifact
   the AI operator published when it judged itself stuck and no
   declared blocker class fit. (Artifact-kind schema lands in the
   RFC 0053 Phase A follow-up; today this surface is approximated
   by a `blocker` with operator-chosen text.)

Either way the escalation appears in your inbox alongside
ordinary state. Check it whenever you sit down at the runner:

```bash
striatum --repo "$TARGET_REPO" inbox --json
striatum --repo "$TARGET_REPO" status --json | jq '.blockers'
```

`inbox --session-id <session_id>` is still available as the
operator-on-behalf packet helper; the principal inbox does not require a
session id.

### Inspect

For a blocker, look at what's reported:

```bash
striatum --repo "$TARGET_REPO" why <session_id> --json
striatum --repo "$TARGET_REPO" run summary --run-id <run_id> --json
```

`why` includes the active blockers with their `kind` and `reason`.
Read the most recent artifact the AI was working on (the workflow
will tell you where it lives on disk) and any decision artifacts
the AI cited.

### Decide

Form the resolution outside the runner — you are the authority.
Common shapes:

- **Ambiguous goal** → narrow the goal, then record it as a
  decision.
- **Missing authority** → either delegate (record a decision
  granting the authority) or substitute the action with one the AI
  is already authorized to take.
- **Contradicting decisions** → record a new decision that
  supersedes one of the prior ones (RFC 0047 propagates the
  supersession through the verdicts table).
- **No available reviewer lane** → either change the workflow to
  use available lanes or accept a single-lane review with a
  recorded rationale.
- **Committee stalemate (RFC 0052)** → record a decision that
  selects one of the contending designs or rejects all and
  re-scopes.
- **Override required** → record an `accepted` or `rejected`
  decision against the run; RFC 0047 propagates it.

### Resolve

Record the decision through the runner so the audit chain
captures it:

```bash
striatum --repo "$TARGET_REPO" decision record \
  --run-id <run_id> \
  --outcome accepted | rejected | accepted_with_follow_up \
  --title "<short>" \
  --rationale "<why>" \
  --json
```

If the decision resolves a blocker, also clear the blocker so the
AI operator can proceed:

```bash
striatum --repo "$TARGET_REPO" recovery resume \
  --blocker-id <blocker_id> \
  --json
```

(A dedicated `striatum escalation resolve` verb is on the RFC 0053
Phase A backlog; for now `recovery resume` is the path.)

The AI operator picks up the next packet automatically.

### When to override a verdict

Sometimes the escalation is a non-accepting verdict you disagree
with. Use `override-verdict` per RFC 0047:

```bash
striatum --repo "$TARGET_REPO" override-verdict \
  --job-id <job_id> \
  --verdict accept \
  --rationale "<why this overrides>" \
  --auto-fresh-session \
  --json
```

The override flows through the same audit-chain machinery; the
prior verdict is marked superseded.

### When not to publish on behalf

You almost never need to. The AI operator has tools for stalls
(RFC 0051 auto-finalize from frontmatter, RFC 0046 lane-evidence
guard with operator override) that cover most of what
publish-on-behalf used to do. If you find yourself needing
`publish-artifact --allow-no-process-execution`, that is itself a
signal — record what went wrong as a decision so the next AI
operator session can avoid the same trap.

### Cross-reference

- [HOW_TO_AGENT.md](HOW_TO_AGENT.md) — the AI operator's playbook.
- [SPEC.md § Branch Confirmation](SPEC.md) — confirmation is the
  operator's job, not the principal's.
- [RFC 0052](rfcs/0052-committee-deliberation-workflow.md) —
  committee stalemate is one of the named escalation triggers.

---

## Manual operator reference

> The rest of this document is **reference** for the rare case
> where a human drives the runner by hand. Per RFC 0053 / D103
> this is no longer the default; the AI operator does the work and
> you (the principal) only show up for escalations covered above.
> Read past this point only if you really are the keyboard for
> some specific reason.

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

This creates `.striatum/` as operational scratch under the target
repo (supervised wrapper FIFOs, pidfiles, the daemon
capability-token cache) and adds `.striatum/` to that repo's
`.gitignore`. Authoritative workflow state lives in the daemon-
owned PostgreSQL instance under a `repository_id` scope per D094 /
RFC 0043; `init` registers the repository with the daemon when one
is reachable. The daemon is a hard prerequisite — without a
reachable daemon, the verbs above refuse with exit code 11
(`daemon_unreachable`); against a pre-D094 SQLite-only repo they
refuse with exit code 12 (`repo_not_migrated`) and point you at
`striatum daemon migrate-repo-local`. See
[POSTGRES_TRANSITION.md](POSTGRES_TRANSITION.md) for the full
bootstrap runbook.

To also drop a self-contained agent skill bundle that teaches a
Striatum-aware agent how to drive the runner (RFC 0015 V1):

```bash
# Guided day-zero path: init scratch, install skills/plugins,
# scaffold DDD docs, and migrate/register into daemon Postgres.
"$RUNNER" --repo "$TARGET_REPO" adopt --profile claude_code --json

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

### Backport harness-profile fragments

RFC 0040 V1 bakes the "no-questions" and gemini front-matter
completeness fragments into the bundled template catalog (see
[`docs/HARNESS_FRICTION_PATTERNS.md`](HARNESS_FRICTION_PATTERNS.md)).
New workflows scaffolded via `workflow generate` pick them up
automatically. Existing workflows can be upgraded in place:

```bash
# Preview the change set without writing.
"$RUNNER" --repo "$TARGET_REPO" workflow upgrade path/to/workflow.json --dry-run --json

# Apply the catalog fragments where the existing instruction is empty
# or already matches the catalog default; refuse on conflict.
"$RUNNER" --repo "$TARGET_REPO" workflow upgrade path/to/workflow.json --json

# Override a conflicting custom instruction. The change set records
# `forced: true` on each overwritten field.
"$RUNNER" --repo "$TARGET_REPO" workflow upgrade path/to/workflow.json --force --json
```

`workflow upgrade` refuses to mutate a workflow that has any non-
terminal run referencing it; cancel or complete the run first, or
duplicate the workflow if the active run should be left alone. The
verb is scoped to harness-profile fragments in V1; other corrections
will land as separate verbs.

## Drive a dogfood through the MCP chat surface (RFC 0040 V1)

When the operator's AI session is connected to `striatum serve --web
--allow-mutations`, the chat surface exposes the dogfood-lifecycle
verbs as structured tool calls (see
[`docs/MCP.md`](MCP.md#dogfood-lifecycle-tools)) so the operator no
longer has to copy session/lease/message ids between bash
invocations:

```text
run_prepare(workflow_path="…/workflow.json")
run_start(run_id=…)
register_session(run_id=…, role="implementer", lane="claude_code")
supervise_start(session_id=…)
claim_next(session_id=…)
ack(session_id=…, message_id=…, lease_id=…)
publish_artifact(session_id=…, job_id=…, lease_id=…, kind=…, logical_name=…, path=…)
complete(session_id=…, job_id=…, lease_id=…, summary="…")
supervise_stop(session_id=…, reason="…")
run_summary(run_id=…, path="…")
evidence_export(run_id=…, path="…")
```

The bash CLI surface stays canonical (RFC 0040 §"Non-Goals"); the
chat tools are an additive surface for the operator session, not a
replacement. Anything the chat tools do is also available as a
bash CLI command.

For deeper authoring guidance, see
[WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md).

To choose from the bundled generator catalog instead of a fixed starter:

```bash
"$RUNNER" workflow templates list --kind shape --json
"$RUNNER" workflow generate workflows/my-change \
  --shape code_change \
  --lane-set local \
  --artifact-root striatum/my-change \
  --dry-run --json
```

The preview writes nothing. Remove `--dry-run` to create the workflow
tree, then validate or edit `workflows/my-change/workflow.json` before
`run prepare`.

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

### Doctor triage and recovery

`striatum doctor --verbose --json` returns both the legacy
`problems` strings and structured `problem_records`. The web UI
groups those records by problem kind and links each group back to
this section, so start by reading the group name, then inspect the
record `context` for the run id, job id, lease id, blocker id, or
session id involved.

Common recovery paths are:

- `stale_queue_message_claim`, `unreaped_expired_lease`, and
  other stale-lease symptoms: run `striatum recovery stale-leases`
  first, then use `striatum recovery requeue-stale` only for
  review-only work the runner says is safe to requeue.
- `active_session_on_terminal_run`: close the session with
  `striatum session close` as described in
  [Close active sessions](#close-active-sessions).
- `open_blocker_on_terminal_run`: inspect the blocker with `why`
  and decide whether it should be resolved, canceled, or left as
  audit evidence.
- `process_*` supervisor issues: run
  `striatum recovery process-reconcile --run-id <run_id> --json`
  before requeueing anything.

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

### Close active sessions

When a run is terminal but a session is still active, close the
session explicitly:

```bash
"$RUNNER" --repo "$TARGET_REPO" session close \
  --session-id <session_id> \
  --reason terminal-run-cleanup \
  --json
```

Closing a session records the lifecycle transition; it does not
delete artifacts, verdicts, or events. If `doctor` reports several
active sessions on terminal runs, close each listed `session_id`.

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
root, refuses symlink/path-traversal ambiguity, derives a
realpath/inode-based repository identity, and refuses active
path re-occupation by a different identity. Pass `--init`
when no `.striatum/` directory exists; `--no-migrate` refuses
registration when daemon-side schema migrations would be needed.

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

`--daemon` (or `STRIATUM_DAEMON=1`) routes the read verb
through the daemon RPC envelope under a `read` token. Read
surfaces supported: `status`, `doctor`, `why`, `dashboard --all`.
The CLI refuses (does not silently fall back to direct mode) on
forced-daemon verbs that are not registered. The V1 `--no-daemon`
flag is retired (D094 / RFC 0043); parsing it returns the standard
argparse "unrecognized arguments" error.

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

What daemon mode does **not** ship today:

- It does not ship Windows daemon support, sealed-apply
  authority owning hosted semantics, mutation MCP tools beyond
  the current closed set, or remote/network-accessible serving.
- It does not bundle PostgreSQL; operators install and own the
  Postgres service. Bundled, embedded, and Dockerized
  distributions are deferred (RFC 0033 §8, inherited by RFC 0043).
- RFC 0048 (proposed, V2.0 phase) covers the remaining daemon-side
  handler-port work where some single-repo business logic still
  delegates through the SQLite-backed CLI path under the
  `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` test-harness
  escape.

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

For chat-assisted workflow generation, start the web service with
`--allow-mutations` only when you want the browser to be able to write
generated workflow files. The model may call `generate_workflow_preview`
freely. `generate_workflow_write` is hidden without the mutation flag and,
when enabled, still pauses for your browser confirmation before Striatum
writes files.
- It is not a replacement for `recovery watch` against a
  single repo, only for multi-repo sweeping.

### Daemon storage substrate (RFC 0033 + D094 / RFC 0043)

RFC 0033 put daemon-global state on operator-installed system
PostgreSQL. D094 / RFC 0043 then moves per-repository workflow
state — runs, jobs, sessions, queue messages, leases, artifacts,
verdicts, blockers, worktrees, process supervisors, and repo-local
events — into the same daemon-owned Postgres under a
`repository_id` scope. The daemon is a hard prerequisite for every
Striatum verb; the V1 `--no-daemon` direct-CLI path is retired and
parsing the flag returns the standard argparse "unrecognized
arguments" error. See
[POSTGRES_TRANSITION.md](POSTGRES_TRANSITION.md) for the full
runbook.

The operator provides PostgreSQL. Striatum connects through
`STRIATUM_DAEMON_DB_URL`, `~/.config/striatum/daemon.toml`, or an
explicit `--postgres-url` client surface; the daemon owns schema
migrations and roles, but it does not install, start, stop, or
upgrade PostgreSQL. Bundled, embedded, and Dockerized Postgres
distributions are deferred.

RFC 0048 (proposed, V2.0 phase) covers the remaining daemon-side
handler-port work where some single-repo business logic still
delegates through the SQLite-backed CLI dispatch path under the
`STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` test-harness
escape; production operators leave the variable unset.

Cut over a V1 daemon registry (RFC 0033 §4):

```bash
# Inspect what would be imported.
"$RUNNER" daemon migrate --from sqlite --to pg --dry-run --json

# Import the V1 registry into the V2 daemon DB.
"$RUNNER" daemon migrate --from sqlite --to pg --json
```

Cut over a single target repo's workflow state (RFC 0043):

```bash
# Inspect — writes nothing.
"$RUNNER" daemon migrate-repo-local \
  --from sqlite --to pg --repo "$TARGET_REPO" --dry-run --json

# Apply. Safe default keeps the source as state.sqlite3.tombstone (0444).
"$RUNNER" daemon migrate-repo-local \
  --from sqlite --to pg --repo "$TARGET_REPO" --json

# Irreversible cleanup — both flags required.
"$RUNNER" daemon migrate-repo-local \
  --from sqlite --to pg --repo "$TARGET_REPO" \
  --no-keep-sqlite-readonly --confirm-delete --json
```

The registry migration writes the V2 schema, imports registered
repositories, clients, capabilities, scheduler cursors, and
metadata-only audit rows, verifies hash continuity, and writes a
cutover marker. `--keep-sqlite-readonly` retains the old registry
file as an audit tombstone while blocking V1 writes.

The per-repo migration runs inside a single serializable Postgres
transaction with byte-equivalent audit-chain re-anchor, then
finalizes the source `.striatum/state.sqlite3` per the tombstone
flags. CLI verbs against an unmigrated repo refuse with exit code
12 (`repo_not_migrated`); CLI verbs without a reachable daemon
refuse with exit code 11 (`daemon_unreachable`). See
[POSTGRES_TRANSITION.md](POSTGRES_TRANSITION.md) for the full
runbook and rollback notes.

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

### Go runtime/helper developer fixture (RFC 0039 history)

> Status: historical/developer fixture. The Python daemon (`striatum
> daemon start`) is the production core. Go is narrowed to
> supervisor/helper/runtime and harness roles; it is not a peer
> production daemon implementation and is not planned to become the
> default.

RFC 0039 produced a Go `go/cmd/striatumd` prototype that speaks the
RFC 0030 envelope-v1 wire protocol over the RFC 0033 PostgreSQL
substrate. Phase 1 (Steps 1+2) landed the read-only RPC skeleton and the
PostgreSQL connection/migration/audit layer. The Phase 3 architecture
decision keeps Python as the primary daemon core, so production mutating
verbs, daemon-owned supervision, migration ownership, and release
packaging remain Python-daemon responsibilities unless a future accepted
decision changes that boundary.

Build the binary from a contributor checkout:

```bash
make -C go build
ls go/bin/striatumd
```

The build requires Go 1.23+ and the system `make`. The binary is not
shipped with `pip install striatum-orchestrator` and is not packaged by
release tooling.

Run it directly for developer inspection:

```bash
./go/bin/striatumd \
  --socket "${XDG_RUNTIME_DIR:-/tmp}/striatum/daemon.sock" \
  --db-url "$STRIATUM_DAEMON_DB_URL" \
  --migrations-dir src/striatum/daemon_pg/sql
```

The historical read-only method registry exposes `daemon.hello`,
`daemon.welcome`, `daemon.describe`, `daemon.status`, `daemon.version`,
`audit.show`, and `repo.list`. Other verbs return `method_not_found`
because the Go binary is not the production mutation surface.

Coexistence rule: only one daemon may own the PostgreSQL substrate at a
time. **Stop the Python daemon before starting the Go daemon** (and vice
versa). The Go binary refuses to start with exit code 14
`daemon_already_running` when it detects another `striatumd-*`
connection in `pg_stat_activity`.

The RFC 0035 multi-repo test harness can still target the Go fixture for
compatibility checks through the `daemon_core` parameter; Python remains
the production default:

```python
from _harness.multi_repo import MultiRepoHarness

# Production/default path — Python core.
harness = MultiRepoHarness(daemon_pg_url=...)

# Opt into the Go fixture. The harness invokes `make -C go build` if
# the binary is missing; set STRIATUMD_GO_BIN=/path/to/striatumd to
# skip the build step and reuse a prebuilt binary.
harness = MultiRepoHarness(daemon_pg_url=..., daemon_core="go")
```

## Web UI

Start the local web UI with:

```bash
"$RUNNER" --repo "$TARGET_REPO" serve --web
```

The startup envelope prints the bound URL. The UI is
server-rendered HTML with vanilla-JS enhancements and the same
localhost-first mutation gate as the service API. Important routes:
`/` for runs, `/run/<run_id>` for a run, `/run/<run_id>/job/<id>`
for a job, `/run/<run_id>/artifact/<id>` for an artifact,
`/workflows` for workflow files, `/chat` for configured chat
sessions, and `/doctor` for health checks.

The run list supports free-text search over run id, branch, and
workflow id; state filters; date ranges; and a duration column. The
workflow list supports path/workflow-id search, valid/invalid
filters, and a last-modified column. Filter preferences are stored
in browser `localStorage`, not in the repository or SQLite.

### Repository file browser (`/view/`)

Visit `/view/` (no path) for a tree-style repository file browser.
Click a directory to expand or collapse it; click a file to open it
in the single-file viewer at `/view/<path>`. The browser uses
`GET /v1/repo/tree?path=<rel>` for lazy directory expansion, sorts
directories before files, hides `.git/` and `.striatum/`, and refuses
paths that try to escape the repository root. The breadcrumb at the
top of the page links to every ancestor; the filter input narrows
the visible rows by fuzzy subsequence match. Keyboard navigation:
ArrowUp/Down move between rows, ArrowRight/Left expand/collapse a
directory, Enter opens a file or toggles a directory, Home/End jump
to the first/last loaded row.

### Workflow chooser wizard (`/workflows/new`)

Visit `/workflows/new` to scaffold a new workflow with the
step-by-step chooser. Step 1 picks the workflow shape (radio cards
of the bundled RFC 0034 V1 catalog). Step 2 picks a lane set filtered
by the selected shape's recommendations. Step 3 selects optional
modifiers; mutually incompatible modifiers self-disable. Step 4 fills
the required fields (`workflow_id`, `name`, `scaffold_root`,
`artifact_root`, `branch_suggestion`, optional per-lane commands).
Step 5 calls `POST /workflows/generate/preview` and renders the
generated workflow JSON, file list, and any warnings; the preview
writes nothing on disk and re-runs whenever you edit a step 1–4
field. Step 6 opens a `<dialog>`-driven operator confirmation; only
after you accept does the wizard call `POST /workflows/generate` with
`confirm_write: true`. The local service must be running with
`--allow-mutations` for the confirm step to succeed.

### Drag-drop workflow graph editor

The Edit affordance on a workflow detail page (now a button next to
"Run this workflow now", not the muted text link) opens
`/workflows/edit/<path>`. The page renders a React Flow drag-drop
graph editor. The left palette adds new jobs from the closed RFC 0034
block vocabulary (`draft`, `review`, `synthesis`, `implementation`,
`test`, `human_checkpoint`, `support_ledger`, `evidence_audit`,
`final_review`). The canvas centre shows the workflow as nodes and
edges; cycles render with dashed styling. Click a node to select it;
the right inspector edits the selected job with structured widgets:
dropdowns for role/lane/type/access scope/context policy, radio sets
for posture and write-scope mode, multi-select chips for
`required_review_postures`, repeating-row editors for allowed and
forbidden paths, and a structured per-row editor for
`expected_artifacts`. Save calls the existing
`POST /workflows/edit/<path>` endpoint with the same `If-Match`
sha256 semantics as before; server-side `validate_workflow()` remains
authoritative.

### Syntax-highlighted code viewer (`/view/<path>`)

Visiting `/view/<path>` for non-Markdown text files renders a
syntax-highlighted view via Shiki. Toolbar buttons: Copy (writes the
file contents to the clipboard, announces "Copied" via a polite live
region), Wrap (toggles soft wrap; default is no-wrap with internal
horizontal scroll), and Raw (opens the unhighlighted bytes in a new
tab with `rel="noopener"`). Files over 500 lines collapse by default
with an Expand banner. Files over 5 MB skip Shiki entirely and render
escaped plain text. Markdown files (`.md`) continue to render
server-side as before; other file types fall through to a
`<pre>`-fallback when Shiki fails to load.

The doctor page groups structured problem records by kind. Use the
`Hide problems on terminal runs` toggle to suppress completed,
failed, or canceled run noise when you are focused on active work.
The toggle is also stored in browser `localStorage`.

The header timestamp toggle switches visible `<time>` elements
between raw UTC and the browser's local timezone. Keyboard
shortcuts are available when focus is not inside an editable
control:

| Shortcut | Destination |
|---|---|
| `g r` | Runs |
| `g w` | Workflows |
| `g c` | Chat |
| `g d` | Doctor |
| `?` | Shortcut help |
| `Esc` | Close shortcut help |

On a run detail page, non-terminal runs show deterministic next
actions immediately below the run header. Graph nodes are clickable
and expose job id, role, state, and duration on hover or keyboard
focus. The graph viewport supports drag-to-pan, arrow-key panning,
zoom in/out (`+` / `-`), fit (`f`), and reset (`0`) through the
visible graph controls.

The run list's Workflow column uses the snapshotted workflow identity:
the visible label is the workflow `name` when available, the metadata
shows `workflow_id` and version, the local link opens the workflow
detail page, and repositories with a GitHub `origin` also get a
default-branch source-directory link.

## Dashboards and graphs

For a compact at-a-glance view of a run, use the dashboard. It is
a dependency-free terminal renderer over the same daemon-owned
PostgreSQL state that `status` exposes:

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
├── .striatum/                 # gitignored operational scratch (FIFOs, pidfiles, token cache)
│   ├── scratch/
│   └── bin/
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
parallel naming (`.striatum/` for operational scratch,
`striatum/` for durable output) is a clean visual reminder that:

- `.striatum/` is **not** committed (gitignored by `init`); it's
  operational scratch. Authoritative workflow state lives in the
  daemon-owned PostgreSQL instance, not in this directory.
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

## Optional: export a corpus bundle for an external memory consumer

`striatum corpus export --since <ref> --out <dir>` (RFC 0044 V1) emits a
redacted JSONL bundle of Striatum's durable provenance — RFCs,
decision-log rows, operator reports, run summaries, audit-chain entries,
changelog entries, ubiquitous-language terms, harness-friction patterns,
and recent commits — plus a verifying `manifest.json`. This is an
*optional, post-run maintenance step*. It does not modify live state,
does not write under `.striatum/`, and is never required for any
workflow command to succeed.

```bash
"$RUNNER" --repo "$TARGET_REPO" corpus export \
    --since "$(git -C "$TARGET_REPO" merge-base origin/main HEAD)" \
    --out  "$TARGET_REPO/striatum-corpus-bundle"
```

The bundle is durable, replay-stable provenance: re-running over
unchanged inputs produces byte-identical JSONL files and identical
per-file SHA-256s (only `generated_at` varies). An optional retrieval
consumer (Engram is the first reference under RFC 0044) may ingest the
bundle locally and serve search over it; Striatum does not call the
consumer at runtime and continues to run unchanged when no consumer is
configured. The V2 contract decisions (multi-corpus identity,
redaction-tier metadata, incremental watermarks, optional workflow-level
context-injection policy) are scoped by
[RFC 0057](rfcs/0057-corpus-contract-v2.md).

## See also

- **[CLI_REFERENCE.md](CLI_REFERENCE.md)** — every verb in one
  flat list with stable exit codes.
- **[WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md)** — author your
  own `workflow.json`.
- **[SPEC.md](SPEC.md)** — the implementation contract; the
  source of truth when this doc and the runner disagree.
