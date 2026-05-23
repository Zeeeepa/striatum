---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/evidence_presentation.py", "tests/test_corpus_redaction.py"]
---

# Redaction Existing-Fields Report
author: redaction-closer-codex-001
date: 2026-05-23

## Result

One current evidence-field gap was found and closed.

Evidence redaction previously treated session `close_reason` and
`non_fresh_reason` as safe fields. Both can carry operator-supplied prose:
`session close --reason <text>` stores `close_reason`, and
`register-session --force-non-fresh --reason <text>` stores
`non_fresh_reason`. When present in an evidence payload, they now redact to
the standard evidence free-text placeholder.

## Source And Test Changes

- `src/striatum/evidence_presentation.py`: changed `sessions.close_reason`
  and `sessions.non_fresh_reason` policy from `safe` to `redacted`.
- `tests/test_corpus_redaction.py`: added synthetic coverage proving those
  fields redact while structural session identifiers remain visible.

## Remaining Coverage

No other existing artifact/evidence field gap was identified in this pass.
Future evidence fields still need an explicit policy entry and synthetic
injection coverage before they can be considered safe.

## Verification

- `PYTHONPATH=src python3 -m striatum.cli workflow validate docs/operator/workflows/artifact-schema-redaction-closure/workflow.json`: passed.
- `PYTHONPATH=src .venv/bin/python -m pytest tests/test_corpus_redaction.py tests/test_artifact_schemas.py -q`: 47 passed, 23 skipped.
- `PYTHONPATH=src .venv/bin/ruff check src/striatum/evidence_presentation.py tests/test_corpus_redaction.py`: passed.
