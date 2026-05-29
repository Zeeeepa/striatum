---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: needs_revision
---

# RFC 0093 Design Threat-Model Review

## Scope

Reviewed only the packet-listed target documents:

- `docs/operator/workflows/rfc-0093-collaboration-shapes/TASK.md`
- `docs/rfcs/0093-structured-live-collaboration-workflow-shapes.md`
- `docs/rfcs/0083-iterated-panel-review-with-interrogation.md`
- `docs/rfcs/0034-workflow-generator-and-template-catalog.md`
- `docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md`

Interrogation attempt: opened `intg_7afcbe515e68fb4293a45a02f2a04d8c` against
the active synthesizer session and asked one threat-model question about the
substance-gate evidence contract. No answer had landed before this verdict, so
the interrogation is recorded as attempted but not used as evidence.

## Verdict

`needs_revision`

The design names the right trust boundary, but the proposed enforcement still
lets the central anti-theater gate be satisfied by a syntactically valid ledger
rather than by structurally checked evidence that a material challenge landed
and was rebutted.

## Findings

### 1. Clearing verdicts are not tied to enforceable evidence

RFC 0093 says the adjudicator emits a `collaboration_ledger` with entries and a
verdict, and the downstream commit/proposal job stays gated until the verdict
clears. The artifact schema in RFC 0093 section 4 validates shape, topic,
participants, entries, verdict, and rationale, but it does not define the
minimum evidence invariant for a clearing verdict. A malicious, careless, or
ritual-following adjudicator can publish:

- `verdict: accept`
- a generic rationale
- entries that are syntactically valid but do not include a landed material
  challenge and a specific rebuttal tied to that challenge

That bypasses the exact threat the RFC is meant to eliminate: dialogue theater.

Revision needed: define a gate-level invariant, not only an artifact shape. For
example, for `falsification_gate` and `cross_examination`, a clearing verdict
must require at least one `challenge` entry and one `rebuttal` entry whose
`refs[]` point to dialogue turns in the same run/topic/participant set, plus a
shape-specific rubric field explaining why the rebuttal resolves that exact
challenge. If the invariant is absent, the only allowed clearing state should be
`needs_revision` or `reject`.

The generator and gate wiring should make the commit/proposal phase depend on
that invariant for the adjudicator job's expected ledger path, not on existence
of any valid `collaboration_ledger` with an accept-like verdict.

### 2. Ledger references need same-run and same-topic binding

The ledger references RFC 0081 dialogue turn ids, but RFC 0093 does not state
that validation or gate evaluation must reject refs outside the current
run/topic, outside the declared participants, or outside the pre-gate dialogue
window. Without that binding, an adjudicator could cite stale, unrelated, or
cross-topic turns as evidence and still publish a plausible-looking ledger.

Revision needed: require every `entries[].refs[]` value to resolve to a
dialogue turn owned by the current run, shape topic, declared participant set,
and gate window. This can be a publisher validation check if the publisher has
access to the dialogue read model, or a gate check if artifact validation must
remain purely syntactic.

### 3. V1 acceptance currently conflicts with the task's deferral path

The task says V1 should build items 1-5, but also says `fog_of_war_review` and
`synaptic_prune` may be deferred if time runs short. RFC 0093 acceptance
criterion 7 still requires `synaptic_prune` to run without a liveness race and
prove `post_dialog_hook` fan-out while participants are live. That makes the V1
bar ambiguous: an implementation can follow the task's pure-composition slice
and still fail the RFC acceptance list.

Revision needed: split acceptance into V1-required and future-shape criteria.
For this run, make `falsification_gate`, `cross_examination`, `scribe`, the
ledger contract, and the anti-theater gate tests mandatory. Move
`synaptic_prune` and any `post_dialog_hook` fixture into a clearly deferred
acceptance block unless the run explicitly chooses to ship that shape.

## Trust Boundaries And Attack Surfaces

- **Adjudicator boundary:** a model lane converts dialogue into a gate verdict.
  The design must assume this lane can be wrong, lazy, or compromised.
- **Artifact publisher boundary:** a ledger is durable provenance, but a valid
  front matter shape is not the same as valid gate evidence.
- **Generator/scheduler boundary:** the generated graph decides whether the
  downstream job is reachable. This is the last place to prevent a syntactically
  valid but substantively empty ledger from clearing the gate.
- **Trajectory reference boundary:** dialogue turn ids are capabilities to cite
  evidence. They must not be accepted across runs, topics, participants, or
  windows without validation.
- **D028 boundary:** ledger text must remain curated authored text and must not
  become a raw transcript or provider-output side channel.

## Recommendation

Revise before implementation. The RFC should preserve the current architecture
boundary of no new daemon dialogue method, but it needs explicit gate semantics:
which ledger evidence patterns clear each shipped shape, where those patterns
are checked, and how invalid or unrelated dialogue refs keep the downstream job
unreachable.
