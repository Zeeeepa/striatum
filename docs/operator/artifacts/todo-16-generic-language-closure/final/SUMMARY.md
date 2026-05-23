---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/todo-16-generic-language-closure/scan/SCAN.md", "docs/operator/artifacts/todo-16-generic-language-closure/apply/HANDOFF.md", "docs/operator/artifacts/todo-16-generic-language-closure/review/REVIEW.md"]
---

# TODO 16 Generic Language Closure Summary
author: generic-language-closer-codex-gpt-5-001

## Result

Completed the current TODO 16 sweep and left TODO 16 open as standing hygiene.

Changed:

- Added the bounded plan, runnable workflow scaffold, prompts, and artifacts
  under `docs/operator/plans/todo-16-generic-language-closure.md`,
  `docs/operator/workflows/todo-16-generic-language-closure/`, and
  `docs/operator/artifacts/todo-16-generic-language-closure/`.
- Reworded RFC 0056's generic layout recommendation from
  `Engram-style dogfood corpus` to `structured run-record corpus`.
- Broadened `tests/test_doc_links.py` so stale current-doc Engram phrases are
  checked across the curated current Markdown doc set plus the historical tmux
  script, with explicit historical allowlisting.

## Validation

```bash
PYTHONPATH=src python3 -m striatum.cli workflow validate docs/operator/workflows/todo-16-generic-language-closure/workflow.json --json
# {"data":{"valid":true,"workflow_id":"todo-16-generic-language-closure"},"ok":true}

.venv/bin/python -m pytest -q tests/test_doc_links.py
# 7 passed in 0.11s

.venv/bin/python -m ruff check tests/test_doc_links.py
# All checks passed!

rg -n --glob '*.md' --glob '*.sh' --glob '!docs/operator/**' --glob '!docs/issues/**' --glob '!docs/ENGRAM_INCUBATION_CONTEXT.md' --glob '!docs/INTERVIEW_LOG.md' --glob '!docs/PRIOR_ART.md' --glob '!docs/RFC_0014_DOGFOOD_FIX_SPEC.md' --glob '!docs/dogfood/**' "Corpus Contract V2 RFC in design|Engram-style|Engram repo root" README.md AGENTS.md docs scripts
# no matches

PYTHONPATH=src python3 - <<'PY'
from pathlib import Path
from striatum.artifact_contracts import validate_artifact_front_matter
for kind, path in [
    ("work_plan", "docs/operator/plans/todo-16-generic-language-closure.md"),
    ("synthesis", "docs/operator/artifacts/todo-16-generic-language-closure/scan/SCAN.md"),
    ("synthesis", "docs/operator/artifacts/todo-16-generic-language-closure/apply/HANDOFF.md"),
    ("finding", "docs/operator/artifacts/todo-16-generic-language-closure/review/REVIEW.md"),
]:
    p = Path(path)
    validate_artifact_front_matter(kind=kind, path=p, payload=p.read_bytes())
PY
# all listed front matter validated
```

## Shared-Doc Updates To Report

No edits were made to `docs/TODO.md`, `docs/ROADMAP.md`, or
`docs/operator/BRIEF.md`.

Suggested operator note for the next allowed shared-doc refresh:

- TODO 16 remains open; add a follow-up note that the
  `todo-16-generic-language-closure` sweep fixed RFC 0056 wording and
  broadened the current-doc stale Engram phrase guardrail.
