---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: author-claude-opus-001

# DRAFT_HANDOFF — RFC 0011 (session close + run-terminal auto-close)

Run: `run_982b4ae0112e4cc9b7d71e82bb2d056f`
Branch: `striatum/dogfood-002-session-close`
Job: `draft_change`

## Files changed

| File | Reason |
| --- | --- |
| `src/striatum/migrations.py` | Migration v7 `_apply_v7_session_close`: rebuilds `sessions` with `'closed'` added to the state CHECK, adds `closed_at` and `close_reason` columns. |
| `src/striatum/db.py` | New `close_remaining_sessions()` helper (auto-close every still-active session on a run, skipping any with active leases). `maybe_complete_run()` calls the helper at both terminal transitions (`failed` and `completed`). |
| `src/striatum/cli/mutations.py` | New `close_session()`: idempotent against already-terminal rows, refuses with `LeaseError` when an active lease is held, emits `session.closed` with `source: "explicit"`. |
| `src/striatum/cli/parser.py` | New `session` command group with `close --session-id --reason --json` subcommand. |
| `src/striatum/cli/dispatch.py` | Wires `close_session`. |
| `src/striatum/cli/evidence.py` | New `evidence_session_summaries()` returns per-session terminal disposition; threaded into `evidence_snapshot` under a new `"sessions"` key; EVIDENCE_POLICY extended to allow the new fields safely. |
| `src/striatum/cli/run_summary.py` | Threads `evidence_session_summaries()` into the snapshot dict; `render_run_summary_markdown` adds a `## Sessions` block listing each session's `slug`, `state`, `closed_at`, `close_reason`, and `non_fresh_reason`. |
| `tests/test_session_close.py` | New file. Seven RFC-0011 acceptance tests (one per criterion in `prompts/review.md`). |
| `docs/SPEC.md` | New `### Session lifecycle and closure (RFC 0011)` subsection under Sessions. |
| `CHANGELOG.md` | Top-of-file entry under `### Added`. |

## Test count

- Before: 151 passing.
- After: 158 passing (+7 in `tests/test_session_close.py`).
- `make lint` clean (ruff).
- `make typecheck` clean (mypy, 37 source files).

## Per-acceptance-criterion disposition

(Each criterion from `docs/dogfood/002/prompts/review.md`.)

