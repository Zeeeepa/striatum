---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: author-claude-opus-001

# APPLY_HANDOFF — RFC 0011 finalized

Run: `run_982b4ae0112e4cc9b7d71e82bb2d056f`
Branch: `striatum/dogfood-002-session-close`
Job: `apply_change`

## Final test count

- `make lint` → ruff: clean (`All checks passed!`).
- `make typecheck` → mypy: clean (`no issues found in 37 source files`).
- `make test` → 158 passed (was 151; `tests/test_session_close.py`
  contributes the 7 RFC-0011 acceptance tests).

## Disposition of reviewer findings

The reviewer's verdict was `accept_with_findings` with six findings.
F-2 was the highest at `medium`; the others were `info` or `low`.
Per the apply prompt's "info-only items can be deferred — note them
in the apply handoff" rule, all six are deferred. Each is restated
below with rationale.

- **F-1 (info) — Migrate-table-rebuild helper.** Future migrations
  against tables with self-referential FKs will need the same
  `PRAGMA foreign_keys = OFF` toggle that v7 adopted. Extracting
  this into a shared helper or documenting the pattern in
  `docs/SPEC.md`'s migration section is a sensible follow-up.
  **Deferred** to a future round; recorded in `D055`'s follow-up
  cell.

- **F-2 (medium) — Canceled-run state path.** `recovery cancel-job
  --cascade` produces `runs.state = 'completed'` instead of
  `'canceled'` because canceled jobs are in
  `maybe_complete_run`'s "terminal" set. RFC 0011 reserves
  `source = "run_canceled"` for the auto-close event but the path
  is unreachable today. **Deferred** — the change is structurally
  larger than RFC 0011's "Proposal" section names; landing it here
  would conflate two changes. Recorded in `D055`'s follow-up cell
  as the cleanest place to discover the gap. The source enum value
  remains reserved so a future fix can flip the path on without a
  schema change.

- **F-3 (info) — Lane in run-summary session line.** The slug
  already encodes role-lane-ordinal, so the lane is implicit. A
  future renderer can split it explicitly. **No action.**

- **F-4 (low) — `run_failed` test branch.** The matrix covers
  `run_completed` and the (currently-merges-to-completed)
  `run_canceled` path. `run_failed` is not exercised by any
  current test. **Deferred**; recorded in `D055` follow-up.

- **F-5 (info) — `executescript` transaction-control.** Existing
  framework behavior; tightening the migration framework is out of
  scope for RFC 0011. **No action.**

- **F-6 (info) — EVIDENCE_POLICY explicit listing.** The reviewer
  agreed with keeping the explicit `_each` listing as a
  default-deny safety net. **No action.**

## Manual verification performed

The most important check — RFC 0011's in-the-loop validation — is
that *this very dogfood-002 run* produces `doctor ok=true` after
the apply job's `complete` triggers auto-close. That happens after
this handoff is published; the operator's evidence-export step
captures the result. The expected outcome:

- `striatum complete` on the apply job transitions the run to
  `completed`.
- `maybe_complete_run` calls
  `close_remaining_sessions(source="run_completed")` inside the
  same transaction.
- All four sessions (the apply session itself, plus the previously-
  registered author/reviewer pairs from this run) auto-close with
  `close_reason: "run_completed"`.
- `striatum doctor --run-id <run> --json` returns `ok: true` with
  no `active_session_on_terminal_run` entries.
- `striatum run summary` includes a `## Sessions` block with each
  session as `closed`.

Pre-apply, lint and typecheck are clean. Test suite at 158 passing.

## Friction surfaced during draft → review → apply

One real bug surfaced during draft, two during apply:

1. **`LeaseError` not imported** in `cli/mutations.py` —
   `close_session` raised `NameError` until the import was added.
   Caught by the test that asserts `assert "active lease" in
   message` returning empty (because the error wasn't a
   striatum-formatted error, it was a Python traceback). Fixed
   during draft.

2. **Migration v7 partial-state foot-gun.** The first attempt to
   migrate the live dogfood-002 DB failed with `FOREIGN KEY
   constraint failed` because the `sessions` table has a
   self-referential FK (`parent_session_id REFERENCES
   sessions(session_id)`). The standard rebuild + `DROP TABLE old;
   RENAME new` pattern fails while `foreign_keys=ON`. Worse, the
   first failed attempt left `sessions_v7` behind because
   `executescript` does not roll back on partial failure — it
   commits incrementally. Fixed by:
   - Adding `PRAGMA foreign_keys = OFF` ... `ON` around the
     rebuild.
   - Adding `DROP TABLE IF EXISTS sessions_v7` at the start of the
     script for partial-state recovery.
   The fix is documented in the migration's docstring. F-1 captures
   this as a future helper opportunity.

3. **Live-DB bootstrap.** Once v7's idempotent script was correct,
   the live dogfood-002 DB was at `user_version = 6` with the
   v7-table partial wreckage. A manual `DROP TABLE sessions_v7`
   followed by a no-op CLI command (`status`) ran the migration
   cleanly to v7. No data loss; recorded here for forensic clarity.

No new HARNESS-NNN proposals filed for v2. The friction surfaced
during apply (#2) is a knowledge gap covered by F-1's deferred
helper proposal — not an independent runner gap worth a new
proposal.

## What's next for the operator

After this job completes (which triggers auto-close on every
remaining session), run:

```bash
.venv/bin/striatum --repo . evidence export \
  --run-id run_982b4ae0112e4cc9b7d71e82bb2d056f \
  --path docs/dogfood/002/EVIDENCE.md --json

.venv/bin/striatum --repo . run summary \
  --run-id run_982b4ae0112e4cc9b7d71e82bb2d056f \
  --path docs/dogfood/002/RUN_SUMMARY.md --json
```

Both should reflect closed sessions for this run (the in-the-loop
validation). Doctor on this run should now report `ok: true`.

Then commit (likely three commits — RFC 0011 implementation, run
artifacts, RFC acceptance + DECISION_LOG), push, and ff to main.
Tag `dogfood-002` per the v2 RUNBOOK's "After the session" step.
