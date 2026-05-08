---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["dogfood-002", "rfc-0011", "session-close"]
---

# Review finding — RFC 0011 implementation

Run: `run_982b4ae0112e4cc9b7d71e82bb2d056f`
Branch: `striatum/dogfood-002-session-close`
Job: `review_change` (lease `lease_1152642578cf45458996a251dfd756d5`)

## Independence note

Same caveat as dogfood-001 v2: this reviewer was registered with
`--force-non-fresh --reason "operator-driven; supervised lane work
deferred to a future RFC"`. The reason is durable on the session row
(`sessions.non_fresh_reason`). Byline omitted on this artifact so
HARNESS-003 byline integrity records `null` and the run summary
renders `author: <missing>`.

## Gate disposition (9 gates from `prompts/review.md`)

### Gate 1: Migration v7 schema

**Pass.** `_apply_v7_session_close` rebuilds the table with the new
CHECK accepting `'closed'` and adds `closed_at` plus `close_reason`
columns. The author hit a real foot-gun mid-implementation: the
sessions table has a self-referential FK
(`parent_session_id REFERENCES sessions(session_id)`), so the
standard rebuild + `DROP TABLE old; RENAME new` pattern fails with
`FOREIGN KEY constraint failed` while `foreign_keys=ON`. The
migration now wraps the rebuild with `PRAGMA foreign_keys = OFF`
... `ON`, and adds `DROP TABLE IF EXISTS sessions_v7` to clean up
partial-state from prior aborted attempts. The docstring documents
both. **F-1 (info)**: a future migration that rebuilds another
table with self-referential FKs will need the same toggle; worth
extracting into a small helper (`migrate_table_rebuild(...)`) or
documenting the pattern in `docs/SPEC.md`'s migration section. Not
in scope for this round.

### Gate 2: Explicit close happy path

**Pass.** `close_session` updates state, columns, emits
`session.closed` with `source: "explicit"`. Returns the structured
payload the RFC describes. Test
`test_session_close_explicit_transitions_active_to_closed` covers
all three (state row, event payload, return value).

### Gate 3: Explicit close idempotency

**Pass.** `_SESSION_TERMINAL_STATES` is the explicit allowlist of
states (`expired`, `stopped`, `lost`, `closed`) that short-circuit
to the idempotent return path. `note: "session was already <state>"`
preserves the original `closed_at` and `close_reason` so a second
call cannot rewrite history. Test
`test_session_close_idempotent_against_already_closed` asserts the
event count stays at 1 — no duplicate emission.

### Gate 4: Active-lease refusal

**Pass.** Refuses with `LeaseError` (exit 4); message includes the
lease id and points the operator at `striatum release`. The author
correctly imported `LeaseError` (it was missing initially and the
draft handoff mentions surfacing the import bug during local
testing — that's a real bug the test suite caught; appropriate
severity). Covered by
`test_session_close_refuses_when_active_lease_held`.

### Gate 5: Run-completed auto-close

**Pass.** `maybe_complete_run` calls `close_remaining_sessions` with
`source="run_completed"` immediately after the
`runs.state = 'completed'` UPDATE. `close_remaining_sessions`
correctly skips sessions with active leases (LEFT JOIN where
`l.lease_id IS NULL`). Test
`test_run_complete_auto_closes_active_sessions` drives the full
docs-review-flow lifecycle and asserts both author and reviewer end
up `closed`, the close_reason is `"run_completed"`, and the event
count contains exactly two `run_completed` sources. Solid.

### Gate 6: Run-canceled auto-close

**Pass with finding (medium).** **F-2**: today the schema's
`runs.state` CHECK accepts `'canceled'` but no code path produces
it. `recovery cancel-job --cascade` cancels every job; then
`maybe_complete_run` transitions the run to `'completed'` because
canceled jobs are in the `('completed','skipped','canceled')`
"terminal" set. The auto-close fires under
`source="run_completed"`, not `"run_canceled"`. The draft handoff
flags this honestly as Open Question 1 and the test
`test_run_canceled_auto_closes_active_sessions` accepts either run
state. **My read**: this is the correct disposition for v2's
scope. Adding a dedicated canceled-run state path is structurally
larger than RFC 0011's "Proposal" section describes; doing it here
would conflate two changes. The correct next step is either a small
follow-up RFC ("Add canceled-run state path") or an extension to
RFC 0011 with a new round of dogfood. Recommend documenting this
gap in DECISION_LOG when RFC 0011 is accepted, so the next operator
who reads it sees the deferred work.

### Gate 7: Doctor cleared

