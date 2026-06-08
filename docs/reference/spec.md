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
Slack/remote serving, durable transcript capture, provider SDK
integration, malicious-local-operator-resistant sealed apply, or
automatic commits.

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
repository. Per [D094 / RFC 0043](../rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md),
this supersedes the V1 carve-out that kept repo-local workflow state
in `.striatum/retired-local-state`. The daemon is a hard prerequisite for
every Striatum verb; the V1 `--no-daemon` flag is retired and parsing
it returns the standard argparse "unrecognized arguments" error.
Repository artifacts are durable provenance only. Marker files, tmux
panes, terminal output, and provider hooks are never live
control-plane state. See [`docs/how-to/postgres-transition.md`](../how-to/postgres-transition.md)
for the operator runbook.

RFC 0048 (v1.49.0 → v1.55.0) completed the PostgreSQL substrate port:
every single-repo mutation, recovery, and read handler had a native
PostgreSQL implementation before the Go cutover. D107 / RFC 0068 and D111
set the current target architecture: the production daemon is Go; the Python
daemon module and selector are removed; and the Python MCP wrapper is removed.
RFC 0078 removes all Python traces from the active repository head.
The legacy local-state package, root DB/migration facades, direct corpus
exporter, V1 local-state schema module, deterministic repo-local fixture,
and broad skipped compatibility tests are deleted. Remaining
legacy SQLite handling is completely retired, and registrations reject any old SQLite files.
`STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` escape no
longer takes effect for ported methods — mapped CLI verbs fail
closed instead of falling back to any local database when the daemon is
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

`striatum repo add <path> [--init]` registers a target repository
with the daemon. `--init` creates `.striatum/scratch` and the
`.gitignore` entry for a fresh target repository.
The authoritative workflow state lives in the daemon-owned PostgreSQL
instance under a `repository_id` scope; `.striatum/` is operational
scratch for supervised wrapper FIFOs, pidfiles, and transient supervisor
scratch.

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
exit code 10. The pre-D094 repo-local migration implementation is deleted;
ordinary CLI verbs do not apply or import it.

Day-zero setup is guided by `striatum repo add --init`, `striatum skills
install`, `striatum plugin install`, `daemon install/start/status`,
and `doctor`. These helpers are bootstrap surfaces: they may render
agent-side bundles, register a repository, render a user service,
repair local Postgres grants, or run smoke checks, but they do not
become an alternate workflow-state authority.

Writable SQLite import windows are closed. The retired `migrate-repo-local`
and `daemon migrate` spellings are fully removed. CLI verbs against an unregistered repo refuse with exit code 12
(`repo_not_migrated`) and point operators to register with `repo add --init`; CLI verbs without a
reachable daemon refuse with exit code 11 (`daemon_unreachable`). Neither
refusal opens or creates any local database file.

### Daemon→PostgreSQL authentication and the database-enforced write boundary (RFC 0110)

RFC 0110 (accepted D164; authoritative implementation spec at
`docs/operator/artifacts/rfc-0110-pg-auth/spec_publication/synthesis/SPEC_PUBLICATION.md`)
moves the "artifact/RPC API is the sole write path" invariant out of the daemon
process and into PostgreSQL itself, in phases. This is the binding model; the
phase nomenclature and claim-keying below are normative.

- **Authority gate (L1).** Protected durable surfaces are written only through
  owner-owned `SECURITY DEFINER` functions that begin with
  `assert_daemon_authority()` — a check of a per-instance, RAM-only secret whose
  digest lives in an **owner-only** registry the runtime role (`striatumd_rw`)
  cannot read. Direct DML on a protected surface is `REVOKE`d from the runtime
  role per phase, so a leaked runtime DSN cannot forge artifacts or tamper with
  the audit chain: it lacks both the table-level privilege and the secret. Two
  guarantees are separated: **G1** invariant integrity (append-only / hash-chain
  / attempt-scope cannot be violated) and **G2** daemon issuance (a write
  succeeds only if the caller presents the daemon-authority secret).
- **In-transaction attribution prelude (L3).** Every mutating transaction is
  opened through a single constructor that, as its first statement and over the
  **extended protocol** (bound parameters, never simple-protocol text), sets the
  authority secret plus `striatum.rpc_id` / `striatum.principal_id` /
  `app.session_id` labels so neither secret nor labels appear in
  `pg_stat_activity`. These GUCs are **attribution labels only, never
  authority**; RLS row-scoping is defense-in-depth under an already-authorized
  session, never the trust boundary.
- **Fail-closed, mutation-coupled audit.** For a mutating RPC the audit row is
  the final write **inside the same transaction**, so success-without-audit-row
  is impossible. Standalone audit appends (reads, denials, transport errors)
  convert an append failure into an `audit_append_failed` error rather than
  succeeding silently. The audit chain hash moves in-DB through a versioned
  length-prefixed canonical form (`hash_format_version` 3) that is byte-identical
  in Go and PL/pgSQL; the prior Go-only v2 hash remains the permanent reader of
  pre-cutover rows, and the cutover is a single operator-gated release step.
- **Phased write closure (claim-keyed).** Closure lands in fixed phases, each
  reported by the `daemon doctor` `pg_write_boundary` posture string, and every
  operator-facing claim is keyed to it: **P0 `audit_only`** (`audit_log`
  DB-enforced) → **P1 `audit_artifacts`** (+ `artifacts`) → **P2 `full`**
  (+ `events`). The claim "the daemon's durable write paths are DB-enforced" is
  reserved to **P2 `full`** and may not be made before the posture string reads
  `full`.
- **Ephemeral runtime credential (L0) and lane isolation (L2).** The runtime
  password is owner-bootstrapped and RAM-only, re-rotated each daemon restart, so
  a captured DSN string dies at the next restart; where owner==runtime (the
  documented live PEER posture) rotation is inert and surfaces as a doctor
  posture finding. A dedicated PG-less lane OS user plus a `0700` socket
  directory make PostgreSQL unreachable from lanes as the hardened default; the
  daemon asserts the owner-only socket directory at startup and refuses
  permissive/custom shared socket directories.
  The daemon consumes `STRIATUM_LANE_OS_USER` for supervised lane launch and
  records `run_as_user` metadata, but **GH #87 closes only when the host has the
  dedicated PG-less lane OS user, passwordless sudo, PostgreSQL rejection rules,
  `make lane-isolation-check` (`T-LANE-ISOLATION-NEG`) green in
  hardened-profile CI for both UNIX-socket and loopback TCP probes, and blocking
  doctor behavior under the secure profile all live**. The current
  secure-profile doctor gate is controlled by
  `STRIATUM_SECURITY_PG_SOCKET_HARDENED`; when enabled, `lane_pg_reachable` is a
  blocking doctor problem instead of an advisory warning.

Read confidentiality against a *live* runtime credential is **not** fully
claimed by RFC 0110/#164 yet. Two behavior-changing reductions have landed:
owner bundle 0005 (RFC 0113 R1) revokes direct runtime `SELECT` on
`striatumd.clients.token_hash` and `striatumd.clients.token_salt`, and owner
bundle 0006 (RFC 0114 / D173) transfers ownership of `striatumd.principals`,
`striatumd.principal_clients`, and `striatumd.client_sessions` to the owner
role and revokes direct runtime reads (`principals`, `client_sessions`: full
deny; `principal_clients`: `principal_id` column denied). Gated reads route
through owner-owned `SECURITY DEFINER` projections guarded by
`assert_daemon_authority()`, with SQLSTATE-driven fallback while a database
has not had the bundles applied. `daemon doctor` DERIVES the `pg_read_scope`
posture from authority stamps plus live privilege/ownership probes: with
bundles 0005+0006 applied and verified it reports `partial_projection_gated`
(four gates: `clients` columns, `principals` table, `principal_clients`
columns, `client_sessions` table), otherwise `broad_runtime_select`, with a
`grant_drift` array naming any re-opened surface. `private_read_denial`
remains `false`: prose/workflow tables (RFC 0113 R2), artifact/event metadata
(R3), and `client_capabilities` (RFC 0114 OQ1) are still directly selectable,
so L0 rotation and L2 lane isolation still bound that remaining exposure. The
read surface is inventoried in `go/pkg/db/read_authority_inventory.go` and
guarded against unclassified table growth. #164 stays open for the remaining
surfaces.
The decision log records each per-phase decision on landing.

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
the workflow snapshot for a run is taken from that file. `workflow
generate` is the current scaffold generator; it supports catalog
shapes such as `minimal`, `review`, and `code_change` and previews by
default unless `--write` is passed. Checked-in `examples/` are
fixtures or starting points, not runtime defaults.

