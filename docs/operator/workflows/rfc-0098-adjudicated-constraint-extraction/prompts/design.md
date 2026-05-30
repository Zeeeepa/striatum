# Design (one of three parallel lanes)

Task: {{TASK}}

You are one of three design lanes running in parallel. Do not coordinate
with the other lanes — independent perspectives are the point.

Produce a single `DESIGN.md` inside your lane's allowed write path. The
document should cover:

- Problem framing in your own words: why RFC 0093's bare `needs_revision`
  verdict is too weak, and what "productive refusal" must add.
- Proposed approach, sliced smallest-blast-radius first (RFC 0098
  §"Implementation Plan"): (1) `collaboration_ledger.v1.1` additive
  `constraints[]` + `branches[]` and the productive-refusal publish gate
  (`needs_revision` ⇒ non-empty `constraints[]`), also accepting the clearing
  verbs the contract advertises (#88) and natural ledger front matter (#79);
  then (2) the `adjudicated_constraint_extraction` generator shape + fixture
  (needs #84 cycle-aware logical names); then (3) the discharge-verifying
  `final_review`.
- Concretely: where does the constraint schema live in
  `go/pkg/artifactcontracts`? How does the gate hook `review.submit` /
  `publish-artifact` in `go/pkg/mutations` without a new daemon method? How do
  you keep the change additive so every RFC 0093 V1 ledger still validates?
- Two or three alternatives considered and why this one wins (e.g.
  `v1.1`-additive vs a new `v2` schema; artifact-only vs first-class
  `constraint.*` objects — note the latter is explicitly deferred).
- Risks, unknowns, and an explicit "what could go wrong" section — pay
  special attention to keeping the schema change additive, to the
  `adjudication → revision_synthesis` cross-phase edge being legal under
  `run.prepare` (#66), and to the #84 republish dependency.
- A short rollout sketch (what lands first, what lands second).

Keep the document focused. Do not write code. Do not edit files outside
your lane's allowed paths.

When the design is complete, emit the `submit-handoff` packet that the
runner provided in your work packet's `commands` block.
