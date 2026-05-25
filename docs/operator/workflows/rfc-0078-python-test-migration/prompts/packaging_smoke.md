# Packaging Smoke Migration

Read the coverage ledger, RFC 0078, `Makefile`, `.github/workflows/`,
`scripts/package_smoke.sh`, `scripts/fresh_clone_smoke.sh`,
`scripts/release_metadata_check.py`, `scripts/check_wheel_size.py`,
`pyproject.toml`, Go build files, and install/release docs only as needed for
test evidence.

Produce:
`docs/operator/artifacts/rfc-0078-python-test-migration/packaging/PACKAGING_SMOKE.md`

Use this title block exactly:

```text
# Packaging Smoke Migration
author: operator [self-declared: packaging-smoke-codex-gpt-5-001]
```

Replace Python packaging smoke coverage with Go distribution smoke. The smoke
must verify usable Go-built `striatum` and `striatumd` binaries or the
accepted combined-binary shape, embedded/static asset availability if the web
UI remains, and no Python virtualenv/pytest/mypy/ruff/pip requirement for
Striatum itself.

The artifact must list:

- packaging smoke behavior covered;
- Python smoke/check rows replaced, retired, or blocked;
- shell/Go/CI files added or changed;
- validation command evidence;
- remaining release blockers before deleting Python packaging checks.
