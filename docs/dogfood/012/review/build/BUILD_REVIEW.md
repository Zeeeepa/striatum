---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0016 step 3 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Verdict: `accept`

## Pinned contracts (verified)

- **Unicode `fancy` style**: rendered output contains `┌`, `┐`,
  `└`, `┘`, `─`, `│` and the cycle arrow is `╌╌▶`. ✓
- **Fancy fall-back to layered** when slot_width < 14 (4-fan-out
  at width=40). ✓
- **`--graph-orient lr`**: layers render as columns, all layer
  labels appear on the same line. ✓
- **LR fall-back to TB** when column_width < 14 (10-layer chain
  at width=80). ✓
- **Color**: fancy mode emits `\x1b[32m`/`\x1b[0m` around inner
  content; frame stays uniform white across states. ✓
- **No-external-URL invariant**: walks fancy output. ✓
- **`run graph --format ascii --graph-orient lr`**: end-to-end
  CLI path passes. ✓
- **`render_graph_panel` dispatch**: deterministic style+orient
  resolution; style trumps orient when both can't be honored.
  Documented in BUILD_HANDOFF, exercised by the LR fall-back
  test. ✓
- **Tests**: 23 dashboard tests pass (15 baseline + 8 new); full
  suite 280 passing.
- **Lint + typecheck**: clean.
- **Version pins**: pyproject.toml + `__version__` synced at
  1.3.0.

## Notes

- The "color wraps inner content, not the frame" choice is the
  right call. Multi-state layers with state-colored frames
  would be visually noisier than uniform white frames with
  state-colored content; the implementer's note in the handoff
  matches the synthesis.
- LR rendering uses `─→` (fancy) and `->` (plain) consistently.
  Cycle arrows use `╌╌▶` (fancy) and `~~>` (plain). All
  symmetric.
- `_render_graph_lr` works for both fancy and ASCII modes via
  the `fancy: bool` parameter. Single function for both
  orientations of LR is simpler than two functions.
- Step 3 closes RFC 0016's deferred V2 slice. The next time
  someone touches the renderer, they'll be adding a *new*
  feature, not finishing this one.

## Decision

`accept`. Box-drawing characters are portable, fall-back rules
are deterministic, color path stays unchanged, tests cover the
new modes plus their fallbacks. The implementation lands cleanly
on top of v1.2.0.
