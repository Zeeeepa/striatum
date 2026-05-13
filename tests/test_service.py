"""RFC 0012 V1 local service tests.

Spawn `striatum serve` in a subprocess (so signal handlers work
correctly), exercise endpoints via HTTP, assert behaviour.
"""

from __future__ import annotations

import json
import os
import signal
import socket
import sqlite3
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, cast


ROOT = Path(__file__).resolve().parents[1]


def _git_init_repo(repo: Path) -> None:
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "checkout", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "t@e.com"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "t"], cwd=repo, check=True, capture_output=True)
    seed = repo / ".gitseed"
    seed.write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", ".gitseed"], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "commit", "-m", "seed", "--no-gpg-sign"],
        cwd=repo, check=True, capture_output=True,
    )


def _striatum_init(repo: Path) -> None:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), "init", "--json"],
        cwd=repo, env=env, check=True, capture_output=True,
    )


def _free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return cast(int, sock.getsockname()[1])


def _spawn_service(
    repo: Path,
    *args: str,
) -> tuple[subprocess.Popen[bytes], int]:
    """Spawn `striatum serve` and return (proc, port).

    Polls /v1/health until the service responds or 10s elapse.
    """
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    if "--port" not in args and "--unix" not in args:
        port = _free_port()
        full_args = [*args, "--host", "127.0.0.1", "--port", str(port)]
    else:
        port = 0
        full_args = list(args)
    proc = subprocess.Popen(
        [
            sys.executable, "-m", "striatum.cli",
            "--repo", str(repo), "serve",
            *full_args,
        ],
        cwd=repo, env=env,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    if port == 0 and "--unix" not in args:
        # Wait briefly for any startup output and retry from default 0.
        time.sleep(0.5)
    # macOS GitHub runners can be substantially slower than Linux,
    # especially for the first invocation in a job (cold imports).
    # Bump the readiness window to 30s; locally this still resolves
    # in well under a second.
    timeout_seconds = 30.0
    if "--unix" in args:
        unix_path = args[args.index("--unix") + 1]
        deadline = time.time() + timeout_seconds
        while time.time() < deadline:
            if Path(unix_path).exists():
                return proc, 0
            if proc.poll() is not None:
                stdout, stderr = proc.communicate(timeout=2)
                raise AssertionError(
                    f"service exited before binding socket: rc={proc.returncode} "
                    f"stdout={stdout!r} stderr={stderr!r}"
                )
            time.sleep(0.1)
        raise AssertionError(
            f"service did not create unix socket within {timeout_seconds}s"
        )
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        # Use a raw socket connect so that token-protected services
        # don't false-fail the readiness probe (they would 401 the
        # unauthenticated HTTP request).
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.5):
                return proc, port
        except (ConnectionRefusedError, socket.timeout, OSError):
            time.sleep(0.1)
        if proc.poll() is not None:
            stdout, stderr = proc.communicate(timeout=2)
            raise AssertionError(
                f"service exited before responding: rc={proc.returncode} "
                f"stdout={stdout!r} stderr={stderr!r}"
            )
    raise AssertionError(
        f"service did not respond on port within {timeout_seconds}s"
    )


def _stop_service(proc: subprocess.Popen[bytes]) -> int:
    if proc.poll() is None:
        proc.send_signal(signal.SIGTERM)
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
    return proc.returncode if proc.returncode is not None else -1


def _http_get(port: int, path: str, *, headers: dict[str, str] | None = None) -> tuple[int, dict[str, Any]]:
    req = urllib.request.Request(f"http://127.0.0.1:{port}{path}")
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            body = resp.read().decode("utf-8")
            return resp.status, json.loads(body)
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8")
        return exc.code, json.loads(body)


def _http_post_json(
    port: int,
    path: str,
    payload: dict[str, Any],
    *,
    headers: dict[str, str] | None = None,
) -> tuple[int, dict[str, Any]]:
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        data=body,
        headers={"Content-Type": "application/json", **(headers or {})},
    )
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        return exc.code, json.loads(exc.read().decode("utf-8"))


# ----- 1. Health endpoint --------------------------------------------------


