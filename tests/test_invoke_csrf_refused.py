# ruff: noqa
"""GH #9 regression: ``/v1/invoke`` CSRF mitigations.

Two attack shapes must fail closed on the TCP ``/v1/invoke`` surface:

- ``Content-Type: text/plain`` (or any non-``application/json``) — the
  classic CORS "simple request" CSRF, since browsers do not preflight
  these and a cross-origin ``<form enctype="text/plain">`` would
  otherwise execute the body as CLI argv.
- Cross-origin ``Origin`` / ``Referer`` header — caught regardless of
  Content-Type, so fetch-based CSRF is also rejected.

Same-origin browser POSTs continue to work. Unauthenticated non-browser
API clients that omit Origin / Referer fail closed; authenticated Bearer
clients remain viable.
"""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite / web_ui test helpers are retired", allow_module_level=True)

import json
import urllib.error
import urllib.request
from pathlib import Path

from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)


def _post_raw(
    port: int,
    path: str,
    body: bytes,
    *,
    content_type: str | None,
    origin: str | None = None,
    referer: str | None = None,
    host: str | None = None,
    authorization: str | None = None,
) -> tuple[int, bytes]:
    headers: dict[str, str] = {}
    if content_type is not None:
        headers["Content-Type"] = content_type
    if origin is not None:
        headers["Origin"] = origin
    if referer is not None:
        headers["Referer"] = referer
    if host is not None:
        headers["Host"] = host
    if authorization is not None:
        headers["Authorization"] = authorization
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        data=body,
        headers=headers,
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read()


def test_invoke_refuses_text_plain_content_type(tmp_path: Path) -> None:
    """GH #9 mitigation 1: simple-request CSRF must fail with 415."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="text/plain",
            origin=f"http://127.0.0.1:{port}",
        )
        envelope = json.loads(body)
        assert status == 415, envelope
        assert envelope["ok"] is False
        assert envelope["error"]["code"] == 415
        assert "application/json" in envelope["error"]["message"]
    finally:
        _stop_service(proc)


def test_invoke_refuses_form_url_encoded_content_type(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/x-www-form-urlencoded",
            origin=f"http://127.0.0.1:{port}",
        )
        envelope = json.loads(body)
        assert status == 415, envelope
    finally:
        _stop_service(proc)


def test_invoke_refuses_missing_content_type(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type=None,
            origin=f"http://127.0.0.1:{port}",
        )
        envelope = json.loads(body)
        assert status == 415, envelope
    finally:
        _stop_service(proc)


def test_invoke_refuses_malformed_or_non_exact_json_content_type(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        for content_type in (
            "application/jsonx",
            "text/application/json",
            "application/json, text/plain",
            "application/json; bogus",
        ):
            status, body = _post_raw(
                port,
                "/v1/invoke",
                json.dumps({"argv": ["status"]}).encode("utf-8"),
                content_type=content_type,
                origin=f"http://127.0.0.1:{port}",
            )
            envelope = json.loads(body)
            assert status == 415, (content_type, envelope)
    finally:
        _stop_service(proc)


def test_invoke_accepts_application_json_with_charset(tmp_path: Path) -> None:
    """The strict match must still accept the charset parameter."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json; charset=utf-8",
            origin=f"http://127.0.0.1:{port}",
        )
        envelope = json.loads(body)
        # Must pass the strict Content-Type gate. The inner status verb
        # may succeed or fail depending on whether a run exists; either
        # is fine here — we only care that 415 is not returned.
        assert status != 415, envelope
    finally:
        _stop_service(proc)


def test_invoke_rejects_malformed_json_with_correct_ct(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            b"{not json",
            content_type="application/json",
            origin=f"http://127.0.0.1:{port}",
        )
        envelope = json.loads(body)
        assert status == 400
        assert envelope["error"]["code"] == 400
    finally:
        _stop_service(proc)


def test_invoke_refuses_cross_origin_post(tmp_path: Path) -> None:
    """GH #9 mitigation 2: an Origin that doesn't match Host is 403."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
            origin="http://evil.example",
        )
        envelope = json.loads(body)
        assert status == 403, envelope
        assert envelope["error"]["code"] == 403
    finally:
        _stop_service(proc)


def test_invoke_refuses_null_and_malformed_origin(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        for origin in ("null", "not a url", "http://[::1"):
            status, body = _post_raw(
                port,
                "/v1/invoke",
                json.dumps({"argv": ["status"]}).encode("utf-8"),
                content_type="application/json",
                origin=origin,
            )
            envelope = json.loads(body)
            assert status == 403, (origin, envelope)
    finally:
        _stop_service(proc)


def test_invoke_refuses_dns_rebinding_host_origin_pair(tmp_path: Path) -> None:
    """Host and Origin agreeing with each other is not sufficient.

    The allowed origin is derived from the service's bound loopback
    listener, so a DNS-rebinding-shaped request to the loopback socket
    with Host/Origin set to an attacker domain fails closed.
    """
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
            host=f"evil.example:{port}",
            origin=f"http://evil.example:{port}",
        )
        envelope = json.loads(body)
        assert status == 403, envelope
    finally:
        _stop_service(proc)


def test_invoke_accepts_same_origin_post(tmp_path: Path) -> None:
    """Browser POST from the rendered page (Origin matches Host)."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
            origin=f"http://127.0.0.1:{port}",
        )
        envelope = json.loads(body)
        # Allowed past the origin gate; the inner status verb still
        # fails because there is no run id, but that's an envelope-error
        # not an origin-error.
        assert status != 403, envelope
    finally:
        _stop_service(proc)


def test_invoke_refuses_cross_origin_referer_when_origin_missing(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
            referer="http://evil.example/some/page",
        )
        envelope = json.loads(body)
        assert status == 403, envelope
    finally:
        _stop_service(proc)


def test_invoke_refuses_malformed_referer_when_origin_missing(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
            referer="not a url",
        )
        envelope = json.loads(body)
        assert status == 403, envelope
    finally:
        _stop_service(proc)


def test_invoke_accepts_same_origin_referer_when_origin_missing(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
            referer=f"http://127.0.0.1:{port}/run/run_x/job/job_y",
        )
        envelope = json.loads(body)
        assert status != 403, envelope
    finally:
        _stop_service(proc)


def test_invoke_refuses_unauthenticated_post_with_no_origin_or_referer(
    tmp_path: Path,
) -> None:
    """Unauthenticated POSTs fail closed when origin evidence is absent."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
        )
        envelope = json.loads(body)
        assert status == 403, envelope
    finally:
        _stop_service(proc)


def test_invoke_refuses_missing_origin_referer_without_web_ui(tmp_path: Path) -> None:
    """The /v1/invoke CSRF gate is not skipped when --web is off."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path)
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
        )
        envelope = json.loads(body)
        assert status == 403, envelope
    finally:
        _stop_service(proc)


def test_invoke_accepts_authenticated_bearer_without_origin_or_referer(
    tmp_path: Path,
) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--token", "secret")
    try:
        status, body = _post_raw(
            port,
            "/v1/invoke",
            json.dumps({"argv": ["status"]}).encode("utf-8"),
            content_type="application/json",
            authorization="Bearer secret",
        )
        envelope = json.loads(body)
        assert status != 403, envelope
        assert status != 401, envelope
    finally:
        _stop_service(proc)


def test_static_files_still_serve_after_origin_check(tmp_path: Path) -> None:
    """Same-origin enforcement applies only to POST. GET still works."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _headers, _body = _http_get_raw(port, "/v1/health")
        assert status == 200
    finally:
        _stop_service(proc)
