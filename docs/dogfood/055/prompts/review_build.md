# Build Review — RFC 0050 V1.5

Read:
- `docs/dogfood/055/DESIGN_SYNTHESIS.md`
- `docs/dogfood/055/build/HANDOFF.md`
- Source files the HANDOFF cites.
- `docs/design/UI_REWORK.md` §9 V1.5-applicable rows.
- The V1 fix-up: `docs/dogfood/054b/build/HANDOFF.md` (so V1
  invariants are preserved through V1.5).

Posture supplied in your work packet. Write to assigned
`docs/dogfood/055/review/build/<lane>/REVIEW.md`. Front matter
must NOT include `author:`; put byline on title-block line per
v1.41 pattern.

## Required checks (do not relax)

1. V1 non-negotiables still hold: no byline forgery on
   unattested sessions, override rationale prominent every
   surface, `LaneEvidenceChip` muted, no transcript capture, no
   inferred-override, attestation recording-time.
2. V1.5 surfaces use V1 primitives — no redefined components.
3. New partials follow the working v1.41 byline pattern (front
   matter has no `author:`).
4. Doctor per-record recipes name the deterministic CLI verbs
   that close each problem.
5. view_file breadcrumb is heuristic-safe — never wrong-links.

Cite file:line for every finding. Verdict: accept /
accept_with_findings / needs_revision.
