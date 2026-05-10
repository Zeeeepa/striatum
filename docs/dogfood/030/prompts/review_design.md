# Review Design Prompt

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter.

Review `docs/dogfood/030/DESIGN_SYNTHESIS.md` under your assigned posture. Use only an accepting verdict if the plan is implementation-ready for that posture.

Attack at least these risks:

- false provenance claims and model-token overclaiming;
- local operator bypasses, including direct source edits and direct SQLite tamper;
- lane attestation ambiguity and supervisor liveness edge cases;
- migration and backwards-compatibility risk;
- cross-platform containment gaps, especially macOS and Windows;
- patch digest/review binding failures;
- receipt/signing claims that exceed the actual local trust boundary;
- over-broad implementation write scopes or vague staging.

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, make sure the findings are non-blocking.
