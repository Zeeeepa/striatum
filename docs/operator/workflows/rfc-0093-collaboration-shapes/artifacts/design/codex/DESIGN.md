# RFC 0093 Implementation Design
date: 2026-05-29
status: draft

## Problem framing

RFC 0093 is not asking Striatum to invent another live-dialog engine. The
interrogation, conversation, preserved-context agent loop, dialogue trajectory,
and chat UI already exist. The missing product behavior is the structure around
those primitives: named generator shapes and a gate that refuses to treat
"the dialog finished" as proof that useful pressure happened.

The implementation should therefore keep the live-state boundary unchanged and
make the new behavior visible in three ordinary Striatum surfaces:

- a typed `collaboration_ledger` artifact contract that records curated
  challenge/rebuttal evidence;
- generated workflow shapes that compile to normal V1.1 jobs, phases, edges,
  cycles, roles, prompts, and existing `interrogation.*` / `conversation.*`
  method calls;
- a verdict-capable adjudicator job whose clearing verdict is the only path to
  the downstream commit/proposal job.

The load-bearing distinction is that the gate is not a conversation close event,
an interrogation close event, or a max-rounds counter. It is an adjudicator
verdict backed by structured ledger entries that cite the RFC 0081 `dialogue`
trajectory.

One current-code caveat is important: the Go validator accepts workflow cycles,
but `recordVerdict(..., "needs_revision", ...)` currently opens a human
checkpoint instead of requeueing the declared cycle. RFC 0093 cannot satisfy
"needs_revision routes back into a bounded dialog round" until that runtime
cycle router exists for verdict-capable jobs.

## Proposed approach

### 1. Add the ledger contract first

Add a new artifact kind `collaboration_ledger` in
`go/pkg/artifactcontracts/contracts.go`.

Use front matter as the machine contract, because the adjudicator's gate
evidence must be parseable at publish time:

```yaml
---
schema_version: "striatum.collaboration_ledger.v1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "short topic"
participants: ["sess_holder", "sess_falsifier", "sess_adjudicator"]
entries:
  - kind: "claim"
    by: "sess_holder"
    refs: ["dialogue:3"]
    text: "The proposal relies on the existing phase_synthesis gate."
  - kind: "challenge"
    by: "sess_falsifier"
    refs: ["dialogue:4"]
    text: "The current needs_revision path opens a checkpoint instead of cycling."
  - kind: "rebuttal"
    by: "sess_holder"
    refs: ["dialogue:5"]
    text: "The implementation will add a cycle router before shipping the shape."
verdict: "accept_with_findings"
rationale: "A material challenge landed and was answered with an implementation step."
---
```

Validation rules:

- `schema_version` equals `striatum.collaboration_ledger.v1`.
- `artifact_kind` equals `collaboration_ledger`.
- `shape` is one of the shipped collaboration ids for V1:
  `falsification_gate`, `cross_examination`, or `scribe`. Keep
  `fog_of_war_review` and `synaptic_prune` out of the V1 generator unless the
  run explicitly takes the deferral work.
- `topic` and `rationale` are non-empty strings.
- `participants` is a non-empty string list.
- `entries` is a non-empty list of maps with exactly `kind`, `by`, `refs`, and
  `text`.
- `entries[].kind` is one of `claim`, `challenge`, `rebuttal`, `constraint`,
  `nomination`.
- `entries[].by` must appear in `participants`.
- `entries[].refs` is a non-empty string list. V1 should accept a stable
  `dialogue:<seq>` reference shape derived from `trajectory.export --profile
  dialogue`. Artifact-contract validation can shape-check refs; it should not
  add a database dependency.
- `entries[].text` is curated authored text. The schema has no raw-output,
  transcript, stdout, stderr, PTY, or provider-payload fields. Unknown fields
  are rejected, which is the D028 guard the publisher can enforce structurally.
- `verdict` is one of `accept`, `accept_with_findings`, `needs_revision`,
  `reject`.

Make this schema require front matter. Existing schemas can keep their current
optional-front-matter compatibility, but a `collaboration_ledger` without the
block is not a ledger.