RFC 0034 V1 adds a workflow generator over the same schema. RFC 0078 has
ported the active Go generator path to reuse Go workflow validation and lint
before returning output. The legacy Python generator API is retired.
`shape: "multi_phase"` emits V1.1 with ordered `phases` and
`phase_synthesis` jobs. `shape: "implementation_panel"` emits a V1
workflow from RFC 0074 role/adversary pack options such as
`implementation_panel_roles`, `maintainer_cost`, and `operator_ergonomics`;
it remains a normal workflow tree and does not use RFC 0052 typed
committee artifacts. RFC 0093 collaboration shapes
(`falsification_gate`, `cross_examination`) emit V1.1 phased workflows with a
`phase_synthesis` adjudicator job, a `collaboration_ledger` artifact, and a
bounded `needs_revision` cycle back into the dialogue phase. These two generated
shapes are static challenge/rebuttal gates over published artifacts; they do not
keep the author or holder session live and do not imply runtime model-family
attestation for manually registered sessions. `shape:
"adjudicated_constraint_extraction"` emits a V1.1 phased workflow whose
cross-examiner jobs declare explicit interrogation consumers targeting
`convener_draft`; the shape remains experimental until a later graduation
decision changes the support tier. Other built-in shapes emit V1.
Generator preview envelopes also include the same advisory workflow lint
payload exposed by `workflow lint`, including warning count and coverage
summary; lint remains informational and does not change validation
status.

The bundled template catalog is package data under
`striatum.workflow_templates/catalog.json`; V1 does not fetch remote
templates and does not load target-repository catalog extensions.
`workflow templates list/show` expose catalog metadata, and `workflow
templates render-md` writes or prints a Markdown catalog summary with
Mermaid graph-preview diagrams for shape entries.
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
edges, bounded cycles, repo-relative artifact paths, valid shared-resource
declarations, and declared parallelism with disjoint write scopes or
review-only unique artifact paths.

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

> Design rationale: [RFC 0010](../rfcs/0010-tool-harness-profiles.md).

Workflows may declare an optional `harness_profiles` map at the top level
and reference one profile per lane via `harness_profile_id`. The map is a
passthrough projection surfaced to work packets; it does not change
adapter or scheduler behaviour.

V1 validation rules:

- `tool_family` must be one of `generic`, `codex`, `claude_code`,
  `agy`. Other values are rejected.
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
  repo root. Bare binary names (`codex`, `claude`, `agy`) and
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

### Implementation Envelope

When a workflow job has reachable downstream jobs with declared
`write_scope` or `expected_artifacts`, `claim-next` adds
`context.implementation_envelope` to the work packet. The block lists the
reachable downstream jobs in graph order with their `workflow_job_id`, type,
role/lane, write scope, and expected artifacts. Its instruction tells design
and synthesis lanes to keep recommended implementation layouts inside those
frozen downstream envelopes, or to call out that the scope must be revised
instead of assuming it can widen later. Workflows with no downstream envelope
produce packets without this key.

### Shared Resources

Workflow jobs may declare `shared_resources` for mutable resources outside the
repository tree, such as a test database, hardware device, or external fixture
that parallel jobs could collide on. Entries may be strings, which are shorthand
for an exclusive resource, or objects:

```json
{
  "shared_resources": [
    "postgres:test-db",
    {
      "id": "postgres:test-db",
      "mode": "per_lane_namespace",
      "namespace": "reviewer_a",
      "description": "DB-backed validation fixture"
    }
  ]
}
```

Object entries require a non-empty `id`. `mode` defaults to `exclusive` and may
be `exclusive` or `per_lane_namespace`. Namespace mode requires a non-empty
`namespace`. The validator rejects malformed declarations. `workflow plan` and
graph JSON include declared resources on each planned job.

`workflow lint` warns with `parallel_shared_resource_contention` when jobs in
the same `parallel_group` share an exclusive resource id, or when
`per_lane_namespace` jobs reuse the same namespace. The warning is
informational; V1 does not create daemon-owned scheduler locks for external
resources.

When a claimed job declares shared resources, `claim-next` adds
`context.shared_resources` to the work packet. The block lists the current
job's declared resources, includes the `parallel_group` when present, and names
related parallel jobs with overlapping resource claims. Agents should serialize
mutating validation against exclusive resources or use the declared namespace
before running resource-mutating gates. Workflows with no declared shared
resources produce packets without this key.

### Explicit Interrogation Consumers

Workflow jobs may declare `interrogation_targets` (RFC 0112 V1) to consume the
live preserved context of an upstream interrogable job without adding fake graph
edges:

```json
{
  "id": "cross_examiner_1",
  "interrogation_targets": [
    {"workflow_job_id": "convener_draft", "required": true}
  ]
}
```

Each target entry must be an object with a non-empty `workflow_job_id`; `required`
is optional and defaults to `false`. Validation rejects duplicate targets,
self-targeting, unknown target jobs, targets that are not `interrogable: true`,
targets that are not upstream of the consumer in the workflow graph, and chained
consumers (a job that is itself `interrogable: true` may not declare
`interrogation_targets` in V1). Unknown target-entry fields are lint warnings,
not validation errors. `workflow lint` also warns when a job declares more than
three targets, when its lane does not advertise `interrogate`, or when a target
is already a direct dependency and the declaration is redundant.

The resolver is snapshot-derived: it reads the frozen workflow snapshot and live
`jobs` rows for the run. Striatum does not materialize interrogation consumers in
a table, does not add synthetic `job_dependencies`, and does not infer consumers
from terminal output or provider hooks.

An interrogable target's preserved-context window stays open while any direct
dependent or explicit interrogation consumer is non-terminal. Terminal consumer
states for this predicate are `completed`, `failed`, `canceled`, `skipped`, and
`waiting_human`. When a consumer enters one of those states, the terminal hook
evaluates the union of direct and explicit consumers and closes the target
session with `interrogation_window_closed` once none remain pending. Re-opening
the target for a revision closes the prior attempt's target session and any open
interrogations with `revision_reopened`; the fresh attempt gets its own
`session.awaiting_interrogation` event. That event payload includes the target's
current `workflow_job_id` and `attempt` so packet projection and evidence queries
stay attempt-scoped.

`required: true` is advisory in V1. It never blocks claiming, completion,
verdicts, or human checkpoints. Instead, terminalizing a consumer that skipped a
required target records `interrogation.required_skipped`, and an attempted open
against a legitimately retired target records
`interrogation.unavailable_signaled`. These evidence events are daemon DB facts,
not durable transcript capture.

When a job declares targets, `claim-next` adds
`context.interrogation_targets` to its work packet. Each entry includes the
declared `workflow_job_id`, `required`, `state`, `reason`, and instruction text.
Available targets also include `target_session_id` and `target_attempt`.
Unavailable or not-yet-ready targets may include `target_attempt` and a reason
such as `revision_reopened`, `target_skipped`, or `target_retired`. The
projection is attempt-aware and must not point a consumer at a stale prior
attempt target after a revision re-open. Jobs without declarations produce
packets without the block.

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

> Design rationale: [RFC 0018](../rfcs/0018-focused-adversarial-review-postures.md);
> see also [`docs/dogfood/016/decisions/V1_ACCEPTANCE.md`](../dogfood/016/decisions/V1_ACCEPTANCE.md)
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
or, when registered with `--operator-label <label>`, under the YAML-safe
form `author: operator-self-declared-<label>`. Operator labels must match
`^[a-z0-9._-]{1,64}$` and may not be reserved attestation terms, lane
ids, or role/model/ordinal-shaped bylines. Older packets may contain the
historical title-block form `author: operator [self-declared: <label>]`;
the publisher canonicalizes that legacy form to the current YAML-safe
line when it appears outside front matter, but new packets do not emit it.

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

### Operator Modes

Workflows may declare `operator_mode: constrained` as the RFC 0099 Phase 1
operator assertion. The assertion is surfaced in `status --run-id` and
`run.summary` so the operator boundary is visible in run reads. In V1 this
field is advisory: it does not by itself sandbox an AI operator. The initial
RFC 0099 mediated surfaces are `repo.write`, an exact-content mutation,
`repo.patch-preview` / `repo.patch-apply`, scoped unified-patch operations that
check applyability and all changed paths before mutation, and `process.run`, a
command-array execution surface. These surfaces require active job leases;
repository mutation surfaces require repo-write jobs and refuse paths outside
the job's `write_scope` before writing. `process.run` requires
`capability_requirements.process_execution: true` and records
`process_executions` evidence without durable stdout/stderr transcripts. If the
job lacks that process-execution opt-in, `process.run` is refused unless a prior
typed escape decision exists for the same run with `escape_surface` equal to
`process.run` or `shell_command` and `escape_action` equal to either the joined
command text or `sha256:<command_sha256>`.

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

### Per-run serialization invariant (RFC 0104)

Every transaction that mutates a single run's aggregate — its rows in
`sessions`, `runs`, `jobs`, `leases`, `queue_messages`, `verdicts`, `blockers`,
and `interrogations` — MUST take a per-run, transaction-scoped advisory lock
(`pg_advisory_xact_lock` keyed on `striatum:run:<repository_id>:<run_id>`, via
`lockRun`) as its **first statement**, before any `FOR UPDATE` on a run-scoped
row. This gives the hot per-run write paths an identical, earlier serialization
point so the historical `{sessions, runs}` deadlock cannot form: the claim path
(`work.claim_next` / `work.await_packet`) locks sessions→runs→jobs, while the
verdict-completion path (`review.submit` / `review.verdict` → `maybe_complete_run`
→ close-remaining-sessions), the lifecycle paths (`work.complete`, `run.cancel`,
`run.retry_job`, `checkpoint.resolve`), and the per-run recovery sweep lock the
same rows in the opposite order. The lock is per `(repository_id, run_id)` — the
narrowest scope that serializes only the small, lane-bounded write concurrency
*within* one run; unrelated runs and repositories never serialize against each
other, and the claim queue's `FOR UPDATE … SKIP LOCKED` is exempt (already
deadlock-safe). The bounded deadlock-retry wrapper is retained only as
defense-in-depth; a surfaced `40P01` is now a should-never-happen signal. A
guard test asserts every per-run mutation handler takes `lockRun` before any
run-scoped `FOR UPDATE`.

