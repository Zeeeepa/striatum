# ruff: noqa
from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

from pathlib import Path

from striatum.legacy_sqlite.db import connect
from test_cli_mvp import prepare_started_run
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


def test_view_file_breadcrumb_omits_legacy_sqlite_run_link(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    target = tmp_path / "docs" / "dogfood" / "055" / "build" / "HANDOFF.md"
    target.parent.mkdir(parents=True)
    target.write_text("# Handoff\n", encoding="utf-8")
    with connect(tmp_path) as conn:
        conn.execute(
            "UPDATE runs SET branch_name = 'striatum/dogfood-055-rfc-0050-v1-5' WHERE run_id = ?",
            (run_id,),
        )
        conn.commit()

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/view/docs/dogfood/055/build/HANDOFF.md")
        assert status == 200
        assert f'/run/{run_id}'.encode() not in body
    finally:
        _stop_service(proc)


def test_view_file_breadcrumb_hides_ambiguous_matches(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    target = tmp_path / "docs" / "dogfood" / "055" / "build" / "HANDOFF.md"
    target.parent.mkdir(parents=True)
    target.write_text("# Handoff\n", encoding="utf-8")
    with connect(tmp_path) as conn:
        conn.execute(
            "UPDATE runs SET branch_name = 'striatum/dogfood-055-rfc-0050-v1-5' WHERE run_id = ?",
            (run_id,),
        )
        conn.execute(
            """
            INSERT INTO runs(run_id, workflow_snapshot_id, repo_root, state, branch_name, created_at)
            SELECT 'run_second_055', workflow_snapshot_id, repo_root, state,
                   'striatum/dogfood-055-other', created_at
            FROM runs WHERE run_id = ?
            """,
            (run_id,),
        )
        conn.commit()

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/view/docs/dogfood/055/build/HANDOFF.md")
        assert status == 200
        assert b"run_second_055" not in body
        assert f'/run/{run_id}'.encode() not in body
    finally:
        _stop_service(proc)
