# Implement — RFC 0050 V1.5 fix-up (close gemini's 3 V1.5 provenance findings)

**Spec:** `docs/dogfood/055/review/build/gemini/REVIEW.md` IS the
authoritative spec. Each finding lists file:line + required fix.

**Scope is strict.** Close exactly the 3 findings. No new
features, no V2 scope creep, no refactoring beyond what closing
each finding requires.

**Write scope:** `src/striatum/web/`, `src/striatum/service.py`,
`src/striatum/dashboard.py`, `src/striatum/cli/introspect.py`,
`tests/`, `docs/dogfood/055b/build/`. No writes to
`.striatum/`, `go/`, prior dogfoods.

## The 3 fixes (per gemini REVIEW.md)

### Finding 001 (HIGH) — weak attestation check in artifact rows

- `src/striatum/service.py:278` `_recorded_artifact_attestation_chip`
  applies a regex to `author_line` and declares "attested" if the
  pattern matches. An operator-override publish that wrote a
  model-byline-shaped string into the artifact body produces a
  false-green "attested" chip.
- **Fix per gemini correction:** UI must only claim "attested"
  when `artifacts.attestation_override_rationale IS NULL` AND the
  byline matches the recorded `expected_author_line`. The regex
  alone is insufficient — override-published artifacts always
  carry a non-null override rationale, so the rationale presence
  is the authoritative gate.
- Pin via regression: an artifact published with
  `--allow-no-process-execution --override-rationale "..."` whose
  body matches the model-byline regex MUST NOT render "attested".

### Finding 002 (MEDIUM) — attestation drift on verdicts

- `src/striatum/service.py:683` `_shape_verdict_rows` recomputes
  attestation by querying live session/supervisor state per page
  load. A verdict from a session whose supervisor has since
  closed flips from "attested" to "unattested" — false-negative.
- **Fix per gemini correction:** if recording-time attestation
  state cannot be derived (schema limitation — RFC 0046 V1
  shipped the `attestation_override_rationale` column but no
  `verdict_attestation_state` column), the UI must at minimum
  distinguish "previously attested (session closed)" from
  "never attested." Use the session's current `state` column:
  if session is `closed` or `lost`, render
  `previously_attested` (neutral) rather than `unattested`
  (warning).
- Pin via regression: a fixture verdict from a session whose
  `state = 'closed'` must render `previously_attested`, not
  `unattested`.

### Finding 003 (LOW) — LaneEvidenceChip ignores override rationale

- `src/striatum/service.py:710` hardcodes
  `LaneEvidenceChip = not_yet_correlated` everywhere.
- **Fix per gemini correction:** when
  `artifacts.attestation_override_rationale IS NOT NULL`, render
  `override:<rationale>` (truncated for display) instead of
  `not_yet_correlated`. RFC 0050 §5.9 specifies this as the
  override-aware state.
- Pin via regression: an artifact with a non-null
  `attestation_override_rationale` must render
  `LaneEvidenceChip` in the `override` state with the rationale
  text visible.

## HANDOFF

`docs/dogfood/055b/build/HANDOFF.md`. Front matter MUST NOT
include `author:`. Byline on title-block line. One section per
finding with file:line + which test pins it.
