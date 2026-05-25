---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0080-test-and-build-hardening.md"]
---

# RFC 0080 Test And Build Hardening Summary
author: implementer-codex-gpt-5-001

## Summary

Implemented the Go live-PostgreSQL test harness and hardened the build gates:

- Added `go/pkg/pgtest` with `pgtest.Pool(t)` for isolated migrated databases and `pgtest.DB(t)` for rollback-scoped runner tests. It reads `STRIATUM_PG_TEST_URL` and skips when unset.
- Moved existing live-PG audit-chain and supervisor-pointer tests onto `pgtest`, and added live-PG coverage for migration invariants, capability denial reasons, and lane-attested artifact author validation.
- CI now provisions PostgreSQL 16, exports `STRIATUM_PG_TEST_URL`, installs `golangci-lint`, and runs `make -C go check`.
- `go/Makefile` now has `vet`, `race`, `lint`, `coverage`, and `check` targets. Core coverage is measured across daemon/runtime packages with a `20.0%` floor.
- Fixed the completion write-scope guard to use packet-claim baselines. Claim records changed-path hashes in `packet_json`; complete compares current hashes to the baseline so pre-existing untracked paths are not reported as lane-created violations.
- Live-PG execution exposed that `SupervisorPointerStore` no longer matched the current migration schema. The store now uses the current `process_supervisor_pointers` columns and stores compatibility fields in `metadata_json`.

## Validation

- `go test ./...` passed with `STRIATUM_PG_TEST_URL` unset.
- `go vet ./...` passed.
- `go build ./...` passed.
- `go test -race ./...` passed with `STRIATUM_PG_TEST_URL` unset.
- `STRIATUM_PG_TEST_URL='postgres:///postgres?host=/var/run/postgresql' go test ./...` passed; live-PG tests ran in `pkg/db`, `pkg/rpc`, and `pkg/mutations`.
- `make -C go coverage` passed: core coverage `45.8% >= 20.0%`.
- `golangci-lint run ./... 2>/dev/null || true` was executed per prompt. `golangci-lint` is not installed locally; CI installs it before `make -C go check`.
