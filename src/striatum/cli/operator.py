"""Read-only operator progress-surface commands."""

from __future__ import annotations

from pathlib import Path

from striatum.artifacts import parse_artifact_front_matter
from striatum.errors import StriatumError


BRIEF_FILENAME = "BRIEF.md"


def current_brief(repo: Path, *, operator_docs_root: str | None = None) -> dict[str, object]:
    """Read and validate the current operator brief.

    This is intentionally local and read-only. RFC 0058 keeps the operator
    progress surface in repository Markdown provenance, not daemon state.
    """
    docs_root = _resolve_operator_docs_root(repo, operator_docs_root)
    brief_path = docs_root / BRIEF_FILENAME
    if brief_path.is_symlink():
        raise StriatumError(
            f"operator brief must be a regular file, not a symlink: {brief_path}",
            exit_code=8,
        )
    if not brief_path.exists():
        raise StriatumError(
            f"operator brief not found: {brief_path}",
            exit_code=8,
        )
    if not brief_path.is_file():
        raise StriatumError(
            f"operator brief must be a regular file: {brief_path}",
            exit_code=8,
        )

    payload = brief_path.read_bytes()
    front_matter = parse_artifact_front_matter(
        kind="operator_brief",
        path=brief_path,
        payload=payload,
    )
    if front_matter is None:
        raise StriatumError(
            f"operator brief is missing operator_brief front matter: {brief_path}",
            exit_code=8,
        )
    if front_matter.get("status") != "current":
        raise StriatumError(
            "operator brief is not current: "
            f"{brief_path} has status {front_matter.get('status')!r}",
            exit_code=8,
        )

    return {
        "path": str(brief_path),
        "brief_id": front_matter["brief_id"],
        "supersedes": front_matter["supersedes"],
        "scope_links": front_matter["scope_links"],
        "context_budget_lines": front_matter["context_budget_lines"],
        "retrieval_priority": front_matter["retrieval_priority"],
        "cold_start_paths": _cold_start_paths(repo, brief_path, front_matter),
    }


def format_current_brief(payload: dict[str, object]) -> str:
    """Render current-brief data for non-JSON CLI output."""
    scope_links = payload.get("scope_links")
    cold_start_paths = payload.get("cold_start_paths")
    return "\n".join(
        [
            f"path: {payload['path']}",
            f"brief_id: {payload['brief_id']}",
            f"supersedes: {payload['supersedes']}",
            f"scope_links: {_format_list(scope_links)}",
            f"context_budget_lines: {payload['context_budget_lines']}",
            f"retrieval_priority: {payload['retrieval_priority']}",
            f"cold_start_paths: {_format_list(cold_start_paths)}",
        ]
    )


def _resolve_operator_docs_root(repo: Path, operator_docs_root: str | None) -> Path:
    if operator_docs_root is None:
        return repo / "docs" / "operator"
    root = Path(operator_docs_root).expanduser()
    if not root.is_absolute():
        root = repo / root
    return root.resolve()


def _cold_start_paths(
    repo: Path, brief_path: Path, front_matter: dict[str, object]
) -> list[str]:
    paths = ["AGENTS.md"]
    if (repo / "CLAUDE.md").exists():
        paths.append("CLAUDE.md")
    try:
        paths.append(str(brief_path.relative_to(repo)))
    except ValueError:
        paths.append(str(brief_path))
    scope_links = front_matter.get("scope_links")
    if isinstance(scope_links, list):
        paths.extend(str(item) for item in scope_links)
    return paths


def _format_list(value: object) -> str:
    if not isinstance(value, list):
        return "[]"
    return ", ".join(str(item) for item in value)
