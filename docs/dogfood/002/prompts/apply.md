# Apply prompt — finalize RFC 0011 after review

## Task

The reviewer has accepted (with or without findings). Apply any
accepted findings, mark RFC 0011 accepted, add the DECISION_LOG
entry, run the full check suite, and produce a final handoff.

## Context to read

- `docs/dogfood/002/review/FINDING.md` — verdict + per-gate
  disposition.
- `docs/dogfood/002/DRAFT_HANDOFF.md` — earlier handoff.
- The state of the files modified in draft.

## What to do

1. **Apply accepted findings.** For each finding marked blocking or
   important, make the change. Info/low items can be deferred —
   note them in the apply handoff.
2. **Promote RFC 0011 to accepted.**
   - Edit `docs/rfcs/0011-session-close-and-run-terminal-auto-close.md`:
     change `Status: proposed` to `Status: accepted`.
   - Edit `docs/rfcs/README.md`: update the index row's Status
     column from `proposed` to `accepted`.
3. **Add the DECISION_LOG entry** in `docs/DECISION_LOG.md`. Pick
   the next free `Dnnn` and follow the existing template:
   acceptance summary, rationale, surface area, follow-ups. The
   text in RFC 0011's "Decision Log Touch-Points" section is a
   starting point; refine for tone and current numbering.
4. **Run the full suite.**
   ```bash
   make lint typecheck test
   ```
   Must pass cleanly.
5. **Verify in the dogfood-002 run itself.** The most important
   manual check is whether dogfood-002's *own* run exhibits the
   new behavior:
   - When you `complete` the apply job, the run should transition to
     `completed` and auto-close should fire on every still-active
     session.
   - `striatum doctor --run-id <run> --json` should return
     `ok: true` immediately afterward (no
     `active_session_on_terminal_run`).
   - `striatum run summary` output should include a `## Sessions`
     block showing every session as `closed`, with the appropriate
     `source` reason.

   This is the in-the-loop validation; if it fails, file a
   `harness_improvement_proposal` and decide whether to revise the
   draft (cycle once) or accept-with-findings and capture the
   issue for a follow-up.
6. **CHANGELOG.md** — update with one entry summarizing the shipped
   command + behavior + migration version.
7. **Write `docs/dogfood/002/APPLY_HANDOFF.md`** summarizing:
   - Final test count.
   - Per-finding disposition (addressed / deferred / no-op).
   - Manual verification result (especially the in-the-loop
     auto-close on this run).
   - Anything else surfaced during apply.
   - Friction surfaces hit; cross-link any
     `docs/dogfood/002/findings/HARNESS-NNN.md`.

## Handoff

Publish (`kind: handoff`, `logical_name: apply_handoff`) and call
`striatum complete`. Apply is the last non-terminal job; completing
it transitions the run to `completed` and auto-close should run.

## After completion

Operator runs:

```bash
striatum evidence export --run-id "$RUN_ID" \
  --path docs/dogfood/002/EVIDENCE.md --json
striatum run summary --run-id "$RUN_ID" \
  --path docs/dogfood/002/RUN_SUMMARY.md --json
```

Both should reflect closed sessions. Then commit and tag
`dogfood-002`.
