---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0016-dashboard-dependency-graph.md", "docs/dogfood/012/research/FANCY_AND_ORIENT.md", "src/striatum/dashboard.py", "tests/test_dashboard.py"]
---

# RFC 0016 step 3 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0016 step 3 (Unicode `fancy`
style + `--graph-orient {tb,lr}`).

## Locked Contracts

### `_render_graph_fancy` (new)

Mirrors `_render_graph_layered` but uses Unicode box-drawing:

- Box top: `┌` + `─` * (inner) + `┐`
- Box body: `│ <content> │`
- Box bottom: `└` + `─` * (inner) + `┘`
- Inter-layer connector: `│` per box, centered (same as layered).
- Cycle arrow (when fancy on, no_cycles off): `╌╌▶` instead of `~~>`.

`box_width` calc unchanged. Inner content budget is
`box_width - 4` (two corners + two padding spaces). Fall-back to
layered fires when `(width - 4) // max_layer_size < 14` (was 12;
fancy needs 2 more chars for the corner glyphs).

### `--graph-orient {tb, lr}` (new flag)

`tb` (top-to-bottom) is the current layout — no change.
`lr` (left-to-right) renders layers as vertical columns:

```python
def _render_graph_lr(*, layers, ...) -> list[str]:
    """Layers as columns instead of rows.

    Each column is `column_width` wide; columns separated by
    " ─→ " (or " -> " in non-fancy mode).
    Tallest layer caps the row count.
    """
```

LR fallback to TB fires when
`(width - 4) // max(num_layers, 1) < 14`.

### Parser additions

`dashboard` parser gains `--graph-orient` with choices
`{tb, lr}`, default `tb`. Goes alongside the existing
`--graph-style`.

`run graph` parser also gains `--graph-orient`. The flag is
ignored when `--format` is not `ascii`.

### Dispatch wiring

`cli/dispatch.py` passes `graph_orient` through to
`run_dashboard(...)` and to `run_graph(...)`'s ascii path.

`render_frame` and `render_graph_panel` gain an `orient: str =
"tb"` keyword. Non-`tb` values that are not in `{tb, lr}` raise
`InvalidTransitionError` (defensive; the parser already
constrains the choice).

### ASCII vs fancy decision tree (final)

```text
chosen = style
if chosen == "auto":
    chosen = "layered" if slot_width >= 12 else "list"
if chosen == "fancy":
    # Need 2 more chars for box-drawing corners
    chosen = "fancy" if slot_width >= 14 else "layered"
    if chosen == "fancy" and orient == "lr":
        chosen = "fancy_lr" if num_layers >= 1 and column_width >= 14 else "fancy"
        if chosen == "fancy_lr" and column_width < 14:
            chosen = "layered"  # gives up on both upgrades
elif chosen == "layered" and orient == "lr":
    chosen = "layered_lr" if column_width >= 12 else "layered"
```

In code: keep one render-dispatcher function that picks among
`_render_graph_layered`, `_render_graph_fancy`,
`_render_graph_lr` (which itself uses fancy or ASCII per the
style flag), and `_render_graph_list`. Style trumps orient when
both can't be honored.

### Color interaction

`ANSI_STATE_COLORS` unchanged. Fancy rendering wraps the inner
content (not the box frame) with the color escape — keeps the
frame uniform across states. ANSI-aware `_truncate` already
preserves trailing resets through Unicode-character truncation
because the budget is counted in code points.

### Tests (additions, ~8 cases)

`tests/test_dashboard.py`:

- `test_render_graph_panel_fancy_uses_box_drawing`
- `test_render_graph_panel_fancy_falls_back_to_layered_at_narrow_width`
- `test_render_graph_panel_orient_lr_columns`
- `test_render_graph_panel_orient_lr_falls_back_to_tb_at_many_layers`
- `test_render_graph_panel_fancy_no_external_url_invariant`
- `test_render_graph_panel_fancy_color`
- `test_dashboard_graph_orient_flag_is_threaded`
- `test_run_graph_format_ascii_orient_lr`

### Doc updates

- `docs/SPEC.md` § "Dashboard": add `--graph-orient` flag and
  the `fancy` style note.
- `docs/UBIQUITOUS_LANGUAGE.md`: extend the `graph panel` entry
  with the new flags.
- `docs/HOW_TO_HUMAN.md` § "Dashboards and graphs": add both
  flags.
- `docs/rfcs/0016-dashboard-dependency-graph.md`: status →
  `accepted (V1+step 3)`.
- `docs/rfcs/README.md` index update.
- `docs/DECISION_LOG.md`: D064.
- `docs/TODO.md`: F12.
- `pyproject.toml` + `__init__.py`: 1.2.0 → 1.3.0.
- `CHANGELOG.md`: 1.3.0 section.

## Acceptance Criteria

- `dashboard --graph-style fancy` renders box-drawn output on a
  wide TTY.
- Fancy falls back to layered when `(width-4)/max_layer_size <
  14`.
- `dashboard --graph-orient lr` renders layers as columns.
- LR falls back to TB when `(width-4)/num_layers < 14`.
- `run graph --format ascii --graph-orient lr` works end-to-end.
- Existing tests on layered/list pass unchanged.
- Lint + typecheck + tests clean (~280 total).
- Version pins synced at 1.3.0.

## Acceptance Gate

Implementation job blocks until human acceptance recorded under
`docs/dogfood/012/decisions/`.
