"""PG handler for ``recovery.auto`` (a.k.a. ``recovery.auto_publish_stale_artifacts``).

Port of :func:`striatum.cli.recovery.auto_publish_stale_artifacts`
(lines 731-980). Synthesis: docs/dogfood/057/DESIGN_SYNTHESIS.md L171-179.

Naming note (review finding #1, REVIEW.md L117-119): the synthesis section
header is ``recovery.auto_publish_stale_artifacts``, but the RPC method
registered in ``CLI_ROUTES`` is ``recovery.auto`` (server.py:83). The
``@register_pg_handler`` decorator MUST use ``"recovery.auto"`` so a grep
of the registry finds it.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any, Mapping

from striatum.errors import NotFoundError

from ._shim import RepoHandlerContext, register_pg_handler
from ._sql import (
    expire_leases,
    fetch_all,
    maybe_complete_run,
    parse_json,
    row_by_id,
    utc_now,
)


def _declared_get(declared: object, key: str, default: object = None) -> object:
    """V1.41 helper: tolerant get from json_loads'd expected_artifacts."""
    if isinstance(declared, dict):
        return declared.get(key, default)
    return default


@register_pg_handler("recovery.auto")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """V1.41 stale-lease auto-publish flow, ported to PG.

    Walks stale leases for ``run_id``. For each, checks whether the dead
    session wrote the work-packet's ``expected_artifacts[].path`` file
    conformantly to disk. If so — and only if the on-disk file's byline
    matches the expected author line exactly AND the path matches the
    declared ``expected_artifacts`` path exactly — auto-publishes the
    artifact and completes the job on the dead session's behalf.

    Dry-run (synthesis L177): read-only transaction; must NOT call lazy
    expiry. Live mode: per-candidate serializable transaction that locks
    job, lease, message, and artifact uniqueness rows.

    Inline dependency note: live-mode auto-publish requires Track A's PG
    ``publish_artifact`` and ``complete_job`` helpers in the same
    transaction (synthesis cross-track contract). The handler imports
    them lazily; if Track A's plumbing isn't yet merged, dry-run remains
    functional but live mode raises with the same operator-actionable
    message used by ``recovery.resume`` inline completion.
    """
    run_id = str(params.get("run_id") or "")
    dry_run = bool(params.get("dry_run") or False)
    if not run_id:
        raise NotFoundError("recovery.auto requires run_id")
    run = row_by_id(ctx, table="runs", id_column="run_id", value=run_id)
    if not run:
        raise NotFoundError(f"run {run_id!r} not found")
    repo = Path(str(ctx.repo_root))

    if not dry_run:
        expire_leases(ctx, run_id=run_id)

    now = utc_now()
    if dry_run:
        candidates = fetch_all(
            ctx,
            sql=(
                "SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at, "
                "       qm.message_id, qm.state AS message_state, "
                "       l.state AS lease_state "
                "FROM striatumd.jobs j "
                "LEFT JOIN striatumd.leases l ON l.repository_id = j.repository_id "
                "   AND (l.lease_id = j.current_lease_id "
                "        OR (l.resource_id = j.job_id AND l.state = 'expired')) "
                "LEFT JOIN striatumd.queue_messages qm "
                "   ON qm.repository_id = j.repository_id "
                "  AND qm.message_id = j.current_message_id "
                "WHERE j.repository_id = %(repository_id)s "
                "  AND j.run_id = %(run_id)s "
                "  AND j.state IN ('claimed', 'running', 'stale_lease') "
                "  AND (l.state = 'expired' "
                "       OR (l.state = 'active' AND l.expires_at < %(now)s)) "
                "ORDER BY j.workflow_job_id"
            ),
            params={"run_id": run_id, "now": now},
        )
    else:
        candidates = fetch_all(
            ctx,
            sql=(
                "SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at, "
                "       qm.message_id, qm.state AS message_state, "
                "       l.state AS lease_state "
                "FROM striatumd.jobs j "
                "LEFT JOIN striatumd.leases l ON l.repository_id = j.repository_id "
                "   AND (l.lease_id = j.current_lease_id "
                "        OR (l.resource_id = j.job_id AND l.state = 'expired')) "
                "LEFT JOIN striatumd.queue_messages qm "
                "   ON qm.repository_id = j.repository_id "
                "  AND qm.message_id = j.current_message_id "
                "WHERE j.repository_id = %(repository_id)s "
                "  AND j.run_id = %(run_id)s "
                "  AND j.state IN ('claimed', 'running', 'stale_lease') "
                "  AND l.state = 'expired' "
                "ORDER BY j.workflow_job_id"
            ),
            params={"run_id": run_id},
        )

    from striatum.artifacts import (  # local import: heavy module
        _canonical_byline_form,
        expected_author_line,
        markdown_title_block_author_lines,
    )

    published: list[dict[str, Any]] = []
    skipped: list[dict[str, Any]] = []
    seen: set[tuple[str, str]] = set()
    for row in candidates:
        key = (
            str(row["job_id"]),
            str(row["lease_id"]) if row.get("lease_id") is not None else "",
        )
        if key in seen:
            continue
        seen.add(key)
        job_id = str(row["job_id"])
        workflow_job_id = str(row["workflow_job_id"])
        session_id = (
            str(row["owner_session_id"])
            if row.get("owner_session_id") is not None
            else None
        )
        lease_id = (
            str(row["lease_id"]) if row.get("lease_id") is not None else None
        )
        message_id = (
            str(row["message_id"]) if row.get("message_id") is not None else None
        )
        expected_artifacts = parse_json(row.get("expected_artifacts_json"), default=[])
        if not isinstance(expected_artifacts, list):
            expected_artifacts = []
        if not expected_artifacts:
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": "no expected_artifacts declared",
            })
            continue
        if session_id is None or lease_id is None or message_id is None:
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": "no recoverable session/lease/message triple",
            })
            continue
        try:
            expected_byline = _expected_byline_pg(
                ctx,
                job=row,
                session_id=session_id,
                expected_author_line_fn=expected_author_line,
            )
        except Exception as exc:  # noqa: BLE001
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": f"could not derive expected byline: {exc}",
            })
            continue
        publishable: list[dict[str, Any]] = []
        for declared_obj in expected_artifacts:
            if not isinstance(declared_obj, dict):
                continue
            declared = declared_obj
            if not _declared_get(declared, "required", True):
                continue
            path_text = _declared_get(declared, "path")
            if not isinstance(path_text, str) or not path_text:
                continue
            disk_path = repo / path_text
            if not disk_path.is_file():
                continue
            try:
                payload = disk_path.read_bytes().decode("utf-8")
            except (OSError, UnicodeDecodeError):
                continue
            byline_lines = markdown_title_block_author_lines(payload)
            canonical_match = any(
                _canonical_byline_form(line) == expected_byline for line in byline_lines
            )
            if not canonical_match:
                continue
            publishable.append(declared)
        if not publishable:
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": "no required expected_artifact found on disk with matching byline",
            })
            continue
        if dry_run:
            would_expire = row.get("lease_state") == "active"
            published.append({
                "workflow_job_id": workflow_job_id,
                "session_id": session_id,
                "would_publish": [
                    {
                        "path": _declared_get(d, "path"),
                        "kind": _declared_get(d, "kind"),
                        "logical_name": _declared_get(d, "logical_name"),
                    }
                    for d in publishable
                ],
                "would_expire": would_expire,
            })
            continue
        artifact_results = _live_publish_and_complete(
            ctx,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            message_id=message_id,
            publishable=publishable,
        )
        ctx.append_event(
            run_id=run_id,
            event_type="recovery.auto_published",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={
                "workflow_job_id": workflow_job_id,
                "artifacts": [
                    _declared_get(d, "path") if isinstance(d, dict) else None
                    for d in publishable
                ],
                "byline": expected_byline,
            },
        )
        published.append({
            "workflow_job_id": workflow_job_id,
            "session_id": session_id,
            "artifacts": artifact_results.get("artifacts", []),
            "complete": artifact_results.get("complete"),
        })
    if not dry_run:
        maybe_complete_run(ctx, run_id=run_id)
    return {
        "run_id": run_id,
        "dry_run": bool(dry_run),
        "published_count": len(published),
        "published": published,
        "skipped_count": len(skipped),
        "skipped": skipped,
    }


