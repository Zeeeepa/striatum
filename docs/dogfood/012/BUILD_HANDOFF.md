---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0016 step 3 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: dogfood-012 / RFC 0016 step 3 (Unicode `fancy` style + `--graph-orient`)
Decision: `accepted_with_follow_up` (V1_ACCEPTANCE; autonomous)
Version: `1.3.0`

## Files Changed

- **`src/striatum/dashboard.py`**:
  - `_FANCY_TL/TR/BL/BR/H/V` Unicode box-drawing constants
    (BMP only).
  - `_FANCY_CYCLE_ARROW = "╌╌▶"`, `_FANCY_LR_ARROW = "─→"`.
  - `_format_fancy_box(node, state, box_width, *, color)` —
    returns `(top, mid, bot)` strings; color wraps the inner
    content (not the frame) so the box frame stays uniform
    across states.
  - `_render_graph_fancy(...)` — top-to-bottom Unicode-box
    layered renderer; mirror of `_render_graph_layered`.
  - `_render_graph_lr(...)` — column-major LR renderer; works
    with both fancy and ASCII boxes; uses `─→` (fancy) or `->`
    (plain) as the inter-column arrow.
  - `render_graph_panel(...)` rewritten to dispatch among
    four renderers (`list`, `layered`, `fancy`, `lr`) with
    deterministic style+orient resolution. Fall-back rules:
    `fancy → layered` when slot_width < 14;
    `lr → tb` when column_width < 14;
    `layered → list` when slot_width < 12.
  - `render_frame(...)` accepts `graph_orient: str = "tb"`.
  - `run(...)` accepts `graph_orient: str = "tb"` and threads
    it.
- **`src/striatum/cli/parser.py`**:
  - Dashboard parser gains `--graph-orient {tb, lr}`.
  - `run graph` parser gains `--graph-orient {tb, lr}` and
    `--graph-style {auto, layered, list, fancy}` (the ASCII
    path now honors more than the default).
- **`src/striatum/cli/dispatch.py`**: threads
  `graph_orient`/`graph_style` to both `run_dashboard(...)`
  and `run_graph(...)`.
- **`src/striatum/cli/introspect.py`**: `run_graph(...)`
  accepts `graph_orient` + `graph_style`, passes through to
  `render_graph_panel`.
- **`tests/test_dashboard.py`** (8 new cases, 23 total):
  - `test_render_graph_panel_fancy_uses_box_drawing`
  - `test_render_graph_panel_fancy_falls_back_to_layered_at_narrow_width`
  - `test_render_graph_panel_orient_lr_renders_columns`
  - `test_render_graph_panel_orient_lr_falls_back_to_tb_at_many_layers`
  - `test_render_graph_panel_fancy_color_emits_ansi`
  - `test_render_graph_panel_fancy_no_external_url_invariant`
  - `test_dashboard_graph_orient_flag_is_threaded`
  - `test_run_graph_format_ascii_orient_lr`
- **`docs/SPEC.md`** § Dashboard updated for both new flags.
- **`docs/HOW_TO_HUMAN.md`** § "Dashboards and graphs" updated.
- **`docs/rfcs/0016-dashboard-dependency-graph.md`** status →
  `accepted (V1+step 3)`.
- **`docs/rfcs/README.md`** index reflects `accepted (V1+step 3)`
  + D064 reference.
- **`docs/DECISION_LOG.md`** D064.
- **`docs/TODO.md`** F12.
- **`pyproject.toml`** + **`src/striatum/__init__.py`**: 1.2.0
  → 1.3.0.
- **`CHANGELOG.md`** 1.3.0 section.

## Verification

- `make lint` clean.
- `make typecheck` clean (51 source files).
- `make test` — 280 passed (272 baseline + 8 new).
- Smoke-tested all three new modes against a 3-node, 2-layer
  fixture (TB fancy, LR fancy, LR layered). Cycle arrows
  render correctly in both fancy (`╌╌▶`) and ASCII (`~~>`)
  styles.

## Notes For The Reviewer

- **Color wrapping the inner content, not the frame.** This is
  intentional. If the frame carried color, multi-state layers
  would have visually inconsistent corner glyphs in the same
  row, which is harder to read than uniform white frames with
  state-colored content.
- **Style trumps orient** when both can't be honored. A 10-layer
  workflow at width=80 with `--graph-style fancy --graph-orient
  lr` falls back to layered TB (not fancy LR), because the
  column-width clamp is the binding constraint and we choose to
  drop orient first. The synthesis pinned this.
- **Code-point counting in ANSI-aware truncate.** The existing
  `_truncate` counts visible code points and preserves escape
  sequences. Unicode box-drawing characters are single code
  points (no double-width concerns for the BMP set we use), so
  width math stays simple.
- **Step 3 closes RFC 0016 V2.** RFC 0016's Implementation Path
  listed step 3 as "deferred V2"; this run lands it. The RFC
  is now `accepted (V1+step 3)`.
