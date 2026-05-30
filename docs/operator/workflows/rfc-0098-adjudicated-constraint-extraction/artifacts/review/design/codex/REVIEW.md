---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
---

# Design Review - Threat Model Attempt 2

author: reviewer-codex-gpt-5.5-xhigh-001

## Verdict

needs_revision

## Interrogation

Opened interrogation `intg_d48d3996a417fe050d82475559fa42ec` against the live
synthesizer session `sess_76fe26aae7c4d815f28731663ba4804b`.

I used 1 interrogation round. I stopped because the question remained
unanswered after repeated polls, and closing the thread closed the target
session. Additional rounds were therefore unanswerable.

Question asked: the revision hardens `binding: true` constraints by requiring a
high/critical `findings[]` source and non-empty verification, but
`kind: unresolved_question` rows still satisfy the productive-refusal gate while
leaving `source_finding`, `source_refs`, and `verification` optional. I asked
what prevents an adjudicator from satisfying the gate with a trivial unsourced
unresolved-question row.

## Trust Boundaries And Attack Surfaces

- Artifact front matter is untrusted adjudicator-authored YAML entering the
  daemon contract boundary.
- The productive-refusal gate is structural only, so every field it accepts as
  gate-satisfying must be tied to the objection lifecycle strongly enough that
  "one hollow row" does not become the new ritual.
- D028 prohibits raw transcript/provide-output capture; source references must
  point at curated dialogue turns or typed findings, not raw provider output.
- The revised shape-only gate correctly separates format version from shape
  behavior, but the gate's productive-row predicate still decides whether a
  refusal is meaningful enough to unblock the next revision cycle.

## Findings

### F1 - `unresolved_question` Rows Can Still Satisfy The Gate Without Provenance

Severity: high

The attempt-2 synthesis fixes the original gate-scope issue: the productive
refusal gate is now shape-only, and `adjudicated_constraint_extraction` requires
`schema_version: striatum.collaboration_ledger.v1.1`. It also fixes the
`findings[]` contract gap and adds an idempotent-submit bypass test.

The remaining bypass is narrower but still load-bearing. The productive-row
predicate remains:

```
binding == true OR kind == "unresolved_question"
```

For `binding: true`, the revision now requires a high/critical `findings[]`
source and concrete verification. For `kind: unresolved_question`, the revision
explicitly leaves `source_finding`, `source_refs`, and `verification` optional.
That means the minimum gate-satisfying refusal can be an unsourced row saying,
in effect, "open question remains." Structurally, this preserves the same
theater risk RFC 0098 is trying to remove: the adjudicator can publish a
`needs_revision` verdict with no binding constraint and no traceable link to a
load-bearing objection.

The RFC allows unresolved-question rows to satisfy the gate, but it also frames
the v1.1 table as typed and sourced. An unresolved question does not need a
verification gate, but it does need provenance: either a `source_finding` that
resolves to a high/critical `findings[]` row, or `source_refs` to curated
`dialogue:<seq>` turns, and preferably both where available.

Required revision before build:

- Require every `unresolved_question` productive row to carry at least one
  provenance anchor: a resolving `source_finding`, non-empty `source_refs`, or a
  specific equivalent field with the same D028-safe semantics.
- Add regression tests showing an unsourced `unresolved_question` row does not
  satisfy the productive-refusal gate, while a sourced unresolved-question row
  does.
- Update the slice-1 authoring examples to show sourced unresolved-question
  rows, not bare placeholders.

## Resolved Items

- The shape/version truth table is now explicit and preserves RFC 0093 v1
  additivity.
- The `findings[]` row schema is now specified with unknown-key rejection.
- The write-path tests now call out idempotent/already-published review
  submission behavior.
- Keeping the verdict enum to the four daemon-routable values remains the right
  choice.
