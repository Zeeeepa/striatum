from __future__ import annotations

from pathlib import Path

import pytest

from striatum.web.artifacts import (
    ArtifactViewPayloadError,
    artifact_content_type,
    artifact_view_template_context,
    byline_line,
    inline_artifact_body,
    shape_artifact_rows,
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


def test_artifact_view_template_context_shapes_daemon_payload(tmp_path: Path) -> None:
    artifact = tmp_path / "docs" / "artifact.md"
    artifact.parent.mkdir()
    artifact.write_text("# Title\n", encoding="utf-8")

    context = artifact_view_template_context(
        tmp_path,
        {
            "run": {"run_id": "run_1"},
            "artifact": {
                "artifact_id": "art_1",
                "repo_path": "docs/artifact.md",
                "author_line": "author: reviewer-codex-001",
            },
            "expected_author_line": "author: reviewer-codex-001",
            "provenance_trail": [{"event_type": "artifact.publish"}],
        },
    )

    assert context["run"] == {"run_id": "run_1"}
    assert context["artifact"]["lane_attestation_chip"]["state"] == "attested"
    assert context["artifact"]["provenance_trail"] == [{"event_type": "artifact.publish"}]
    assert context["body_text"] is None
    assert "<h1>Title</h1>" in str(context["rendered_md"])


def test_artifact_view_template_context_preserves_existing_web_shape(tmp_path: Path) -> None:
    context = artifact_view_template_context(
        tmp_path,
        {
            "run": {"run_id": "run_1"},
            "artifact": {
                "artifact_id": "art_1",
                "repo_path": "docs/missing.md",
                "lane_attestation_chip": "verified",
            },
        },
    )

    assert context["artifact"]["lane_attestation_chip"] == "verified"
    assert context["artifact"]["provenance_trail"] == []
    assert context["rendered_md"] is None
    assert context["body_text"] is None


def test_artifact_view_template_context_rejects_missing_run_or_artifact(tmp_path: Path) -> None:
    with pytest.raises(ArtifactViewPayloadError, match="daemon artifact DTO missing fields"):
        artifact_view_template_context(
            tmp_path,
            {"run": {"run_id": "run_1"}},
        )


def test_unattested_byline_renderers_hide_forged_model_author() -> None:
    shaped = byline_line("author: author-codex-gpt-5.5-001", attested=False)

    assert shaped["display"] == "author: operator"
    assert "codex-gpt" not in shaped["display"]


def test_shape_artifact_rows_uses_recorded_author_line_not_live_session() -> None:
    shaped = shape_artifact_rows(
        artifacts=[
            {
                "artifact_id": "art_1",
                "repo_path": "docs/out.md",
                "session_id": "closed_or_lost_session",
                "author_line": "author: author-codex-gpt-5.5-001",
            }
        ],
        expected_rows=[
            {
                "path": "docs/out.md",
                "expected_author_line": "author: author-codex-gpt-5.5-001",
            }
        ],
    )

    chip = shaped[0]["lane_attestation_chip"]
    assert chip["attested"] is True
    assert chip["state"] == "attested"
    assert shaped[0]["byline_line"]["display"] == "author: author-codex-gpt-5.5-001"


def test_shape_artifact_rows_override_never_renders_attested() -> None:
    shaped = shape_artifact_rows(
        artifacts=[
            {
                "artifact_id": "art_1",
                "repo_path": "docs/out.md",
                "session_id": "sess_operator_override",
                "author_line": "author: author-codex-gpt-5.5-001",
                "attestation_override_rationale": "operator published on behalf",
            }
        ],
        expected_rows=[
            {
                "path": "docs/out.md",
                "expected_author_line": "author: author-codex-gpt-5.5-001",
            }
        ],
    )

    chip = shaped[0]["lane_attestation_chip"]
    assert chip["attested"] is False
    assert chip["state"] == "unattested"
    assert chip["reason"] == "operator_override"
    assert shaped[0]["byline_line"]["display"] == "author: operator"
    assert shaped[0]["lane_evidence_chip"]["state"] == "override"
    assert shaped[0]["lane_evidence_chip"]["rationale"] == "operator published on behalf"
