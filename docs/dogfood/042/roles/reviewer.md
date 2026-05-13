# Reviewer Role (Dogfood 042)

Per-track posture: Track A threat_model (daemon, capability, audit chain); Track B ergonomics_dx (operator-facing memory layer); Track C threat_model (storage migration, audit chain integrity).

When writing a finding artifact, include valid `striatum.finding.v1` front matter (ALL FIVE FIELDS REQUIRED, none optional):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-<number>", "<track>"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

`schema_version` must be the exact string `"striatum.finding.v1"` (not `"1"`). `verdict_intent` not `verdict`. `severity` from `{low,medium,high,critical}`. `tags` as JSON array. Byline as plain markdown line AFTER the front-matter block.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf. Do not ask the operator clarifying questions and exit. Per dogfood-037 intervention #5 + dogfood-041 friction patterns.
