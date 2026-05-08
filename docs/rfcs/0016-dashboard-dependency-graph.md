# RFC 0016: Run Dependency Graph On The Dashboard

Status: proposed
Date: 2026-05-08
Context:
`src/striatum/dashboard.py`,
`src/striatum/workflow.py` (`workflow_graph_data`, `workflow_graph_mermaid`,
`MERMAID_STATE_FILLS`, `mermaid_state_class`),
`src/striatum/cli/introspect.py` (`run_graph`),
`docs/SPEC.md` § "Workflow Config",
RFC 0007 (workflow visualization, accepted),
RFC 0013 (local web UI, proposed)

## Problem

`striatum dashboard --run-id <id>` is the in-terminal answer to "what is
this run doing right now?" It surfaces aggregate counts (jobs by state,
verdicts, blockers), claimable lanes, next actions, and the last 10
events. What it does *not* surface is the workflow's shape: which jobs
gate which, where the parallel groups sit, and where the run's frontier
actually is in the DAG.

Operators today work around this by running
`striatum run graph --run-id <id> --format mermaid`, pasting the result
into a Mermaid renderer, and reading it side-by-side with the dashboard.
That output is a snapshot — it doesn't refresh, and it lives outside the
operator's tmux pane. For workflows with more than a handful of jobs and
any review or parallel structure, "where are we in the graph" becomes
the live question, and the dashboard is silent about it.

The data needed to render this is already on disk:

- `workflow_graph_data(workflow)` returns nodes, edges, and cycles, with
  parallel-group metadata.
- `run_graph(conn, run_id=...)` annotates each node with its current
  state (highest-attempt `jobs.state` per `workflow_job_id`, plus a
  `latest_verdict` for review jobs).
- The dashboard's refresh loop already pulls a status payload every
  ~2 seconds and could pull the graph at the same cadence.

The gap is purely the renderer: nothing in `dashboard.py` knows how to
draw a layered graph in ASCII.

## Goals

- The dashboard renders an in-terminal dependency graph annotated with
  current job state, refreshed at the same cadence as the existing
  panels.
- Layout handles linear chains, parallel groups, review-gate edges, and
  bounded `needs_revision` cycles.
- State-to-color mapping reuses the existing
  `MERMAID_STATE_FILLS` / `mermaid_state_class` taxonomy so the
  dashboard, `run graph`, and the (proposed) RFC 0013 web UI agree on
  what "running" or "blocked" looks like.
- Graceful degradation on narrow terminals — never truncate job ids
  into ambiguity; collapse to a per-layer list when the layered layout
  doesn't fit.
- Read-only. The graph never mutates state and never reads outside
  `.striatum/state.sqlite3`.
- No new runtime dependencies. `dashboard.py`'s "raw ANSI, no curses,
  no third-party libs" stance (`src/striatum/dashboard.py:1-8`) holds.

## Non-Goals

- A browser-based graph. RFC 0013 owns the web UI; this RFC stays in
  the terminal.
- Interactive panning, zooming, filtering, or click-to-drill. The
  dashboard is a passive read-out.
- Live edge animation, latency overlays, or per-edge throughput.
- A new graph data model. The renderer consumes `workflow_graph_data`
  output unchanged.
- Replacing `striatum run graph`. That command remains the
  scriptable / pipe-to-file surface (Mermaid / JSON). This RFC does
  add an ASCII format on it for parity (see Proposal §7), but that's
  the only `run graph` change.
- Capturing per-job timing in the graph (could be a follow-up RFC; out
  of scope here).

## Proposal

Six landable changes, scoped so the renderer can ship before the polish
items.

### 1. Shared node-state helper

Extract the highest-attempt-per-workflow-job query out of
`introspect.run_graph` into
`striatum.workflow.compute_node_states(conn, *, run_id) -> dict[str, str]`.
The dashboard and `run_graph` both call it. This keeps the dashboard and
the existing `run graph` output from drifting on what "current state"
means after a requeue.

