# Synthesis prompt: lock RFC 0018 V1 design

## Goal

Pin the RFC 0018 V1 contract (steps 1+2; step 3 deferred per the
RFC's own implementation path). The build phase implements
*exactly* what this synthesis says.

## Deliverable

`docs/dogfood/016/DESIGN_SYNTHESIS.md` covering:

1. **Posture vocabulary.** The closed V1 set (9 postures) plus
   the `custom:<name>` grammar. Cite the RFC's table verbatim.
2. **`POSTURE_INSTRUCTIONS` table.** One sentence per first-class
   posture. `custom:` postures get no auto-appended sentence.
3. **Validator rules (step 1).**
   - Reject `review_posture` on non-`review` jobs.
   - Reject unknown values that are neither in the closed set
     nor `custom:<non-empty>`.
   - Reject empty string and bare `custom:` prefix.
   - All rejections use `WorkflowError` exit code 8 (existing).
4. **Packet exposure (step 1).**
   - `review_policy.posture` set on the packet when posture
     declared.
   - `review_policy.instruction` augmented with the
     `POSTURE_INSTRUCTIONS` sentence when posture is non-neutral
     and first-class.
5. **`required_review_postures` validator (step 2).**
   - Reject on non-`build` jobs.
   - Reject entries whose name is neither in the closed set nor
     `custom:<non-empty>`.
   - Empty list is rejected (defeats the field's purpose).
6. **Acceptance gate (step 2).**
   - In the `complete` mutation path, when a build has
     `required_review_postures`, walk downstream review jobs.
     Each review's posture is its `review_posture` (or
     `"neutral"` when omitted).
   - For each required posture, require at least one
     downstream review whose posture matches AND whose latest
     verdict for the build's current attempt is `accept` or
     `accept_with_findings`.
   - If any required posture is unsatisfied, refuse `complete`
     with `InvalidTransitionError` (exit code 4) naming the
     missing postures.
   - The error message format is deterministic so tests can
     assert against it.
7. **Test plan.** Enumerate the test cases in
   `tests/test_review_postures.py`. Cover RFC 0018's V1
   acceptance criteria minus step-3-only items.
8. **Out-of-scope confirmation.** Step 3 (`verdicts.posture`
   column + introspection surfaces) is explicitly deferred.

Stay inside `docs/dogfood/016/`. Cite line numbers from
research/POSTURE_SHAPE.md as the source of truth for current
shape.
