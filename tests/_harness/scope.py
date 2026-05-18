"""Path scope helpers for multi-repo tests."""

from __future__ import annotations

from pathlib import Path

from striatum.repo_policy import path_allowed


def resolve_artifact_path(repo: Path, path_text: str) -> Path:
    return (repo / path_text).resolve()


def in_write_scope(repo: Path, path_text: str, allowed_paths: list[str]) -> bool:
    return path_allowed(
        repo,
        path_text,
        {"allowed_paths": allowed_paths, "forbidden_paths": [".striatum/"]},
    )
