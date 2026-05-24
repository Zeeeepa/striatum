---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0078-go-only-runtime-and-python-removal.md", "tests/architecture/", "scripts/"]
---

# Python Deletion Guardrails Handoff
author: operator [self-declared: guardrail-owner-codex-gpt-5-001]

## Guardrail Target

After deletion gates pass, tracked HEAD should fail if active Striatum Python
source, tests, packaging, or operator instructions return.

## Proposed Checks

Strict file check:

```text
git ls-files | rg '(^src/.*\.py$|^tests/.*\.py$|^scripts/.*\.py$|\.pyi$|^pyproject\.toml$|__pycache__|\.pyc$)'
```

Active guidance check:

```text
rg -n -i '\b(pytest|mypy|ruff|pip install|python3? -m striatum|venv|wheel|sdist|striatum-orchestrator)\b' README.md docs Makefile scripts .github
```

## Allowlist Policy

No allowlist should be added for product source, tests, packaging, CI, Make
targets, or current operator docs. A historical-provenance allowlist, if any,
must list exact archived RFC/review/prompt paths and must not include active
README, SPEC, ROADMAP, TODO, operator brief, skills, plugins, or examples.

## First Slice

Guardrails are designed here but not enabled yet because active Python surfaces
still exist. Enabling them before CLI/web/test/package parity would correctly
fail the repository.
