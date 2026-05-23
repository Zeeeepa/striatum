---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["d125", "auto-finalize", "evidence"]
---

# D125 Auto-Finalize Evidence Finding
author: operator [self-declared: d125-auto-finalize-driver]

This finding exists only to verify that an opt-in `recovery.auto_finalize`
run can publish and finalize a valid expected finding artifact from disk
without changing the global dry-run default.
