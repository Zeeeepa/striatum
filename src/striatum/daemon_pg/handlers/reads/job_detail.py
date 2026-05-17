"""PG handler for ``job.detail``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, session_lane_attestation
from striatum.daemon_rpc.envelope import RpcError

from ._registry import register_pg_handler
from ._sql import parse_json_list, parse_json_object, require_text, row_to_json


@register_pg_handler("job.detail", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    job_ref = require_text(params, "job_id")
    run = _run(ctx, run_id=run_id)
    job = _job(ctx, run_id=run_id, job_ref=job_ref)
    job_id = str(job["job_id"])
    artifacts = _artifacts(ctx, job_id=job_id)
    packet = _latest_work_packet(ctx, job_id=job_id)
    expected_artifact_rows = _expected_artifact_rows(job=job, artifacts=artifacts, packet=packet)
    shaped_artifacts = _shape_artifacts(artifacts=artifacts, expected_rows=expected_artifact_rows)
    verdicts = _shape_verdicts(
        ctx,
        verdicts=_verdicts(ctx, job_id=job_id),
        override_verdict_ids=_override_verdict_ids(ctx, run_id=run_id),
    )
    latest_verdict = verdicts[0] if verdicts else None
    session_id = packet.get("session_id") if isinstance(packet, Mapping) else None
    job["work_packet"] = packet
    job["expected_artifact_rows"] = expected_artifact_rows
    job["lane_attestation_chip"] = _lane_attestation_chip(
        ctx,
        session_id=str(session_id) if isinstance(session_id, str) and session_id else None,
    )
    job["process_evidence"] = _process_evidence(ctx, run_id=run_id, job_id=job_id)
    job["state_chip"] = _state_chip("job", job.get("state"))
    run["state_chip"] = _state_chip("run", run.get("state"))
    return {
        "run": run,
        "job": job,
        "artifacts": shaped_artifacts,
        "latest_verdict": latest_verdict,
        "verdicts": verdicts,
        "expected_artifact_rows": expected_artifact_rows,
        "process_evidence": job["process_evidence"],
    }


def _run(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.*
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", "not found")
    return row_to_json(row)


def _job(ctx: RepoHandlerContext, *, run_id: str, job_ref: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.*
            FROM striatumd.jobs j
            WHERE j.repository_id = %s
              AND j.run_id = %s
              AND (j.job_id = %s OR j.workflow_job_id = %s)
            """,
            (ctx.repository_id, run_id, job_ref, job_ref),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", "not found")
    job = row_to_json(row)
    job["lane_id"] = parse_json_object(job.get("lane_selector_json")).get("lane_id")
    return job


def _artifacts(ctx: RepoHandlerContext, *, job_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT a.*
            FROM striatumd.artifacts a
            WHERE a.repository_id = %s AND a.job_id = %s
            ORDER BY a.created_at
            """,
            (ctx.repository_id, job_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def _latest_work_packet(ctx: RepoHandlerContext, *, job_id: str) -> dict[str, Any] | None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT wp.packet_json, wp.session_id, wp.lease_id, wp.message_id
            FROM striatumd.work_packets wp
            WHERE wp.repository_id = %s AND wp.job_id = %s
            ORDER BY wp.created_at DESC, wp.packet_id DESC
            LIMIT 1
            """,
            (ctx.repository_id, job_id),
        )
        row = cur.fetchone()
    if row is None:
        return None
    packet = parse_json_object(row.get("packet_json"))
    packet.setdefault("session_id", row["session_id"])
    packet.setdefault("lease_id", row["lease_id"])
    packet.setdefault("message_id", row["message_id"])
    return packet


def _expected_artifact_rows(
    *,
    job: Mapping[str, Any],
    artifacts: list[dict[str, Any]],
    packet: Mapping[str, Any] | None,
) -> list[dict[str, Any]]:
    declared = parse_json_list(job.get("expected_artifacts_json"))
    packet_expected = parse_json_list(packet.get("expected_artifacts")) if packet is not None else []
    packet_by_path = {
        str(item.get("path")): item
        for item in packet_expected
        if isinstance(item, Mapping) and item.get("path") is not None
    }
    artifacts_by_path = {str(item.get("repo_path")): item for item in artifacts}
    rows: list[dict[str, Any]] = []
    for item in declared:
        if not isinstance(item, Mapping):
            continue
        path = str(item.get("path") or "")
        packet_item = packet_by_path.get(path, {})
        expected_author_line = (
            packet_item.get("author_line") if isinstance(packet_item, Mapping) else None
        )
        actual = artifacts_by_path.get(path)
        actual_author_line = actual.get("author_line") if actual else None
        required = bool(item.get("required", True))
        if actual is not None:
            status = (
                "byline_drift"
                if expected_author_line and actual_author_line != expected_author_line
                else "published"
            )
        else:
            status = "missing_required" if required else "missing_optional"
        recipe = None
        if actual is None and path:
            recipe_parts = [
                "striatum",
                "publish-artifact",
                "--job-id",
                str(job.get("job_id")),
                "--path",
                path,
            ]
            if packet and packet.get("session_id"):
                recipe_parts.extend(["--session-id", str(packet["session_id"])])
            if packet and packet.get("lease_id"):
                recipe_parts.extend(["--lease-id", str(packet["lease_id"])])
            if item.get("kind"):
                recipe_parts.extend(["--kind", str(item["kind"])])
            if item.get("logical_name"):
                recipe_parts.extend(["--logical-name", str(item["logical_name"])])
            recipe = " ".join(recipe_parts)
        rows.append(
            {
                "logical_name": item.get("logical_name"),
                "kind": item.get("kind"),
                "path": path,
                "required": required,
                "expected_author_line": expected_author_line,
                "actual_artifact": actual,
                "actual_author_line": actual_author_line,
                "byline_line": _byline_line(
                    actual_author_line,
                    expected_author_line=expected_author_line,
                ),
                "status": status,
                "publish_recipe": recipe,
                "lane_evidence_chip": _lane_evidence_chip(
                    attestation_override_rationale=(
                        actual.get("attestation_override_rationale") if actual else None
                    ),
                ),
            }
        )
    return rows


