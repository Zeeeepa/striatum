# Operator Report — Dogfood 065 (Go Daemon Port and SQLite Eradication)

author: operator

**Branch:** `main`
**Workflow:** manual fallback after daemon migration refusal
**Started:** 2026-05-17

## Goal

Synthesize the Codex and Gemini remediation plans from 2026-05-17, turn the
remaining remediation steps into RFCs, scaffold the dogfood record, and execute
unblocked remediation work without waiting for human approval.

Updated operator goal: port the production daemon and surrounding runtime
responsibilities to Go, retire the Python daemon after parity, keep the Python
CLI acceptable as a client, and eliminate SQLite everywhere except bounded
one-way migration/import fixtures until those fixtures are retired.

## Dogfood Preflight

`doctor --first-run` passed: daemon socket, runtime token, Postgres,
repository registration, MCP capability visibility, and a sample read route
were healthy.

Normal workflow state verbs still refuse this checkout:

```text
repo_not_migrated: /home/halbritt/git/striatum has not been migrated to daemon PostgreSQL state
```

Work continues as a manual fallback dogfood on `main`; no direct SQLite or
PostgreSQL surgery is performed.

## Operator Direction

The operator rejected the D105 Python-only production-daemon constraint and
then clarified that the desired end state is not merely language-neutral
eligibility. The desired end state is a Go production daemon, Python CLI/web
clients where useful, Python daemon retirement after parity, and SQLite
eradication from production and compatibility paths.

## Work Log

- 2026-05-17: Read both remediation plans and current architecture/TODO/roadmap
  state.
- 2026-05-17: Recorded normal dogfood runner blocker and selected manual
  fallback process.
- 2026-05-17: Scaffolded RFC 0068-0071 and the remediation synthesis document.
- 2026-05-17: Started the unblocked hardcoded client-version remediation slice
  in parallel.
- 2026-05-17: Revised RFC 0068 from a language-neutral gate to the accepted Go
  production daemon port and SQLite-eradication target.
- 2026-05-17: Began Go freshness and schema-parity implementation by adding
  `supported_schema` to Go daemon self-description, Python launcher stale
  binary refusal, and Go doctor substrate-version parity.
- 2026-05-17: Landed hardcoded daemon-client version removal:
  CLI/day-zero handshakes now use `striatum.__version__`.
- 2026-05-17: Tightened Go freshness checks: `--describe` reports migration
  count, SHA-source verification rejects extra source migrations, the Python
  launcher accepts packaged `_daemongo.find_binary()`, and release CI uses Go
  1.23 to match `go/go.mod`.
- 2026-05-17: Focused verification passed:
  `go test ./pkg/db ./pkg/reads`,
  `.venv/bin/pytest tests/cli/test_daemon_core.py tests/test_cli_daemon_rpc_route.py tests/test_day_zero.py`,
  and ruff on touched Python files.

## Blockers / Human Decisions Recorded

- Normal Striatum dogfood workflow remains blocked in this checkout until an
  operator decides how to handle the repo-local SQLite/PostgreSQL checkpoint
  mismatch recorded in dogfood 064.
- Go production eligibility cannot be claimed until the conformance harness
  passes. One concrete current gap is stale Go daemon binaries: older
  `go/bin/striatumd` builds do not self-report supported schema and may refuse
  current daemon PostgreSQL databases.
