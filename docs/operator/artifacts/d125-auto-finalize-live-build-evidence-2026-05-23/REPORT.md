---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/DECISION_LOG.md", "docs/TODO.md", "docs/operator/BRIEF.md", "src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py", "src/striatum/artifact_contracts.py", "docs/operator/artifacts/next-todos-2026-05-23/phase-01-d125/REPORT.md"]
---

# D125 Live Build Auto-Finalize Evidence
author: worker-codex-gpt-5-002

## Result

Scaffolded and ran
`docs/operator/workflows/d125-auto-finalize-live-build-evidence-2026-05-23/workflow.json`.
The nested run `run_6ff2b4939f9a37987cc9fb38413b8079` completed through
workflow-opted-in live `recovery auto-finalize --live`.

The dry-run projection found one eligible expected synthesis artifact. The
live invocation published
`docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/BUILD.md`
as artifact `art_20038e27a0e720033b21f27fae15fa86` and completed build job
`job_run_6ff2b4939f9a37987cc9fb38413b8079_write_build_evidence`.

## D125 Gate Status

D125 remains pending. This run records a second opt-in live behavioral success
and uses a different lane shape from the previous evidence slice:
`operator_self_declared_build` instead of `operator_self_declared_review`.
The packet was operator self-declared and unattested
(`no_attached_supervisor`), so it is not attested lane evidence.

The pending gate artifact is
`docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/GATE.md`.
It does not claim `gate_status: "satisfied"`. I recorded
`contested_audit_chain_events: 1` because
`daemon doctor --repo /home/halbritt/git/striatum --json` reported an existing
repo event-chain `row_hash_mismatch` for event `7506`. The run-scoped doctor
for this nested run reported `ok: true` and no run-scoped problems.

## Validation

- `PYTHONPATH=src python3 -m striatum.cli workflow validate docs/operator/workflows/d125-auto-finalize-live-build-evidence-2026-05-23/workflow.json --json` passed.
- `PYTHONPATH=src python3 -m striatum.cli recovery auto-finalize --run-id run_6ff2b4939f9a37987cc9fb38413b8079 --dry-run --mtime-grace-seconds 0 --json` returned one eligible artifact.
- `PYTHONPATH=src python3 -m striatum.cli recovery auto-finalize --run-id run_6ff2b4939f9a37987cc9fb38413b8079 --live --mtime-grace-seconds 0 --json` finalized one artifact and completed the build job.
- `PYTHONPATH=src python3 -m striatum.cli evidence export --run-id run_6ff2b4939f9a37987cc9fb38413b8079 --path docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/evidence.json --json` exported evidence with `doctor.ok: true`.
- `PYTHONPATH=src python3 -m striatum.cli doctor --run-id run_6ff2b4939f9a37987cc9fb38413b8079 --verbose --json` returned `ok: true` with no problems.
- `PYTHONPATH=src python3 -m striatum.cli daemon doctor --repo /home/halbritt/git/striatum --json` returned overall command success but reported pre-existing repo cutover/event-chain problems, including one `row_hash_mismatch`.
- Focused front-matter validation passed for `BUILD.md`, `GATE.md`, and `REPORT.md`.
