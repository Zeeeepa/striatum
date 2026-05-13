# Track B Build Review Prompt (ergonomics_dx, 3-way)

Produce REVIEW.md at `docs/dogfood/042/track_b/review/build/<lane>/REVIEW.md`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0044", "engram", "build", "track_b"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Review the RFC 0044 body at `docs/rfcs/0044-engram-phase-1-implementation-spec.md` under **ergonomics_dx** posture.

Per-lane angle:

- **codex** (you if codex): technical clarity — schemas, MCP capability vocabulary, retrieval API shape, augmentation-not-replacement boundary.
- **claude** (you if claude_code): operator-side UX — how a first-time operator discovers + uses the Engram retrieval surface, graceful degradation.
- **gemini** (you if gemini): adversarial — does the RFC accurately cite Engram's existing schemas from `~/git/engram/`? Cross-corpus capability scope? Provenance overclaim?

Required checks (all):

- RFC 0044 V1 acceptance criteria concrete enough that a future dogfood can implement against them.
- Augmentation-not-replacement boundary preserved (Engram's claims/beliefs/ingestion/segmentation schemas NOT redesigned).
- Striatum-must-run-without-Engram fallback explicit.
- Capability vocabulary documented.
- Open questions for future RFCs (Phase 2/3/4) clearly named.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