## Sessions

Agents must call `register-session` before claiming work. Database identity is
an opaque `session_id`; human display uses `<role>-<lane>-<ordinal>` slugs.

Sessions match work by run, role, lane, and capabilities. Jobs can require
fresh sessions. Native sub-agents spawned inside an agent CLI inherit the
parent session unless explicitly registered as first-class sessions.

### Session lifecycle and closure

> Design rationale: [RFC 0011](../rfcs/0011-session-close-and-run-terminal-auto-close.md).

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
manual PostgreSQL editing, partial recovery). After auto-close it should
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
text. For constrained-operator escape hatches, `decision record` accepts
`--escape-surface` and `--escape-action`; both are required together and require
`--rationale`. Escape decisions add `escape_decision: true`, `escape_surface`,
and `escape_action` to the decision front matter and event payload, turning an
ambient escape into a durable audited run-level decision.

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
published repo-relative path; otherwise clean `process_executions` rows from
mediated `process.run` or older wrappers can satisfy process evidence for
wrappers that have not yet reported path-specific observations. The
operator-only `--allow-no-process-execution
--override-rationale` path records both a provenance event and the artifact's
`attestation_override_rationale`.

`complete` and review `verdict` commands verify all required artifacts before
terminal job transition.

`submit-review` composes the common review path: it publishes the review
artifact, records the verdict, applies review-gate behavior, and returns the
artifact, verdict, blocker, run, and downstream state. It is for the normal
case where the reviewer is publishing the finding artifact during this call. If
a re-claimed review job already has its required finding artifact published for
the current attempt, use `verdict` / `review.verdict` instead; it records the
verdict against the existing artifact and avoids re-publishing an immutable
logical name.

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
block remain accepted as before, except `collaboration_ledger` where the
front-matter block is required because the verdict gate reads structured
metadata. The publisher never rewrites artifact files.

Front-matter values are parsed as YAML. The older JSON-compatible scalar and
list style (`key: "value"`, `flag: true`, `items: ["a", "b"]`) remains valid,
and schemas may now opt into nested mappings or list-of-mapping rows where the
contract declares them.

**Standard optional metadata (RFC 0100).** Any artifact kind tolerates a common
set of byline/workflow metadata keys without rejection, so a lane can keep the
natural front matter its template produces instead of reverse-engineering each
kind's schema: `author`, `workflow`, `phase`, `lane`, `role`, `model`, `date`,
`created_at`, `updated_at`, `visibility`, `title`, `status`, `tags`, `summary`,
`related`, `run_id`, `session_id`, `ordinal`, `cycle`. These are accepted
free-form (not value-checked) unless a specific kind gives one of them a
required, checked meaning (e.g. `decision.title`, `decision.run_id`). A field
that is neither in the kind's schema nor in this standard set is still rejected,
but the rejection now names the kind's required keys, optional keys, and the
standard-metadata set — no source reading required.

V1 schemas:

- `striatum.decision.v1` (kind `decision`): required `schema_version`,
  `artifact_kind: decision`, `decision_id`, `run_id`, `owner: human`,
  `outcome` (one of `accepted`, `rejected`, `accepted_with_follow_up`),
  `follow_up_required` (boolean), `title`, `created_at`; optional
  `escape_decision` (boolean), `escape_surface`, and `escape_action` for
  constrained-operator escape decisions.
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
- `striatum.operator_brief.v1` (kind `operator_brief`): required
  `schema_version`, `artifact_kind: operator_brief`, `brief_id`,
  `supersedes` (string or null), `scope_links` (list of repo-relative
  strings), `context_budget_lines` (non-negative integer),
  `retrieval_priority` (one of `low`, `normal`, `high`), and `status`
  (one of `current`, `superseded`); optional `author`.
- `striatum.work_plan.v1` (kind `work_plan`): required `schema_version`,
  `artifact_kind: work_plan`, `plan_id`, `scope_kind` (one of `rfc`, `phase`,
  `initiative`, `bugfix`), `scope_ref`, `state` (one of
  `open`, `in_progress`, `closed`), `opened_at`, `closed_at` (string or
  null), `closure_summary` (string or null), `supersedes` (string or null),
  and `retrieval_priority`; optional `author`.
- `striatum.progress_note.v1` (kind `progress_note`): required
  `schema_version`, `artifact_kind: progress_note`, `note_date`,
  `session_slug`, `related_plan` (string or null), `related_brief` (string or
  null), and `retrieval_priority`; optional `author`.
- `striatum.operator_report.v1` (kind `operator_report`): required
  `schema_version` and `artifact_kind: operator_report`; optional `author`,
  `retrieval_priority`, and `supersedes` (string or null).
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
  new live blocker by itself. Live `work.block` mutations validate the
  blocker request shape and persist `striatum.blocker_payload.v1` metadata
  under `blockers.payload_json`; escalation-class blockers mirror the same
  payload into `escalation_inbox.payload_json`.
- `striatum.commit_request.v1` (kind `commit_request`, RFC 0067 / D127):
  required `schema_version`, `artifact_kind: commit_request`, `request_id`,
  `base_head`, `branch`, `git_snapshot_hash`, `included_paths` (non-empty
  list of strings), `commit_message`, `rationale`, and
  `confirmation_status` (one of `pending`, `operator_confirmed`,
  `human_confirmed`, `refused`);
  optional `run_id`, `reviewed_artifacts`, `confirmed_by`, and
  `confirmed_at`. Daemon `git.commit_apply` accepts only explicitly
  confirmed local commit requests and never pushes.
- `striatum.pr_request.v1` (kind `pr_request`, RFC 0067 / D127): required
  `schema_version`, `artifact_kind: pr_request`, `request_id`,
  `target_branch`, `summary`, `body_draft`, and `confirmation_status` (one of
  `pending`, `human_confirmed`, `refused`); optional `run_id`,
  `related_commit_request`, `local_commit_sha`, `provider_target`,
  `confirmed_by`, and `confirmed_at`. Core Striatum records the request
  artifact only; hosted provider actions remain out of core.
- `striatum.auto_finalize_gate_evidence.v1` (kind
  `auto_finalize_gate_evidence`, D125): required `schema_version`,
  `artifact_kind: auto_finalize_gate_evidence`, `decision_id: D125`,
  `gate_status` (one of `pending`, `satisfied`), `live_success_count`,
  `lane_shape_count`, `lane_shapes`, `contested_audit_chain_events`,
  `evidence_artifacts`, and `created_at`. A `satisfied` artifact must record
  at least three live successes, at least two lane shapes, and zero contested
  audit-chain events. This artifact records the default-on evidence gate; it
  did not itself make live auto-finalize the global default. D133 is the
  separate decision that flips default-live allowance after the gate is
  satisfied.
