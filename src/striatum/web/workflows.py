"""RFC 0024 V1: workflow file discovery for the browser surface.

Walks the target repo for ``**/workflow.json`` files (skipping the
usual hidden / build / vendored directories), parses each, runs
``validate_workflow`` in a try/except, and returns a list of dicts
the index page + chat tool consume.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

__all__ = ["discover", "load_workflow_at"]


# Directories to skip during the rglob walk. Hidden striatum + git
# state, common Python virtualenv dirs, build outputs, vendored
# JS. Per design synthesis.
_SKIP_DIRS = frozenset({
    ".git",
    ".striatum",
    ".venv",
    "venv",
    "__pycache__",
    "node_modules",
    "build",
    "dist",
    ".mypy_cache",
    ".pytest_cache",
    ".ruff_cache",
    ".tox",
    ".coverage",
    "htmlcov",
})


def discover(repo: Path) -> list[dict[str, Any]]:
    """Discover every ``workflow.json`` under ``repo`` and report
    validation status per file. Never raises."""
    from striatum.errors import WorkflowError
    from striatum.workflow import validate_workflow

    found: list[dict[str, Any]] = []
    repo = repo.resolve()
    for path in repo.rglob("workflow.json"):
        if not path.is_file():
            continue
        try:
            rel_parts = path.relative_to(repo).parts
        except ValueError:
            # Symlink target outside repo; skip.
            continue
        if any(part in _SKIP_DIRS for part in rel_parts):
            continue
        rel = "/".join(rel_parts)
        entry: dict[str, Any] = {"path": rel}
        try:
            raw = path.read_text(encoding="utf-8")
        except OSError as exc:
            entry["status"] = "parse_error"
            entry["message"] = f"{type(exc).__name__}: {exc}"[:200]
            entry["job_count"] = 0
            entry["lane_count"] = 0
            entry["role_count"] = 0
            found.append(entry)
            continue
        try:
            data = json.loads(raw)
        except json.JSONDecodeError as exc:
            entry["status"] = "parse_error"
            entry["message"] = f"JSONDecodeError: {exc}"[:200]
            entry["job_count"] = 0
            entry["lane_count"] = 0
            entry["role_count"] = 0
            found.append(entry)
            continue
        if not isinstance(data, dict):
            entry["status"] = "parse_error"
            entry["message"] = "workflow.json root must be an object"
            entry["job_count"] = 0
            entry["lane_count"] = 0
            entry["role_count"] = 0
            found.append(entry)
            continue
        try:
            validate_workflow(data)
            status = "valid"
            message: str | None = None
        except WorkflowError as exc:
            status = "workflow_error"
            message = str(exc)[:200]
        except Exception as exc:  # noqa: BLE001
            status = "workflow_error"
            message = f"{type(exc).__name__}: {exc}"[:200]
        entry["workflow_id"] = data.get("workflow_id")
        entry["workflow_version"] = data.get("workflow_version")
        entry["status"] = status
        entry["message"] = message
        jobs = data.get("jobs") or []
        lanes = data.get("lanes") or {}
        roles = data.get("roles") or {}
        entry["job_count"] = len(jobs) if isinstance(jobs, list) else 0
        entry["lane_count"] = len(lanes) if isinstance(lanes, dict) else 0
        entry["role_count"] = len(roles) if isinstance(roles, dict) else 0
        entry["data"] = data
        found.append(entry)
    found.sort(key=lambda e: e["path"])
    return found


def load_workflow_at(repo: Path, rel_path: str) -> dict[str, Any] | None:
    """Load and validate a single workflow file at ``repo/rel_path``.

    Returns the same per-entry dict shape as ``discover()``, or
    ``None`` when the path is unsafe / hidden / missing.
    """
    from striatum.errors import WorkflowError
    from striatum.workflow import validate_workflow

    if not isinstance(rel_path, str) or rel_path == "":
        return None
    if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
        return None
    target = (repo / rel_path).resolve()
    repo_root = repo.resolve()
    try:
        rel_parts = target.relative_to(repo_root).parts
    except ValueError:
        return None
    if any(part in _SKIP_DIRS for part in rel_parts):
        return None
    if not target.is_file():
        return None
    rel = "/".join(rel_parts)
    entry: dict[str, Any] = {"path": rel}
    try:
        raw = target.read_text(encoding="utf-8")
    except OSError as exc:
        entry["status"] = "parse_error"
        entry["message"] = f"{type(exc).__name__}: {exc}"[:500]
        return entry
    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        entry["status"] = "parse_error"
        entry["message"] = f"JSONDecodeError: {exc}"[:500]
        return entry
    if not isinstance(data, dict):
        entry["status"] = "parse_error"
        entry["message"] = "workflow.json root must be an object"
        return entry
    try:
        validate_workflow(data)
        entry["status"] = "valid"
        entry["message"] = None
    except WorkflowError as exc:
        entry["status"] = "workflow_error"
        entry["message"] = str(exc)[:1000]
    except Exception as exc:  # noqa: BLE001
        entry["status"] = "workflow_error"
        entry["message"] = f"{type(exc).__name__}: {exc}"[:1000]
    entry["workflow_id"] = data.get("workflow_id")
    entry["workflow_version"] = data.get("workflow_version")
    entry["data"] = data
    return entry
