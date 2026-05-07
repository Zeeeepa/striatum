# Apply prompt — finalize DOT export change after review

## Task

The reviewer has accepted (with or without findings). Apply any
accepted findings, run the full check suite, and produce a final
handoff.

## Context to read

- `docs/dogfood/001/review/FINDING.md` — reviewer's verdict and findings.
- `docs/dogfood/001/DRAFT_HANDOFF.md` — your earlier handoff.
- The current state of the files you modified in the draft step.

## What to do

1. **Apply accepted findings.** For each finding the reviewer marked as
   blocking or important, make the change. Items the reviewer left as
   info-only can be deferred — note them in the apply handoff.
2. **Run the full suite.**
   ```bash
   make lint typecheck test
   ```
   Must pass cleanly.
3. **Verify the new functionality manually:**
   ```bash
   .venv/bin/striatum --repo . workflow graph \
     examples/rfc-ledger-cleanup/workflow.json --format dot
   ```
   Output should start with `digraph`. If Graphviz is installed,
   `dot -Tsvg` should consume it without error.
4. **Update CHANGELOG.md** if you made any further changes during apply.
5. **Write `docs/dogfood/001/APPLY_HANDOFF.md`** summarizing:
   - Final test count.
   - Each finding addressed (or explicitly deferred with rationale).
   - Anything else discovered while applying.
   - Friction surfaces hit during review→apply (cross-link harness
     proposals).

## Handoff

Publish the apply handoff (kind `handoff`, logical_name `apply_handoff`)
and call `striatum complete`. The runner will close the run when this
is the last non-terminal job.

## After completion

The operator (you, the human) will run:

```bash
striatum evidence export --run-id "$RUN_ID" \
  --path docs/dogfood/001/EVIDENCE.md --json
striatum run summary --run-id "$RUN_ID" \
  --path docs/dogfood/001/RUN_SUMMARY.md --json
```

and commit the redacted snapshot.
