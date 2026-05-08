---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["dogfood-005", "rfc-0014"]
---

# RFC 0014 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Run: run_833b407118184930b154288684dadbee
Read (fresh, repo-level access):

- the changed source under `src/striatum/`;
- `tests/test_process_adapter.py`;
- `docs/dogfood/005/BUILD_HANDOFF.md`;
- `docs/dogfood/005/DESIGN_SYNTHESIS.md`;
- `docs/dogfood/005/review/design/DESIGN_REVIEW.md`;
- `docs/dogfood/005/decisions/V1_ACCEPTANCE.md`;
- updated SPEC, UBIQUITOUS_LANGUAGE, RFC 0014, RFC index,
  DECISION_LOG, TODO, README, CHANGELOG.

Verdict intent: **accept**.

The build implementation matches the accepted design slice and all
six design-review follow-ups (F1–F6). 209 / 209 tests pass; lint and
typecheck clean. Findings F1–F3 below are informational only.

## Schema And Migrations

- **Migration v8.** Rebuilds `process_executions` with the new
  state CHECK accepting `'timed_out'` and `'lost'` via the
  `rebuild_table` helper. `INSERT INTO ... SELECT` preserves
  existing rows. Idempotent in the sense that re-running on a
  migrated DB is a no-op (the pragma sequence and the
  CREATE/INSERT/DROP/RENAME work regardless of source CHECK
  shape).
- **Migration v9.** `ALTER TABLE blockers ADD COLUMN
  payload_json` guarded by a `PRAGMA table_info` check so fresh
  DBs whose `schema.py` already created the column don't fail
  with `duplicate column name`. Idempotency confirmed by
  `test_migrations_v8_v9_idempotent`.
- **`schema.py` parity.** Both the `blockers.payload_json`
  column and the wider `process_executions.state` CHECK landed
  in the V1 baseline alongside the migrations. F5 (dual-update)
  honored.
- **`PROCESS_SCHEMA_SQL`.** The inline CHECK in
  `src/striatum/process_adapter.py` updated in lockstep with
  the migration. F5 honored.

## Diagnostic Envelope (D028 Compliance)

- `build_diagnostic_envelope` returns exactly the nine documented
  fields; no field carries child stdout/stderr. The dedicated
  regression test
  `test_envelope_contains_no_stdout_stderr` JSON-dumps the
  envelope and asserts neither `"stdout"` nor `"stderr"` appears
  in the serialized form.
- The envelope is recorded as `blockers.payload_json` (a JSON
  string of `json.dumps(envelope, sort_keys=True)`) and embedded
  in the `process_adapter.outputs_missing` event's
  `payload_json`.

## Blocker Reason Vocabulary

- All five reasons used by code:
  `process_outputs_missing`,
  `process_review_verdict_missing`,
  `process_exit_nonzero`,
  `process_timeout_exceeded`,
  `process_lost_with_outputs_missing`. Vocabulary matches
  synthesis § 3.
- `pick_inline_blocker_kind` enforces the exit-nonzero >
  timeout > review-verdict-missing > outputs-missing priority
  ladder.
- Reconciler-path blockers go through
  `evaluate_and_block_after_reconcile`, which uses the dedicated
  `process_lost_with_outputs_missing` kind and skips when an
  open blocker already exists for the job (F1 behaviour).

## Timeout

- `--timeout-seconds <n>` parses cleanly through argparse;
  positive-integer validation is enforced both by `argparse`
  (type=int) and by `run_process_adapter` (raises
  `InvalidTransitionError` on non-positive).
- `process.communicate(payload, timeout=n)` is the actual
  bounded wait. `subprocess.TimeoutExpired` triggers
  `process.terminate()` (SIGTERM); a 5-second `wait` with
  `subprocess.TimeoutExpired` falls back to `process.kill()`
  (SIGKILL). Verified by
  `test_timeout_terminates_and_blocks` (15s test budget for a
  1s timeout against a 30s sleep).
- Lane field validation (`adapter_timeout_seconds`) caps at
  86400 in `workflow.py:_validate_lane_constraints`. Tested by
  `test_workflow_validation_rejects_excessive_timeout`. F3
  honored.
- Lane-default vs CLI-flag precedence verified by
  `test_lane_field_default_used_when_flag_omitted` and
  `test_cli_flag_overrides_lane_default`.

## Liveness Reconciliation

- `recovery process-reconcile` follows the existing
  `requeue-stale` shape: `row_by_id(conn, runs, run_id)` then a
  read of `process_executions.state='running'` then per-row
  `os.kill(pid, 0)`.
