"""Small helpers for repository-scoped read handlers."""

from __future__ import annotations

import json
from collections.abc import Mapping
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError
from striatum.primitives import sha256_bytes
from striatum.repo_policy import repo_relative_path


RUN_STATES = {
    "needs_branch_confirmation",
    "ready",
    "running",
    "blocked",
    "completed",
    "failed",
    "canceled",
}
SESSION_STATES = {"active", "expired", "stopped", "lost", "closed"}
JOB_STATES = {
    "blocked",
    "queued",
    "claimed",
    "running",
    "stale_lease",
    "waiting_human",
    "completed",
    "failed",
    "canceled",
    "skipped",
}
ARTIFACT_KINDS = {
    "prompt",
    "finding",
    "findings_ledger",
    "synthesis",
    "marker",
    "handoff",
    "decision",
    "patch_summary",
    "test_report",
    "other",
}


def require_text(params: Mapping[str, Any], key: str) -> str:
    value = params.get(key)
    if not isinstance(value, str) or not value:
        raise RpcError("schema_invalid", f"{key} must be a non-empty string")
    return value


def optional_text(params: Mapping[str, Any], key: str) -> str | None:
    value = params.get(key)
    if value is None:
        return None
    if not isinstance(value, str) or not value:
        raise RpcError("schema_invalid", f"{key} must be a non-empty string when present")
    return value


def positive_limit(params: Mapping[str, Any], *, default: int = 100) -> int:
    value = params.get("limit", default)
    if not isinstance(value, int) or value < 1:
        raise RpcError("schema_invalid", "limit must be a positive integer")
    return value


def validate_choice(value: str | None, *, allowed: set[str], label: str) -> None:
    if value is None:
        return
    if value not in allowed:
        options = ", ".join(sorted(allowed))
        raise RpcError("schema_invalid", f"unknown {label}: {value!r} (valid options: {options})")


def run_exists(ctx: RepoHandlerContext, run_id: str) -> bool:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT 1
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        return cur.fetchone() is not None


def require_run(ctx: RepoHandlerContext, run_id: str) -> None:
    if not run_exists(ctx, run_id):
        raise RpcError("not_found", f"run not found: {run_id}")


def row_to_json(row: Mapping[str, Any]) -> dict[str, Any]:
    return {str(key): json_value(value) for key, value in row.items()}


def json_value(value: Any) -> Any:
    if isinstance(value, datetime):
        dt = value if value.tzinfo is not None else value.replace(tzinfo=UTC)
        return dt.astimezone(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    if isinstance(value, Path):
        return str(value)
    if isinstance(value, (dict, list, str, int, float, bool)) or value is None:
        return value
    try:
        return json.loads(str(value))
    except json.JSONDecodeError:
        return str(value)


def parse_json_object(value: Any) -> dict[str, Any]:
    if isinstance(value, dict):
        return dict(value)
    if value is None:
        return {}
    loaded = json.loads(str(value))
    return dict(loaded) if isinstance(loaded, dict) else {}


def parse_json_list(value: Any) -> list[Any]:
    if isinstance(value, list):
        return list(value)
    if value is None:
        return []
    loaded = json.loads(str(value))
    return list(loaded) if isinstance(loaded, list) else []


def safe_output_path(ctx: RepoHandlerContext, path_text: str) -> Path:
    try:
        target = repo_relative_path(ctx.repo_root, path_text)
    except Exception as exc:  # pragma: no cover - repo_relative_path owns details
        raise RpcError("path_outside_scope", str(exc), exit_code=8) from exc
    try:
        rel = target.resolve().relative_to(ctx.repo_root.resolve())
    except ValueError as exc:
        raise RpcError("path_outside_scope", "path must stay inside the repository", exit_code=8) from exc
    if rel.parts and rel.parts[0] == ".striatum":
        raise RpcError("path_outside_scope", "path must not be under .striatum", exit_code=8)
    return target


def write_text_export(ctx: RepoHandlerContext, *, path_text: str, body: str) -> dict[str, Any]:
    target = safe_output_path(ctx, path_text)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(body, encoding="utf-8")
    return {"path": path_text, "sha256": sha256_bytes(body.encode("utf-8"))}
