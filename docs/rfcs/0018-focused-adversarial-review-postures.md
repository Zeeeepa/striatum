# RFC 0018: Focused Adversarial Review Postures

Status: accepted (V1+step 3)
Date: 2026-05-08
Context:
RFC 0002 (reviewer independence policy, accepted),
RFC 0004 (critique-to-action loop, accepted),
RFC 0005 (harness meta-optimization, accepted),
`docs/SPEC.md` § "Reviewer Policy",
`src/striatum/workflow.py` (`_validate_review_job_fields`,
the `verdict` enum {accept, accept_with_findings,
needs_revision, reject}),
`src/striatum/db.py` (`build_packet`, the `review_policy`
block on work packets),
`examples/rfc-ledger-cleanup/roles/reviewer.md` (an
adversarial-prompt role today)

## Problem

Striatum reviewers today are *neutral by construction*. The runner
treats every review job identically — a session claims a `review`
job, reads the artifact, returns one of four verdicts (`accept`,
`accept_with_findings`, `needs_revision`, `reject`), and that's
the entire vocabulary. RFC 0002 added access scope and context
policy fields, but did not introduce any concept of *what kind of
review* the reviewer is performing.

In practice, workflow authors paper over this by writing
adversarial-style prompts in `roles/reviewer.md` files (the
striatum source repo's own
`examples/rfc-ledger-cleanup/roles/reviewer.md` says "reviews
artifacts adversarially"). That works as far as it goes, but it
has three failure modes:

1. **Indistinguishable verdicts.** A neutral reviewer's `accept`
   and a red-team reviewer's `accept` are recorded identically. An
   operator reading evidence cannot tell whether an artifact passed
   one happy-path read or survived a hostile probe.
2. **No coverage signal.** Workflows that *want* adversarial
   coverage have no way to declare "this build must be reviewed by
   at least one adversarial reviewer before acceptance." The
   workflow author can declare three review jobs, but if all three
   are claimed by sessions running with neutral prompts, the runner
   has no way to flag the gap.
3. **No structured findings about *what kind of weakness* was
   caught.** A finding artifact today is a free-form Markdown
   document. There is no way to ask "show me every review where the
   *security posture* reviewer raised concerns" or "show me the
   *latency posture* findings across the last ten runs."

The runner already has the right primitives in place: review jobs,
findings as a typed artifact kind (RFC 0003/0004), front-matter
schemas, and the `review_policy` block on work packets (RFC 0002).
What is missing is a *posture* dimension that travels with both the
review job declaration and the resulting finding artifact.

This is not just a security-review concern. The same machinery
serves devil's-advocate review, latency review, ergonomics review,
threat-model review, accessibility review, license-compliance
review, and dependency-supply-chain review. Each is a *focused
adversarial posture*: the reviewer is told to look for a specific
class of weakness, and the runner records that posture as
first-class metadata.

## Goals

- **Declare a review posture per review job.** Workflow authors
  add an optional `review_posture` field to `type: "review"` jobs.
  The runner exposes it on work packets and records it on the
  resulting finding artifact's front matter.
- **Catalog the V1 posture vocabulary.** A small closed set
  shipped in V1, extensible in V1.5: `neutral` (default; today's
  behavior), `devils_advocate`, `security`, `threat_model`,
  `latency_performance`, `ergonomics_dx`, `accessibility`,
  `compliance_license`, `supply_chain`. Authors who need an
  off-list posture pass `--posture custom:<name>` and the runner
  treats it as opaque.
- **Require posture coverage for acceptance.** Workflow authors
  add an optional `required_review_postures: ["security",
  "threat_model"]` to `type: "build"` jobs. The runner refuses to
  mark such a build job `completed` until at least one accepting
  review verdict exists for *each* required posture. (Exit code 4,
  same shape as today's "completed-review-without-accepting-verdict"
  refusal.)
- **Surface posture in introspection.** `striatum status`,
  `run summary`, `evidence export`, `run graph --format json`,
  the dashboard graph panel, and the web UI all surface posture
  alongside verdict so an operator reading evidence can answer
  "what kind of review caught this?" without reading prose.
- **Record posture-keyed verdict counts.** `verdicts` table
  schema gains a `posture` column; doctor / dashboard panels can
  group counts by posture.
- **Pair builds with adversarial reviewers cleanly.** A new
  workflow validator rule warns (lint, V1) and later errors (V2)
  when a `type: "build"` job has zero accompanying review jobs
  whose `review_posture` is anything other than `neutral`.
- **Zero regression for existing workflows.** A workflow that
  omits `review_posture` produces work packets and finding
  artifacts byte-identical to today. Posture is purely additive.

## Non-Goals

- Auto-generating adversarial prompts. Posture is *metadata*; the
  prompt body still lives in the role definition or task prompt
  (RFC 0010 harness-profile guidance applies). The runner does not
  ship boilerplate "be adversarial" text.
- Encoding *what* a posture should look for. `security` does not
  imply OWASP; `threat_model` does not imply STRIDE. The role
  prompt and finding-artifact body carry the substance; the
  runner records only the posture *name* and uses it for coverage
  rules.
- Cross-run statistical analysis ("posture X catches 40% more
  issues than posture Y"). That is a meta-analytics layer
  (RFC 0005-adjacent) and is out of scope here.
- Adversarial *generation* (red-team agents that try to make the
  build fail). Postures describe how a reviewer *reads* an
  artifact; they do not coordinate active probing.
- A separate "red-team" job type. Postures live on existing
  `review` jobs; we do not add a new `job_type`.
- Auto-routing by posture. The runner does not pick *which*
  reviewer session claims which posture; that's a workflow-author
  choice expressed through `role_id` + `lane_id` + the session's
  registration metadata.

## Proposal

V1 ships in three landable steps. Each can be its own PR.

### Step 1. `review_posture` field on review jobs

Workflow validator additions:

```python
# In _validate_review_job_fields:
ALLOWED_POSTURES = {
    "neutral", "devils_advocate", "security", "threat_model",
    "latency_performance", "ergonomics_dx", "accessibility",
    "compliance_license", "supply_chain",
}

posture = job.get("review_posture")
if posture is not None:
    if not isinstance(posture, str):
        raise ...
    if not (posture in ALLOWED_POSTURES or posture.startswith("custom:")):
        raise WorkflowError(...)
```

Non-review jobs that declare `review_posture` are rejected (mirrors
the existing `reviewer_access_scope` rule). Default behavior when
omitted: equivalent to `"neutral"`. Schema validation rejects an
empty string and the bare `"custom:"` prefix.

`build_packet` extends the `review_policy` block:

```json
{
  "review_policy": {
    "access_scope": "repo_level",
    "context_policy": "fresh",
    "posture": "security",
    "instruction": "...standard text..."
  }
}
```

`instruction` is augmented (when posture != neutral) with one
deterministic sentence per posture, e.g. for `security`:

> "This is a security-focused review. Read the artifact looking
> for security weaknesses; verdict acceptance means you actively
> looked and found nothing actionable."

The sentence table is in `striatum.workflow:POSTURE_INSTRUCTIONS`
(opaque to the runner; just rendered into the packet so the
reviewer prompt is self-contained without reading a remote doc).
For `custom:<name>` postures, no sentence is appended; the
workflow author owns the prompt body.

### Step 2. `required_review_postures` on build jobs and acceptance gating

Workflow validator additions on `type: "build"` jobs:

```json
{
  "id": "implement_v1",
  "type": "build",
  "required_review_postures": ["security", "threat_model"]
}
```

The validator rejects a `required_review_postures` entry whose
name is neither in `ALLOWED_POSTURES` nor a `custom:<name>`
form. It also rejects the field on non-build jobs.

Workflow-validation acceptance rule (re-cast from runtime gate
per V1_ACCEPTANCE / D069 — see "Implementation note" below):

- The workflow validator walks the directed edge graph in both
  directions from each build job declaring
  `required_review_postures`.
- For each required posture P, it requires at least one
  *reachable* `type: "review"` job (forward or reverse from the
  build) whose `review_posture == P`. An undeclared posture on a
  review job counts as `"neutral"` for this gate.
- Failure raises `WorkflowError` (exit code 8) at
  `striatum workflow validate` and `run prepare`, naming the
  build, the missing posture, and the postures available across
  reachable reviews.

Runtime enforcement is preserved by the existing edge-verdict
gate (a downstream-of-review job stays blocked until the review
accepts) and the existing run-completion semantics (a run cannot
terminate while jobs remain non-terminal). No new runtime gate
is added in V1; V1.5 may add a run-completion "blocked on
missing posture" surfacing if dogfood evidence shows operators
want explicit signaling.

**Implementation note.** The original RFC text described a
runtime build-completion gate. That gate is a deadlock against
striatum's lifecycle: a build's `complete` mutation precedes its
downstream review's verdict by construction, so requiring the
verdict to gate the build's completion is impossible. The
workflow-validation gate above delivers the same operator value
(catches mis-wired workflows whose declared review jobs cannot
collectively satisfy a build's required postures) at authoring
time, before any session claims work.

### Step 3. `posture` column on `verdicts` + introspection surfacing

Migration v10:

```sql
ALTER TABLE verdicts ADD COLUMN posture TEXT;
CREATE INDEX IF NOT EXISTS idx_verdicts_posture ON verdicts(posture);
```

When `submit-review` records a verdict, it copies the review job's
`review_posture` (or `"neutral"`) into the `verdicts.posture`
column. Existing rows are backfilled with `"neutral"` as part of
the migration.

Surfaces:

- `striatum status --run-id <id>` adds a `verdicts_by_posture`
  block alongside the existing `verdicts` counts.
- `striatum run summary` groups per-build verdicts by posture in
  the rendered Markdown.
- `striatum evidence export` includes posture in the redacted
  per-verdict block.
- `striatum run graph --run-id <id> --format json` adds
  `posture` to each review node.
- The dashboard's Verdicts panel renders a one-line per-posture
  count when any non-neutral posture exists in the run.
- The web UI's job detail view shows posture as a chip next to
  the verdict.
- A new doctor check `build_missing_required_posture_review`
  surfaces a build whose `required_review_postures` cannot be
  satisfied with the currently declared review jobs (workflow
  authoring help; fires at run start).

## Acceptance Criteria

- A workflow that omits `review_posture` and
  `required_review_postures` produces work packets and finding
  artifacts byte-identical to today.
- A workflow with `review_posture: "security"` on a review job
  surfaces it on the work packet's `review_policy.posture`.
- `submit-review` for that job records `verdicts.posture =
  "security"`.
- A build job with `required_review_postures: ["security"]`
  refuses `complete` with exit code 4 until a review with that
  posture has an accepting verdict.
- `striatum status --run-id <id>` reports
  `verdicts_by_posture` accurately.
- A workflow with a `custom:my_thing` posture validates,
  surfaces it on the packet, records it on the verdict, and
  participates in the `required_review_postures` gate
  identically to first-class postures.
- The doctor check `build_missing_required_posture_review` fires
  exactly when the workflow's declared review jobs cannot
  collectively satisfy a build's `required_review_postures`.
- Migration v10 backfills existing `verdicts.posture` rows with
  `"neutral"`; a database newer than the runner refuses with
  exit code 9 (existing migration policy).
- Tests at `tests/test_review_postures.py` cover: validator
  rejections (unknown posture, posture-on-non-review-job,
  required-posture-on-non-build-job, empty/bare-prefix invalid
  values), packet exposure, verdict recording, build-completion
  gate (refusal + accept paths), `custom:` postures, doctor
  check, status surfacing, run-summary surfacing, evidence
  export surfacing, migration idempotency, and a fixture
  workflow exercising all V1 postures end-to-end.

## Open Questions

- **Posture vocabulary growth.** V1 ships nine postures plus
  `custom:<name>`. If `custom:` becomes the dominant case in
  practice, V1.5 should graduate the most-used custom names
  into first-class entries with deterministic instruction
  sentences. We learn that from dogfood evidence.
- **Multi-posture review jobs.** A single review job today has
  one posture. A reviewer who is told to look for *both*
  security and threat-modeling concerns either splits into two
  review jobs (preferred — clean evidence per posture) or
  declares `review_posture: "custom:security_and_threat_model"`.
  V1 picks "split into two." If operators push back, V1.5 could
  accept `review_posture: ["security", "threat_model"]`.
- **Posture mismatch between role definition and field.** A
  workflow whose `roles/reviewer.md` says "be neutral" but
  declares `review_posture: "security"` is incoherent. The
  runner records the field; the prose is the workflow author's
  problem. A V1.5 follow-up could lint role-document text
  against the declared posture, but that's expensive and
  probably out of scope.
- **Acceptance under needs-revision cycles.** When a build
  re-runs (cycle), the runner already requires the *new*
  attempt's reviews to be accepting. The posture gate inherits
  this: each attempt independently needs each required posture
  to clear. This is the right default; no need to special-case.
- **Dashboard visual encoding.** Per-posture chips on the
  Verdicts panel work for ≤ 4 postures; a 9-posture run would
  overflow. V1 truncates to top-3 with "+N more" overflow; the
  full list is in the web UI.
- **Front-matter on findings.** Should a finding artifact's V1
  schema gain a required `posture` field? Probably yes, defaulted
  to `"neutral"` when not declared, validated against the same
  vocabulary. V1 of this RFC adds the field as **optional**
  (omission means the artifact's posture is whatever the
  associated verdict records). V1.5 could promote to required if
  evidence shows divergence.
- **Cross-RFC interaction with action-item ledgers (RFC 0004).**
  When a reviewer records action items, should each item carry
  the posture? Probably yes — same defaulting rule. A V1.5
  follow-up that touches the `action_item_ledger` schema can
  fold this in at low cost.

## Implementation Path

V1 ships in the three steps above, in order. Each step is its
own PR; together they are step 1+2+3 of this RFC.

1. **Step 1 (validator + packet exposure):** `workflow.py`
   adds `ALLOWED_POSTURES`, `_validate_review_posture`,
   `POSTURE_INSTRUCTIONS`. `db.py` extends the `review_policy`
   block. Tests for validation + packet exposure.
2. **Step 2 (acceptance gate):** `db.py:complete` refines the
   downstream-review check. `workflow.py` adds the
   `required_review_postures` field to build jobs and a doctor
   check (`introspect.py`). Tests for the gate.
3. **Step 3 (verdicts column + introspection):** Migration v10.
   `db.py:submit_review` writes `verdicts.posture`. `status`,
   `run summary`, `evidence export`, `run graph --format json`,
   dashboard, and web UI all surface the column. Tests for
   each surface.

RFC 0018 is "accepted" once steps 1+2 land. Step 3 is the
introspection completeness pass; it does not block acceptance
because the underlying data is already on the verdicts row by
the time it ships.

## Relationship To Other RFCs

- **RFC 0002 (reviewer independence):** posture is the third
  axis of review-job declaration alongside `reviewer_access_scope`
  and `reviewer_context_policy`. The three fields are independent;
  any combination is valid.
- **RFC 0003 (support ledgers):** support-ledger reviewers can
  carry a posture; an `evidence_audit`-conventioned review job
  with `review_posture: "compliance_license"` is a
  license-evidence audit.
- **RFC 0004 (critique-to-action):** action-item ledgers will
  inherit posture from their source review when the V1.5
  follow-up adds the field.
- **RFC 0005 (harness meta-optimization):** harness-improvement
  proposals can target a `posture` field as a valid
  `target.kind`, opening "this posture's reviews keep returning
  the same finding shape — maybe the prompt or the posture
  vocabulary needs revision."
- **RFC 0010 (harness profiles):** posture sits *above* the
  harness profile. A given posture may have tool-family-specific
  prompt guidance, but the runner only records the posture name;
  the prompt body remains the role document's responsibility.
- **RFC 0013 V1 / step 7 (web UI mutations):** the verdict
  modal in the mutation buttons should carry the posture chip
  so operators see the posture context when recording a verdict
  manually.
