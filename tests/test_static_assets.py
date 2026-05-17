from __future__ import annotations

import pytest

from striatum.web.static_assets import (
    StaticAssetError,
    content_type_for_path,
    load_static_asset,
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
