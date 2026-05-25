# Gate E — Remove Python packaging and tooling

You are the implementer for RFC 0078 Gate E. Runs AFTER Gate D. Read first:
`docs/operator/plans/rfc-0078-remaining-work.md` (Gate E),
`pyproject.toml`, the `legacy-python-*` targets in `Makefile`,
`scripts/release_metadata_check.py`, `scripts/check_wheel_size.py`,
`scripts/check_ui_bundle_size.py`, and the Go replacements
`scripts/go_release_metadata_check.sh`, `scripts/go_package_smoke.sh`,
`scripts/go_fresh_clone_smoke.sh`, plus `.github/workflows/ci.yml`.

## Steps

1. Delete `pyproject.toml` (Python packaging, console scripts, pytest/ruff/mypy
   config). Confirm no Go build/install/smoke path depends on it.
2. In `Makefile`: remove the `legacy-python-install/lint/typecheck/test/
   metadata-check` targets and their `.PHONY` entries, and the
   `.venv/bin/python scripts/release_metadata_check.py` invocation. Keep
   `python-trace-report` / `python-trace-guardrail` (they are Go-only removal
   guards, not Python runtime). Rewrite any Python-runtime guidance in Makefile
   comments to Go-only.
3. Confirm `ui-check-bundle` / `ui-bundle-size` cover bundle-size checking
   without `check_ui_bundle_size.py` / `check_wheel_size.py`; if a gap exists,
   fold the check into the existing shell/Go path. Then delete
   `scripts/check_wheel_size.py`, `scripts/check_ui_bundle_size.py`, and
   `scripts/release_metadata_check.py`.
4. Ensure the daemon binary install path does not depend on the deleted Python
   tree (`make install` already copies from `go/bin`; confirm nothing references
   `src/striatum/_daemongo/binaries/`).

## Constraints

- Do not weaken release validation — the Go smoke scripts must still pass.
- Stay within `write_scope.allowed_paths`. Leave `tests/`, `src/`, and
  current-guidance docs to other gates.

## Validate

```bash
scripts/go_release_metadata_check.sh
scripts/go_package_smoke.sh
scripts/go_fresh_clone_smoke.sh
```

## Required artifact

Publish `docs/operator/artifacts/rfc-0078-closure/packaging/SUMMARY.md`
(`artifact_kind: synthesis`): files deleted, Makefile targets removed, the
bundle-size coverage confirmation, and validation output. Use your byline.
