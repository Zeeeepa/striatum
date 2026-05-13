---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0016 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Review target: dogfood-008 / RFC 0016 V1 build slice (steps 1+2)
Verdict: `accept`

## Scope

Cross-checked the implementation against the locked design synthesis
(`docs/dogfood/008/DESIGN_SYNTHESIS.md`), the RFC contract
(`docs/rfcs/0016-dashboard-dependency-graph.md`), and the V1
acceptance gate (`docs/dogfood/008/decisions/V1_ACCEPTANCE.md`).
Verification window: `make lint`, `make typecheck`, `make test`,
plus targeted reads of `src/striatum/dashboard.py`,
`src/striatum/cli/introspect.py`, `src/striatum/workflow.py`,
`src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`, and
`tests/test_dashboard.py`.

## Pinned Contracts (verified)

- **`compute_node_states`** lives in `src/striatum/workflow.py` and
  matches the synthesis docstring verbatim. `cli/introspect.run_graph`
  imports it; the highest-attempt-per-workflow_job_id rule is
  preserved (regression covered by
  `test_compute_node_states_picks_highest_attempt`).
- **`render_graph_panel`** signature matches the synthesis: pure,
  keyword-only, returns `list[str]` of pre-trimmed lines. Style fall
  back rules respected (`fancy → layered`; layered drops to list
  when per-slot width drops below 12 chars). Height-budget clipping
  works.
- **CLI flags** plumb correctly. Dashboard parser exposes `--graph` /
  `--no-graph` (mutex), `--graph-only`, `--graph-style
  {auto,layered,list,fancy}`, `--graph-no-cycles`. `run graph
  --format` choices include `ascii`. `dispatch.py` threads all four
  through to `dashboard.run`.
- **Auto rule** is exactly the synthesis's: graph rendered when
  `width >= 100 AND terminal_height >= 30 AND workflow has at least
  one edge`. Verified by
  `test_dashboard_graph_panel_present_above_threshold` and the
  no-graph regression
  (`test_dashboard_no_graph_unchanged`).
- **Color gating**. `dashboard.run` computes
  `color_active = is_tty AND not os.environ.get("NO_COLOR")` and
  passes it as `color=...` to `render_frame`. `--once` is
  non-TTY by construction (`color=False`), so script and CI paths
  are plain ASCII. Verified by
  `test_render_graph_panel_no_color_by_default` and
  `test_render_graph_panel_color_emitted_when_color_true`.
- **ASCII parity**. `test_run_graph_format_ascii` exercises
  `striatum run graph --run-id <id> --format ascii` end-to-end and
  confirms the renderer produces `"Graph"`-prefixed output via the
  same code path as the dashboard panel.
- **State-class coverage**. `test_ansi_state_colors_keys_match_mermaid_fills`
  guards against a Mermaid state class being added without a matching
  `ANSI_STATE_COLORS` entry; this is the same shape the synthesis
  asked for.

## Notes

- `_truncate` is now ANSI-aware. The new branch only fires when the
  line contains `\x1b[`, so existing call sites (header, events,
  jobs/verdicts/blockers panels) keep their byte-counting behavior;
  the change is a purely additive enhancement that preserves the
  trailing reset escape so colors don't bleed across truncated
  lines. This is a sensible implementation detail and not a contract
  the RFC pins.
- `_STATE_CHARS["claimed"] == _STATE_CHARS["running"] == "R"` is
  deliberate per the RFC's "actively in flight" grouping.
- The `_render_graph_layered` connector line uses `│`. The synthesis
  did not pin a specific glyph; both the design review and the RFC
  list it as illustrative. The current choice reads cleanly inside
  a 100-column TTY and is consistent with the layered ASCII style.
- `gather_payload` now reads the workflow snapshot JSON; rendering
  remains a pure function of the payload, so the dashboard's testing
  surface is preserved.

## Verification

- `make lint`: clean.
- `make typecheck`: clean (46 source files).
- `make test`: 244 passed (11 new dashboard tests on top of 233
  baseline).
- Spot-checked `striatum run graph --run-id <id> --format ascii`
  against a started run: returns layered output starting with the
  `"Graph"` prefix.

## Decision

`accept`. The V1 build slice meets every pinned contract from the
design synthesis. Step 3 of the RFC's path (Unicode `fancy` style +
`--graph-orient`) is correctly deferred and the parser already
accepts `fancy` with a `layered` fallback so the follow-up does not
need to re-open RFC 0016.
