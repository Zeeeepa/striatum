"""RFC 0023 V1: /view/<path> endpoint tests."""

from __future__ import annotations

from pathlib import Path

from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)


def test_view_md_renders_html(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "doc.md").write_text("# Hello\n\nThis is **bold**.\n", encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/view/doc.md")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        assert b"<h1>Hello</h1>" in body
        assert b"<strong>bold</strong>" in body
    finally:
        _stop_service(proc)


def test_view_text_renders_pre(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "snippet.py").write_text("def hello():\n    return 1\n", encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/view/snippet.py")
        assert status == 200
        assert b'id="island-code-viewer"' in body
        assert b"/static/build/island-code-viewer.js" in body
        assert b"<pre" in body
        assert b"def hello" in body
    finally:
        _stop_service(proc)


def test_view_root_renders_tree_browser_island(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "doc.md").write_text("# Hello\n", encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/view/")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        assert b'id="island-tree-browser"' in body
        assert b"/static/build/island-tree-browser.js" in body
    finally:
        _stop_service(proc)


def test_view_root_without_trailing_slash_renders_tree_browser_island(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/view")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        assert b'id="island-tree-browser"' in body
    finally:
        _stop_service(proc)


def test_view_path_traversal_refused(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        for path in (
            "/view/../../etc/passwd",
            "/view/../../../etc/shadow",
        ):
            status, _, _ = _http_get_raw(port, path)
            assert status in (400, 404), f"path {path} did not refuse: {status}"
    finally:
        _stop_service(proc)


def test_view_dotgit_hidden(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/view/.git/HEAD")
        assert status == 404
    finally:
        _stop_service(proc)


def test_view_dotstriatum_hidden(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/view/.striatum/state.sqlite3")
        assert status == 404
    finally:
        _stop_service(proc)


def test_view_directory_404(tmp_path: Path) -> None:
    """V1 doesn't support directory listings; full file-tree UI is V1.5."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "subdir").mkdir()
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/view/subdir")
        assert status == 404
    finally:
        _stop_service(proc)


def test_view_binary_metadata_panel(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "blob.png").write_bytes(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/view/blob.png")
        assert status == 200
        assert b"Binary file" in body
    finally:
        _stop_service(proc)


def test_view_missing_file_404(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/view/does-not-exist.md")
        assert status == 404
    finally:
        _stop_service(proc)
