# Synthesis: lock RFC 0018 step 3 design

Pin:

1. **Migration v10.** Exact SQL: `ALTER TABLE verdicts ADD
   COLUMN posture TEXT;` + `CREATE INDEX IF NOT EXISTS
   idx_verdicts_posture ON verdicts(posture);`. Backfill rule:
   existing rows get `posture = 'neutral'`. Idempotency: the
   migration system already enforces forward-only via
   `PRAGMA user_version`.
2. **submit-review hook.** When `record_review_verdict` runs,
   look up the review job's `review_posture` in the workflow
   snapshot; default to `"neutral"` when omitted; INSERT into
   `verdicts.posture`.
3. **Per-surface rendering.** For each of the six surfaces,
   describe exactly what changes:
   - `status --json` adds `verdicts_by_posture` block alongside
     `verdicts` counts.
   - `run summary` Markdown groups per-build verdicts by posture
     when any non-neutral posture exists.
   - `evidence export` includes posture in the redacted
     per-verdict block.
   - `run graph --format json` adds `posture` to each review
     node.
   - Dashboard verdicts panel renders one-line per-posture
     count when any non-neutral posture exists.
   - Web UI job detail shows posture as a chip next to verdict.
4. **Zero-regression contract.** A run with no posture-declaring
   review jobs produces output byte-identical to v1.8.1 across
   all six surfaces.
5. **Test plan.** One test file
   `tests/test_review_postures_introspection.py` covering migration,
   submit-review backfill, and each surface.

Deliverable: `docs/dogfood/018/DESIGN_SYNTHESIS.md`.