No behavior change for `run_graph`; only a refactor.

### 2. Layered ASCII renderer

A new pure function in `dashboard.py`:

```python
def render_graph_panel(
    *,
    workflow: JsonObject,
    node_states: Mapping[str, str],
    width: int,
    height_budget: int,
    style: str = "auto",
    color: bool = False,
) -> list[str]:
    ...
```

Returns a list of pre-trimmed lines ready to be appended to the frame.
Pure: deterministic for a given input; trivial to unit test.

Layout algorithm:

- **Layer assignment.** For each node, `layer = 1 + max(layer(parent))`
  over forward edges only; nodes with no incoming forward edges sit on
  layer 0. Cycle edges (`needs_revision` back-edges) are ignored when
  computing layers and rendered separately as dashed arrows. Parallel
  groups do not affect layer assignment — group membership is metadata,
  not a layer constraint.
- **Per-layer rendering.** Each layer is a horizontal strip of
  fixed-width boxes:

  ```
  L0  [ingest    R]   [seed      C]
       |               |
  L1  [analyze   Q]   [score     Q]
       |               |
  L2          [review     P]
              .
              .  (needs_revision -> L1.analyze)
              v
  ```

  Box content is `[<workflow_job_id> <state-char>]`, padded to the
  width budget. State char uses a one-letter mnemonic
  (`Q`=queued, `R`=running, `C`=completed, `B`=blocked, `H`=human,
  `F`=failed, `P`=pending, `X`=canceled/skipped, `S`=stale_lease).
- **Parallel groups.** Nodes sharing a `parallel_group` on the same
  layer are bracketed by a thin border with the group label
  (`||  parallel: grp_x  ||`). When members of a group end up on
  different layers, the group label appears once next to the lowest
  layer.
- **Cycle edges.** Drawn after all forward edges as dashed arrows
  (`~ ~ >`) labeled with the cycle's `max_iterations` value. They are
  *always* drawn going leftward / upward visually, regardless of
  graph orientation.
- **Width budget.** The renderer is given `width`. It computes
  `box_width = max(min_box, (width - margins) // max_layer_size)`. If
  `box_width < min_box` (currently 12), it falls back to the list
  style (see §3).

The renderer never line-wraps box content. If a job id is longer than
`box_width - 4` (state char + brackets + spacer), it is *suffix-elided*
to retain the leading discriminating characters
(e.g. `dogfood-005-aut…`).

### 3. List-style fallback

When the terminal is too narrow for the layered layout, or when
`--graph-style list` is passed, render a per-layer summary:

```
Graph (layers):
  L0  ingest [R]   seed [C]
  L1  analyze [Q]  score [Q]
  L2  review [P]  (cycle -> analyze, max 2)
```

This shape is reproducible at 60 cols (the dashboard's minimum width
`max(60, int(terminal_width))`), which the layered shape often is not
for graphs with three or more nodes per layer.

### 4. CLI surface on `dashboard`

`striatum dashboard --run-id <id>` gains:

- `--graph` / `--no-graph` — explicit toggle. Default `auto`: graph
  rendered when `terminal_width >= 100` *and* `terminal_height >= 30`
  *and* the workflow has at least one edge.
- `--graph-only` — hide the aggregate panels and dedicate the canvas
  to the graph.
- `--graph-style {auto,layered,list,fancy}` — default `auto`. `fancy`
  opts into Unicode box-drawing characters (`┌─┐│└─┘`, `→`, `⇢`).
  `layered` forces ASCII layered. `list` forces the §3 fallback.
- `--graph-no-cycles` — suppresses dashed `needs_revision` back-edges
  for busy graphs.

Defaults preserve current dashboard output: with no flags on a normal
terminal, the graph appears below the existing event log, *not*
replacing it. With `--no-graph`, the dashboard renders byte-identical
to today's output.

### 5. Color

