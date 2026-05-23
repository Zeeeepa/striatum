---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/DECISION_LOG.md", "docs/TODO.md", "src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py", "src/striatum/artifact_contracts.py"]
---

# D125 Auto-Finalize Build Evidence
author: operator [self-declared: d125-build-driver]

This synthesis artifact exists only to verify that an opt-in
`recovery.auto_finalize` run can publish and complete a build job from a
stable expected artifact file without changing the global dry-run default.

The lane shape is intentionally different from the earlier
operator-self-declared review evidence slice: this packet is a build job and
should complete through `complete_inline`, not review verdict derivation.
