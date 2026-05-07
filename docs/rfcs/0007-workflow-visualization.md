# RFC 0007: Workflow Visualization

Status: proposed
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
