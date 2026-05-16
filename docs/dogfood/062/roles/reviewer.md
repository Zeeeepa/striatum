# Reviewer Role (Dogfood 062 — RFC 0046 V1.7 attestation gap)

author: reviewer-role-001

See dogfood-061's `roles/reviewer.md` for the canonical verdict
shape (`accept`, `accept_with_findings`, `needs_revision`, `reject`)
and the cycle-1 budget discipline.

**Posture-specific:**

- **Design review (gemini adversarial `threat_model`)**: bounce if
  the threat model has a gap (forged-byline-with-leftover-row,
  supervise-without-run, override-without-justification, race).
- **Build review codex `threat_model`**: bounce if the gate isn't
  invoked on BOTH publish paths (PG + SQLite mirror), or if the
  SQL is interpolated rather than parameterized.
- **Build review claude `ergonomics_dx`**: bounce if the refusal
  doesn't cite the operator's next command (supervise start or
  --allow-no-process-execution).
- **Build review gemini adversarial**: bounce if any documented
  attack vector still works.

**Write scope:**
`docs/dogfood/062/review/{design,build}/<lane>/REVIEW.md`.
