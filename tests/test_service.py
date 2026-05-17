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


def _insert_phased_run(repo: Path) -> str:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        from striatum.db import init_repo
    finally:
        sys.path.pop(0)

    init_repo(repo)
    workflow = {
        "schema_version": "striatum.workflow.v1.1",
        "workflow_id": "wf_phased",
        "workflow_version": "v1",
        "name": "Phased",
        "phases": [
            {"id": "phase_design", "name": "Design", "description": "Design work", "color": "#6b7280"},
            {"id": "phase_build", "name": "Build"},
        ],
        "jobs": [
            {"id": "design_a", "type": "generic", "role_id": "author", "phase_id": "phase_design"},
            {"id": "synthesize_design", "type": "phase_synthesis", "role_id": "author", "phase_id": "phase_design"},
            {"id": "build_a", "type": "generic", "role_id": "author", "phase_id": "phase_build"},
            {"id": "synthesize_build", "type": "phase_synthesis", "role_id": "author", "phase_id": "phase_build"},
        ],
        "edges": [],
        "cycles": [],
    }
    db = repo / ".striatum" / "state.sqlite3"
    conn = sqlite3.connect(db)
    try:
        conn.execute(
            """
            INSERT INTO workflow_snapshots (
              workflow_snapshot_id, workflow_id, workflow_version, source_path,
              content_sha256, workflow_json, loaded_at
            )
            VALUES ('wfs_phased', 'wf_phased', 'v1', 'workflow.json', 'abc', ?, '2026-05-13T00:00:00Z')
            """,
            (json.dumps(workflow),),
        )
        conn.execute(
            """
            INSERT INTO runs (
              run_id, workflow_snapshot_id, repo_root, state, branch_name,
              branch_base, created_at
            )
            VALUES ('run_phased', 'wfs_phased', ?, 'running', 'main', NULL, '2026-05-13T00:00:00Z')
            """,
            (str(repo),),
        )
        for workflow_job_id, job_type, state in (
            ("design_a", "generic", "completed"),
            ("synthesize_design", "generic", "blocked"),
            ("build_a", "generic", "blocked"),
            ("synthesize_build", "generic", "blocked"),
        ):
            conn.execute(
                """
                INSERT INTO jobs (
                  job_id, run_id, workflow_job_id, title, job_type, role_id,
                  lane_selector_json, capability_requirements_json, state,
                  attempt, max_attempts, fresh_session_required, write_scope_json,
                  expected_artifacts_json, idempotency_key, created_at
                )
                VALUES (?, 'run_phased', ?, ?, ?, 'author', '{}', '[]', ?,
                        1, 1, 0, '{}', '[]', ?, '2026-05-13T00:00:00Z')
                """,
                (
                    f"job_run_phased_{workflow_job_id}",
                    workflow_job_id,
                    workflow_job_id,
                    job_type,
                    state,
                    f"run_phased:{workflow_job_id}:1",
                ),
            )
        conn.commit()
    finally:
        conn.close()
    return "run_phased"


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
    env["STRIATUM_TEST_HARNESS"] = "1"
    env["STRIATUM_DAEMON_REQUIRED"] = "0"
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
    request_headers = {
        "Content-Type": "application/json",
        "Origin": f"http://127.0.0.1:{port}",
    }
    if headers:
        request_headers.update(headers)
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        data=body,
        headers=request_headers,
    )
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        return exc.code, json.loads(exc.read().decode("utf-8"))


