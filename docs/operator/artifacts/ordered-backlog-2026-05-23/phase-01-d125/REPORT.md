---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/DECISION_LOG.md", "docs/TODO.md", "docs/operator/BRIEF.md", "src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py", "src/striatum/artifact_contracts.py"]
---

# TODO 56 D125 Evidence Gate
author: operator [self-declared: codex-driver]

## Result

The next safe TODO 56 action remains an opt-in live auto-finalize dogfood
evidence run. This phase did not mark the gate satisfied because D125 requires
three live successes across at least two lane shapes with zero contested
audit-chain events, and that evidence is not present yet.

Current source matches the intended safety boundary:

- `recovery.auto_finalize` defaults to dry-run.
- Live mode requires workflow opt-in or explicit operator force.
- Policy payloads expose `global_default_mode: "dry_run"` and the D125
  `default_live_gate`.
- `auto_finalize_gate_evidence` artifacts can validate pending or satisfied
  gate state, and satisfied artifacts require the D125 evidence bar.

## Executable Next Step

Run an opt-in workflow with:

```json
"recovery": {
  "auto_finalize": {
    "enabled": true
  }
}
```

Then create a stalled lane that writes a valid declared expected artifact
without publishing/completing, and run:

```bash
PYTHONPATH=src python3 -m striatum.cli recovery auto-finalize --run-id <run_id> --dry-run --mtime-grace-seconds 0 --json
PYTHONPATH=src python3 -m striatum.cli recovery auto-finalize --run-id <run_id> --live --mtime-grace-seconds 0 --json
PYTHONPATH=src python3 -m striatum.cli evidence export --run-id <run_id> --path docs/operator/artifacts/<run>/evidence.json
```

Record one row toward the gate only if the live invocation publishes and
completes from stable expected artifacts, emits clean
`artifact.auto_finalized` / `job.auto_finalized` events, and leaves zero
contested audit-chain events.

## Validation

Scaffold validation passed:

```bash
PYTHONPATH=src python3 -m striatum.cli workflow validate --allow-same-model-pairing docs/operator/workflows/ordered-backlog-2026-05-23/workflow.json --json
```

Gate count after this phase: `0 / 3` live successes recorded.
