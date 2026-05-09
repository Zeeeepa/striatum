# Implement prompt: RFC 0018 V1 (steps 1+2)

## Goal

Implement *exactly* the contract pinned in
`docs/dogfood/016/DESIGN_SYNTHESIS.md`, modulo any
`accept_with_findings` items recorded in
`docs/dogfood/016/review/design/DESIGN_REVIEW.md`.

## Deliverables

1. Code changes:
   - `src/striatum/workflow.py` — `ALLOWED_POSTURES` constant,
     `POSTURE_INSTRUCTIONS` table, `_validate_review_posture`
     helper, `_validate_required_review_postures` helper, and
     wire them into `_validate_review_job_fields` /
     `_validate_build_job_fields` (or equivalents per
     research).
   - `src/striatum/db.py` — extend the `review_policy` block
     in `build_packet` to carry `posture` and the augmented
     `instruction`. Refine the `complete` mutation's
     downstream-review check to gate on
     `required_review_postures`.
   - No SQLite migration in V1. (Step 3 adds
     `verdicts.posture`; deferred.)

2. Tests at `tests/test_review_postures.py` (new file) covering
   the test plan in DESIGN_SYNTHESIS § "Test plan."

3. Docs:
   - `docs/SPEC.md` — § "Reviewer Policy" gains a posture
     subsection; § "Workflow Schema" adds the new fields.
   - `docs/UBIQUITOUS_LANGUAGE.md` — entries for `review
     posture` and `required review postures`.
   - `docs/DECISION_LOG.md` — D069 row for accepting RFC 0018
     V1.
   - `docs/TODO.md` — F16 marked done.
   - `docs/rfcs/0018-focused-adversarial-review-postures.md`
     — status moves to `accepted (V1; step 3 deferred)`.
   - `docs/rfcs/README.md` — index row updated.
   - `CHANGELOG.md` — `## 1.7.0 — 2026-05-09` section.
   - `pyproject.toml` and `src/striatum/__init__.py` — bump
     to `1.7.0`.

4. `docs/dogfood/016/BUILD_HANDOFF.md` listing the file
   changes.

## Constraints

- Stay inside `write_scope.allowed_paths`.
- `make lint` and `make typecheck` must pass.
- Run `make test` and report the result in BUILD_HANDOFF.md.
- Existing tests must continue to pass — RFC 0018 V1 is
  purely additive for posture-omitting workflows.
