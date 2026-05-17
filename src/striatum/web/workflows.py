"""RFC 0024 V1: workflow file discovery for the browser surface.

Walks the target repo for ``**/workflow.json`` files (skipping the
usual hidden / build / vendored directories), parses each, runs
``validate_workflow`` in a try/except, and returns a list of dicts
the index page + chat tool consume.
"""

from __future__ import annotations

import json
import hashlib
from datetime import UTC, datetime
from dataclasses import dataclass
from pathlib import Path
from typing import Any

__all__ = [
    "WorkflowFileError",
    "discover",
    "list_repo_tree",
    "load_workflow_at",
    "save_workflow_file",
    "workflow_edit_payload",
]

_MAX_LINT_WARNING_MESSAGES = 3
_MAX_LINT_WARNING_MESSAGE_CHARS = 220


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


@dataclass(frozen=True)
class _WorkflowPath:
    target: Path
    rel_norm: str
    rel_parts: tuple[str, ...]


class WorkflowFileError(Exception):
    def __init__(
        self,
        status_code: int,
        message: str,
        *,
        errors: list[dict[str, str]] | None = None,
        current_sha256: str | None = None,
    ) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.message = message
        self.errors = errors
        self.current_sha256 = current_sha256


def _resolve_edit_path(repo: Path, rel_path: str) -> _WorkflowPath:
    if not rel_path:
        raise WorkflowFileError(404, "missing path")
    if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
        raise WorkflowFileError(400, "invalid path")
    repo_root = repo.resolve()
    target = (repo / rel_path).resolve()
    try:
        target.relative_to(repo_root)
    except ValueError as exc:
        raise WorkflowFileError(400, "path escapes repo") from exc
    rel_parts = target.relative_to(repo_root).parts
    if rel_parts and rel_parts[0] in (".git", ".striatum"):
        raise WorkflowFileError(404, "hidden path")
    return _WorkflowPath(target=target, rel_norm="/".join(rel_parts), rel_parts=rel_parts)


def workflow_edit_payload(repo: Path, rel_path: str) -> dict[str, Any]:
    """Return template data for the workflow visual editor."""
    resolved = _resolve_edit_path(repo, rel_path)
    target = resolved.target
    is_new = not target.is_file()
    if is_new:
        stem = (
            resolved.rel_parts[-2]
            if len(resolved.rel_parts) >= 2
            else (resolved.rel_parts[0] if resolved.rel_parts else "new-workflow")
        )
        workflow_data: dict[str, Any] = {
            "schema_version": "striatum.workflow.v1",
            "workflow_id": stem,
            "workflow_version": "1",
            "name": "",
            "branch": {"mode": "confirm", "suggested_name": f"wf/{stem}", "allow_dirty": False},
            "coordinator": {"role_id": "", "lane_id": ""},
            "lanes": {},
            "roles": {},
            "context_docs": [],
            "parallelism": {
                "mode": "declared",
                "max_active_jobs": 1,
                "require_disjoint_write_scopes": True,
            },
            "jobs": [],
            "edges": [],
            "cycles": [],
        }
        sha256 = ""
    else:
        try:
            workflow_data = json.loads(target.read_text(encoding="utf-8"))
            if not isinstance(workflow_data, dict):
                workflow_data = {}
        except (OSError, json.JSONDecodeError):
            workflow_data = {}
        try:
            sha256 = hashlib.sha256(target.read_bytes()).hexdigest()
        except OSError:
            sha256 = ""
    return {
        "rel_path": resolved.rel_norm,
        "is_new": is_new,
        "workflow_data": workflow_data,
        "workflow_sha256": sha256,
    }


def save_workflow_file(
    repo: Path,
    rel_path: str,
    data: dict[str, Any],
    *,
    if_match: str = "",
) -> dict[str, str]:
    """Validate and atomically write a workflow JSON file."""
    from striatum.errors import WorkflowError
    from striatum.workflow import validate_workflow

    resolved = _resolve_edit_path(repo, rel_path)
    target = resolved.target
    try:
        validate_workflow(data)
    except WorkflowError as exc:
        errors: list[dict[str, str]] = []
        field_path = getattr(exc, "field_path", None)
        if isinstance(field_path, str) and field_path:
            errors.append({"field_path": field_path, "message": str(exc)})
        raise WorkflowFileError(422, str(exc), errors=errors) from exc
    except Exception as exc:  # noqa: BLE001
        raise WorkflowFileError(422, f"{type(exc).__name__}: {exc}", errors=[]) from exc

    expected_sha = if_match.strip().strip('"')
    if expected_sha and target.is_file():
        try:
            current_sha = hashlib.sha256(target.read_bytes()).hexdigest()
        except OSError as exc:
            raise WorkflowFileError(500, f"read failed: {exc}") from exc
        if current_sha != expected_sha:
            raise WorkflowFileError(
                412,
                "If-Match precondition failed; file changed on disk",
                current_sha256=current_sha,
            )

    try:
        target.parent.mkdir(parents=True, exist_ok=True)
        tmp = target.with_suffix(target.suffix + ".tmp")
        tmp.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
        if expected_sha and target.is_file():
            try:
                final_sha = hashlib.sha256(target.read_bytes()).hexdigest()
            except OSError:
                final_sha = expected_sha
            if final_sha != expected_sha:
                tmp.unlink(missing_ok=True)
                raise WorkflowFileError(
                    412,
                    "If-Match precondition failed; file changed during validate",
                    current_sha256=final_sha,
                )
        tmp.replace(target)
    except WorkflowFileError:
        raise
    except OSError as exc:
        raise WorkflowFileError(500, f"write failed: {exc}") from exc
    new_sha = hashlib.sha256(target.read_bytes()).hexdigest()
    return {"path": resolved.rel_norm, "status": "saved", "sha256": new_sha}


