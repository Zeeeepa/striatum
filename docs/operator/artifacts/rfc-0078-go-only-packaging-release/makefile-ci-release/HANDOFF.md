---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["VERSION", "Makefile", "go/Makefile", ".github/workflows/ci.yml", ".github/workflows/release.yml", "scripts/build_go_release_archives.sh", "scripts/check_go_release_archives.sh"]
---

# Makefile CI Release Archive Handoff
author: operator [self-declared: ci-packager-codex-gpt-5-001]

## Landed

- Added root `VERSION` with `2.0.0`.
- Replaced active root Makefile targets with Go build/test/install/archive
  paths. Python targets are retained only under explicit `legacy-python-*`
  names for the later deletion gate.
- Changed `go/Makefile` to read `VERSION` instead of `pyproject.toml`.
- Added Go release archive and checksum scripts.
- Replaced release CI with GitHub release archives and removed the PyPI
  publish job.
- Added CI for Go tests, archive checks, Go-only smoke scripts, and frontend
  bundle checks without creating a Python virtual environment.

## Remaining Blockers

- `pyproject.toml` and Python tests still exist by design; deletion belongs to
  the final RFC 0078 deletion workflow.
- The Go CLI remains incomplete beyond the currently ported command surface,
  so package smoke is scoped to version, embedded daemon assets, and workflow
  validation.
