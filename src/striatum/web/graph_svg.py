"""RFC 0022 V1: SVG dependency graph renderer for the web UI.

Renders the workflow's forward DAG (edges from
``workflow_graph_data``) as a layered top-down SVG. Each node
becomes a clickable link to the job's detail page. Cycles
(revision loops) are not rendered as edges — they live on the
review job as chip annotations elsewhere; this matches the
RFC 0022 design-review Finding 1 clarification.
"""

from __future__ import annotations

from html import escape as html_escape
from typing import Any, Mapping, Sequence

NODE_W = 180
NODE_H = 44
GAP_X = 24
GAP_Y = 44
PADDING = 24


def render_run_graph(
    workflow: dict[str, Any],
    node_states: Mapping[str, str] | None = None,
    *,
    run_id: str | None = None,
) -> str:
    """Return an SVG string for the workflow graph.

    Parameters
    ----------
    workflow:
        A workflow JSON dict (the same shape passed to
        ``workflow_graph_data``).
    node_states:
        Optional mapping from ``workflow_job_id`` to current
        state (e.g., ``"running"``, ``"completed"``). Missing
        entries default to ``"pending"``.
    run_id:
        When supplied, each node renders as an ``<a
        href="/run/<run_id>/job/<id>">`` link. When ``None``,
        nodes render without links (still styled).
    """
    from striatum.workflow import workflow_graph_data

    data = workflow_graph_data(workflow)
    graph = data.get("graph", {})
    nodes_in = list(graph.get("nodes", []))
    edges_in = list(graph.get("edges", []))
    if not nodes_in:
        return '<svg viewBox="0 0 100 60" xmlns="http://www.w3.org/2000/svg"><text x="10" y="35" font-size="11" fill="currentColor">(no jobs)</text></svg>'

    # Layered topological depth: longest path from any source.
    incoming: dict[str, list[str]] = {}
    outgoing: dict[str, list[str]] = {}
    for node in nodes_in:
        incoming.setdefault(str(node["job_id"]), [])
        outgoing.setdefault(str(node["job_id"]), [])
    for edge in edges_in:
        src = str(edge.get("from") or edge.get("source") or "")
        dst = str(edge.get("to") or edge.get("target") or "")
        if not src or not dst:
            continue
        outgoing.setdefault(src, []).append(dst)
        incoming.setdefault(dst, []).append(src)

    depth: dict[str, int] = {}
    for node in nodes_in:
        depth[str(node["job_id"])] = 0

    # Iterate to a fixed point for longest-path depth (acyclic
    # forward graph, so this terminates in O(layers) iterations).
    changed = True
    iterations = 0
    while changed and iterations < len(nodes_in) + 1:
        changed = False
        iterations += 1
        for node_id in list(depth.keys()):
            for parent in incoming.get(node_id, []):
                if parent not in depth:
                    continue
                candidate = depth[parent] + 1
                if candidate > depth[node_id]:
                    depth[node_id] = candidate
                    changed = True

    layers: dict[int, list[Mapping[str, Any]]] = {}
    for node in nodes_in:
        layer = depth[str(node["job_id"])]
        layers.setdefault(layer, []).append(node)
    for layer_nodes in layers.values():
        layer_nodes.sort(key=lambda n: str(n["job_id"]))

    # Position nodes.
    positions: dict[str, tuple[int, int]] = {}
    for layer_idx in sorted(layers.keys()):
        layer_nodes = layers[layer_idx]
        for index, node in enumerate(layer_nodes):
            x = PADDING + index * (NODE_W + GAP_X)
            y = PADDING + layer_idx * (NODE_H + GAP_Y)
            positions[str(node["job_id"])] = (x, y)

    max_layer_size = max(len(nodes) for nodes in layers.values())
    width = PADDING * 2 + max_layer_size * NODE_W + (max_layer_size - 1) * GAP_X
    height = PADDING * 2 + (max(layers.keys()) + 1) * NODE_H + max(0, len(layers) - 1) * GAP_Y

    parts: list[str] = []
    parts.append(
        f'<svg viewBox="0 0 {width} {height}" xmlns="http://www.w3.org/2000/svg" '
        f'role="img" aria-label="Workflow dependency graph">'
    )
    parts.append(
        '<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" '
        'markerWidth="6" markerHeight="6" orient="auto-start-reverse">'
        '<path d="M0 0 L 10 5 L 0 10 z" fill="currentColor"/></marker></defs>'
    )

    # Edges first (so they render under nodes).
    for edge in edges_in:
        src = str(edge.get("from") or edge.get("source") or "")
        dst = str(edge.get("to") or edge.get("target") or "")
        if src not in positions or dst not in positions:
            continue
        x1, y1 = positions[src]
        x2, y2 = positions[dst]
        cx1 = x1 + NODE_W // 2
        cy1 = y1 + NODE_H
        cx2 = x2 + NODE_W // 2
        cy2 = y2
        my = (cy1 + cy2) // 2
        path_d = f"M {cx1} {cy1} L {cx1} {my} L {cx2} {my} L {cx2} {cy2}"
        parts.append(
            f'<path class="graph-edge" d="{path_d}" marker-end="url(#arrow)" />'
        )

    # Nodes.
    for node in nodes_in:
        node_id = str(node["job_id"])
        x, y = positions[node_id]
        state = "pending"
        if node_states is not None:
            state = str(node_states.get(node_id, "pending"))
        role = str(node.get("role_id") or "")
        title_text = html_escape(f"{node_id}: {state} ({role})")
        node_cls = f"graph-node graph-node-{html_escape(state)}"
        node_inner = (
            f'<g class="{node_cls}" transform="translate({x} {y})">'
            f'<rect width="{NODE_W}" height="{NODE_H}" rx="6" />'
            f'<text x="12" y="18" class="graph-node-title">{html_escape(node_id)}</text>'
            f'<text x="12" y="34" class="graph-node-meta">{html_escape(role)} · {html_escape(state)}</text>'
            f'<title>{title_text}</title>'
            f'</g>'
        )
        if run_id is not None:
            href = f"/run/{html_escape(run_id)}/job/{html_escape(node_id)}"
            parts.append(
                f'<a class="graph-node-link" href="{href}">{node_inner}</a>'
            )
        else:
            parts.append(node_inner)

    parts.append("</svg>")
    return "".join(parts)


def compute_node_states_from_jobs(jobs: Sequence[Mapping[str, Any]]) -> dict[str, str]:
    """Project a list of job rows to a {workflow_job_id: state} map for
    the SVG renderer."""
    out: dict[str, str] = {}
    for job in jobs:
        wf_id = job.get("workflow_job_id")
        state = job.get("state")
        if isinstance(wf_id, str) and isinstance(state, str):
            out[wf_id] = state
    return out
