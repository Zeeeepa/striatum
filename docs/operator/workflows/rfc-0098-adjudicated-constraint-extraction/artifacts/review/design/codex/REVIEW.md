---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
---

# Design Review - Threat Model

author: reviewer-codex-gpt-5.5-xhigh-001

## Verdict

needs_revision

## Interrogation

Opened interrogation `intg_e5ddf663e130c22c89920d128cf3e7b9` against the live
synthesizer session `sess_76fe26aae7c4d815f28731663ba4804b`.

I used 1 interrogation round. I stopped because the question remained
unanswered after repeated polls, so further rounds would only queue more
unanswered work rather than resolve the open finding. The unanswered question is
therefore evidence for this review, not a resolved objection.

Question asked: whether the proposed productive-refusal gate scoping,
`schema_version == striatum.collaboration_ledger.v1.1 || shape ==
adjudicated_constraint_extraction`, is intentionally global v1.1 semantics or
should be scoped only to `adjudicated_constraint_extraction`.

## Trust Boundaries And Attack Surfaces

- Artifact front matter is untrusted reviewer/adjudicator-authored YAML entering
  the daemon's contract boundary. New nested rows must reject unknown raw-output
  fields without weakening the existing D028 guard.
- The collaboration ledger verdict is coupled to daemon review routing. Any new
  front-matter verdict that is not routable by the daemon can wedge the run.
- The version/shape predicate is a schema boundary. It must prevent ACE ledgers
  that forget the v1.1 schema bump from bypassing the gate, while preserving
  RFC 0093 v1 additivity.
- The publish and review-submission paths are security-sensitive duplicates of
  the same write surface. A productive-refusal gate that is not enforced on all
  paths is bypassable.
- Slice 2's generated graph crosses a phase boundary into revision synthesis.
  If the generator relies on an edge shape that `run.prepare` rejects, the
  workflow can validate statically but fail at run preparation time.

## Findings

### F1 - Ambiguous v1.1 Gate Scope Can Over-Gate Or Under-Test Future Ledgers

Severity: high

The synthesis proposes `isV11Ledger = schema_version == "...v1.1" || shape ==
"adjudicated_constraint_extraction"`. The second half is good threat-modeling:
an ACE ledger that forgets to bump `schema_version` still cannot publish a naked
`needs_revision` refusal.

The unresolved risk is the first half. If `schema_version == "...v1.1"` is
intended to mean "all v1.1 collaboration ledgers require productive refusals,"
that is a global contract semantic and should be named as such. If the gate is
intended only for RFC 0098's ACE shape, the predicate is too broad. The design
does not decide this explicitly, and the synthesizer did not answer the
interrogation question.

Required revision: state the intended truth table and require tests for it:

- v1 `falsification_gate` with `needs_revision` and no `constraints[]` remains
  valid.
- `adjudicated_constraint_extraction` with `needs_revision` and no productive
  row is rejected even if it forgets the v1.1 schema bump.
- non-ACE `schema_version: striatum.collaboration_ledger.v1.1` with
  `needs_revision` is either deliberately rejected as global v1.1 semantics or
  deliberately accepted as ACE-only semantics. Do not leave this implicit.

### F2 - `findings[]` Is Added As A D028-Relevant Typed Home But Its Row Contract Is Not Specified

Severity: high

The synthesis correctly rejects loosening `entries[]` and instead adds
`constraints[]`, `branches{}`, and `findings[]` as typed homes. It fully
specifies `constraints[]` row keys and explicitly says unknown keys in a
constraint row are rejected so raw-output-shaped fields cannot ride along. It
does not give the same row schema for `findings[]`, even though `findings[]` is
the proposed fix for natural cross-exam rows and is equally inside the
front-matter trust boundary.

That leaves two unsafe implementation paths: under-validation, where arbitrary
keys such as `stdout` can ride in under `findings[]`; or over-validation, where
the build invents a narrower schema and recreates the #79 natural-front-matter
failure in a new field.

Required revision: define the `findings[]` row validator before build starts,
including required keys, optional keys, allowed enum values, and unknown-key
rejection. Add a D028 regression fixture showing a raw-output-shaped unknown key
inside a `findings[]` row is rejected, and a natural multiline/nested
cross-exam row is accepted.

### F3 - Review-Submission Bypass Class Needs An Explicit Idempotency Test

Severity: medium

The synthesis identifies the correct single validator and states that
`publish-artifact`, `submit-review` precheck, and primitive `review.verdict` all
reach it. That is the right architecture. The remaining bypass surface is the
review-submission path when an artifact has already been published or when
submission is treated as idempotent. The design's "all three paths" test should
cover the friendly/idempotent path as well, not only the initial publish path.

Required revision: make the slice-1 tests prove that an already-published or
idempotent review submission cannot record or clear a v1.1 naked
`needs_revision` ledger without reusing the validated front matter.

## Positive Findings

- Keeping the front-matter verdict enum to the four daemon-routable values is
  the correct safety choice. It avoids a contract-passes/runtime-wedges failure
  mode.
- Scoping the gate through `validateCollaborationLedger` is the right
  enforcement point if the three write paths are tested as described.
- Treating `posture` as a non-empty string is correct; a closed enum would
  contradict the RFC's workflow-overridable posture set.
- Deferring shape generation and final discharge review until slice 1 is green
  is appropriate. The #66 back-edge risk should not be hidden inside the
  contract slice.

## Required Revisions Before Build

1. Resolve and document the v1.1/shape gate truth table, then add regression
   tests for each branch of that table.
2. Specify the `findings[]` row schema and D028 unknown-key behavior.
3. Extend the write-path tests to include review-submission idempotency or
   already-published artifact behavior.
