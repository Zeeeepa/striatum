# striatum Specification

Status: implementation contract
Date: 2026-05-06

This specification binds the V1 MVP described in
`docs/design/V1_MVP_DESIGN.md` and synthesized in
`docs/reviews/v1/V1_MVP_SYNTHESIS.md`.

## Product Boundary

`striatum` is a standalone, local-first workflow runner for
terminal-based AI coding agents. It coordinates registered target
repositories through a local daemon, daemon RPC methods, and
capability-gated client surfaces (CLI, MCP, and local web UI). It
does not provide hosted services, external persistence, telemetry,
Slack/remote serving, transcript capture, provider SDK integration,
malicious-local-operator-resistant sealed apply, or automatic commits.

RFC 0033, RFC 0043, and RFC 0048 establish the current substrate:
daemon-owned PostgreSQL is authoritative for daemon-global state and
per-repository workflow state; `.striatum/` next to a target repository
is operational scratch only. RFC 0030 supplies the daemon RPC envelope,
RFC 0031 supplies daemon-owned supervision/apply foundations, and RFC
0032 supplies cross-repository workflow and MCP mutation capability
foundations. Hosted service semantics and bundled PostgreSQL remain
separate future product decisions.

The authoritative live state is the daemon-owned PostgreSQL instance
(RFC 0033) under a `repository_id` scope per registered target
repository. Per [D094 / RFC 0043](rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md),
this supersedes the V1 carve-out that kept repo-local workflow state
in `.striatum/state.sqlite3`. The daemon is a hard prerequisite for
every Striatum verb; the V1 `--no-daemon` flag is retired and parsing
it returns the standard argparse "unrecognized arguments" error.
Repository artifacts are durable provenance only. Marker files, tmux
panes, terminal output, and provider hooks are never live
control-plane state. See [`docs/POSTGRES_TRANSITION.md`](POSTGRES_TRANSITION.md)
for the operator runbook.

RFC 0048 (v1.49.0 → v1.55.0) completed the substrate port: every
single-repo mutation, recovery, and read handler runs natively
against the daemon's per-repo Postgres tables in the Python daemon
core. The Go tree retains helper/runtime and developer-harness
counterparts for selected read and mutation paths, but it is not a
second production daemon core and is not planned to displace the
Python daemon. The
`STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` escape no
longer takes effect for ported methods — mapped CLI verbs fail
closed instead of falling back to SQLite when the daemon is
unreachable or the target repository is not registered. Schema v6
(migration 0006) anchors the per-event hash chain in dedicated
`previous_hash` / `row_hash` columns plus a
`striatumd.repo_event_chain_heads` pointer for O(1) chain-head
reads.

External memory or retrieval systems (Engram, under RFC 0044, is the first
reference consumer) may ingest the read-only `striatum corpus export` bundle
as optional local augmentation. The runner does not import any such consumer,
does not register `memory.*` capabilities, and does not call retrieval
during state transitions; see § Corpus Export And Augmentation Boundary.

## State Store

`striatum init` creates `.striatum/` next to the target repository as
operational scratch (supervised wrapper FIFOs, pidfiles, the
capability-token cache) and ensures `.striatum/` is ignored by git.
The authoritative workflow state lives in the daemon-owned PostgreSQL
instance under a `repository_id` scope; `striatum init` registers the
repository with the daemon when one is reachable.

The per-repository schema in the daemon DB holds:

- `repositories` (registry; per-repo identity and lifecycle)
- `workflow_snapshots`
- `runs`
- `sessions`
- `jobs`
- `job_dependencies`
- `queue_messages`
- `leases`
- `work_packets`
- `artifacts`
- `verdicts`
- `blockers`
- `command_requests`
- `process_executions`
- `events`
- `job_worktrees`
- `process_supervisors`
- `process_supervisor_pointers`

`events` and artifact records are append-only (UPDATE/DELETE are
revoked from the daemon read-write role). Mutations use short
serializable Postgres transactions and emit structured events.

Schema upgrades are forward-only, daemon-owned, and applied at
daemon startup; `daemon doctor` reports the on-disk substrate
version. A database whose schema version is higher than the daemon
binary supports is refused; client/daemon version skew refuses with
exit code 10. The pre-D094 repo-local SQLite migration list is
retained only for the `migrate-repo-local` golden fixture and is not
applied by ordinary CLI verbs.

Day-zero setup is guided by `striatum adopt`, `daemon service
install/start/status`, `daemon doctor --provision-rw-role
--repair-grants`, and `doctor --first-run`. These helpers are local
bootstrap surfaces: they may initialize scratch files, render a user
service, repair local Postgres grants, or run smoke checks, but they do
not become an alternate workflow-state authority.

The `migrate-repo-local` command converts an existing pre-D094
`.striatum/state.sqlite3` into per-repo Postgres rows and finalizes
the source file as a read-only `.striatum/state.sqlite3.tombstone`
(safe default) or deletes it with `--no-keep-sqlite-readonly
--confirm-delete`. CLI verbs against an unmigrated repo refuse with
exit code 12 (`repo_not_migrated`); CLI verbs without a reachable
daemon refuse with exit code 11 (`daemon_unreachable`). Neither
refusal opens or creates a SQLite file.

## Workflow Config

Workflow config is JSON only. The validator rejects `.yaml` and `.yml` files
and rejects non-object JSON roots.

Required workflow fields:

- `schema_version`
- `workflow_id`
- `workflow_version`
- `name`
- `branch`
- `coordinator`
- `lanes`
- `roles`
- `context_docs`
- `parallelism`
- `jobs`
- `edges`
- `cycles`

The V1 schema version is `striatum.workflow.v1`.

The runner does not infer or select a default workflow for a target
repository. `run prepare` requires an explicit workflow file path, and
the workflow snapshot for a run is taken from that file. `workflow init`
is only a scaffold generator; it currently supports `minimal`, `review`,
and `code-change` starter styles and uses `review` when `--style` is
omitted. Checked-in `examples/` are fixtures or starting points, not
runtime defaults.

RFC 0034 V1 adds a workflow generator over the same schema. The public
Python API is `striatum.workflow_generator.generate_workflow(spec)`,
where `spec.schema_version` is `striatum.workflow_generator.v1`. The
generator is pure: it writes no files, performs no network access, and
does not touch SQLite. It compiles a workflow shape, lane set, optional
lane modifiers, and optional closed-vocabulary custom plan into an
ordinary `striatum.workflow.v1` JSON object, then calls the existing
workflow validator before returning success. `workflow init --style`
is compatibility sugar over this generator with `lane_set: "local"`.
Generator preview envelopes also include the same advisory workflow lint
payload exposed by `workflow lint`, including warning count and coverage
summary; lint remains informational and does not change validation
status.

The bundled template catalog is package data under
`striatum.workflow_templates/catalog.json`; V1 does not fetch remote
templates and does not load target-repository catalog extensions.
`workflow templates list/show` expose catalog metadata.
`workflow generate` writes the generated tree only after the same
immediate validation pass and refuses to overwrite existing generated
files. The local service exposes read endpoints
`GET /workflow-templates` and `GET /workflow-templates/<id>`, plus
generation endpoints `POST /workflows/generate/preview` and
`POST /workflows/generate`. Preview writes nothing and is not mutation
gated; generation requires `--allow-mutations` and
`confirm_write: true`. No database migration is part of RFC 0034 V1.

RFC 0036 V1 adds the chat-assisted scaffolding harness over the same
generator path. The chat closed set includes `generate_workflow_preview`
at all times and includes `generate_workflow_write` only when the local
service was started with `--allow-mutations`. The write tool still fails
closed unless `confirm_write: true` is present and a separate operator
confirmation gesture is recorded by the web UI.

The validator enforces unique job ids, resolved role/lane references, valid
edges, bounded cycles, repo-relative artifact paths, and declared parallelism
with disjoint write scopes or review-only unique artifact paths.

Workflows may opt into RFC 0032 cross-repo shape with a top-level
`repositories` object and required `primary_repository` alias. Each
repository entry names a daemon-registered `repo_id`. Cross-repo jobs must
declare a `repository` alias explicitly; single-repo workflows must not
declare job-level `repository`. Artifact path uniqueness and parallel
write-scope overlap are checked per repository alias, not globally across
all participants. `reviewer_access_scope:
"cross_repo_artifact_augmented"` is valid only for review jobs in
cross-repo workflows. Cross-repo cycles must opt in with
`cross_repo_cycle: true`, and `parallelism.per_repo_max_active_jobs` may
declare per-alias positive integer limits. Core workflow validation checks
shape only; daemon-backed `run prepare` owns live repository registration and
accessibility checks.
`workflow validate` refuses lint-detected same-model implementer/reviewer
pairings and revision cycles by default; operators can pass
`--allow-same-model-pairing` to accept that workflow-authoring risk
explicitly. This CLI-level refusal uses the advisory lint rules and does not
change the pure `validate_workflow()` API or generator preview behavior.

Lane selection is workflow-authored. There is no provider-default lane and
lane ids have no built-in semantic meaning. A job with `lane_id` is queued for
that lane; a job without `lane_id` is queued without a lane target and can be
claimed by a session with the matching role. `register-session` records the
session's `lane_id`, and `claim-next` matches pending work by run, role,
fresh-session rules, and lane target when one is present.

Lane configs may declare adapter constraints for network access, transcript
handling, and repository scope. The validator accepts only known constraint
names and values, and work packets expose both the requested constraint and the
adapter's recorded enforcement level. Lanes may also declare
`required_enforcement` for any declared constraint. Validation rejects a lane
when the adapter can only provide a weaker level than the workflow requires.

Workflows may declare `review_revision_policy` for root review
`needs_revision` verdicts. V1 supports the explicit
`root_review_needs_revision: "human_checkpoint"` policy for RFC-style workflows
that intentionally pause for operator judgment instead of entering a revision
loop. Per RFC 0053 the operator is the AI agent by default; the pause routes
to the human principal only when the AI escalates. (The schema field name
`human_checkpoint` is retained for compatibility; renaming it is deferred.) `root_review_needs_revision: "declared_cycle"` is accepted only when each
root review job declares a matching `needs_revision` cycle.

### Harness Profiles

> Design rationale: [RFC 0010](rfcs/0010-tool-harness-profiles.md).

Workflows may declare an optional `harness_profiles` map at the top level
and reference one profile per lane via `harness_profile_id`. The map is a
passthrough projection surfaced to work packets; it does not change
adapter or scheduler behaviour.

