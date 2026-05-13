# Build Review Prompt (RFC 0045, 3-way)

Produce REVIEW.md at `docs/dogfood/043/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0045", "multi-phase-workflow", "build"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Review BOTH implementations:

- Python core handoff: `docs/dogfood/043/build/python/HANDOFF.md`
- Frontend handoff: `docs/dogfood/043/build/frontend/HANDOFF.md`

Per-lane angle:

- **codex (threat_model)**: schema-version handling correctness, validator rule integrity for cross-phase edges, audit chain unaffected, `phase_synthesis` enforcement cannot be bypassed.
- **claude (ergonomics_dx)**: first-time-operator discoverability of multi-phase shape, validator error messages operator-actionable, CLI `workflow upgrade --add-phases` UX clear, React Flow affordances obvious.
- **gemini (adversarial threat_model)**: malformed v1.1 inputs (missing `phase_id`, dangling phase references, duplicate phase ids), backwards-compat edge cases (v1 workflow with stray `phase_id`), frontend prop contract under v1 input, drag-drop refusal path.

Required checks (all lanes):

- **Backwards compatibility**: every existing v1 workflow still validates and executes unchanged. The handoffs MUST cite a passing backwards-compat test.
- **V1.1 acceptance criteria** from RFC 0045 are met (cite the RFC bullet → implementation site).
- **Validator output is operator-actionable**: error messages name the field, the rule, and a suggested fix.
- **React Flow band rendering does not break v1 graph rendering**: v1 fixtures render with no bands, no thick edges, unchanged from prior.

Cite specific files / lines / test names. "Looks good" is not a review.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
