# Dogfood 005 Source List

Status: scaffolded
Date: 2026-05-08

## Striatum-Side

- `docs/rfcs/0014-process-adapter-completion-guarantees.md` — the
  authoritative design.
- `docs/rfcs/0009-long-lived-process-supervision.md` — the
  long-lived counterpart; this RFC fills the parallel gap on the
  one-shot path.
- `docs/SPEC.md` — § "Adapter Boundary", § "Process Supervision",
  § "Run Lifecycle" / Recovery.
- `docs/DECISION_LOG.md` — D028 (no transcripts), D036 (lazy
  stale-lease expiry), D055 (session close + auto-close).
- [Issue #1](https://github.com/halbritt/striatum/issues/1) —
  the failure surface this RFC addresses.

## Source Files To Inspect

- `src/striatum/process_adapter.py` — `run_process_adapter`,
  `prepare_process_launch`, `mark_process_running`,
  `mark_process_exited`, `mark_process_failed`.
- `src/striatum/db.py` — `process_executions` schema, `blockers`,
  `events` helpers, `transaction`, `insert_event`, blocker insert
  helpers.
- `src/striatum/cli/dispatch.py` — `adapter run` and `recovery`
  subcommand wiring.
- `src/striatum/cli/parser.py` — `adapter run` argparse, recovery
  subcommand argparse.
- `src/striatum/cli/recovery.py` — existing recovery helpers
  (especially the stale-lease pattern from D036).
- `src/striatum/cli/introspect.py` — `doctor`, `status`, `why`.
- `src/striatum/migrations.py` — only relevant if
  `process_executions.state` actually has a CHECK that needs
  loosening; verify this in research before assuming a migration
  is needed.

## Existing Tests To Mirror

- `tests/test_supervise.py` — patterns for testing process
  lifecycle.
- `tests/test_recovery_extended.py` — recovery subcommand patterns.
- `tests/test_cli_mvp.py` — broad CLI test patterns; useful for
  the issue #1 reproduction shape.
- `tests/test_session_close.py` — example of a feature that
  introduced new state values cleanly.

## What To Verify (Not Take At Face Value)

The RFC was written from inspection but not exhaustively
cross-checked. The research job should specifically confirm:

- `process.communicate(payload)` really has no timeout in the
  current implementation (RFC 0014 § Problem step 2 claims this).
- `mark_process_exited` is the only place `process_executions.state`
  is updated for normal exits, so an external kill really does
  bypass the bookkeeping.
- Existing blocker rows (which schema, which `severity`s, what
  `payload_json` looks like today) so the synthesis can pin the
  envelope shape consistently with current conventions.
- Existing event types (`process_adapter.*` namespaces) so the
  new `process_adapter.outputs_missing` event slots in cleanly.
- Whether `recovery` already has subcommands (yes — `requeue-stale`,
  `cancel-job`, `cancel-run`) and what their JSON shapes look like.