**Pass.** Test `test_doctor_no_longer_flags_terminal_run_after_auto_close`
asserts neither the message string nor the structured record
appears for `active_session_on_terminal_run`. The check itself is
preserved (correctly) so genuinely anomalous skipped-transition
states still fire it.

### Gate 8: Evidence + run summary rendering

**Pass.** `evidence_session_summaries` returns each session's
terminal disposition; threaded into `evidence_snapshot` under a new
`"sessions"` key; EVIDENCE_POLICY explicitly lists the new safe
fields (`closed_at`, `close_reason`, `non_fresh_reason`).
`render_run_summary_markdown` emits a `## Sessions` section listing
`slug`, `state`, `closed_at`, `close_reason`, `non_fresh_reason`.
Empty-sessions branch is handled (`- No sessions recorded.`). Test
`test_evidence_summary_renders_closed_state_and_reason` covers both
artifacts (run summary + evidence export). **F-3 (info)**: the
session block doesn't include `lane_id` in the rendered line. The
slug already encodes role-lane-ordinal so the lane is implied, but
a future renderer might want the lane explicit. Non-blocking.

### Gate 9: Test coverage

**Pass.** All seven prescribed tests are in
`tests/test_session_close.py` and pass. The tests are well-shaped —
each asserts the specific behavior the RFC named, fixtures are
small, no hidden coupling. **F-4 (low)**: a test for the
`source="run_failed"` path is missing. The handoff Open Question 2
notes this. Easy to add (drive a job to `failed` via
`recovery cancel-job` on the only job — though the failed-vs-canceled
disambiguation may not exist in the current schema). Non-blocking
for v2 acceptance; worth filing as a follow-up so the source enum
is fully exercised.

## Cross-cutting findings

- **F-5 (info) — `executescript` transaction-control behaviour.**
  The author found that `PRAGMA foreign_keys` cannot be toggled
  inside a transaction; SQLite's `executescript` implicitly commits
  the prior transaction, so the toggle works. This is fine; the
  migration framework's `apply_migrations` opens a `BEGIN
  IMMEDIATE` and commits on success, but each migration's
  `executescript` runs autocommit. That's why the partial-state
  recovery (`DROP TABLE IF EXISTS sessions_v7`) is needed — a
  failed v7 attempt could not be rolled back as a unit. The
  framework could be tightened (run each migration's script inside
  the parent transaction, requiring migrations to use `execute`
  instead of `executescript`), but that's a framework change
  outside RFC 0011's scope. **No action this round.**
- **F-6 (info) — Open Question 4 (EVIDENCE_POLICY posture).** The
  author asks whether to keep listing safe fields explicitly or to
  flip the policy to `_each: "safe"`. **Keep the explicit listing.**
  Default-deny on a redaction policy is the correct posture; flipping
  to allow-everything would let future schema additions leak through
  without review. The cost (every PR that adds a column also adds an
  EVIDENCE_POLICY line) is real but low.

## Parity / scope / docs check

- **Parity**: RFC 0011's Proposal and Acceptance Criteria sections
  are walked one-to-one in both the implementation and the tests.
  No silent scope expansion; the canceled-run-state-path question
  is flagged as deferred (Gate 6, F-2).
- **Determinism**: `close_remaining_sessions` orders by
  `s.registered_at` for stable event-emission order. New string
  vocabulary (`session.closed` event, `source` values, `note`
  string) is grep-stable.
- **Test coverage**: 7/7 tests present and passing; matrix is
  almost-complete (run_failed branch unexercised; F-4).
- **Scope hygiene**: changes are within the v2 author write_scope.
  Confirmed via `git diff --stat`.
- **Doc currency**: SPEC has the new subsection; CHANGELOG entry is
  a top-of-file `### Added`. RFC 0011 is still `proposed`; it should
  flip to `accepted` in apply.

## Verdict

`accept_with_findings`.

All 9 gates pass. Six findings; the highest-severity is F-2
(canceled-run state path) which the author correctly deferred and
the RFC's Proposal section does not actually mandate fixing. The
others are info-level: a test-matrix gap (F-4 run_failed path), a
small future-proofing concern (F-1 migrate-table-rebuild helper),
and a couple of style notes. None are blockers; none require a
revision cycle. The implementation is on-spec for RFC 0011 as
written; the deferred sub-points should be captured in the
DECISION_LOG entry that apply will add.

The in-the-loop validation (this very dogfood-002 run produces
`doctor ok=true` after the apply job completes via auto-close) will
be the final acceptance check; if it passes, RFC 0011 is shipped.
