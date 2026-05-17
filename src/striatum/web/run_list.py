"""Run-list presentation helpers for the local web UI."""

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any, Mapping, Protocol
from urllib.parse import quote, urlsplit

JsonObject = dict[str, Any]

__all__ = [
    "git_config_get",
    "git_symbolic_ref",
    "parse_github_remote",
    "repo_relative_source_path",
    "run_list_view_item",
    "state_chip",
    "workflow_tree_url",
]


class RunListState(Protocol):
    repo: Path

    def github_base_url(self) -> str | None: ...

    def default_branch(self) -> str: ...


def git_config_get(repo: Path, key: str) -> str | None:
    try:
        result = subprocess.run(
            ["git", "-C", str(repo), "config", "--get", key],
            text=True,
            capture_output=True,
            check=False,
        )
    except OSError:
        return None
    if result.returncode != 0:
        return None
    value = result.stdout.strip()
    return value or None


def git_symbolic_ref(repo: Path, ref: str) -> str | None:
    try:
        result = subprocess.run(
            ["git", "-C", str(repo), "symbolic-ref", "--short", ref],
            text=True,
            capture_output=True,
            check=False,
        )
    except OSError:
        return None
    if result.returncode != 0:
        return None
    value = result.stdout.strip()
    return value or None


def parse_github_remote(value: str | None) -> str | None:
    if not value:
        return None
    raw = value.strip()
    if raw.startswith("git@github.com:"):
        path = raw.removeprefix("git@github.com:")
    elif raw.startswith("ssh://git@github.com/"):
        path = raw.removeprefix("ssh://git@github.com/")
    else:
        parsed = urlsplit(raw)
        if parsed.scheme not in {"http", "https"} or parsed.netloc.lower() != "github.com":
            return None
        path = parsed.path.lstrip("/")
    path = path.removesuffix(".git").strip("/")
    parts = [part for part in path.split("/") if part]
    if len(parts) != 2:
        return None
    owner, repo = parts
    return f"https://github.com/{quote(owner, safe='')}/{quote(repo, safe='')}"


def _workflow_directory(source_path: str) -> str:
    path = Path(source_path)
    if path.name:
        parent = path.parent.as_posix()
        return "" if parent == "." else parent
    return source_path.strip("/")


def repo_relative_source_path(repo: Path, source_path: str) -> str:
    if not source_path:
        return ""
    path = Path(source_path)
    if not path.is_absolute():
        return source_path
    try:
        return path.resolve().relative_to(repo.resolve()).as_posix()
    except ValueError:
        return source_path


def workflow_tree_url(
    base_url: str | None,
    branch: str | None,
    source_path: str | None,
) -> str | None:
    if not base_url or not branch or not source_path:
        return None
    directory = _workflow_directory(source_path)
    branch_part = quote(branch, safe="")
    if not directory:
        return f"{base_url}/tree/{branch_part}"
    return f"{base_url}/tree/{branch_part}/{quote(directory, safe='/-._~')}"


def state_chip(kind: str, state: Any) -> JsonObject:
    normalized = str(state or "unknown")
    return {
        "kind": kind,
        "state": normalized,
        "label": normalized,
        "css_class": f"status-pill status-{normalized}",
    }


def run_list_view_item(state: RunListState, raw: Mapping[str, Any]) -> dict[str, Any]:
    run = dict(raw)
    identity_raw = run.get("workflow_identity")
    identity = identity_raw if isinstance(identity_raw, Mapping) else {}
    workflow_id = str(run.get("workflow_id") or identity.get("workflow_id") or "")
    workflow_name = str(run.get("workflow_name") or workflow_id)
    workflow_version = run.get("workflow_version") or identity.get("workflow_version")
    workflow_snapshot_id = run.get("workflow_snapshot_id") or identity.get("workflow_snapshot_id")
    run["workflow_id"] = workflow_id
    run["workflow_name"] = workflow_name
    run["workflow_identity"] = {
        "workflow_id": workflow_id or None,
        "workflow_version": workflow_version,
        "workflow_snapshot_id": workflow_snapshot_id,
    }
    source_path = repo_relative_source_path(
        state.repo,
        str(run.get("source_path") or run.get("workflow_source_path") or ""),
    )
    run["workflow_source_path"] = source_path
    run["workflow_local_url"] = (
        f"/workflows/{quote(source_path, safe='/-._~')}" if source_path else None
    )
    run["workflow_github_url"] = workflow_tree_url(
        state.github_base_url(),
        state.default_branch(),
        source_path,
    )
    run["state_chip"] = state_chip("run", run.get("state"))
    return run
