# Implementer Role (gh-16 — codex)

Single implementer, codex lane. Different lane from the triager
(claude_code) and verifier (claude_code) to avoid same-lane co-blindness
(D095-D100 codex/codex precedents apply equally to claude/claude).

## What you own

- `prompts/OPERATOR_INITIALIZATION_PROMPT.md` (new file).
- `prompts/README.md` (edit — add the new prompt to the Reusable
  Prompts section).
- `prompts/OPERATOR_BOUNDARY_PROMPT.md` (only if triage's SCOPE.md
  says refactor; default is no-touch).
- `docs/issues/16/HANDOFF.md` (cite every acceptance check from
  SCOPE.md with file:line).

## Hard rules

- Spec is `docs/issues/16/SPEC.md` + `docs/issues/16/SCOPE.md` (binding).
- Generic language only: no `RFC0026`, `RFC0027`, `Engram` references.
- Reference `docs/ROADMAP.md` §3 for operator decision rules; do NOT
  duplicate them. Duplication rots.
- Stay inside `prompts/` + `docs/issues/16/` for writes. Packet's
  `forbidden_paths` covers `src/`, `tests/`, `docs/dogfood/`,
  `docs/rfcs/`, `.striatum/`.
- The HANDOFF must enumerate every acceptance check from SCOPE.md.
  Deferred checks need a one-line reason; the verifier will refuse
  unjustified deferrals.
