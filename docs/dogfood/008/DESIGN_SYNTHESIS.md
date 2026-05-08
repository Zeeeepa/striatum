---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0016-dashboard-dependency-graph.md", "docs/dogfood/008/research/DASHBOARD_GRAPH.md", "src/striatum/dashboard.py", "src/striatum/cli/introspect.py", "src/striatum/workflow.py"]
---

# RFC 0016 V1 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0016 (steps 1–2 of its
implementation path). Step 3 (Unicode `fancy` style + `--graph-orient`)
deferred.

## Locked Contracts

### `compute_node_states`

```python
def compute_node_states(conn: sqlite3.Connection, *, run_id: str) -> dict[str, str]:
    """Return {workflow_job_id: current_state} for the highest attempt
    per workflow_job_id. Workflow jobs that have not been materialised
    yet do NOT appear in the result; callers fall back to "pending"
    for missing keys."""
```

Lifted verbatim from `introspect.run_graph`'s existing logic. New
home: `src/striatum/workflow.py`. `run_graph` rewires to import
from there. No behavior change.

### `render_graph_panel`

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

Pure. Returns a list of pre-trimmed lines. Lives in
`src/striatum/dashboard.py` (alongside `render_frame`); also imported
by `cli/introspect.run_graph` for `--format ascii`.

`style="auto"` chooses `layered` when `width >= 100` and the layered
layout fits within `height_budget`; else `list`. `style="layered"`
forces layered (truncates if too tall). `style="list"` forces list.
`style="fancy"` is V2 (deferred; falls back to `layered` for V1).

### Layout details

- Forward edges only for layer assignment.
- `layer(node) = 1 + max(layer(parent))` over forward edges.
- Cycle (`needs_revision`) edges rendered as dashed `~ ~ >` arrows
  after the layered grid. `--graph-no-cycles` suppresses them.
- Parallel groups bracketed `||  parallel: <group>  ||` on the
  layer where their lowest member sits.
- Box content: `[<workflow_job_id> <state-char>]` where state-char
  is one of `Q`/`R`/`C`/`B`/`H`/`F`/`P`/`X`/`S` per the RFC.
- Box width: `max(12, (width - 4) // max_layer_size)`. Below 12,
  the renderer falls back to list style.
- Suffix-elide job ids that exceed `box_width - 4`:
  `dogfood-005-aut…`.

### List fallback

Per-layer summary as RFC § 3:

```text
Graph (layers):
  L0  ingest [R]   seed [C]
  L1  analyze [Q]  score [Q]
  L2  review [P]  (cycle -> analyze, max 2)
```

### CLI flags (dashboard)

- `--graph` / `--no-graph` — explicit toggle. Default `auto`:
  rendered when `width >= 100 AND height >= 30 AND workflow has
  edges`.
- `--graph-only` — hide other panels.
- `--graph-style {auto,layered,list,fancy}` — default `auto`;
  `fancy` falls back to `layered` in V1.
- `--graph-no-cycles` — suppress dashed back-edges.

### CLI flags (run graph)

- Add `ascii` to `--format` choices alongside `mermaid`/`json`/`dot`.
- `run graph --format ascii` calls `render_graph_panel` with
  `width=80, height_budget=10000, style="layered", color=False`.

### Color

`ANSI_STATE_COLORS: dict[str, str]` in `dashboard.py`, keyed off the
Mermaid state class names so adding a state in
`MERMAID_STATE_FILLS` flags the missing mapping in tests.

```python
ANSI_STATE_COLORS = {
    "state-completed": "\x1b[32m",   # green
    "state-running": "\x1b[34m",     # blue
    "state-claimed": "\x1b[34m",
    "state-acked": "\x1b[34m",
    "state-blocked": "\x1b[33m",     # yellow
    "state-stale_lease": "\x1b[33m",
    "state-waiting_human": "\x1b[33m",
    "state-failed": "\x1b[31m",      # red
    "state-canceled": "\x1b[31m",
    "state-queued": "\x1b[39m",      # default fg
    "state-pending": "\x1b[2m",      # dim
}
ANSI_RESET = "\x1b[0m"
```

Color emitted only when:

- `color=True` is passed to `render_graph_panel`, AND
- `stdout.isatty()` is True (gated at the dashboard call site), AND
- `NO_COLOR` env var is unset (de-facto standard).

`--once` is non-TTY by construction; tests assert plain ASCII.

### Frame integration

`gather_payload` adds `workflow` (the snapshot JSON) and
`node_states` to the payload. `render_frame` calls
`render_graph_panel` after the events panel when `state.graph_active`
is True. `--graph-only` suppresses the other panels.

## Test Plan (pinned)

`tests/test_dashboard.py` additions:

| Test | Asserts |
|---|---|
| `test_compute_node_states_picks_highest_attempt` | refactor regression vs the existing `run_graph` logic |
| `test_render_graph_panel_layered_two_layers` | 3-node, 2-layer fixture renders boxes + arrows |
| `test_render_graph_panel_falls_back_to_list_at_narrow_width` | width=40 → list style |
| `test_render_graph_panel_no_cycles_flag` | back-edge suppressed when flag is set |
| `test_render_graph_panel_no_color_by_default` | no ANSI escapes in default output |
| `test_render_graph_panel_color_emitted_when_color_true` | ANSI present when `color=True` |
| `test_render_graph_panel_no_color_env_suppresses` | `NO_COLOR=1` overrides `color=True` at call site |
| `test_dashboard_no_graph_unchanged` | golden-file regression for `--no-graph --once` |
| `test_dashboard_graph_panel_present_above_threshold` | wide TTY frame contains "Graph" header |
| `test_dashboard_graph_only_hides_other_panels` | `--graph-only` removes events/jobs panels |
| `test_run_graph_format_ascii` | `run graph --format ascii` returns the same shape as `render_graph_panel` |
| `test_ansi_colors_table_keys_match_mermaid_fills` | unit guard so adding a state class flags the mapping |

## Doc updates

- `docs/SPEC.md` — under "Introspection / Dashboard", add the graph
  panel description.
- `docs/UBIQUITOUS_LANGUAGE.md` — add "graph panel".
- `docs/rfcs/0016-dashboard-dependency-graph.md` — status to
  `accepted (V1)` with implementation slice subsection.
- `docs/rfcs/README.md` — index status flip.
- `docs/DECISION_LOG.md` — D-row.
- `docs/TODO.md` — F-row.
- `README.md` — short pointer.
- `CHANGELOG.md` — `0.4.0` section.
- `pyproject.toml` — version bump to `0.4.0`.

## Deferred

- `--graph-style fancy` Unicode box-drawing.
- `--graph-orient {tb,lr}`.
- Memoization per `(workflow_snapshot_id, hash(node_states))`.
- Mouse-driven node selection.

## Acceptance Gate

Implementation job blocks until human acceptance recorded under
`docs/dogfood/008/decisions/`.
