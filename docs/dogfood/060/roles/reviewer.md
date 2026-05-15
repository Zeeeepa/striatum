# Reviewer Role (Dogfood 060)

Four review jobs:

1. `review_design` — claude `ergonomics_dx`. Verdict gates implementation. Cycle: `max_iterations: 1` (one revision; do NOT allow the 058 pattern of three synth attempts).
2. `review_build_codex` — `threat_model`.
3. `review_build_claude` — `ergonomics_dx`.
4. `review_build_gemini` — adversarial `threat_model`.

Fresh-session in every case (`fresh_session_required: true`, `reviewer_context_policy: fresh`).

`review_design`: read `DESIGN_SYNTHESIS.md` + cited code files + RFC 0048. Do NOT read the three design inputs.

`review_build_*`: read HANDOFF.md + synthesis + source under `daemon_pg/handlers/reads/` + tests. Do NOT read implementer chat transcripts or sibling reviewers' findings.

Verdicts: `accept` / `accept_with_findings` / `needs_revision`.

## Byline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.
