"""PG handler for ``run.graph``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, cast

from striatum.dashboard import render_graph_panel
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._registry import register_pg_handler
from ._sql import optional_text, parse_json_object, row_to_json, require_text

_FORMATS = {"json", "mermaid", "dot", "ascii"}
_GRAPH_ORIENTS = {"tb", "lr"}
_GRAPH_STYLES = {"auto", "layered", "list", "fancy"}


@register_pg_handler("run.graph", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    output_format = _choice(params, "format", default="mermaid", allowed=_FORMATS)
    workflow = _workflow_for_run(ctx, run_id)
    dependency_edges = _dependency_edges(ctx, run_id=run_id)
    graph_workflow = _workflow_with_pg_dependencies(
        workflow=workflow,
        dependency_edges=dependency_edges,
    )
    node_states, latest_by_workflow_job = _live_job_state(ctx, run_id=run_id)

    if output_format == "json":
        latest_verdicts = _latest_verdicts_for_jobs(
            ctx,
            job_ids=[
                str(row["job_id"])
                for row in latest_by_workflow_job.values()
                if str(row.get("job_type")) == "review"
            ],
        )
        return _graph_data_payload(
            run_id=run_id,
            workflow=graph_workflow,
            dependency_edges=dependency_edges,
            latest_by_workflow_job=latest_by_workflow_job,
            latest_verdicts=latest_verdicts,
        )

    if output_format == "mermaid":
        from striatum.workflow import workflow_graph_mermaid

        return {
            "format": "mermaid",
            "source": workflow_graph_mermaid(graph_workflow, node_states=node_states),
        }

    if output_format == "dot":
        from striatum.workflow import workflow_graph_dot

        return {"format": "dot", "source": workflow_graph_dot(graph_workflow)}

    graph_orient = _choice(params, "graph_orient", default="tb", allowed=_GRAPH_ORIENTS)
    graph_style = _choice(params, "graph_style", default="layered", allowed=_GRAPH_STYLES)
    lines = render_graph_panel(
        workflow=graph_workflow,
        node_states=node_states,
        width=80,
        height_budget=10000,
        style=graph_style,
        orient=graph_orient,
        color=False,
    )
    return {"format": "ascii", "source": "\n".join(lines) + "\n"}


def _workflow_for_run(ctx: RepoHandlerContext, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT w.workflow_json
            FROM striatumd.runs r
            JOIN striatumd.workflow_snapshots w
              ON w.repository_id = r.repository_id
             AND w.workflow_snapshot_id = r.workflow_snapshot_id
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"run not found: {run_id}")
    return parse_json_object(row["workflow_json"])


def _workflow_with_pg_dependencies(
    *,
    workflow: Mapping[str, Any],
    dependency_edges: list[dict[str, Any]],
) -> dict[str, Any]:
    if not dependency_edges:
        return dict(workflow)
    graph_workflow = dict(workflow)
    graph_workflow["edges"] = [
        {"from": edge["from"], "to": edge["to"], "on": "completed"}
        for edge in dependency_edges
    ]
    return graph_workflow


def _dependency_edges(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT upstream.workflow_job_id AS from_id,
                   downstream.workflow_job_id AS to_id,
                   d.gate_json,
                   upstream.attempt AS upstream_attempt,
                   downstream.attempt AS downstream_attempt
            FROM striatumd.job_dependencies d
            JOIN striatumd.jobs downstream
              ON downstream.repository_id = d.repository_id
             AND downstream.job_id = d.job_id
            JOIN striatumd.jobs upstream
              ON upstream.repository_id = d.repository_id
             AND upstream.job_id = d.depends_on_job_id
            WHERE d.repository_id = %s
              AND downstream.run_id = %s
              AND upstream.run_id = %s
            ORDER BY upstream.workflow_job_id, downstream.workflow_job_id,
                     downstream.attempt DESC, upstream.attempt DESC
            """,
            (ctx.repository_id, run_id, run_id),
        )
        rows = cur.fetchall()

    edges: list[dict[str, Any]] = []
    seen: set[tuple[str, str]] = set()
    for row in rows:
        from_id = str(row["from_id"])
        to_id = str(row["to_id"])
        key = (from_id, to_id)
        if key in seen:
            continue
        seen.add(key)
        gate = parse_json_object(row.get("gate_json"))
        graph_gate: dict[str, Any] = {"on": str(gate.get("on") or "completed")}
        requires_verdict = gate.get("requires_verdict")
        if isinstance(requires_verdict, list):
            graph_gate["requires_verdict"] = requires_verdict
        edges.append({"from": from_id, "to": to_id, "gate": graph_gate})
    return edges


