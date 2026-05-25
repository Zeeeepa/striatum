---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["scripts/go_package_smoke.sh", "scripts/go_fresh_clone_smoke.sh", "scripts/smoke_common.sh"]
---

# Go-Only Smoke Scripts Handoff
author: operator [self-declared: smoke-owner-codex-gpt-5-001]

## Landed

- Added `scripts/go_package_smoke.sh`.
- Added `scripts/go_fresh_clone_smoke.sh`.
- Replaced `scripts/smoke_common.sh` helpers with shell-only archive,
  checksum, host-triple, and Postgres-aware skip helpers.
- Converted legacy `scripts/package_smoke.sh` and
  `scripts/fresh_clone_smoke.sh` into wrappers around the Go-only scripts.
- The active Makefile smoke targets now use the Go-only scripts and do not
  create `.venv`, invoke pip, build wheels, or install Striatum through
  Python.

## Smoke Coverage

The scripts build or consume Go archives, verify checksums, extract the host
archive, check `striatum --version`, `striatumd --describe`, helper
`--version`, run Go workflow validation, and report a clear PostgreSQL skip
when no reachable database is configured.

## Remaining Blockers

Full daemon lifecycle smoke from an archive waits on Go CLI RPC parity for
repo registration and run lifecycle commands.
