---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["README.md", "docs/SPEC.md", "docs/DECISION_LOG.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/operator/BRIEF.md", "docs/rfcs/0064-review-diversity-enforcement.md", "docs/rfcs/0066-replay-archive-corpus-v2-foundations.md", "docs/rfcs/0057-corpus-contract-v2.md", "docs/operator/workflows/todo-55-56-59-60-decisions/prompts/analyze_55.md"]
---

author: analyst-codex-002

# TODO 55 Accepted-Risk Persistence Options

## Scope

TODO 55 is not asking whether workflow risk lint should exist. RFC 0064 has
already landed advisory lint, strict refusal with `--override-rationale`,
generator/web surfacing, default `workflow validate` refusal for same-model
review-pair and revision-cycle risks, and an optional
`--accepted-risk-decision-id` reference. The open product question is where
accepted lint-risk evidence should live durably.

The decision should preserve these current boundaries:

- Striatum's live state is daemon-owned PostgreSQL, scoped per registered
  target repository.
- Repository artifacts are durable provenance, not the live message bus.
- `workflow lint` is currently a CLI-local authoring helper: it reads a
  workflow file and returns structured findings, but it does not mutate
  daemon state or write artifacts.
- No hosted service, telemetry, transcript capture, external persistence, or
  optional memory consumer may become part of the accepted-risk authority.

## Candidate Authority Homes

### Option A: Decision Artifact Linkage Only

Accepted risks are represented by normal durable decision artifacts and
referenced by `--accepted-risk-decision-id`. Strict lint returns the decision
id in its result, but no new daemon table or workflow metadata is introduced.

Auditability is simple and local-first: the durable record is an existing
artifact kind and can be reviewed in git. The weakness is that the lint result
does not become a queryable daemon fact. A later operator can see that a
workflow was lint-overridden only if they have the decision id, preserved CLI
output, run notes, or a workflow/run artifact that cites it.

Implementation impact is smallest. CLI validation of decision id shape can
tighten, generator/web can surface "strict override requires decision id" copy,
and docs can define citation expectations. The daemon remains unchanged.

### Option B: Workflow Metadata

Accepted risks are stored in the workflow JSON, for example under a dedicated
`accepted_risks` block keyed by lint rule, finding fingerprint, decision id,
scope, and expiry/revisit notes.

This keeps the override next to the authored workflow and makes generator,
web editor, and `workflow validate` able to read accepted risk without daemon
state. It is attractive for reusable workflow templates: the risk acceptance
travels with the workflow file.

The tradeoff is authority confusion. Workflow files are authoring inputs, not
live state, and embedding accepted risk in them can make a risky template look
self-approved. It also creates drift problems when lint finding fingerprints
change after a workflow edit. If this option is chosen, each record should
still cite an accepted decision artifact, and persisted records should be
limited to explicit authoring-time risk waivers rather than run-specific
operator decisions.

Implementation impact is moderate: schema validation, generator/editor UI,
lint fingerprint stability, migration/upgrade behavior, and docs all need
work. The daemon need not mutate for file-only lint, but `run prepare` should
probably echo accepted risks into run provenance so a prepared run is auditable
after the workflow file changes.

### Option C: Run-Preparation Record

Accepted risks are recorded when a workflow is prepared or started. The record
belongs to a specific run/workflow snapshot and cites the decision artifact
used to accept strict lint warnings.

This aligns with Striatum's authority model better than mutating during
`workflow lint`: the daemon already owns run preparation, workflow snapshots,
and durable state transitions. The accepted-risk decision becomes tied to the
exact workflow snapshot that ran, not a mutable source file. It is local,
queryable, and exportable through archive/corpus surfaces without inventing a
new external authority.

The tradeoff is that authoring-time lint remains advisory until a run exists.
Operators could still run `workflow lint --strict` locally and see an override
result that is not durable unless `run prepare` records it. That is acceptable
if docs state the distinction clearly: lint can preview acceptance, but
durable accepted risk is created only at daemon-backed prepare/start.

Implementation impact is significant but bounded. CLI `run prepare` needs a
way to receive accepted-risk references or re-run strict lint against the
snapshot. The daemon needs a PostgreSQL table or snapshot-attached projection
for accepted risks, plus audit/events. Web and generator preview can keep
surfacing warnings and hand off to run preparation for durable acceptance.

### Option D: Dedicated Daemon Accepted-Risk Table

