# Build Review — RFC 0050 V1.5 fix-up (dogfood-055b)

**Reference spec:** `docs/dogfood/055/review/build/gemini/REVIEW.md`
(the original adversarial findings).

Read:
- The reference spec above.
- `docs/dogfood/055b/build/HANDOFF.md`
- The source files the HANDOFF cites.

Posture supplied in your work packet. Write to assigned
`docs/dogfood/055b/review/build/<lane>/REVIEW.md` with v1 finding
front matter. Front matter must NOT include `author:`; byline on
title-block line.

## Required check matrix

1. **Finding 001 (high) byline forgery** — try a fixture artifact
   with `attestation_override_rationale != NULL` AND
   `author_line` matching the model-byline regex. The UI MUST
   NOT render "attested". Override-rationale-present is the
   authoritative gate.
2. **Finding 002 (medium) attestation drift** — a verdict from
   a session whose `state = 'closed'` or `'lost'` must render
   `previously_attested` (neutral), not `unattested` (warning).
3. **Finding 003 (low) LaneEvidenceChip override** — when
   `attestation_override_rationale IS NOT NULL`, chip renders
   `override:<rationale>` not `not_yet_correlated`.

Cite file:line for every finding. Verdict: accept /
accept_with_findings / needs_revision.
