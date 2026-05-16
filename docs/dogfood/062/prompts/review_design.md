# Design review (gemini adversarial `threat_model`)

Read `DESIGN_SYNTHESIS.md` and the three designs. Produce
`docs/dogfood/062/review/design/REVIEW.md` (`finding.v1`,
byline `author: reviewer-gemini-001`, `verdict_intent: …`).

**Bouncing checklist (any one → `needs_revision`):**

1. Threat model has a gap — at least one of: forged byline + leftover
   process_executions row from a prior session that still satisfies
   the SQL; supervise.start that wrote the row but the subprocess
   crashed before running; check-by-session is locked when
   check-by-artifact was demonstrably stronger; operator-override
   path missing the audit reason; multi-session test absent.
2. SQL query is interpolated or unparameterized.
3. Operator-override compatibility statement missing.
4. Acceptance tests not concretely named.

**Write scope:** `docs/dogfood/062/review/design/REVIEW.md`.
