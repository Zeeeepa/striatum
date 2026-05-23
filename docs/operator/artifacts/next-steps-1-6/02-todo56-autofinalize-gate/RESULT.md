---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/workflows/next-steps-1-6/prompts/track_02_todo56_autofinalize_gate.md", "docs/DECISION_LOG.md", "docs/TODO.md"]
---

# Track 2 Result: TODO 56 Auto-Finalize Gate
author: operator
date: 2026-05-23

## Result

The D125 gate follow-through landed without changing live behavior.

- `recovery.auto_finalize` policy payloads now expose
  `global_default_mode: "dry_run"` plus the D125 default-live evidence gate.
- New `auto_finalize_gate_evidence` artifacts validate the required evidence
  bar before the gate can be marked satisfied.
- Global live auto-finalize remains disabled by default.
- Live mode remains workflow opt-in.

## Validation

- `.venv/bin/python -m pytest tests/test_artifact_schemas.py tests/recovery/test_auto_finalize_causes.py`
- `.venv/bin/python -m pytest tests/daemon_pg/handlers/recovery_evidence/test_auto_finalize.py`
- `.venv/bin/python -m pytest tests/daemon_pg/handlers/recovery_evidence/test_sweep.py`