V1 validation rules:

- `tool_family` must be one of `generic`, `codex`, `claude_code`,
  `gemini_cli`. Other values are rejected.
- `strategy_version` must be a non-empty string.
- `accountability.native_subagents`, when set, must equal
  `internal_to_parent_session`.
- `accountability.first_class_registration`, when set, must equal
  `not_supported`.
- `prompt_envelope_path`, when set, must be a non-empty repo-relative
  string with no `..` segments. Existence is not checked at validate
  time.
- `fallback_profile_id`, when set, must reference a profile declared in
  the same workflow.
- A lane's `harness_profile_id`, when set, must reference a profile
  declared in `harness_profiles`. Unknown references are rejected.
- Unknown sibling fields on a profile body are accepted as lint
  warnings, surfaced in `striatum workflow validate --json` and
  `workflow plan --json` under the `warnings` key. They are not
  errors in V1; future versions may tighten this.
- (V1.5) Repo-relative process-lane command paths that do not exist on
  disk surface as lint warnings under the same `warnings` key. The
  check fires when `lane.command[0]` looks like a path (contains a
  slash or starts with `./`/`../`) and is missing under the workflow's
  repo root. Bare binary names (`codex`, `claude`, `gemini`) and
  absolute paths are not checked. The warning is non-blocking; future
  versions may graduate it to a hard error.

When a job's lane references a declared profile, `claim-next` adds a
`harness_profile` block to the work packet:

```json
{
  "harness_profile": {
    "profile_id": "codex_default",
    "tool_family": "codex",
    "strategy_version": "2026-05-08",
    "...": "every other declared profile field, verbatim"
  }
}
```

Lanes without `harness_profile_id` produce work packets with no
`harness_profile` key — the contract for existing workflows is unchanged.

Profiles are referenced at lane level only; job-level overrides are
reserved for a future RFC. The reference fixture lives at
`examples/harness-profiles/workflow.json`.

### Reviewer Policy

`type: "review"` jobs may declare two optional policy fields (RFC 0002):

- `reviewer_access_scope` is one of `document_only`, `artifact_augmented`, or
  `repo_level`. It tells the reviewer what they may inspect: only the target
  documents listed in `inputs`; those plus supporting artifacts/reports/ledgers
  also listed in `inputs`; or the repository within the job's declared
  `write_scope.allowed_paths`/`forbidden_paths`.
- `reviewer_context_policy` is one of `fresh` or `cross_round`. `fresh` requires
  a brand-new role/session with no prior thread state; `cross_round` lets the
  reviewer retain context to verify whether previously raised issues were
  resolved.

Validation rejects unknown values, non-review jobs that declare either field,
and the explicit conflict between `reviewer_context_policy: "fresh"` and
`fresh_session_required: false`. When a review job declares
`reviewer_context_policy: "fresh"` and does not set `fresh_session_required`,
the prepared job row is silently stored with `fresh_session_required = 1`.

When a review job declares either field, work packets gain a `review_policy`
block that exposes `access_scope`, `context_policy`, and a deterministic
`instruction` string. The instruction is the access-scope sentence followed by
a single space and the context-policy sentence, so reviewers can be prompted
without parsing the policy values themselves. Workflows that do not declare
the fields produce work packets without the block, preserving prior behavior.

#### Review Postures (RFC 0018 V1)

Review jobs may also declare an optional `review_posture` field that names
the kind of adversarial reading the reviewer is performing. V1 ships nine
first-class postures plus a `custom:<name>` grammar for off-list flavors:

```
neutral | devils_advocate | security | threat_model |
latency_performance | ergonomics_dx | accessibility |
compliance_license | supply_chain | custom:<non-empty>
```

