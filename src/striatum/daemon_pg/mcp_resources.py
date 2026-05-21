"""PostgreSQL-backed daemon MCP resource projections."""

from __future__ import annotations

import uuid
from collections.abc import Mapping
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, urlparse

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.reads._read_model import status_payload
from striatum.daemon_pg.handlers.reads._sql import row_to_json
from striatum.daemon_pg.handlers.reads.doctor import doctor_payload
from striatum.daemon_pg.handlers.reads.why import handle as why_handle
from striatum.daemon_pg.handlers.recovery_evidence._sql import is_repo_write_scope
from striatum.daemon_rpc.capability import RpcAuthContext, authorize
from striatum.daemon_rpc.request_log import append_audit_row
from striatum.repo_policy import DB_NAME, state_dir
from striatum.primitives import JsonObject

READ_CAPABILITY = "read"
PROTOCOL_VERSION = 1


def daemon_mcp_resources_pg(
    pg_conn: Any,
    *,
    token: str | None,
    request_id: str | None = None,
) -> list[dict[str, str]]:
    """Return daemon MCP resources using PostgreSQL repository metadata."""

    allowed_ids, auth = readable_repository_ids_pg(pg_conn, token=token)
    _audit(pg_conn, auth=auth, method="mcp.resources.list", request_id=request_id, params={})
    _require_allowed(auth)
    resources = [
        {"uri": "striatum://daemon/repos", "name": "daemon repositories", "mimeType": "application/json"},
        {"uri": "striatum://daemon/dashboard", "name": "daemon dashboard", "mimeType": "application/json"},
    ]
    for repository_id in sorted(allowed_ids):
        resources.extend([
            {"uri": f"striatum://repo/{repository_id}/status", "name": f"repository {repository_id} status", "mimeType": "application/json"},
            {"uri": f"striatum://repo/{repository_id}/doctor", "name": f"repository {repository_id} doctor", "mimeType": "application/json"},
            {"uri": f"striatum://repo/{repository_id}/runs", "name": f"repository {repository_id} runs", "mimeType": "application/json"},
            {"uri": f"striatum://repo/{repository_id}/blockers", "name": f"repository {repository_id} blockers", "mimeType": "application/json"},
            {"uri": f"striatum://repo/{repository_id}/stale-leases", "name": f"repository {repository_id} stale leases", "mimeType": "application/json"},
        ])
    return resources


def daemon_mcp_read_resource_pg(
    pg_conn: Any,
    uri: str,
    *,
    token: str | None,
    request_id: str | None = None,
) -> JsonObject:
    """Read one daemon MCP resource from PostgreSQL-backed state."""

    parsed = urlparse(uri)
    if parsed.scheme != "striatum":
        raise ValueError("resource URI must use the striatum scheme")
    host = parsed.netloc
    parts = [part for part in parsed.path.split("/") if part]
    if host == "daemon" and parts == ["repos"]:
        return repo_list_resource_pg(pg_conn, token=token, request_id=request_id)
    if host == "daemon" and parts == ["dashboard"]:
        return dashboard_all_resource_pg(pg_conn, token=token, request_id=request_id)
    if host != "repo" or not parts:
        raise ValueError(f"unsupported daemon resource URI {uri!r}")

    repository_id = parts[0]
    ctx = _repo_context(pg_conn, repository_id, token=token, request_id=request_id, uri=uri)
    if len(parts) == 2 and parts[1] == "status":
        return _with_resource_envelope(ctx, status_payload(ctx, run_id=None))
    if len(parts) == 2 and parts[1] == "doctor":
        return _with_resource_envelope(ctx, doctor_payload(ctx, run_id=None, verbose=True))
    if len(parts) == 2 and parts[1] == "runs":
        return _with_resource_envelope(ctx, status_payload(ctx, run_id=None))
    if len(parts) == 2 and parts[1] == "blockers":
        return {
            "mode": "daemon",
            "repository_id": repository_id,
            "protocol_version": PROTOCOL_VERSION,
            "blockers": status_payload(ctx, run_id=None).get("open_blockers", []),
        }
    if len(parts) == 2 and parts[1] == "stale-leases":
        return {
            "mode": "daemon",
            "repository_id": repository_id,
            "protocol_version": PROTOCOL_VERSION,
            "stale_leases": stale_leases_for_active_runs(ctx),
        }
    if len(parts) == 3 and parts[1] == "run":
        return _with_resource_envelope(ctx, status_payload(ctx, run_id=parts[2]))
    if len(parts) == 4 and parts[1] == "run" and parts[3] == "why":
        query = parse_qs(parsed.query)
        ids = query.get("id", [])
        if not ids:
            raise ValueError("why resource requires ?id=<entity>")
        return _with_resource_envelope(ctx, why_handle(ctx, {"target_id": ids[0]}))
    raise ValueError(f"unsupported daemon resource URI {uri!r}")


