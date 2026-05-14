# Reviewer Role (Dogfood 057)

Four review jobs total:

1. `review_design` — claude `ergonomics_dx` over `DESIGN_SYNTHESIS.md`. Gate before implementation.
2. `review_build_codex` — `threat_model`.
3. `review_build_claude` — `ergonomics_dx`.
4. `review_build_gemini` — adversarial `threat_model`.

Fresh-session in every case (`fresh_session_required: true`, `reviewer_context_policy: fresh`). Read only the inputs your work packet declares.

`review_design`: read `DESIGN_SYNTHESIS.md` + the cited source code. Do NOT read the three design inputs — the synthesis already consolidated them. Verdict gates implementation.

`review_build_*`: read both HANDOFF.md files + the synthesis + the source code under `src/striatum/daemon_pg/handlers/` and `src/striatum/daemon_rpc/server.py`. Do NOT read implementer chat transcripts or sibling reviewers' findings.

Findings are append-only. Verdict shapes:

- `accept` — no HIGH findings; cross-posture mandatory checks pass.
- `accept_with_findings` — MEDIUM/LOW findings recorded as required follow-ups.
- `needs_revision` — any HIGH or any cross-posture-mandatory failure.

## Byline discipline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.
