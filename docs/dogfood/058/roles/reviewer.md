# Reviewer Role (Dogfood 058)

Four review jobs:

1. `review_design` — claude `ergonomics_dx`; verdict gates implementation.
2. `review_build_codex` — `threat_model`.
3. `review_build_claude` — `ergonomics_dx`.
4. `review_build_gemini` — adversarial `threat_model`.

Fresh-session in every case (`fresh_session_required: true`, `reviewer_context_policy: fresh`).

`review_design`: read `DESIGN_SYNTHESIS.md` + the cited V1 finding files + the source files the synthesis targets. Do NOT read the three design inputs.

`review_build_*`: read both HANDOFF.md files + the synthesis + the source code under `src/striatum/daemon.py`, `daemon_rpc/`, `daemon_pg/handlers/`, `daemon_pg/sql/`, `tests/daemon_*`, `docs/POSTGRES_TRANSITION.md`. Do NOT read other reviewers' findings.

Verdicts: `accept` / `accept_with_findings` / `needs_revision`.

## Byline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.
