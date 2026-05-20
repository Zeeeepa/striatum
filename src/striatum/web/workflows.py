"""RFC 0024 V1: workflow file discovery for the browser surface.

Walks the target repo for ``**/workflow.json`` files (skipping the
usual hidden / build / vendored directories), parses each, runs
``validate_workflow`` in a try/except, and returns a list of dicts
the index page + chat tool consume.
"""

from __future__ import annotations

import json
from collections.abc import Callable
from datetime import UTC, datetime
from dataclasses import dataclass
from pathlib import Path
from typing import Any

__all__ = [
    "WorkflowFileError",
    "WorkflowDetailPageResponse",
    "WorkflowRouteContext",
    "discover",
    "list_repo_tree",
    "load_workflow_at",
    "render_workflow_detail_page",
    "render_workflows_index_page",
    "workflow_detail_page_response",
    "workflow_index_page_response",
]

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
HtmlSender = Callable[[int, str], None]
TemplateEnvFactory = Callable[[], Any]
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


@dataclass(frozen=True)
class WorkflowDetailPageResponse:
    workflow: JsonObject
    graph_svg: str | None


@dataclass(frozen=True)
class WorkflowRouteContext:
    repo: Path
    allow_mutations: bool
    headers: Any
    rfile: Any
    send_json: JsonSender
    send_html: HtmlSender
    jinja_env: TemplateEnvFactory


def render_workflows_index_page(ctx: WorkflowRouteContext) -> None:
    try:
        workflows = workflow_index_page_response(ctx.repo)
        html = ctx.jinja_env().get_template("workflows_index.html").render(
            workflows=workflows,
        )
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def render_workflow_detail_page(ctx: WorkflowRouteContext, rel_path: str) -> None:
    try:
        response = workflow_detail_page_response(ctx.repo, rel_path)
        html = ctx.jinja_env().get_template("workflow_detail.html").render(
            workflow=response.workflow,
            graph_svg=response.graph_svg,
        )
        ctx.send_html(200, html)
    except WorkflowFileError as exc:
        ctx.send_json(exc.status_code, _error(exc.status_code, exc.message))
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


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


def workflow_index_page_response(repo: Path) -> list[JsonObject]:
    """Return workflow index entries without parsed workflow bodies."""
    return [{k: v for k, v in entry.items() if k != "data"} for entry in discover(repo)]


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


def workflow_detail_page_response(repo: Path, rel_path: str) -> WorkflowDetailPageResponse:
    """Return workflow detail page data for a safe repo-relative path."""
    from striatum.web.graph_svg import render_run_graph

    if not rel_path:
        raise WorkflowFileError(404, "missing path")
    if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
        raise WorkflowFileError(400, "invalid path")
    entry = load_workflow_at(repo, rel_path)
    if entry is None:
        raise WorkflowFileError(404, "workflow not found")

    graph_svg: str | None = None
    data = entry.get("data")
    if isinstance(data, dict) and entry.get("status") == "valid":
        try:
            graph_svg = render_run_graph(data, node_states={}, run_id=None)
        except Exception:  # noqa: BLE001
            graph_svg = None
    if isinstance(data, dict):
        _annotate_workflow_doc_links(repo, rel_path, data)
    return WorkflowDetailPageResponse(workflow=entry, graph_svg=graph_svg)


def _annotate_workflow_doc_links(repo: Path, workflow_rel_path: str, data: dict[str, Any]) -> None:
    """Walk a parsed workflow JSON and attach `_view_link` strings for
    every prompt-shaped path (context_docs, role definitions,
    per-job task_prompts).

    Paths in workflow.json are sometimes workflow-relative
    (``prompts/draft.md``) and sometimes repo-relative
    (``examples/foo/roles/author.md``). The resolver checks both
    relative to ``repo``; the first existing file wins. When neither
    exists, the workflow-relative form is recorded — clicking through
    will land on the /view/ route, which can render a "missing"
    message itself.
    """
    workflow_dir = "/".join(workflow_rel_path.split("/")[:-1])

    def resolve(path: str) -> str:
        if not isinstance(path, str) or not path:
            return ""
        cleaned = path.lstrip("./")
        candidates: list[str] = []
        if cleaned:
            candidates.append(cleaned)
        if workflow_dir and cleaned and not cleaned.startswith(workflow_dir + "/"):
            candidates.append(workflow_dir + "/" + cleaned)
        for candidate in candidates:
            full = (repo / candidate).resolve()
            try:
                full.relative_to(repo.resolve())
            except ValueError:
                continue
            if full.is_file():
                return candidate
        return candidates[-1] if candidates else cleaned

    context_docs = data.get("context_docs")
    if isinstance(context_docs, list):
        for doc in context_docs:
            if isinstance(doc, dict):
                doc_path = doc.get("path")
                if isinstance(doc_path, str):
                    doc["_view_link"] = resolve(doc_path)

    roles = data.get("roles")
    if isinstance(roles, dict):
        for role in roles.values():
            if isinstance(role, dict):
                role_path = role.get("definition_path")
                if isinstance(role_path, str):
                    role["_view_link"] = resolve(role_path)

    jobs = data.get("jobs")
    if isinstance(jobs, list):
        for job in jobs:
            if not isinstance(job, dict):
                continue
            task_prompt = job.get("task_prompt")
            if isinstance(task_prompt, dict):
                prompt_path = task_prompt.get("path")
                if isinstance(prompt_path, str):
                    task_prompt["_view_link"] = resolve(prompt_path)


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


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}