def repo_list_resource_pg(
    pg_conn: Any,
    *,
    token: str | None,
    request_id: str | None = None,
) -> JsonObject:
    allowed_ids, auth = readable_repository_ids_pg(pg_conn, token=token)
    _audit(pg_conn, auth=auth, method="mcp.repo.list", request_id=request_id, params={})
    _require_allowed(auth)
    rows = _repository_rows(pg_conn, active_only=False)
    return {
        "repositories": [
            row for row in rows
            if row["state"] == "active" and str(row["repository_id"]) in allowed_ids
        ]
    }


def dashboard_all_resource_pg(
    pg_conn: Any,
    *,
    token: str | None,
    request_id: str | None = None,
) -> JsonObject:
    allowed_ids, auth = readable_repository_ids_pg(pg_conn, token=token)
    _audit(pg_conn, auth=auth, method="dashboard.all", request_id=request_id, params={})
    _require_allowed(auth)
    return dashboard_all_payload_pg(pg_conn, repository_ids=allowed_ids)


def dashboard_all_payload_pg(
    pg_conn: Any,
    *,
    repository_ids: set[str] | None = None,
) -> JsonObject:
    """Return the daemon-global dashboard projection without auth side effects."""

    repos = _repository_rows(pg_conn, active_only=False)
    result: list[dict[str, Any]] = []
    for repo in repos:
        if repo["state"] == "removed":
            continue
        repository_id = str(repo["repository_id"])
        if repository_ids is not None and repository_id not in repository_ids:
            continue
        entry: dict[str, Any] = {
            "repository_id": repository_id,
            "display_name": repo["display_name"],
            "repo_root": repo["repo_root"],
            "state": repo["state"],
        }
        auth = RpcAuthContext(None, None, repository_id, READ_CAPABILITY, "allowed")
        ctx = RepoHandlerContext(
            pg_conn=pg_conn,
            repository_id=repository_id,
            repo_root=Path(str(repo["repo_root"])),
            auth=auth,
        )
        try:
            entry["status"] = status_payload(ctx, run_id=None)
            entry["stale_leases"] = stale_leases_for_active_runs(ctx)
            entry["run_progress"] = run_progress_for_active_runs(ctx)
        except Exception as exc:  # noqa: BLE001 - keep one bad repo from hiding others.
            entry["state"] = "degraded"
            entry["error"] = str(exc)
        result.append(entry)
    return {"mode": "daemon", "protocol_version": PROTOCOL_VERSION, "repositories": result}