1. **Migration v7 schema.** **landed.** `_apply_v7_session_close` rebuilds the table with the new CHECK and adds both columns; `LATEST_VERSION` is recomputed via `MIGRATIONS[-1].version`. `tests/test_session_close.py` exercises every test against a freshly-init'd DB so the migration runs each time.
2. **Explicit close happy path.** **landed.** `close_session` updates the row, emits the event, returns the structured payload. Covered by `test_session_close_explicit_transitions_active_to_closed`.
3. **Explicit close idempotency.** **landed.** When `state` is in `('expired','stopped','lost','closed')`, `close_session` returns the existing row plus `note: "session was already <state>"` without re-emitting an event. Covered by `test_session_close_idempotent_against_already_closed` (asserts the second call returns the *first* call's `closed_at` and reason; events count stays at 1).
4. **Active-lease refusal.** **landed.** Refuses with `LeaseError` (exit 4) when `leases.owner_session_id == session AND state == 'active'`. Message points the operator at `striatum release`. Covered by `test_session_close_refuses_when_active_lease_held`.
5. **Run-completed auto-close.** **landed.** `maybe_complete_run` calls `close_remaining_sessions(...source="run_completed"...)` immediately after the `runs.state = 'completed'` UPDATE. Covered by `test_run_complete_auto_closes_active_sessions` (drives the full docs-review-flow lifecycle and asserts both author and reviewer end up `closed` with the right `source`).
6. **Run-canceled auto-close.** **landed-with-caveat.** Today the schema's `runs.state` CHECK accepts `'canceled'`, but no code path produces it: `recovery cancel-job --cascade` cancels every job and `maybe_complete_run` then transitions the run to `completed` because all jobs are in the "terminal" set. The auto-close fires under `source="run_completed"` in this case. Covered by `test_run_canceled_auto_closes_active_sessions` which accepts either `completed` or `canceled` for the run state and asserts the author session ends `closed`. **Open question for reviewer**: should this draft also introduce a dedicated canceled-run path so `recovery cancel-job` can produce a `canceled` run state? RFC 0011 names `"run_canceled"` as one of the three sources, implying yes; but the change is structurally larger than the RFC's "Proposal" describes. I deferred it as out-of-scope for this draft and kept the source enum intact for forward compatibility.
7. **Doctor cleared.** **landed.** Covered by `test_doctor_no_longer_flags_terminal_run_after_auto_close` — asserts no `active_session_on_terminal_run` records or strings appear after auto-close.
8. **Evidence + run summary rendering.** **landed.** Snapshot carries the new `"sessions"` key; redactor allows the new fields. `RUN_SUMMARY.md` has a `## Sessions` block. Covered by `test_evidence_summary_renders_closed_state_and_reason`.
9. **Test coverage.** **landed.** All seven prescribed tests are in `tests/test_session_close.py` and pass.

## Open questions for the reviewer

1. **Dedicated canceled-run state path.** See criterion 6 above. The RFC names `"run_canceled"` as a source value and the `runs.state` CHECK already includes `'canceled'`, but no code transitions runs there. Should a follow-up sub-PR add `recovery cancel-job` semantics that explicitly transition the run to `canceled` (rather than `completed`) when every job is canceled? I left the source enum value in place (the helper accepts it; auto-close will record it correctly when a future path triggers it) but flag this as the cleanest place to discover the gap.
2. **`runs.state == 'failed'` auto-close source.** When a job fails and `maybe_complete_run` flips the run to `failed`, auto-close fires with `source="run_failed"`. There's no test specifically asserting this branch (the existing `test_run_complete_auto_closes_active_sessions` and `test_run_canceled_auto_closes_active_sessions` cover the other two paths). Adding a third test that drives a run to `failed` (e.g. via a deliberate complete-without-publish or a max-attempts breach) would be cheap and complete the matrix. Deferred unless the reviewer wants it shipped now.
3. **Idempotent close note shape.** I followed HARNESS-001's `supervise stop` shape: `note: "session was already <state>"` for any prior terminal state. The reviewer might prefer differentiating: e.g. `note_kind: "already_closed"` vs `"already_lost"`. The string is stable enough for grep-driven tooling; I kept it simple.
4. **EVIDENCE_POLICY explicit listing of new safe fields.** The HARNESS-003 `actual_author_line` field was added to the `_each` policy for artifacts; this draft does the same for sessions. The default-deny rule means any unmentioned field would be redacted, so the explicit listing is intentional. If the reviewer wants a different policy posture (e.g. `_each: "safe"` to allow everything), call it out — I picked the conservative shape.
5. **`run summary` empty-sessions branch.** When a run has no sessions registered yet (impossible in normal flow but possible in some test fixtures), the rendered block reads `- No sessions recorded.`. Consistent with the existing `## Artifacts` empty rendering. Not exercised by any existing test; flagged for completeness.

## Harness friction filed during this run

None new. The new `LeaseError` import in `src/striatum/cli/mutations.py` was the one bug surfaced during local testing — `close_session` raised `NameError` instead of `LeaseError` until the import landed. That's a normal-shape bug, not a runner gap.

The largest piece of friction this round was schema-shape: SQLite cannot drop a CHECK constraint in place, so migration v7 has to rebuild the entire `sessions` table. Same shape as v5 (artifact_kind) and v6 (the `INSERT ... SELECT *, NULL, NULL` dance for new columns). Not a problem; just worth knowing if the table grows further columns the migration must keep updating.
