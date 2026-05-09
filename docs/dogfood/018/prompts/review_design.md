# Design review prompt (devils_advocate)

Adversarial review of `docs/dogfood/018/DESIGN_SYNTHESIS.md`.

Sweep:

1. Migration safety — does v10's ALTER TABLE preserve all
   triggers and indices on the `verdicts` table, or do we need
   to rebuild?
2. Backfill rule — `'neutral'` is the right default for existing
   rows, but what about *future* verdicts on workflows that
   declare `review_posture: "neutral"` explicitly? The SQL
   value should be identical to "implicit neutral" so queries
   group correctly.
3. Snapshot lookup — `record_review_verdict` needs to read the
   review job's posture from the workflow snapshot, not the
   live workflow file. Confirm the snapshot is the source.
4. Per-surface zero-regression — for each of the six surfaces,
   is the change additive (new key/block/chip), or does it
   modify an existing one in a way that breaks downstream
   consumers?
5. Web UI rendering — chips need a CSS class; does the existing
   stylesheet have a slot, or does this require new CSS?
6. Dashboard truncation — the RFC notes "≤ 4 postures" on the
   verdicts panel; does the synthesis pin a truncation rule for
   runs with more?
7. Test plan completeness — does each surface have at least one
   asserting test?

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
Deliverable: `docs/dogfood/018/review/design/DESIGN_REVIEW.md`.