Add a kind-specific substance check:

- A clearing `falsification_gate` or `cross_examination` ledger must contain at
  least one `claim`, one `challenge`, and one `rebuttal`, each with refs.
- A `needs_revision` ledger may contain a challenge without a rebuttal.
- A `reject` ledger may contain unrebutted challenges or constraints.
- A `scribe` ledger clears only when it contains at least one `claim` or
  `constraint` entry and a non-empty rationale.

This does not pretend to prove the model's semantic judgment. It makes a hollow
dialog impossible to clear without the adjudicator explicitly fabricating
challenge/rebuttal evidence rows, which is a much better review surface than a
completed-turn counter.

Also update `review.submit` for this kind: when `kind=collaboration_ledger`,
the submitted `verdict` must match the front-matter `verdict`. Otherwise a lane
could publish a `needs_revision` ledger and pass `--verdict accept`.

Primary tests:

- `go/pkg/artifactcontracts`: valid ledger; missing front matter; unknown
  fields; invalid entry kind; clearing verdict without challenge/rebuttal;
  participant mismatch; invalid verdict.
- `go/pkg/mutations`: `artifact.publish` refuses invalid ledger and
  `review.submit` refuses verdict mismatch.
- `go/cmd/striatum`: invalid publish maps to exit 6 through the existing CLI
  path.

### 2. Fix verdict cycles before relying on the gate

The adjudicator's `needs_revision` path must use declared cycles. Implement a
small cycle router in `go/pkg/mutations/review.go`:

1. When a verdict-capable job returns `needs_revision`, load the workflow
   snapshot and find a cycle whose `from` is the job's `workflow_job_id` and
   `on_verdict` is `needs_revision`.
2. Count prior `needs_revision` verdicts for this cycle source in this run.
   If the count has reached `max_iterations`, open the existing human
   checkpoint with a clear "cycle budget exhausted" reason.
3. If budget remains, release the current lease, mark the source job complete
   enough to close the current message, reset the cycle target and downstream
   jobs in the cycle slice to queued/blocked as appropriate, increment attempts,
   cancel stale pending/claimed messages for those jobs, enqueue the target, and
   emit an event such as `cycle.requeued`.
4. Preserve the current behavior for workflows with no declared route.

The reset set should be the jobs reachable from `cycle.to` up to and including
`cycle.from`, not the whole run. That matches the old review-cycle intent and
prevents a collaboration retry from reviving unrelated completed phases.

This is not a new RFC 0093 primitive; it is making the already-declared workflow
cycle contract true in the Go runtime. It is required for both
`falsification_gate` and `cross_examination`.

Primary tests:

- A review/phase-synthesis `needs_revision` with a declared cycle requeues the
  target instead of opening a checkpoint.
- The next `needs_revision` after `max_iterations` opens the checkpoint.
- Accepting the adjudicator verdict leaves the cycle path untouched and enqueues
  downstream jobs.
- Stale queue messages from the previous cycle attempt are not claimable.

### 3. Represent the collaboration pack in the generator/catalog

Keep the public generator surface simple: operators should call
`workflow generate --shape falsification_gate` and
`workflow generate --shape cross_examination`.

Internally, add a `collaboration` generator module rather than growing
`generate.go` further. The catalog can use existing `shape` entries tagged with
`shape_family: "collaboration"` and `generation_status: "generated"`; a new
top-level catalog kind is unnecessary for V1 unless the UI needs a separate
filter later.

Add generator options conservatively:

- `topic` (required for collaboration shapes)
- `max_dialog_rounds` (default 3)
- `max_revision_cycles` (default 1)
- `falsifier_count` (default 2, only for `falsification_gate`)
- `include_scribe` (default false)

Refuse `single_agent` for these shapes unless the operator uses the existing
same-model override path. The point of an adjudicator is independent judgment.
The generator should prefer `multi_review` or `author_reviewer` lane sets and
bind the adjudicator to a lane different from the holder/proposer.

