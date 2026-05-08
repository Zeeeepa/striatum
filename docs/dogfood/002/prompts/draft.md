# Draft prompt — Land RFC 0011 (session close + run-terminal auto-close)

## Task

Implement RFC 0011 end to end. The RFC is the authoritative spec for
this change; everything in this prompt is a pointer at the RFC's
mandatory deliverables.

## Required reading

- `docs/rfcs/0011-session-close-and-run-terminal-auto-close.md` — the
  complete proposal. The "Proposal", "Acceptance Criteria", and
  "Implementation Notes" sections are load-bearing.
- `src/striatum/migrations.py` — current LATEST_VERSION is 6. v7 is
  yours.
- `src/striatum/cli/mutations.py` — `register_session` is the model for
  how new commands assemble a transaction + event.
- `src/striatum/supervisor.py::supervise_stop` — the idempotency
  pattern the new `session close` command should mirror
  (`_latest_terminal_supervisor` returns the existing terminal row;
  the public function returns it with a `note` instead of raising).
- `src/striatum/cli/introspect.py::doctor` — the
  `active_session_on_terminal_run` check is what auto-close must
  resolve.
- `src/striatum/cli/parser.py` — register new `session` subcommand
  group with `close` as its first member.
- `src/striatum/cli/dispatch.py` — wire it.
- `src/striatum/cli/evidence.py` and `cli/run_summary.py` — add
  per-session rendering.

## What to implement

### 1. Migration version 7

Add `_apply_v7_session_close` to `src/striatum/migrations.py`. The
SQLite CHECK on `sessions.state` must be rebuilt to add `'closed'`.
Use the same `CREATE TABLE → INSERT … SELECT *, NULL, NULL → DROP →
RENAME` pattern as v5 used for `artifact_kind`. Add the columns
`closed_at TEXT` and `close_reason TEXT` (both NULL on existing
rows). Append the migration to `MIGRATIONS`. `LATEST_VERSION`
becomes 7 by reflection on the list — do not hardcode.

### 2. `striatum session close` CLI

Add a top-level `session` command group in
`src/striatum/cli/parser.py` and dispatch in
`src/striatum/cli/dispatch.py`. The first subcommand is `close`:

- `--session-id` (required)
- `--reason` (required, must be non-empty after stripping whitespace)
- `--json` (optional)

Implementation lives in `src/striatum/cli/mutations.py` as
`close_session(conn, *, session_id, reason)`. Behavior:

- `NotFoundError` (exit 5) when the session does not exist.
- `LeaseError` (exit 4) when `state == 'active'` and the session
  has any active lease (`leases.owner_session_id = ? AND state =
  'active'`). Message must point the operator at `striatum release`.
- Idempotent against `state IN ('closed','expired','stopped','lost')`:
  return the existing terminal row plus
  `note: "session was already <state>"`. Mirrors HARNESS-001's
  `supervise_stop` pattern.
- On the happy path: `UPDATE sessions SET state='closed',
  closed_at=?, close_reason=? WHERE session_id=?`.
- Emit a `session.closed` event with payload
  `{session_id, role_id, lane_id, reason, source: "explicit"}`.
- Return
  `{session_id, state: "closed", closed_at, close_reason, note?: ...}`.

### 3. Run-terminal auto-close helper

Add `close_remaining_sessions(conn, run_id, *, source, reason)` in
`src/striatum/cli/mutations.py` (or a focused new module if that
file gets unwieldy). It:

- Selects all sessions on `run_id` whose `state == 'active'`.
- Skips any session that holds an active lease (the run reaching a
  terminal state with held leases is itself anomalous; the existing
  recovery path handles that, and auto-close should not paper over
  it).
- Otherwise transitions each remaining session to `closed`, with
  `closed_at = utc_now()` and `close_reason = reason`, emitting a
  `session.closed` event whose payload includes
  `source` (one of `"run_completed"`, `"run_failed"`,
  `"run_canceled"`, `"explicit"`).

Call sites — every path that transitions a run to a terminal state
must call this helper inside the same transaction:

1. `complete_job` and `submit_review` whichever path resolves the
   final non-terminal job and flips the run to `completed`.
