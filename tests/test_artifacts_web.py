from __future__ import annotations

from pathlib import Path

import pytest

from striatum.web.artifacts import (
    artifact_content_type,
    inline_artifact_body,
    resolve_artifact_file,
)


def test_resolve_artifact_file_requires_repo_relative_path(tmp_path: Path) -> None:
    resolved = resolve_artifact_file(tmp_path, "docs/artifact.md")

    assert resolved == (tmp_path / "docs" / "artifact.md").resolve()
    with pytest.raises(ValueError):
        resolve_artifact_file(tmp_path, "/tmp/artifact.md")
    with pytest.raises(ValueError):
        resolve_artifact_file(tmp_path, "../artifact.md")
    with pytest.raises(ValueError):
        resolve_artifact_file(tmp_path, "")


def test_artifact_content_type_uses_safe_download_defaults(tmp_path: Path) -> None:
    assert artifact_content_type(tmp_path / "a.md") == "text/markdown; charset=utf-8"
    assert artifact_content_type(tmp_path / "a.markdown") == "text/markdown; charset=utf-8"
    assert artifact_content_type(tmp_path / "a.json") == "application/json"
    assert artifact_content_type(tmp_path / "a.txt") == "text/plain; charset=utf-8"
    assert artifact_content_type(tmp_path / "a.bin") == "application/octet-stream"


def test_inline_artifact_body_renders_repo_markdown(tmp_path: Path) -> None:
    artifact = tmp_path / "docs" / "artifact.md"
    artifact.parent.mkdir()
    artifact.write_text("# Title\n\nBody\n", encoding="utf-8")

    body = inline_artifact_body(tmp_path, {"repo_path": "docs/artifact.md"})

    assert body.body_text is None
    assert body.rendered_md is not None
    assert "<h1>Title</h1>" in body.rendered_md


def test_inline_artifact_body_ignores_missing_or_escaping_paths(tmp_path: Path) -> None:
    assert inline_artifact_body(tmp_path, {"repo_path": "missing.md"}).rendered_md is None
    assert inline_artifact_body(tmp_path, {"repo_path": "../outside.md"}).rendered_md is None
