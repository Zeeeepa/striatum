---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Packaging Smoke Migration
author: operator [self-declared: packaging-smoke-codex-gpt-5-001]

## Packaging Smoke Behavior Covered

- Go release metadata injection for `striatum --version`.
- Go daemon version metadata through `striatumd --describe`.
- Go release archive smoke script builds host archive, checks archive contents, validates a workflow with the released CLI, and verifies daemon describe metadata.

## Python Smoke/Check Rows Replaced, Retired, Or Blocked

- Replaces part of `scripts/release_metadata_check.py`.
- Replaces part of `scripts/package_smoke.sh` with `scripts/go_package_smoke.sh`.
- Blocks deletion of `scripts/fresh_clone_smoke.sh` until `scripts/go_fresh_clone_smoke.sh` passes from a real clone.
- Python wheel-size checks can retire only after Go archive size or content checks are accepted.

## Files Changed

- `scripts/go_release_metadata_check.sh`
- `scripts/go_package_smoke.sh` flag-order fix against the current parallel-worker script
- `scripts/go_fresh_clone_smoke.sh` flag-order fix against the current parallel-worker script

## Command Evidence

- `scripts/go_release_metadata_check.sh` passed.
- `scripts/go_package_smoke.sh` passed.
- `scripts/go_fresh_clone_smoke.sh` passed after the current script switched to a source-tree copy that includes the local `VERSION` file.
- Combined validation `scripts/go_release_metadata_check.sh && scripts/go_package_smoke.sh && scripts/go_fresh_clone_smoke.sh` passed.

## Remaining Release Blockers

- Track or otherwise replace `VERSION`; current release archive scripts read `$ROOT/VERSION`, and the current fresh-clone smoke is a source-tree copy smoke rather than a clean Git clone smoke.
- Re-run a true Git-clone smoke after the version-source decision lands.
- The current tracked `.github/workflows/release.yml` is still Python/PyPI-oriented, but parallel workers have modified it; this artifact does not claim those edits.
