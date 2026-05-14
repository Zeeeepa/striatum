# Implement — RFC 0050 V1 fix-up (close gemini's 4 V1 non-negotiable findings)

**Spec:** `docs/dogfood/054/review/build/gemini/REVIEW.md` IS the
authoritative spec. Read it first; each numbered finding lists the
exact file:line and the required fix.

**Scope is strict.** Close *exactly* these 4 findings. No new
features, no V1.5 scope creep, no refactoring beyond what closing
the finding requires.

**Write scope:** `src/striatum/web/`, `src/striatum/service.py`,
`src/striatum/dashboard.py`, `tests/`,
`docs/dogfood/054b/build/`. No writes to `.striatum/`, `go/`,
prior dogfoods.

## The 4 fixes (per gemini REVIEW.md)

### F1 — Byline forgery loophole

- `src/striatum/web/frontend/src/shared/components/BylineLine.tsx`
  and `src/striatum/web/templates/_components.html` `byline_line`
  macro currently render the literal `author_line` from disk
  even for unattested sessions, only adding a CSS class.
- **Fix:** if `attested` is false, the component MUST refuse to
  render a model-author string (`<role>-<model>-<ord>` pattern)
  and instead substitute `author: operator` (or
  `author: operator [self-declared: <label>]` when an operator
  label exists). The forged disk-byline is hidden from the
  rendered output entirely; the CSS class alone is not
  sufficient.
- Pin via regression in `tests/test_byline_regression.py`: a
  fixture artifact whose on-disk `author_line` is a model
  byline AND whose session attestation is unattested MUST
  render `author: operator` in both the Jinja macro and the
  TS component output.

### F2 — Verdict forgery via "inferred override"

- `src/striatum/service.py:328-333` infers `operator_override`
  for any accepting verdict following a non-accepting one when
  `source` column is missing. This falsely attributes natural
  model revision cycles to operator.
- **Fix:** remove the `inferred_override` heuristic entirely.
  If `source` is missing in the verdict row, treat as `natural`
  (the default the schema implies) or `unknown`. The provenance
  badge then renders nothing or `natural`; it never fabricates
  `operator_override`.
- Pin via regression in
  `tests/test_override_rationale_regression.py`: a verdict row
  with `source = NULL` MUST NOT render an override badge.

### F3 — Attestation-drift honesty failure

- `src/striatum/service.py:361` (around `_lane_attestation_chip`)
  recomputes lane attestation from the *current* session /
  supervisor state instead of using the state at the time the
  artifact was recorded.
- **Fix:** the attestation surface for a given artifact must
  read from the artifact row's recording-time evidence:
  `artifacts.author_line` is the literal disk byline at publish
  time (HARNESS-003), and a recording-time attestation snapshot
  exists in `process_supervisors`/`sessions` joined on the
  publishing session/lease. Live-recompute is correct only for
  surfaces that are intrinsically "current session" (e.g.
  session-list page).
- Run-detail / job-detail / artifact-view chips MUST use the
  recording-time path. Update the helper to take an artifact
  row context.

### F4 — Dashboard override-rationale omission

- `src/striatum/dashboard.py:192` (`_verdict_chip`) appends
  ` (override)` for override verdicts but omits the rationale.
- **Fix:** when `source = 'operator_override'`, include the
  rationale in the dashboard chip. Truncate to a reasonable
  width (60–80 chars with ellipsis) so the chip stays compact.
- Pin via regression test in
  `tests/test_override_rationale_regression.py` covering the
  dashboard path: a fixture with an override + rationale must
  render rationale text in the dashboard chip.

## Tools / patterns to use

- `striatum byline --session-id <s> --job-id <j>` for the
  expected byline in the HANDOFF byline line (V1.41).
- `striatum inbox --session-id <s>` for the current packet
  shape (V1.41).
- Existing test scaffolding under `tests/test_byline_regression.py`,
  `tests/test_override_rationale_regression.py` — extend, don't
  rewrite.

## HANDOFF

`docs/dogfood/054b/build/HANDOFF.md`. Front matter MUST NOT
include `author:`. The byline goes on its own title-block line
(per the working v1.41 pattern in dogfood-053 reviews). One
section per F1–F4 with file:line evidence + which test pins
the fix.