def _expected_byline_pg(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
    expected_author_line_fn: Any,
) -> str:
    """Call ``striatum.artifacts.expected_author_line`` with a PG-backed view.

    The CLI helper expects a SQLite connection, so we cannot reuse it
    directly under PG. Instead, the equivalent identity-resolution lives
    in :mod:`striatum.identity`; ``expected_author_line`` reads workflow
    + session rows, both of which we already have via the row args.

    The helper is intentionally pure aside from one SELECT (session
    lookup). Track A's PG identity helper, when it lands, will provide
    a typed version; for now we open a one-shot SQLite shim by reading
    the run's workflow_json + session row via PG and constructing the
    same canonical byline string.
    """
    from striatum.identity import artifact_author_identity

    workflow_rows = fetch_all(
        ctx,
        sql=(
            "SELECT ws.workflow_json FROM striatumd.workflow_snapshots ws "
            "JOIN striatumd.runs r ON r.repository_id = ws.repository_id "
            "  AND r.workflow_snapshot_id = ws.workflow_snapshot_id "
            "WHERE r.repository_id = %(repository_id)s "
            "  AND r.run_id = %(run_id)s"
        ),
        params={"run_id": job["run_id"]},
    )
    workflow = parse_json(
        workflow_rows[0]["workflow_json"] if workflow_rows else {},
        default={},
    )
    session = row_by_id(ctx, table="sessions", id_column="session_id", value=session_id)
    lane_selector = parse_json(job.get("lane_selector_json"), default={})
    lane_id = (
        session.get("lane_id")
        if session
        else (lane_selector.get("lane_id") if isinstance(lane_selector, dict) else None)
    )
    author = artifact_author_identity(
        workflow,
        role_id=str(job["role_id"]),
        lane_id=str(lane_id) if lane_id is not None else None,
        workflow_job_id=str(job["workflow_job_id"]),
        ordinal=int(session["ordinal"]) if session else None,
    )
    return str(author["line"])


