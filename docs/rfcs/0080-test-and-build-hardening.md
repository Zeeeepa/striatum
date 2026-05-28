# RFC 0080: Test And Build Hardening

Status: accepted
Date: 2026-05-25
Accepted: 2026-05-25 (D136; implemented in the rfc-0079-0081-closure run)
author: proposer-claude-opus-4-7-001
Context:
[`RFC 0043`](0043-postgres-as-sole-substrate-and-daemon-required-runtime.md),
[`RFC 0078`](0078-go-only-runtime-and-python-removal.md),
[`docs/DECISION_LOG.md`](../decisions/decision-log.md)

## Problem

RFC 0078 migrated coverage to Go under an explicitly *pragmatic* bar: rows that
were E2E-only, live-PostgreSQL-only, or docs-only were retired with a recorded
reason rather than fully replaced. That was the right call to reach closure,
but it left real gaps that must be closed for the suite to be trustworthy:

- **Live-PostgreSQL tests skip by default.** `pkg/db` (and others) gate on
  `STRIATUM_PG_TEST_URL`; `go test ./...` and CI run with package-local fakes
  only, so daemon transaction, migration, capability-token, and audit-chain
  behavior is not exercised end-to-end.
- **No race/vet/lint gates in CI.** `go vet` and `go test -race` pass locally
  but are not enforced; there is no linter gate.
- **Coverage regressions are invisible.** There is no coverage floor or report.
- **`complete` write-scope guard ergonomics.** The guard counts *pre-existing
  untracked paths* (e.g. a workflow scaffold the lane did not create) instead
  of baseline-diffing against the packet start, producing false blocks that
  force agents to commit out-of-scope files to proceed (observed in the
  two-model-conversation run).

## Goals

- A reusable Go PostgreSQL test harness, used by every live-PG test.
- Live-PG tests RUN in CI against a real ephemeral database.
- Restore behavioral coverage for the RFC 0078 rows retired-not-replaced where
  the behavior is load-bearing (recovery causes, capability denial matrix,
  publish-artifact lane-attestation path, migration invariants).
- Enforce `go vet`, `go test -race`, and a linter in CI and `make`.
- Fix the write-scope guard to baseline-diff so it only flags paths the lane
  actually touched.

## Non-Goals

- Reintroducing pytest or any Python test surface (RFC 0078 / D134 stand).
- 100% coverage targets; the floor is behavioral confidence, not a number.

## Proposal

### 1. `go/pkg/pgtest` harness

A reusable package that provisions an isolated database per test run — a
template-clone or a transaction-per-test rollback against a configured server —
exposing `pgtest.DB(t)` returning a ready `db.Runner` with migrations applied.
It reads `STRIATUM_PG_TEST_URL`; when unset it still `t.Skip`s locally, but CI
always sets it. This replaces the ad-hoc per-package fakes for integration-level
tests while package-local fakes remain for pure unit tests.

### 2. CI database + un-skip

The CI `go` job provisions a PostgreSQL service, creates a test role/db, exports
`STRIATUM_PG_TEST_URL`, and runs the full suite so the live-PG tests execute.
The retired `needs_replacement` rows from the RFC 0078 coverage ledger that
protect load-bearing behavior are reinstated as Go integration tests on this
harness.

### 3. Build hygiene gates

`make check` and CI gain: `go vet ./...`, `go test -race ./...`, and
`golangci-lint run` (vetted, conservative linter set). A coverage report
(`go test -coverprofile`) is produced and surfaced; a modest floor guards the
core packages.

### 4. Write-scope guard baseline-diff

The `complete` guard snapshots the tracked/untracked set at packet claim and
diffs against completion, flagging only paths the lane created or modified
outside `allowed_paths` — not pre-existing untracked paths. This removes the
false block that forces out-of-scope commits.

## Acceptance Criteria

- Live-PG tests execute (not skip) in CI and pass; `go/pkg/pgtest` is the single
  harness they share.
- `go vet`, `go test -race ./...`, and the linter pass in CI and are required.
- The reinstated coverage rows have named Go tests; the coverage report is
  emitted with a floor on core packages.
- The `complete` guard no longer blocks on pre-existing untracked paths; a
  regression test proves it.
- `go test ./...` (with `STRIATUM_PG_TEST_URL`) is green; `-race` is clean.

## Implementation Plan

1. `go/pkg/pgtest` harness + migrate-on-setup.
2. CI Postgres service + `STRIATUM_PG_TEST_URL`; un-skip and port the retired
   load-bearing rows.
3. vet/race/lint/coverage gates in `make` + CI.
4. Write-scope guard baseline-diff + regression test.

## Risks

- A real CI database can be flaky/slow; mitigate with transaction-rollback
  isolation and a tight connection pool.
- Granting broad DB privileges to the test runner can expose latent failures in
  one cascade; sequence the un-skip per package and triage in one pass.

## Open Questions

- Template-clone vs transaction-rollback isolation as the default? (Lean:
  rollback for speed, clone for migration tests.)
- Which linter set — start with `govet,staticcheck,errcheck,ineffassign`?

## Domain Modeling

No workflow-domain change. This RFC hardens the *quality* boundary so "Go-only"
also means "well-tested and lint-clean", closing the coverage debt RFC 0078
deliberately deferred.