Update workflow lint so adjudicator jobs are treated as reviewer-like for model
family independence. The narrow rule can key on either `role_id:
"adjudicator"` or an expected artifact of kind `collaboration_ledger`; it does
not need to reclassify every ordinary `phase_synthesis` job.

### 4. Generate V1.1 phase graphs

Use `striatum.workflow.v1.1` because the substance-gate is a
`phase_synthesis`-class job.

For `falsification_gate`, generate:

- Phase `dialogue`:
  - `holder` job, `interrogable: true`, produces the draft/proposal artifact
    and remains live.
  - One or more serialized `falsifier_<n>` jobs. These are ordinary
    dialogue-driver jobs, not review gates. Their role prompt tells them to use
    existing MCP/CLI `interrogation.open`, `interrogation.ask`,
    `interrogation.close` against the holder and then complete. They do not
    publish the gate artifact.
  - `adjudicate` job, `type: "phase_synthesis"`, role `adjudicator`, expected
    artifact kind `collaboration_ledger`.
- Phase `commit`:
  - `commit_proposal` or `apply` job, the downstream work that must be
    unreachable until `adjudicate` records `accept` or `accept_with_findings`.
  - A simple phase synthesis/final summary job if required by the V1.1 phase
    invariant that every phase has one `phase_synthesis` and at least one peer.
- Edges:
  - holder -> falsifier_1 -> falsifier_2 -> adjudicate
  - adjudicate -> commit_proposal
- Cycle:
  - adjudicate -> falsifier_1 on `needs_revision`, bounded by
    `max_revision_cycles`.

For `cross_examination`, generate:

- `author_draft`, `interrogable: true`, producing a draft finding/proposal.
- Serialized `cross_examiner_<n>` jobs. Each non-author lane must ask a
  falsifying question through the existing interrogation methods.
- `adjudicate`, expected `collaboration_ledger`.
- `publish_finding` or `commit_proposal`, downstream of the clearing ledger.
- Cycle from `adjudicate` back to the first cross-examiner, or to an
  `author_revision` job when the generated option requests author rework.

For `scribe`, implement it as a participant modifier first:

- `include_scribe=true` adds a `scribe_note` job to the dialogue phase.
- The role prompt forbids hypothesizing and asks only for a timestamped
  `progress_note` / decision trail from the dialogue trajectory.
- The scribe output is provenance, not the gate input. The adjudicator still
  reads the `dialogue` trajectory and emits the only ledger.

If a standalone `--shape scribe` is needed for catalog completeness, make it a
small conversation-plus-scribe fixture, but do not let that delay the two
required generated shapes.

### 5. Make dialogue access explicit in packets/prompts

No new daemon methods are needed. The generated role prompts should name the
existing MCP methods and the CLI fallback shapes:

- falsifier / cross-examiner:
  - `interrogation.open`
  - `interrogation.ask`
  - `interrogation.close`
- adjudicator:
  - `trajectory.export` with `profile: "dialogue"`
  - `artifact.publish` / `review.submit` with kind `collaboration_ledger`
- scribe:
  - `trajectory.export` with `profile: "dialogue"`

It is worth adding optional command templates to `buildPacketCommands` for the
existing dialogue methods, but they should remain templates when target session
ids are not known at packet-build time. The correctness requirement is that
agents have explicit, audited daemon surfaces to call; the workflow must not ask
them to scrape terminal output or infer state from panes.

### 6. Anti-theater tests

Use a deterministic test harness around the ledger and trajectory projection
rather than trying to unit-test model judgment.

Recommended fixtures:

1. Hollow transcript:
   - Seed dialogue records with polite but non-specific questions and fluent
     non-answers.
   - Publish a ledger with `verdict: needs_revision`, a claim, and no landed
     challenge/rebuttal pair.
   - Submit the adjudicator verdict as `needs_revision`.
   - Assert the commit/proposal job is not queued and the bounded cycle target
     is queued for another dialog round.

2. Landed challenge:
   - Seed dialogue records with a concrete challenge and a concrete rebuttal.
   - Publish a ledger with `claim`, `challenge`, and `rebuttal` entries
     referencing `dialogue:<seq>` records.
   - Submit `accept` or `accept_with_findings`.
   - Assert the commit/proposal job becomes claimable.

