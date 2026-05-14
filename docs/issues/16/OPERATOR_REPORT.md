# GH #16 Operator Report

**Issue:** [#16 — Prompt: add complete operator initialization prompt](https://github.com/halbritt/striatum/issues/16)
**Run:** `run_<id>` (fill at kickoff)
**Branch:** `striatum/gh-16-operator-init-prompt`
**Workflow type:** `docs/issues/<N>/` (lightweight, 3 jobs: triage → fix → verify)

## Scope

Add `prompts/OPERATOR_INITIALIZATION_PROMPT.md` — a complete, reusable,
generic operator initialization prompt with a fill-in block, behavior
rules, recovery rules, and first-action sequence — sufficient that a
fresh AI session can become the Striatum operator without reading
historical dogfood prompts. Index it in `prompts/README.md`. Decide
whether the existing `OPERATOR_BOUNDARY_PROMPT.md` stays as-is, gets
trimmed to focused guardrail, or refactors so the new prompt reuses
its boundary section.

Captured spec: `docs/issues/16/SPEC.md` (verbatim from GH #16, dated
2026-05-14).

## Lane plan

- **triage** — claude_code, ergonomics-leaning (read spec carefully,
  bound scope).
- **fix** — codex (precise text production).
- **verify** — claude_code, fresh session, ergonomics_dx posture.

Avoids same-lane co-blindness (D095-D100 codex/codex precedents; same
risk on claude/claude).

## Interventions

_TBD — append per intervention, not only at end (per memory
`feedback_operator_report_incremental`)._

## Run Outcome

_TBD_

## Closure

When the verifier accepts and the run reaches `completed`:

```bash
git add prompts/OPERATOR_INITIALIZATION_PROMPT.md prompts/README.md \
        docs/issues/16/
git commit -m "gh-16: add operator initialization prompt"
git push origin striatum/gh-16-operator-init-prompt
gh pr create  # or merge directly to main if branch policy allows
gh issue close 16 --comment "Closed by run_<id>; see docs/issues/16/HANDOFF.md and prompts/OPERATOR_INITIALIZATION_PROMPT.md"
```
