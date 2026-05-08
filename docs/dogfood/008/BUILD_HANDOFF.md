---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0016 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: dogfood-008 / RFC 0016 (Run Dependency Graph On The Dashboard)
Decision: `accepted_with_follow_up` (V1_ACCEPTANCE; autonomous)
Version: `0.4.0`

V1 build slice (RFC 0016 steps 1+2) ships in one commit. Step 3
(Unicode `fancy` style + `--graph-orient`) deferred per the RFC's
implementation path.

## Files Changed

- **`src/striatum/workflow.py`** — new `compute_node_states(conn, *,
  run_id) -> dict[str, str]` lifted verbatim from
  `cli/introspect.run_graph` so the dashboard and the existing graph
  CLI share a single source of truth for "current state after a
  requeue."
- **`src/striatum/cli/introspect.py`** — `run_graph` rewires onto
  `compute_node_states`. New `ascii` value on its `--format` choices;
  reuses `dashboard.render_graph_panel` with `width=80,
  height_budget=10000, style="layered", color=False` for one-shot
  snapshots.
- **`src/striatum/dashboard.py`** — main implementation:
  - `render_graph_panel(*, workflow, node_states, width,
    height_budget, style, color, no_cycles)` — pure function. Returns
    pre-trimmed lines; deterministic for a given input.
  - `_graph_topology`, `_layer_assignment`, `_render_graph_layered`,
    `_render_graph_list`, `_format_box`, `_format_inline`,
    `_state_class`, `_colorize_box` helpers.
  - `ANSI_STATE_COLORS: dict[str, str]` keyed off the same Mermaid
    state class names as `MERMAID_STATE_FILLS` (16-color quantization
    of the existing Mermaid palette). `ANSI_RESET` constant.
  - `_STATE_CHARS: dict[str, str]` — RFC 0016 § 2 single-letter
    mnemonics (`Q`, `R`, `C`, `B`, `H`, `F`, `P`, `X`, `S`).
  - ANSI-aware `_truncate` — preserves escape sequences and emits a
    trailing `ANSI_RESET` so colors don't bleed across truncated
    lines.
  - `gather_payload` loads the workflow snapshot JSON and the result
    of `compute_node_states` for the run.
  - `render_frame` accepts `terminal_height`, `graph`, `graph_only`,
    `graph_style`, `graph_no_cycles`, `color`. Auto rule:
    `width >= 100 AND terminal_height >= 30 AND workflow has at
    least one edge`. `graph_only` suppresses the existing panels and
    renders only the graph.
  - `run` accepts the same flags + `_detect_size` (width, height
    pair). TTY gating: `is_tty AND not os.environ.get("NO_COLOR")`.
    `--once` is non-TTY by construction.
- **`src/striatum/cli/parser.py`** — dashboard parser gains:
  `--graph` / `--no-graph` (mutex), `--graph-only`,
  `--graph-style {auto,layered,list,fancy}`, `--graph-no-cycles`.
  `run graph --format` choices add `ascii`.
- **`src/striatum/cli/dispatch.py`** — passes the four new flags
  through to `dashboard.run(...)`.
- **`tests/test_dashboard.py`** — 11 new cases:
  `test_compute_node_states_picks_highest_attempt`,
  `test_render_graph_panel_layered_two_layers`,
  `test_render_graph_panel_falls_back_to_list_at_narrow_width`,
  `test_render_graph_panel_no_cycles_flag`,
  `test_render_graph_panel_no_color_by_default`,
  `test_render_graph_panel_color_emitted_when_color_true`,
  `test_dashboard_no_graph_unchanged`,
  `test_dashboard_graph_panel_present_above_threshold`,
  `test_dashboard_graph_only_hides_other_panels`,
  `test_run_graph_format_ascii`,
  `test_ansi_state_colors_keys_match_mermaid_fills`.
- **`docs/SPEC.md`** — § Dashboard gains the graph-panel paragraph
  (auto rule, flags, ANSI gating, `run graph --format ascii`).
- **`docs/UBIQUITOUS_LANGUAGE.md`** — adds "graph panel".
- **`docs/rfcs/0016-dashboard-dependency-graph.md`** — status flips to
  `accepted (V1)`.
- **`docs/rfcs/README.md`** — index reflects `accepted (V1)` plus the
  D060 reference.
- **`docs/DECISION_LOG.md`** — D060 row.
- **`docs/TODO.md`** — F8 row.
- **`README.md`** — dashboard section gains the graph flags + the
  `run graph --format ascii` pointer.
- **`CHANGELOG.md`** — `0.4.0` section.
- **`pyproject.toml`** — version bump 0.3.0 → 0.4.0.

## Verification

- `make lint` — clean.
- `make typecheck` — clean (46 source files).
- `make test` — 244 passed (11 new dashboard tests on top of the
  baseline of 233).
- Spot-checked `striatum run graph --format ascii --run-id <id>`
  against a started run: returns the same shape as the dashboard
  panel.
- Spot-checked the graph panel against a 3-node, 2-layer fixture and
  a 4-fan-out fixture: layered style on width=120, list fallback on
  width=40.

## Notes For The Reviewer

- `_truncate` is now ANSI-aware. The branch with `\x1b[` is taken
  only for graph-panel lines that already contain escapes; all
  pre-existing call sites still hit the original byte-counting path.
- The auto rule's edge-presence check is intentional: a workflow
  with one job and zero edges has nothing to render, so the panel
  stays off even on a wide terminal.
- `_STATE_CHARS["claimed"] = "R"` matches `_STATE_CHARS["running"]`
  on purpose (RFC 0016 § 2 group: claimed/running/acked are all
  "actively in flight").
- Step 3 (Unicode `fancy` + `--graph-orient`) is deferred. The
  parser accepts `fancy` and falls back to `layered`; this lets the
  follow-up RFC swap in box-drawing characters without re-opening
  RFC 0016.