ANSI 16-color mapping, applied only when `stdout.isatty()` returns
`True` (the dashboard already gates ANSI clearing on `is_tty`). The
mapping derives from `MERMAID_STATE_FILLS` but quantized to the
16-color palette so the bucket survives:

| Mermaid class | ANSI |
|---|---|
| `state-completed` | green |
| `state-running` / `state-claimed` / `state-acked` | blue |
| `state-blocked` / `state-stale_lease` / `state-waiting_human` | yellow |
| `state-failed` / `state-canceled` | red |
| `state-queued` | default fg |
| `state-pending` | dim |

A new module-level `ANSI_STATE_COLORS` table mirrors
`MERMAID_STATE_FILLS` so adding a state in one place flags the missing
mapping in the other.

Non-TTY (`--once`, redirect, CI capture) emits no ANSI; tests assert
the `--once` output is plain ASCII bytes.

### 6. Frame integration

`gather_payload` is extended to also load the workflow snapshot JSON
for the run and call `compute_node_states`, attaching them to the
payload as `workflow` and `node_states`. `render_frame` calls
`render_graph_panel` after the events panel when graph rendering is
active, with `height_budget = max(8, terminal_height - lines_used)`.

The graph shares the existing refresh cadence
(`refresh_seconds`, default 2.0). For very large workflows where layout
becomes noticeable, a follow-up could memoize per
`(workflow_snapshot_id, hash(node_states))`; V1 ships uncached because
graph data is small (workflows are O(tens) of jobs) and the dashboard
is not a hot path.

### 7. ASCII format on `striatum run graph`

`striatum run graph --run-id <id> --format ascii` reuses the same
`render_graph_panel` to emit a one-shot rendering, so operators can
pipe a snapshot to a file or paste it into a PR description without
running the dashboard. This is a thin reuse — the function is already
pure — and keeps the two surfaces consistent.

Existing `--format mermaid` / `--format json` behavior is unchanged.

## Acceptance Criteria

- With a 120-column TTY, `striatum dashboard --run-id <id>` shows the
  layered graph below the events panel; node colors match the state
  classification used by `run graph --format mermaid`.
- `--no-graph` produces output byte-identical to the pre-RFC dashboard
  (regression-tested against a captured fixture).
- `--graph-only` hides the aggregate panels and renders just the
  header plus the graph.
- A workflow with at least one parallel group, one review-gate edge,
  and one `needs_revision` cycle renders correctly: parallel-group
  bracket, gate-labeled forward edge, dashed back-edge with
  `max_iterations`.
- A 60-column TTY falls back to the list style without truncating
  job ids ambiguously.
- `tests/test_dashboard.py` adds cases for: linear workflow, parallel
  group, review-gate edge, `needs_revision` cycle, narrow-width
  fallback, no-graph parity with the existing fixture, and
  TTY-vs-`--once` ANSI emission.
- ANSI escape sequences appear in the rendered output only when
  `is_tty=True`. Asserted by a unit test that captures the `--once`
  bytes from a non-TTY pipe.
- `striatum run graph --run-id <id> --format ascii` produces the same
  graph (modulo whitespace) as the dashboard's graph panel for the
  same run state.
- `compute_node_states` is the only place in the codebase that
  computes "highest-attempt state per workflow job" for a run. The
  refactor leaves `run_graph` JSON output unchanged (golden test).
- No new entries appear in `pyproject.toml`'s runtime dependencies.

## Open Questions

- **Box-drawing character compatibility.** `--graph-style fancy` is
  opt-in for V1 to avoid breaking terminals that lack the relevant
  glyphs (some CI capture pipelines, minimal Linux containers). Should
  V2 promote `fancy` to the default once we have signal that it
  renders cleanly across the agent CLIs Striatum targets? Leaning yes
  but not in this RFC.
- **Vertical vs horizontal orientation.** A wide workflow with many
  parallel siblings on one layer is more readable rotated 90° (each
  layer becomes a column, edges go right). V1 ships top-to-bottom
  layered; orientation can be a `--graph-orient {tb,lr}` follow-up.
