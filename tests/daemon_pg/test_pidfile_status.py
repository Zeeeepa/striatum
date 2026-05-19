from __future__ import annotations

import os
import subprocess
from pathlib import Path
from typing import Any

from striatum.daemon_pg import client_admin


def test_daemon_status_reports_running_when_pidfile_points_to_live_pid(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    pidfile = tmp_path / "striatumd.pid"
    pidfile.write_text(f"{os.getpid()}\n", encoding="utf-8")
    monkeypatch.setattr(client_admin, "pid_path", lambda: pidfile)
    monkeypatch.setattr(client_admin, "runtime_dir", lambda: tmp_path)
    monkeypatch.setattr(client_admin, "_require_pg_auth", lambda *args, **kwargs: None)
    monkeypatch.setattr(client_admin, "_pg_instance_id", lambda _pg_conn: "instance-test")

    status = client_admin.daemon_status_pg(object(), token="test-token")

    assert status["running"] is True
    assert status["pid"] == os.getpid()
    assert status["runtime_dir"] == str(tmp_path)


def test_daemon_status_reports_not_running_for_stale_pidfile(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    completed = subprocess.Popen(["true"])
    completed.wait(timeout=5)
    pidfile = tmp_path / "striatumd.pid"
    pidfile.write_text(f"{completed.pid}\n", encoding="utf-8")
    monkeypatch.setattr(client_admin, "pid_path", lambda: pidfile)
    monkeypatch.setattr(client_admin, "runtime_dir", lambda: tmp_path)
    monkeypatch.setattr(client_admin, "_require_pg_auth", lambda *args, **kwargs: None)
    monkeypatch.setattr(client_admin, "_pg_instance_id", lambda _pg_conn: "instance-test")

    status = client_admin.daemon_status_pg(object(), token="test-token")

    assert status["running"] is False
    assert status["pid"] == completed.pid
