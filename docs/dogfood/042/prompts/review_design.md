# Design Review Prompt (per-track posture)

Produce the REVIEW.md artifact at the path your work packet specifies (under `docs/dogfood/042/track_<x>/review/design/<posture>/`).

Use the posture supplied in your work packet's `review_policy`. Tracks A and C: `threat_model`. Track B: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — schema_version must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-<number>", "<track>", "design"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the synthesis at `docs/dogfood/042/track_<x>/DESIGN_SYNTHESIS.md`. Apply the posture lens. Track A + C: are the trust boundaries enumerated and mitigated? Track B: are the affordances discoverable from a first-time-operator perspective?

Specific checks per track:

- **Track A (Go daemon)**: capability vocabulary, audit chain integrity, Postgres credential handling, Go module supply-chain hygiene, harness extension correctness.
- **Track B (Engram)**: cites Engram's actual schemas accurately, augmentation-not-replacement boundary clear, capability scope between corpora, Striatum-runs-without-Engram fallback.
- **Track C (Repo-local PG)**: migration safety, schema integrity, daemon-mandatory boundary defensible, audit chain integrity preserved.

**IMPORTANT — write the REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
