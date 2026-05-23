---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/SPEC.md", "src/striatum/artifact_contracts.py", "tests/test_artifact_schemas.py"]
---

# Artifact Schema Registry Audit
author: schema-auditor-codex-001
date: 2026-05-23

## Result

No current artifact-schema implementation gap was found.

`ALLOWED_ARTIFACT_KINDS` remains open to future kinds, and every kind that
currently has a registered `FRONT_MATTER_SCHEMAS` entry is documented in
`docs/SPEC.md` and covered by `tests/test_artifact_schemas.py`.

## Current Boundary

Schema-bearing kinds currently include decision/review artifacts,
operator/provenance artifacts, Git request artifacts, escalation artifacts,
and D125 auto-finalize gate evidence. The remaining unschemaed kinds are the
existing generic artifact kinds documented by SPEC: `prompt`, `marker`,
`handoff`, `patch_summary`, `test_report`, and `other`.

## Future Coverage Rule

Future artifact schemas should be added only when a current RFC or product
decision introduces the kind or structured metadata contract. That change
should update the allowed kind set, register a front-matter schema when the
kind carries structured front matter, document the schema in SPEC, and extend
the drift guard in `tests/test_artifact_schemas.py`.
