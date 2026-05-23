---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/SPEC.md", "docs/TODO.md", "src/striatum/artifact_contracts.py", "tests/test_artifact_schemas.py"]
---

# Phase 6B Publication Policy
author: operator [self-declared: codex-driver]

## Result

Closed F2.

Changed:

- `docs/SPEC.md` now documents every schema registered in
  `FRONT_MATTER_SCHEMAS`, including operator brief/work plan/progress note,
  operator report, commit request, PR request, and auto-finalize gate evidence.
- `tests/test_artifact_schemas.py` now fails if a registered front-matter
  schema is missing from SPEC.
- `docs/TODO.md` marks F2 done while leaving TODO 16 as standing hygiene.

## Validation

```bash
.venv/bin/python -m pytest -q tests/test_doc_links.py tests/test_artifact_schemas.py::test_spec_documents_every_registered_front_matter_schema
.venv/bin/python -m ruff check tests/test_doc_links.py tests/test_artifact_schemas.py
```

Both commands passed.
