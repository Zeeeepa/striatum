# Build review prompt (devils_advocate)

Adversarial review of the V1.5 build.

Sweep:

1. Migration v10 lands cleanly on a v9 database. Existing
   verdict rows have `posture = 'neutral'`.
2. New verdicts (post-migration) carry the review job's actual
   posture (or `'neutral'` when omitted).
3. `status --json` `verdicts_by_posture` shape is correct.
4. `run summary` Markdown renders per-posture only when at least
   one non-neutral posture exists in the run.
5. `evidence export` includes posture in the per-verdict block.
6. `run graph --format json` review nodes carry posture.
7. Dashboard verdicts panel renders per-posture line when
   non-neutral postures exist.
8. Web UI shows posture chip.
9. Zero regression: a posture-omitting run produces output
   byte-identical to v1.8.1 across all six surfaces.
10. Suite health: lint, typecheck, full test pass.

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
Deliverable: `docs/dogfood/018/review/build/BUILD_REVIEW.md`.