3. Theater bypass attempt:
   - Try to publish `verdict: accept` without a challenge/rebuttal pair.
   - Assert publish fails before any downstream job can enqueue.

This pins the structural gate. It does not claim the adjudicator can never make
a bad semantic call.

## Alternatives considered

### New collaboration daemon primitive

Rejected. A `collaboration.open` or floor-control method would duplicate RFC
0082/0086 and expand the live-state authority surface. RFC 0093 V1 can compile
to ordinary jobs plus existing dialog methods.

### Gate on dialog completion

Rejected. This is exactly the theater failure mode: all participants can take
turns without extracting a material constraint. Completion remains a
precondition; the ledger verdict is the gate.

### Reuse `finding` or `findings_ledger`

Rejected for the gate evidence. Findings already carry verdict intent, but they
do not normalize claim/challenge/rebuttal rows or cite dialogue turn refs. The
adjudicator needs a structured artifact whose validator can enforce minimum
substance.

### Ship every RFC 0093 shape immediately

Rejected for V1. `fog_of_war_review` wants hidden-fragment sequencing, and
`synaptic_prune` wants the `post_dialog_hook` liveness fix. The first slice
should land the contract, gate, `falsification_gate`, `cross_examination`, and
the `scribe` modifier. Defer the rest explicitly in docs and catalog metadata.

## Risks and unknowns

- The adjudicator can still be fooled. Structural ledger validation raises the
  bar but does not replace human judgment for high-stakes gates.
- The cycle router is easy to over-broaden. Reset only the cycle slice, cancel
  stale messages, and test that unrelated completed jobs stay completed.
- `dialogue:<seq>` refs are shape-checked, not database-resolved, in the pure
  artifact contract. Stronger ref resolution can be added later inside
  `artifact.publish`, but V1 should not make artifactcontracts depend on
  Postgres.
- Current docs describe front matter as flat `key: <json-value>` while the Go
  parser now supports YAML sequences/mappings. The spec update for
  `collaboration_ledger` should state the current behavior accurately.
- Same-model adjudication would make the shape look stronger than it is. The
  generator and lint rules should refuse it by default.
- Scribe can accidentally become another synthesizer. Its prompt must forbid
  hypothesizing and keep the output to observed decision trail/progress notes.

## What could go wrong

The most likely bad implementation is a beautiful catalog entry that validates
but does not actually gate anything. That happens if the generated graph uses a
non-verdict job for the adjudicator, forgets the edge from adjudicator to
commit, or leaves `needs_revision` on the current human-checkpoint path. The
tests should assert job reachability in daemon state, not just inspect generated
JSON.

The second likely failure is an accept verdict decoupled from ledger content.
That happens if `review.submit --verdict accept` can publish a ledger whose
front matter says `needs_revision`, or if a clearing ledger can omit
challenge/rebuttal evidence. The submit-review consistency check and
kind-specific ledger validator are the guardrails.

The third failure is D028 drift. The ledger must cite dialogue trajectory refs
and contain curated summaries only. Do not add transcript, stdout/stderr,
provider payload, PTY log, or raw message dump fields to the artifact contract
or examples.

## Rollout sketch

First land the contract slice: `collaboration_ledger` kind, required front
matter, substance validation, submit-review verdict consistency, unit tests,
and spec/glossary updates.

Second land the runtime gate slice: declared-cycle routing for
`needs_revision`, adjudicator independence lint, and daemon tests that prove a
clearing adjudicator verdict is required before the downstream job becomes
claimable.

Third land the generator/catalog slice: `falsification_gate`,
`cross_examination`, `include_scribe`, example workflows, role/prompt stubs,
catalog docs, and workflow validate/lint tests.

Fourth run the anti-theater fixtures and documentation pass: seeded hollow vs
landed-challenge transcripts, D028 guard, `workflow generate` smoke checks for
both required shapes, `docs/reference/workflow-types.md`,
`docs/reference/ubiquitous-language.md`, `docs/reference/spec.md`, and the RFC
0083 catalog re-expression.
