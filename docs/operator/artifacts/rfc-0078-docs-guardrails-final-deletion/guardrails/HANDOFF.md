---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["scripts/python_trace_guardrail.sh", "Makefile", "docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/guardrails/HANDOFF.md"]
---

# Python Trace Guardrail Handoff
author: operator [self-declared: guardrail-implementer-codex-gpt-5-001]

## Entry Points

- `scripts/python_trace_guardrail.sh --report`: readiness report, exits zero
  even when active blockers remain.
- `scripts/python_trace_guardrail.sh --strict`: final deletion guardrail,
  exits nonzero when blocked or unclassified traces remain.
- `make python-trace-report`: Make wrapper for report mode.
- `make python-trace-guardrail`: Make wrapper for strict mode.

## Forbidden Classes

- `active_striatum_python_source`: tracked `src/**/*.py` or `src/**/*.pyi`.
- `active_pytest_surface`: tracked `tests/**/*.py` or `tests/**/*.pyi`.
- `active_python_script`: tracked `scripts/**/*.py` or `scripts/**/*.pyi`.
- `active_python_packaging`: tracked `pyproject.toml`.
- `tracked_python_cache`: tracked `*.pyc` or `__pycache__`.
- `active_python_runtime_guidance`: current operator/adopter docs, scripts,
  Makefile, CI, or skill/plugin templates that still instruct Python
  packaging/test/runtime behavior.

## Allowed Classes

- `historical_provenance`: RFCs, reviews, dogfood material, operator
  artifacts, bounded plans/workflows, handoffs, research, and historical
  prompts.
- `target_workload_allowed`: examples or issue fixtures where a target
  repository may use Python as its own workload.

The scanner now reports zero `unclassified` findings, so every remaining
Python trace is classified as blocked, historical/provenance, or target
workload.

## Validation

- `bash -n scripts/python_trace_guardrail.sh`: passed.
- `make python-trace-report`
  - result: passed report mode
  - counts: `blocked=459`, `unclassified=0`,
    `historical_provenance=736`, `target_workload_allowed=107`
- `make python-trace-guardrail`
  - result: failed as expected
  - failure: `python-trace guardrail failed: blocked=459 unclassified=0`

## Blocker

The strict guardrail cannot be added to `check` or CI yet. It correctly fails
until the Go CLI, Go web/service, Go packaging/release, workflow/artifact
parity, and pytest migration gates delete or retire the active classes.
