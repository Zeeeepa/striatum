---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0016 step 3 — Unicode fancy + --graph-orient research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08

## Existing layered renderer

`src/striatum/dashboard.py:_render_graph_layered` walks layers
top-to-bottom and renders each as a row of `[<id> <state-char>]`
boxes separated by spaces, with `│` connectors between rows. The
ANSI-aware `_truncate` ensures color escapes don't blow the width
budget. The current `--graph-style` choices are
`{auto, layered, list, fancy}`; `fancy` already accepts but
falls back to `layered`.

## Unicode box-drawing character set

V1 uses ASCII-safe characters (`[`, `]`, `│`, `~~>`). For fancy:

- Box top/bottom: `─` (U+2500) and `─`.
- Corners: `┌` `┐` `└` `┘` (U+250C, U+2510, U+2514, U+2518).
- Vertical sides: `│` (U+2502, already used).
- Connectors between boxes: `│` between layers, `└─┴─┘` for
  multi-edge convergence (V2 — V1 keeps the simple `│` per box).
- Cycle (back-edge) arrow: `╌╌▶` using dashed horizontal
  (U+254C) + black right-pointing triangle (U+25B6). Falls back
  to `~~>` when fancy is off.

These are all in the BMP and rendered correctly by every
modern terminal (xterm, kitty, alacritty, iTerm2, Windows
Terminal). No need to ship a font.

## `fancy` shape

A box looks like:

```text
┌─────────────────┐
│ ingest C        │
└─────────────────┘
```

For a layer of three nodes:

```text
┌──────────┐  ┌──────────┐  ┌──────────┐
│ ingest C │  │ parse R  │  │ score Q  │
└──────────┘  └──────────┘  └──────────┘
       │             │             │
       └─────────────┼─────────────┘
                     │
              ┌──────────┐
              │ review P │
              └──────────┘
```

The connector logic for `fancy` is the same as `layered`'s
single `│` per box; we don't try to draw the join glyphs in V1.
Multi-source / multi-target convergence stays as parallel `│`s.

Width per box stays at `max(12, (width - 4) // max_layer_size)`
as today, minus 2 for the `│ ... │` framing — so the inner
content gets `box_width - 4` characters (was `box_width - 2`
for the `[ ... ]` form). Layered fallback fires when
`(width - 4) // max_layer_size < 14` (was 12).

## `--graph-orient {tb, lr}`

`tb` (top-to-bottom) is the current rendering. `lr` (left-to-right)
swaps the layer axis: layers stack horizontally, nodes within a
layer stack vertically.

LR shape:

```text
L0:                L1:                L2:
┌──────────┐       ┌──────────┐       ┌──────────┐
│ ingest C │ ─→    │ parse R  │ ─→    │ score Q  │
└──────────┘       └──────────┘       └──────────┘

┌──────────┐
│ seed C   │
└──────────┘
```

LR rules:

- Each layer is a vertical column of boxes.
- Boxes are joined with `─→` between adjacent columns (or `─`
  for non-fancy: `->`).
- Width budget: each column gets
  `(width - 4) // max(1, max(num_layers, 1))` space.
- Height budget: tallest layer caps the row count.
- Falls back to `tb` (the default) when `(width - 4) //
  max(num_layers) < 14`.

LR is most useful for *long* workflows (10+ nodes in a chain)
that don't fit vertically inside a 30-line dashboard frame. TB
is the right default; LR is opt-in.

## Color interaction

`color` parameter unchanged. Both new modes (`fancy` and
`lr`) honor `ANSI_STATE_COLORS`. The existing ANSI-aware
`_truncate` already handles escape sequences across multi-byte
Unicode characters because Python's `len()` over the rendered
string still counts code points (not bytes), and the truncate
budget is in code points.

## Test plan

`tests/test_dashboard.py` additions:

- `test_render_graph_panel_fancy_uses_box_drawing` — assert
  rendered output contains `┌`, `┐`, `└`, `┘`, `─`.
- `test_render_graph_panel_fancy_falls_back_to_layered_at_narrow_width` —
  width=40 with 4 nodes per layer → falls back to layered ASCII.
- `test_render_graph_panel_orient_lr_columns` — three-layer
  workflow renders as three vertical columns, not three rows.
- `test_render_graph_panel_orient_lr_falls_back_to_tb_at_many_layers` —
  10-layer workflow at width=80 falls back to `tb`.
- `test_render_graph_panel_fancy_no_external_url_invariant` —
  the rendered output has no http/https URLs (boilerplate
  guard, mirrors RFC 0015 test).
- `test_render_graph_panel_fancy_color` — `color=True` produces
  ANSI escapes and a trailing reset; ANSI-aware truncate doesn't
  strip the reset.
- `test_dashboard_graph_orient_flag_is_threaded` — exercise the
  `--graph-orient lr` CLI flag end-to-end via `dashboard --once`.
- `test_run_graph_format_ascii_orient` — `run graph --format
  ascii --graph-orient lr` produces an LR-shaped snapshot.

## Friction anticipated

- **Terminal Unicode support.** All five characters used are in
  the Latin-1 / BMP range and render correctly in every modern
  terminal we've tested. Remote SSH sessions over UTF-8 locales
  are fine. We don't need a "is this terminal Unicode-capable"
  detection — operators who need ASCII-only output use
  `--graph-style layered`.
- **`lr` and `--graph-only`** interact: an `lr` chart at
  width=200 with `--graph-only` could fill the frame
  comfortably; the LR path still respects the height budget
  via the column-width clamp.
- **`fancy` + cycle edges.** Cycles render after the layered
  grid as dashed `╌╌▶` arrows (or `~~>` if fancy is off). They
  do not participate in the box-drawn layout.