def _shape_artifacts(
    *,
    artifacts: list[dict[str, Any]],
    expected_rows: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    expected_by_path = {str(row.get("path")): row for row in expected_rows}
    shaped: list[dict[str, Any]] = []
    for artifact in artifacts:
        item = dict(artifact)
        expected = expected_by_path.get(str(item.get("repo_path")), {})
        expected_author_line = expected.get("expected_author_line")
        item["expected_author_line"] = expected_author_line
        item["lane_attestation_chip"] = _recorded_artifact_attestation_chip(
            item.get("author_line"),
            expected_author_line=expected_author_line,
            attestation_override_rationale=item.get("attestation_override_rationale"),
        )
        item["lane_evidence_chip"] = _lane_evidence_chip(
            attestation_override_rationale=item.get("attestation_override_rationale"),
        )
        item["byline_line"] = _byline_line(
            item.get("author_line"),
            expected_author_line=expected_author_line,
            attested=bool(item["lane_attestation_chip"].get("attested")),
        )
        shaped.append(item)
    return shaped


def _verdicts(ctx: RepoHandlerContext, *, job_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT v.*
            FROM striatumd.verdicts v
            WHERE v.repository_id = %s AND v.job_id = %s
            ORDER BY v.created_at DESC, v.verdict_id DESC
            """,
            (ctx.repository_id, job_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def _override_verdict_ids(ctx: RepoHandlerContext, *, run_id: str) -> set[str]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT e.payload_json
            FROM striatumd.events e
            WHERE e.repository_id = %s
              AND e.run_id = %s
              AND e.event_type = 'verdict.overridden'
            """,
            (ctx.repository_id, run_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    verdict_ids: set[str] = set()
    for row in rows:
        payload = parse_json_object(row.get("payload_json"))
        verdict_id = payload.get("verdict_id")
        if isinstance(verdict_id, str) and verdict_id:
            verdict_ids.add(verdict_id)
    return verdict_ids


def _shape_verdicts(
    ctx: RepoHandlerContext,
    *,
    verdicts: list[dict[str, Any]],
    override_verdict_ids: set[str],
) -> list[dict[str, Any]]:
    shaped: list[dict[str, Any]] = []
    for verdict in verdicts:
        item = dict(verdict)
        provenance = (
            "operator-override"
            if item.get("verdict_id") in override_verdict_ids
            else "natural"
        )
        item["source"] = provenance
        item["provenance"] = provenance
        item["override_rationale"] = (
            item.get("rationale") if provenance == "operator-override" else None
        )
        item["lane_attestation_chip"] = _lane_attestation_chip(
            ctx,
            session_id=str(item["session_id"]) if item.get("session_id") else None,
        )
        shaped.append(item)
    return shaped


def _process_evidence(ctx: RepoHandlerContext, *, run_id: str, job_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT p.*
            FROM striatumd.process_executions p
            WHERE p.repository_id = %s AND p.run_id = %s AND p.job_id = %s
            ORDER BY p.started_at DESC, p.process_id DESC
            """,
            (ctx.repository_id, run_id, job_id),
        )
        process_rows = [row_to_json(row) for row in cur.fetchall()]
    blockers_by_process: dict[str, list[dict[str, Any]]] = {}
    for blocker in _open_blockers(ctx, run_id=run_id, job_id=job_id):
        payload = blocker.get("payload")
        if isinstance(payload, Mapping) and payload.get("process_id"):
            blockers_by_process.setdefault(str(payload["process_id"]), []).append(blocker)
    for process in process_rows:
        process["command"] = parse_json_list(process.get("command_json"))
        process["blockers"] = blockers_by_process.get(str(process.get("process_id")), [])
        process["diagnostics"] = [
            blocker.get("payload")
            for blocker in process["blockers"]
            if isinstance(blocker.get("payload"), Mapping)
        ]
    return process_rows


def _open_blockers(ctx: RepoHandlerContext, *, run_id: str, job_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT b.*, j.workflow_job_id, j.job_type, j.state AS job_state
            FROM striatumd.blockers b
            LEFT JOIN striatumd.jobs j
              ON j.repository_id = b.repository_id
             AND j.job_id = b.job_id
            WHERE b.repository_id = %s
              AND b.run_id = %s
              AND b.job_id = %s
              AND b.state = 'open'
            ORDER BY
              CASE b.severity WHEN 'human_checkpoint' THEN 0 WHEN 'blocked' THEN 1 ELSE 2 END,
              b.created_at
            """,
            (ctx.repository_id, run_id, job_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    blockers: list[dict[str, Any]] = []
    for row in rows:
        item = dict(row)
        payload = parse_json_object(item.get("payload_json"))
        item["payload"] = payload
        item["recipes"] = _recipes_for_blocker(item)
        blockers.append(item)
    return blockers


def _recipes_for_blocker(blocker: Mapping[str, Any]) -> list[str]:
    payload = blocker.get("payload")
    payload_obj = payload if isinstance(payload, dict) else {}
    recipes = payload_obj.get("recovery_commands")
    if isinstance(recipes, list):
        return [str(recipe) for recipe in recipes if recipe]
    run_id = str(blocker.get("run_id") or "")
    job_id = str(blocker.get("job_id") or "")
    blocker_id = str(blocker.get("blocker_id") or "")
    kind = str(blocker.get("blocker_kind") or "")
    severity = str(blocker.get("severity") or "")
    if severity == "human_checkpoint":
        return [
            f"striatum checkpoint resolve --blocker-id {blocker_id} --action continue",
            f"striatum checkpoint resolve --blocker-id {blocker_id} --action cancel",
        ]
    if kind.startswith("process_") and run_id:
        return [f"striatum recovery process-reconcile --run-id {run_id}"]
    if job_id:
        return [
            f'striatum recovery cancel-job --run-id {run_id} --job-id {job_id} --reason "operator inspected blocker {blocker_id}"'
        ]
    return []


def _lane_attestation_chip(ctx: RepoHandlerContext, *, session_id: str | None) -> dict[str, Any]:
    if not session_id:
        return {
            "state": "unattested",
            "attested": False,
            "reason": "session_missing",
            "supervisor_id": None,
            "label": "unattested",
        }
    attestation = session_lane_attestation(ctx, session_id=session_id)
    return {
        "state": attestation.get("state"),
        "attested": bool(attestation.get("attested")),
        "reason": attestation.get("reason"),
        "supervisor_id": attestation.get("supervisor_id"),
        "label": attestation.get("state"),
    }


def _recorded_artifact_attestation_chip(
    author_line: Any,
    *,
    expected_author_line: Any = None,
    attestation_override_rationale: Any = None,
) -> dict[str, Any]:
    actual = str(author_line).strip().lower() if author_line else ""
    expected = (
        str(expected_author_line).strip().lower()
        if expected_author_line is not None
        else ""
    )
    if attestation_override_rationale:
        return {
            "state": "unattested",
            "attested": False,
            "reason": "operator_override",
            "supervisor_id": None,
            "label": "unattested",
        }
    if actual.startswith("author: operator"):
        return {
            "state": "unattested",
            "attested": False,
            "reason": "operator_byline",
            "supervisor_id": None,
            "label": "unattested",
        }
    if expected and actual == expected:
        return {
            "state": "attested",
            "attested": True,
            "reason": None,
            "supervisor_id": None,
            "label": "attested",
        }
    return {
        "state": "unattested",
        "attested": False,
        "reason": "expected_author_line_mismatch" if expected else "expected_author_line_missing",
        "supervisor_id": None,
        "label": "unattested",
    }


def _lane_evidence_chip(*, attestation_override_rationale: Any = None) -> dict[str, Any]:
    rationale = (
        str(attestation_override_rationale).strip()
        if attestation_override_rationale is not None
        else ""
    )
    if rationale:
        return {
            "state": "override",
            "label": "override",
            "muted": False,
            "rationale": rationale,
        }
    return {
        "state": "not_yet_correlated",
        "label": "not yet correlated",
        "muted": True,
        "rationale": None,
    }


def _byline_line(
    author_line: Any,
    *,
    expected_author_line: Any = None,
    attested: bool | None = None,
    operator_label: Any = None,
) -> dict[str, Any]:
    actual = str(author_line) if author_line is not None else None
    expected = str(expected_author_line) if expected_author_line is not None else None
    if attested is False:
        label = str(operator_label).strip() if operator_label else ""
        display = f"author: operator [self-declared: {label}]" if label else "author: operator"
    else:
        display = actual if actual else "author: <missing>"
    return {
        "author_line": actual,
        "expected_author_line": expected,
        "display": display,
        "attested": attested,
        "matches_expected": (
            None if actual is None or expected is None else actual == expected
        ),
    }


def _state_chip(kind: str, state: Any) -> dict[str, str]:
    normalized = str(state or "unknown")
    return {
        "kind": kind,
        "state": normalized,
        "label": normalized,
        "css_class": f"status-pill status-{normalized}",
    }
