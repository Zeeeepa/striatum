# Build Review — RFC 0050 V1 fix-up (dogfood-054b)

**Reference spec:** `docs/dogfood/054/review/build/gemini/REVIEW.md`
(the original adversarial findings).

Read:
- The reference spec above (gemini's 4 V1 non-negotiable findings).
- `docs/dogfood/054b/build/HANDOFF.md`
- The source files the HANDOFF cites.

Posture supplied in your work packet. Write to assigned
`docs/dogfood/054b/review/build/<lane>/REVIEW.md` with v1 finding
front matter. Front matter must NOT include `author:`; put byline
on a title-block line per the working v1.41 pattern.

## Required check matrix

Each fix must close the cited finding completely. Refuse if any
forged path still works.

- **F1 byline forgery** — try injecting a fake model-byline in a
  fixture artifact for an unattested session. The component must
  output `author: operator`, never the forged string. The CSS
  class alone is not sufficient.
- **F2 inferred override** — find the line in `service.py` where
  the heuristic lived (was 328-333). Verify it's gone, and that
  a verdict with `source = NULL` does not produce an override
  badge.
- **F3 attestation-drift** — verify the attestation chip for a
  past artifact uses the recording-time path, not live recompute.
- **F4 dashboard rationale** — confirm `_verdict_chip` renders
  the rationale text for override verdicts.

Cite file:line for every finding. Verdict: accept /
accept_with_findings / needs_revision.
