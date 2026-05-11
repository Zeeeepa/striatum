# Striatum Run Summary

Run ID: `run_4e95a7c06d1e414cba6765f5045d4d07`
Branch: `striatum/dogfood-034-rfc-0030-0031-rpc-supervision`
Run state: `completed`
Verification: `doctor ok=false`

## Timing

- Created at: `2026-05-11T20:25:29Z`
- Started at: `2026-05-11T20:25:37Z`
- Completed at: `2026-05-11T21:54:39Z`
- Duration: `1h 29m 2s`

## Jobs

- `completed`: 9

## Verdicts

- `review_design_threat` (1 attempts): `accept` [posture: `threat_model`]
- `review_build_threat` (2 attempts): `accept_with_findings` [posture: `threat_model`] after 1x `needs_revision`

## Artifacts

- `handoff` `build_handoff`: `docs/dogfood/034/BUILD_HANDOFF.md` - `author: implementer-codex-gpt-5.5-001`
- `handoff` `build_handoff`: `docs/dogfood/034/BUILD_HANDOFF.md` - `author: implementer-codex-gpt-5.5-001`
- `synthesis` `design_synthesis`: `docs/dogfood/034/DESIGN_SYNTHESIS.md` - `author: designer-codex-gpt-5.5-001`
- `handoff` `claude_code_design`: `docs/dogfood/034/design/claude_code/DESIGN.md` - `author: designer-claude-opus-001`
- `handoff` `codex_design`: `docs/dogfood/034/design/codex/DESIGN.md` - `author: designer-codex-gpt-5.5-001`
- `handoff` `gemini_design`: `docs/dogfood/034/design/gemini/DESIGN.md` - `author: designer-gemini-pro-001`
- `finding` `build_review_threat`: `docs/dogfood/034/review/build/threat/REVIEW.md` - `author: reviewer-claude-opus-002`
- `finding` `build_review_threat`: `docs/dogfood/034/review/build/threat/REVIEW.md` - `author: reviewer-claude-opus-001`
- `finding` `design_review_threat`: `docs/dogfood/034/review/design/threat/REVIEW.md` - `author: reviewer-gemini-pro-001`

## Sessions

- `designer-claude_code-1` `closed` (closed_at: `2026-05-11T21:54:39Z`) reason: `run_completed`
- `designer-gemini-1` `closed` (closed_at: `2026-05-11T21:54:39Z`) reason: `run_completed`
- `designer-codex-1` `closed` (closed_at: `2026-05-11T21:54:39Z`) reason: `run_completed`
- `reviewer-gemini-1` `closed` (closed_at: `2026-05-11T21:54:39Z`) reason: `run_completed`
- `implementer-codex-1` `closed` (closed_at: `2026-05-11T21:54:39Z`) reason: `run_completed`
- `reviewer-claude_code-1` `closed` (closed_at: `2026-05-11T21:54:39Z`) reason: `run_completed`
- `reviewer-claude_code-2` `closed` (closed_at: `2026-05-11T21:54:39Z`) reason: `run_completed`

## Blockers

- No blockers recorded.

## Next Actions

- No deterministic next actions.
