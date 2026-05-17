"""Artifact file and presentation helpers for the local web UI."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping

__all__ = [
    "InlineArtifactBody",
    "artifact_content_type",
    "inline_artifact_body",
    "resolve_artifact_file",
]


@dataclass(frozen=True)
class InlineArtifactBody:
    rendered_md: str | None = None
    body_text: str | None = None


def resolve_artifact_file(repo: Path, repo_path: object) -> Path:
    if not isinstance(repo_path, str) or not repo_path:
        raise ValueError("artifact repo_path must be a non-empty string")
    relative = Path(repo_path)
    if relative.is_absolute():
        raise ValueError("artifact repo_path must be repository-relative")
    full = (repo / relative).resolve()
    full.relative_to(repo.resolve())
    return full


def artifact_content_type(path: Path) -> str:
    suffix = path.suffix.lower()
    return {
        ".md": "text/markdown; charset=utf-8",
        ".markdown": "text/markdown; charset=utf-8",
        ".json": "application/json",
        ".txt": "text/plain; charset=utf-8",
    }.get(suffix, "application/octet-stream")


def inline_artifact_body(repo: Path, artifact: Mapping[str, Any]) -> InlineArtifactBody:
    repo_path = artifact.get("repo_path") or ""
    try:
        full = resolve_artifact_file(repo, repo_path)
        if not isinstance(repo_path, str) or not repo_path.endswith(".md") or not full.is_file():
            return InlineArtifactBody()
        from striatum.web.markdown import render as md_render

        body = full.read_text(encoding="utf-8", errors="replace")
        return InlineArtifactBody(rendered_md=md_render(body), body_text=None)
    except (ValueError, OSError):
        return InlineArtifactBody()
