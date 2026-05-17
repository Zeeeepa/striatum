"""PG-backed operator decision handlers."""

from __future__ import annotations

import json
from collections.abc import Mapping
from pathlib import Path
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import ArtifactError
from striatum.primitives import sha256_bytes
from striatum.repo_policy import repo_relative_path


@register_pg_handler("decision.record")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id", method="decision.record")
    path_text = _required_text(params, "path", method="decision.record")
    outcome = _required_text(params, "outcome", method="decision.record")
    title = _required_text(params, "title", method="decision.record").strip()
    if title == "":
        raise RpcError(
            "schema_invalid",
            "decision.record requires run_id, path, outcome, and title",
        )
    rationale = _optional_text(params, "rationale")
    follow_up = _optional_text(params, "follow_up")
    if outcome == "accepted_with_follow_up" and (follow_up is None or follow_up.strip() == ""):
        raise ArtifactError("accepted_with_follow_up decisions require --follow-up")
    raw_decision_id = params.get("decision_id")
    decision_id = str(raw_decision_id).strip() if raw_decision_id is not None else ctx.new_id("dec")
    if decision_id == "" or any(character.isspace() for character in decision_id):
        raise ArtifactError("decision id cannot be empty or contain whitespace")

    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id)
        _refuse_duplicate_decision(
            ctx,
            run_id=run_id,
            decision_id=decision_id,
            path_text=path_text,
        )
        repo_root = _repo_root_for(ctx, run)
        target = repo_relative_path(repo_root, path_text)
        if target.exists():
            raise ArtifactError("decision artifact path already exists")
        target.parent.mkdir(parents=True, exist_ok=True)

        created_at = ctx.now()
        body = render_decision_markdown(
            decision_id=decision_id,
            run_id=run_id,
            outcome=outcome,
            title=title,
            created_at=created_at,
            rationale=rationale,
            follow_up=follow_up,
        )
        target.write_text(body, encoding="utf-8")
        payload = body.encode("utf-8")
        digest = sha256_bytes(payload)
        artifact_id = ctx.new_id("art")
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.artifacts (
                  repository_id, artifact_id, run_id, job_id, session_id,
                  logical_name, artifact_kind, repo_path, content_sha256,
                  size_bytes, publish_mode, created_at
                )
                VALUES (%s, %s, %s, NULL, NULL, %s, 'decision', %s, %s, %s, 'create', %s)
                """,
                (
                    ctx.repository_id,
                    artifact_id,
                    run_id,
                    decision_id,
                    path_text,
                    digest,
                    len(payload),
                    created_at,
                ),
            )
        ctx.append_event(
            run_id=run_id,
            event_type="decision.recorded",
            artifact_id=artifact_id,
            payload={
                "decision_id": decision_id,
                "outcome": outcome,
                "path": path_text,
                "sha256": digest,
            },
        )
        return {
            "status": "recorded",
            "run_id": run_id,
            "decision_id": decision_id,
            "artifact_id": artifact_id,
            "path": path_text,
            "outcome": outcome,
            "sha256": digest,
            "run_state_transition": None,
            "superseded_verdict_count": 0,
        }


def render_decision_markdown(
    *,
    decision_id: str,
    run_id: str,
    outcome: str,
    title: str,
    created_at: str,
    rationale: str | None,
    follow_up: str | None,
) -> str:
    """Render the durable operator decision Markdown artifact."""

    follow_up_required = outcome == "accepted_with_follow_up"
    lines = [
        "---",
        "schema_version: striatum.decision.v1",
        f"decision_id: {json.dumps(decision_id)}",
        f"run_id: {json.dumps(run_id)}",
        "artifact_kind: decision",
        "owner: human",
        f"outcome: {outcome}",
        f"follow_up_required: {str(follow_up_required).lower()}",
        f"title: {json.dumps(title)}",
        f"created_at: {json.dumps(created_at)}",
        "---",
        "",
        f"# {title}",
        "",
        f"Decision ID: `{decision_id}`",
        f"Run ID: `{run_id}`",
        f"Outcome: `{outcome}`",
        "",
    ]
    if rationale is not None and rationale.strip() != "":
        lines.extend(["## Rationale", "", rationale.strip(), ""])
    if follow_up is not None and follow_up.strip() != "":
        lines.extend(["## Follow-Up", "", follow_up.strip(), ""])
    return "\n".join(lines)


def _refuse_duplicate_decision(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    decision_id: str,
    path_text: str,
) -> None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT artifact_id
            FROM striatumd.artifacts
            WHERE repository_id = %s
              AND run_id = %s
              AND artifact_kind = 'decision'
              AND (logical_name = %s OR repo_path = %s)
            LIMIT 1
            """,
            (ctx.repository_id, run_id, decision_id, path_text),
        )
        existing = cur.fetchone()
    if existing is not None:
        raise ArtifactError("decision artifact already exists for this run id/path")


def _repo_root_for(ctx: RepoHandlerContext, run: Mapping[str, Any]) -> Path:
    raw = run.get("repo_root")
    if raw is None:
        return ctx.repo_root
    return Path(str(raw)).expanduser().resolve()


def _required_text(params: Mapping[str, Any], key: str, *, method: str) -> str:
    value = str(params.get(key) or "")
    if value == "":
        raise RpcError("schema_invalid", f"{method} requires run_id, path, outcome, and title")
    return value


def _optional_text(params: Mapping[str, Any], key: str) -> str | None:
    value = params.get(key)
    if value is None:
        return None
    return str(value)


__all__ = ["handle", "render_decision_markdown"]