- The `_pid_alive` helper handles `ProcessLookupError` (gone),
  `PermissionError` (alive but not ours; treated as still
  running per synthesis § 6), and `OSError` (treated as alive,
  conservative).
- After transitioning a row to `'lost'`, the reconciler calls
  `evaluate_and_block_after_reconcile` to run the same output
  validation; idempotent against an already-blocked job.
- Tested by `test_reconcile_keeps_alive_pid_running` and
  `test_reconcile_transitions_dead_pid_to_lost`. The dead-PID
  test correctly resolves the inline-path blocker before the
  reconcile so the reconciler's blocker has room to land.

## Doctor + Status

- Two new doctor checks registered in `DOCTOR_CHECKS`:
  `process_running_but_pid_gone` and
  `process_running_with_expired_lease`. Tested by
  `test_doctor_flags_pid_gone`.
- `_process_health` helper returns the documented summary
  shape; `next_actions` includes
  `recovery_process_reconcile` when stale_running > 0. The
  helper folds its `next_actions` into the existing `status`
  envelope without breaking existing callers (existing
  `next_actions` keys are preserved). Tested by
  `test_status_process_health_summary`.

## Issue #1 Reproduction Confirmed

`test_issue_one_reproduction` exercises the exact failure shape:
fixture lane with `bash -c 'exit 0'`, required artifact, no
publish. Asserts:

- adapter run returns `state='exited'`, `exit_code=0`;
- job state transitioned to `'blocked'`;
- blocker has `blocker_kind='process_outputs_missing'`;
- envelope's `recovery_commands` includes a
  `recovery process-reconcile` invocation.

Pre-V1 behaviour would have left the job in `running` until
lease expiry; post-V1 the bridge closes deterministically.

## Test, Lint, Typecheck

Independently verified:

- `make test`: 209 / 209 passed in ~157s.
- `make lint`: clean.
- `make typecheck`: clean (41 source files).
- `tests/test_process_adapter.py`: 15 / 15 passed in ~14s.

## Documentation

| Doc | Update | Accurate? |
|---|---|---|
| `docs/SPEC.md` § Process Supervision | new "Single-Shot Process Adapter Completion Guarantees" subsection | yes |
| `docs/UBIQUITOUS_LANGUAGE.md` | "diagnostic envelope" + "process completion validation" entries | yes |
| `docs/rfcs/0014-...md` | status `proposed` → `accepted (V1)` with V1 Implementation Slice | yes |
| `docs/rfcs/README.md` | index entry status flip | yes |
| `docs/DECISION_LOG.md` | D057 row | yes (accurate, comprehensive) |
| `docs/TODO.md` | F5 marked done | yes |
| `README.md` | new "Process Adapter Completion Guarantees (RFC 0014 V1)" subsection | yes |
| `CHANGELOG.md` | Unreleased entry under Added | yes |

## Findings

### F1 (info) — Reproduction fixture path was deferred

**Issue.** Synthesis § 12 sketched
`examples/process-adapter-failure-fixture/workflow.json`. The
implementer used inline temp-path fixtures
(`tests/test_process_adapter.py:_build_workflow`) instead. Both
approaches work; the inline pattern is cleaner because each test
is self-contained.

**Recommendation.** None. Documenting the deviation for future
reviewers who might look for the file.

### F2 (info) — Single event type was kept

**Issue.** Synthesis § 4 chose one event type
(`process_adapter.outputs_missing`) for all five reasons. The
build follows that decision.

**Recommendation.** None. If RFC 0013 web UI work needs
per-kind events later, an additive event-type change is a small
follow-up.

### F3 (info) — `mark_process_lost` opens its own transaction

**Issue.** In `cli/recovery.py:process_reconcile`, the loop
calls `mark_process_lost(conn, ...)` (which opens its own
`with transaction(conn)`) then opens a separate
`with transaction(conn):` block to call
`evaluate_and_block_after_reconcile`. This is two transactions
per row instead of one. SQLite handles it correctly; for V1 the
transaction-per-row cost is acceptable. A V2 pass could
consolidate into one `with transaction(conn):` per iteration.

**Recommendation.** None blocking. Worth a small follow-up
refactor if reconciliation perf ever matters.

## Verdict

**accept.** Build slice is correct, fully tested, and matches
the accepted design plus all six design-review follow-ups.
Findings F1–F3 are informational only; nothing requires
revision before run completion.