- `striatum.collaboration_ledger.v1` / `striatum.collaboration_ledger.v1.1`
  (kind `collaboration_ledger`, RFC 0093 / RFC 0098): required
  `schema_version`, `artifact_kind: collaboration_ledger`, `shape` (one of
  `falsification_gate`, `cross_examination`, `fog_of_war_review`,
  `synaptic_prune`, `adjudicated_constraint_extraction`), `topic`,
  `participants` (non-empty list), `entries` (list of objects with `kind`,
  `by`, `refs`, and `text`), `verdict` (one of `accept`,
  `accept_with_findings`, `needs_revision`, `reject`), and `rationale`. Entry
  kinds are `claim`, `challenge`, `rebuttal`, `constraint`, or `nomination`;
  `by` must name a participant; every ref must be a `dialogue:<sequence>` turn
  reference. Clearing verdicts must include at least one referenced `claim`,
  `challenge`, and `rebuttal`. `review.submit` rejects a
  `collaboration_ledger` artifact when the submitted verdict differs from the
  ledger front-matter verdict.
  `v1.1` is additive. Optional fields are `cycle` (non-negative integer),
  `findings[]`, `constraints[]`, and `branches{}`. `shape:
  adjudicated_constraint_extraction` requires `schema_version:
  striatum.collaboration_ledger.v1.1`; if that shape publishes `verdict:
  needs_revision`, `constraints[]` must include at least one productive row
  (`binding: true` or `kind: unresolved_question`). The refined RFC 0098
  states `blocked_pending_answer` and `defer_with_successor` are valid
  `branches{}` dispositions, not valid `verdict` values.

  `findings[]` rows require `id`, `severity` (`low`, `medium`, `high`,
  `critical`), `posture` (non-empty string), `status` (`open`, `answered`,
  `accepted`, `rejected`, `converted_to_constraint`, `deferred_with_owner`),
  and `challenge`. Optional keys are `closest_acceptable_answer`,
  `affected_invariants`, `requested_constraint_shape`,
  `requires_convener_rebuttal`, and `source_refs`.

  `constraints[]` rows require `id`, `posture` (non-empty string), `severity`,
  `kind` (`invariant`, `gate`, `schema`, `policy`, `non_goal`,
  `accepted_risk`, `unresolved_question`), `binding`, and `text`. Optional
  keys are `source_finding`, `source_refs`, `verification`, and
  `final_review_required`. When `binding: true`, `source_finding` must resolve
  to a same-ledger `findings[]` row with `severity: high` or
  `severity: critical`, and `verification` must contain a non-empty `gate` or
  `expected_stage`.

  `branches{}` is a map from posture string to one of `cleared`,
  `cleared_with_constraints`, `blocked`, `blocked_pending_answer`, or
  `defer_with_successor`.

  Example productive `needs_revision` ledger:

  ```yaml
  ---
  schema_version: "striatum.collaboration_ledger.v1.1"
  artifact_kind: "collaboration_ledger"
  shape: "adjudicated_constraint_extraction"
  topic: "RFC 0098 build"
  participants: ["sess_builder", "sess_reviewer"]
  entries:
    - kind: claim
      by: sess_builder
      refs: ["dialogue:1"]
      text: "The contract is implemented."
    - kind: challenge
      by: sess_reviewer
      refs: ["dialogue:2"]
      text: "The gate lacks a regression test."
  cycle: 1
  verdict: "needs_revision"
  rationale: "The revision needs a concrete gate."
  findings:
    - id: F1
      severity: high
      posture: implementation
      status: converted_to_constraint
      challenge: "The gate lacks a regression test."
  constraints:
    - id: C1
      source_finding: F1
      posture: implementation
      severity: high
      kind: gate
      binding: true
      text: "Add a regression test for naked needs_revision ledgers."
      verification:
        gate: "go -C go test ./pkg/artifactcontracts/..."
    - id: Q1
      posture: product
      severity: medium
      kind: unresolved_question
      binding: false
      text: "Confirm whether this shape should reopen a revision cycle."
  branches:
    implementation: blocked_pending_answer
    product: defer_with_successor
  ---
  ```

  Example clearing ledger:

  ```yaml
  ---
  schema_version: "striatum.collaboration_ledger.v1.1"
  artifact_kind: "collaboration_ledger"
  shape: "adjudicated_constraint_extraction"
  topic: "RFC 0098 build"
  participants: ["sess_builder", "sess_reviewer"]
  entries:
    - kind: claim
      by: sess_builder
      refs: ["dialogue:1"]
      text: "The contract is implemented."
    - kind: challenge
      by: sess_reviewer
      refs: ["dialogue:2"]
      text: "The gate lacks a regression test."
    - kind: rebuttal
      by: sess_builder
      refs: ["dialogue:3"]
      text: "The regression test now covers publish and review paths."
  cycle: 1
  verdict: "accept_with_findings"
  rationale: "The challenge was answered on the record."
  branches:
    implementation: cleared_with_constraints
  ---
  ```

Other artifact kinds (`prompt`, `marker`, `handoff`, `patch_summary`,
`test_report`, `other`) remain unschemaed in V1 and pass through without a
front-matter check.

Artifact kinds are validated by runtime contract code rather than by SQL
`CHECK`. Migration version 5 dropped the
`CHECK (artifact_kind IN (...))` clause from the `artifacts` table.
The Go runtime uses `go/pkg/artifactcontracts` as the shared allowed-kind
and front-matter schema package; the legacy Python contract module is
retired.

`publish-artifact` and workflow validation reject kinds outside the canonical
set.

## Corpus Export And Augmentation Boundary

`striatum corpus export --since <ref> --out <dir>` (RFC 0044 V1) emits a
redacted JSONL bundle of Striatum's durable provenance — RFCs, decision-log
rows, operator reports, run summaries, audit-chain entries, changelog
entries, ubiquitous-language terms, harness-friction patterns, and recent
commits — plus a verifying `manifest.json` with per-file row counts, SHA-256
hashes, explicit `state_authority` metadata for the daemon/PostgreSQL
authority, and a derived `bundle_sha256`. The bundle is read-only durable
provenance, not live state, and re-running the export over unchanged inputs
produces byte-identical JSONL with stable hashes (`generated_at` is the only
allowed timestamp variation and is excluded from the bundle digest).

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

These invariants are pinned by the active V2 boundary guardrail
`tests/test_corpus_verify.py::test_corpus_v2_surface_keeps_augmentation_boundary_local`.
The contract version, multi-corpus identity, redaction-tier metadata,
incremental-export watermark, and optional context-injection policy that
power V2 are scoped by [RFC 0057](../rfcs/0057-corpus-contract-v2.md).

`striatum archive create --run-id <id> --out <dir>` is the Phase 11 run
archive foundation. It is a daemon/Postgres-backed read command that writes
a local archive directory for one run: run row, workflow snapshot,
run-scoped rows including command requests, process executions, job
worktrees, process supervisors, process supervisor pointers, artifact
metadata, event metadata, and a self-verifying `manifest.json`. New archives
advertise `archive_contract_version=2`, `verification_depth=deep_chain`,
hybrid archive defaults (`snapshot`, `event_log`, and
`verify_replay_by_default` all true), and `artifact_content_policy:
metadata_only`. The archive does not copy artifact contents, transcripts,
`.striatum/` scratch, or any external-service state.

`striatum archive verify --bundle <dir>` checks an existing archive locally
without daemon state and runs offline deep-chain semantic replay by default:
run/repository consistency, FK-style references among run rows,
duplicate/missing archived row-id rejection for command requests, process
supervisors, process supervisor pointers, verdicts, blockers, process
executions, and job worktrees, monotonic event ordering, event-row hash
recomputation, and event-chain continuity when `previous_hash` / `row_hash`
anchors are present. `--manifest-only` is the explicit fast path that skips
semantic replay and checks only manifest shape, file hashes, byte counts, and
row counts. Because the archive stores artifact metadata rather than artifact
bytes, artifact content hashes are checked only when the operator also
provides `--repo-root <path>`. `striatum archive inspect --bundle <dir>` is
a read-only local semantic inspection surface over the same verifier; it
reports consistency, replay posture, and privacy-relevant metadata without
daemon reachability or workflow-state mutation.

## Branches And Commits

Workflow startup is gated by the workflow's `branch.mode` setting.

`branch.mode` is a closed enum: `"auto"` (the default when omitted) or
`"confirm"`.

**Auto mode (default).** When the workflow declares `branch.mode: "auto"`
or omits the `mode` field, `run prepare` atomically:

1. Validates and snapshots workflow JSON.
2. Ensures the local branch ref `<suggested_name>` exists at the run's recorded
   base commit using `git branch`, without checking it out.
3. Records the branch and transitions the run to state `ready`.

The response includes `branch_mode: "auto"`, the resolved `branch`,
`branch_created` (true only when a new branch was created), and the
`current_git_branch` for cross-check. If git ref creation fails (conflicting
branch name, invalid base, invalid branch name), the run remains in
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

No job is claimable before branch confirmation. Branch confirmation itself
does not commit, push, merge, or rebase.

`branch confirm --json` includes the requested branch and detected current git
branch, warns when they differ, and reports whether the confirmation was
`records_only`. The default confirm path ensures the branch ref exists at the
run's recorded base using `git branch` without checking it out; `records_only`
is `false` only when that call creates a missing ref. Three flags adjust the
confirmation gate:

- `--create`: explicitly run the same ref-only create-if-absent operation. If
  git refuses, the runner exits with `WorkflowError` (code 8) and does NOT
  record the confirmation. The response field `created` is `true` only when a
  new branch ref was created.
- `--use-current`: ignore `--branch` as a target and record the current git
  branch instead. If `--branch` is also given and disagrees with the
  current branch, exit with code 8.
- `--strict`: require that the current git branch matches `--branch`
  exactly before recording. If they differ, exit with code 8 and do not
  record. This is the safe default for CI and other automation.

The response also includes a `mode` field
(`"records_only" | "create" | "use_current" | "strict"`). The default
mode name preserves backwards compatibility for existing callers, but the
`records_only` boolean is authoritative.

`git snapshot --json` is a daemon read-only local Git projection for the
registered target repository. It reports branch, HEAD metadata, dirty counts,
changed paths, and bounded ancestry without fetching, pushing, committing,
reading remote URLs, importing hosted-provider SDKs, or including diff hunks
or commit bodies.

`git commit-apply <commit-request-path> --confirm --confirm-request-id <id>
--json` is the only core local commit creation surface. It is daemon-routed,
requires `apply` capability, consumes a repository-relative
`striatum.commit_request.v1` artifact, and refuses unless the artifact's
`confirmation_status` is `operator_confirmed` or `human_confirmed` and the
CLI confirmation id matches the artifact `request_id`. Before creating a
commit, the daemon verifies that current HEAD equals the artifact `base_head`,
the current branch equals the artifact `branch`, every dirty path is within
the artifact `included_paths`, and all included paths stay inside the
repository and outside `.striatum/`. The commit is local only, uses the
artifact `commit_message`, stages only `included_paths`, disables repository
Git hooks for the commit invocation, and returns the local commit SHA. It
does not push, fetch, open or update PRs, call hosted providers, import
provider SDKs, or load provider credentials.

## CLI

Required commands, grouped by concern:

```text
# Core lifecycle
striatum repo add
striatum workflow validate
striatum workflow generate
striatum workflow templates
striatum run prepare
striatum branch confirm
striatum run start
striatum run drive
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
striatum archive verify --bundle <dir> [--manifest-only] [--repo-root <path>]
striatum archive inspect --bundle <dir> [--repo-root <path>]

# Adapter
striatum adapter run
```

