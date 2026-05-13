# Reviewer Role (Dogfood 043)

Two design reviews (one per track, gating implement) plus a 3-way build
review at the end.

## Design reviews (claude, `ergonomics_dx`)

- Track A design review — does the synthesized Python design preserve
  v1 backwards compatibility, give operators a clean `workflow upgrade`
  experience, and keep the generator output legible?
- Track B design review — does the synthesized frontend design open v1
  workflows without surprises, surface validator errors clearly, and
  render parallel groups and write scopes usefully?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — systems posture. Schema migration safety,
  validator soundness, write-scope enforcement, runtime invariants.
- **claude** `ergonomics_dx` — operator UX. Upgrade verb friction,
  status reporting clarity, editor usability, error messages.
- **gemini** `adversarial` — break-the-build posture. Edge cases,
  mixed v1/v2 fixtures, hostile inputs, race conditions.

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "track-a-or-b", "dogfood-043"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

`schema_version` must be the exact string `"striatum.finding.v1"`
(not `"1"`). `artifact_kind` is `"finding"`. `verdict_intent` is one of
`accept | accept_with_findings | needs_revision | reject` (not
`verdict`). `severity` is one of `low | medium | high | critical`.
`tags` is a JSON array. The `author:` byline is a plain markdown line
AFTER the front-matter block — not inside it.

**IMPORTANT — write the REVIEW.md / finding artifact directly.** If
`striatum ack` is denied, write the artifact and exit normally; the
operator publishes on your behalf. Do not ask the operator clarifying
questions and exit. Per dogfood-037 intervention #5 + dogfood-041
friction patterns.
