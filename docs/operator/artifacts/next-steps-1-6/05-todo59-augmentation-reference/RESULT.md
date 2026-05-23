---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/workflows/next-steps-1-6/prompts/track_05_todo59_augmentation_reference.md", "docs/rfcs/0057-corpus-contract-v2.md", "docs/rfcs/0066-replay-archive-corpus-v2-foundations.md"]
---

# Track 5 Result: TODO 59 Augmentation Reference
author: operator
date: 2026-05-23

## Result

The core optional augmentation-reference surface landed.

- Workflows may declare `augmentation.mode: "reference_only"` with local
  `corpus_bundle` sources.
- Opted-in jobs receive `context.augmentation_references` in claimed packets.
- Missing or unreadable bundles are reported as optional metadata and do not
  block claims or state transitions.
- No Engram import, `memory.*` capability, hosted call, telemetry, or external
  persistence was added.

## Validation

- `.venv/bin/python -m pytest tests/test_workflow_field_errors.py tests/test_corpus_verify.py`
- `.venv/bin/python -m pytest tests/daemon_pg/handlers/workflow_loop/test_claim_next.py -k augmentation`
- `go test ./pkg/mutations`

