"""PG handler for ``artifact.show``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._registry import register_pg_handler
from ._sql import require_text, row_to_json


@register_pg_handler("artifact.show", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    artifact_id = require_text(params, "artifact_id")
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT artifact_id, run_id, job_id, session_id, logical_name,
                   artifact_kind, repo_path, content_sha256, size_bytes,
                   publish_mode, created_at, author_line
            FROM striatumd.artifacts
            WHERE repository_id = %s AND artifact_id = %s
            """,
            (ctx.repository_id, artifact_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"artifact not found: {artifact_id}")
    return {"artifact": row_to_json(row)}