Human read commands can pretty-print. `--json` returns stable machine-readable
JSON. Mutation commands support JSON output for agent use.

`striatum run drive --run-id <id> [--interval 15s] [--once] [--json]` is a
CLI-local operator loop over existing daemon RPC methods. It re-reads run
detail and sessions, registers/supervises fresh role-lane sessions as queued
jobs unblock, adopts already-active matching sessions, and stops terminal or
superseded launched lanes before fresh-reviewer registration. It is
not a new daemon RPC method and does not use recovery, retry, override, or
force-non-fresh verbs.

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
`recovery.auto_finalize` unless the workflow explicitly opts out with
`recovery.auto_finalize.enabled=false`; live sweep mode never supplies
the standalone auto-finalize `--force` override. It then runs lazy lease
expiry, optional process reconciliation, optional autonomous review-only requeue
(D036-safe), human-checkpoint timeout escalation, and eligible-blocker
doctor flagging — and returns a structured envelope `{run_id, swept_at,
run_state, policy_source, dry_run, actions, escalations, still_stuck}`. Workflows
declare a `recovery_policy` block to opt into autonomous behavior.
Timed-out human checkpoints run the configured escalation hook only in live
sweeps; dry-runs report hook eligibility without side effects, and hook
failures are folded into `escalations[]`. Escalation is represented by
daemon state plus blocker/escalation artifact projections; any local
notification hook is non-authoritative and must never be treated as workflow
state. CLI flags
(`--autonomous-review-requeue`, `--autonomous-process-reconcile`,
`--max-requeue`, `--checkpoint-timeout`, `--eligible-after`,
`--dry-run`) override workflow defaults. Workflows that omit
`recovery_policy` get diagnostic-only output; today's flow is
preserved as closely as the daemon PG substrate allows.

The Go production-daemon port also runs a resident active-run recovery
scheduler over the same `recovery.sweep` behavior. That scheduler updates
daemon-owned PostgreSQL scheduler cursors.

`recovery auto-publish --run-id <id> [--dry-run]` emits the explicit
`recovery.auto_publish_stale_artifacts` daemon method. It is the
stale-lease auto-publish path for declared on-disk expected artifacts.
The deprecated `recovery.auto` daemon method remains only as a
compatibility alias for older clients and is not emitted by the current
CLI.

`recovery watch --run-id <id>` (RFC 0020 step 3) is the long-lived
counterpart for operators who want one foreground command instead of
a cron entry. In production it is not a daemon RPC method; the CLI
keeps the sleep loop, pidfile
(`.striatum/scratch/recovery-watch-<run_id>.pid`), `SIGTERM` /
`SIGINT` graceful shutdown, JSONL stream, and final `watch_exit`
envelope in the foreground process while each iteration calls daemon
RPC `recovery.sweep`. The sweep envelope's `run_state` drives the
exit-on-terminal default (`--no-exit-on-terminal` keeps looping), and
`--max-sweeps N` caps tests / probes. The same CLI overrides as
`recovery auto` are accepted. A pidfile collision with an alive watcher
exits 4 with `another recovery watch is active (pid <N>)`; stale
pidfiles (dead PIDs) are overwritten cleanly.

### Self-Contained Agent Skills

> Design rationale: [RFC 0015](../rfcs/0015-self-contained-agent-skills.md).


`striatum skills install [--profile {claude_code, codex, agy,
generic, all}] [--scope {project, user}] [--namespace <prefix>]
[--force] [--dry-run]` writes a self-contained agent skill bundle
into the target tree. The bundle teaches a Striatum-aware agent how
to drive the runner without reading the source repo: each rendered
Markdown file lists the relevant daemon MCP methods and CLI compatibility
verbs, the boundary conditions the runner does not enforce (no direct PostgreSQL writes,
no marker files as state, no durable transcript provenance), and copy-pasteable method
examples.

V1.2 ships four profiles plus an `all` fan-out:

- `claude_code` writes one SKILL.md per skill under
  `.claude/skills/<namespace>striatum-*/SKILL.md`. The five skills are
  `striatum-workflow` (router), `striatum-scaffold` (repo add /
  workflow generate / run prepare / run start / branch confirm), `striatum-claim-loop`
  (register-session / claim-next / ack / heartbeat / publish-artifact /
  verdict / submit-review / complete / worktree create / release),
  `striatum-supervise` (RFC 0009 supervisor lifecycle), and
  `striatum-recover` (status / why / doctor / recovery / checkpoint /
  dashboard).
- `codex` writes the same five-skill content as flat files at
  `.codex/agents/<namespace>striatum-*.md`, reusing the Claude Code
  skill bodies verbatim. Manifest at
  `.codex/agents/<namespace>striatum-workflow.manifest.json`.
- `agy` writes one SKILL.md per skill under
  `.agy/skills/<namespace>striatum-*/SKILL.md`, reusing the `claude_code`
  skill bundle shape and structure via standard imports (agy plugin import claude),
  eliminating the need for a separate profile template tree.
- `generic` writes a single concatenated guide at
  `<namespace>STRIATUM_AGENT_GUIDE.md` for any agent CLI without a
  skill-discovery convention.
- `all` fans out across the four real profiles in deterministic
  order (`claude_code, codex, agy, generic`) and returns a
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
`--dry-run` prints the plan without writing. When a requested install
finds an older Striatum-generated manifest for the same profile and
namespace in the other known scope (`project` vs `user`), it removes
that peer-scope generated pack if manifest hashes still match; with
local operator edits it refuses unless `--force` is supplied. This keeps
project and user scopes from exposing two generated Striatum generations
to the same agent.

Repository registration and skill installation are separate commands:
run `striatum repo add <path> --init` first, then
`striatum --repo <path> skills install --profile <profile>`. There is
no combined `init --with-skills` surface in the current Go CLI.

`striatum doctor` checks every installed manifest across all four
profiles in the registered target repository and user scopes:
`skills_missing` (a recorded file is absent on disk) and
`skills_outdated` (the manifest's runner version is older than the
running install, or a packaged template's bundled SHA256 differs from
the recorded `template_sha256`). It also scans loose generated
`striatum-*` skill headers and warns when their `Generated by striatum`
version is more than one minor release behind. Findings surface a
`recovery_command` with the exact `striatum skills install` invocation
that would clear the condition; the runner never auto-regenerates during
doctor.

The bundle is self-contained by construction: the renderer rejects
external URLs in template output (a unit test enforces no `http://` /
`https://`), templates ship inside the Go binary/release archive, and
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

`workflow generate --shape <shape> [--lane-set <set>] [--workflow-id
<id>] [--scaffold-root <path>] [--artifact-root <path>] [--option
key=value ...] [--write]` writes a starter workflow tree when
`--write` is supplied and otherwise previews the planned files. The
generated tree includes `<scaffold-root>/workflow.json` plus role and
prompt stubs and validates cleanly with `workflow validate` for the
supported catalog shapes. The command refuses to overwrite existing
files.

## Local API And MCP Boundary

The legacy local Python API is retired. Current production authority
remains the Go daemon RPC/MCP boundary; compatibility clients must
route through that boundary and must not become a second state
authority.

The production MCP surface is native to the Go `striatumd` daemon. It serves
loopback HTTP at `/mcp`, keeps `/mcp/sse` as an SSE/backcompat alias, publishes
its current endpoint in the daemon runtime directory as `mcp-http-endpoint`,
and supports `initialize`, `tools/list`, and `tools/call` over MCP JSON-RPC.
`tools/list` is derived from the daemon method registry and
capability-filtered per bearer token and repository scope. `tools/call`
dispatches through daemon RPC, so authorization, request logging, audit rows,
and method-denial vocabulary match the Unix-socket daemon path.

The legacy Python stdio MCP wrapper is completely retired. Tests and compatibility
harnesses must use the Go-based daemon MCP, the local web UI, or a documented daemon-backed
CLI fallback.

Operator-driven production runs use daemon MCP as the mandatory tool surface.
CLI use remains acceptable only when it is daemon-backed or when a documented
bootstrap/admin/debug exception is recorded by the operator. See `docs/MCP.md`
for the wire shape and tool list.

### Local Service

> Design rationale: [RFC 0012](../rfcs/0012-local-service-api.md).


`striatum serve` runs a `ThreadingHTTPServer` on TCP loopback (default
`127.0.0.1`) or a Unix-domain socket. Non-loopback hosts (`0.0.0.0`, public
IPs, hostnames that resolve outside loopback) are refused at startup with
exit 8 — the no-hosted-services boundary (D020) is preserved by
construction.

Endpoints return the same `{ok, data | error}` envelope used by Striatum's
CLI/API clients:

- `GET /v1/health` — `{started_at, version, mode}`. No DB hit.
- `POST /v1/invoke` — body `{argv: [...]}`; daemon-mapped production reads
  and mutations route through daemon RPC. Explicit CLI-local authoring and
  test/fixture surfaces may still use `api.invoke`. Returns 405 when the argv
  falls outside the read-verb whitelist and `--allow-mutations` is off.
