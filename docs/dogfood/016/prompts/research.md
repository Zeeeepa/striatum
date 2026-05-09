# Research prompt: review-job validator + packet shape

## Goal

Map the existing review-job machinery so the V1 patch is surgical:

1. `_validate_review_job_fields` in `src/striatum/workflow.py` — what
   fields it accepts, what it rejects, what error code it raises.
2. `build_packet` in `src/striatum/db.py` — where the
   `review_policy` block is assembled, what currently lives in it.
3. The `complete` mutation path — where today's "downstream
   reviews must accept" gate lives, what data shape it walks.
4. The `verdicts` table schema — where the verdict is recorded,
   what columns exist today.
5. The example fixture(s) that exercise reviewer-independence
   policy (RFC 0002) — what shape they take.

## Deliverable

`docs/dogfood/016/research/POSTURE_SHAPE.md` (~50-150 lines) listing:

- The exact functions / line ranges to touch.
- The validator error codes used today (so V1 reuses them).
- The packet block layout (so V1's `posture` field slots in
  cleanly).
- The downstream-reviews walker pattern (so V1's
  `required_review_postures` gate composes with it).
- Test-file precedents (`tests/test_workflow_validator.py`,
  `tests/test_review_policy.py`, etc. — confirm names).

Stay strictly inside `docs/dogfood/016/research/`. Read-only
elsewhere.