- **Color blindness and `NO_COLOR`.** The renderer should honor the
  `NO_COLOR` environment variable as a hard override even on a TTY.
  V1 includes this; flag here for reviewer attention.
- **Worktree-required badge.** Jobs whose lane has
  `worktree_required: true` (RFC 0008) could carry a `[w]` badge
  inside the box. V1 omits it; promote in a follow-up if operators
  ask.
- **Should the graph survive a missing snapshot?** A workflow whose
  snapshot row is corrupt or unreadable should not crash the
  dashboard. V1 catches the load error and renders
  `Graph unavailable: <reason>` in place of the panel.
- **Graph height under tight budgets.** When
  `height_budget < min_layered_height`, V1 falls back to list style.
  An alternative is to truncate the graph mid-layer with a
  `... +N more nodes` summary; V1 declines that complexity.
- **Reuse for `striatum dashboard --once` in CI.** If operators end up
  pasting the ASCII graph into PRs or chat, monospace integrity
  matters. Tests should assert deterministic output across
  invocations with the same run state. V1 includes that test.
- **Cycle representation when a cycle has fired.** Once the runner has
  taken a `needs_revision` cycle and incremented attempt, the dashed
  edge could be drawn solid for that one transition. V1 keeps it
  dashed (the cycle is workflow shape, not run history); per-attempt
  visualization is a follow-up.

## Relationship To Other RFCs

- **RFC 0007** — workflow visualization. Accepted. RFC 0007 defines
  the static-and-run-annotated graph data path (Mermaid / JSON / DOT).
  RFC 0016 reuses that data and renders it as ASCII inside the
  dashboard. No change to RFC 0007's surface, only an additional
  consumer.
- **RFC 0013** — local web UI. Independent. RFC 0013 will render the
  same graph in a browser using the same data path
  (`workflow_graph_data` + `compute_node_states`). The terminal
  rendering is canonical for in-tmux operators; the web rendering is
  canonical for richer interaction. The shared node-state helper
  (Proposal §1) is the contract that keeps them in sync.
- **RFC 0008** — worktree isolation. The graph could surface a
  `worktree_required` badge; V1 declines but the renderer's box
  format leaves room for one trailing flag character.
- **RFC 0010** — tool harness profiles. Independent. Profile choice
  affects which lanes a job runs on, not the dependency structure of
  the workflow.
- **RFC 0014** — process adapter completion guarantees. Independent.
  A node in the `blocked` state with a process-adapter blocker
  carries the same yellow color as any other blocker; the diagnostic
  envelope lives on `striatum why`, not on the graph node.
- **D006 / D007 / D009** — SQLite as the single source of truth.
  Graph rendering reads from the same SQLite via the shared helper;
  no out-of-band file or marker is consulted.
- **D028** — no transcripts. The graph contains no agent stdout /
  stderr. Each node is a job id and a state.

## Implementation Path

V1 ships in three landable steps:

1. **Refactor + ASCII renderer.** Extract
   `compute_node_states` into `striatum.workflow`. Add
   `render_graph_panel` and the list-style fallback. Add the
   `--graph` / `--no-graph` / `--graph-style` / `--graph-no-cycles`
   flags. Plumb `gather_payload` and `render_frame`. New tests in
   `tests/test_dashboard.py`. (Smallest tractable PR; gives operators
   the feature.)
2. **Polish + parity.** Add `--graph-only`. Add color via
   `ANSI_STATE_COLORS` (TTY only, `NO_COLOR`-aware). Add the
   `striatum run graph --format ascii` mirror. Tighten the no-graph
   regression test against a checked-in fixture.
3. **Style follow-ups.** Add `--graph-style fancy` (Unicode
   box-drawing) and a `--graph-orient {tb,lr}` flag. Each is its own
   PR; neither blocks RFC 0016 acceptance.

RFC 0016 is "accepted" once steps 1 and 2 land. Step 3 is housekeeping
that promotes the renderer without re-opening the RFC.
