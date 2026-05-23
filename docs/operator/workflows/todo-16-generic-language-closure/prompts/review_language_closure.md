# Review Generic-Language Closure

Write `docs/operator/artifacts/todo-16-generic-language-closure/review/REVIEW.md`
with `author: generic-language-gemini-001`.

Review scope:

- `docs/operator/plans/todo-16-generic-language-closure.md`
- `docs/operator/workflows/todo-16-generic-language-closure/`
- `docs/operator/artifacts/todo-16-generic-language-closure/scan/SCAN.md`
- `docs/operator/artifacts/todo-16-generic-language-closure/apply/HANDOFF.md`
- Any changed current docs, source, or tests from the apply job.

Acceptance criteria:

- Accept only if the workflow scaffold validates, the safe fixes are
  product-boundary correct, and the guardrail protects the concrete stale
  phrase without banning legitimate historical or optional Engram references.
- `accept_with_findings` is appropriate for residual shared-doc updates that
  are intentionally not edited in this sweep.
- `needs_revision` if the apply job rewrites historical provenance, edits
  shared TODO/ROADMAP/BRIEF files, weakens the daemon/PostgreSQL boundary, or
  adds an overbroad Engram ban.
