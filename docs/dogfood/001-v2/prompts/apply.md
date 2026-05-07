# Apply prompt — finalize HARNESS fixes after review

## Task

The reviewer has accepted (with or without findings). Apply any
accepted findings, run the full check suite, and produce a final
handoff.

## Context to read

- `docs/dogfood/001-v2/review/FINDING.md` — reviewer's verdict and
  per-HARNESS gate disposition.
- `docs/dogfood/001-v2/DRAFT_HANDOFF.md` — your earlier handoff.
- The current state of the files you modified in the draft step.

## What to do

1. **Apply accepted findings.** For each finding the reviewer marked
   as blocking or important, make the change. Items the reviewer
   left as info-only can be deferred — note them in the apply
   handoff.
2. **Run the full suite.**
   ```bash
   make lint typecheck test
   ```
   Must pass cleanly.
3. **Verify the new functionality manually:**
   - Set up a tmp repo with a `process_supervisors` row in state
     `lost` plus an active lease, confirm `striatum doctor
     --verbose` reports `supervisor_lost_with_held_lease` and
     `striatum status --json` surfaces a `next_action`.
   - Run `striatum supervise stop --session-id <S>` against the
     same lost supervisor; confirm exit 0 with the new note.
   - Move the editable install temporarily outside the repo (e.g.,
     `pip install -e /tmp/striatum-clone` against a clone), confirm
     the new `editable_install_outside_repo` warning fires, then
     re-install from the repo root.
   - In a fresh tmp repo, fabricate a higher `LATEST_VERSION` in
     the repo's `src/striatum/migrations.py`, run `striatum init`,
     confirm exit 3 with a clear pointer to `pip install -e`.
   - `striatum register-session --role reviewer` against a workflow
     that declares `reviewer_context_policy: fresh`, without
     `--force-non-fresh`: refusal. With `--force-non-fresh --reason
     "smoke"`: success; verify the reason is on the session row.
   - Publish a finding artifact whose file omits an `author:` line;
     confirm the snapshot records the byline as missing/null, not
     the workflow's declared expected.
4. **Update CHANGELOG.md** with one entry per shipped fix.
5. **Write `docs/dogfood/001-v2/APPLY_HANDOFF.md`** summarizing:
   - Final test count.
   - Per-HARNESS: each accepted-with-findings item addressed (or
     explicitly deferred with rationale).
   - Manual verification results (above).
   - Anything else discovered while applying.
   - Friction surfaces hit during review→apply (cross-link any
     `docs/dogfood/001-v2/findings/HARNESS-NNN.md`).

## Handoff

Publish the apply handoff (kind `handoff`, logical_name
`apply_handoff`) and call `striatum complete`. The runner will close
the run when this is the last non-terminal job.

## After completion

The operator will run:

```bash
striatum evidence export --run-id "$RUN_ID" \
  --path docs/dogfood/001-v2/EVIDENCE.md --json
striatum run summary --run-id "$RUN_ID" \
  --path docs/dogfood/001-v2/RUN_SUMMARY.md --json
```

and commit + tag the snapshot (`git tag dogfood-001-v2 -m "harness
fixes round 1"`).
