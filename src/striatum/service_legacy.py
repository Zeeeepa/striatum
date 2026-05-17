"""Gated legacy service fallbacks for subprocess web fixtures.

Production service paths are daemon-first. These helpers exist only so old
subprocess web fixtures can keep exercising pre-cutover SQLite semantics under
the explicit test-harness environment gate.
"""

from __future__ import annotations

import os
import subprocess
from http.server import BaseHTTPRequestHandler
from pathlib import Path
from typing import Any, Mapping


JsonObject = dict[str, Any]


def legacy_fixture_fallback_enabled(code: str) -> bool:
    """Return whether service tests may exercise the pre-cutover SQLite path."""
    return code == "repo_not_registered" and os.environ.get("STRIATUM_TEST_HARNESS") == "1"


def legacy_artifact_raw_fallback_enabled(code: str) -> bool:
    return (
        code in {"daemon_unreachable", "repo_not_registered"}
        and os.environ.get("STRIATUM_TEST_HARNESS") == "1"
        and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
    )


def legacy_web_read_fallback_enabled(code: str) -> bool:
    return (
        code in {"daemon_unreachable", "repo_not_registered"}
        and os.environ.get("STRIATUM_TEST_HARNESS") == "1"
        and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
    )


def legacy_run_cancel(repo: Path, *, run_id: str, reason: str | None) -> JsonObject:
    from striatum.db import cancel_run, connect, transaction

    with connect(repo) as conn, transaction(conn):
        return cancel_run(conn, run_id=run_id, reason=reason)


def legacy_run_pause(repo: Path, *, run_id: str, reason: str | None) -> JsonObject:
    from striatum.db import connect, pause_run, transaction

    with connect(repo) as conn, transaction(conn):
        return pause_run(conn, run_id=run_id, reason=reason)


def legacy_run_resume(repo: Path, *, run_id: str) -> JsonObject:
    from striatum.db import connect, resume_run, transaction

    with connect(repo) as conn, transaction(conn):
        return resume_run(conn, run_id=run_id)


def legacy_workflow_run_now(repo: Path, *, workflow_path: Path) -> JsonObject:
    from striatum.cli.mutations import branch_confirm, run_start
    from striatum.db import connect, transaction
    from striatum.workflow import create_run

    with connect(repo) as conn:
        with transaction(conn):
            prepared = create_run(conn, repo=repo, workflow_path=workflow_path)
        run_id = str(prepared["run_id"])
        requires_confirm = (
            prepared.get("state") == "needs_branch_confirmation"
            and prepared.get("branch_mode") != "auto"
        )
        if prepared.get("branch_mode") == "auto":
            suggested = prepared.get("suggested_branch_name")
            if isinstance(suggested, str) and suggested:
                branch_confirm(
                    conn,
                    repo=repo,
                    run_id=run_id,
                    branch=suggested,
                    create=True,
                )
        if requires_confirm:
            return {
                "run_id": run_id,
                "status": "needs_branch_confirmation",
                "suggested_branch_name": prepared.get("suggested_branch_name"),
            }
        run_start(conn, run_id=run_id)
    return {"run_id": run_id, "status": "running"}


def legacy_job_cancel(
    repo: Path,
    *,
    run_id: str,
    job_id: str,
    reason: str,
    cascade: bool,
) -> JsonObject:
    from striatum.cli.recovery import cancel_job
    from striatum.db import connect

    with connect(repo) as conn:
        return cancel_job(
            conn,
            run_id=run_id,
            job_id=job_id,
            reason=reason,
            cascade=cascade,
        )


def legacy_job_retry(repo: Path, *, run_id: str, job_id: str) -> JsonObject:
    from striatum.db import connect, retry_job, transaction

    with connect(repo) as conn, transaction(conn):
        return retry_job(conn, run_id=run_id, job_id=job_id)


def send_legacy_fixture_error(handler: BaseHTTPRequestHandler, exc: Exception) -> bool:
    from striatum.errors import InvalidTransitionError, NotFoundError, StriatumError

    if isinstance(exc, NotFoundError):
        _handler_send_json(handler, 404, {"ok": False, "error": {"code": 404, "message": str(exc)}})
        return True
    if isinstance(exc, InvalidTransitionError):
        _handler_send_json(
            handler,
            409,
            {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "invalid_transition"}},
        )
        return True
    if isinstance(exc, StriatumError):
        _handler_send_json(handler, 400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
        return True
    return False


def send_legacy_run_now_error(
    handler: BaseHTTPRequestHandler,
    repo: Path,
    exc: Exception,
) -> bool:
    from striatum.errors import (
        BranchConfirmationError,
        InvalidTransitionError,
        WorkflowError,
    )

    if isinstance(exc, BranchConfirmationError):
        _handler_send_json(
            handler,
            409,
            {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "branch_confirmation"}},
        )
        return True
    if isinstance(exc, InvalidTransitionError):
        _handler_send_json(
            handler,
            409,
            {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "invalid_transition"}},
        )
        return True
    if isinstance(exc, WorkflowError):
        msg = str(exc)
        if "git checkout failed" in msg:
            _handler_send_json(
                handler,
                409,
                {
                    "ok": False,
                    "error": {
                        "code": 409,
                        "message": msg,
                        "kind": "dirty_tree",
                        "git_status": short_git_status(repo),
                    },
                },
            )
            return True
        errors = []
        if exc.field_path:
            errors.append({"field_path": exc.field_path, "message": msg})
        _handler_send_json(
            handler,
            422,
            {"ok": False, "error": {"code": 422, "message": msg, "errors": errors}},
        )
        return True
    return send_legacy_fixture_error(handler, exc)


def short_git_status(repo: Path) -> str:
    try:
        proc = subprocess.run(
            ["git", "status", "--short"],
            cwd=repo,
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    lines = (proc.stdout or "").splitlines()
    if len(lines) > 80:
        lines = lines[:80] + [f"... ({len(lines) - 80} more lines)"]
    return "\n".join(lines)


def _handler_send_json(handler: BaseHTTPRequestHandler, status: int, body: Mapping[str, Any]) -> None:
    send_json = getattr(handler, "_send_json")
    send_json(status, dict(body))


__all__ = [
    "legacy_artifact_raw_fallback_enabled",
    "legacy_fixture_fallback_enabled",
    "legacy_job_cancel",
    "legacy_job_retry",
    "legacy_run_cancel",
    "legacy_run_pause",
    "legacy_run_resume",
    "legacy_web_read_fallback_enabled",
    "legacy_workflow_run_now",
    "send_legacy_fixture_error",
    "send_legacy_run_now_error",
    "short_git_status",
]