When declared, the work packet's `review_policy` block includes a `posture`
key and the `instruction` string gains a deterministic posture-specific
sentence (e.g. `security` appends "This is a security-focused review. […]
verdict acceptance means you actively looked and found nothing actionable.").
Custom postures expose the literal string but get no auto-appended
sentence; the workflow author owns the prompt body for off-list flavors.

Build jobs may declare `required_review_postures`: a non-empty list of
posture names declaring which adversarial coverage the build wants. The
workflow validator walks the directed edge graph in both directions from
each such build and refuses (`WorkflowError`, exit code 8) when any
required posture is not the `review_posture` of a reachable review job.
This catches mis-wired posture coverage at workflow-validate / run-prepare
time, before any session claims work. Runtime enforcement is preserved
by the existing edge-verdict gate (a downstream-of-review job stays
blocked until the review accepts) plus run-completion semantics; no
separate runtime gate is added in V1.

> Design rationale: [RFC 0018](rfcs/0018-focused-adversarial-review-postures.md);
> see also [`docs/dogfood/016/decisions/V1_ACCEPTANCE.md`](dogfood/016/decisions/V1_ACCEPTANCE.md)
> for the lifecycle re-cast (D069).

#### Reviewer Independence (advisory)

`fresh_session_required: true` and `reviewer_context_policy: fresh` are
**advisory** beyond what the runner can mechanically observe. The runner
enforces session-id distinctness (a reviewer session is a different
`session_id` from the author session) and refuses to register a fresh
reviewer when an active author session already exists in the run, unless
`register-session --force-non-fresh --reason "..."` is passed. The reason
is recorded on the session row (`sessions.non_fresh_reason`) so evidence
exports document the override explicitly.

What the runner **cannot** verify: whether the OS process driving the
reviewer session has actually been kept free of the author's context.
A single human at a single keyboard can satisfy session-id distinctness
trivially while still having read the entire draft handoff. `striatum
doctor` surfaces two observable breaches as
`reviewer_independence_unverified` problem records:

1. Two active sessions in the same run whose supervisor rows share a
   `pid`. Same OS process is driving both lanes.
2. An active reviewer session on a run whose author session has an
   active supervisor but the reviewer does not. The asymmetric
   supervised/unsupervised mix usually means the operator is driving
   the reviewer manually from the same shell as the author.

Operator obligation: when running with `--force-non-fresh`, the recorded
reason should describe how independence was preserved (e.g., "different
agent CLI invoked from a fresh shell", "review delegated to teammate")
or explicitly note the breach ("operator drove both lanes; HARNESS-001
working supervised lane not yet shipped"). The runner records the string
verbatim; reviewers and auditors read it later.

#### Byline Integrity

Workflow-declared `expected_artifacts.author_line` is computed at packet
and publish time from the session's current lane-liveness attestation
(RFC 0026). A session is lane-attested only when it has an attached
`process_supervisors` row for the same run and session, the recorded pid
is alive, the recorded Linux `/proc/<pid>/stat` start-time token still
matches, and the supervisor command equals the session lane command from
the immutable workflow snapshot. `starting` supervisors do not attest a
lane. Platforms that cannot provide a stable process-start token are
unattested rather than silently upgraded.

Attestation is not model-token authorship proof and is not source-byte
provenance. It means only that the runner has a live process binding for
the declared lane. Unattested sessions publish under `author: operator`
or, when registered with `--operator-label <label>`, under
`author: operator [self-declared: <label>]`. Operator labels must match
`^[a-z0-9._-]{1,64}$` and may not be reserved attestation terms, lane
ids, or role/model/ordinal-shaped bylines.

The runner records the **actual** `author:` line read from each published
Markdown artifact in `artifacts.author_line`; when the file omits the
line entirely the column is NULL. Evidence exports and run summaries read
the actual column, so a missing byline renders as `author: <missing>`
rather than the workflow's expected string. This prevents the snapshot
lying about who reviewed when the operator drove a job whose declared
lane never executed it (HARNESS-003).

Review jobs may declare `require_attested_lane: true`. In V1 this field
is valid only on review jobs. When set, `publish-artifact`, `verdict`,
and `submit-review` refuse before side effects unless the calling
session is lane-attested, and the error points operators at
`striatum supervise start --session-id <id>`.

### Provenance Modes

Workflows may declare `provenance_mode`. The closed set is `advisory`,
`attested_bylines`, and `sealed_patch`; absent mode defaults to
`advisory`.

`advisory` is the current default provenance level: Striatum records
workflow state, artifacts, verdicts, and evidence, but it does not
prevent an operator with native file tools from editing source bytes
directly.

`attested_bylines` means RFC 0026 lane-liveness attestation affects
byline derivation and optional review-job gates. It still does not prove
artifact bytes came from a model process and does not prevent direct
source edits.

`sealed_patch` is the reserved hard-containment mode. The workflow
validator accepts structurally valid `sealed_patch` workflows with
non-overlapping repo-relative `protected_paths` and
`operator_writable_paths`. Daemon-backed `run start` refuses sealed
runs unless the daemon has the required containment/apply authority.
Daemon-mediated sealed apply is represented by RFC 0031 apply receipts
and fail-closed apply authority. Silent downgrade to `advisory` is a
correctness bug.

## Run Lifecycle

A run starts in `running` (after `run start`). Terminal transitions
that `maybe_complete_run` produces:

- `failed` — any job in the run reaches `state = 'failed'`. The run
  ends with `stop_reason = 'job_failed'`.
- `completed` — every job is in a terminal state (`completed`,
  `skipped`, or `canceled`) and at least one job is `completed`.
  Partial success counts: a run that finished any work is recorded
  as completed.
- `canceled` — every job is in a terminal state and none is
  `completed`. `recovery cancel-job --cascade` over an entire run is
  the typical trigger; `stop_reason = 'all_jobs_canceled'`.

Auto-close on a run-terminal transition (RFC 0011) records each
session's `close_reason` from the same vocabulary: `run_completed`,
`run_failed`, `run_canceled`, or `explicit`.

## Sessions

Agents must call `register-session` before claiming work. Database identity is
an opaque `session_id`; human display uses `<role>-<lane>-<ordinal>` slugs.

Sessions match work by run, role, lane, and capabilities. Jobs can require
fresh sessions. Native sub-agents spawned inside an agent CLI inherit the
parent session unless explicitly registered as first-class sessions.

### Session lifecycle and closure

> Design rationale: [RFC 0011](rfcs/0011-session-close-and-run-terminal-auto-close.md).

Sessions are created `active` by `register-session`. The `state` column
ranges over `('active','expired','stopped','lost','closed')`:

- `active`: registered and able to claim work.
- `expired`: an explicit recovery path released the session's lease and
  marked the session expired. Reserved for the existing recovery surface.
- `stopped`/`lost`: the session's supervised process exited (RFC 0009).
- `closed`: the new terminal state introduced by RFC 0011, set either by
  the explicit `striatum session close` command or by run-terminal
  auto-close.

`striatum session close --session-id <id> --reason <text>` is idempotent
against an already-terminal session (returns the existing terminal row
plus a `note`) and refuses with exit 4 when the session still holds an
active lease (the message points the operator at `striatum release`).
On the happy path it transitions the session to `closed`, records
`closed_at` and `close_reason`, and emits a `session.closed` event with
payload `{session_id, role_id, lane_id, reason, source: "explicit"}`.

When a run transitions to a terminal state (`completed`, `failed`,
`canceled`), the runner automatically closes every still-active session
on the run inside the same transaction. Each auto-close emits a
`session.closed` event whose `source` is one of `"run_completed"`,
`"run_failed"`, or `"run_canceled"`. Auto-close skips any session that
holds an active lease — the existing `expire_leases`/recovery flow
remains the path for those.

The doctor check `active_session_on_terminal_run` is preserved as the
residual warning for genuinely anomalous states (transition skipped,
manual SQLite editing, partial recovery). After auto-close it should
no longer fire on a clean-finish run.

`evidence export` and `run summary` include a per-session block with
each session's `state`, `closed_at`, `close_reason`,
`lane_attestation`, `operator_label`, and (when set by HARNESS-003
override) `non_fresh_reason`. The `RUN_SUMMARY.md`
`## Sessions` section lists one line per session in registration order.

## Work Queue

`claim-next` lazily expires active leases, then atomically claims the oldest
eligible pending work message. It returns a structured work packet and stores
the packet JSON plus hash.

Required transition commands:

- `ack`
- `heartbeat`
- `release`
- `block`
- `complete`
- `verdict`
- `publish-artifact`
- `send`

Expired review-only leases can be requeued when attempts remain. Expired
repo-write leases become stale or blocked and require coordinator or
operator inspection before requeue. When the inspection raises an
unresolvable question, the operator escalates to the human principal
(RFC 0053).

## Artifacts

Published artifacts are curated outputs: prompts, findings, ledgers,
syntheses, decisions, handoffs, markers, and test reports.

Owner choices can be recorded with `decision record`. The command writes a
durable Markdown artifact with YAML front matter using
`schema_version: striatum.decision.v1`, `artifact_kind: decision`, a stable
`decision_id`, `run_id`, `outcome`, `follow_up_required`, title, owner, and
creation timestamp. It records the file as a run-level artifact of kind
`decision` with no job, session, or active lease requirement, and emits a
`decision.recorded` event. Outcomes are `accepted`, `rejected`, and
`accepted_with_follow_up`; the follow-up outcome requires explicit follow-up
text.

Durable Markdown artifacts should include the work packet's privacy-safe
`author:` line in their title block when one is provided. For unattested
sessions this line is `author: operator`, not a lane/model byline.

`publish-artifact` validates file existence, repo-relative path, write scope,
artifact kind, and content hash. Transcript artifacts are rejected by default.
Markdown artifacts may include YAML front matter or title-block `author:`
metadata; when they do, the line must exactly match the work packet's lowercase
author line. The publisher still records artifacts rather than rewriting them.
Model-bylined artifacts require lane evidence: if the daemon supervisor has
reported `artifact_observed` events for the session, one must match the
published repo-relative path; otherwise the legacy clean `process_executions`
fallback applies for wrappers that have not yet reported path-specific
observations. The operator-only `--allow-no-process-execution
--override-rationale` path records both a provenance event and the artifact's
`attestation_override_rationale`.

`complete` and review `verdict` commands verify all required artifacts before
terminal job transition.

`submit-review` composes the common review path: it publishes the review
artifact, records the verdict, applies review-gate behavior, and returns the
artifact, verdict, blocker, run, and downstream state.

`override-verdict` is an explicit operator recovery command for a completed or
`waiting_human` review job whose latest verdict is non-accepting. It requires a
fresh active session on the same run, appends a newer `accept` or
`accept_with_findings` verdict without editing prior verdict rows, resolves
revision-routing human checkpoints when present, and re-evaluates downstream
gates.

`evidence export` writes a redacted Markdown snapshot of run, job, blocker,
verdict, artifact, status, doctor, and downstream-blocking state. Export paths
must stay inside the repository and outside `.striatum/`; the daemon DB
remains live state outside the repository, and `.striatum/` scratch or
migration tombstones are not durable provenance. Free-text fields that may
contain agent or user prose, including blocker descriptions and verdict
rationales, are redacted in the export.
Workflow job titles are omitted by default; job and artifact authorship is
reported through stable identity metadata: role id, lane id, declared model
display name, and workflow job id.

Evidence redaction is **default-deny**. The export schema is defined by an
explicit per-field policy registry that classifies every emitted field as
`safe`, `redacted`, or `dropped`. Any field added to `evidence_snapshot()`,
`status()`, or `doctor()` that is not registered as `safe` is replaced with
the redaction placeholder. New fields cannot leak agent or user prose into a
committed export without an explicit, reviewable change to the registry.
`safe` is scalar-only: if an emitted safe field unexpectedly contains an
object or list, the exporter replaces it with the same placeholder instead
of recursively trusting nested content.

Work packets expose an exact lowercase `author:` line for agents to place in
durable Markdown artifacts. This byline is distinct from evidence-export
identity metadata: exports keep stable role id, lane id, declared model display
name, and workflow job id; artifact files use the compact
`author: <role-name>-<model-name>-<ordinal>` convention so workflow job titles
or other project-specific prose do not leak into the artifact byline. The
artifact publisher records and validates artifact references; it does not
rewrite artifact files to insert headers.

### Artifact Front Matter Schemas

Durable Markdown artifacts may include an optional YAML-style `---`-delimited
front-matter block at the top of the file. When the artifact kind has a
registered schema and a front-matter block is present, `publish-artifact`
validates the parsed metadata against the schema. Files without a front-matter
block remain accepted as before; the publisher never rewrites artifact files.

Front-matter values are written as `key: <json-value>` lines so the parser is
unambiguous without adding a YAML dependency. Strings must be JSON-quoted,
booleans use `true` and `false`, integers and lists follow JSON syntax, and
nested mappings are not supported.

V1 schemas:

- `striatum.decision.v1` (kind `decision`): required `schema_version`,
  `artifact_kind: decision`, `decision_id`, `run_id`, `owner: human`,
  `outcome` (one of `accepted`, `rejected`, `accepted_with_follow_up`),
  `follow_up_required` (boolean), `title`, `created_at`.
- `striatum.finding.v1` (kind `finding`): required `schema_version`,
  `artifact_kind: finding`, and `verdict_intent` (one of `accept`,
  `accept_with_findings`, `needs_revision`, `reject`); optional `severity`
  (one of `info`, `low`, `medium`, `high`, `critical`) and `tags` (list of
  strings).
- `striatum.findings_ledger.v1` (kind `findings_ledger`): required
  `schema_version`, `artifact_kind: findings_ledger`, and `summary_count`
  (non-negative integer); optional `entries_path`. Ledger entries themselves
  are body content, not structured front matter.
- `striatum.synthesis.v1` (kind `synthesis`): required `schema_version` and
  `artifact_kind: synthesis`; optional `inputs` (list of logical-name
  strings).
- `striatum.support_ledger.v1` (kind `support_ledger`, RFC 0003): required
  `schema_version`, `artifact_kind: support_ledger`, and `audited_artifact`
  (string repo-relative path or logical name); optional `claim_count`
  (non-negative integer). Ledger rows themselves are body content.
- `striatum.action_item_ledger.v1` (kind `action_item_ledger`, RFC 0004):
  required `schema_version`, `artifact_kind: action_item_ledger`,
  `source_review_artifact` (string), and `revision_round` (non-negative
  integer); optional `total_items` (non-negative integer). Action-item rows
  themselves are body content.
- `striatum.harness_improvement_proposal.v1` (kind
  `harness_improvement_proposal`, RFC 0005): required `schema_version`,
  `artifact_kind: harness_improvement_proposal`, `target` (one of `prompt`,
  `workflow`, `spec`, `defaults`, `documentation`), and `expected_benefit`
  (string); optional `risk` and `rollback` (strings).
- `striatum.escalation.v1` (kind `escalation`, RFC 0053): required
  `schema_version`, `artifact_kind: escalation`, `escalation_id`, `run_id`,
  `severity` (one of `blocked`, `human_checkpoint`), `blocker_kind` (one of
  `ambiguous_goal`, `missing_authority`, `contradicting_decisions`,
  `no_available_reviewer_lane`, `committee_stalemate`, `override_required`,
  `ai_self_declared`), `description`, `reasoning`, `requested_action`, and
  `created_at`; optional `job_id`, `session_id`, and `related_artifacts`
  (list of strings). These artifacts are AI-authored escalation requests for
  the human principal; they do not create a dedicated live-state table. When
  published through daemon `artifact.publish`, `escalation_id` is treated as
  the target blocker id for an existing escalation-class blocker in the same
  repository/run. Successful linkage stores compact metadata under
  `blockers.payload_json.escalation_artifact` and the escalation inbox
  projections surface it; publishing an escalation artifact does not create a
  new live blocker by itself.

Other artifact kinds (`prompt`, `marker`, `handoff`, `patch_summary`,
`test_report`, `other`) remain unschemaed in V1 and pass through without a
front-matter check.

Artifact kinds are validated in Python rather than by SQL `CHECK`. Migration
version 5 dropped the `CHECK (artifact_kind IN (...))` clause from the
`artifacts` table; the canonical allowed-kinds set is
`striatum.artifacts.ALLOWED_ARTIFACT_KINDS`. Both `publish-artifact`
(`ArtifactError`, exit code 6) and workflow validation (`WorkflowError`, exit
code 8) reject kinds outside that set.

## Corpus Export And Augmentation Boundary

`striatum corpus export --since <ref> --out <dir>` (RFC 0044 V1) emits a
redacted JSONL bundle of Striatum's durable provenance — RFCs, decision-log
rows, operator reports, run summaries, audit-chain entries, changelog
entries, ubiquitous-language terms, harness-friction patterns, and recent
commits — plus a verifying `manifest.json` with per-file row counts, SHA-256
hashes, and a derived `bundle_sha256`. The bundle is read-only durable
provenance, not live state, and re-running the export over unchanged inputs
produces byte-identical JSONL with stable hashes (`generated_at` is the only
allowed timestamp variation).

Corpus exports are produced on operator demand. Striatum does not stream
runtime events to any external consumer and does not call any external
service during a run. Bundles live wherever the operator points `--out`;
nothing under `.striatum/` is written by the verb. Corpus source-path checks
deny transcript/output/private path shapes case-insensitively.

The export is an **augmentation boundary**, not a runtime dependency. An
external memory or retrieval system (Engram is the first reference consumer
under RFC 0044) may ingest a bundle and serve retrieval over its rows, but
the Striatum runner does not import any consumer client library, register
any `memory.*` capability, or call any retrieval surface during state
transitions. The non-negotiable invariants are:

- No `import engram` or `from engram` in Striatum source.
- No `memory.*` capability in the Striatum daemon method registry.
- No state transition (`ack`, `publish-artifact`, `complete`, `verdict`,
  recovery, `run prepare`, `run start`, `corpus export`) that fails when
  an external memory consumer is missing, unreachable, or misconfigured.

These invariants are pinned by
`tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`.
The contract version, multi-corpus identity, redaction-tier metadata,
incremental-export watermark, and optional context-injection policy that
power V2 are scoped by [RFC 0057](rfcs/0057-corpus-contract-v2.md).

`striatum archive create --run-id <id> --out <dir>` is the Phase 11 V1 run
archive foundation. It is a daemon/Postgres-backed read command that writes
a local archive directory for one run: run row, workflow snapshot,
run-scoped rows, artifact metadata, event metadata, and a self-verifying
`manifest.json`. It does not copy artifact contents, transcripts,
`.striatum/` scratch, or any external-service state. `striatum archive
verify --bundle <dir>` checks an existing archive locally without daemon
state. `striatum archive verify --bundle <dir> --replay` adds an offline
semantic replay over the archived metadata: run/repository consistency,
FK-style references among run rows, monotonic event ordering, and event-chain
continuity when `previous_hash` / `row_hash` anchors are present. Because the
archive stores artifact metadata rather than artifact bytes, artifact content
hashes are checked only when the operator also provides `--repo-root <path>`.

## Branches And Commits

Workflow startup is gated by the workflow's `branch.mode` setting.

`branch.mode` is a closed enum: `"auto"` (the default when omitted) or
`"confirm"`.

**Auto mode (default).** When the workflow declares `branch.mode: "auto"`
or omits the `mode` field, `run prepare` atomically:

1. Validates and snapshots workflow JSON.
2. Calls `git checkout -b <suggested_name>` (idempotent fallback to
   `git checkout <suggested_name>` if the branch already exists).
3. Records the branch and transitions the run to state `ready`.

The response includes `branch_mode: "auto"`, the resolved `branch`,
`branch_created` (true only when a new branch was created), and the
`current_git_branch` for cross-check. If git checkout fails (dirty
working tree, conflicting branch), the run remains in
`needs_branch_confirmation` and the operator can resolve the issue and
run `striatum branch confirm` manually. Auto mode requires
`branch.suggested_name` to be set.

**Confirm mode (opt-in).** When the workflow declares
`branch.mode: "confirm"`:

1. `run prepare` validates and snapshots workflow JSON and leaves the
   run in `needs_branch_confirmation`.
2. `branch confirm` records explicit operator confirmation and
   optionally creates or selects a branch. Per RFC 0053 the operator
   is the AI agent by default; the human principal is not required for
   branch confirmation.
3. `run start` makes eligible root jobs claimable.

Use confirm mode for workflows that require operator review of the
target repository state before any branch is touched (e.g., RFC-style
spec reviews where the branch is part of the deliberation).

No job is claimable before branch confirmation. V1 does not commit, push,
merge, or rebase.

`branch confirm --json` is records-only by default: it includes the requested
branch and detected current git branch, warns when they differ, and reports
`records_only: true`. Three opt-in flags promote the gate from advisory to
git-enforcing:

- `--create`: run `git checkout -b <branch>` (idempotent fallback to
  `git checkout <branch>` if the branch already exists). If git refuses, the
  runner exits with `WorkflowError` (code 8) and does NOT record the
  confirmation. The response field `created` is `true` only when a new
  branch was created.
- `--use-current`: ignore `--branch` as a target and record the current git
  branch instead. If `--branch` is also given and disagrees with the
  current branch, exit with code 8.
- `--strict`: require that the current git branch matches `--branch`
  exactly before recording. If they differ, exit with code 8 and do not
  record. This is the safe default for CI and other automation.

The response also includes a `mode` field
(`"records_only" | "create" | "use_current" | "strict"`). The default
records-only mode preserves backwards compatibility for existing callers.

## CLI

Required commands, grouped by concern:

```text
# Core lifecycle
striatum init
striatum workflow validate
striatum workflow plan
striatum workflow graph
striatum workflow init
striatum run prepare
striatum branch confirm
striatum run start
striatum run summary
striatum run graph

# Agent / session work loop
striatum register-session
striatum claim-next
striatum ack
striatum heartbeat
striatum release
striatum send
striatum block
striatum publish-artifact
striatum submit-review
striatum complete
striatum verdict
striatum override-verdict
striatum decision record

# Worktree (opt-in per lane)
striatum worktree create
striatum worktree release
striatum worktree list

# Supervisor (RFC 0009)
striatum supervise start
striatum supervise send
striatum supervise stop
striatum supervise status
striatum supervise list

# Dashboard
striatum dashboard

# Inspection and recovery
striatum status
striatum why
striatum doctor
striatum evidence export
striatum recovery stale-leases
striatum recovery requeue-stale
striatum recovery resume

# Corpus export (RFC 0044 V1; RFC 0052 contract)
striatum corpus export --since <ref> --out <dir>
striatum archive create --run-id <id> --out <dir>
striatum archive verify --bundle <dir> [--replay] [--repo-root <path>]

# Adapter
striatum adapter run
```

Human read commands can pretty-print. `--json` returns stable machine-readable
JSON. Mutation commands support JSON output for agent use.

## Introspection

`status --json` keeps aggregate run and job counts and also reports open
blockers, human checkpoints, latest non-accepting review verdicts, claimable
jobs grouped by role and lane, blocked downstream jobs, and deterministic
`next_actions`.

`why <id> --json` resolves run, job, queue message, blocker, artifact,
verdict, session, and process ids. Blocker introspection includes owning
context, related verdict when present, blocked downstream jobs,
human-checkpoint context when relevant, and next actions.

### Doctor And Verbose Records

`doctor [--verbose]` returns a stable string `problems` list by default. With
`--verbose` the payload also carries a `problem_records` list of structured
records with stable `check` names (e.g. `active_job_without_active_lease`,
`stale_queue_message_claim`, `worktree_path_missing_on_disk`,
`supervisor_pid_missing`, `supervisor_stdin_pipe_missing`), the affected `id`,
and a small `context` map. The string list is preserved verbatim so callers
that already grep `problems` keep working.

### Dashboard

`striatum dashboard --run-id <id>` renders a compact, dependency-free terminal
view over the same daemon-owned PostgreSQL state that `status` and `why`
expose. It refreshes every 2 seconds by default and shows run state and branch,
job counts by state, verdict counts, open blockers (including human
checkpoints), claimable work grouped by role/lane, deterministic next actions,
and the most recent events. `--refresh <seconds>` changes cadence; `--once`
renders a single frame to stdout and exits, which makes the dashboard useful in
scripts and CI assertions that should not redraw a TUI.

When the terminal is at least 100 columns wide and 30 lines tall and the
run's workflow has at least one edge, the dashboard appends a *graph
panel*: a layered ASCII view of the workflow's job DAG annotated with each
job's current state (highest-attempt `jobs.state` per `workflow_job_id`).
State letters are `Q`/`R`/`C`/`B`/`H`/`F`/`P`/`X`/`S` for
queued/running/completed/blocked/waiting_human/failed/pending/canceled/stale_lease.
`needs_revision` cycles render after the layered grid as dashed `~~>`
arrows. Auto-detection can be overridden with `--graph` / `--no-graph`;
`--graph-only` hides the rest of the frame; `--graph-style
{auto,layered,list,fancy}` forces a layout — `fancy` uses Unicode
box-drawing characters (`┌`, `┐`, `└`, `┘`, `─`, `│`, with `╌╌▶`
for cycle back-edges) and falls back to `layered` when the per-slot
width drops below 14; `--graph-orient {tb,lr}` picks orientation —
`tb` (default) is top-to-bottom; `lr` arranges layers as columns
with `─→` separators (or `->` in non-fancy mode) and falls back to
`tb` when per-column width drops below 14; `--graph-no-cycles`
suppresses back-edges. ANSI 16 colors mirror the existing Mermaid
state palette and are emitted only on TTY output and only when
`NO_COLOR` is unset (de-facto standard); `--once` is non-TTY by
construction. The same renderer powers `striatum run graph --run-id
<id> --format ascii [--graph-style ...] [--graph-orient ...]` for
one-shot snapshots that share the same shape as the dashboard
panel.

### Run Summary

`run summary` writes a compact durable Markdown note with run id, branch
context (recorded plus current git branch with an explicit `(MISMATCH)`
annotation when they differ), run timing (`created_at`, `started_at`,
`completed_at`, and a wall-clock `duration`), job counts, verdicts grouped by
review job with attempt counts, artifacts annotated with structured author
bylines, blockers, and verification state. The renderer is deterministic so
two runs with the same daemon state produce the same Markdown.

### Recovery

`recovery stale-leases --json` applies lazy lease expiry for a run and
reports stale lease recovery context, explicitly distinguishing repo-write
work that requires operator inspection from review-only work that can be
reclaimed safely. `recovery requeue-stale --run-id <id> --job-id <id> --json`
is a bounded operator mutation for expired non-repo-write work only. It
restores the job's work message to `pending` when needed, reports when the
work was already reclaimable, and refuses repo-write jobs so abandoned write
work still requires operator inspection or a future worktree-isolated recovery
path.

`recovery resume --blocker-id <id> --json` resolves an open process-adapter
blocker after the operator has remediated missing outputs. It revalidates
required artifacts, extends the preserved process-adapter lease, marks the
blocker resolved, and returns the job to `running`. Review jobs whose only
remaining gap is the verdict can then use the normal `verdict --verdict
accept_with_findings` path. `--complete --session-id <id>` additionally
completes remediated non-review work after validation; nonzero-exit and
timeout blockers require `--force`.

`recovery auto --run-id <id>` (RFC 0020 V1) is a one-shot autonomous
sweeper composable with cron / systemd timer. In daemon RPC the
canonical method is `recovery.sweep`. The sweep first evaluates
`recovery.auto_finalize` only when the workflow opted into
`recovery.auto_finalize.enabled=true`; live sweep mode never supplies
the standalone auto-finalize `--force` override and does not make global
live auto-finalize the default. It then runs lazy lease expiry, optional
process reconciliation, optional autonomous review-only requeue
(D036-safe), human-checkpoint timeout escalation, and eligible-blocker
doctor flagging — and returns a structured envelope `{run_id, swept_at,
policy_source, dry_run, actions, escalations, still_stuck}`. Workflows
declare a `recovery_policy` block to opt into autonomous behavior.
Escalation is represented by daemon state plus blocker/escalation
artifact projections; any local notification hook is non-authoritative
and must never be treated as workflow state. CLI flags
(`--autonomous-review-requeue`, `--autonomous-process-reconcile`,
`--max-requeue`, `--checkpoint-timeout`, `--eligible-after`,
`--dry-run`) override workflow defaults. Workflows that omit
`recovery_policy` get diagnostic-only output; today's flow is
preserved as closely as the daemon PG substrate allows.

`recovery auto-publish --run-id <id> [--dry-run]` emits the explicit
`recovery.auto_publish_stale_artifacts` daemon method. It is the
stale-lease auto-publish path for declared on-disk expected artifacts.
The deprecated `recovery.auto` daemon method remains only as a
compatibility alias for older clients and is not emitted by the current
CLI.

`recovery watch --run-id <id>` (RFC 0020 step 3) is the long-lived
counterpart for operators who want one foreground command instead of
a cron entry. It wraps `run_auto_sweep` in a sleep loop with a per-
run pidfile (`.striatum/scratch/recovery-watch-<run_id>.pid`),
`SIGTERM` / `SIGINT` graceful shutdown via interruptible `wait`,
JSONL emission per sweep plus a final `watch_exit` envelope, exit-
on-terminal-run-state default (`--no-exit-on-terminal` keeps
looping), and `--max-sweeps N` for tests / probes. The same CLI
overrides as `recovery auto` are accepted and resolve once at
startup. A pidfile collision with an alive watcher exits 4 with
`another recovery watch is active (pid <N>)`; stale pidfiles (dead
PIDs) are overwritten cleanly. The watcher does not duplicate sweep
logic — every sweep is the existing one-shot path.

### Self-Contained Agent Skills

> Design rationale: [RFC 0015](rfcs/0015-self-contained-agent-skills.md).


`striatum skills install [--profile {claude_code, codex, gemini,
generic, all}] [--scope {project, user}] [--namespace <prefix>]
[--force] [--dry-run]` writes a self-contained agent skill bundle
into the target tree. The bundle teaches a Striatum-aware agent how
to drive the runner without reading the source repo: each rendered
Markdown file lists the relevant CLI verbs, the boundary conditions
the runner does not enforce (no SQLite writes, no marker files as
state, no transcript capture), and a copy-pasteable command sequence.

V1.2 ships four profiles plus an `all` fan-out:

- `claude_code` writes one SKILL.md per skill under
  `.claude/skills/<namespace>striatum-*/SKILL.md`. The five skills are
  `striatum-workflow` (router), `striatum-scaffold` (init / workflow
  init / run prepare / run start / branch confirm), `striatum-claim-loop`
  (register-session / claim-next / ack / heartbeat / publish-artifact /
  verdict / submit-review / complete / worktree create / release),
  `striatum-supervise` (RFC 0009 supervisor lifecycle), and
  `striatum-recover` (status / why / doctor / recovery / checkpoint /
  dashboard).
- `codex` writes the same five-skill content as flat files at
  `.codex/agents/<namespace>striatum-*.md`, reusing the Claude Code
  skill bodies verbatim. Manifest at
  `.codex/agents/<namespace>striatum-workflow.manifest.json`.
- `gemini` writes a single concatenated guide at
  `<namespace>STRIATUM_GEMINI_GUIDE.md`. Single-guide fallback per
  RFC 0015 § "Profile coverage" until Gemini CLI's skill-discovery
  convention stabilizes; the dedicated filename keeps `--profile all`
  collision-free with `generic`.
- `generic` writes a single concatenated guide at
  `<namespace>STRIATUM_AGENT_GUIDE.md` for any agent CLI without a
  skill-discovery convention.
- `all` fans out across the four real profiles in deterministic
  order (`claude_code, codex, gemini, generic`) and returns a
  `{"profile": "all", "results": [...]}` envelope. Per-profile
  manifests stay independent; there is no combined "all" manifest.

`--scope user` rewrites the prefix to the user's home directory so a
developer who works across many target repos installs once. The
default `striatum-` namespace can be changed with `--namespace` for
operators with a collision against an existing skill directory.

Each install writes a `.manifest.json` describing every rendered file
(rendered SHA256 + bundled-template SHA256 + runner version). A second
invocation against an unchanged tree is byte-identical. An on-disk file
whose hash differs from the manifest is `refused_modified` without
`--force`; `--force` overwrites and updates the manifest;
`--dry-run` prints the plan without writing.

`striatum init [--with-skills [profile]]` runs `init` first and then
calls the same install pipeline; default profile when the flag is
present without a value is `claude_code`. `--with-skills all` flows
through the same fan-out as `skills install --profile all`. The flag
is opt-in; default `init` behavior is preserved byte-for-byte.

`striatum doctor` checks every installed manifest across all four
profiles: `skills_missing` (a recorded file is absent on disk) and
`skills_outdated` (the manifest's runner version is older than the
running install, or a packaged template's bundled SHA256 differs from
the recorded `template_sha256`). Both surface a `recovery_command`
with the exact `striatum skills install` invocation that would clear
the condition; the runner never auto-regenerates.

The bundle is self-contained by construction: the renderer rejects
external URLs in template output (a unit test enforces no `http://` /
`https://`), templates ship inside the installed Python distribution
(`[tool.setuptools.package-data]` for `striatum.skills.templates`), and
each generated file's header records the runner version that produced
it.

## Workflow Authoring Tools

`workflow plan --json` validates a workflow and returns a dry-run plan with
claim waves, review gates, declared revision cycles, and graph nodes/edges.

`workflow graph <workflow.json>` validates a workflow and exports graph data
for authoring review. The default output is Mermaid `flowchart TD`, including
declared dependency edges, accepting-review gates, bounded revision-cycle
edges, and declared parallel groups. `--format json --json` returns stable
machine-readable graph data with nodes, edges, and cycles. `--format dot`
emits a Graphviz `digraph striatum_workflow { ... }` with the same nodes
and edges, parallel groups rendered as `subgraph cluster_<group>` blocks,
and `needs_revision` cycle edges rendered as dashed arrows; pipe through
`dot -Tsvg` to render.

`run graph --run-id <id> [--format mermaid|json]` renders the same graph for
a live run, annotated with current job state. The Mermaid output appends a
`classDef` palette and per-node `class` assignments so renderers can highlight
completed (green), running/claimed/acked (blue), blocked/stale_lease/
waiting_human (yellow), failed/canceled (red), queued (grey), and pending
(light grey, default for jobs with no row yet) jobs. The JSON form extends
each node with `current_state`, `attempt`, and a `latest_verdict` block for
review jobs.

`workflow init [--style minimal|review|code-change] <path>` writes a starter
workflow tree. The generated tree includes `<path>/workflow.json` plus role
and prompt stubs and validates cleanly with `workflow validate`. The default
`review` style mirrors the `examples/code-change-flow/` shape with placeholder
paths; `minimal` writes a single author job with no review; `code-change`
adds a one-shot `needs_revision` cycle. The command refuses to overwrite an
existing path.

## Local API And MCP Wrapper Boundary

`striatum.api.invoke(args, repo=...)` is the minimal local Python API. It
parses the same command arguments as the CLI, calls the same dispatcher, and
returns the same JSON-style result envelope:

```json
{"ok": true, "data": {}}
```

Errors use the CLI's existing exit-code semantics:

```json
{"ok": false, "error": {"message": "...", "code": 3}}
```

This API is an adapter convenience only. It must not write SQLite directly,
reimplement workflow transitions, bypass artifact validation, or define a
separate command vocabulary.

The legacy local MCP-like wrapper exposes tools over stdio JSON-RPC with
LSP-style `Content-Length` framing by default and automatic line-delimited
fallback. `python -m striatum.mcp --framing {auto,line,framed}` lets tests and
compatibility harnesses pin the wire shape. Each tool maps to an existing CLI
command or `striatum.api.invoke` call. MCP resources may expose read-only views
such as status, `why`, doctor output, or stored work packets.

Post-D103, operator-driven production runs use daemon MCP as the mandatory
tool surface. The legacy local wrapper is not an authority boundary and is not
the normal operator contract. CLI use remains acceptable only when it is
daemon-backed or when a documented bootstrap/admin/debug exception is recorded
by the operator. See `docs/MCP.md` for the wire shape and tool list.

### Local Service

> Design rationale: [RFC 0012](rfcs/0012-local-service-api.md).


`striatum serve` runs a `ThreadingHTTPServer` on TCP loopback (default
`127.0.0.1`) or a Unix-domain socket. Non-loopback hosts (`0.0.0.0`, public
IPs, hostnames that resolve outside loopback) are refused at startup with
exit 8 — the no-hosted-services boundary (D020) is preserved by
construction.

Endpoints (all return the same `{ok, data | error}` envelope as
`striatum.api.invoke`):

- `GET /v1/health` — `{started_at, version, mode}`. No DB hit.
- `POST /v1/invoke` — body `{argv: [...]}`; routes through
  `api.invoke`. Returns 405 when the argv falls outside the read-verb
  whitelist and `--allow-mutations` is off.
- `GET /v1/runs` — daemon `status`.
- `GET /v1/runs/<id>` — daemon `status` scoped to the run.
- `GET /v1/runs/<id>/why?id=<entity>` — daemon `why`.
- `GET /v1/runs/<id>/dashboard` — daemon `dashboard` DTO.
- `GET /v1/runs/<id>/events` — Server-Sent Events stream over daemon
  `run.events` in production. Honors `?since=<event_id>` and
  `Last-Event-ID` for replay. Emits a `striatum.run_terminal` event and
  closes when the run reaches a terminal state.
- `GET /v1/doctor` — daemon `doctor` with verbose problem records.

The daemon-backed read endpoints above retain legacy CLI invoke fallback only
for subprocess compatibility tests (`STRIATUM_TEST_HARNESS=1` and
`STRIATUM_DAEMON_REQUIRED=0`); production service reads fail closed when the
daemon or repository registration is unavailable.
Production service startup also calls daemon `doctor` before binding, so a
missing daemon or unregistered repository is reported before the HTTP/Unix
socket listener starts. The historical SQLite integrity check is retained
only for subprocess compatibility tests using the legacy fallback mode.

Auth: Unix sockets bind `0o600` (filesystem permissions are the
boundary); HTTP loopback supports an optional `--token` validated by
length-safe constant-time compare. Single-instance enforcement via a
PID file; stale PID files are overwritten. Graceful shutdown on SIGTERM /
SIGINT. Mutations gate behind `--allow-mutations`; daemon-routed commands
are classified as read-only only when the daemon method contract has
`required_capability: "read"`. CLI-local workflow authoring reads
(`workflow validate`, `workflow lint`, `workflow plan`, `workflow graph`,
`workflow templates`) remain explicitly allowed without mutation mode.

### Registry-Backed Multi-Repo Coordination

> Design rationale: [RFC 0028](rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md).

`striatum daemon start` (also exposed as the `striatumd` console
script) is the supported foreground entry point. Per D094 / RFC 0043
the daemon is a hard prerequisite for every Striatum CLI verb;
clients route through the daemon RPC envelope under token/capability
checks. The V1 `--no-daemon` direct-CLI path is retired and parsing
the flag returns the standard argparse "unrecognized arguments"
error. CLI verbs without a reachable daemon refuse with exit code
11 (`daemon_unreachable`); the stderr message names the socket path
and the platform-specific remediation, and no SQLite file is opened
or created.

Daemon-global state — registered repositories, clients, capability
grants, metadata-only hash-chained audit rows, audit segment
manifests, scheduler cursors, and daemon metadata — lives in the
daemon-owned PostgreSQL instance (the "daemon DB"). Per-repository
workflow tables — runs, jobs, sessions, queue messages, leases,
work packets, artifacts, verdicts, blockers, command requests,
process executions, events, worktrees, process supervisors, and
supervisor pointers — live in the same Postgres instance under a
`repository_id` scope. The historical V1 carve-out that kept those
tables in `.striatum/state.sqlite3` is superseded by RFC 0043.

RFC 0033 specifies the daemon-global PostgreSQL substrate: the
daemon connects through `STRIATUM_DAEMON_DB_URL`,
`~/.config/striatum/daemon.toml`, or an explicit `--postgres-url`
client surface. The daemon owns schema migrations and database
roles, but it does not start, stop, install, or upgrade PostgreSQL.
Bundled, embedded, and Dockerized Postgres distributions are
deferred product choices.

Daemon DB migrations are forward-only and daemon-owned. Startup
applies pending migrations and refuses to run when the on-disk
daemon schema is newer than the daemon binary. `daemon doctor`
reports substrate version, schema version, audit-chain status,
and segment-manifest verification.

The RFC 0033 cutover command for the daemon-global V1 registry is
`striatum daemon migrate --from sqlite --to pg` with `--dry-run`
for inspection and `--keep-sqlite-readonly` when the operator
wants the V1 registry file retained as an audit tombstone. After
a successful cutover marker is present, V1 registry reads are
refused and operators are pointed at the V2 daemon DB.

The RFC 0043 cutover for an existing repository's workflow state is
`striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>`
with `--dry-run`, `--keep-sqlite-readonly` (default; renames the
source to `state.sqlite3.tombstone` at mode 0444), and
`--no-keep-sqlite-readonly --confirm-delete` for irreversible
cleanup. CLI verbs against an unmigrated repo refuse with exit code
12 (`repo_not_migrated`) and point at this command. See
[`docs/POSTGRES_TRANSITION.md`](POSTGRES_TRANSITION.md) for the
full operator runbook. RFC 0048 completed in v1.55.0: production mapped
verbs are daemon/Postgres-backed and fail closed without the daemon or
repository registration. Legacy SQLite paths are quarantined for
migration sources, golden fixtures, and explicitly gated subprocess
compatibility tests only.

Registry location is platform-local and overrideable for tests with
`STRIATUM_DAEMON_REGISTRY`; runtime files are overrideable with
`STRIATUM_DAEMON_RUNTIME_DIR`. Linux uses XDG state/runtime locations;
macOS uses Application Support for registry state and Caches for
runtime files. Windows daemon support is not claimed in V1.

`striatum repo add <path>` registers an initialized target repository.
It authorizes the daemon admin token before recording the repository in
daemon-owned Postgres. If `.striatum/` scratch is absent, registration
refuses unless `--init` is passed; `--init` is the explicit opt-in that
runs the same initialization as `striatum init`. With `--no-migrate`,
registration refuses if an existing pre-D094 repo-local SQLite source
still requires migration. The command canonicalizes the repository root,
refuses symlink/path-traversal ambiguity including symlinked parent
components, derives a realpath/inode-based repository identity from the
root, and refuses active path re-occupation by a different identity. `repo remove` is
idempotent, marks the repository removed, revokes live repo-scoped
capabilities, preserves audit rows, and never reuses `repository_id`;
re-adding allocates a fresh id.

Every non-health registry-backed request requires a token. Both
`striatum daemon start` and `repo add` bootstrap one admin token when the
registry has no clients and write the local runtime fallback file with
`0600` permissions. Operators should treat runtime-file token storage as
degraded compared with an OS keyring. Plaintext token secrets are not read
from environment variables and are never stored in the registry or audit
log. Authorization uses the closed daemon method capability vocabulary:
`read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, and
`surgical_recovery`.

Daemon audit rows are metadata-only. They include client id when known,
repository id when scoped, command, authorization result, denial reason
when safe, transport, request id when supplied, exit code when known,
payload hash, and a continuous hash chain across retained rows. Audit
segment manifests record row ranges and hash anchors; closed segment rows
are SQL-guarded against daemon-API updates/deletes, and `daemon doctor`
checks retained segment manifests against retained rows. Audit does not
contain request bodies, response bodies, artifact text, blocker prose,
model rationales, terminal output, token secrets, salts, or tracebacks.
It is per-machine daemon evidence, not transcript evidence, authorship
proof, human identity proof, model proof, source provenance, or
resistance to a local filesystem writer.

All production workflow reads and mutations route through daemon RPC.
`status`, `doctor`, `why`, run dashboards, run/job/artifact detail DTOs,
SSE event reads, and `dashboard --all` read daemon/Postgres state under
capability checks. `dashboard --all` fans out across registered target
repositories and degrades individual bad repositories without treating
repository files or scratch paths as live run truth. Daemon MCP exposes
capability-gated tools derived from the method registry and read-only
resources derived from daemon state. MCP clients must pass an explicit
token parameter; repo-scoped tokens filter resource lists and are denied
when reading or mutating another repository.

RFC 0030 adds the daemon V2 RPC server-side foundation. The envelope is JSON
`schema_version: 1` with client-supplied `request_id`, dotted `method`,
object `params`, optional `capability_token`, and `deadline_ms`.
Responses echo the envelope version and request id, set `ok`, and carry
either data or a stable error object. `daemon.hello` / `daemon.welcome`
negotiate envelope version and framing; no ordinary route may run before
the handshake. Incompatible envelope or framing is refused with exit code
10 and must not silently downgrade to direct mode.

The method registry is the code source of truth for daemon V2 routes and
publishes a stable `methods_etag` through `daemon.describe`. The closed
capability vocabulary for the RFC 0030/0031 foundation is `read`,
`write`, `review`, `claim`, `apply`, and `admin`. Every authorized or
denied RPC request records metadata-only PostgreSQL request/audit
helpers: method, decision, denial reason where safe, transport, request
id, canonical params hash, row hash, and audit-chain linkage. The audit
contract still excludes request/response bodies, transcripts, artifact
contents, token secrets, salts, tracebacks, and model prose.

Post-D094 and D103, ordinary operator commands must use the daemon authority
boundary. Direct repo-local dispatch is a development/test harness path, not a
production run mode. Any installed CLI path used by an operator must route
through the daemon or be documented as a bootstrap/admin/debug exception.

RFC 0031 adds daemon-owned supervision and sealed-apply foundation
state. The daemon DB contains `daemon_supervisors` and `apply_receipts`;
`process_supervisor_pointers` live in the daemon-owned per-repo
PostgreSQL tables under `repository_id`, so packet delivery,
lane-attestation, and evidence paths read the same live substrate. The
daemon `apply.reviewed_patch` route is an AI guardrail and fails closed
without signing-key/apply authority; it is not a cryptographic
non-repudiation claim against a malicious local operator.

RFC 0032 extends the daemon V2 capability vocabulary to `read`, `write`,
`review`, `claim`, `apply`, `admin`, and `recovery`, and each registry
method declares a repository scope mode: `single_repo`, `cross_repo`, or
`daemon_global`. Daemon MCP mutation tools are derived from that method
registry when the PostgreSQL daemon substrate is active. `tools/list`
returns only the effective tool set allowed by the token's capability and
scope, while `tools/call` re-authorizes every request even if the tool was
listed earlier. Denials are metadata-audited with transport `mcp`; hidden
tools are not treated as authorized. There is no V2 daemon MCP equivalent
of `serve --allow-mutations`. Per D103, this daemon MCP surface is mandatory
for operator-driven workflow mutation, not an optional convenience wrapper.

RFC 0032 also adds daemon DB tables for `cross_repo_runs`,
`cross_repo_run_repositories`, `cross_repo_cycle_counters`, and
`audit_repositories`, plus a per-repo `runs.cross_repo_run_id`
back-reference. The daemon DB is canonical for the cross-repo run and for
each participant repository's workflow state. Cross-repo lifecycle
coordination is best-effort across local repos, not distributed
filesystem atomicity. The dogfood-035 implementation shipped unit and
mock-based lifecycle coverage. Dogfood-037 adds developer-only
end-to-end coverage under `tests/_harness/`: `MultiRepoHarness` boots a
daemon plus multiple registered target repositories on an ephemeral
PostgreSQL daemon DB and exercises the RFC 0032 prepare/lifecycle/
recovery/MCP capability/write-scope seams. The harness is test
infrastructure, not a public operator API.

The foreground sweep process uses the existing `recovery auto` policy
against active registered runs without requiring one `recovery watch`
process per run. The running process uses internal authority for its
periodic sweep. The manual `striatum daemon sweep` CLI verb is admin-gated
and audited. The sweep does not auto-resolve human checkpoints, requeue
repo-write stale work, or substitute for daemon-owned process supervision.
Each per-run sweep writes a daemon `daemon.recovery_sweep` event with
payload `author: striatumd-<instance-id>`; review-only stale requeue
events produced by this path carry the same payload author. Other
underlying recovery event bylines remain direct-recovery semantics and are
deferred to a follow-up RFC. The first sweep iteration is in registration
order; later iterations order repositories by last sweep time where cursor
data is available. Per-run ordering inside one repository remains
`runs.created_at` order. A per-run timeout marks the scheduler cursor
`sweep_degraded`, and `daemon doctor` surfaces degraded cursors and an
active `recovery watch` pidfile for the same registered run as duplicate
recovery scheduling.

Audit segment append-only manifests are implemented, but production
retention/rotation policy is deferred; the active registry can grow until
an operator or future RFC supplies rotation/export behavior.

Registry-backed mode alone does not strengthen lane attestation. RFC 0031
adds daemon-owned supervisor metadata and repo-local pointers as the V2
schema/API foundation for future stronger supervision. Existing direct
repo-local supervision remains the compatibility path until daemon
spawn, reattach, and routing take over method by method.

Python is the production daemon core. RFC 0039 introduced a Go
`go/cmd/striatumd` prototype behind the RFC 0030 envelope-v1 wire
protocol and RFC 0033 PostgreSQL substrate, but the Phase 3 architecture
decision narrows Go to supervisor/helper/runtime and developer-harness
roles. It is supporting runtime material, not a peer production daemon
implementation, and there is no plan to make Go the default.
Historical RFC 0039 Phase 1 work
(dogfood-042) remains useful as a read-only RPC skeleton and
cross-language audit/hash compatibility fixture: `daemon.hello`,
`daemon.welcome`, `daemon.describe`, `daemon.status`, `daemon.version`,
`audit.show`, and `repo.list`, plus PostgreSQL connection, migration,
and audit-chain code that reads the Python migrations under
`src/striatum/daemon_pg/sql/` directly. Production mutating verbs,
daemon-owned supervision, migration ownership, and release packaging
remain Python-daemon responsibilities unless a future accepted decision
changes that boundary.

### Local Web UI

> Design rationale: [RFC 0013](rfcs/0013-local-web-ui.md) (V1 surface
> + JSON API + SSE feed); [RFC 0022](rfcs/0022-web-ui-redesign.md) (V1
> server-rendered redesign + SVG dependency graph).


`striatum serve --web` activates the bundled UI. As of v1.11.0
the UI is a server-rendered Jinja2 multi-page app (RFC 0022 V1);
the prior hash-routed SPA is superseded. Static assets and HTML
templates ship inside the wheel via
`[tool.setuptools.package-data]` (`striatum.web.static` for CSS /
JS islands, `striatum.web.templates` for `.html`). The handler
resolves both via `importlib.resources.files(...)` so editable
installs and wheel installs both work.

Routes:

- `GET /` → server-rendered `run_list.html`.
- `GET /run/<run_id>` → `run_detail.html`. Includes an inline
  SVG dependency graph (RFC 0022 step 3) — layered top-down
  layout, state-colored nodes via CSS custom properties, click
  to navigate to a job's detail page, SVG `<title>` tooltip on
  hover for accessibility. Production page-state reads use daemon
  `run.detail`; the web service keeps HTML/SVG rendering local, and the
  legacy SQLite page read exists only for subprocess test fixtures.
- `GET /run/<run_id>/posture/<posture>` →
  `run_posture_verdicts.html`. Production reads use daemon
  `run.posture_verdicts`; the legacy SQLite path exists only for
  subprocess test fixtures.
- `GET /run/<run_id>/job/<job_id>` → `job_detail.html`. Job
  metadata + verdict + posture chip + artifacts list. Production
  page-state reads use daemon `job.detail`; override-verdict context-token
  minting remains local to the web service, and the legacy SQLite page read
  exists only for subprocess test fixtures.
- `GET /run/<run_id>/artifact/<artifact_id>` →
  `artifact_view.html`. Metadata + sha256 + raw-API pointer.
  Production metadata reads use daemon `artifact.show` with optional
  web context for run scoping, expected author line, and provenance trail
  rows; the legacy SQLite path exists only for subprocess test fixtures.
- `GET /doctor` → `doctor.html`. Production page state comes from daemon
  `doctor` with verbose problem records; direct SQLite is limited to the
  subprocess compatibility fallback.
- `GET /static/<path>` → bundled asset (CSS / JS islands).
- All HTML responses set `Content-Security-Policy: default-src 'self';
  script-src 'self'; style-src 'self'; img-src 'self' data:;
  connect-src 'self'` — byte-identical to v1.10.0.
- Path traversal (`..`, leading `/`, null bytes) on
  `/run/<id>/...` paths is rejected with HTTP 400.
- `GET /v1/artifacts/<artifact_id>/raw` streams the raw bytes of a
  published artifact; read-only, no mutation gate. The endpoint uses the
  default daemon `artifact.show` metadata shape and does not request web
  context.
- `GET /v1/health` includes an `allow_mutations: bool` field
  (RFC 0013 step 7); the page reads this on load to decide
  whether to render mutation buttons.
- `GET /v1/runs/<id>/artifacts` returns the run's full artifact
  rollup from daemon `list.artifacts`.

A small JS island (`/static/legacy_hash_redirect.js`) loaded by
`base.html` reads `window.location.hash` on page load and
rewrites legacy `#/run/<id>` SPA URLs to `/run/<id>` so
bookmarked SPA URLs still reach the right page.

Visual styling: CSS custom-property palette in `base.css` with
`@media (prefers-color-scheme: dark)` overrides (no toggle
button — the OS preference is the source of truth). System font
stack, 4px-grid spacing scale. State-colored status pills and
posture chips reuse the same CSS variables as the SVG graph
nodes, so dark-mode rendering inherits consistently.

When the service was started with `--allow-mutations`, the SPA
also renders five click-driven mutation buttons that POST to
`/v1/invoke` with the same argv shapes the CLI accepts:

- **Continue blocker** / **Cancel blocker** (job-detail view,
  open blocker present) → `striatum checkpoint resolve
  --blocker-id <id> --action {continue, cancel}`.
- **Record verdict** (review-job detail, state = running) →
  `striatum verdict --session-id <s> --job-id <j> --lease-id <l>
  --verdict <v> [--rationale <text>]`.
- **Record decision** (run-detail view; no lease required) →
  `striatum decision record --run-id <r> --path <p> --outcome <o>
  --title <t>`.
- **Requeue stale review** (job-detail, state = stale_lease,
  review-only) → `striatum recovery requeue-stale --run-id <r>
  --job-id <j>`.

Each button opens a confirmation modal showing the literal argv
before firing. Destructive actions (cancel job, reject verdict)
get red confirm buttons. The runner-side `--allow-mutations` gate
remains authoritative; SPA-side hiding is only a UX hint.

## Adapter Boundary

The minimum integration contract is process-based: command array, cwd, env,
stdin, stdout, stderr, exit code, and optional PTY/tmux wrapping. Provider
features live in lane command configuration. Core scheduling does not parse
terminal output or infer behavior from provider names.

Adapter constraint enforcement has four levels: `enforced` (the adapter
prevents the constraint from being violated), `advisory_strict` (the adapter
takes best-effort steps the agent cannot easily undo, such as scrubbing proxy
env vars or setting `STRIATUM_NETWORK_POLICY` / `STRIATUM_REPO_SCOPE`
sentinels), `advisory` (the constraint is recorded and surfaced but not
mechanically restricted), and `unsupported` (the adapter cannot represent
the constraint). Workflow validation rejects a lane whose `required_enforcement`
asks for a stronger level than the adapter can provide.

`adapter run` is the remaining single-shot process-adapter compatibility
path. It launches the configured `process` lane command for an active
claimed lease, can pass the stored work packet on stdin, sets
`STRIATUM_*` environment variables, creates a
`.striatum/scratch/<process_id>` scratch directory, and records process
metadata plus lifecycle events through the legacy compatibility schema.
The daemon-owned supervised-session path is the production long-lived
process path. Stdout and stderr are suppressed unless the operator
explicitly requests inherited stdio; Striatum does not capture transcripts.
The process adapter graduates `network=forbidden` and
`repo_scope=local_only` to `advisory_strict`; transcript-off is `enforced`.

### Process Supervision

`adapter run` is single-shot: the child exits with the configured command,
and the next work packet must spawn a fresh process. Long-lived agent CLIs
(Codex, Claude Code, Gemini CLI, etc.) need a different shape: one
persistent process that receives multiple work packets across multiple
turns. RFC 0009 introduces a `striatum supervise` command group plus a new
`process_supervisors` table for that flow. The two adapter modes coexist;
`adapter run` is unchanged.

#### Single-Shot Process Adapter Completion Guarantees (RFC 0014 V1)

After every `adapter run` exit (including timeout-fired SIGTERMs), the
runner inspects required `expected_artifacts` and, for `type: "review"`
jobs, whether a verdict was recorded. When any required output is
missing — or the child exited non-zero or hit the timeout — the job
transitions from `running` to `blocked`, a blocker row is inserted, and
a privacy-safe diagnostic envelope is recorded as the new
`blockers.payload_json` column.

`--timeout-seconds <n>` on `adapter run` wraps `process.communicate`
with a deadline; on expiry the child is SIGTERM'd, then SIGKILL'd
after a 5-second wait. `lanes.<id>.adapter_timeout_seconds` provides
a per-lane default (capped at 86400 / 24 hours by workflow validation);
the CLI flag overrides the lane field; with neither set, behaviour
stays unbounded for backwards compatibility.

Blocker reasons (`blockers.blocker_kind`):

- `process_outputs_missing` — exit `0`, required artifact(s) missing.
- `process_review_verdict_missing` — exit `0`, review job without a
  recorded verdict.
- `process_exit_nonzero` — non-zero exit (priority over output
  checks).
- `process_timeout_exceeded` — `--timeout-seconds` fired.
- `process_lost_with_outputs_missing` — reconciler found a dead PID
  whose job had missing required outputs.

`striatum recovery process-reconcile --run-id <id>` walks
`process_executions.state = 'running'` rows; for each, `os.kill(pid, 0)`
checks liveness. Externally-killed rows transition to `'lost'` and
re-run the same output validation; the JSON output mirrors the
existing `recovery requeue-stale` shape (D036's lazy-on-CLI policy).
Two doctor checks surface bookkeeping mismatches:
`process_running_but_pid_gone` and
`process_running_with_expired_lease`. `striatum status --run-id`
gains a `process_health` summary key.

The diagnostic envelope contains zero child stdout/stderr (D028
preserved by construction); it carries only metadata Striatum
already collected plus output-validation deltas:

```json
{
  "envelope_version": "striatum.process_adapter.envelope.v1",
  "process_id": "proc_<hex>",
  "command": [],
  "exit_code": 0,
  "duration_seconds": 0.0,
  "timeout_seconds": null,
  "missing_artifact_paths": [],
  "review_verdict_missing": false,
  "recovery_commands": []
}
```

`process_supervisors` is added by migration version 4 and is separate from
`process_executions` so single-shot launches and supervised sessions keep
distinct rows and event streams. State values are
`('starting','attached','detached','lost','stopped')` and a partial unique
index on `session_id` enforces "at most one active supervisor per session"
without blocking historical `stopped` or `lost` rows.

The supervise CLI surface:

- `striatum supervise start --session-id <id>` validates the session is
  active and that its lane uses the `process` adapter, refuses if the
  session already has a supervisor in `('starting','attached','detached')`
  state, creates `.striatum/scratch/<supervisor_id>/stdin.pipe` via
  `os.mkfifo`, forks the lane command with `start_new_session=True`, sends
  stdout/stderr to `DEVNULL` (no transcripts, per D028), and transitions
  the row to `attached` once the child pid is alive. A
  `supervisor.starting` and `supervisor.started` event are recorded.
- `striatum supervise send --session-id <id> --packet-id <id>` looks up
  the stored work packet, writes its `packet_json` plus a trailing newline
  to the supervisor's named pipe, refreshes `heartbeat_at`, and records a
  `supervisor.packet_delivered` event with the byte count and stdin-delivery
  mode. By default, the agent reads packets line-by-line from a persistent
  FIFO. A lane can opt in to
  `supervision.stdin_delivery: "one_shot_eof"` with pipe transport for
  commands that read all stdin and require EOF before they start work; after
  one packet write, Striatum closes/removes the FIFO and marks the one-shot
  stdin as consumed. Reactions remain CLI-driven (publish, ack, complete,
  verdict) so the supervisor never parses agent stdout.
- `striatum supervise stop --session-id <id> --reason <text>` sends
  `SIGTERM`, waits up to five seconds, falls back to `SIGKILL` if the
  process is still present, removes the FIFO, marks the row `stopped`,
  and records `supervisor.stopped`.
- `striatum supervise status --session-id <id>` probes PID liveness via
  `os.kill(pid, 0)` and lane progress through the active lease/session/
  supervisor heartbeat timestamps. An active row whose pid is gone is
  transitioned to `lost` with a `supervisor.lost` event before returning.
  An attached row whose pid exists but whose active lease has stale progress
  returns `liveness: "stalled"` plus `last_progress_at`,
  `last_progress_age_seconds`, active lease metadata, and
  `stall_after_seconds`. Status itself never starts or kills processes.
- `striatum supervise list --run-id <id> [--state <state>]` lists rows
  for a run, optionally filtered by state.
- Daemon RPC `supervise.reattach_status` returns a read-only
  supervisor health DTO for a run/session/supervisor filter. It compares
  repo supervisor rows, daemon supervisor pointers, daemon supervisor
  rows, PID liveness, and PID start-time identity, classifying each row
  as `reattachable`, `lost_candidate`, `needs_repair`,
  `needs_verification`, or `terminal`. It does not mutate state; actual
  restart reattach/lost-state transitions remain daemon lifecycle work.

Recovery: before ordinary stale-lease handling, `recovery.sweep` evaluates
attached supervisors with active claimed/running work. A stale-but-unexpired
heartbeat emits `supervisor.heartbeat_stall` once per lease/supervisor so
`doctor`, `why`, and status surfaces show the lane as suspect. When the
same attached supervisor's active lease has expired, sweep opens a
`heartbeat_stall_lease_expired` blocker, transitions the job/message to
`blocked`, expires the lease with `release_reason='heartbeat_stall'`, marks
the supervisor row/pointer `lost`, and records `supervisor.heartbeat_stall`,
`lease.expired`, `job.blocked`, and
`supervisor.lease_expired_with_supervisor`. The OS process is not
auto-killed; operator inspection is required, mirroring D036's stale-lease
policy for repo-write work.

The Go `striatum-supervisor-helper` is a narrow process/PTY helper. It emits
newline-delimited control events with schema
`striatum.supervisor_helper.event.v1`: `agent_started`, `packet_accepted`,
`progress`, `artifact_observed`, `helper_error`, and `agent_exited`.
Daemon `supervise.report` can consume those helper events as JSONL text, a
path, or an object list and records them through the same durable
`supervisor.<event>` event path used by wrapper reports. Helper timestamps are
preserved as `reported_at`; `agent_exited` applies the normal stopped-state
transition. The control channel carries lifecycle metadata and byte counts,
not model transcript output.

Doctor: `striatum doctor` flags supervisors in `('starting','attached',
'detached')` whose pid is gone, and `attached` supervisors whose
`stdin_pipe_path` no longer exists on disk. It also surfaces
`supervisor_lost_with_held_lease` (HARNESS-001) when a supervisor row
is in state `lost` while the session still owns an unexpired active
lease — the symptom that the supervisor exited before the work
completed and the run is silently stuck. `striatum status` adds the
stable next-action `recover_orphan_supervisor` for the same condition
so dashboards and scripts react before the lease default expiry (30
minutes) is hit. In daemon/Pg mode, `doctor` also surfaces non-healthy
`supervise.reattach_status` states (`lost_candidate`, `needs_repair`,
and `needs_verification`) so stale supervisor repair is visible before
a mutating recovery path runs. It also reports
`supervisor_attached_stale_heartbeat` and
`supervisor_heartbeat_stall_lease_expired` for attached supervisors whose
control-plane progress is stale. `striatum supervise stop` is idempotent
against a supervisor whose latest row is already `lost` or `stopped`:
rather than raising `InvalidTransitionError`, it returns the existing
terminal row plus a `note` describing the prior state.

#### Supervised Lane Command Contract

The `lanes.<id>.command` array configured for a process-adapter lane
is the program Striatum forks under `supervise start`. To work with
the supervised flow, that command must satisfy three requirements
(absent any of them, `supervise start` happens, but the run silently
fails to advance and `doctor` surfaces
`supervisor_lost_with_held_lease`):

1. **Choose an explicit stdin-delivery contract.** The default
   `persistent_fifo` contract is for wrappers that stay alive across
   packets, keep stdin open, and continue reading newline-terminated
   packets until SIGTERM. One-shot print-mode CLIs that read all stdin
   before starting work must opt in to
   `supervision.stdin_delivery: "one_shot_eof"` and should expect a
   single packet for that supervised process.
2. **Read JSON packets from stdin according to that contract.** Each
   delivery is the work packet's `packet_json` followed by a trailing
   newline. Persistent wrappers parse one packet per line; one-shot
   commands read until EOF and parse the single payload they receive.
3. **Call back via the `striatum` CLI.** The agent advances workflow
   state by invoking `striatum ack`, `heartbeat`, `publish-artifact`,
   `block`, `verdict`/`submit-review`, and `complete` with the
   identifiers from the packet. The supervisor sends stdout and
   stderr to `DEVNULL`; the agent's only durable output is the
   artifacts and verdicts it records via the CLI.

A working supervised lane therefore needs an agent that knows the
Striatum protocol — a project skill, an embedded loop, or a wrapper
script — not just a raw model invocation. The shipped wrappers live at
`.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh`. Each
wrapper consumes newline-delimited work packets, starts a fresh
provider CLI invocation per packet with non-interactive tool-approval
flags, logs provider stdout/stderr under `STRIATUM_SCRATCH_DIR`, and
traps `SIGTERM` to clean up the in-flight inner process. Tests under
`tests/test_claude_supervised_wrapper.py` verify those loop semantics
with provider-command stubs so they do not depend on real agent
binaries. dogfood-001's HARNESS-001 captured the "default scaffold
ships a non-viable lane command" foot-gun; this contract is the
explicit form of what that proposal asked the runner to require.

### Worktree Isolation

Lanes may opt into per-job filesystem isolation by setting
`worktree_isolation: "per_job"`. The default is `"off"`, which keeps current
single-worktree behavior. When a lane is configured for `per_job` isolation,
work packets for repo-write jobs in that lane include
`worktree_required: true` and a `commands.worktree_create` invocation. The
runner does not auto-create worktrees on claim; the agent must call
`striatum worktree create` itself.

`striatum worktree create --session-id ... --job-id ... --lease-id ...`
validates the active lease, requires the lane to declare `per_job` isolation,
requires the job to be repo-write, and rejects requests when an active
worktree already exists for the job. It runs
`git worktree add --detach .striatum/worktrees/<worktree_id> <base_branch>`
based on the run's confirmed branch and records a row in the new
`job_worktrees` table with state `active`. `striatum worktree release
--worktree-id <id>` runs `git worktree remove --force` and marks the row
`removed`; releasing an already-terminal row is a no-op. `striatum worktree
list [--run-id <id>]` returns the rows verbatim plus each job's
`workflow_job_id`.

`publish-artifact` continues to validate write scope and content against the
logical repo-relative path, but when an active worktree exists for the job it
reads the file from `<worktree_path>/<logical_path>` and records the
artifact's `repo_path` as the logical path. Artifacts remain durable
provenance for the main branch regardless of which worktree the work
happened in.

Lazy lease expiry preserves the worktree directory for operator inspection.
The `job_worktrees` row is marked `abandoned` and an event is emitted, but
`git worktree remove` is not run. `striatum doctor` flags active worktrees
whose lease is no longer active and active worktrees whose path no longer
exists on disk.

## First Validation Fixture

The first fixture is RFC-ledger cleanup:

```text
draft -> parallel reviews -> findings ledger -> synthesis -> final review
```

Tests exercise it with fake sessions and no live model calls.

A smaller generic docs-only workflow fixture also lives at
`examples/docs-review-flow/workflow.json`. It covers draft, review, and apply
steps without Engram-specific paths or live model requirements.

## Verification

The required check is:

```bash
make test
```

The smoke sequence is:

```bash
striatum init
striatum status --json
striatum doctor
```