def _web_mutation_handler(
    tmp_path: Path,
    monkeypatch: Any,
    payload: dict[str, Any],
    calls: list[tuple[str, dict[str, Any]]],
) -> Any:
    from email.message import Message
    from io import BytesIO

    import striatum.service as service
    import striatum.service_daemon as service_daemon
    import striatum.db as db

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("web POST mutation handler opened repo-local SQLite")

    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        assert repo == tmp_path
        calls.append((method, dict(params)))
        if method == "run.prepare":
            return {
                "run_id": "run_123",
                "state": "ready",
                "branch_mode": "auto",
                "suggested_branch_name": "striatum/demo",
            }
        if method == "branch.confirm":
            return {"run_id": params["run_id"], "state": "ready", "branch": params["branch"]}
        if method == "run.start":
            return {"run_id": params["run_id"], "state": "running"}
        if method == "run.cancel":
            return {"run_id": params["run_id"], "state": "canceled", "status": "canceled"}
        if method == "run.pause":
            return {"run_id": params["run_id"], "state": "running", "status": "paused"}
        if method == "run.resume":
            return {"run_id": params["run_id"], "state": "running", "status": "resumed"}
        if method == "recovery.cancel_job":
            return {"run_id": params["run_id"], "job_id": params["job_id"], "status": "canceled"}
        if method == "run.retry_job":
            return {"run_id": params["run_id"], "job_id": params["job_id"], "new_state": "queued"}
        raise AssertionError(f"unexpected daemon RPC method: {method}")

    raw = json.dumps(payload).encode("utf-8")
    headers = Message()
    headers["Content-Type"] = "application/json"
    headers["Content-Length"] = str(len(raw))
    sent: dict[str, Any] = {}

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=True,
        token=None,
        web_enabled=True,
    )
    handler.headers = headers
    handler.rfile = BytesIO(raw)
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(db, "connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)
    return handler, sent


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
        assert body["data"]["lint"]["valid"] is True
        assert "coverage" in body["data"]["lint"]
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


def test_status_derives_phase_progress_from_snapshot(tmp_path: Path) -> None:
    run_id = _insert_phased_run(tmp_path)
    sys.path.insert(0, str(ROOT / "src"))
    try:
        from striatum.cli.introspect import phase_progress_for_run
        from striatum.db import connect
    finally:
        sys.path.pop(0)

    with connect(tmp_path) as conn:
        progress = phase_progress_for_run(conn, run_id=run_id)

    assert progress is not None
    assert progress["current_phase_id"] == "phase_design"
    assert progress["phases"][0] == {
        "id": "phase_design",
        "name": "Design",
        "index": 0,
        "state": "active",
        "jobs_total": 2,
        "jobs_completed": 1,
        "jobs_by_state": {"completed": 1, "blocked": 1},
        "synthesis_job_id": "synthesize_design",
        "synthesis_state": "blocked",
        "synthesis_verdict": None,
        "description": "Design work",
        "color": "#6b7280",
    }
    assert progress["phases"][1]["state"] == "active"


def test_service_run_detail_passes_phase_progress_context(tmp_path: Path, monkeypatch: Any) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
        import striatum.web.graph_svg as graph_svg
    finally:
        sys.path.pop(0)

    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    class FakeTemplate:
        def render(self, **kwargs: Any) -> str:
            captured.update(kwargs)
            return "ok"

    class FakeEnvironment:
        def get_template(self, name: str) -> FakeTemplate:
            assert name == "run_detail.html"
            return FakeTemplate()

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("run-detail page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "run": {
                "run_id": "run_phased",
                "state": "running",
                "branch_name": "main",
                "created_at": "2026-05-13T00:00:00Z",
                "started_at": "2026-05-13T00:00:01Z",
                "completed_at": None,
                "paused_at": None,
            },
            "workflow": {
                "schema_version": "striatum.workflow.v1.1",
                "workflow_id": "wf_phased",
                "workflow_version": "v1",
                "name": "Phased",
                "branch": {"suggested_name": "feature/phased", "allow_dirty": True},
                "jobs": [],
                "edges": [],
                "cycles": [],
            },
            "jobs": [
                {
                    "job_id": "job_run_phased_design_a",
                    "workflow_job_id": "design_a",
                    "state": "completed",
                    "role_id": "author",
                }
            ],
            "next_actions": ["claim_available_work"],
            "recovery_panel": {"run_id": "run_phased", "blockers": [], "recipes": []},
            "verdicts_by_posture": {"security": 1},
            "sessions": [{"session_id": "sess_1"}],
            "phase_progress": [
                {
                    "id": "phase_design",
                    "name": "Design",
                    "jobs_completed": 1,
                }
            ],
            "current_phase_id": "phase_design",
            "suggested_branch_name": "feature/phased",
            "allow_dirty": True,
        }

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    sent: dict[str, Any] = {}
    monkeypatch.setattr(
        handler,
        "_send_html",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(service, "_jinja_env", lambda: FakeEnvironment())
    monkeypatch.setattr(graph_svg, "compute_node_states_from_jobs", lambda jobs: {})
    monkeypatch.setattr(
        graph_svg,
        "render_run_graph",
        lambda workflow, node_states, *, run_id, jobs: "<svg></svg>",
    )
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_run_detail_page("run_phased")

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [(tmp_path, "run.detail", {"run_id": "run_phased"})]
    assert captured["current_phase_id"] == "phase_design"
    assert captured["phase_progress"][0]["name"] == "Design"
    assert captured["phase_progress"][0]["jobs_completed"] == 1
    assert captured["suggested_branch_name"] == "feature/phased"
    assert captured["allow_dirty"] is True


def test_service_job_detail_reads_daemon_dto_without_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    class FakeTemplate:
        def render(self, **kwargs: Any) -> str:
            captured.update(kwargs)
            return "ok"

    class FakeEnvironment:
        def get_template(self, name: str) -> FakeTemplate:
            assert name == "job_detail.html"
            return FakeTemplate()

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("job-detail page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "run": {"run_id": "run_123", "state": "running", "branch_name": "main"},
            "job": {
                "job_id": "job_456",
                "workflow_job_id": "review",
                "state": "completed",
                "job_type": "review",
                "role_id": "reviewer",
                "lane_id": "codex",
                "attempt": 1,
                "created_at": "2026-05-17T00:00:00Z",
                "lane_attestation_chip": {
                    "attested": False,
                    "reason": "no_attached_supervisor",
                    "supervisor_id": None,
                    "state": "unattested",
                },
            },
            "artifacts": [],
            "latest_verdict": {
                "verdict_id": "verdict_1",
                "session_id": "sess_1",
                "verdict": "accept",
                "provenance": "natural",
                "override_rationale": None,
            },
            "verdicts": [],
            "expected_artifact_rows": [
                {"path": "docs/review.md", "status": "missing_required"}
            ],
            "process_evidence": [{"process_id": "proc_1", "state": "failed"}],
        }

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    sent: dict[str, Any] = {}
    monkeypatch.setattr(
        handler,
        "_send_html",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(service, "_jinja_env", lambda: FakeEnvironment())
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_job_detail_page("run_123", "review")

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [(tmp_path, "job.detail", {"run_id": "run_123", "job_id": "review"})]
    assert captured["run"]["run_id"] == "run_123"
    assert captured["job"]["job_id"] == "job_456"
    assert captured["expected_artifact_rows"][0]["path"] == "docs/review.md"
    assert captured["process_evidence"][0]["process_id"] == "proc_1"
    assert captured["override_session_id"] == "sess_1"
    assert captured["override_context_token"]


def test_service_run_list_reads_daemon_dto_without_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    class FakeTemplate:
        def render(self, **kwargs: Any) -> str:
            captured.update(kwargs)
            return "ok"

    class FakeEnvironment:
        def get_template(self, name: str) -> FakeTemplate:
            assert name == "run_list.html"
            return FakeTemplate()

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("run-list page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "items": [
                {
                    "run_id": "run_daemon",
                    "state": "running",
                    "branch_name": "main",
                    "created_at": "2026-05-17T00:00:00Z",
                    "started_at": "2026-05-17T00:00:01Z",
                    "completed_at": None,
                    "workflow_id": "wf-daemon",
                    "workflow_name": "Daemon workflow",
                    "workflow_version": "1",
                    "workflow_snapshot_id": "snap_daemon",
                    "source_path": "workflows/daemon/workflow.json",
                    "workflow_identity": {
                        "workflow_id": "wf-daemon",
                        "workflow_version": "1",
                        "workflow_snapshot_id": "snap_daemon",
                    },
                }
            ],
            "count": 1,
        }

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    sent: dict[str, Any] = {}
    monkeypatch.setattr(
        handler,
        "_send_html",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(service, "_jinja_env", lambda: FakeEnvironment())
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_run_list_page()

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [(tmp_path, "list.runs", {"limit": 500})]
    runs = captured["runs"]
    assert len(runs) == 1
    assert runs[0]["workflow_name"] == "Daemon workflow"
    assert runs[0]["workflow_local_url"] == "/workflows/workflows/daemon/workflow.json"
    assert runs[0]["workflow_identity"] == {
        "workflow_id": "wf-daemon",
        "workflow_version": "1",
        "workflow_snapshot_id": "snap_daemon",
    }


def test_chat_briefing_active_runs_reads_daemon_dto_without_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("chat briefing opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "items": [
                {"run_id": "run_running", "state": "running"},
                {"run_id": "run_ready", "state": "ready"},
                {"run_id": "run_completed", "state": "completed"},
            ],
            "count": 3,
        }

    monkeypatch.setattr(service, "_safe_git", lambda repo, argv: "")
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    briefing = service._build_chat_briefing(tmp_path)

    assert calls == [(tmp_path, "list.runs", {"limit": 10})]
    assert "Active runs:\n  run_running (running)\n  run_ready (ready)" in briefing
    assert "run_completed" not in briefing


def test_run_posture_verdicts_reads_daemon_dto_without_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    class FakeTemplate:
        def render(self, **kwargs: Any) -> str:
            captured.update(kwargs)
            return "ok"

    class FakeEnvironment:
        def get_template(self, name: str) -> FakeTemplate:
            assert name == "run_posture_verdicts.html"
            return FakeTemplate()

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("posture verdict page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "run": {
                "run_id": "run_daemon",
                "state": "running",
                "branch_name": "main",
            },
            "posture": "security",
            "verdicts": [
                {
                    "verdict_id": "verdict_daemon",
                    "verdict": "accept",
                    "rationale": "survived security review",
                    "job_id": "job_review",
                    "workflow_job_id": "review",
                    "role_id": "reviewer",
                    "lane_id": "codex",
                    "session_slug": "reviewer-codex-001",
                    "created_at": "2026-05-17T00:00:00Z",
                    "provenance": "natural",
                    "lane_attestation_chip": {
                        "attested": False,
                        "reason": "no_attached_supervisor",
                        "supervisor_id": None,
                        "state": "unattested",
                    },
                }
            ],
        }

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    sent: dict[str, Any] = {}
    monkeypatch.setattr(
        handler,
        "_send_html",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(service, "_jinja_env", lambda: FakeEnvironment())
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_run_posture_verdicts_page("run_daemon", "security")

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [
        (
            tmp_path,
            "run.posture_verdicts",
            {"run_id": "run_daemon", "posture": "security"},
        )
    ]
    assert captured["run"]["run_id"] == "run_daemon"
    assert captured["posture"] == "security"
    assert captured["verdicts"][0]["workflow_job_id"] == "review"


def test_artifact_raw_reads_daemon_dto_without_sqlite(tmp_path: Path, monkeypatch: Any) -> None:
    from io import BytesIO

    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    artifact_path = tmp_path / "docs" / "test" / "HELLO.md"
    artifact_path.parent.mkdir(parents=True)
    artifact_path.write_text("hello from daemon\n", encoding="utf-8")
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("artifact raw handler opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "artifact": {
                "artifact_id": "art_daemon",
                "artifact_kind": "handoff",
                "repo_path": "docs/test/HELLO.md",
            }
        }

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    responses: list[int] = []
    headers: dict[str, str] = {}
    handler.wfile = BytesIO()
    monkeypatch.setattr(handler, "send_response", lambda status: responses.append(status))
    monkeypatch.setattr(handler, "send_header", lambda key, value: headers.update({key: value}))
    monkeypatch.setattr(handler, "end_headers", lambda: None)
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._handle_artifact_raw("art_daemon")

    assert calls == [(tmp_path, "artifact.show", {"artifact_id": "art_daemon"})]
    assert responses == [200]
    assert "text/markdown" in headers["Content-Type"]
    assert handler.wfile.getvalue() == b"hello from daemon\n"