def readable_repository_ids_pg(pg_conn: Any, *, token: str | None) -> tuple[set[str], RpcAuthContext]:
    global_auth = authorize(
        pg_conn,
        required=READ_CAPABILITY,
        repository_id=None,
        token=token,
    )
    if global_auth.decision == "allowed":
        return {
            str(row["repository_id"])
            for row in _repository_rows(pg_conn, active_only=True)
        }, global_auth
    if global_auth.denial_reason and global_auth.denial_reason.startswith("token_"):
        return set(), global_auth
    if global_auth.client_id is None:
        return set(), global_auth
    with _dict_cursor(pg_conn) as cur:
        cur.execute(
            """
            SELECT c.repository_id
            FROM striatumd.client_capabilities c
            JOIN striatumd.repositories r
              ON r.repository_id = c.repository_id
            WHERE c.client_id = %s
              AND c.capability = %s
              AND c.repository_id IS NOT NULL
              AND c.revoked_at IS NULL
              AND (c.expires_at IS NULL OR c.expires_at > now())
              AND r.state = 'active'
            ORDER BY c.repository_id
            """,
            (global_auth.client_id, READ_CAPABILITY),
        )
        rows = cur.fetchall()
    allowed = {str(row["repository_id"]) for row in rows}
    if not allowed:
        return set(), global_auth
    with pg_conn.cursor() as cur:
        cur.execute(
            "UPDATE striatumd.clients SET last_used_at = now() WHERE client_id = %s",
            (global_auth.client_id,),
        )
    return (
        allowed,
        RpcAuthContext(
            global_auth.client_id,
            global_auth.token_id,
            None,
            READ_CAPABILITY,
            "allowed",
        ),
    )


def stale_leases_for_active_runs(ctx: RepoHandlerContext) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT run_id
            FROM striatumd.runs
            WHERE repository_id = %s
              AND (state IN ('running', 'blocked') OR paused_at IS NOT NULL)
            ORDER BY created_at, run_id
            """,
            (ctx.repository_id,),
        )
        run_ids = [str(row["run_id"]) for row in cur.fetchall()]
    return [_stale_leases_for_run(ctx, run_id=run_id) for run_id in run_ids]


def run_progress_for_active_runs(ctx: RepoHandlerContext) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT run_id, state
            FROM striatumd.runs
            WHERE repository_id = %s
              AND (
                state IN ('needs_branch_confirmation', 'ready', 'running', 'blocked')
                OR paused_at IS NOT NULL
              )
            ORDER BY created_at, run_id
            """,
            (ctx.repository_id,),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    progress: list[dict[str, Any]] = []
    for row in rows:
        run_id = str(row["run_id"])
        status = status_payload(ctx, run_id=run_id)
        progress.append(
            {
                "run_id": run_id,
                "state": row["state"],
                "current_phase_id": status.get("current_phase_id"),
                "phases": status.get("phases") or [],
                "auto_finalize_dry_run": status.get("auto_finalize_dry_run"),
                "supervisor_stalls": status.get("supervisor_stalls"),
            }
        )
    return progress