def _live_job_state(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
) -> tuple[dict[str, str], dict[str, dict[str, Any]]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.job_id, j.workflow_job_id, j.state, j.attempt, j.job_type
            FROM striatumd.jobs j
            WHERE j.repository_id = %s AND j.run_id = %s
            ORDER BY j.workflow_job_id, j.attempt DESC
            """,
            (ctx.repository_id, run_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]

    node_states: dict[str, str] = {}
    latest_by_workflow_job: dict[str, dict[str, Any]] = {}
    for row in rows:
        workflow_job_id = str(row["workflow_job_id"])
        if workflow_job_id in latest_by_workflow_job:
            continue
        latest_by_workflow_job[workflow_job_id] = row
        node_states[workflow_job_id] = str(row["state"])
    return node_states, latest_by_workflow_job


def _graph_data_payload(
    *,
    run_id: str,
    workflow: Mapping[str, Any],
    dependency_edges: list[dict[str, Any]],
    latest_by_workflow_job: Mapping[str, Mapping[str, Any]],
    latest_verdicts: Mapping[str, Mapping[str, Any]],
) -> dict[str, Any]:
    from striatum.workflow import mermaid_state_class, workflow_graph_data

    graph_payload = workflow_graph_data(cast(dict[str, Any], dict(workflow)))
    graph = cast(dict[str, Any], graph_payload["graph"])
    if dependency_edges:
        graph["edges"] = [
            {"from": edge["from"], "to": edge["to"], "gate": edge["gate"]}
            for edge in dependency_edges
        ]
    nodes = cast(list[dict[str, Any]], graph.get("nodes", []))
    annotated_nodes: list[dict[str, Any]] = []
    for node in nodes:
        annotated = dict(node)
        workflow_job_id = str(node["job_id"])
        row = latest_by_workflow_job.get(workflow_job_id)
        if row is None:
            annotated["current_state"] = "pending"
            annotated["attempt"] = None
            annotated["state_class"] = mermaid_state_class("pending")
        else:
            state = str(row["state"])
            annotated["current_state"] = state
            annotated["attempt"] = int(row["attempt"])
            annotated["state_class"] = mermaid_state_class(state)
            if str(row["job_type"]) == "review":
                verdict_row = latest_verdicts.get(str(row["job_id"]))
                if verdict_row is None:
                    annotated["latest_verdict"] = None
                else:
                    annotated["latest_verdict"] = {
                        "verdict_id": verdict_row["verdict_id"],
                        "verdict": verdict_row["verdict"],
                        "rationale": verdict_row.get("rationale"),
                        "findings_artifact_id": verdict_row.get("findings_artifact_id"),
                        "posture": verdict_row.get("posture", "neutral"),
                    }
        annotated_nodes.append(annotated)
    graph["nodes"] = annotated_nodes
    return {
        "run_id": run_id,
        "workflow_id": graph_payload["workflow_id"],
        "workflow_version": graph_payload.get("workflow_version"),
        "graph": graph,
    }


def _latest_verdicts_for_jobs(
    ctx: RepoHandlerContext,
    *,
    job_ids: list[str],
) -> dict[str, dict[str, Any]]:
    if not job_ids:
        return {}
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT DISTINCT ON (v.job_id)
                   v.job_id, v.verdict_id, v.verdict, v.rationale,
                   v.findings_artifact_id, v.posture, v.created_at
            FROM striatumd.verdicts v
            WHERE v.repository_id = %s AND v.job_id = ANY(%s)
            ORDER BY v.job_id, v.created_at DESC, v.verdict_id DESC
            """,
            (ctx.repository_id, job_ids),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    return {str(row["job_id"]): row for row in rows}


def _choice(
    params: Mapping[str, Any],
    key: str,
    *,
    default: str,
    allowed: set[str],
) -> str:
    value = optional_text(params, key) or default
    if value not in allowed:
        options = ", ".join(sorted(allowed))
        raise RpcError("schema_invalid", f"unknown {key}: {value!r} (valid options: {options})")
    return value


__all__ = ["handle"]