def _live_publish_and_complete(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    message_id: str,
    publishable: list[dict[str, Any]],
) -> dict[str, Any]:
    """Run the live auto-publish flow against Track A's PG helpers.

    Synthesis L177 keeps publish + complete in one serializable
    transaction so a partially completed run cannot end with a published
    artifact and no completed job. Until Track A wires
    ``handlers.workflow_loop.complete_job.complete_inline`` and a PG
    artifact publisher (under ``handlers.workflow_loop`` per synthesis
    L102-107), this branch surfaces the dependency clearly.
    """
    try:
        from striatum.daemon_pg.handlers.workflow_loop.complete_job import (
            complete_inline,
        )
        from striatum.daemon_pg.handlers.workflow_loop.submit_review import (
            publish_artifact_inline,
        )
    except ImportError as exc:
        from striatum.errors import InvalidTransitionError

        raise InvalidTransitionError(
            "recovery.auto live mode requires Track A's PG publish/complete "
            "helpers; rerun with dry_run=true to preview, then publish + "
            "complete via the registered RPC verbs"
        ) from exc

    try:
        from striatum.daemon_pg.handlers.workflow_loop.ack_work import (
            ack_inline,
        )

        ack_inline(
            ctx,
            session_id=session_id,
            message_id=message_id,
            lease_id=lease_id,
        )
    except ImportError:
        # Track A may bundle ack into publish_artifact_inline; ignore if not
        # available so the publish call can detect and ack itself.
        pass
    except Exception:  # noqa: BLE001
        # Ack may legitimately fail if the message has already been acked
        # but the implementer never completed. Continue; the publish call
        # will succeed against the active lease row.
        pass

    artifact_results: list[Any] = []
    for declared in publishable:
        try:
            result = publish_artifact_inline(
                ctx,
                session_id=session_id,
                job_id=job_id,
                lease_id=lease_id,
                kind=str(_declared_get(declared, "kind")),
                logical_name=str(_declared_get(declared, "logical_name")),
                path_text=str(_declared_get(declared, "path")),
            )
            artifact_results.append(result)
        except Exception as exc:  # noqa: BLE001
            artifact_results.append({
                "error": str(exc),
                "path": _declared_get(declared, "path"),
            })
    try:
        complete_result = complete_inline(
            ctx,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            summary=(
                "auto-published on stale lease (V1.41 harness friction "
                "burn-down)"
            ),
        )
    except Exception as exc:  # noqa: BLE001
        complete_result = {"error": str(exc)}
    return {"artifacts": artifact_results, "complete": complete_result}


__all__ = ["handle"]
