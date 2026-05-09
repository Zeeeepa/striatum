# Design review prompt (devils_advocate posture)

Adversarial review of `docs/dogfood/017/DESIGN_SYNTHESIS.md`.
Sweep:

1. Are the seven templates the right set? Anything missing that would
   leave a fresh DDD-shaped target repo broken?
2. Is the idempotency contract airtight? What if a target repo has a
   `docs/SPEC.md` directory (not file)? A symlink? A read-only file?
3. Is the envelope shape consistent with `--with-skills`? Tooling
   that parses one should parse the other.
4. Does `force=False` (V1) close enough loops, or should V1 also
   ship `--force`?
5. Does the package-data wiring correctly include `.md.tmpl` files
   in the wheel? What if `setuptools` filters them out by extension?
6. Test plan completeness — anything not covered?

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
Deliverable: `docs/dogfood/017/review/design/DESIGN_REVIEW.md`.
