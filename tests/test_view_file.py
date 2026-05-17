from __future__ import annotations

from pathlib import Path

import pytest

from striatum.web.view_file import ViewFileError, ViewRouteContext, render_view_path, view_file_payload


def test_view_file_payload_renders_markdown(tmp_path: Path) -> None:
    (tmp_path / "doc.md").write_text("# Hello\n\nBody\n", encoding="utf-8")

    payload = view_file_payload(tmp_path, "doc.md")

    assert payload["rel_path"] == "doc.md"
    assert payload["text_body"] is None
    assert "<h1>Hello</h1>" in str(payload["rendered_html"])


def test_view_file_payload_returns_text_body(tmp_path: Path) -> None:
    (tmp_path / "snippet.py").write_text("def hello():\n    return 1\n", encoding="utf-8")

    payload = view_file_payload(tmp_path, "snippet.py")

    assert payload["rendered_html"] is None
    assert "def hello" in str(payload["text_body"])
    assert payload["lang"] == "py"


def test_view_file_payload_reports_binary_file(tmp_path: Path) -> None:
    (tmp_path / "blob.png").write_bytes(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100)

    payload = view_file_payload(tmp_path, "blob.png")

    assert payload["rendered_html"] is None
    assert payload["text_body"] is None
    assert "Binary file" in str(payload["message"])


def test_render_view_path_does_not_consult_legacy_sqlite_breadcrumb(tmp_path: Path) -> None:
    (tmp_path / "doc.md").write_text("# Hello\n", encoding="utf-8")
    sent: dict[str, object] = {}

    class Template:
        def render(self, **payload: object) -> str:
            sent["payload"] = payload
            return "html"

    class Env:
        def get_template(self, name: str) -> Template:
            assert name == "view_file.html"
            return Template()

    ctx = ViewRouteContext(
        repo=tmp_path,
        send_json=lambda status, body: sent.update({"json": (status, body)}),
        send_html=lambda status, html: sent.update({"html": (status, html)}),
        jinja_env=lambda: Env(),
    )

    render_view_path(ctx, "doc.md")

    assert sent["html"] == (200, "html")
    payload = sent["payload"]
    assert isinstance(payload, dict)
    assert payload["run_breadcrumb"] is None


@pytest.mark.parametrize(
    ("subpath", "status_code"),
    [
        ("../outside.md", 400),
        (".git/HEAD", 404),
        (".striatum/state.sqlite3", 404),
        ("missing.md", 404),
    ],
)
def test_view_file_payload_refuses_unsafe_or_missing_paths(
    tmp_path: Path,
    subpath: str,
    status_code: int,
) -> None:
    with pytest.raises(ViewFileError) as exc_info:
        view_file_payload(tmp_path, subpath)

    assert exc_info.value.status_code == status_code