def _stale_leases_for_run(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.job_id, j.workflow_job_id, j.state AS job_state,
                   j.write_scope_json,
                   l.lease_id, l.owner_session_id, l.acquired_at,
                   l.expires_at, l.released_at, l.release_reason,
                   qm.message_id, qm.state AS message_state
            FROM striatumd.jobs j
            LEFT JOIN striatumd.leases l
              ON l.repository_id = j.repository_id
             AND (l.lease_id = j.current_lease_id
                  OR (l.resource_id = j.job_id AND l.state = 'expired'))
            LEFT JOIN striatumd.queue_messages qm
              ON qm.repository_id = j.repository_id
             AND qm.message_id = j.current_message_id
            WHERE j.repository_id = %s
              AND j.run_id = %s
              AND (j.state = 'stale_lease' OR l.state = 'expired')
            ORDER BY j.workflow_job_id, l.expires_at
            """,
            (ctx.repository_id, run_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    entries: list[dict[str, Any]] = []
    seen: set[tuple[str, str | None]] = set()
    for row in rows:
        key = (str(row["job_id"]), str(row["lease_id"]) if row.get("lease_id") is not None else None)
        if key in seen:
            continue
        seen.add(key)
        repo_write = is_repo_write_scope(row.get("write_scope_json"))
        entries.append(
            {
                "job_id": row["job_id"],
                "workflow_job_id": row["workflow_job_id"],
                "job_state": row["job_state"],
                "lease_id": row.get("lease_id"),
                "owner_session_id": row.get("owner_session_id"),
                "expires_at": row.get("expires_at"),
                "released_at": row.get("released_at"),
                "release_reason": row.get("release_reason"),
                "message_id": row.get("message_id"),
                "message_state": row.get("message_state"),
                "repo_write": repo_write,
                "recovery_policy": (
                    "manual_inspection_required"
                    if repo_write
                    else "safe_to_reclaim_when_pending"
                ),
                "next_actions": (
                    ["inspect_worktree_and_artifacts", "decide_requeue_or_cancel"]
                    if repo_write
                    else ["register_or_select_session", "claim_available_work"]
                ),
            }
        )
    return {
        "run_id": run_id,
        "stale_count": len(entries),
        "stale_leases": entries,
        "next_actions": (
            ["inspect_worktree_and_artifacts", "decide_requeue_or_cancel"]
            if entries
            else []
        ),
    }


def _repo_context(
    pg_conn: Any,
    repository_id: str,
    *,
    token: str | None,
    request_id: str | None,
    uri: str,
) -> RepoHandlerContext:
    auth = authorize(
        pg_conn,
        required=READ_CAPABILITY,
        repository_id=repository_id,
        token=token,
    )
    _audit(
        pg_conn,
        auth=auth,
        method="mcp.resources.read",
        request_id=request_id,
        params={"uri": uri, "repository_id": repository_id},
    )
    _require_allowed(auth)
    with _dict_cursor(pg_conn) as cur:
        cur.execute(
            """
            SELECT repository_id, repo_root
            FROM striatumd.repositories
            WHERE repository_id = %s AND state = 'active'
            """,
            (repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        raise ValueError(f"unknown daemon repository {repository_id!r}")
    return RepoHandlerContext(
        pg_conn=pg_conn,
        repository_id=repository_id,
        repo_root=Path(str(row["repo_root"])),
        auth=auth,
    )


def _with_resource_envelope(ctx: RepoHandlerContext, payload: Mapping[str, Any]) -> JsonObject:
    return {
        "mode": "daemon",
        "repository_id": ctx.repository_id,
        "protocol_version": PROTOCOL_VERSION,
        **dict(payload),
    }


def _repository_rows(pg_conn: Any, *, active_only: bool) -> list[dict[str, Any]]:
    where = "WHERE state = 'active'" if active_only else ""
    with _dict_cursor(pg_conn) as cur:
        cur.execute(
            f"""
            SELECT repository_id, repo_identity, repo_root, display_name,
                   registered_at, removed_at, last_seen_at,
                   last_schema_version, state, state_db_path
            FROM striatumd.repositories
            {where}
            ORDER BY repository_id
            """
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    for row in rows:
        repo_root = row.get("repo_root")
        if repo_root is None:
            continue
        scratch_path = state_dir(Path(str(repo_root)))
        row["state_dir"] = str(scratch_path)
        stored_state_path = Path(str(row.get("state_db_path", "")))
        if stored_state_path.name == DB_NAME and stored_state_path.parent.name == scratch_path.name:
            row["state_db_path"] = str(scratch_path)
    return rows


def _audit(
    pg_conn: Any,
    *,
    auth: RpcAuthContext,
    method: str,
    request_id: str | None,
    params: Mapping[str, Any],
) -> None:
    append_audit_row(
        pg_conn,
        auth=auth,
        method=method,
        transport="mcp",
        request_id=request_id or f"mcp_resource_{uuid.uuid4().hex}",
        params=params,
        exit_code=None if auth.decision == "allowed" else 10,
    )


def _require_allowed(auth: RpcAuthContext) -> None:
    if auth.decision != "allowed":
        raise PermissionError(auth.denial_reason or "capability_missing")


def _dict_cursor(conn: Any) -> Any:
    try:
        from psycopg.rows import dict_row
    except ImportError:
        return conn.cursor()
    return conn.cursor(row_factory=dict_row)
