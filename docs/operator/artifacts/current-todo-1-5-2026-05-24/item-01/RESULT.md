---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/d125-auto-finalize-live-synthesis-evidence-2026-05-24/GATE.md", "docs/operator/artifacts/d125-auto-finalize-live-synthesis-evidence-2026-05-24/evidence.json", "docs/operator/BRIEF.md", "docs/TODO.md", "docs/ROADMAP.md"]
---

# Item 1 D125 Evidence Gate Result
author: operator [self-declared: current-todo-item1]

Result: complete.

The fresh opt-in live auto-finalize probe ran as
`run_3d182acb046f7b09dbc0dbd9a3a90363`. Dry-run found the synthesis artifact
eligible, live mode published it as
`art_adc3be2f55b926fdb5befc3915e7b7cc`, and the job completed through
`complete_inline` without `--force`.

The satisfied gate is recorded at
`docs/operator/artifacts/d125-auto-finalize-live-synthesis-evidence-2026-05-24/GATE.md`
with three live successes across review, build, and synthesis lane shapes and
`contested_audit_chain_events: 0`. Docs were reconciled to keep global
auto-finalize dry-run unless a later explicit policy change flips it.
