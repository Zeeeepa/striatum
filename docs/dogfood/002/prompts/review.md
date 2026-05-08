# Review prompt — RFC 0011 implementation

## Task

Independently review the RFC 0011 draft change. Access scope is
`artifact_augmented`: read the draft handoff, the modified source,
the new tests, and the source RFC. Do not browse beyond the
declared `context_docs` and the changes the handoff cites.

## Context to read

- `docs/rfcs/0011-session-close-and-run-terminal-auto-close.md` —
  authoritative spec; walk the "Acceptance Criteria" list against
  the code.
- `docs/dogfood/002/DRAFT_HANDOFF.md` — author's per-criterion
  disposition.
- `src/striatum/migrations.py` — migration v7 and `LATEST_VERSION`.
- `src/striatum/cli/mutations.py` — `close_session` and the new
  helper `close_remaining_sessions`.
- `src/striatum/cli/parser.py` and `cli/dispatch.py` — wiring.
- `src/striatum/cli/evidence.py` and `cli/run_summary.py` — sessions
  rendering.
- `tests/test_session_close.py` (or wherever the new tests live) —
  the seven prescribed tests.

## Gates (one per RFC acceptance criterion)

1. **Migration v7 schema.** Confirm `sessions.state` accepts
   `'closed'`; `closed_at` and `close_reason` exist and are NULL on
   migrated rows. `apply_migrations` runs cleanly against an
   already-v6 DB.
2. **Explicit close happy path.** Call `striatum session close`
   against an active session; row state is `closed`, `closed_at`
   matches `utc_now()`, `close_reason` matches the input.
   `session.closed` event is recorded with
   `source: "explicit"`.
3. **Explicit close idempotency.** Second call against the same
   session returns the existing terminal row plus a
   `note: "session was already closed"` and exits 0.
4. **Active-lease refusal.** Closing an active session with an
   active lease exits 4, message points at `striatum release`.
5. **Run-terminal auto-close: completed.** A clean run that
   transitions to `completed` has every active session transitioned
   to `closed` with `source: "run_completed"`. A `session.closed`
   event is recorded for each.
6. **Run-terminal auto-close: canceled.** A run that transitions to
   `canceled` via `recovery cancel-job` has every active session
   transitioned to `closed` with `source: "run_canceled"`.
7. **Doctor cleared.** After auto-close, `striatum doctor --json`
   returns `ok: true` for the run; `active_session_on_terminal_run`
   does not fire.
8. **Evidence + run summary rendering.** `RUN_SUMMARY.md` contains
   a `## Sessions` section with one line per session, listing state,
   `closed_at`, and `close_reason`. `EVIDENCE.md` snapshot includes
   the same fields.
9. **Test coverage.** All seven prescribed tests are present and
   passing. Each test asserts the specific behavior the RFC named.

## Disposition

For each gate, mark **landed**, **partial**, or **deferred** and
explain. Cross-reference the test name and (where helpful) the line
number.

## Verdict choices

- `accept` — every gate passes, all advertised sub-points landed,
  tests cover each criterion.
- `accept_with_findings` — change is mergeable; capture partial or
  deferred sub-points for follow-up.
- `needs_revision` — at least one acceptance criterion fails. The
  workflow declares a one-shot revision cycle.
- `reject` — bundle is structurally wrong (regressed test count,
  broke a previously-passing check, RFC contradicted in code). Use
  sparingly.

## Finding artifact

Write at `docs/dogfood/002/review/FINDING.md` with valid
`striatum.finding.v1` front matter:

```yaml
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["dogfood-002", "rfc-0011", "session-close"]
---
```

Submit:

```bash
striatum submit-review \
  --session-id "$REVIEWER" \
  --job-id "$REVIEW_JOB_ID" \
  --lease-id "$REVIEW_LEASE_ID" \
  --kind finding \
  --logical-name review_finding \
  --path docs/dogfood/002/review/FINDING.md \
  --verdict <verdict> \
  --json
```

If you hit runner friction during review, file a
`harness_improvement_proposal` under
`docs/dogfood/002/review/HARNESS-NNN.md` (within the reviewer's
`write_scope.allowed_paths` per HARNESS-004).