def _modified_at(path: Path) -> str | None:
    try:
        modified = datetime.fromtimestamp(path.stat().st_mtime, tz=UTC)
        return modified.isoformat().replace("+00:00", "Z")
    except OSError:
        return None


def _shorten_message(message: str) -> str:
    compact = " ".join(message.split())
    if len(compact) <= _MAX_LINT_WARNING_MESSAGE_CHARS:
        return compact
    return compact[: _MAX_LINT_WARNING_MESSAGE_CHARS - 3].rstrip() + "..."


def _lint_summary(workflow: dict[str, Any], *, repo_root: Path) -> dict[str, Any]:
    """Return a non-failing lint summary for web display."""
    from striatum.workflow import lint_workflow

    try:
        payload = lint_workflow(workflow, repo_root=repo_root)
    except Exception as exc:  # noqa: BLE001
        return {
            "lint_warning_count": 0,
            "lint_warnings": [],
            "lint_error": f"{type(exc).__name__}: {exc}"[:200],
        }

    raw_warnings = payload.get("warnings")
    warnings = raw_warnings if isinstance(raw_warnings, list) else []
    raw_count = payload.get("warning_count")
    warning_count = raw_count if isinstance(raw_count, int) and not isinstance(raw_count, bool) else len(warnings)
    short: list[dict[str, str | None]] = []
    for warning in warnings[:_MAX_LINT_WARNING_MESSAGES]:
        if isinstance(warning, dict):
            raw_message = warning.get("message")
            raw_rule = warning.get("rule")
            message = raw_message if isinstance(raw_message, str) else ""
            rule = raw_rule if isinstance(raw_rule, str) and raw_rule else None
        else:
            message = str(warning)
            rule = None
        if not message:
            continue
        short.append({"rule": rule, "message": _shorten_message(message)})
    return {
        "lint_warning_count": warning_count,
        "lint_warnings": short,
        "lint_error": None,
    }


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
        entry: dict[str, Any] = {
            "path": rel,
            "modified_at": _modified_at(path),
            "lint_warning_count": 0,
            "lint_warnings": [],
            "lint_error": None,
        }
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
        entry.update(_lint_summary(data, repo_root=repo))
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
    entry: dict[str, Any] = {
        "path": rel,
        "modified_at": _modified_at(target),
        "lint_warning_count": 0,
        "lint_warnings": [],
        "lint_error": None,
    }
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
    entry.update(_lint_summary(data, repo_root=repo_root))
    entry["data"] = data
    return entry


def list_repo_tree(repo: Path, rel_path: str) -> dict[str, Any] | None:
    """Return a safe, shallow directory listing for the web tree browser.

    The response is repo-relative and hides runner/private implementation
    state by default. Returns ``None`` for unsafe, hidden, missing, or
    non-directory paths so the service can choose the HTTP status.
    """
    if not isinstance(rel_path, str):
        return None
    if rel_path in ("", "."):
        rel_path = ""
    if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
        return None
    repo_root = repo.resolve()
    target = (repo / rel_path).resolve()
    try:
        rel_parts = target.relative_to(repo_root).parts
    except ValueError:
        return None
    if any(part in _SKIP_DIRS for part in rel_parts):
        return None
    if not target.is_dir():
        return None

    entries: list[dict[str, Any]] = []
    try:
        children = sorted(target.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower()))
    except OSError:
        return None
    for child in children:
        try:
            child_parts = child.resolve().relative_to(repo_root).parts
        except (OSError, ValueError):
            continue
        if any(part in _SKIP_DIRS for part in child_parts):
            continue
        if child.is_dir():
            kind = "dir"
            size = None
        elif child.is_file():
            kind = "file"
            try:
                size = child.stat().st_size
            except OSError:
                size = None
        else:
            continue
        entries.append(
            {
                "name": child.name,
                "path": "/".join(child_parts),
                "kind": kind,
                "size": size,
            }
        )
    return {"path": "/".join(rel_parts), "entries": entries}