Accepted risks live in a first-class daemon PostgreSQL table keyed by
repository, workflow identity or run id, workflow snapshot hash, lint rule,
finding fingerprint, decision id, rationale hash or text policy, accepting
session/operator identity, and timestamp.

This is the most queryable option. Dashboards can list active accepted risks,
archive/replay can include them, and lint/run preparation can consult the same
authority. It also fits the current product direction: daemon-owned PostgreSQL
is the live authority, and durable provenance can remain a linked decision
artifact.

The risk is overbuilding and creating a new approval registry before deciding
whether accepted risk is per workflow source, per workflow snapshot, per run,
or per repository policy. It also makes it tempting for `workflow lint` to
become mutating. That would be a product-boundary change because `workflow
lint` is currently a local authoring/read helper.

Implementation impact is largest: daemon method contract, Go handler, CLI
route, MCP/UI parity, archive/corpus projection, authority guardrail tests,
and docs. If chosen, mutation should probably be a new daemon-backed command
such as `risk accept` or a `run prepare` sub-flow, not hidden inside lint.

### Option E: Daemon Audit Row Only

Accepted risks are not modeled as domain state. Instead, a strict override
call writes a daemon audit row containing method metadata and hashes.

This is weak as the primary authority. Audit rows prove a request happened;
they are metadata-only by design and deliberately avoid workflow prose and
artifact content. They are good supporting evidence, not the durable record a
future operator should consult to decide whether a same-model review risk is
still accepted.

Implementation impact is deceptively high if `workflow lint` remains
CLI-local: making it write audit rows means routing it through daemon RPC or
adding a new mutating surface. If paired with another option, audit rows are
valuable corroboration.

## Citation Policy For Strict Overrides

Strict lint overrides should cite an accepted decision artifact, not only an
inline rationale. A usable citation should include:

- the decision id, such as `D###`, or a durable artifact path if the decision
  has not been promoted into the decision log yet;
- the lint rule ids accepted, such as `same_model_review_pair` or
  `repo_write_without_worktree_isolation`;
- the workflow identity and, for run-bound acceptance, the workflow snapshot
  hash or run id;
- the scope: one lint invocation, one workflow version, one run, or a reusable
  template policy;
- revisit conditions, especially for same-model review-pair acceptance.

`--override-rationale` should remain an immediate human-readable explanation,
but it should not be the authority by itself. The durable citation should be
the decision artifact plus, if the chosen option includes daemon persistence,
the daemon record id.

## Cross-Surface Consequences

For the CLI, `workflow lint` should stay non-mutating unless the product
decision explicitly changes that surface. If persistence is daemon-backed,
prefer a separate mutating command or `run prepare` flow that records accepted
risk after re-running lint against the exact workflow snapshot.

For the generator, preview should continue to include lint warnings and
coverage scoring without creating accepted-risk records. Generated workflows
should not silently include waivers unless the operator supplied an explicit
decision citation in the generation spec or accepted the risk during run
preparation.

For the web UI, workflow browser/detail pages can keep showing advisory
warnings. A future accepted-risk UI should distinguish "warning exists",
"warning preview overridden", and "warning durably accepted for this
snapshot/run" so the operator does not mistake UI acknowledgement for durable
policy.

For daemon state, any durable accepted-risk record should be append-oriented,
repository-scoped, and tied to immutable evidence: workflow snapshot hash,
lint rule/finding fingerprint, decision id, accepting actor/session, and
timestamp. Do not store transcripts or external provider identifiers.

For archive/corpus, accepted-risk records are good local provenance. RFC 0066
and RFC 0057 caution against implying external persistence or memory
dependency. Export should remain operator-triggered and local; accepted-risk
records can be included only as redacted, versioned provenance once their
authority home is decided.

## Recommendation

The strongest product fit is a two-layer policy:

1. The durable human-readable authority is always a decision artifact.
2. The daemon records accepted risk at run preparation against the immutable
   workflow snapshot, citing that decision artifact.

This avoids making `workflow lint` a hidden mutating command, preserves the
local-first boundary, gives dashboards/archive/replay queryable evidence, and
keeps risk acceptance tied to what actually ran. It also leaves room for
workflow metadata later, but only as reusable authoring guidance that must
still cite a decision and be re-confirmed or snapshotted at run preparation.

The minimum accepted decision could therefore be: "Accepted lint risks are
durable only when recorded by daemon-backed run preparation against a workflow
snapshot; every record must cite a decision artifact; CLI-local lint overrides
remain previews and are not durable by themselves."
