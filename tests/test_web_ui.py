"""RFC 0013 V1 web UI tests.

Spawn `striatum serve --web` and assert the SPA assets are served, the
artifact-raw endpoint streams bytes, and the bundled assets contain no
external URLs.
"""

from __future__ import annotations

import json
import os
import re
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
    import socket as _socket

    with _socket.socket(_socket.AF_INET, _socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return cast(int, sock.getsockname()[1])


def _spawn_service(repo: Path, *args: str) -> tuple[subprocess.Popen[bytes], int]:
    import socket as _socket

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    port = _free_port()
    proc = subprocess.Popen(
        [
            sys.executable, "-m", "striatum.cli",
            "--repo", str(repo), "serve",
            "--host", "127.0.0.1", "--port", str(port),
            *args,
        ],
        cwd=repo, env=env,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    deadline = time.time() + 10.0
    while time.time() < deadline:
        try:
            with _socket.create_connection(("127.0.0.1", port), timeout=0.5):
                return proc, port
        except (ConnectionRefusedError, _socket.timeout, OSError):
            time.sleep(0.1)
        if proc.poll() is not None:
            stdout, stderr = proc.communicate(timeout=2)
            raise AssertionError(
                f"service exited before responding: rc={proc.returncode} "
                f"stdout={stdout!r} stderr={stderr!r}"
            )
    raise AssertionError("service did not respond on port within 10s")


def _stop_service(proc: subprocess.Popen[bytes]) -> int:
    import signal

    if proc.poll() is None:
        proc.send_signal(signal.SIGTERM)
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
    return proc.returncode if proc.returncode is not None else -1


def _http_get_raw(port: int, path: str) -> tuple[int, dict[str, str], bytes]:
    req = urllib.request.Request(f"http://127.0.0.1:{port}{path}")
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            headers = dict(resp.headers.items())
            body = resp.read()
            return resp.status, headers, body
    except urllib.error.HTTPError as exc:
        headers = dict(exc.headers.items()) if exc.headers else {}
        body = exc.read()
        return exc.code, headers, body


# ----- 1. Static assets served when --web is set --------------------------


def test_static_assets_served_when_web_enabled(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        assert b"<title>Striatum</title>" in body
        assert b"id=\"app\"" in body
        # CSS
        status_css, headers_css, body_css = _http_get_raw(port, "/static/app.css")
        assert status_css == 200
        assert "text/css" in headers_css.get("Content-Type", "")
        assert b"--bg" in body_css
        # JS
        status_js, headers_js, body_js = _http_get_raw(port, "/static/app.js")
        assert status_js == 200
        assert "javascript" in headers_js.get("Content-Type", "")
        assert b"renderRunList" in body_js
    finally:
        _stop_service(proc)


# ----- 2. /static/<x> 404 when --web is OFF -------------------------------


def test_static_assets_404_when_web_disabled(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, _headers, body = _http_get_raw(port, "/")
        assert status == 404
        # Static path should also 404 when web disabled.
        status_css, _, _ = _http_get_raw(port, "/static/app.css")
        assert status_css == 404
    finally:
        _stop_service(proc)


# ----- 3. CSP header on static responses ---------------------------------


def test_csp_header_on_static_responses(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        _status, headers, _body = _http_get_raw(port, "/")
        csp = headers.get("Content-Security-Policy", "")
        assert "default-src 'self'" in csp
        assert "script-src 'self'" in csp
    finally:
        _stop_service(proc)


# ----- 4. Path traversal rejected ----------------------------------------


def test_static_assets_reject_path_traversal(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/static/../service.py")
        assert status in (400, 404)
    finally:
        _stop_service(proc)


# ----- 5. Artifact raw endpoint ------------------------------------------


def test_artifact_raw_endpoint(tmp_path: Path) -> None:
    """The /v1/artifacts/<id>/raw endpoint streams the file bytes for a
    published artifact, and 404s for missing ids.

    Drives a tiny workflow end-to-end via the CLI to land a real artifact
    row, avoiding schema-coupling that hand-crafted rows had.
    """
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wui-artifact-test",
        "workflow_version": "v1",
        "name": "wui-artifact-test",
        "branch": {"mode": "auto", "suggested_name": "striatum/wui-artifact-test"},
        "coordinator": {"role_id": "author", "lane_id": "stub"},
        "lanes": {
            "stub": {"adapter": "process", "command": ["true"], "capabilities": ["write"]},
        },
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1},
        "jobs": [{
            "id": "demo",
            "type": "generic",
            "title": "Demo",
            "role_id": "author",
            "lane_id": "stub",
            "objective": "demo",
            "task_prompt": {"path": "prompts/demo.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/test/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [{
                "logical_name": "out",
                "kind": "handoff",
                "path": "docs/test/HELLO.md",
                "required": True,
            }],
        }],
        "edges": [],
        "cycles": [],
    }
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    def _cli(*args: str) -> dict[str, Any]:
        result = subprocess.run(
            [sys.executable, "-m", "striatum.cli", "--repo", str(tmp_path), *args, "--json"],
            cwd=tmp_path, env=env, check=True, capture_output=True, text=True,
        )
        return cast(dict[str, Any], json.loads(result.stdout))

    prepared = _cli("run", "prepare", "--workflow", str(workflow_path))["data"]
    run_id = prepared["run_id"]
    _cli("run", "start", "--run-id", run_id)
    session = _cli("register-session", "--run-id", run_id, "--role", "author", "--lane", "stub", "--capability", "write")["data"]
    session_id = session["session_id"]
    claim = _cli("claim-next", "--session-id", session_id)["data"]
    packet = claim["packet"]
    job_id = packet["job"]["job_id"]
    lease_id = packet["lease"]["lease_id"]
    message_id = packet["lease"]["message_id"]
    _cli("ack", "--session-id", session_id, "--message-id", message_id, "--lease-id", lease_id)
    artifact_path = "docs/test/HELLO.md"
    full_path = tmp_path / artifact_path
    full_path.parent.mkdir(parents=True, exist_ok=True)
    full_path.write_text("hello world\n", encoding="utf-8")
    pub = _cli(
        "publish-artifact",
        "--session-id", session_id, "--job-id", job_id, "--lease-id", lease_id,
        "--kind", "handoff", "--logical-name", "out", "--path", artifact_path,
    )["data"]
    artifact_id = pub["artifact_id"]

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, f"/v1/artifacts/{artifact_id}/raw")
        assert status == 200, body
        assert "text/markdown" in headers.get("Content-Type", "")
        assert body == b"hello world\n"
        # Missing artifact returns 404 with envelope.
        status404, _h404, body404 = _http_get_raw(port, "/v1/artifacts/art_missing/raw")
        assert status404 == 404
        envelope = json.loads(body404)
        assert envelope["ok"] is False
    finally:
        _stop_service(proc)


# ----- 6. No external URLs in shipped assets -----------------------------


def test_static_assets_no_external_urls() -> None:
    """Shipped SPA assets must contain no third-party origins.

    D020 forbids hosted services / CDN imports. Walk the bundled bytes
    and assert no `http://` or `https://` outside loopback comments.
    """
    from importlib.resources import files

    pkg = files("striatum.web.static")
    forbidden = re.compile(r"https?://(?!127\.0\.0\.1|localhost|::1)\S*", re.IGNORECASE)
    for name in ("index.html", "app.js", "app.css"):
        asset = pkg.joinpath(name)
        text = asset.read_text(encoding="utf-8")
        # Strip CSP/comment lines that legitimately mention http(s):// for
        # config defaults; any concrete external URL is forbidden.
        for line in text.splitlines():
            stripped = line.strip()
            # Skip CSP meta which mentions things like 'self', not URLs.
            if "Content-Security-Policy" in stripped:
                continue
            match = forbidden.search(stripped)
            assert match is None, (
                f"{name} contains external URL: {match.group(0) if match else ''} | line: {stripped}"
            )


# ----- 7. Bundled assets resolvable via importlib.resources --------------


def test_assets_resolvable_via_importlib_resources() -> None:
    from importlib.resources import files

    pkg = files("striatum.web.static")
    for name in ("index.html", "app.js", "app.css"):
        asset = pkg.joinpath(name)
        assert asset.is_file(), f"missing bundled asset: {name}"
        assert len(asset.read_bytes()) > 0


# ----- 8. /v1/* endpoints still work when --web is set ------------------


def test_v1_endpoints_still_work_with_web(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/v1/health")
        assert status == 200
        envelope = json.loads(body)
        assert envelope["ok"] is True
    finally:
        _stop_service(proc)
