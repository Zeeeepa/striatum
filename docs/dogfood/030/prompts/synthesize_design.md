# Synthesize Design Prompt

Produce `docs/dogfood/030/DESIGN_SYNTHESIS.md` with valid `striatum.synthesis.v1` front matter.

Read all three design artifacts and synthesize one implementation plan. The synthesis must explicitly reconcile RFC 0026 and RFC 0027 rather than treating them as unrelated changes.

Required sections:

- accepted implementation scope;
- deferred scope and why it is deferred;
- phased plan with testable milestones;
- schema, migration, CLI/API, artifact, verdict, evidence, status, doctor, web/docs, and fixture changes;
- compatibility and upgrade risks;
- test matrix, including adversarial cases;
- staging plan that avoids overclaiming sealed provenance before containment exists;
- human-decision questions that must be answered before implementation proceeds.

If the designs disagree, choose one path and explain the tradeoff. If a guarantee is advisory, label it advisory.
