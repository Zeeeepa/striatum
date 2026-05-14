"""Static contract tests for copy-on-click web assets."""

from __future__ import annotations

from pathlib import Path

from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)


ROOT = Path(__file__).resolve().parents[1]


def test_copy_on_click_asset_contract() -> None:
    script = (ROOT / "src/striatum/web/static/copy_on_click.js").read_text(encoding="utf-8")
    assert "window.StriatumCopyOnClick" in script
    assert "init" in script
    assert "initializedRoots" in script
    assert '"[data-copy]"' in script
    assert "navigator.clipboard.writeText" in script
    assert 'event.key !== "Enter"' in script
    assert 'showToast("copied")' in script
    assert 'role", "status"' in script
    assert 'aria-live", "polite"' in script
    assert "position: \"fixed\"" in script
    # GH #12: copy activation must be gated by a closed allowlist of
    # container selectors, not the global `[data-copy]` selector.
    assert "ALLOWED_COPY_CONTAINER_SELECTORS" in script
    assert "isAllowedCopyTarget" in script
    assert '".recipe-list"' in script
    assert '".recovery-auto-publish"' in script
    assert '".code-recipe"' in script
    assert '".copyable-token"' in script
    # The allowlist gate must run in both the document-level click handler
    # path (`findCopyTarget`) and the affordance-decoration path
    # (`enhanceTargets`). Both call sites use `isAllowedCopyTarget`.
    assert script.count("isAllowedCopyTarget") >= 3


def test_base_js_wires_copy_on_click_on_dom_content_loaded() -> None:
    script = (ROOT / "src/striatum/web/static/base.js").read_text(encoding="utf-8")
    assert 'script.src = "/static/copy_on_click.js"' in script
    assert "initCopyOnClick" in script
    assert "window.StriatumCopyOnClick" in script
    assert "api.init(document)" in script
    assert "document.addEventListener(\"DOMContentLoaded\"" in script


def test_copy_on_click_asset_is_served_when_web_enabled(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/static/copy_on_click.js")
        assert status == 200
        assert "javascript" in headers.get("Content-Type", "")
        assert b"StriatumCopyOnClick" in body
    finally:
        _stop_service(proc)
