---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0016 step 3 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Verdict: `accept`

## Pinned contracts (verified)

- **`fancy` style**: Unicode box-drawing (`┌`, `┐`, `└`, `┘`, `─`,
  `│`, `╌`, `▶`) — all in BMP, portable across modern terminals.
  Layered fallback fires when `(width-4)/max_layer_size < 14`. ✓
- **`--graph-orient {tb,lr}`**: tb is unchanged default; lr renders
  layers as columns. lr falls back to tb when
  `(width-4)/num_layers < 14`. ✓
- **Style/orient interaction**: style trumps orient when both can't
  be honored — sensible failure mode that keeps fallbacks
  deterministic. ✓
- **Color**: `ANSI_STATE_COLORS` unchanged. Fancy wraps inner
  content (not the box frame), keeping the frame uniform across
  states. ANSI-aware `_truncate` already handles Unicode +
  escape-sequence interleaving. ✓
- **Tests**: 8 new cases cover both new modes plus their fallbacks.
- **`run graph --format ascii --graph-orient lr`**: end-to-end
  CLI path documented and tested. ✓

## Notes

- **Box-drawing in `_truncate`.** Code-point counting is correct
  for this BMP set; no double-width concerns.
- **Cycle arrows in fancy mode** use `╌╌▶` (dashed + triangle);
  fall back to `~~>` when not fancy. Symmetric with how the
  layered/list cycle arrows work today.
- **`--graph-orient` parser placement**: dashboard parser AND
  `run graph` parser both gain it. Same flag name, same choices,
  same default. The `run graph` flag is ignored when `--format`
  is not `ascii`; the synthesis correctly notes this.
- **Step 3 is the deferred slice from RFC 0016 D060**. Once this
  lands, RFC 0016 is fully closed.

## Decision

`accept`. Box-drawing characters are portable, fallback rules are
deterministic, color path is unchanged, tests are scoped right.
The implementation is the next-smallest step on top of v1.2.0.