- `GET /v1/runs` — daemon `status`.
- `GET /v1/runs/<id>` — daemon `status` scoped to the run.
- `GET /v1/runs/<id>/why?id=<entity>` — daemon `why`.
- `GET /v1/runs/<id>/dashboard` — daemon `dashboard` DTO.
- `GET /v1/runs/<id>/events` — Server-Sent Events stream over daemon
  `run.events` in production. Honors `?since=<event_id>` and
  `Last-Event-ID` for replay. Emits a `striatum.run_terminal` event and
  closes when the run reaches a terminal state.
- `GET /v1/doctor` — daemon `doctor` with verbose problem records.

The daemon-backed service endpoints above retain legacy CLI invoke fallback
only for subprocess compatibility tests (`STRIATUM_TEST_HARNESS=1` and
`STRIATUM_DAEMON_REQUIRED=0`); production service reads and mutations fail
closed when the daemon or repository registration is unavailable.
Production service startup also calls daemon `doctor` before binding, so a
missing daemon or unregistered repository is reported before the HTTP/Unix
socket listener starts.

Auth: Unix sockets bind `0o600` (filesystem permissions are the
boundary); HTTP loopback supports an optional `--token` validated by
length-safe constant-time compare. Single-instance enforcement via a
PID file; stale PID files are overwritten. Graceful shutdown on SIGTERM /
SIGINT. Mutations gate behind `--allow-mutations`; daemon-routed commands
are classified as read-only only when the daemon method contract has
`required_capability: "read"`. CLI-local workflow authoring reads
(`workflow validate`, `workflow lint`, `workflow plan`, `workflow graph`,
`workflow templates list/show`) remain explicitly allowed without mutation
mode; `workflow templates render-md` is a CLI file writer, not a local
service mutation endpoint.

### Registry-Backed Multi-Repo Coordination

> Design rationale: [RFC 0028](../rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md).

`striatum daemon start` (also exposed as the `striatumd` console
script) is the supported foreground entry point. Per D094 / RFC 0043
the daemon is a hard prerequisite for every Striatum CLI verb;
clients route through the daemon RPC envelope under token/capability
checks. The V1 `--no-daemon` direct-CLI path is retired and parsing
the flag returns the standard argparse "unrecognized arguments"
error. CLI verbs without a reachable daemon refuse with exit code
11 (`daemon_unreachable`); the stderr message names the socket path
and the platform-specific remediation, and no local database file is opened
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
tables in `.striatum/retired-local-state` is superseded by RFC 0043.

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

The RFC 0033 daemon-global V1 registry and RFC 0043 repo-local SQLite cutover
commands are retired operator surfaces. Legacy SQLite databases and their local-state
files are completely retired and not supported. RFC 0048 completed in v1.55.0:
production mapped verbs are daemon/Postgres-backed and fail closed without the daemon or
repository registration. The legacy local-state implementation and golden
fixtures are deleted.

Production daemon registry state is PostgreSQL-only and production dispatch
refuses retired registry fallback. Runtime files are overrideable with
`STRIATUM_DAEMON_RUNTIME_DIR`.
Linux uses XDG runtime locations; macOS uses Caches for runtime files. Windows
daemon support is not claimed in V1.

`striatum repo add <path> [--init]` registers a target repository.
It authorizes the daemon admin token before recording the repository in
daemon-owned Postgres. If `.striatum/` scratch is absent, registration
refuses unless `--init` is passed; `--init` creates only operational
scratch and does not create any SQLite or local database files. If a pre-D094
repo-local SQLite source exists, registration refuses and tells the operator
to archive/remove the legacy SQLite file before registering.
The command canonicalizes the repository root, refuses
symlink/path-traversal ambiguity including symlinked parent components,
derives a realpath/inode-based repository identity from the root, and
refuses active path re-occupation by a different identity. `repo remove` is
idempotent, marks the repository removed, revokes live repo-scoped
capabilities, preserves audit rows, and never reuses `repository_id`;
re-adding allocates a fresh id.

Every non-health registry-backed request requires a token. `striatum daemon
start` bootstraps one admin token when daemon-owned Postgres has no clients
and writes the local runtime `client-token` file with `0600` permissions.
Operators should treat runtime-file token storage as degraded compared with
an OS keyring. Plaintext token secrets are not read from environment
variables and are never stored in the registry or audit log. Authorization
uses the closed daemon method capability vocabulary:
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
reviewed-patch apply mutation is not part of the production daemon RPC
contract per D112; stale direct calls to `apply.reviewed_patch` return and
audit as `method_unknown`. Apply receipt read/verify routes remain as
evidence helpers, not cryptographic non-repudiation claims against a
malicious local operator. The Go daemon can rotate and load the local
Ed25519 `0600` fallback signing-key file through `daemon.key.rotate`; full
reviewed-patch mutation and stronger key custody are still separate
apply-gate work.

RFC 0032 extends the daemon V2 capability vocabulary to `read`, `write`,
`review`, `claim`, `apply`, `admin`, and `recovery`, and each registry
method declares a repository scope mode: `single_repo`, `cross_repo`, or
`daemon_global`. Daemon MCP mutation tools are derived from that method
registry when the PostgreSQL daemon substrate is active. `tools/list`
returns only the effective supported production tool set allowed by the
token's capability, repository scope, and production-support visibility
filter, while `tools/call` re-authorizes every request even if the tool was
listed earlier. Denials are metadata-audited with transport `mcp`; hidden
tools are not treated as authorized. There is no V2 daemon MCP equivalent of
`serve --allow-mutations`. Per D103, this daemon MCP surface is mandatory for
operator-driven workflow mutation, not an optional convenience wrapper.

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

Production supervision metadata, lane attestation inputs, and supervisor
pointers live in daemon-owned PostgreSQL under the registered repository
scope. `supervise.*` CLI calls are daemon clients, not direct repo-local
state edits. Legacy repo-local supervision shapes and one-shot wrappers are completely
retired. Supervision state and delivery are daemon-owned; supervised lane
processes default to the daemon OS user, or launch as the configured
`STRIATUM_LANE_OS_USER` through noninteractive sudo when the host adopts the
PG-less lane-user profile.

The Python daemon is no longer a selectable production core. RFC 0039
introduced `go/cmd/striatumd` behind the RFC 0030 envelope-v1 wire protocol
and RFC 0033 PostgreSQL substrate. RFC 0068 and D111 make that Go daemon the
only supported daemon implementation launched by `striatum daemon start`;
`--core go` is a deprecated no-op compatibility flag. The legacy `--core python` option is completely retired. Current Go handler
coverage has no missing or generic
`not_implemented` active contract methods. D110 deliberately removed the
SQLite-bound
`daemon.migrate_repo_local`, `dogfood.publish_on_behalf`, and
`dogfood.surgical_recovery` RPC names from production discovery and the daemon
method contract. D112 likewise removed `apply.reviewed_patch` from the
production daemon RPC contract; stale direct calls audit as `method_unknown`.
D113 closes the writable import window. The Python daemon module, Python MCP
wrapper, legacy local-state package, root compatibility facades, direct corpus
exporter, V1 local-state schema module, deterministic repo-local fixture, and
all Python CLI/web/source/test surfaces have been completely retired and deleted per RFC 0078.

### Multi-Principal Trust Model

> Design rationale: [RFC 0107](../rfcs/0107-multi-principal-trust-model.md)
> (multi-principal trust model); builds on [RFC 0028](../rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md)
> (per-repository scoping), [RFC 0053](../rfcs/0053-human-principal-and-terminology-truing.md)
> (human principal / operator roles), and [RFC 0096](../rfcs/0096-supervised-lane-trust-boundary.md)
> (session-bound capability tokens).

Striatum is **self-hosted on a laptop or a server, serves multiple
repositories, and may serve multiple human users — and is explicitly not a
hosted, multi-tenant SaaS control plane.** The multi-principal model makes
multi-user a deliberate, bounded design over the existing trust substrate,
not an emergent accident.

A **principal** is a named identity that holds capability tokens. A principal
has a `kind`:

- `human` — a human operator. This generalizes RFC 0053's single
  escalation-only human principal to several humans, each with their own
  identity, capability grants, and audit attribution.
- `ai_operator` — an autonomous operator identity that scaffolds and drives
  runs.
- `service` — a non-interactive automation/daemon client.

Principals sit **above** the existing `clients` / `client_capabilities`
substrate; the model adds no new wire RPC method and reuses the existing
`daemon.token.*` admin verbs:

- A principal **owns one or more clients**. A `client` (with its
  per-capability, per-repository, optionally session-bound grants in
  `client_capabilities`) is the wire credential. Because token rotation mints
  a **new** client row, the principal→client link is what keeps a principal's
  identity stable across rotation.
- The mapping is recorded in two daemon-global tables introduced by the RFC
  0107 migration: `striatumd.principals` (the identity) and
  `striatumd.principal_clients` (the active client links). A client is actively
  linked to at most one principal, so audit attribution is unambiguous.
  `principal_clients.client_id` is a bare column with no foreign key into the
  owner-held `clients` table — referential integrity is enforced in the daemon,
  exactly as `audit_log.client_id` already is — which keeps the migration
  ownership-safe (it only creates new tables; it never alters an owner table).
- `daemon.token.create` accepts an optional `principal_id` (with
  `principal_kind` + `principal_display_name` to create the principal on first
  use) and attributes the minted client to that principal in the same
  transaction that issues the token. `daemon.token.rotate` carries the
  rotated-from client's principal onto the replacement client, so rotation
  preserves a stable principal identity. `daemon.token.revoke` revokes the
  client's capabilities but leaves its historical principal link intact so past
  audit rows remain attributable.

