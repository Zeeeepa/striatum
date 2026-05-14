"""GH #10 regression: override-verdict modal binds to the rendered
job page context on both client and server.

Client side (static JS):
- ``buildArgv`` uses the page-rendered identifiers, not user input.
- ``parsePageContext`` extracts ``(run_id, job_id)`` from ``/run/.../
  job/...``.
- ``buildWebContext`` carries the server-issued context token.
- The POST body shape includes ``web_context``.

Server side (``/v1/invoke``):
- Refuses ``override-verdict`` without ``web_context`` → 403.
- Refuses mismatched argv ``--job-id`` / ``--session-id`` → 403.
- Refuses a forged or malformed context token → 403.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "src/striatum/web/static/override_verdict.js"
TEMPLATE = ROOT / "src/striatum/web/templates/job_detail.html"


def _script() -> str:
    return SCRIPT.read_text(encoding="utf-8")


def _template() -> str:
    return TEMPLATE.read_text(encoding="utf-8")


# --- static client checks -------------------------------------------


def test_override_modal_parses_page_url_for_run_and_job() -> None:
    js = _script()
    assert "parsePageContext" in js
    assert "/\\/run\\/([^/]+)\\/job\\/([^/]+)/" in js or "/run/" in js
    assert "window.location.pathname" in js


def test_override_modal_refuses_on_dom_url_mismatch_before_fetch() -> None:
    js = _script()
    # The mismatch flag must guard openDialog and submit, before
    # postInvoke is ever called.
    assert "contextMismatch" in js
    submit_index = js.index("submitButton.disabled = true;")
    mismatch_check = js.rindex("if (contextMismatch)", 0, submit_index)
    assert mismatch_check < submit_index


def test_override_modal_posts_web_context_token() -> None:
    js = _script()
    assert "buildWebContext" in js
    assert "kind: \"override_verdict\"" in js
    assert "token: context.contextToken" in js
    assert "web_context: webContext" in js


def test_job_detail_template_emits_context_token_attribute() -> None:
    html = _template()
    assert "data-context-token" in html
    assert "override_context_token" in html


# --- server-side enforcement ----------------------------------------


def _post_invoke(
    port: int,
    payload: dict[str, Any],
    *,
    origin: str | None = None,
) -> tuple[int, dict[str, Any]]:
    headers = {"Content-Type": "application/json"}
    if origin is not None:
        headers["Origin"] = origin
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}/v1/invoke",
        data=json.dumps(payload).encode("utf-8"),
        headers=headers,
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as exc:
        return exc.code, json.loads(exc.read())


def test_invoke_refuses_override_verdict_without_web_context(tmp_path: Path) -> None:
    from test_web_ui import _git_init_repo, _spawn_service, _stop_service, _striatum_init

    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, envelope = _post_invoke(
            port,
            {
                "argv": [
                    "override-verdict",
                    "--session-id", "sess_x",
                    "--job-id", "job_y",
                    "--verdict", "accept",
                    "--rationale", "test",
                ],
            },
            origin=f"http://127.0.0.1:{port}",
        )
        assert status == 403, envelope
        assert envelope["ok"] is False
        assert "web_context" in envelope["error"]["message"]
    finally:
        _stop_service(proc)


def test_invoke_refuses_override_verdict_with_argv_job_mismatch(tmp_path: Path) -> None:
    from test_web_ui import _git_init_repo, _spawn_service, _stop_service, _striatum_init

    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, envelope = _post_invoke(
            port,
            {
                "argv": [
                    "override-verdict",
                    "--session-id", "sess_x",
                    "--job-id", "job_DIFFERENT",
                    "--verdict", "accept",
                    "--rationale", "test",
                ],
                "web_context": {
                    "kind": "override_verdict",
                    "run_id": "run_a",
                    "job_id": "job_y",
                    "session_id": "sess_x",
                    "token": "deadbeef" * 4,
                },
            },
            origin=f"http://127.0.0.1:{port}",
        )
        assert status == 403, envelope
        assert "web_context" in envelope["error"]["message"]
    finally:
        _stop_service(proc)


def test_invoke_refuses_override_verdict_with_forged_token(tmp_path: Path) -> None:
    from test_web_ui import _git_init_repo, _spawn_service, _stop_service, _striatum_init

    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, envelope = _post_invoke(
            port,
            {
                "argv": [
                    "override-verdict",
                    "--session-id", "sess_x",
                    "--job-id", "job_y",
                    "--verdict", "accept",
                    "--rationale", "test",
                ],
                "web_context": {
                    "kind": "override_verdict",
                    "run_id": "run_a",
                    "job_id": "job_y",
                    "session_id": "sess_x",
                    "token": "0" * 32,  # wrong HMAC
                },
            },
            origin=f"http://127.0.0.1:{port}",
        )
        assert status == 403, envelope
        assert "token" in envelope["error"]["message"]
    finally:
        _stop_service(proc)


def test_invoke_refuses_override_verdict_with_wrong_kind(tmp_path: Path) -> None:
    from test_web_ui import _git_init_repo, _spawn_service, _stop_service, _striatum_init

    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, envelope = _post_invoke(
            port,
            {
                "argv": [
                    "override-verdict",
                    "--session-id", "sess_x",
                    "--job-id", "job_y",
                    "--verdict", "accept",
                    "--rationale", "test",
                ],
                "web_context": {
                    "kind": "totally_other_action",
                    "run_id": "run_a",
                    "job_id": "job_y",
                    "session_id": "sess_x",
                    "token": "ab" * 16,
                },
            },
            origin=f"http://127.0.0.1:{port}",
        )
        assert status == 403, envelope
    finally:
        _stop_service(proc)
