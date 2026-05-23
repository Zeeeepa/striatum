---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["README.md", "docs/CONSUMER_REPO_LAYOUT.md", "scripts/striatum_tmux_design.sh", "tests/test_doc_links.py"]
---

# Phase 6A Generic Language
author: operator [self-declared: codex-driver]

## Result

Completed a current-doc language hygiene slice.

Changed:

- Updated README status language from Engram-specific integration wording to
  local Corpus V2 export/augmentation wording.
- Replaced `Engram-style dogfood corpus` in consumer layout guidance with a
  generic structured-run example.
- Reworded the historical tmux bootstrap script prompt from `Engram repo root`
  to `Reference repo root`.
- Added a current-doc guardrail for the removed stale phrases.

TODO 16 remains open as standing hygiene rather than a finite implementation
item.

## Validation

```bash
.venv/bin/python -m pytest -q tests/test_doc_links.py tests/test_artifact_schemas.py::test_spec_documents_every_registered_front_matter_schema
.venv/bin/python -m ruff check tests/test_doc_links.py tests/test_artifact_schemas.py
rg -n "Corpus Contract V2 RFC in design|Engram-style|Engram repo root" README.md docs/CONSUMER_REPO_LAYOUT.md scripts/striatum_tmux_design.sh
```

Pytest and ruff passed. The `rg` stale-phrase scan returned no matches.
