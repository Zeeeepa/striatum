# Build review prompt

## Goal

Adversarial review of the V1 build per RFC 0018 V1 (steps 1+2).
Fresh-context review against the contract in
`docs/dogfood/016/DESIGN_SYNTHESIS.md`.

## Sweep

1. **Validator rejections.** `tests/test_review_postures.py`
   covers: unknown posture, posture-on-non-review-job,
   required-posture-on-non-build-job, empty/bare-prefix
   invalid values?
2. **Packet exposure.** A workflow with
   `review_posture: "security"` produces a packet whose
   `review_policy.posture == "security"` and whose
   `instruction` includes the deterministic posture sentence.
3. **Acceptance gate refusal.** A build with
   `required_review_postures: ["security"]` whose downstream
   review for that posture has *not* reached an accepting
   verdict cannot be `complete`'d (exit code 4).
4. **Acceptance gate accept path.** Same workflow but with the
   security review accepted: `complete` succeeds.
5. **Custom posture.** `custom:my_thing` validates, surfaces
   on the packet, and participates in the gate identically.
6. **Zero regression.** A workflow that omits both fields
   produces packets and verdicts byte-identical to v1.6.0.
7. **Build re-run / attempt.** When a build re-runs after a
   `needs_revision` cycle, the new attempt independently
   needs each required posture to clear (i.e., a stale
   accepting verdict from attempt N doesn't satisfy attempt
   N+1). Confirm this behavior.
8. **Suite health.** `make lint`, `make typecheck`, and
   `make test` all pass.

## Deliverable

`docs/dogfood/016/review/build/BUILD_REVIEW.md` — title block
with `author: <line>`, `date:`, `verdict:`, then a sweep matrix
covering the eight points above. Verdict: `accept` if the gates
are satisfied; `accept_with_findings` if minor items exist;
`needs_revision` if any gate is broken.
