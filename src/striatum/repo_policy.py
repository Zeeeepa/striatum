"""Repository-local path and lane policy helpers."""

from __future__ import annotations

from pathlib import Path

from striatum.errors import ArtifactError
from striatum.primitives import JsonObject

STATE_DIR = ".striatum"
WORKTREES_SUBDIR = "worktrees"
ADAPTER_ENFORCEMENT_LEVELS = {
    "unsupported": 0,
    "advisory": 1,
    "advisory_strict": 2,
    "enforced": 3,
}


def state_dir(repo: Path) -> Path:
    """Return the repo-local state directory."""
    return repo / STATE_DIR


def repo_relative_path(repo: Path, path_text: str) -> Path:
    """Resolve a repo-relative path and reject escapes."""
    return _repo_relative_path(repo, path_text, allow_state=False)


def path_allowed(repo: Path, path_text: str, write_scope: JsonObject) -> bool:
    """Return whether a repo-relative path is allowed by the job write scope."""
    resolved = repo_relative_path(repo, path_text)
    allowed = write_scope.get("allowed_paths", [])
    forbidden = write_scope.get("forbidden_paths", [STATE_DIR])
    if not isinstance(allowed, list) or not isinstance(forbidden, list):
        return False
    for item in forbidden:
        if not isinstance(item, str):
            continue
        denied = _repo_relative_path(repo, item, allow_state=True).resolve()
        if resolved == denied or denied in resolved.parents:
            return False
    for item in allowed:
        if not isinstance(item, str):
            continue
        base = _repo_relative_path(repo, item, allow_state=True).resolve()
        if resolved == base or base in resolved.parents:
            return True
    return False


def lane_worktree_isolation(workflow: JsonObject, lane_id: str | None) -> str:
    """Return the lane's declared worktree isolation mode."""
    if lane_id is None:
        return "off"
    lanes = workflow.get("lanes", {})
    if not isinstance(lanes, dict):
        return "off"
    lane = lanes.get(lane_id)
    if not isinstance(lane, dict):
        return "off"
    mode = lane.get("worktree_isolation")
    if isinstance(mode, str) and mode in {"off", "per_job"}:
        return mode
    return "off"


def adapter_constraint_enforcement(adapter: object, *, constraint: str, requested: str) -> str:
    """Return the enforcement level an adapter can provide for a requested constraint."""
    if adapter == "process":
        if constraint == "transcripts" and requested == "off":
            return "enforced"
        if constraint == "network" and requested == "forbidden":
            return "advisory_strict"
        if constraint == "repo_scope" and requested == "local_only":
            return "advisory_strict"
        return "advisory"
    return "unsupported"


def adapter_enforcement_satisfies(*, actual: str, required: str) -> bool:
    """Return whether an actual enforcement level satisfies a workflow requirement."""
    return ADAPTER_ENFORCEMENT_LEVELS[actual] >= ADAPTER_ENFORCEMENT_LEVELS[required]


def _repo_relative_path(repo: Path, path_text: str, *, allow_state: bool) -> Path:
    """Resolve a repo-relative path with optional state-dir allowance."""
    path = Path(path_text)
    if path.is_absolute():
        raise ArtifactError("artifact path must be repo-relative")
    resolved = (repo / path).resolve()
    repo_resolved = repo.resolve()
    try:
        resolved.relative_to(repo_resolved)
    except ValueError as exc:
        raise ArtifactError("artifact path must stay inside the repository") from exc
    if not allow_state and (
        resolved == repo_resolved / STATE_DIR or (repo_resolved / STATE_DIR) in resolved.parents
    ):
        raise ArtifactError("artifact path cannot be under .striatum")
    return resolved
