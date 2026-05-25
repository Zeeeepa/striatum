---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Python Test Deletion Readiness
author: operator [self-declared: deletion-readiness-codex-gpt-5-001]

## Aggregate Status

`not_ready`

Python test deletion is not ready. The coverage ledger still contains `needs_replacement` and `blocked` rows for live PostgreSQL harness behavior, Go CLI daemon client routes, Go local web service routes, corpus writer/redaction/verify/replay, skills/plugin/scaffold installers, and true clean-clone release validation.

## Remaining Python Test/Package Smoke Traces

- `tests/**/*.py`: 176 Python test/helper files at snapshot time.
- `pyproject.toml`: pytest/ruff/mypy/package metadata still present.
- Python smoke/check scripts: `scripts/package_smoke.sh`, `scripts/fresh_clone_smoke.sh`, `scripts/release_metadata_check.py`, `scripts/check_wheel_size.py`, `scripts/check_ui_bundle_size.py`.
- Python source under `src/striatum/**/*.py` still exists and still backs many pytest rows.

## Replacement Aggregate Validation Command

Current candidate after pytest deletion:

```bash
cd go && go test ./...
(cd ../src/striatum/web/frontend && npm test)
cd ..
scripts/go_release_metadata_check.sh
scripts/go_package_smoke.sh
scripts/go_fresh_clone_smoke.sh
make python-trace-guardrail
```

This is a candidate only; it cannot be declared final until the blocked rows are covered or retired.

## Command Evidence

- `cd go && go test ./cmd/striatum ./pkg/rpc ./pkg/repositories ./pkg/mutations ./pkg/reads` passed.
- `cd go && go test ./...` passed.
- `npm test -- --run src/__tests__/api-client.test.ts` passed from `src/striatum/web/frontend`.
- `scripts/go_release_metadata_check.sh` passed.
- `scripts/go_package_smoke.sh` passed.
- `scripts/go_fresh_clone_smoke.sh` passed after the current script switched to
  a source-tree copy that includes the RFC 0078 release files in this change.

## Deletion Order

1. Land all Go/shell/browser replacements and update the ledger rows to `covered`, `retire`, or `historical_exception`.
2. Replace Python packaging/version/release metadata and make `scripts/go_fresh_clone_smoke.sh` pass from a real clone.
3. Delete or retire Python smoke/check scripts after the Go smoke scripts pass.
4. Delete pytest files under `tests/` by ledger row, starting with retired pytest shims and package markers.
5. Remove pytest/ruff/mypy config from `pyproject.toml`, then remove `pyproject.toml` when packaging is Go-only.
6. Delete Python source only after CLI/web/workflow/artifact/corpus/plugin/skills/scaffold replacements or retirements are complete.
7. Enable the Python-trace guardrail to prevent reintroduction.

## Remaining Risks

- Parallel workers are modifying release, web, and CLI surfaces concurrently; this artifact only claims the files changed in this worker's scope.
- Without a real Go PostgreSQL harness, package-local fakes can miss daemon transaction, migration, and capability-token behavior.
- Web deletion remains blocked until the Go local service route boundary is tracked and tested.
- Corpus deletion remains blocked until redaction and replay verification move to Go or are explicitly retired.
- The current fresh-clone smoke is still source-tree based; it should become a
  strict Git clone/archive smoke after the Go-only release files are tracked.
