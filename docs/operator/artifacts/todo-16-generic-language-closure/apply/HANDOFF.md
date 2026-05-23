---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/todo-16-generic-language-closure/scan/SCAN.md", "docs/rfcs/0056-consumer-repo-directory-structure-opinions.md", "tests/test_doc_links.py"]
---

# TODO 16 Apply Handoff
author: generic-language-codex-gpt-5-002

## Changes

- Updated `docs/rfcs/0056-consumer-repo-directory-structure-opinions.md` to
  replace `Engram-style dogfood corpus` with generic
  `structured run-record corpus` wording.
- Broadened
  `tests/test_doc_links.py::test_current_product_docs_do_not_regress_stale_engram_framing`
  so the exact stale phrases are checked across the curated current Markdown
  doc set plus `scripts/striatum_tmux_design.sh`.
- Added an explicit historical allowlist for the broadened guardrail so
  `docs/ENGRAM_INCUBATION_CONTEXT.md`, `docs/INTERVIEW_LOG.md`,
  `docs/PRIOR_ART.md`, `docs/RFC_0014_DOGFOOD_FIX_SPEC.md`, and
  `docs/dogfood/` remain provenance rather than cleanup targets.

## Validation To Run

```bash
PYTHONPATH=src python3 -m striatum.cli workflow validate docs/operator/workflows/todo-16-generic-language-closure/workflow.json --json
.venv/bin/python -m pytest -q tests/test_doc_links.py::test_current_product_docs_do_not_regress_stale_engram_framing
.venv/bin/python -m pytest -q tests/test_doc_links.py
.venv/bin/python -m ruff check tests/test_doc_links.py
```

## Shared-Doc Updates To Report

No shared TODO, ROADMAP, or BRIEF edits were made. TODO 16 remains a standing
hygiene item; this sweep should be reported as a completed follow-up, not as a
closure of the TODO row.