def test_artifact_view_reads_daemon_dto_without_sqlite(tmp_path: Path, monkeypatch: Any) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    class FakeTemplate:
        def render(self, **kwargs: Any) -> str:
            captured.update(kwargs)
            return "ok"

    class FakeEnvironment:
        def get_template(self, name: str) -> FakeTemplate:
            assert name == "artifact_view.html"
            return FakeTemplate()

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("artifact view page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "run": {"run_id": "run_daemon", "state": "running", "branch_name": "main"},
            "artifact": {
                "artifact_id": "art_daemon",
                "run_id": "run_daemon",
                "job_id": "job_draft",
                "session_id": "sess_1",
                "logical_name": "draft",
                "artifact_kind": "handoff",
                "repo_path": "docs/draft.md",
                "content_sha256": "sha256:artifact",
                "size_bytes": 12,
                "publish_mode": "create",
                "created_at": "2026-05-17T00:00:00Z",
                "author_line": "author: author-codex-001",
                "attestation_override_rationale": None,
            },
            "expected_author_line": "author: author-codex-001",
            "provenance_trail": [
                {
                    "event_type": "provenance.publish_without_process_execution",
                    "payload": {"artifact_id": "art_daemon"},
                }
            ],
        }

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    sent: dict[str, Any] = {}
    monkeypatch.setattr(
        handler,
        "_send_html",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(service, "_jinja_env", lambda: FakeEnvironment())
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_artifact_view_page("run_daemon", "art_daemon")

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [
        (
            tmp_path,
            "artifact.show",
            {
                "artifact_id": "art_daemon",
                "run_id": "run_daemon",
                "include_web_context": True,
            },
        )
    ]
    assert captured["run"]["run_id"] == "run_daemon"
    artifact = captured["artifact"]
    assert artifact["expected_author_line"] == "author: author-codex-001"
    assert artifact["lane_attestation_chip"]["attested"] is True
    assert artifact["provenance_trail"][0]["payload"]["artifact_id"] == "art_daemon"


def test_json_read_endpoints_route_daemon_without_invoke_or_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("JSON read endpoint opened repo-local SQLite")

    def invoke_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("JSON read endpoint routed through legacy invoke")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {"method": method, "params": dict(params)}

    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service, "invoke", invoke_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    cases = [
        ("/v1/runs", "status", {}),
        ("/v1/doctor?run_id=run_1", "doctor", {"verbose": True, "run_id": "run_1"}),
        ("/v1/runs/run_1", "status", {"run_id": "run_1"}),
        ("/v1/runs/run_1/why?id=job_1", "why", {"target_id": "job_1"}),
        ("/v1/runs/run_1/dashboard", "dashboard", {"run_id": "run_1"}),
        ("/v1/runs/run_1/artifacts", "list.artifacts", {"run_id": "run_1"}),
    ]
    for path, expected_method, expected_params in cases:
        sent: dict[str, Any] = {}
        handler = object.__new__(service.StriatumServiceHandler)
        handler.state = service.ServiceState(
            repo=tmp_path,
            allow_mutations=False,
            token=None,
            web_enabled=True,
        )
        handler.path = path
        monkeypatch.setattr(handler, "_authenticate", lambda: True)
        monkeypatch.setattr(
            handler,
            "_send_json",
            lambda status, body: sent.update({"status": status, "body": body}),
        )

        handler._dispatch_get()

        assert sent == {
            "status": 200,
            "body": {
                "ok": True,
                "data": {"method": expected_method, "params": expected_params},
            },
        }

    assert calls == [
        (tmp_path, expected_method, expected_params)
        for _, expected_method, expected_params in cases
    ]


def test_doctor_page_reads_daemon_dto_without_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    class FakeTemplate:
        def render(self, **kwargs: Any) -> str:
            captured.update(kwargs)
            return "ok"

    class FakeEnvironment:
        def get_template(self, name: str) -> FakeTemplate:
            assert name == "doctor.html"
            return FakeTemplate()

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("doctor page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "ok": False,
            "schema_version": 5,
            "problems": ["human_checkpoint_open"],
            "problem_records": [
                {
                    "check": "human_checkpoint_open",
                    "severity": "info",
                    "run_id": "run_1",
                    "blocker_id": "blk_1",
                }
            ],
        }

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    sent: dict[str, Any] = {}
    monkeypatch.setattr(
        handler,
        "_send_html",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(service, "_jinja_env", lambda: FakeEnvironment())
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_doctor_page()

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [(tmp_path, "doctor", {"verbose": True})]
    records = captured["doctor"]["problem_records"]
    assert records[0]["recipes"] == [
        "striatum checkpoint resolve --blocker-id blk_1 --action continue",
        "striatum checkpoint resolve --blocker-id blk_1 --action cancel",
    ]
    assert captured["problem_groups"] == {"human_checkpoint_open": records}


def test_web_run_cancel_posts_daemon_rpc_without_sqlite(tmp_path: Path, monkeypatch: Any) -> None:
    calls: list[tuple[str, dict[str, Any]]] = []
    handler, sent = _web_mutation_handler(
        tmp_path,
        monkeypatch,
        {"reason": "operator requested stop"},
        calls,
    )

    handler._handle_run_cancel("run_123")

    assert sent["status"] == 200
    assert sent["body"]["data"]["state"] == "canceled"
    assert calls == [("run.cancel", {"run_id": "run_123", "reason": "operator requested stop"})]


def test_web_run_pause_resume_post_daemon_rpc_without_sqlite(tmp_path: Path, monkeypatch: Any) -> None:
    pause_calls: list[tuple[str, dict[str, Any]]] = []
    pause_handler, pause_sent = _web_mutation_handler(
        tmp_path,
        monkeypatch,
        {"reason": "operator pause"},
        pause_calls,
    )
    pause_handler._handle_run_pause("run_123")

    resume_calls: list[tuple[str, dict[str, Any]]] = []
    resume_handler, resume_sent = _web_mutation_handler(
        tmp_path,
        monkeypatch,
        {},
        resume_calls,
    )
    resume_handler._handle_run_resume("run_123")

    assert pause_sent["status"] == 200
    assert resume_sent["status"] == 200
    assert pause_calls == [("run.pause", {"run_id": "run_123", "reason": "operator pause"})]
    assert resume_calls == [("run.resume", {"run_id": "run_123"})]


def test_web_job_actions_post_daemon_rpc_without_sqlite(tmp_path: Path, monkeypatch: Any) -> None:
    cancel_calls: list[tuple[str, dict[str, Any]]] = []
    cancel_handler, cancel_sent = _web_mutation_handler(
        tmp_path,
        monkeypatch,
        {"reason": "bad lane output", "cascade": False},
        cancel_calls,
    )
    cancel_handler._handle_job_action("/run/run_123/job/job_456/cancel")

    retry_calls: list[tuple[str, dict[str, Any]]] = []
    retry_handler, retry_sent = _web_mutation_handler(
        tmp_path,
        monkeypatch,
        {},
        retry_calls,
    )
    retry_handler._handle_job_action("/run/run_123/job/job_456/retry")

    assert cancel_sent["status"] == 200
    assert retry_sent["status"] == 200
    assert cancel_calls == [
        (
            "recovery.cancel_job",
            {
                "run_id": "run_123",
                "job_id": "job_456",
                "reason": "bad lane output",
                "cascade": False,
            },
        )
    ]
    assert retry_calls == [("run.retry_job", {"run_id": "run_123", "job_id": "job_456"})]


def test_workflow_run_now_posts_daemon_lifecycle_without_sqlite(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    workflow_path = tmp_path / "examples" / "wf" / "workflow.json"
    workflow_path.parent.mkdir(parents=True)
    workflow_path.write_text("{}", encoding="utf-8")
    calls: list[tuple[str, dict[str, Any]]] = []
    handler, sent = _web_mutation_handler(tmp_path, monkeypatch, {}, calls)

    handler._handle_workflow_run_now("examples/wf/workflow.json")

    assert sent == {
        "status": 200,
        "body": {"ok": True, "data": {"run_id": "run_123", "status": "running"}},
    }
    assert calls == [
        ("run.prepare", {"workflow": "examples/wf/workflow.json"}),
        (
            "branch.confirm",
            {"run_id": "run_123", "branch": "striatum/demo", "create": True},
        ),
        ("run.start", {"run_id": "run_123"}),
    ]


def test_workflow_run_now_maps_daemon_workflow_error_details_to_422(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    from email.message import Message
    from io import BytesIO

    import striatum.db as db
    import striatum.service as service
    import striatum.service_daemon as service_daemon

    workflow_path = tmp_path / "examples" / "wf" / "workflow.json"
    workflow_path.parent.mkdir(parents=True)
    workflow_path.write_text("{}", encoding="utf-8")
    raw = b"{}"
    headers = Message()
    headers["Content-Type"] = "application/json"
    headers["Content-Length"] = str(len(raw))
    calls: list[tuple[str, dict[str, Any]]] = []
    sent: dict[str, Any] = {}

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("web run-now opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        assert repo == tmp_path
        calls.append((method, dict(params)))
        raise service_daemon.ServiceDaemonRpcError(
            400,
            "workflow_error",
            "workflow job references unknown role",
            details={"field_path": "jobs[0].role_id"},
        )

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=True,
        token=None,
        web_enabled=True,
    )
    handler.headers = headers
    handler.rfile = BytesIO(raw)
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(db, "connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._handle_workflow_run_now("examples/wf/workflow.json")

    assert sent["status"] == 422
    assert sent["body"]["error"]["code"] == 422
    assert sent["body"]["error"]["errors"] == [
        {
            "field_path": "jobs[0].role_id",
            "message": "workflow job references unknown role",
        }
    ]
    assert calls == [("run.prepare", {"workflow": "examples/wf/workflow.json"})]


def test_service_daemon_rpc_error_preserves_response_details(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    monkeypatch.setattr(service_daemon, "resolve_socket_path", lambda: tmp_path / "daemon.sock")
    monkeypatch.setattr(service_daemon, "daemon_socket_is_reachable", lambda path: True)
    monkeypatch.setattr(service_daemon, "_lookup_repository_id", lambda repo: "repo_test")
    monkeypatch.setattr(
        service_daemon,
        "_call_with_handshake",
        lambda sock_path, method, params: {
            "ok": False,
            "data": {
                "code": "workflow_error",
                "message": "workflow job references unknown role",
                "details": {"field_path": "jobs[0].role_id"},
            },
        },
    )

    try:
        service_daemon.call_repo_method(tmp_path, "run.prepare", {"workflow": "workflow.json"})
    except service_daemon.ServiceDaemonRpcError as exc:
        assert exc.status == 400
        assert exc.code == "workflow_error"
        assert exc.details == {"field_path": "jobs[0].role_id"}
    else:  # pragma: no cover - defensive assertion path.
        raise AssertionError("expected ServiceDaemonRpcError")


def test_web_branch_confirm_posts_daemon_rpc_then_run_start_without_sqlite(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    calls: list[tuple[str, dict[str, Any]]] = []
    handler, sent = _web_mutation_handler(
        tmp_path,
        monkeypatch,
        {"branch": "striatum/demo", "create": True, "use_current": False},
        calls,
    )

    handler._handle_run_branch_confirm("run_123")

    assert sent == {
        "status": 200,
        "body": {
            "ok": True,
            "data": {"run_id": "run_123", "state": "running", "branch": "striatum/demo"},
        },
    }
    assert calls == [
        (
            "branch.confirm",
            {
                "run_id": "run_123",
                "branch": "striatum/demo",
                "create": True,
                "use_current": False,
            },
        ),
        ("run.start", {"run_id": "run_123"}),
    ]


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


def test_service_startup_checks_daemon_doctor_without_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("production service startup opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {"ok": True}

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    service._verify_service_startup(tmp_path)

    assert calls == [(tmp_path, "doctor", {"verbose": False})]


def test_service_startup_refuses_when_daemon_doctor_fails(
    tmp_path: Path, monkeypatch: Any
) -> None:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        raise service_daemon.ServiceDaemonRpcError(
            503,
            "daemon_unreachable",
            "daemon_unreachable: doctor requires the Striatum daemon",
        )

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    try:
        service._verify_service_startup(tmp_path)
    except service.ServiceConfigError as exc:
        assert "daemon doctor failed with daemon_unreachable" in str(exc)
    else:
        raise AssertionError("startup should refuse when daemon doctor fails")


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


def test_sse_stream_reads_daemon_events_without_sqlite(
    tmp_path: Path, monkeypatch: Any
) -> None:
    from io import BytesIO

    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.service as service
        import striatum.service_daemon as service_daemon
    finally:
        sys.path.pop(0)

    calls: list[tuple[Path, str, dict[str, Any]]] = []
    batches: list[dict[str, Any]] = [
        {
            "run": {"run_id": "run_daemon", "state": "running"},
            "events": [
                {
                    "event_id": 11,
                    "run_id": "run_daemon",
                    "type": "demo.event",
                    "actor_session_id": "sess_1",
                    "job_id": "job_1",
                    "payload": {"i": 1},
                    "created_at": "2026-05-17T00:00:00Z",
                }
            ],
        },
        {"run": {"run_id": "run_daemon", "state": "completed"}, "events": []},
    ]

    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("SSE stream opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path, method: str, params: dict[str, Any]
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return batches.pop(0)

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )
    responses: list[int] = []
    headers: dict[str, str] = {}
    handler.wfile = BytesIO()
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)
    monkeypatch.setattr(handler, "send_response", lambda status: responses.append(status))
    monkeypatch.setattr(handler, "send_header", lambda key, value: headers.update({key: value}))
    monkeypatch.setattr(handler, "end_headers", lambda: None)
    monkeypatch.setattr("striatum.service.time.sleep", lambda seconds: None)
    monkeypatch.setattr("striatum.service_legacy.sqlite3.connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._stream_events("run_daemon", since=10)

    assert calls == [
        (tmp_path, "run.events", {"run_id": "run_daemon", "since_event_id": 10, "limit": 100}),
        (tmp_path, "run.events", {"run_id": "run_daemon", "since_event_id": 11, "limit": 100}),
    ]
    assert responses == [200]
    assert headers["Content-Type"] == "text/event-stream"
    body = handler.wfile.getvalue().decode("utf-8")
    assert "event: striatum.event" in body
    assert '"type": "demo.event"' in body
    assert "event: striatum.run_terminal" in body


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
    assert is_read_command(["list", "jobs", "--run-id", "x"]) is True
    assert is_read_command(["workflow", "validate", "x"]) is True
    assert is_read_command(["workflow", "lint", "x"]) is True
    assert is_read_command(["run", "graph", "--run-id", "x"]) is True
    assert is_read_command(["supervise", "status", "--session-id", "s"]) is True
    assert is_read_command(["workflow", "init"]) is False
    assert is_read_command(["init"]) is False
    assert is_read_command(["claim-next"]) is False
    assert is_read_command(["publish-artifact", "--session-id", "s", "--job-id", "j", "--path", "x"]) is False
    assert is_read_command(["run", "pause", "--run-id", "x"]) is False
    assert is_read_command(["recovery", "stale-leases", "--run-id", "x"]) is False
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
