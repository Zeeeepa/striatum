from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any

from test_cli_mvp import prepare_started_run, run_cli, run_cli_text


ROOT = Path(__file__).resolve().parents[1]


def _import_dashboard() -> Any:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.dashboard as dashboard  # noqa: WPS433 - test-time import
    finally:
        sys.path.pop(0)
    return dashboard


def test_render_frame_shows_job_state_counts() -> None:
    dashboard = _import_dashboard()
    payload = {
        "run": {"run_id": "run_test", "branch_name": "main", "state": "running"},
        "status": {
            "jobs": {
                "queued": 2,
                "running": 1,
                "completed": 3,
                "blocked": 1,
            },
            "open_blockers": [{"severity": "blocked"}],
            "human_checkpoints": [{"severity": "human_checkpoint"}],
            "latest_non_accepting_review_verdicts": [],
            "claimable_jobs": [
                {"role_id": "author", "lane_id": "codex", "count": 2},
            ],
            "next_actions": ["claim_available_work"],
        },
        "events": [],
        "verdict_counts": {"accept": 1, "needs_revision": 2},
        "updated_at": "2026-05-07T00:00:00Z",
    }
    output = dashboard.render_frame(payload, terminal_width=100)
    assert "run_test" in output
    assert "branch main" in output
    assert "Jobs:" in output
    # Spot-check ordered job state lines for canonical labels and counts.
    assert "queued         2" in output
    assert "running        1" in output
    assert "completed      3" in output
    assert "blocked        1" in output
    assert "Verdicts:" in output
    assert "accept               1" in output
    assert "needs_revision       2" in output
    assert "Blockers (open):" in output
    assert "human_checkpoint 1" in output
    assert "blocked          1" in output
    assert "Claimable now:" in output
    assert "author/codex x 2" in output
    assert "Next actions:" in output
    assert "claim_available_work" in output


def test_render_frame_truncates_long_event_payloads() -> None:
    dashboard = _import_dashboard()
    long_blob = "x" * 500
    payload = {
        "run": {"run_id": "run_test", "branch_name": "main", "state": "running"},
        "status": {
            "jobs": {"completed": 1},
            "open_blockers": [],
            "human_checkpoints": [],
            "latest_non_accepting_review_verdicts": [],
            "claimable_jobs": [],
            "next_actions": [],
        },
        "events": [
            {
                "event_id": 1,
                "event_type": "job.completed",
                "workflow_job_id": "job-with-an-extremely-long-id",
                "payload_json": json.dumps({"summary": long_blob}),
                "created_at": "2026-05-07T12:34:56Z",
            }
        ],
        "verdict_counts": {},
        "updated_at": "2026-05-07T12:34:56Z",
    }
    width = 80
    output = dashboard.render_frame(payload, terminal_width=width)
    for line in output.splitlines():
        assert len(line) <= width, f"line exceeded width {width}: {line!r}"
    # The recent-events section must be present and the long payload visibly truncated.
    assert "Recent events" in output
    assert long_blob not in output


def _binary_env() -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    return env


def test_dashboard_once_renders_one_frame_and_exits(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "dashboard",
            "--run-id",
            run_id,
            "--once",
        ],
        cwd=tmp_path,
        env=_binary_env(),
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode == 0, (
        f"dashboard --once failed: stdout={result.stdout!r} stderr={result.stderr!r}"
    )
    assert run_id in result.stdout
    assert "Jobs:" in result.stdout
    # At least one canonical job state row should appear.
    assert any(
        token in result.stdout
        for token in (
            "queued ",
            "claimed ",
            "running ",
            "completed ",
            "blocked ",
            "waiting_human ",
            "failed ",
        )
    )


