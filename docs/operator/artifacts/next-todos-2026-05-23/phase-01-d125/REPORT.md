---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/DECISION_LOG.md", "docs/TODO.md", "src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py", "src/striatum/artifact_contracts.py"]
---

# Phase 1 D125 Live Evidence
author: operator [self-declared: codex-driver]

## Result

Scaffolded and ran the opt-in live evidence workflow
`docs/operator/workflows/d125-auto-finalize-live-evidence-2026-05-23/workflow.json`.
The nested run `run_793f221c416768dffbe82d4286bc1f10` completed through
`recovery auto-finalize --live` after a dry-run projection found one eligible
expected finding artifact.

The live invocation published
`docs/operator/artifacts/d125-auto-finalize-live-evidence-2026-05-23/REVIEW.md`
as artifact `art_807c61acb0d0cdf273da1d3f306f2bf6` and completed the review
job with verdict `accept`. Evidence was exported to
`docs/operator/artifacts/d125-auto-finalize-live-evidence-2026-05-23/evidence.json`.

## D125 Gate Status

D125 remains pending. This run records one live success in one lane shape:
`operator_self_declared_review`. The packet was operator self-declared and
unattested (`no_attached_supervisor`), so it is useful evidence for the live
path but does not satisfy the default-live gate by itself.

The pending gate artifact is
`docs/operator/artifacts/d125-auto-finalize-live-evidence-2026-05-23/GATE.md`.
It validates as `auto_finalize_gate_evidence` and does not claim
`gate_status: "satisfied"`.

## Validation

- `PYTHONPATH=src python3 -m striatum.cli workflow validate --allow-same-model-pairing docs/operator/workflows/d125-auto-finalize-live-evidence-2026-05-23/workflow.json --json`
- `PYTHONPATH=src python3 -m striatum.cli recovery auto-finalize --run-id run_793f221c416768dffbe82d4286bc1f10 --dry-run --mtime-grace-seconds 0 --json`
- `PYTHONPATH=src python3 -m striatum.cli recovery auto-finalize --run-id run_793f221c416768dffbe82d4286bc1f10 --live --mtime-grace-seconds 0 --json`
- `PYTHONPATH=src python3 -m striatum.cli evidence export --run-id run_793f221c416768dffbe82d4286bc1f10 --path docs/operator/artifacts/d125-auto-finalize-live-evidence-2026-05-23/evidence.json --json`
- `PYTHONPATH=src python3 - <<'PY' ... validate_artifact_front_matter(kind='auto_finalize_gate_evidence', ...)`
