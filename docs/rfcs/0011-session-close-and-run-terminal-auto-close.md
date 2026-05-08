# RFC 0011: Session Close and Run-Terminal Auto-Close

Status: proposed
Date: 2026-05-08
Context:
`src/striatum/cli/mutations.py` (`register_session`),
`src/striatum/cli/introspect.py` (`doctor`'s
`active_session_on_terminal_run` check),
`src/striatum/migrations.py` (`sessions` table state column,
migration version 6),
`src/striatum/cli/run_summary.py`,
`src/striatum/cli/evidence.py`,
`docs/dogfood/001/EVIDENCE.md` (the dogfood evidence that demonstrated
the gap),
`docs/dogfood/001/SYNTHESIS.md`,
`docs/SPEC.md` (Sessions),
`docs/DECISION_LOG.md` (D011 persistent sessions, D029 fresh context).

## Problem

`sessions` rows are created with `state = 'active'` by
`register-session` and there is no CLI surface that transitions them to
any other state. The schema's `state` column already enumerates
`('active','expired','stopped','lost')` (added in migration v1 baseline)
and existing code paths transition sessions to `expired` only when their
lease is explicitly expired during recovery. A clean, completed run
therefore leaves every session it registered in `active` forever.

`striatum doctor` flags this as `active_session_on_terminal_run`: the
run state is `completed`/`failed`/`canceled` but one or more sessions
are still `active`. The check is correct — it surfaces a real
inconsistency. But there is no command in the CLI that would clear it.
The operator has three options today: leave the warning permanent,
edit SQLite directly (forbidden by AGENTS.md product-boundary rules),
or wait for `expire_leases` to opportunistically reap the session
through some other path.