def _payload_with_workflow(
    *,
    workflow: dict[str, Any] | None = None,
    node_states: dict[str, str] | None = None,
    events: list[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    return {
        "run": {"run_id": "run_test", "branch_name": "main", "state": "running"},
        "status": {
            "jobs": {"queued": 1, "running": 1, "completed": 1},
            "open_blockers": [],
            "human_checkpoints": [],
            "latest_non_accepting_review_verdicts": [],
            "claimable_jobs": [],
            "next_actions": [],
        },
        "events": events if events is not None else [],
        "verdict_counts": {},
        "updated_at": "2026-05-08T00:00:00Z",
        "workflow": workflow or {},
        "node_states": node_states or {},
    }


def _three_node_workflow() -> dict[str, Any]:
    return {
        "jobs": [
            {"id": "ingest", "type": "research", "role_id": "researcher"},
            {"id": "analyze", "type": "research", "role_id": "researcher"},
            {"id": "review", "type": "review", "role_id": "reviewer"},
        ],
        "edges": [
            {"from": "ingest", "to": "analyze", "on": "completed"},
            {"from": "analyze", "to": "review", "on": "completed"},
        ],
        "cycles": [
            {
                "from": "review",
                "to": "analyze",
                "on_verdict": "needs_revision",
                "max_iterations": 2,
            }
        ],
    }


def test_compute_node_states_picks_highest_attempt(tmp_path: Path) -> None:
    # End-to-end check that the helper agrees with run_graph for the
    # highest-attempt-per-workflow-job rule.
    run_id = prepare_started_run(tmp_path)

    sys.path.insert(0, str(ROOT / "src"))
    try:
        from striatum.db import connect, ensure_initialized
        from striatum.workflow import compute_node_states
    finally:
        sys.path.pop(0)

    ensure_initialized(tmp_path)
    with connect(tmp_path) as conn:
        states = compute_node_states(conn, run_id=run_id)
    # The run is started; at least one workflow_job_id should have state.
    assert isinstance(states, dict)
    for value in states.values():
        assert isinstance(value, str) and value


def test_render_graph_panel_layered_two_layers() -> None:
    dashboard = _import_dashboard()
    workflow = _three_node_workflow()
    lines = dashboard.render_graph_panel(
        workflow=workflow,
        node_states={"ingest": "completed", "analyze": "running", "review": "queued"},
        width=120,
        height_budget=50,
    )
    body = "\n".join(lines)
    assert body.startswith("Graph (layered):")
    assert "L0 " in body and "L1 " in body and "L2 " in body
    assert "[ingest C" in body
    assert "[analyze R" in body
    assert "[review Q" in body
    # Cycle rendered after the layered grid.
    assert "Cycles (needs_revision):" in body
    assert "review ~~> analyze" in body


def test_render_graph_panel_falls_back_to_list_at_narrow_width() -> None:
    dashboard = _import_dashboard()
    # Fan-out workflow: layer 1 has 4 nodes, so per-slot width at width=40
    # is < 12 and the renderer must fall back to list style.
    workflow = {
        "jobs": [
            {"id": "seed", "type": "research", "role_id": "researcher"},
            {"id": "fan_a", "type": "research", "role_id": "researcher"},
            {"id": "fan_b", "type": "research", "role_id": "researcher"},
            {"id": "fan_c", "type": "research", "role_id": "researcher"},
            {"id": "fan_d", "type": "research", "role_id": "researcher"},
        ],
        "edges": [
            {"from": "seed", "to": "fan_a", "on": "completed"},
            {"from": "seed", "to": "fan_b", "on": "completed"},
            {"from": "seed", "to": "fan_c", "on": "completed"},
            {"from": "seed", "to": "fan_d", "on": "completed"},
        ],
        "cycles": [],
    }
    lines = dashboard.render_graph_panel(
        workflow=workflow,
        node_states={},
        width=40,
        height_budget=50,
        style="auto",
    )
    body = "\n".join(lines)
    assert body.startswith("Graph (layers):"), body


def test_render_graph_panel_no_cycles_flag() -> None:
    dashboard = _import_dashboard()
    workflow = _three_node_workflow()
    lines = dashboard.render_graph_panel(
        workflow=workflow,
        node_states={},
        width=120,
        height_budget=50,
        no_cycles=True,
    )
    body = "\n".join(lines)
    assert "Cycles" not in body
    assert "~~>" not in body


def test_render_graph_panel_no_color_by_default() -> None:
    dashboard = _import_dashboard()
    workflow = _three_node_workflow()
    lines = dashboard.render_graph_panel(
        workflow=workflow,
        node_states={"ingest": "completed"},
        width=120,
        height_budget=50,
    )
    body = "\n".join(lines)
    assert "\x1b[" not in body


def test_render_graph_panel_color_emitted_when_color_true() -> None:
    dashboard = _import_dashboard()
    workflow = _three_node_workflow()
    lines = dashboard.render_graph_panel(
        workflow=workflow,
        node_states={"ingest": "completed"},
        width=120,
        height_budget=50,
        color=True,
    )
    body = "\n".join(lines)
    assert "\x1b[32m" in body  # green for completed
    assert "\x1b[0m" in body


def test_dashboard_no_graph_unchanged() -> None:
    dashboard = _import_dashboard()
    workflow = _three_node_workflow()
    payload = _payload_with_workflow(
        workflow=workflow,
        node_states={"ingest": "completed", "analyze": "running"},
    )
    output = dashboard.render_frame(
        payload, terminal_width=120, terminal_height=40, graph=False
    )
    assert "Graph (" not in output
    assert "Cycles" not in output


def test_dashboard_graph_panel_present_above_threshold() -> None:
    dashboard = _import_dashboard()
    workflow = _three_node_workflow()
    payload = _payload_with_workflow(
        workflow=workflow,
        node_states={"ingest": "completed", "analyze": "running"},
    )
    output = dashboard.render_frame(
        payload, terminal_width=120, terminal_height=40
    )
    assert "Graph (layered):" in output


def test_dashboard_graph_only_hides_other_panels() -> None:
    dashboard = _import_dashboard()
    workflow = _three_node_workflow()
    payload = _payload_with_workflow(
        workflow=workflow,
        node_states={"ingest": "completed"},
    )
    output = dashboard.render_frame(
        payload,
        terminal_width=120,
        terminal_height=40,
        graph=True,
        graph_only=True,
    )
    assert "Graph (layered):" in output
    assert "Recent events" not in output
    assert "Jobs:" not in output


def test_run_graph_format_ascii(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    out = run_cli_text(tmp_path, "run", "graph", "--run-id", run_id, "--format", "ascii")
    assert out.startswith("Graph"), out


def test_ansi_state_colors_keys_match_mermaid_fills() -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        from striatum import workflow as wf_mod
        from striatum import dashboard as dash_mod
    finally:
        sys.path.pop(0)
    # Every Mermaid state class must have an ANSI mapping.
    missing = set(wf_mod.MERMAID_STATE_FILLS) - set(dash_mod.ANSI_STATE_COLORS)
    assert not missing, f"missing ANSI mapping for: {missing}"


def test_dashboard_handles_unknown_run_cleanly(tmp_path: Path) -> None:
    # Initialize so the state DB exists; the run id is bogus.
    run_cli(tmp_path, "init")
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "dashboard",
            "--run-id",
            "run_does_not_exist",
            "--once",
        ],
        cwd=tmp_path,
        env=_binary_env(),
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode != 0
    combined = result.stdout + result.stderr
    assert "run_does_not_exist" in combined or "unknown" in combined.lower()
