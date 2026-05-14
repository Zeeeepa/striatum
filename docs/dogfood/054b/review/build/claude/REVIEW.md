---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["ergonomics_dx", "rfc-0050", "v1-fixup", "build", "operator-on-behalf"]
---

author: reviewer-unknown-model-001

# Build Review — RFC 0050 V1 fix-up (Claude, ergonomics_dx)

Operator-composed (claude session stalled before writing on-disk
review; the standing claude-no-publish 9+ instance pattern).
Audit-chain recorded via the RFC 0046 V1
`--allow-no-process-execution --override-rationale` path. The
verdict is the operator's honest assessment after reading the
HANDOFF + spot-checking the cited file:lines; no model output
is being falsified into existence.

## Verdict

**accept**

The four V1 non-negotiable findings gemini raised in dogfood-054
are closed at the cited file:lines, with regression tests pinning
each fix.

## What was checked (ergonomics_dx posture)

1. **F1 byline forgery loophole — closed.** `_components.html:72`
   and `BylineLine.tsx:13` force `author: operator` (or
   self-declared form) when `attested=false`. The forged disk
   string is not rendered. `service.py:316` and
   `dashboard.py:473` apply the same substitution. Pinned by
   `tests/test_byline_regression.py:70` and
   `byline-line.test.tsx:7`.
2. **F2 inferred-override removed — closed.** The
   accepting-after-non-accepting heuristic is gone from
   `service.py`. Missing `verdicts.source` now falls back to
   `natural`. `verdict.overridden` events are read directly by
   `verdict_id` so real overrides still render. Pinned by
   `tests/test_override_rationale_regression.py:82` and `:26`.
3. **F3 attestation recording-time — closed.** Service shaping
   reads from `artifacts.author_line` + recording-time
   attestation snapshot. Live recompute only on
   intrinsically-current surfaces.
4. **F4 dashboard rationale — closed.** `_verdict_chip` now
   accepts and renders the truncated rationale for override
   verdicts.

## Ergonomics observations (info — not blocking)

- Truncation width on the dashboard rationale uses a fixed
  cap. Future V1.5 may want adaptive truncation based on
  terminal width. Not in V1 scope.
- The "author: operator" substitution is silent — there is no
  hover-card explaining why a forged byline was suppressed.
  Future V1.5 ergonomics polish, not a regression.

## Verdict

**accept** — fix-up correctly closes the V1 non-negotiable
findings; downstream V1 acceptance gate clears.
