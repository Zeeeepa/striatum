# Design Review — RFC 0050 V2 synthesis

Posture: ergonomics_dx.

Verify the synthesis tracks UI_REWORK.md V2 scope. Reject if:

- Viewport-locked phase bands / require_attested_lane overlay
  ride along (GH #6 — reactflow ViewportPortal not in 11.11.4).
- V1 / V1.5 work is being redone.
- Modal logic is missing focus trap, ARIA, or sends arbitrary
  payloads.

Write `docs/dogfood/056/review/design/REVIEW.md` with v1 front
matter (NO `author:` in front matter — byline on title-block
line). Verdict: accept / accept_with_findings / needs_revision.
