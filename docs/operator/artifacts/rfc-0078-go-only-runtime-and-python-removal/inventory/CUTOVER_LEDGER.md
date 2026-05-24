---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0078-go-only-runtime-and-python-removal.md"]
---

# RFC 0078 Cutover Ledger
author: operator [self-declared: inventory-codex-gpt-5-001]

## Summary

Tracked active Python footprint at workflow start:

- 201 Python files under `src/striatum`
- 176 Python files under `tests`
- 5 Python scripts
- `pyproject.toml` as packaging, version, console-script, pytest, ruff, and mypy authority

## Ledger

| Class | File groups | Cutover dependency |
|---|---|---|
| `port` | `src/striatum/cli/*.py`, `src/striatum/daemon_entrypoint.py` | Replace with Go `striatum` CLI. Route daemon-owned behavior through RPC and keep local authoring Go-native. |
| `port` | `src/striatum/service*.py`, `src/striatum/web/*.py`, web templates | Replace with Go local web/service or retire routes explicitly. |
| `port` | `src/striatum/workflow.py`, `src/striatum/workflow_generator/*.py` | Go packages exist but need parity for full validation/generation/catalog behavior. |
| `port` | `src/striatum/artifacts.py`, `artifact_contracts.py`, `evidence_presentation.py`, run-summary formatting | Go publish/front-matter/artifact contracts must become complete. |
| `port` | `src/striatum/corpus/*.py`, `src/striatum/archive/*.py` | Port accepted corpus/archive/export/verify surfaces or retire the command family. |
| `port` | `src/striatum/skills/*.py`, `src/striatum/plugins/*.py`, `src/striatum/scaffold/*.py` | Move installer/scaffold logic to Go; templates can become embedded assets. |
| `delete_after_gate` | `src/striatum/daemon_pg/**/*.py`, `daemon_rpc/**/*.py`, `daemon_apply/**/*.py` | Delete once Go CLI/web no longer imports Python registries, RPC clients, or handlers. |
| `delete_after_gate` | `src/striatum/_daemongo/**` | Delete when Go release artifacts replace wheel-embedded binaries. |
| `retire` | `src/striatum/api.py` | Python in-process API should not survive unless a new Go library API is accepted. |
| `retire_or_port` | dogfood web modules | Port only if historical dogfood browsing remains current product surface. |
| `port` | `tests/**/*.py` | Migrate behavior coverage to Go, shell, or browser tests; do not preserve files mechanically. |
| `retire` | `pyproject.toml` | Remove after version, packaging, console scripts, and test config move to Go-only surfaces. |
| `rewrite` | `Makefile`, `go/Makefile`, scripts, CI | Replace venv/pip/pytest/wheel workflows with Go build/test/package/check commands. |
| `rewrite_doc` | README, SPEC, getting started, release, MCP, agent/human docs, skill/plugin templates | Stop describing Python as current Striatum install/runtime. |

## Blockers

1. The Go `striatum` CLI did not exist at workflow start.
2. Local web/service is still Python/Jinja/resource-package based.
3. Release/install remains PyPI wheel-first.
4. Python tests carry most CLI/web/workflow/artifact behavior coverage.
5. RPC/table generators are still Python scripts.
6. Product decisions remain for historical mention policy, web route retention, PyPI deprecation, binary names, and replacement aggregate validation.

## Ordered Cutover

1. Accept RFC 0078 decisions.
2. Add Go `striatum` CLI shell over daemon RPC plus local workflow authoring.
3. Port or retire web routes.
4. Finish Go workflow/artifact/corpus/archive/skills/plugin/scaffold parity.
5. Move pytest coverage to Go integration/unit, shell smoke, and existing frontend tests.
6. Replace packaging, release, smoke, and Make targets.
7. Rewrite active docs/templates/examples.
8. Delete Python source/tests/scripts/`pyproject.toml`.
9. Add tracked-head Python-trace guardrails.
