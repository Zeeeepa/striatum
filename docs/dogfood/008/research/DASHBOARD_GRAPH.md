---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0016 V1 — Dashboard Dependency Graph Research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08

## Existing surfaces

- `src/striatum/cli/introspect.py:run_graph` (line 636) computes
  `node_states` via "highest-attempt-per-workflow-job" — the exact
  helper RFC 0016 § 1 wants extracted. Refactor: lift to
  `striatum.workflow.compute_node_states(conn, *, run_id) -> dict[str, str]`,
  call from both `run_graph` and the new dashboard renderer.
- `src/striatum/dashboard.py:gather_payload` (line 57) and
  `render_frame` (line 97) compose the existing TUI panels. The new
  graph panel slots in after the events panel.
- `MERMAID_STATE_FILLS` (workflow.py:348) is the source of truth for
  state class names; `ANSI_STATE_COLORS` mirrors it quantized to
  16 colors.

## State → ANSI color quantization (V1)

| Mermaid class | ANSI |
|---|---|
| state-completed | green |
| state-running / claimed / acked | blue |
| state-blocked / stale_lease / waiting_human | yellow |
| state-failed / canceled | red |
| state-queued | default |
| state-pending | dim |

`mermaid_state_class("skipped")` already maps to `state-canceled`,
so the ANSI table inherits it without a new entry.

## Renderer signature (final)

```python
def render_graph_panel(
    *,
    workflow: JsonObject,
    node_states: Mapping[str, str],
    width: int,
    height_budget: int,
    style: str = "auto",
    color: bool = False,
    no_cycles: bool = False,
) -> list[str]:
```

Pure function; deterministic for a given input. Returns lines
already trimmed to `width`. Emits ANSI escape codes when `color=True`.

## Layer assignment

- Forward edges only.
- Cycle (back) edges (`needs_revision`) ignored for layering;
  rendered separately as dashed `~ ~ >` arrows.
- `layer(node) = 1 + max(layer(parent))` over forward edges; nodes
  with no parents on layer 0.
- Parallel groups are rendered as a labeled bracket; group
  membership doesn't affect layer assignment.

## CLI flag plumbing

`dashboard` parser additions (parser.py): `--graph` /
`--no-graph` / `--graph-only` / `--graph-style {auto,layered,list,fancy}` /
`--graph-no-cycles`. Defaults: graph rendered when terminal width >= 100
AND height >= 30 AND workflow has at least one edge; otherwise
suppressed. `--no-graph` regression test asserts byte-identical output
to today's frames.

`run graph` parser additions: `ascii` to the `--format` choices.
Reuses `render_graph_panel` with `width=80, height_budget=10000` so
`run graph --format ascii` is a one-shot snapshot.

## TTY gating

- Color emitted only when `stdout.isatty()` returns True (existing
  `is_tty` gate in dashboard.py).
- `NO_COLOR=1` env var honored: when set, color is forced off
  regardless of TTY (de-facto standard).
- `--once` is non-TTY by definition; tests assert plain ASCII bytes.

## Test plan

`tests/test_dashboard.py` additions:

- `test_compute_node_states_returns_highest_attempt` — refactor
  regression.
- `test_render_graph_panel_layered_emits_box_chars` — feed a
  3-node, 2-layer workflow; assert the layered shape.
- `test_render_graph_panel_falls_back_to_list_at_narrow_width` —
  width=40 forces list style.
- `test_render_graph_panel_no_cycles_flag_suppresses_back_edges`.
- `test_render_graph_panel_color_only_when_color_true` — without
  color, no ANSI in output.
- `test_dashboard_no_graph_output_unchanged` — fixture-based
  regression: `--no-graph --once` matches the checked-in golden.
- `test_dashboard_graph_appears_above_threshold` — wide TTY
  produces graph; narrow does not.
- `test_run_graph_format_ascii_returns_string` — equivalence with
  the dashboard panel renderer.
- `test_no_color_env_var_suppresses_ansi` — `NO_COLOR=1` forces
  plain output even on a TTY.

## Friction anticipated

- The "byte-identical to today's frames with `--no-graph`" claim
  needs a fixture. V1 captures it during the build (the
  implementer pins the current output as a golden file in
  `tests/fixtures/dashboard_no_graph.txt`).
- The `is_tty` gate already exists; respect it for ANSI clearing.
  The new color path piggybacks on the same gate plus
  `os.environ.get("NO_COLOR")`.

## Recommended order

1. Extract `compute_node_states` (refactor; no behavior change).
2. Add `render_graph_panel` + list fallback as a pure function.
3. Add CLI flags to dashboard + dispatch.
4. Wire into `gather_payload` + `render_frame`.
5. Add `--format ascii` on `run graph`.
6. Add ANSI color via `ANSI_STATE_COLORS`.
7. Tests.