dogfood-001 hit this on the very first end-to-end run. Both author and
reviewer sessions remained `active` after the run completed.
`docs/dogfood/001/EVIDENCE.md` records `doctor ok=false` with two
`active session on terminal run` warnings. dogfood-001-v2 reproduced
the same outcome. The runbook's "After the session" checklist asks the
operator to commit the redacted evidence; that evidence will permanently
report the run as having unclosed sessions. The synthesis describes
this as a low-grade footgun ("There's no `striatum session close`
command") but no proposal was filed at the time.

The runner currently has no story for "what should happen to sessions
when a run finishes". The runner *does* have a story for
`expire_leases`: stale leases get reaped lazily on the next CLI call.
Sessions are not leases; they are the agent's identity. Forcing the
operator to discover and document the gap on every run is friction.

## Goals

1. Add a stable CLI surface for explicitly closing a session
   (`striatum session close --session-id <id> --reason <text>`).
2. Automatically close any still-active session on a run when that run
   transitions to a terminal state (`completed`, `failed`,
   `canceled`). Auto-close is idempotent and records why each session
   was closed so evidence captures the rationale.
3. Eliminate the `active_session_on_terminal_run` doctor warning by
   construction for normal-finish runs while keeping the warning
   functional for genuinely anomalous states (a session active on a
   run whose terminal-state transition was somehow skipped).
4. Preserve existing CLI behavior — adding the new command and the
   auto-close path must not change semantics of any existing
   `register-session`, `complete`, `submit-review`, `verdict`, or
   recovery flow other than emitting the new event and updating
   `sessions.state`.

## Non-Goals

- **Closing supervisors as part of session close.** Supervisor
  lifecycle is its own concern (RFC 0009, plus HARNESS-001's
  idempotent `supervise stop`). A `session close` does not implicitly
  call `supervise stop`; the operator decides when to terminate the
  OS process. Doctor will still surface a supervisor on a closed
  session if that combination is problematic.
- **Re-opening a closed session.** A closed session is terminal. New
  work in the same role/lane requires a fresh `register-session`. This
  matches the existing behavior of `expired` and `stopped` sessions.
- **Auto-closing on first claim of a different session.** Some
  workflows might want to retire the previous reviewer when a new
  reviewer starts. Out of scope; that's a workflow-level pattern and
  can be layered on later.
- **Bulk close for an arbitrary list of runs.** Auto-close is
  triggered by a single run's terminal-state transition. A "close
  every session on every terminal run on this host" maintenance
  command is plausible follow-up but not required.

## Proposal

### CLI: `striatum session close`

Introduce a new top-level `session` command group whose first
subcommand is `close`:

```
striatum session close --session-id <id> --reason <text> [--json]
```

Semantics:

- Looks up the session by id; refuses (`NotFoundError`, exit 5) if it
  doesn't exist.
- If `state != 'active'` already, returns the existing terminal state
  with a `note: "session was already <state>"`. Idempotent — mirrors
  the HARNESS-001 idempotency pattern that `supervise stop` adopted.
- Otherwise transitions `state` to `closed` (new state value; see
  schema migration below), sets a new `closed_at` column to
  `utc_now()`, and records `reason` in a new `close_reason` column.
- Emits a `session.closed` event with payload
  `{session_id, role_id, lane_id, reason, source: "explicit"}`.
- Returns
  `{session_id, state: "closed", closed_at, close_reason, note?}`.

Refusal cases:

- `state == 'active'` AND the session has an active lease. Refuses
  with `LeaseError` (exit 4) and a message pointing the operator at
  `striatum release` first. This guards against accidentally orphaning
  a packet by closing the session out from under it.

### Auto-close on run-terminal transition

Add a single internal helper, `close_remaining_sessions(conn, run_id,
*, source, reason)`, called from each path that transitions a run to
a terminal state. The helper:

- Selects all sessions on the run whose state is `'active'`.
- For each, sets `state = 'closed'`, `closed_at = utc_now()`,
  `close_reason = reason`.
- Emits a `session.closed` event with payload
  `{session_id, role_id, lane_id, reason, source}` where `source` is
  one of `"run_completed"`, `"run_failed"`, `"run_canceled"`.
- Skips sessions with active leases — the lease itself blocks normal
  termination, so the run shouldn't be transitioning to `completed`
  anyway, but if it is being canceled with leases held, the existing
  `expire_leases`/recovery flow handles that.

Call sites (all today; new sites are responsible for calling the
helper as part of the same transaction that updates `runs.state`):

- `complete` and `submit-review` when the resulting verdict transitions
  the run to `completed`.
- `recovery cancel-job` when canceling the last open job on a run.
- `run` cancellation paths (currently `recovery cancel-job` is the
  only entry).
- The `failed` transition that occurs when an unrecoverable
  `claim-next` retry exhaustion or similar exception escapes.

### Schema migration: version 7

Add migration v7 `sessions_close_columns_and_state`:

```sql
-- 1. Drop the existing CHECK on sessions.state and rebuild with 'closed'.
CREATE TABLE sessions_v7 (
  session_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  role_id TEXT NOT NULL,
  lane_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  ordinal INTEGER NOT NULL,
  capabilities_json TEXT NOT NULL DEFAULT '[]',
  parent_session_id TEXT REFERENCES sessions(session_id),
  first_class INTEGER NOT NULL DEFAULT 1 CHECK (first_class IN (0,1)),
  fresh_context INTEGER NOT NULL DEFAULT 0 CHECK (fresh_context IN (0,1)),
  state TEXT NOT NULL CHECK (state IN ('active','expired','stopped','lost','closed')),
  registered_at TEXT NOT NULL,
  last_heartbeat_at TEXT,
  expires_at TEXT,
  non_fresh_reason TEXT,                 -- added by v6
  closed_at TEXT,                         -- new in v7
  close_reason TEXT,                      -- new in v7
  UNIQUE (run_id, slug),
  UNIQUE (run_id, role_id, lane_id, ordinal)
);
INSERT INTO sessions_v7 SELECT *, NULL, NULL FROM sessions;
DROP TABLE sessions;
ALTER TABLE sessions_v7 RENAME TO sessions;
```

The other allowed values (`expired`, `stopped`, `lost`) are unchanged.
`closed` is the *new* state for the auto-close and explicit-close
flows. Existing rows in those other terminal states are not migrated;
they retain their semantics (lease expiry / OS process gone /
supervisor stopped).

### Doctor & evidence

- `active_session_on_terminal_run` continues to fire when a session
  is genuinely `active` on a `completed`/`failed`/`canceled` run. With
  auto-close in place the warning fires only for genuinely anomalous
  states (transition skipped, manual SQLite editing, partial
  recovery). The check name and id are stable.
- `evidence_session_summaries` (new helper, parallel to
  `evidence_artifact_summaries`) returns each session's terminal
  state plus `closed_at` and `close_reason` so evidence exports can
  render the terminal disposition explicitly. `RUN_SUMMARY.md` gets
  a "Sessions" subsection that lists, per closed session, the slug,
  state, and reason.

### Status next-action

No new next-action. The dogfood synthesis-3 candidate was
`recover_orphan_session` for the doctor warning, but with auto-close
the warning becomes rare enough that adding a stable next-action
string would mostly shadow `inspect_blocker` / `export_run_evidence`.
If the warning fires post-auto-close it is a runner bug or a
sysadmin's manual-edit aftermath, neither of which deserves a stable
operator-facing action name.

## Acceptance Criteria

A v2 run that ends with a clean `accept` (or `accept_with_findings`)
verdict, plus `evidence export` and `run summary`, must produce:

- `doctor --json` returns `ok: true` for the run after the apply job
  completes.
- `RUN_SUMMARY.md` "Sessions" subsection lists every session as
  `closed` with the appropriate `source` reason.
- `EVIDENCE.md` snapshot includes each session's `closed_at` and
  `close_reason`.
- `striatum session close` against an already-closed session is
  idempotent (returns the existing terminal row with a `note`).
- `striatum session close` against an active session with an active
  lease refuses with exit 4 and a message pointing at `release`.

Test coverage:

1. `test_session_close_explicit_transitions_active_to_closed`.
2. `test_session_close_idempotent_against_already_closed`.
3. `test_session_close_refuses_when_active_lease_held`.
4. `test_run_complete_auto_closes_active_sessions`.
5. `test_run_canceled_auto_closes_active_sessions`.
6. `test_doctor_no_longer_flags_terminal_run_after_auto_close`.
7. `test_evidence_summary_renders_closed_state_and_reason`.

## Open Questions

1. **State name: `closed` vs `completed`.** The other terminal session
   states are `expired`/`stopped`/`lost`, all describing how the
   session ended. `closed` reads naturally for "the run is done so
   we are closing this session"; `completed` would parallel the
   run/job state vocabulary but conflate with "completed work".
   This RFC picks `closed`.
2. **`source` enum vs free-form `reason`.** The proposal stores both:
   `source` ("run_completed", "run_failed", "run_canceled",
   "explicit") in the event payload, and `close_reason` in the
   column. The column is NULL when source is one of the auto-close
   variants and the operator has not also called `session close`.
   Alternative: enum-only; harder to extend later. Sticking with
   both is the more conservative choice.
3. **Should explicit `session close` require a non-empty reason?**
   `register-session --force-non-fresh --reason "..."` already
   enforces a non-empty reason (HARNESS-003). For symmetry,
   `session close --reason "..."` should require non-empty too.
   The CLI parser will mark `--reason` `required=True`.
4. **Closing a session whose run is still running.** Allowed by this
   proposal: an operator can close a session mid-run (e.g., to
   retire a reviewer who is being replaced). The next
   `register-session` for the same role/lane gets ordinal+1.
   Doctor does not flag this. If we want to forbid mid-run close
   except via `--force` it adds a flag; not in scope here.
5. **`session.closed` event vs reusing `session.expired`.** Today
   the runner has no `session.expired` event because no path emits
   one. Introducing `session.closed` aligns with the new state
   value. If we ever add `session.expired` it should remain
   distinct (lease-expiry semantic vs run-terminal semantic).

## Implementation Notes

- The migration must be in a single `BEGIN IMMEDIATE` transaction
  (all migrations are; `apply_migrations` already enforces this).
- The SQLite CHECK rebuild requires the
  `CREATE TABLE → INSERT → DROP → RENAME` dance — same shape as
  migration v5's `artifact_kind` open-up. Document this in the
  migration's docstring so future readers know why the table is
  being rebuilt.
- The auto-close call sites should be threaded through a single
  helper to avoid drift. Future paths that transition runs to
  terminal states (e.g., a future `recovery fail-run` command)
  must call the same helper or auto-close will regress.
- Cross-link this RFC from `docs/dogfood/001/SYNTHESIS.md` once
  accepted, so the dogfood-001 historical context is connected to
  the resolution.

## Related RFCs

- RFC 0009 (Long-Lived Process Supervision) introduced the
  supervisor lifecycle; this RFC adopts the same idempotency
  pattern (HARNESS-001's `supervise stop`) for explicit close.
- RFC 0010 (Tool Harness Profiles) governs harness defaults; this
  RFC does not affect them.
- RFC 0006 (SQLite Schema Migration System) governs how migration
  v7 must be packaged.

## Decision Log Touch-Points

If accepted, add a decision log entry (next free `Dnnn`) along the
lines of:

> Accept RFC 0011. Add `closed` to the sessions state vocabulary;
> introduce `striatum session close --session-id --reason`;
> automatically close all active sessions on a run when the run
> transitions to a terminal state; record `closed_at` and
> `close_reason` columns; emit `session.closed` events with
> structured `source` payload. Migration version 7. Doctor
> `active_session_on_terminal_run` is preserved as the residual
> warning for anomalous skipped-transition states.
