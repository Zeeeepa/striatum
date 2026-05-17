# Design review (codex `threat_model`)

Read `DESIGN_SYNTHESIS.md` and the three designs. Produce
`docs/dogfood/063/review/design/REVIEW.md` (`finding.v1`,
`verdict_intent: …`).

**Bouncing checklist** (see `roles/reviewer.md` § "Design review"):

1. v1 workflows under `docs/dogfood/*` + `examples/*` don't all
   validate post-rename.
2. Dual-name discipline doesn't cover every existing reference
   (cite which is missed).
3. SQL migration proposed for PG state rename (must stay in code).
4. `workflow upgrade` non-idempotent (run-twice produces different
   output).
5. CLI flag NAMES change (only stderr/help text may change).
6. Scope decision on `escalation` artifact-kind not made or not tied to
   the follow-up Phase 5 landing — bounce on ambiguity.

**Write scope:** `docs/dogfood/063/review/design/REVIEW.md`.
