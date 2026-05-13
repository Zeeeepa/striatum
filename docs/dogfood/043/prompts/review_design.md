# Design Review Prompt (RFC 0045)

Produce the REVIEW.md artifact at the path your work packet specifies (under `docs/dogfood/043/review/design/<track>/`). Track is inferred from the work packet's `allowed_paths` — either `docs/dogfood/043/review/design/python/` (Python core) or `docs/dogfood/043/review/design/frontend/` (React Flow editor).

Use the posture supplied in your work packet's `review_policy`. Both review jobs are `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0045", "<track>", "design"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the synthesis at `docs/dogfood/043/DESIGN_SYNTHESIS_python.md` (Python track) or `docs/dogfood/043/DESIGN_SYNTHESIS_frontend.md` (frontend track). Apply the ergonomics_dx lens: are the affordances discoverable from a first-time-operator perspective?

Specific checks per track:

- **Python core**: schema_version handling for v1 vs v1.1 is unambiguous; validator error messages are operator-actionable (point at exact field path, name the rule, suggest a fix); v1 workflows still validate unchanged; `phase_synthesis` job type contract is well-defined; `striatum workflow upgrade --add-phases` CLI shape is intuitive.
- **Frontend (React Flow)**: phase color-banding does not obscure node interactions; cross-phase edge styling is visually distinct without being jarring; side panel mount/dismiss is obvious; drag-drop policy is consistent and discoverable; v1 single-phase rendering is preserved.

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

**IMPORTANT — write the REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
