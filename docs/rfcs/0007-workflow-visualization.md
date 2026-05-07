# RFC 0007: Workflow Visualization

Status: accepted
Date: 2026-05-07

## Problem

Workflows are defined in JSON, which can become large and difficult for humans to audit. It is hard to visualize the dependency graph, parallel lanes, and potential revision cycles by reading raw JSON. This leads to configuration errors that are only caught at runtime.

## Goals

- Provide a human-readable visual representation of a workflow.
- Allow developers to audit complex DAGs before starting a run.
- Integration with standard documentation tools.

## Non-Goals

- Do not build a custom GUI or web-based graph renderer.
- Do not support real-time "live" graph updates during a run (yet).

## Proposal

1.  **Mermaid.js Export:** Add a command `striatum workflow graph <workflow_path>` that outputs a Mermaid-formatted string.
2.  **Stateful Graph:** Optionally support `striatum run graph --run-id <run_id>` which highlights completed, active, and blocked jobs using Mermaid classes/colors.
3.  **Validation Integration:** Include the graph output in `striatum workflow validate` when requested, allowing for quick visual verification.

## Acceptance Criteria

- `striatum workflow graph` produces valid Mermaid `graph TD` output.
- The graph correctly represents parallel groups, edges, and cycles.
- Output can be piped to a file or rendered directly in Markdown viewers that support Mermaid.

## Open Questions

- Should we support other formats like Graphviz (DOT)?
- Should the graph include artifact paths or just job titles and IDs?

## Implementation Notes

- Static workflow export landed earlier:
  `striatum workflow graph <path> [--format mermaid|json]` is implemented in
  `src/striatum/workflow.py:workflow_graph_data` and
  `workflow_graph_mermaid`, dispatched through `src/striatum/cli/dispatch.py`.
  Parallel groups, edges, and `needs_revision` cycles all render correctly;
  see `tests/test_cli_mvp.py:test_workflow_graph_exports_mermaid_and_json`.
- Stateful run graph: `striatum run graph --run-id <id> [--format mermaid|json]`
  is now implemented. It loads the workflow snapshot for the run, picks the
  highest-`attempt` row per `workflow_job_id`, and annotates the graph with
  current job state.
  - Mermaid output appends a Mermaid `classDef` palette and per-node `class`
    assignments. State classes: `state-completed` (green `#c8e6c9`),
    `state-running`/`state-claimed`/`state-acked` (blue `#bbdefb`),
    `state-blocked`/`state-stale_lease`/`state-waiting_human` (yellow
    `#fff59d`), `state-failed`/`state-canceled` (red `#ffcdd2`),
    `state-queued` (grey `#e0e0e0`), `state-pending` (light grey `#f5f5f5`,
    default for nodes that have no row yet). `skipped` is mapped to
    `state-canceled` so the palette stays compact.
  - JSON output extends each node with `current_state`, `attempt`, and a
    `latest_verdict` block for review jobs.
  - Implementation: `src/striatum/workflow.py:workflow_graph_mermaid`
    (now accepts an optional `node_states` mapping) plus
    `MERMAID_STATE_FILLS` / `mermaid_state_class`; the CLI handler is
    `src/striatum/cli/introspect.py:run_graph`, wired through
    `src/striatum/cli/parser.py` and `src/striatum/cli/dispatch.py`.
  - Tests: `tests/test_cli_mvp.py:test_run_graph_highlights_job_states_in_mermaid`,
    `:test_run_graph_json_includes_current_state`.
- Validation integration ("graph output inside `workflow validate` when
  requested") is intentionally out of scope here: the dedicated
  `workflow graph` command and its `--format json` mode already cover the
  authoring use case, and folding graph output into `workflow validate`
  would change the validate response shape without a clear caller need.
  Reopen if a workflow author actually requests it.
