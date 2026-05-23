from __future__ import annotations

import pytest

from striatum.web.static_assets import (
    CONTENT_SECURITY_POLICY,
    StaticAssetError,
    StaticAssetRouteContext,
    content_type_for_path,
    load_static_asset,
    serve_static_asset,
)


def test_load_static_asset_reads_bundled_css() -> None:
    asset = load_static_asset("base.css")

    assert asset.content_type == "text/css; charset=utf-8"
    assert b"--bg-base" in asset.data


def test_load_static_asset_rejects_unsafe_paths() -> None:
    for relative in ("", "../service.py", "/base.css", "build/../base.css"):
        with pytest.raises(StaticAssetError) as exc_info:
            load_static_asset(relative)

        assert exc_info.value.status_code == 400
        assert exc_info.value.message == "invalid asset path"


def test_load_static_asset_reports_missing_asset() -> None:
    with pytest.raises(StaticAssetError) as exc_info:
        load_static_asset("missing.js")

    assert exc_info.value.status_code == 404
    assert exc_info.value.message == "asset not found"


def test_content_type_for_path_matches_service_contract() -> None:
    assert content_type_for_path("base.css") == "text/css; charset=utf-8"
    assert content_type_for_path("base.js") == "application/javascript; charset=utf-8"
    assert content_type_for_path("manifest.json") == "application/json"
    assert content_type_for_path("logo.svg") == "image/svg+xml"
    assert content_type_for_path("image.png") == "image/png"
    assert content_type_for_path("font.woff2") == "application/octet-stream"


def test_serve_static_asset_writes_headers_and_body() -> None:
    responses: list[int] = []
    headers: list[tuple[str, str]] = []
    bodies: list[bytes] = []

    ctx = StaticAssetRouteContext(
        send_json=lambda status, payload: pytest.fail(f"unexpected JSON {status}: {payload}"),
        send_response=responses.append,
        send_header=lambda name, value: headers.append((name, value)),
        end_headers=lambda: None,
        write_body=bodies.append,
    )

    serve_static_asset(ctx, "base.css")

    assert responses == [200]
    header_map = dict(headers)
    assert header_map["Content-Type"] == "text/css; charset=utf-8"
    assert header_map["Content-Security-Policy"] == CONTENT_SECURITY_POLICY
    assert header_map["Connection"] == "close"
    assert int(header_map["Content-Length"]) == len(bodies[0])
    assert b"--bg-base" in bodies[0]


def test_serve_static_asset_maps_lookup_error_to_json() -> None:
    payloads: list[tuple[int, dict[str, object]]] = []

    ctx = StaticAssetRouteContext(
        send_json=lambda status, payload: payloads.append((status, payload)),
        send_response=lambda status: pytest.fail(f"unexpected response {status}"),
        send_header=lambda name, value: pytest.fail(f"unexpected header {name}: {value}"),
        end_headers=lambda: pytest.fail("unexpected end_headers"),
        write_body=lambda body: pytest.fail(f"unexpected body {body!r}"),
    )

    serve_static_asset(ctx, "../service.py")

    assert payloads == [
        (400, {"ok": False, "error": {"code": 400, "message": "invalid asset path"}})
    ]
