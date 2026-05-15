from __future__ import annotations

import argparse
from pathlib import Path

from striatum.cli.daemon_rpc_route import _LOOKUP


def _route(command: str, subcommand: str | None, **kwargs: object) -> tuple[str, dict[str, object]]:
    args = argparse.Namespace(command=command, **kwargs)
    translator = _LOOKUP[(command, subcommand)]
    return translator(args, Path("/repo"))


def test_list_runs_preserves_state_and_limit_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="runs",
        state="running",
        limit=25,
    )

    assert method == "list.runs"
    assert params == {"state": "running", "limit": 25}


def test_list_sessions_preserves_run_role_lane_and_state_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="sessions",
        run_id="run-1",
        state="active",
        role="reviewer",
        lane="codex",
    )

    assert method == "list.sessions"
    assert params == {
        "run_id": "run-1",
        "state": "active",
        "role": "reviewer",
        "lane": "codex",
    }


def test_list_jobs_preserves_run_state_and_workflow_job_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="jobs",
        run_id="run-1",
        state="queued",
        workflow_job_id="build",
    )

    assert method == "list.jobs"
    assert params == {
        "run_id": "run-1",
        "state": "queued",
        "workflow_job_id": "build",
    }


def test_list_artifacts_preserves_run_and_kind_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="artifacts",
        run_id="run-1",
        kind="finding",
    )

    assert method == "list.artifacts"
    assert params == {"run_id": "run-1", "kind": "finding"}


def test_list_workflows_preserves_limit_filter() -> None:
    method, params = _route(
        "list",
        None,
        list_command="workflows",
        limit=10,
    )

    assert method == "list.workflows"
    assert params == {"limit": 10}


def test_corpus_export_routes_to_daemon_rpc() -> None:
    method, params = _route(
        "corpus",
        "export",
        corpus_command="export",
        since="HEAD~1",
        out="tmp/corpus",
    )

    assert method == "corpus.export"
    assert params == {"since": "HEAD~1", "out": "tmp/corpus"}