def test_serve_health_endpoint(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_get(port, "/v1/health")
        assert status == 200
        assert body["ok"] is True
        assert "started_at" in body["data"]
        assert "version" in body["data"]
        assert body["data"]["mode"] == "tcp"
    finally:
        _stop_service(proc)


# ----- 2. Invoke read endpoint without flag --------------------------------


def test_serve_invoke_read_succeeds_without_mutation_flag(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_post_json(port, "/v1/invoke", {"argv": ["status"]})
        assert status == 200, body
        assert body["ok"] is True
    finally:
        _stop_service(proc)


# ----- 3. Mutation rejected without flag -----------------------------------


def test_serve_invoke_mutation_rejected_without_flag(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_post_json(port, "/v1/invoke", {"argv": ["init"]})
        assert status == 405
        assert body["ok"] is False
        assert "allow-mutations" in body["error"]["message"]
    finally:
        _stop_service(proc)


# ----- 4. Mutation succeeds with flag --------------------------------------


def test_serve_invoke_mutation_with_flag(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--allow-mutations")
    try:
        # `init` is idempotent; running it a second time via the service
        # should succeed when --allow-mutations is set.
        status, body = _http_post_json(port, "/v1/invoke", {"argv": ["init"]})
        assert status == 200, body
        assert body["ok"] is True
    finally:
        _stop_service(proc)


def test_service_workflow_template_and_generate_endpoints(tmp_path: Path) -> None:
    proc, port = _spawn_service(tmp_path)
    spec: dict[str, Any] = {
        "schema_version": "striatum.workflow_generator.v1",
        "shape": "review",
        "lane_set": "local",
        "workflow_id": "demo",
        "name": "Demo",
        "workflow_version": "2026-05-12",
        "branch": {"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": False},
        "scaffold_root": "workflows/demo",
        "artifact_root": "striatum/demo",
        "lanes": {},
        "options": {},
    }
    try:
        status, body = _http_get(port, "/workflow-templates?kind=shape")
        assert status == 200
        assert any(item["template_id"] == "review" for item in body["data"]["templates"])

        status, body = _http_post_json(port, "/workflows/generate/preview", {"spec": spec})
        assert status == 200
        assert body["data"]["workflow"]["workflow_id"] == "demo"
        assert not (tmp_path / "workflows" / "demo").exists()

        status, body = _http_post_json(
            port,
            "/workflows/generate",
            {"spec": spec, "confirm_write": True},
        )
        assert status == 405
        assert body["error"]["field_path"] == "server.allow_mutations"
    finally:
        _stop_service(proc)


def test_service_workflow_generate_writes_when_mutation_gated(tmp_path: Path) -> None:
    proc, port = _spawn_service(tmp_path, "--allow-mutations")
    spec: dict[str, Any] = {
        "schema_version": "striatum.workflow_generator.v1",
        "shape": "minimal",
        "lane_set": "local",
        "workflow_id": "demo",
        "name": "Demo",
        "workflow_version": "2026-05-12",
        "branch": {"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": False},
        "scaffold_root": "workflows/demo",
        "artifact_root": "striatum/demo",
        "lanes": {},
        "options": {},
    }
    try:
        status, body = _http_post_json(
            port,
            "/workflows/generate",
            {"spec": spec, "confirm_write": True},
        )
        assert status == 200
        assert body["data"]["status"] == "created"
        assert (tmp_path / "workflows" / "demo" / "workflow.json").exists()
    finally:
        _stop_service(proc)


def test_service_repo_tree_lists_safe_directory(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "docs").mkdir()
    (tmp_path / "docs" / "readme.md").write_text("# docs\n", encoding="utf-8")
    (tmp_path / "src").mkdir()
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_get(port, "/v1/repo/tree?path=")
        assert status == 200
        assert body["ok"] is True
        entries = body["data"]["entries"]
        assert {"name": "docs", "path": "docs", "kind": "dir", "size": None} in entries
        assert {"name": "src", "path": "src", "kind": "dir", "size": None} in entries
        assert not any(entry["name"] in {".git", ".striatum"} for entry in entries)

        status, body = _http_get(port, "/v1/repo/tree?path=docs")
        assert status == 200
        assert body["data"]["path"] == "docs"
        assert body["data"]["entries"] == [
            {"name": "readme.md", "path": "docs/readme.md", "kind": "file", "size": 7}
        ]
    finally:
        _stop_service(proc)


def test_service_repo_tree_refuses_hidden_and_traversal(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        for path in ("/v1/repo/tree?path=.git", "/v1/repo/tree?path=.."):
            status, body = _http_get(port, path)
            assert status == 404
            assert body["ok"] is False
    finally:
        _stop_service(proc)


# ----- 5. /v1/runs mirrors status -----------------------------------------


def test_serve_runs_endpoint(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_get(port, "/v1/runs")
        assert status == 200
        assert body["ok"] is True
        # status returns runs list and jobs counts; we just verify shape
        assert "runs" in body["data"]
    finally:
        _stop_service(proc)


# ----- 6. Doctor endpoint --------------------------------------------------


def test_serve_doctor_endpoint(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_get(port, "/v1/doctor")
        assert status == 200
        assert body["ok"] is True
    finally:
        _stop_service(proc)


# ----- 7. Non-loopback host refused at startup ----------------------------


def test_serve_refuses_non_loopback_host(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    proc = subprocess.run(
        [
            sys.executable, "-m", "striatum.cli",
            "--repo", str(tmp_path), "serve",
            "--host", "0.0.0.0", "--port", "12345", "--json",
        ],
        cwd=tmp_path, env=env,
        capture_output=True, text=True, timeout=10,
    )
    assert proc.returncode == 8
    assert "non-loopback" in (proc.stderr + proc.stdout)


# ----- 8. Token auth -------------------------------------------------------


def test_serve_token_required_with_token_flag(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--token", "secret123")
    try:
        status, body = _http_get(port, "/v1/health")
        assert status == 401
        status_ok, body_ok = _http_get(
            port, "/v1/health", headers={"Authorization": "Bearer secret123"},
        )
        assert status_ok == 200
        assert body_ok["ok"] is True
        # Wrong same-length token should also fail.
        status_bad, _ = _http_get(
            port, "/v1/health", headers={"Authorization": "Bearer wrong1234"},
        )
        assert status_bad == 401
    finally:
        _stop_service(proc)


# ----- 9. Unix socket binds with 0600 -------------------------------------


def test_serve_unix_socket_binds_with_0600(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    # macOS limits AF_UNIX paths to ~104 bytes; pytest's tmp_path under
    # /Users/runner/work/... already pushes the limit. Put the socket
    # in a short tempdir instead so the test passes on macOS runners.
    import tempfile

    sock_dir = Path(tempfile.mkdtemp(prefix="strs-"))
    socket_path = sock_dir / "s.sock"
    try:
        proc, _port = _spawn_service(tmp_path, "--unix", str(socket_path))
        try:
            assert socket_path.exists()
            mode = socket_path.stat().st_mode & 0o777
            assert mode == 0o600
        finally:
            _stop_service(proc)
    finally:
        if socket_path.exists():
            socket_path.unlink()
        if sock_dir.exists():
            sock_dir.rmdir()


# ----- 10. SSE replay via ?since ------------------------------------------


def test_serve_sse_replay_with_since(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    # Insert a synthetic run + events directly so we don't depend on
    # claim-loop machinery here.
    db = tmp_path / ".striatum" / "state.sqlite3"
    conn = sqlite3.connect(db)
    try:
        conn.execute(
            """
            INSERT INTO workflow_snapshots (
              workflow_snapshot_id, workflow_id, workflow_version, source_path,
              content_sha256, workflow_json, loaded_at
            )
            VALUES ('wfs_test', 'wf_test', 'v1', '/tmp/wf.json', 'abc', '{}', '2026-05-08T00:00:00Z')
            """,
        )
        conn.execute(
            """
            INSERT INTO runs (run_id, workflow_snapshot_id, repo_root, state, branch_name, branch_base, created_at)
            VALUES ('run_sse', 'wfs_test', ?, 'running', 'striatum/x', NULL, '2026-05-08T00:00:00Z')
            """,
            (str(tmp_path),),
        )
        for i in range(3):
            conn.execute(
                """
                INSERT INTO events (run_id, event_type, payload_json, created_at)
                VALUES ('run_sse', 'demo.event', ?, ?)
                """,
                (json.dumps({"i": i}), "2026-05-08T00:00:00Z"),
            )
        conn.commit()
        first_event_id = conn.execute(
            "SELECT MIN(event_id) FROM events WHERE run_id = 'run_sse'"
        ).fetchone()[0]
    finally:
        conn.close()

    proc, port = _spawn_service(tmp_path)
    try:
        # Connect to SSE with ?since=<middle event>; expect the third event,
        # then the run_terminal close.
        # Mark the run terminal so the stream closes deterministically.
        conn = sqlite3.connect(db)
        try:
            conn.execute("UPDATE runs SET state = 'completed' WHERE run_id = 'run_sse'")
            conn.commit()
        finally:
            conn.close()

        url = f"http://127.0.0.1:{port}/v1/runs/run_sse/events?since={first_event_id}"
        events: list[str] = []
        with urllib.request.urlopen(url, timeout=10) as resp:
            for raw in resp:
                line = raw.decode("utf-8").rstrip("\n")
                events.append(line)
                if line.startswith("event: striatum.run_terminal"):
                    # Read the id + data lines and break after the blank.
                    events.append(resp.readline().decode("utf-8").rstrip("\n"))
                    events.append(resp.readline().decode("utf-8").rstrip("\n"))
                    break
        joined = "\n".join(events)
        # Two of the original three events should appear (those with
        # event_id > first_event_id).
        assert joined.count("demo.event") >= 1
        assert "striatum.run_terminal" in joined
    finally:
        _stop_service(proc)


# ----- 11. Single-instance enforcement ------------------------------------


def test_serve_single_instance_via_pid_file(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        # Try to start a second instance against the same repo's TCP service.
        env = os.environ.copy()
        env["PYTHONPATH"] = str(ROOT / "src")
        second = subprocess.run(
            [
                sys.executable, "-m", "striatum.cli",
                "--repo", str(tmp_path), "serve",
                "--host", "127.0.0.1", "--port", "0", "--json",
            ],
            cwd=tmp_path, env=env,
            capture_output=True, text=True, timeout=10,
        )
        assert second.returncode == 7
        assert "already running" in (second.stderr + second.stdout)
    finally:
        _stop_service(proc)


# ----- 12. Mutation-detection unit ----------------------------------------


def test_is_read_command_classification() -> None:
    from striatum.service import is_read_command

    assert is_read_command(["status"]) is True
    assert is_read_command(["why", "id"]) is True
    assert is_read_command(["doctor"]) is True
    assert is_read_command(["list", "jobs"]) is True
    assert is_read_command(["workflow", "validate", "x"]) is True
    assert is_read_command(["workflow", "init"]) is False
    assert is_read_command(["init"]) is False
    assert is_read_command(["claim-next"]) is False
    assert is_read_command(["publish-artifact"]) is False
    assert is_read_command(["recovery", "stale-leases", "--run-id", "x"]) is True
    assert is_read_command(["recovery", "process-reconcile", "--run-id", "x"]) is False
    assert is_read_command([]) is False


# ----- 13. Token timing-safe comparison ----------------------------------


def test_tokens_match_constant_length_safe() -> None:
    from striatum.service import tokens_match

    assert tokens_match("hello", "hello") is True
    assert tokens_match("hello", "Hello") is False
    assert tokens_match("hello", "hello-extra") is False
    assert tokens_match("", "") is True
    assert tokens_match("a" * 200, "a" * 200) is True
    assert tokens_match("a" * 200, "a" * 199) is False


# ----- 14. /v1/invoke argv must be list of strings ------------------------


def test_serve_invoke_argv_validation(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_post_json(port, "/v1/invoke", {"argv": [1, 2]})
        assert status == 400
        assert "list of strings" in body["error"]["message"]
        status, body = _http_post_json(port, "/v1/invoke", {})
        assert status == 400
    finally:
        _stop_service(proc)


# ----- 15. 404 for unknown paths -----------------------------------------


def test_serve_404_for_unknown_path(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _http_get(port, "/nope")
        assert status == 404
        assert body["ok"] is False
    finally:
        _stop_service(proc)


# ----- 16. Graceful shutdown on SIGTERM ----------------------------------


def test_serve_graceful_shutdown_on_sigterm(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    pid_file = tmp_path / ".striatum" / "service.pid"
    assert pid_file.exists()
    rc = _stop_service(proc)
    assert rc == 0
    assert not pid_file.exists()
