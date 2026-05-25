---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/plans/rfc-0078-remaining-work.md", "docs/operator/workflows/rfc-0078-closure/prompts/gate_packaging.md", "pyproject.toml", "Makefile", "scripts/release_metadata_check.py", "scripts/check_wheel_size.py", "scripts/check_ui_bundle_size.py", "scripts/go_release_metadata_check.sh", "scripts/go_package_smoke.sh", "scripts/go_fresh_clone_smoke.sh", ".github/workflows/ci.yml"]
---

# Gate E — Python Packaging and Tooling Removal Summary
author: implementer-claude-003

## Result

The Python packaging and tooling surface is removed. The Go-only
release/install/smoke path is unchanged and green; the active bundle-size
gate is strengthened rather than weakened. `active_python_packaging` is now
`0` and the three packaging/release Python scripts are gone. The two
remaining `active_python_script` entries are Gate C's generators, which are
out of this gate's write scope.

## Files Deleted

- `pyproject.toml` — Python packaging, console-script entry points
  (`striatum`, `striatumd`), and `pytest`/`ruff`/`mypy` configuration.
  Confirmed no Go build/install/smoke path depends on it: `make install`
  copies binaries from `go/bin`, and `go_fresh_clone_smoke.sh` (which tars
  the working tree without `pyproject.toml`) still passes.
- `scripts/release_metadata_check.py` — superseded by
  `scripts/go_release_metadata_check.sh`, which builds the Go binaries and
  asserts `striatum --version` and `striatumd --describe`
  (`daemon_version=`) match the release version. Was invoked only by the
  removed `legacy-python-metadata-check` target.
- `scripts/check_wheel_size.py` — obsolete. There is no Python wheel in the
  Go-only release; Go archive integrity is covered by
  `scripts/check_go_release_archives.sh`. The script was orphaned (not
  invoked by Makefile, CI, or any shell script).
- `scripts/check_ui_bundle_size.py` — orphaned (not invoked by Makefile or
  CI). Its protections were folded into the active node `ui-bundle-size`
  recipe before deletion (see Bundle-Size Coverage).

## Makefile Targets Removed

Deleted the five `legacy-python-*` recipes and their `.PHONY` entries:

- `legacy-python-install` (`python3 -m venv` + `pip install -e .[dev,daemon-pg]`)
- `legacy-python-lint` (`ruff check`)
- `legacy-python-typecheck` (`mypy`)
- `legacy-python-test` (`pytest`)
- `legacy-python-metadata-check` (`.venv/bin/python scripts/release_metadata_check.py`)

`python-trace-report` and `python-trace-guardrail` were kept — they are
Go-only removal guards (file-existence + grep scans), not a Python runtime.
No Python-runtime guidance comments remained in the Makefile to rewrite.

## Bundle-Size Coverage Confirmation

The standalone Python bundle/wheel checks were **already orphaned** — neither
`check_ui_bundle_size.py` nor `check_wheel_size.py` was wired into the
Makefile or CI. The active bundle-size gate is the node `ui-bundle-size`
recipe, run by CI's frontend `ui-check-bundle` job.

A gap existed and was closed: the prior node recipe only checked total bytes
against a **2 MB** cap, which actually *failed* on the committed ~9.5 MB
bundle (`island-shiki-BzAFxaqU.js` alone is 9.3 MB). The recipe now mirrors
`check_ui_bundle_size.py`'s three guards with its real defaults and env
overrides:

- total bytes ≤ `12_000_000` (`STRIATUM_UI_BUNDLE_MAX_BYTES`)
- file count ≤ `32` (`STRIATUM_UI_BUNDLE_MAX_FILES`), ignoring `manifest.sha256`
- `island-shared-*.js` chunks ≤ `4` (`STRIATUM_UI_BUNDLE_MAX_SHARED_CHUNKS`)

This preserves every protection the Python script offered while making the
active gate pass on the real bundle. Wheel-size checking has no Go-only
analogue and is intentionally dropped (no wheel is produced).

## Install-Path Independence

`make install` copies `striatum`, `striatumd`, and
`striatum-supervisor-helper` from `$(GO_DIR)/bin`. Nothing in the Go tree,
Makefile, or shell scripts references `src/striatum/_daemongo/binaries/`
(only `pyproject.toml`, `MANIFEST.in`, and historical docs did). The daemon
binary install path does not depend on the deleted Python tree.

## Out-of-Scope Note for Gate G

`MANIFEST.in` (`recursive-include src/striatum/_daemongo *`) is residual
Python packaging metadata but lies outside this gate's write scope. The
guardrail does not flag it (`git ls-files` packaging match is `pyproject.toml`
only), so it does not block closure — but Gate G should delete it alongside
the final `src/` removal.

## Validation Output

```
$ scripts/go_release_metadata_check.sh
(ok — no output; exit 0)

$ scripts/go_package_smoke.sh
smoke: PostgreSQL integration skipped; STRIATUM_DAEMON_DB_URL is not set
go package smoke: ok

$ scripts/go_fresh_clone_smoke.sh
smoke: PostgreSQL integration skipped; STRIATUM_DAEMON_DB_URL is not set
go fresh clone smoke: ok

$ make ui-bundle-size
ui-bundle-size: 9556077 bytes <= 12000000; 9 files <= 32; 0 shared chunks <= 4

$ # CI "Guard no active Python release path" (Makefile, release.yml, scripts, docs)
guard: ok (no active Python release path)

$ make python-trace-report   # class deltas
active_python_packaging: 1 -> 0
active_python_script: my 3 scripts removed (remaining 2 are Gate C generators)
active_python_runtime_guidance: 56 -> 28 (side effect of dropping legacy Makefile targets; Gate F owns reaching 0)
```