**Capability + repository scoping.** Per-principal scoping reuses RFC 0028's
`client_capabilities.repository_id` and RFC 0096 session-binding unchanged:

- A principal may hold capabilities on a **subset of repositories**; the
  authorizer denies a token acting on a repository it was not granted with
  `capability_scope_mismatch`. No principal can read or mutate another repo's
  live state without an explicit grant.
- A session-bound token (minted for a single lane, held by exactly one
  principal) may act **only as its own session**; principal A's token cannot
  act for principal B's session. This is the existing `enforceSessionBinding`
  rule, stated canonically as `AuthContext.MayActAsSession`.

**Audit + attribution.** The daemon-global hash-chained audit log records the
acting `client_id` on every mutation (RFC 0030/0033); resolving that client to
its principal via `principal_clients` attributes every mutation to a principal.
Durable Markdown artifacts continue to carry the lowercase privacy-safe byline
`author: <role-name>-<model-name>-<ordinal>` (RFC 0026/0090); the principal
linkage is the control-plane attribution that complements that on-disk
provenance.

**Inspection.** `daemon doctor` surfaces a `principals` block listing each
configured principal with its kind, display name, active client count, granted
repositories, and effective capability scope (per capability, per repository,
and whether session-bound). The block never reads or returns token material.

**The boundary that keeps it non-SaaS.** Principals are **local trust grants
on the operator-owned daemon + PostgreSQL**, not cloud accounts. This model
introduces no hosted control plane, no tenant-provisioning service, no external
identity-provider or SSO dependency, and no telemetry. Loopback/tailnet access
(RFC 0085) is the network access model; principal identity is the local
capability-grant dimension over it. Adding any hosted/tenanted surface remains
out of scope and would require an explicit, separate product decision.

### Local Web UI

> Design rationale: [RFC 0013](../rfcs/0013-local-web-ui.md) (V1 surface
> + JSON API + SSE feed); [RFC 0022](../rfcs/0022-web-ui-redesign.md) (V1
> server-rendered redesign + SVG dependency graph).


`striatum serve --web` activates the bundled UI. The current UI is a
Go-served multi-page app from the RFC 0022 V1 line; the prior
hash-routed SPA is superseded. RFC 0078 has completed the Go web
service and route parity.

Routes:

- `GET /` → server-rendered `run_list.html`.
- `GET /run/<run_id>` → `run_detail.html`. Includes an inline
  SVG dependency graph (RFC 0022 step 3) — layered top-down
  layout, state-colored nodes via CSS custom properties, click
  to navigate to a job's detail page, SVG `<title>` tooltip on
  hover for accessibility. Production page-state reads use daemon
  `run.detail`; the web service keeps HTML/SVG rendering local.
- `GET /run/<run_id>/posture/<posture>` →
  `run_posture_verdicts.html`. Production reads use daemon
  `run.posture_verdicts`.
- `GET /run/<run_id>/job/<job_id>` → `job_detail.html`. Job
  metadata + verdict + posture chip + artifacts list. Production
  page-state reads use daemon `job.detail`; override-verdict context-token
  minting remains local to the web service.
- `GET /run/<run_id>/artifact/<artifact_id>` →
  `artifact_view.html`. Metadata + sha256 + raw-API pointer.
  Production metadata reads use daemon `artifact.show` with optional
  web context for run scoping, expected author line, and provenance trail
  rows.
- `GET /doctor` → `doctor.html`. Production page state comes from daemon
  `doctor` with verbose problem records.
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
process path. Single-shot stdout and stderr are suppressed unless the
operator explicitly requests inherited stdio; Striatum does not store
single-shot transcripts in daemon state or durable artifacts.
The process adapter graduates `network=forbidden` and
`repo_scope=local_only` to `advisory_strict`; transcript-off is `enforced`.

### Process Supervision

`adapter run` is single-shot: the child exits with the configured command,
and the next work packet must spawn a fresh process. Long-lived agent CLIs
(Codex, Claude Code, agy, etc.) need a different shape: one
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
  state, and forks the lane command in a daemon-owned persistent supervision
  session. When `STRIATUM_LANE_OS_USER` names a distinct OS user, the daemon
  launches the lane command and any tmux session as that user with
  `sudo -n -u <lane-user> -- env -i ...`; otherwise it preserves the same-user
  behavior. It keeps raw provider output out of daemon/PostgreSQL state and
  durable artifacts, and transitions the row to `attached` once the child pid
  is alive. A
  `supervisor.starting` and `supervisor.started` event are recorded.
  Agent-loop lanes record `agent_loop_mode: "self_driving"` and receive work
  through `work.await_packet`. Non-agent-loop supervised lanes record
  `agent_loop_mode: "supervised_push"`; after attach, `supervise start`
  attempts one atomic auto-dispatch that claims the next eligible packet for
  the session and delivers it to the supervisor FIFO, returning an
  `auto_dispatch` summary (`delivered`, `no_work`, or `failed`).
- `striatum supervise send --session-id <id> --packet-id <id>` delivers the
  work packet prompt directly to the daemon-owned interactive PTY master
  (`stdin-submit`) using the per-adapter submit key-sequence (Enter / `\r`),
  refreshes `heartbeat_at`, and records a `supervisor.packet_delivered` event.
  The helper-facing delivery FIFO is opened in nonblocking mode. If the helper
  or stdin reader is gone, delivery liveness is marked degraded with reason
  `stdin_reader_missing`, the call fails with a structured refusal, and no
  `supervisor.packet_delivered` event is recorded.
  Delivery degradation is stored under `tmux.delivery_liveness` for tmux-shaped
  supervisors and under top-level `delivery_liveness` for no-tmux/plain
  supervisor metadata; send guards and read projections must honor both shapes.
  A transient `tmux_unavailable` probe stores a degraded tmux liveness record
  and refuses delivery without marking the lane lost until the consecutive
  unavailable threshold is reached.
  Agent-loop lanes default to persistent PTY-helper supervision with preserved
  context. The older `supervision.stdin_delivery: "one_shot_eof"` named-pipe
  transport remains an explicit compatibility path for lanes that cannot yet
  run an agent loop. State reactions are daemon-driven through MCP/RPC methods
  (`artifact.publish`, `work.ack`, `work.complete`, `review.verdict`), with CLI
  compatibility fallbacks available for older workflows. The supervisor never
  parses agent stdout.
- PTY-helper lanes can set `supervision.require_tmux: true` to fail closed if
  `tmux` is unavailable or run/lane metadata is missing. Without that opt-in,
  PTY-helper launch may fall back to a plain PTY and records tmux
  unavailability as metadata. Agent-loop lanes default to the PTY-helper
  transport so they are tmux-backed and operator-attachable when tmux is
  available; a workflow may explicitly set `supervision.transport: "pipe"` for
  a lane that still requires legacy pipe delivery.
- Tmux-backed PTY-helper launches treat the tmux pane process as the supervised
  lane identity. Launch creates a placeholder pane, configures
  `remain-on-exit`, and then respawns the lane command so immediate startup
  exits retain a dead pane for diagnostics. Tmux setup and cleanup commands are
  bounded by a setup timeout. Tmux session names are bounded for tmux
  compatibility and include a stable hash suffix over the full run/lane/
  supervisor identity so truncation cannot collide across supervisors. The
  pointer metadata `tmux` block carries
  `state: "backed"`, `session_name`, `window_id`, `pane_id`, `pane_pid`,
  optional `pane_start_token`, `attach_command`, diagnostic
  `attach_client_pid`, `captured_at`, and probe visibility fields such as
  `last_ok_at`, `probe_skipped_at`, `probe_unavailable_count`,
  `last_unavailable_detail`, `liveness_state`, optional `run_as_user`, and
  `liveness`. Tmux liveness
  payloads include `state: healthy|degraded|lost`; failures also include a
  typed `probe_failure` record with `failure_class`, optional `exit_code`,
  optional `errno`, optional `pane_process_alive`, and optional
  `observed_pane_pid`. `tmux attach-session` is an observer and packet-delivery
  handle only; its PID is not the lane liveness identity.
- Helper-local teardown paths, including context cancellation and
  packet-forward failure, send `tmux kill-session -t <session>` for
  tmux-backed lanes before considering any direct pane-PID cleanup. A direct
  pane-PID signal is permitted only when the stored pane start token is numeric
  and still matches the current process; missing, literal, unavailable, or
  mismatched tokens skip direct signalling.
- `striatum supervise stop --session-id <id> --reason <text>` sends
  `tmux kill-session -t <session>` for tmux-backed lanes, then cleans up the
  helper process metadata without signalling the diagnostic attach-client pid.
  Any direct PID cleanup fallback, including helper PID cleanup and tmux
  unavailable pane cleanup, is gated by the recorded pid start token; stale,
  missing, or unverifiable tokens skip the signal and annotate the
  `supervisor.stopped` event. Plain PTY lanes keep the existing SIGTERM then
  SIGKILL path only when the recorded start token still matches. The row is
  marked `stopped` and `supervisor.stopped` is recorded.
