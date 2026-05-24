---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["pyproject.toml", "Makefile", "go/Makefile", ".github/workflows/", "scripts/", "README.md"]
---

# Packaging And Release Cutover Handoff
author: operator [self-declared: packaging-porter-codex-gpt-5-001]

## Finding

Python packaging is still authoritative. `pyproject.toml` defines
`striatum-orchestrator`, version `1.57.0`, console scripts, runtime deps,
pytest/ruff/mypy config, and package data. The root Makefile creates a venv,
uses pip, runs pytest/ruff/mypy, builds wheels, and stages Go binaries into
Python package data.

## Go-Only Release Shape

Recommended cutover:

- make the breaking Go-only cutover `v2.0.0`;
- add a root `VERSION` file as the single version source;
- build `striatum`, `striatumd`, and `striatum-supervisor-helper` with
  ldflags carrying version/git metadata;
- keep separate `striatum` and `striatumd` binaries for the first Go-only
  release;
- package release archives, not wheels/sdists;
- upload GitHub release archives and checksums only;
- do not publish new Python runtime artifacts.

## Required Changes

- Move Go module to repo root or otherwise make Go the product root.
- Embed SQL, workflow catalog, web assets, and skill/plugin templates in Go.
- Replace Make targets with Go build/test/check/install/package-smoke targets.
- Rewrite smoke scripts to run Go binaries and Postgres setup without venv.
- Replace release CI with setup-go/setup-node, Go tests, frontend checks,
  archive build, checksums, and Python-trace guardrails.
- Rewrite active install docs away from PyPI, Python 3.11, `.venv`, pip,
  pytest, ruff, mypy, wheel, and sdist guidance.
