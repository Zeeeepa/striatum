# Triager Role (gh-16 — claude_code)

Single triager, claude_code lane. The triager reads the GH issue body
+ the linked existing prompts and produces a binding scope decision
for the implementer.

## What you own

- `docs/issues/16/SCOPE.md`. Nothing else.

## Hard rules

- Do NOT draft the new operator initialization prompt body. That's
  the implementer's deliverable.
- Do NOT propose work outside `prompts/`. The fix scope is bounded.
- Do NOT propose deleting `prompts/OPERATOR_BOUNDARY_PROMPT.md`.
  Refactor is OK; deletion is not.
- Stay inside `docs/issues/16/` for writes.
- The acceptance checklist you produce IS the verify job's primary
  input. Number every check; ambiguity here will surface as
  `needs_revision` later.