2. `recovery cancel-job` when the cancellation is the last open job
   on the run.
3. The `failed` transition path (currently driven by
   `recovery cancel-job` with a different reason). If a future
   `recovery fail-run` ever exists, it should also call the helper —
   leave a TODO comment if so.

Look for the `runs.state = 'completed'` UPDATE statements to find
the transition points; thread `close_remaining_sessions` after each.

### 4. Evidence + run summary

Add `evidence_session_summaries(conn, *, run_id)` to
`src/striatum/cli/evidence.py` (parallel shape to
`evidence_artifact_summaries`). For each session: `session_id`,
`role_id`, `lane_id`, `slug`, `state`, `closed_at`, `close_reason`,
`non_fresh_reason`. Add a `"sessions"` key to the evidence snapshot.

Add a `## Sessions` section in `cli/run_summary.py` that lists each
session as one line: ``- `<slug>` `<state>` (closed_at: `<ts>`)
`<reason>`.`` For sessions still active (anomalous but possible), no
`closed_at`/`reason` and a marker that `doctor` will flag them.

### 5. Doctor

`doctor`'s `active_session_on_terminal_run` check is preserved
verbatim. After auto-close it should fire only for genuinely
anomalous cases — sessions stuck in `active` on a terminal run
because they had an active lease that prevented auto-close, or
because of manual SQLite tampering. Acceptance criterion: a clean
end-to-end run results in `doctor ok=true`.

### 6. SPEC + DECISION_LOG + CHANGELOG

- `docs/SPEC.md` Sessions section: add a "Session lifecycle and
  closure" subsection summarizing the new state value, the
  explicit close command, and run-terminal auto-close.
- `docs/DECISION_LOG.md`: leave this for the apply step (the RFC
  becomes accepted only after review).
- `CHANGELOG.md`: top-of-file entry under `### Added` describing
  the new `session close` command, the auto-close behavior,
  migration v7, and the evidence/run-summary surface change.

### 7. Tests

Add a focused test file `tests/test_session_close.py` (or extend
`tests/test_harness_v2_fixes.py` if you prefer to keep all
post-RFC-0011 coverage co-located). Required tests:

1. `test_session_close_explicit_transitions_active_to_closed` —
   register a session, close it with a reason, assert state /
   columns / event.
2. `test_session_close_idempotent_against_already_closed` — close
   twice; second call returns `note` and same row.
3. `test_session_close_refuses_when_active_lease_held` — claim a
   packet (which acquires a lease); call `session close`; expect
   exit 4 and a message pointing at `release`.
4. `test_run_complete_auto_closes_active_sessions` — drive a clean
   draft → review → apply run; after `complete` the run is
   `completed` and every session is `closed`. Assert
   `session.closed` events for each, with `source: "run_completed"`.
5. `test_run_canceled_auto_closes_active_sessions` — exercise
   `recovery cancel-job` such that the run reaches `canceled`;
   assert sessions are `closed` with `source: "run_canceled"`.
6. `test_doctor_no_longer_flags_terminal_run_after_auto_close` —
   doctor `--json` returns `ok: true` after auto-close.
7. `test_evidence_summary_renders_closed_state_and_reason` — run
   summary contains the new `## Sessions` block with the closed
   state and the reason.

## Acceptance

- `make lint typecheck test` clean (current baseline 151 → at least
  158 after the seven new tests).
- `striatum doctor --run-id <run> --json` returns `ok: true` after a
  clean run (no `active_session_on_terminal_run` warning).
- `striatum session close` is idempotent and refuses against an
  active lease.
- `RUN_SUMMARY.md` includes a `## Sessions` block with closed states
  and reasons for every session in the run.
- The dogfood-002 run itself ends with auto-closed sessions and a
  clean doctor — that is the in-the-loop validation.

## Handoff

Write `docs/dogfood/002/DRAFT_HANDOFF.md` summarizing:

- Files changed (paths only).
- Test count before/after.
- Per-acceptance-criterion: shipped vs deferred and rationale.
- Any new harness friction surfaced; cross-link
  `docs/dogfood/002/findings/HARNESS-NNN.md` if filed.

Then publish (`kind: handoff`, `logical_name: draft_handoff`) and
call `striatum complete`.
