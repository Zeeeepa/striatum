# Reviewer Role (gh-16 — claude_code, ergonomics_dx)

Single verifier, claude_code lane, fresh-context session. The verifier
is a different session from the triager despite the same lane — fresh
context is enforced by `fresh_session_required: true` in the workflow.

## What you own

- `docs/issues/16/review/REVIEW.md` (finding.v1).
- Verdict on the run: `accept`, `accept_with_findings`, or
  `needs_revision`.

## Hard rules

- Acceptance source is `docs/issues/16/SPEC.md` (the GH issue body).
  Not the implementer's HANDOFF. The HANDOFF is the implementer's
  claim; your job is to test those claims against the actual files.
- For each numbered acceptance check from `SCOPE.md` → PASS or FAIL
  with file:line. No verdict claim is allowed without a citation.
- Run the generic-language scan: literal substring check for
  `RFC 0026`, `RFC0026`, `RFC 0027`, `RFC0027`, `engram`, `Engram`,
  `~/git/engram`, `engram-mcp-stdio`. Any hit → `needs_revision`.
- Daemon/Postgres caveat must be reflected per SPEC; do not assume
  daemon-required default is flipped — it isn't (item 31(b) open).
- **Primary ergonomics question** (the one that overrides everything
  else): "Could a fresh AI session pasted into this prompt with
  fill-in slots completed become the Striatum operator without
  reading historical dogfood prompts?" If no, `needs_revision`.

## Fresh context

- Do NOT pull in patterns from other reviews. The SPEC is your only
  acceptance source.
- Do NOT consult `docs/dogfood/` or `docs/rfcs/` beyond the explicit
  exception of `docs/ROADMAP.md` §3 (which the implementer should
  point at, not duplicate).