- `striatum supervise rebridge --session-id <id>` re-attaches the
  helper-owned tmux delivery bridge in place for an attached tmux-backed
  supervisor. It first performs the same tmux session/pane liveness probe used
  by status and send. The command is valid only when the recorded pane process
  is still live; `tmux_pane_dead`, missing/mismatched pane identity, or
  unverifiable `tmux_unavailable` probe results fail closed with remediation
  guidance. Rebridge recreates the per-supervisor stdin FIFO when missing,
  starts a fresh helper attach path, updates helper/attach-client pointer
  metadata, clears delivery degradation, and records `supervisor.rebridged`.
  It does not kill, reset, or respawn the tmux pane.
- `striatum supervise status --session-id <id>` probes tmux-backed liveness
  with `tmux has-session` plus a pane identity query (`pane_id`, `pane_pid`,
  `pane_dead`, `pane_start_time`). Plain PTY rows continue to use PID/start-token
  liveness. Tmux failure classes are `tmux_session_missing`,
  `tmux_pane_missing`, `tmux_pane_dead`, `tmux_pane_pid_mismatch`, and
  `tmux_unavailable`; the successful class is `tmux_ok`. The status projection
  exposes these under `tmux.liveness` and never reads pane text. A tmux lane
  whose pane is live but whose start token cannot be verified remains
  operationally live but projects `lane_attestation: "unattested"` with
  `lane_attestation_reason: "start_token_unverified"`. Only numeric
  `pane_start_time`/start-token values count as verified identity evidence; if
  tmux cannot report a numeric live `pane_start_time`, the probe may compare
  the recorded numeric pane start token against the OS process start token for
  the observed pane PID. Literal tmux format strings or other non-numeric
  values are treated as unverified. An attached row whose
  lane is alive but whose active lease has stale progress
  returns `liveness: "stalled"` plus `last_progress_at`,
  `last_progress_age_seconds`, active lease metadata, and
  `stall_after_seconds`. Status also exposes `lane_backend`
  (`tmux`, `plain_pty_fallback`, or `plain_pty`), `delivery_state`,
  `pane_liveness`, `trajectory_log`, and failure-class-derived remediation
  hints. `trajectory_log` reports whether the operator-local PTY diagnostic log
  is expected, available, missing, or unreadable and gives its path/size when
  known. Status itself never starts or kills processes and never includes
  terminal log content.
- `striatum supervise list --run-id <id> [--state <state>]` lists rows
  for a run, optionally filtered by state; each row includes the same
  `lane_backend` and `trajectory_log` metadata.
- `striatum supervise trajectory --session-id <id> [--tail | --tail-lines N]`
  reads the latest supervisor's operator-local PTY diagnostic log from
  `.striatum/scratch/<supervisor_id>/pty.log`. Without a tail flag it returns
  the full local file; `--tail` returns the last 200 lines. This is an explicit
  operator diagnostic read, not a workflow-state projection.
- Daemon RPC `supervise.reattach_status` returns a read-only
  supervisor health DTO for a run/session/supervisor filter. It compares
  repo supervisor rows, daemon supervisor pointers, daemon supervisor
  rows, PID/tmux liveness, and PID or pane start-time identity, classifying each row
  as `reattachable`, `lost_candidate`, `needs_repair`,
  `needs_verification`, or `terminal`. It does not mutate state; actual
  restart/lost-state transitions remain daemon lifecycle work. In-place
  delivery repair is handled by `supervise.rebridge` and is deliberately
  limited to live tmux panes; terminal tmux liveness failures still require
  stopping the broken supervisor and starting/reclaiming a replacement lane
  through daemon workflow controls.

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
`progress`, `artifact_observed`, `helper_error`, `attach_client_exited`, and
`agent_exited`. `attach_client_exited` means the tmux attach observer exited
while the helper's pane probe still showed the lane alive or unverifiable. The
daemon treats that helper-reported liveness as advisory and performs its own
fresh tmux session/pane probe before deciding whether to keep the supervisor
attached or move it to `detached`. When the daemon-observed pane is live and the
attach observer was the helper-owned delivery bridge, the daemon keeps pane
liveness attached/attested, records `tmux.attach_client_last_exit`, and marks
`delivery_liveness` degraded with reason `attach_client_exited`; later
`supervise.send` calls refuse that supervisor until `supervise.rebridge`,
restart, or a future alternate delivery path clears the degradation. Non-tmux
or unproven attach exits still move the supervisor to `detached` instead of
treating the lane as lost.
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

1. **Persistent PTY Interactive Model.** All process lanes run natively in
   persistent interactive PTY sessions owned by the daemon. One-shot per-turn
   wrappers and `--print` configurations are completely retired.
2. **Submit Prompts via stdin-submit.** Per-turn prompt payloads are delivered
   directly to the PTY master of the long-lived process using the per-adapter
   submit key-sequence (Enter / `\r`), preserving agent context across turns.
3. **Call back through daemon MCP/RPC.** The agent advances workflow state by
   invoking `work.ack`, `work.heartbeat`, `artifact.publish`, `work.block`,
   `review.verdict` / `review.submit`, and `work.complete` with the
   identifiers from the packet. CLI commands for the same methods remain
   compatibility fallbacks. The agent's only durable output is the artifacts
   and verdicts it records through the daemon.

#### Operator-Local PTY Logs

Agent-loop PTY lanes may tee the provider terminal stream to an operator-local
diagnostic file under `.striatum/scratch/<supervisor_id>/pty.log` (created
`0600`). The path can be overridden with
`STRIATUM_AGENT_LOOP_DEBUG_LOG=<path>` or disabled with
`STRIATUM_AGENT_LOOP_DEBUG_LOG=off` / `/dev/null`.

`striatum supervise status` and `striatum doctor` report the log metadata
(`trajectory_log.status`, path, size, and whether an agent-loop log is
expected). `striatum supervise trajectory --session-id <id> [--tail |
--tail-lines N]` is the first-class read command for the file contents.

These PTY logs are operational scratch, not transcript provenance. They are
not stored in daemon-owned PostgreSQL, not published through
`publish-artifact`, not included in evidence exports, corpus exports, or run
archives, not parsed for workflow state, and not used for byline attestation,
verdicts, completion, or recovery decisions. They may contain provider output,
tool text, prompts, or secrets visible in the terminal, so operators must treat
them as private diagnostics and never commit them. Deleting them does not
change workflow truth.

A working supervised lane therefore needs an agent that knows the
Striatum protocol — a project skill, an embedded loop, or a wrapper
script — not just a raw model invocation. The shipped wrappers live at
`.striatum/bin/{claude,codex,agy}-supervised-wrapper.sh` (where `agy`
profile reuses `claude` template structures under `.agy/`). Each
wrapper maintains the long-lived process loop, logs provider stdout/stderr under
`STRIATUM_SCRATCH_DIR`, and traps `SIGTERM` to clean up the in-flight inner process.
Go tests under `go/pkg/agentloop/` verify those loop semantics
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
based on the run's confirmed branch and records a row in the
`job_worktrees` table with state `active`. If a pre-RFC 0117 run recorded a
confirmed branch name but the git ref is missing, worktree creation creates
the branch ref at the run's recorded base using ref plumbing; it never moves
the operator's primary checkout.

When `work.complete` completes a repo-write job with an active per-job
worktree, the daemon first makes that worktree HEAD reachable from a durable
git ref. It fast-forwards the run branch with compare-and-swap `update-ref`
when the run branch is an ancestor of the worktree HEAD; otherwise it pins
the worktree HEAD at `refs/striatum/<run_id>/<job_id>`. The completion emits
`job.commits_anchored` with the anchor kind.

`striatum worktree release --worktree-id <id>` refuses to remove a worktree
whose HEAD is not reachable from the run branch or a `refs/striatum/` pin,
returning `worktree_head_unreachable`. Passing `--force` performs the
discarding removal and emits `worktree.force_released`; otherwise a
reachable release runs `git worktree remove --force`, emits
`worktree.released`, and marks the row `removed`. Releasing an
already-terminal row is a no-op. `striatum worktree list [--run-id <id>]`
returns the rows plus each job's `workflow_job_id`.

`publish-artifact` continues to validate write scope and content against the
logical repo-relative path, but when an active worktree exists for the job it
reads the file from `<worktree_path>/<logical_path>` and records the
artifact's `repo_path` as the logical path. Artifacts remain durable
provenance for the main branch regardless of which worktree the work
happened in.

For repo-write jobs whose lane declares `worktree_isolation: per_job`, the
repository-touching surfaces `artifact.publish`, `repo.write`,
`repo.patch-preview`, `repo.patch-apply`, `process.run`, and `work.complete`
require an active job worktree and refuse with `worktree_required` if the
agent has not called `worktree.create`.

Lazy lease expiry preserves the worktree directory for operator inspection.
Before the `job_worktrees` row is marked `abandoned`, recovery anchors the
worktree HEAD through the same fast-forward-or-pin helper and records the
anchor in `worktree.abandoned`; `git worktree remove` is not run.
`striatum doctor` flags active worktrees whose lease is no longer active and
active worktrees whose path no longer exists on disk.

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

The contributor smoke sequence is script-owned:

```bash
scripts/package_smoke.sh
scripts/fresh_clone_smoke.sh
```

Both scripts use daemon-owned PostgreSQL and the Go daemon when PostgreSQL
setup is available; if it is unavailable they skip with a clear message
instead of entering a retired local-state fallback.
