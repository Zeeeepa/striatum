---
schema_version: "striatum.operator_report.v1"
artifact_kind: "operator_report"
---

author: operator

# dogfood-060: RFC 0048 Phase C read-surface PG handlers (operator report)

## Header

- Workflow: `docs/dogfood/060/workflow.json` (9 jobs, single implement track, max parallel: 6).
- RFC: 0048 Phase C — port the 8 read-surface CLI verbs (status, dashboard, list.\*, run.summary, why, doctor, evidence.export, corpus.export) from CLI_ROUTES → `invoke()` → repo-local SQLite to PG handlers under `src/striatum/daemon_pg/handlers/reads/`.
- Branch: `striatum/dogfood-060-rfc-0048-read-handlers` (cut from `main` at the v1.51.0 merge commit).
- Operator session: claude-opus-4-7 driving via Claude Code.
- Operating mode: **legacy SQLite via `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`** for the run itself (same break-glass as 057/058). The output of this run is what removes the break-glass for migrated repos going forward.

## Pre-flight (2026-05-15)

- v1.51.0 landed: accept loop functional; CLI Phase C hook routes mutations through daemon RPC; daemon PG admin bootstrapped; systemd user unit enabled.
- v1.50.0 V1.5 fix-up items still deferred (codex F2-F4 + claude HIGH#1/#2 + schema migration 0006 + doctor --explain) — a separate dogfood handles those.
- The dogfood-058 lesson: dual-track synthesis hit cycle exhaustion on track-boundary conflicts. dogfood-060 locks single implement track. Designer prompt + synth prompt + review_design prompt all enforce this.
- Cycle config tightened: `review_design → synth max_iterations: 1` (one revision only; not three).

## Run state (append below per intervention)

- **2026-05-15**: Initial implementation landed all 12 read handlers under
  `src/striatum/daemon_pg/handlers/reads/` with package registration and
  focused tests. First build review returned `needs_revision` on parity,
  routing, and export-redaction gaps.
- **2026-05-15**: Operator completed the revision directly instead of
  restarting the workflow loop: list filters and `corpus.export` now route
  through daemon RPC; status uses the legacy operator action vocabulary;
  run-summary/evidence exports use the real PG doctor projection; corpus
  run-summary rows redact artifact/session prose before rendering; focused
  regression tests and ruff pass.

## Closing summary

Dogfood-060 landed the RFC 0048 Phase C read-handler port as operator-driven
completion work. The remaining `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` escape is no longer needed for the dogfood-060 read
surfaces on migrated repos: `status`, `dashboard`, `list.*`, `run.summary`,
`why`, `doctor`, `evidence.export`, and `corpus.export` all have native PG
handlers registered and routed where the CLI leg exists.

Verification:

```bash
.venv/bin/pytest tests/daemon_pg/handlers/reads tests/test_cli_daemon_rpc_route.py tests/test_corpus_redaction.py -q
.venv/bin/ruff check src/striatum/cli/daemon_rpc_route.py src/striatum/corpus/redaction.py src/striatum/daemon_pg/handlers/reads tests/test_cli_daemon_rpc_route.py tests/test_corpus_redaction.py tests/daemon_pg/handlers/reads
```
