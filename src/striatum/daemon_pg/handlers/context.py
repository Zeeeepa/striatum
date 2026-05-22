"""Shared helpers for repository-scoped daemon PostgreSQL handlers."""

from __future__ import annotations

import hashlib
import json
import os
import posixpath
import uuid
from collections.abc import Mapping
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any, cast

from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from striatum.artifact_contracts import (
    ALLOWED_ARTIFACT_KINDS,
    _canonical_byline_form,
    _first_author_line,
    ensure_required_front_matter,
    markdown_title_block_author_lines,
    validate_artifact_front_matter,
)
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import ArtifactError, InvalidTransitionError, LeaseError, NotFoundError
from striatum.identity import artifact_author_identity, process_start_time, validate_operator_label
from striatum.primitives import JsonObject, json_dumps, sha256_bytes
from striatum.repo_policy import (
    adapter_constraint_enforcement,
    adapter_enforcement_satisfies,
    path_allowed,
    repo_relative_path,
)


def _jsonb(value: object) -> Jsonb:
    return Jsonb(value)


def _json_loads(value: object) -> JsonObject:
    if isinstance(value, dict):
        return cast(JsonObject, value)
    loaded = json.loads(str(value))
    if not isinstance(loaded, dict):
        raise InvalidTransitionError("stored JSON value is not an object")
    return cast(JsonObject, loaded)


def _json_list(value: object) -> list[Any]:
    if isinstance(value, list):
        return value
    loaded = json.loads(str(value))
    if not isinstance(loaded, list):
        raise InvalidTransitionError("stored JSON value is not a list")
    return loaded


def _event_payload(value: object) -> dict[str, Any]:
    if isinstance(value, dict):
        return {str(key): item for key, item in value.items() if key != "_event_chain"}
    loaded = json.loads(str(value))
    if not isinstance(loaded, dict):
        return {}
    return {str(key): item for key, item in loaded.items() if key != "_event_chain"}


def _ts(value: object) -> str:
    if isinstance(value, datetime):
        dt = value if value.tzinfo is not None else value.replace(tzinfo=UTC)
        return dt.astimezone(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    return str(value)


@dataclass(frozen=True)
class RepoHandlerContext:
    pg_conn: Any
    repository_id: str
    repo_root: Path
    auth: RpcAuthContext

    def now(self) -> str:
        return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")

    def expires_after(self, seconds: int) -> str:
        return (
            datetime.now(UTC) + timedelta(seconds=seconds)
        ).replace(microsecond=0).isoformat().replace("+00:00", "Z")

    def new_id(self, prefix: str) -> str:
        return f"{prefix}_{uuid.uuid4().hex}"

    def cursor(self) -> Any:
        return self.pg_conn.cursor(row_factory=dict_row)

    def row_by_id(self, table: str, column: str, value: str, *, for_update: bool = False) -> dict[str, Any]:
        suffix = " FOR UPDATE" if for_update else ""
        with self.cursor() as cur:
            cur.execute(
                f"SELECT * FROM striatumd.{table} WHERE repository_id = %s AND {column} = %s{suffix}",
                (self.repository_id, value),
            )
            row = cur.fetchone()
        if row is None:
            raise NotFoundError(f"could not find {table} row for {column}={value!r}")
        return dict(row)

    def append_event(
        self,
        *,
        run_id: str | None,
        event_type: str,
        actor_session_id: str | None = None,
        job_id: str | None = None,
        message_id: str | None = None,
        artifact_id: str | None = None,
        lease_id: str | None = None,
        payload: Mapping[str, Any] | None = None,
    ) -> int:
        """Append a repository event and return its generated id.

        Schema v6 (migration 0006) added dedicated ``previous_hash`` and
        ``row_hash`` columns to ``striatumd.events`` plus a
        ``striatumd.repo_event_chain_heads`` singleton-per-repo carrying the
        latest event's row hash. This handler reads the head ``FOR UPDATE``
        to serialize concurrent appenders, computes the new ``row_hash``
        over the canonical material (same algorithm as v5 but anchored on
        the columns now, not on ``payload_json._event_chain``), inserts the
        event with the columns populated, and updates the head row.

        Request-level daemon audit hash continuity remains in
        ``striatumd.audit_log``; this chain covers per-repository event
        provenance.
        """
        created_at = self.now()
        base_payload = dict(payload or {})
        # Schema v6 F3: serialize concurrent appenders per-repository inside
        # an explicit transaction so the FOR UPDATE lock on the parent
        # repositories row holds across the head-read and head-write. The
        # repositories lock is used instead of the heads row because the
        # head row may not exist yet on first event per repo; FOR UPDATE
        # on an absent row does not take a lock. ``conn.transaction()``
        # is a savepoint when re-entered from a handler that already
        # opened a transaction (psycopg supports nesting).
        with self.pg_conn.transaction(), self.cursor() as cur:
            cur.execute(
                "SELECT 1 FROM striatumd.repositories "
                "WHERE repository_id = %s FOR UPDATE",
                (self.repository_id,),
            )
            cur.execute(
                """
                SELECT last_hash
                FROM striatumd.repo_event_chain_heads
                WHERE repository_id = %s
                """,
                (self.repository_id,),
            )
            head_row = cur.fetchone()
            previous_hash = (
                str(head_row["last_hash"])
                if isinstance(head_row, dict) and head_row.get("last_hash") is not None
                else (str(head_row[0]) if head_row is not None and head_row[0] is not None else None)
            )
            cur.execute("SELECT pg_get_serial_sequence('striatumd.events', 'event_id') AS sequence_name")
            sequence_row = cur.fetchone()
            if sequence_row is None or sequence_row["sequence_name"] is None:
                raise InvalidTransitionError("events identity sequence could not be resolved")
            cur.execute("SELECT nextval(%s) AS event_id", (sequence_row["sequence_name"],))
            event_row = cur.fetchone()
            if event_row is None:
                raise InvalidTransitionError("events identity sequence did not return a row id")
            event_id = int(event_row["event_id"])
            row_material = {
                "repository_id": self.repository_id,
                "event_id": event_id,
                "run_id": run_id,
                "event_type": event_type,
                "actor_session_id": actor_session_id,
                "job_id": job_id,
                "message_id": message_id,
                "artifact_id": artifact_id,
                "lease_id": lease_id,
                "payload_json": base_payload,
                "created_at": created_at,
            }
            row_hash = canonical_event_hash(row_material, previous_hash=previous_hash)
            cur.execute(
                """
                INSERT INTO striatumd.events (
                  repository_id, event_id, run_id, event_type, actor_session_id, job_id,
                  message_id, artifact_id, lease_id, payload_json, created_at,
                  previous_hash, row_hash
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                """,
                (
                    self.repository_id,
                    event_id,
                    run_id,
                    event_type,
                    actor_session_id,
                    job_id,
                    message_id,
                    artifact_id,
                    lease_id,
                    _jsonb(base_payload),
                    created_at,
                    previous_hash,
                    row_hash,
                ),
            )
            cur.execute(
                """
                INSERT INTO striatumd.repo_event_chain_heads(
                  repository_id, last_event_id, last_hash, updated_at
                ) VALUES (%s, %s, %s, now())
                ON CONFLICT (repository_id)
                DO UPDATE SET last_event_id = EXCLUDED.last_event_id,
                              last_hash = EXCLUDED.last_hash,
                              updated_at = now()
                """,
                (self.repository_id, event_id, row_hash),
            )
        return event_id


def canonical_event_hash(row: Mapping[str, Any], *, previous_hash: str | None = None) -> str:
    payload = {
        "previous_hash": previous_hash,
        "repository_id": row.get("repository_id"),
        "event_id": row.get("event_id"),
        "run_id": row.get("run_id"),
        "event_type": row.get("event_type"),
        "actor_session_id": row.get("actor_session_id"),
        "job_id": row.get("job_id"),
        "message_id": row.get("message_id"),
        "artifact_id": row.get("artifact_id"),
        "lease_id": row.get("lease_id"),
        "payload_json": _event_payload(row.get("payload_json") or {}),
        "created_at": _ts(row.get("created_at")),
    }
    return hashlib.sha256(json_dumps(payload).encode("utf-8")).hexdigest()


def event_row_hash(row: Mapping[str, Any]) -> str:
    """Return the stored row_hash for ``row``, computing it if absent.

    Schema v6 (migration 0006) stores ``row_hash`` and ``previous_hash``
    as dedicated columns on ``striatumd.events``. Pre-v6 rows are
    backfilled from ``payload_json._event_chain`` by the migration, so
    every persisted event has a column-side ``row_hash``. The legacy
    ``_event_chain`` payload key remains supported as a fallback so this
    helper works on in-memory event dicts that haven't gone through PG.
    """
    stored_hash = row.get("row_hash")
    if isinstance(stored_hash, str) and stored_hash:
        return stored_hash
    payload = row.get("payload_json") or {}
    if not isinstance(payload, dict):
        payload = json.loads(str(payload))
    chain = payload.get("_event_chain") if isinstance(payload, dict) else None
    if isinstance(chain, dict) and isinstance(chain.get("row_hash"), str):
        return str(chain["row_hash"])
    previous = chain.get("previous_hash") if isinstance(chain, dict) else None
    if previous is not None and not isinstance(previous, str):
        previous = None
    stored_previous = row.get("previous_hash")
    if previous is None and isinstance(stored_previous, str):
        previous = stored_previous
    return canonical_event_hash(row, previous_hash=previous)


def transaction(ctx: RepoHandlerContext) -> Any:
    return ctx.pg_conn.transaction()


def session_lane_attestation(ctx: RepoHandlerContext, *, session_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.sessions
            WHERE repository_id = %s AND session_id = %s
            """,
            (ctx.repository_id, session_id),
        )
        session = cur.fetchone()
    if session is None:
        return {
            "attested": False,
            "state": "unattested",
            "supervisor_id": None,
            "pid": None,
            "reason": "session_missing",
        }
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.process_supervisors
            WHERE repository_id = %s AND session_id = %s AND state = 'attached'
            ORDER BY started_at DESC
            LIMIT 1
            """,
            (ctx.repository_id, session_id),
        )
        supervisor = cur.fetchone()
    if supervisor is None:
        return {
            "attested": False,
            "state": "unattested",
            "supervisor_id": None,
            "pid": None,
            "reason": "no_attached_supervisor",
        }
    supervisor_id = str(supervisor["supervisor_id"])
    pid = _supervisor_pid(supervisor.get("pid"))
    reason = _inactive_supervisor_reason(ctx, session=session, supervisor=supervisor, pid=pid)
    if reason is not None:
        return {
            "attested": False,
            "state": "unattested",
            "supervisor_id": supervisor_id,
            "pid": pid,
            "reason": reason,
        }
    return {
        "attested": True,
        "state": "attested",
        "supervisor_id": supervisor_id,
        "pid": pid,
        "reason": None,
    }


def _inactive_supervisor_reason(
    ctx: RepoHandlerContext,
    *,
    session: Mapping[str, Any],
    supervisor: Mapping[str, Any],
    pid: int | None,
) -> str | None:
    if str(supervisor["run_id"]) != str(session["run_id"]):
        return "run_mismatch"
    if str(supervisor["session_id"]) != str(session["session_id"]):
        return "session_mismatch"
    if pid is None or not _pid_alive(pid):
        return "pid_gone"
    expected_start = supervisor.get("pid_start_time")
    if expected_start is None or expected_start == "":
        return "pid_identity_unavailable"
    current_start = process_start_time(pid)
    if current_start != str(expected_start):
        return "pid_identity_mismatch"
    expected_command = _session_lane_command(ctx, session=session)
    if expected_command is None:
        return "lane_command_missing"
    recorded_command = _command_json_string_list(supervisor.get("command_json"))
    if recorded_command is None:
        return "supervisor_command_invalid"
    if recorded_command != expected_command:
        return "lane_command_mismatch"
    return None


def _session_lane_command(ctx: RepoHandlerContext, *, session: Mapping[str, Any]) -> list[str] | None:
    try:
        workflow = workflow_for_run(ctx, run_id=str(session["run_id"]))
    except (json.JSONDecodeError, InvalidTransitionError, NotFoundError):
        return None
    lanes = workflow.get("lanes")
    if not isinstance(lanes, dict):
        return None
    lane = lanes.get(str(session["lane_id"]))
    if not isinstance(lane, dict):
        return None
    command = lane.get("command")
    if not isinstance(command, list) or not all(isinstance(part, str) for part in command):
        return None
    return list(cast(list[str], command))


def _command_json_string_list(value: object) -> list[str] | None:
    try:
        command = _json_list(value)
    except (json.JSONDecodeError, InvalidTransitionError, TypeError):
        return None
    if not all(isinstance(part, str) for part in command):
        return None
    return list(cast(list[str], command))


def _supervisor_pid(value: object) -> int | None:
    if value is None:
        return None
    if isinstance(value, int):
        return value
    if not isinstance(value, str):
        return None
    try:
        return int(value)
    except ValueError:
        return None


def _pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except (ProcessLookupError, PermissionError):
        return False
    return True


def active_lease_for(
    ctx: RepoHandlerContext,
    *,
    lease_id: str,
    session_id: str,
    job_id: str | None = None,
) -> dict[str, Any]:
    lease = ctx.row_by_id("leases", "lease_id", lease_id, for_update=True)
    if lease["state"] != "active":
        raise LeaseError("lease is not active")
    if lease["owner_session_id"] != session_id:
        raise LeaseError("lease is owned by another session")
    if job_id is not None and lease["resource_id"] != job_id:
        raise LeaseError("lease does not belong to the job")
    if _ts(lease["expires_at"]) < ctx.now():
        raise LeaseError("lease is expired")
    return lease


def is_repo_write(job: Mapping[str, Any]) -> bool:
    scope = _json_loads(job["write_scope_json"])
    return scope.get("repo_write") is True or scope.get("mode") == "repo_write"


def workflow_for_run(ctx: RepoHandlerContext, *, run_id: str) -> JsonObject:
    run = ctx.row_by_id("runs", "run_id", run_id)
    snapshot = ctx.row_by_id("workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"]))
    return _json_loads(snapshot["workflow_json"])


def enqueue_job(ctx: RepoHandlerContext, *, job_id: str) -> str:
    job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
    if job["state"] not in ("blocked", "queued"):
        raise InvalidTransitionError("job is not enqueueable")
    now = ctx.now()
    message_id = ctx.new_id("msg")
    selector = _json_loads(job["lane_selector_json"])
    target_lane = selector.get("lane_id")
    if target_lane is not None and not isinstance(target_lane, str):
        target_lane = None
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state, priority,
              target_role_id, target_lane_id, payload_json, claim_count,
              max_claims, created_at, updated_at
            )
            VALUES (%s, %s, %s, %s, 'work', 'pending', 0, %s, %s, %s, 0, %s, %s, %s)
            """,
            (
                ctx.repository_id,
                message_id,
                job["run_id"],
                job_id,
                job["role_id"],
                target_lane,
                _jsonb({}),
                job["max_attempts"],
                now,
                now,
            ),
        )
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'queued', ready_at = %s, current_message_id = %s
            WHERE repository_id = %s AND job_id = %s
            """,
            (now, message_id, ctx.repository_id, job_id),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="queue.message_enqueued",
        job_id=job_id,
        message_id=message_id,
        payload={"workflow_job_id": job["workflow_job_id"]},
    )
    return message_id


def verify_required_artifacts(ctx: RepoHandlerContext, *, job_id: str) -> None:
    job = ctx.row_by_id("jobs", "job_id", job_id)
    expected = _json_list(job["expected_artifacts_json"])
    for item in expected:
        if not isinstance(item, dict) or item.get("required") is not True:
            continue
        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT 1 FROM striatumd.artifacts
                WHERE repository_id = %s AND job_id = %s AND logical_name = %s
                  AND artifact_kind = %s AND repo_path = %s
                LIMIT 1
                """,
                (
                    ctx.repository_id,
                    job_id,
                    item.get("logical_name"),
                    item.get("kind"),
                    item.get("path"),
                ),
            )
            found = cur.fetchone()
        if found is None:
            raise InvalidTransitionError(
                "required artifact is missing: "
                f"logical_name={item.get('logical_name')!r}, kind={item.get('kind')!r}, path={item.get('path')!r}"
            )


def maybe_enqueue_downstream(ctx: RepoHandlerContext, *, completed_job_id: str) -> None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT job_id FROM striatumd.job_dependencies
            WHERE repository_id = %s AND depends_on_job_id = %s
            """,
            (ctx.repository_id, completed_job_id),
        )
        dependents = [str(row["job_id"]) for row in cur.fetchall()]
    for job_id in dependents:
        job = ctx.row_by_id("jobs", "job_id", job_id)
        if job["state"] == "blocked" and dependencies_satisfied(ctx, job_id=job_id):
            enqueue_job(ctx, job_id=job_id)


def dependencies_satisfied(ctx: RepoHandlerContext, *, job_id: str) -> bool:
    with ctx.cursor() as cur:
        cur.execute(
            "SELECT * FROM striatumd.job_dependencies WHERE repository_id = %s AND job_id = %s",
            (ctx.repository_id, job_id),
        )
        deps = [dict(row) for row in cur.fetchall()]
    for dep in deps:
        upstream = ctx.row_by_id("jobs", "job_id", str(dep["depends_on_job_id"]))
        gate = _json_loads(dep["gate_json"])
        if upstream["state"] != "completed":
            return False
        required = gate.get("requires_verdict")
        if required is None:
            continue
        if latest_verdict(ctx, job_id=str(upstream["job_id"])) not in set(required):
            return False
    return True


def latest_verdict(ctx: RepoHandlerContext, *, job_id: str) -> str | None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT verdict FROM striatumd.verdicts
            WHERE repository_id = %s AND job_id = %s
            ORDER BY created_at DESC, verdict_id DESC
            LIMIT 1
            """,
            (ctx.repository_id, job_id),
        )
        row = cur.fetchone()
    return str(row["verdict"]) if row is not None else None


def maybe_complete_run(ctx: RepoHandlerContext, *, run_id: str) -> None:
    run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
    with ctx.cursor() as cur:
        cur.execute(
            "SELECT 1 FROM striatumd.jobs WHERE repository_id = %s AND run_id = %s AND state = 'failed' LIMIT 1",
            (ctx.repository_id, run_id),
        )
        failed = cur.fetchone()
        cur.execute(
            """
            SELECT 1 FROM striatumd.jobs
            WHERE repository_id = %s AND run_id = %s
              AND state NOT IN ('completed','skipped','canceled')
            LIMIT 1
            """,
            (ctx.repository_id, run_id),
        )
        remaining = cur.fetchone()
    if failed is not None and run["state"] == "running":
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.runs
                SET state = 'failed', completed_at = %s, stop_reason = 'job_failed'
                WHERE repository_id = %s AND run_id = %s
                """,
                (now, ctx.repository_id, run_id),
            )
        ctx.append_event(run_id=run_id, event_type="run.failed", payload={"reason": "job_failed"})
        close_remaining_sessions(ctx, run_id=run_id, source="run_failed", reason="run_failed")
        return
    if remaining is not None or run["state"] != "running":
        return
    with ctx.cursor() as cur:
        cur.execute(
            "SELECT 1 FROM striatumd.jobs WHERE repository_id = %s AND run_id = %s AND state = 'completed' LIMIT 1",
            (ctx.repository_id, run_id),
        )
        has_completed = cur.fetchone()
    now = ctx.now()
    state, stop_reason, event_type, source, payload = (
        ("completed", "all_jobs_terminal", "run.completed", "run_completed", None)
        if has_completed is not None
        else ("canceled", "all_jobs_canceled", "run.canceled", "run_canceled", {"reason": "all_jobs_canceled"})
    )
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.runs
            SET state = %s, completed_at = %s, stop_reason = %s
            WHERE repository_id = %s AND run_id = %s
            """,
            (state, now, stop_reason, ctx.repository_id, run_id),
        )
    ctx.append_event(run_id=run_id, event_type=event_type, payload=payload)
    close_remaining_sessions(ctx, run_id=run_id, source=source, reason=source)


def close_remaining_sessions(ctx: RepoHandlerContext, *, run_id: str, source: str, reason: str) -> list[JsonObject]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT * FROM striatumd.sessions
            WHERE repository_id = %s AND run_id = %s AND state = 'active'
            ORDER BY registered_at
            FOR UPDATE
            """,
            (ctx.repository_id, run_id),
        )
        rows = [dict(row) for row in cur.fetchall()]
    closed: list[JsonObject] = []
    now = ctx.now()
    for row in rows:
        sid = str(row["session_id"])
        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT 1 FROM striatumd.leases
                WHERE repository_id = %s AND owner_session_id = %s AND state = 'active'
                LIMIT 1
                """,
                (ctx.repository_id, sid),
            )
            if cur.fetchone() is not None:
                continue
            cur.execute(
                """
                UPDATE striatumd.sessions
                SET state = 'closed', closed_at = %s, close_reason = %s
                WHERE repository_id = %s AND session_id = %s
                """,
                (now, reason, ctx.repository_id, sid),
            )
        ctx.append_event(
            run_id=run_id,
            event_type="session.closed",
            actor_session_id=sid,
            payload={
                "session_id": sid,
                "role_id": row["role_id"],
                "lane_id": row["lane_id"],
                "reason": reason,
                "source": source,
            },
        )
        closed.append({"session_id": sid, "role_id": row["role_id"], "lane_id": row["lane_id"]})
    return closed


def build_packet(
    ctx: RepoHandlerContext,
    *,
    run: Mapping[str, Any],
    session: Mapping[str, Any],
    job: Mapping[str, Any],
    message_id: str,
    lease_id: str,
    lease_expires_at: str,
    packet_id: str,
) -> JsonObject:
    snapshot = ctx.row_by_id("workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"]))
    workflow = _json_loads(snapshot["workflow_json"])
    roles = workflow.get("roles", {})
    role_def = roles.get(str(job["role_id"]), {}) if isinstance(roles, dict) else {}
    write_scope = _json_loads(job["write_scope_json"])
    expected_artifacts = _json_list(job["expected_artifacts_json"])
    lane = _json_loads(job["lane_selector_json"]).get("lane_id")
    lane_id = lane if isinstance(lane, str) else None
    lanes = workflow.get("lanes", {})
    lane_config = lanes.get(lane_id, {}) if isinstance(lanes, dict) and lane_id is not None else {}
    worktree_isolation = lane_worktree_isolation(workflow, lane_id)
    worktree_required = worktree_isolation == "per_job" and is_repo_write(job)
    attestation = session_lane_attestation(ctx, session_id=str(session["session_id"]))
    author = artifact_author_identity(
        workflow,
        role_id=str(job["role_id"]),
        lane_id=lane_id,
        workflow_job_id=str(job["workflow_job_id"]),
        ordinal=int(session["ordinal"]),
        attested=bool(attestation["attested"]),
        operator_label=session.get("operator_label"),
    )
    author_line = author["line"]
    if author_line is None:
        raise InvalidTransitionError("session author line could not be derived")
    packet: JsonObject = {
        "packet_version": "striatum.work-packet.v1",
        "packet_id": packet_id,
        "run": {
            "run_id": run["run_id"],
            "workflow_id": workflow.get("workflow_id"),
            "repo_root": str(ctx.repo_root),
            "branch": {"name": run.get("branch_name"), "confirmed": run.get("branch_confirmed_at") is not None},
        },
        "session": {
            "session_id": session["session_id"],
            "slug": session["slug"],
            "role_id": session["role_id"],
            "lane_id": session["lane_id"],
            "capabilities": _json_list(session["capabilities_json"]),
        },
        "lane_attestation": attestation,
        "lease": {
            "lease_id": lease_id,
            "message_id": message_id,
            "expires_at": lease_expires_at,
            "heartbeat_after_seconds": 300,
        },
        "job": {
            "job_id": job["job_id"],
            "workflow_job_id": job["workflow_job_id"],
            "attempt": job["attempt"],
            "type": job["job_type"],
            "title": job["title"],
            "author": author,
            "objective": _json_loads(job["capability_requirements_json"]).get("objective"),
            "fresh_session_required": bool(job["fresh_session_required"]),
        },
        "role": {
            "role_id": job["role_id"],
            "definition_path": role_def.get("definition_path") if isinstance(role_def, dict) else None,
            "inline_summary": role_def.get("summary") if isinstance(role_def, dict) else None,
        },
        "context": {"docs": workflow.get("context_docs", []), "content_mode": "references"},
        "task_prompt": packet_task_prompt(
            _json_loads(job["capability_requirements_json"]).get("task_prompt", {}),
            snapshot=snapshot,
        ),
        "inputs": _json_loads(job["capability_requirements_json"]).get("inputs", []),
        "write_scope": write_scope,
        "adapter_constraints": build_adapter_constraints(lane_config if isinstance(lane_config, dict) else {}),
        "expected_artifacts": expected_artifacts_with_author(expected_artifacts, author_line=author_line),
        "worktree_isolation": worktree_isolation,
        "worktree_required": worktree_required,
        "commands": build_packet_commands(
            session_id=str(session["session_id"]),
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
            worktree_required=worktree_required,
        ),
        "artifact_policy": {"publish_transcripts": False, "curated_artifacts_only": True},
    }
    review_policy = build_review_policy(workflow, workflow_job_id=str(job["workflow_job_id"]))
    if review_policy is not None:
        packet["review_policy"] = review_policy
    profile = harness_profile_view(workflow, lane_id=lane_id)
    if profile is not None:
        packet["harness_profile"] = profile
    return packet


def packet_task_prompt(task_prompt: object, *, snapshot: Mapping[str, Any]) -> JsonObject:
    if not isinstance(task_prompt, Mapping):
        return {}
    result: JsonObject = dict(task_prompt)
    raw_path = result.get("path")
    if not isinstance(raw_path, str) or not raw_path or Path(raw_path).is_absolute():
        return result
    source_path = snapshot.get("source_path")
    if not isinstance(source_path, str) or not source_path:
        return result
    source_dir = posixpath.dirname(source_path)
    if source_dir in {"", "."}:
        return result
    cleaned_raw = posixpath.normpath(raw_path)
    normalized = posixpath.normpath(posixpath.join(source_dir, raw_path))
    if normalized == "." or normalized == ".." or normalized.startswith("../"):
        return result
    if normalized != cleaned_raw:
        result["path"] = normalized
        result["workflow_relative_path"] = raw_path
        result["workflow_source_path"] = source_path
    return result


def lane_worktree_isolation(workflow: Mapping[str, Any], lane_id: str | None) -> str:
    if lane_id is None:
        return "off"
    lanes = workflow.get("lanes")
    lane = lanes.get(lane_id) if isinstance(lanes, dict) else None
    mode = lane.get("worktree_isolation") if isinstance(lane, dict) else None
    return mode if mode in {"off", "per_job"} else "off"


def expected_artifacts_with_author(expected_artifacts: object, *, author_line: str) -> list[object]:
    if not isinstance(expected_artifacts, list):
        return []
    enriched: list[object] = []
    for artifact in expected_artifacts:
        if isinstance(artifact, dict):
            item = dict(artifact)
            item["author_line"] = author_line
            enriched.append(item)
        else:
            enriched.append(artifact)
    return enriched


def build_packet_commands(*, session_id: str, job_id: str, message_id: str, lease_id: str, worktree_required: bool) -> JsonObject:
    commands: JsonObject = {
        "ack": f"striatum ack --session-id {session_id} --message-id {message_id} --lease-id {lease_id}",
        "heartbeat": f"striatum heartbeat --session-id {session_id} --lease-id {lease_id}",
        "publish_artifact": f"striatum publish-artifact --session-id {session_id} --job-id {job_id} --lease-id {lease_id}",
        "block": f"striatum block --session-id {session_id} --job-id {job_id} --lease-id {lease_id}",
        "verdict": f"striatum verdict --session-id {session_id} --job-id {job_id} --lease-id {lease_id}",
        "complete": f"striatum complete --session-id {session_id} --job-id {job_id} --lease-id {lease_id}",
    }
    if worktree_required:
        commands["worktree_create"] = (
            f"striatum worktree create --session-id {session_id} --job-id {job_id} --lease-id {lease_id}"
        )
    return commands


def build_adapter_constraints(lane_config: JsonObject) -> JsonObject:
    adapter = lane_config.get("adapter")
    constraints = lane_config.get("constraints", {})
    if not isinstance(constraints, dict):
        constraints = {}
    required = lane_config.get("required_enforcement", {})
    if not isinstance(required, dict):
        required = {}
    enforcement: list[JsonObject] = []
    for key, value in constraints.items():
        if not isinstance(key, str) or not isinstance(value, str):
            continue
        result = adapter_constraint_enforcement(adapter, constraint=key, requested=value)
        required_text = required.get(key) if isinstance(required.get(key), str) else None
        enforcement.append(
            {
                "constraint": key,
                "requested": value,
                "required_enforcement": required_text,
                "enforcement": result,
                "satisfied": required_text is None
                or adapter_enforcement_satisfies(actual=result, required=required_text),
            }
        )
    return {
        "requested": constraints,
        "required_enforcement": required,
        "enforcement": enforcement,
        "satisfied": all(item["satisfied"] for item in enforcement),
    }


def harness_profile_view(workflow: JsonObject, *, lane_id: str | None) -> JsonObject | None:
    if lane_id is None:
        return None
    lanes = workflow.get("lanes")
    lane = lanes.get(lane_id) if isinstance(lanes, dict) else None
    profile_id = lane.get("harness_profile_id") if isinstance(lane, dict) else None
    profiles = workflow.get("harness_profiles")
    body = profiles.get(profile_id) if isinstance(profiles, dict) and isinstance(profile_id, str) else None
    if not isinstance(body, dict):
        return None
    view: JsonObject = {"profile_id": profile_id}
    view.update(body)
    return view


def build_review_policy(workflow: JsonObject, *, workflow_job_id: str) -> JsonObject | None:
    from striatum.workflow import ALLOWED_POSTURES, POSTURE_INSTRUCTIONS

    access_instructions = {
        "document_only": "Read only the target documents listed in inputs. Do not consult other artifacts, ledgers, reports, or repository contents beyond inputs.",
        "artifact_augmented": "You may read the target documents AND the supporting artifacts/reports/ledgers listed in inputs. Do not inspect other repository contents.",
        "repo_level": "You may inspect the repository within the job's declared write_scope.allowed_paths/forbidden_paths. Stay within that scope.",
    }
    context_instructions = {
        "fresh": " This is a fresh-context review. Do not rely on prior thread state from earlier rounds.",
        "cross_round": " You may retain prior context to verify whether previously raised issues were resolved.",
    }
    jobs = workflow.get("jobs", [])
    if not isinstance(jobs, list):
        return None
    job_def = next(
        (item for item in jobs if isinstance(item, dict) and item.get("id") == workflow_job_id and item.get("type") == "review"),
        None,
    )
    if job_def is None:
        return None
    has_access = "reviewer_access_scope" in job_def
    has_context = "reviewer_context_policy" in job_def
    has_posture = "review_posture" in job_def
    if not (has_access or has_context or has_posture):
        return None
    access = job_def.get("reviewer_access_scope") if has_access else "document_only"
    context = job_def.get("reviewer_context_policy") if has_context else "cross_round"
    posture = job_def.get("review_posture") if has_posture else "neutral"
    if not isinstance(access, str) or access not in access_instructions:
        return None
    if not isinstance(context, str) or context not in context_instructions:
        return None
    if not isinstance(posture, str):
        return None
    block: JsonObject = {
        "access_scope": access,
        "context_policy": context,
        "instruction": access_instructions[access]
        + context_instructions[context]
        + (POSTURE_INSTRUCTIONS[posture] if posture in ALLOWED_POSTURES else ""),
    }
    if has_posture:
        block["posture"] = posture
    return block


def resolve_review_posture(ctx: RepoHandlerContext, *, job: Mapping[str, Any]) -> str:
    workflow = workflow_for_run(ctx, run_id=str(job["run_id"]))
    for item in workflow.get("jobs", []):
        if isinstance(item, dict) and item.get("id") == job["workflow_job_id"] and item.get("type") == "review":
            declared = item.get("review_posture")
            return declared if isinstance(declared, str) and declared else "neutral"
    return "neutral"


def after_timestamp(previous: str, candidate: str) -> str:
    if candidate > previous:
        return candidate
    try:
        parsed = datetime.fromisoformat(previous.replace("Z", "+00:00"))
    except ValueError:
        return candidate
    return (parsed + timedelta(seconds=1)).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def publish_artifact_pg(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    logical_name: str,
    path_text: str,
) -> dict[str, object]:
    job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
    active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=job_id)
    if kind == "transcript":
        raise ArtifactError("transcript artifacts are not allowed by default")
    if kind not in ALLOWED_ARTIFACT_KINDS:
        raise ArtifactError(f"artifact kind {kind!r} is not in the allowed kinds list")
    write_scope = _json_loads(job["write_scope_json"])
    if not path_allowed(ctx.repo_root, path_text, write_scope):
        raise ArtifactError("artifact path is outside the job write scope")
    path = repo_relative_path(ctx.repo_root, path_text)
    if not path.exists() or not path.is_file():
        raise ArtifactError("artifact file does not exist")
    payload = ensure_required_front_matter(kind=kind, path=path, payload=path.read_bytes())
    validate_optional_markdown_author_line_pg(ctx, job=job, session_id=session_id, path=path, payload=payload)
    validate_artifact_front_matter(kind=kind, path=path, payload=payload)
    digest = sha256_bytes(payload)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT * FROM striatumd.artifacts
            WHERE repository_id = %s AND run_id = %s AND job_id = %s AND logical_name = %s
            """,
            (ctx.repository_id, job["run_id"], job_id, logical_name),
        )
        existing = cur.fetchone()
    if existing is not None:
        if existing["content_sha256"] == digest and existing["repo_path"] == path_text:
            return {"status": "already_published", "artifact_id": existing["artifact_id"]}
        raise ArtifactError("artifact logical name already exists with different content")
    artifact_id = ctx.new_id("art")
    actual_author_line = _first_author_line(payload)
    now = ctx.now()
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.artifacts (
              repository_id, artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
              created_at, author_line
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, 'create', %s, %s)
            """,
            (
                ctx.repository_id,
                artifact_id,
                job["run_id"],
                job_id,
                session_id,
                logical_name,
                kind,
                path_text,
                digest,
                len(payload),
                now,
                actual_author_line,
            ),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="artifact.published",
        actor_session_id=session_id,
        job_id=job_id,
        artifact_id=artifact_id,
        lease_id=lease_id,
        payload={"logical_name": logical_name, "path": path_text, "sha256": digest},
    )
    return {"status": "published", "artifact_id": artifact_id, "sha256": digest}


def validate_optional_markdown_author_line_pg(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
    path: Path,
    payload: bytes,
) -> None:
    if path.suffix.lower() not in {".md", ".markdown"}:
        return
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError("markdown artifact must be UTF-8") from exc
    author_lines = markdown_title_block_author_lines(text)
    if not author_lines:
        return
    expected = expected_author_line_pg(ctx, job=job, session_id=session_id)
    for line in author_lines:
        if _canonical_byline_form(line) != expected:
            raise ArtifactError("markdown artifact author line must match expected work packet author line")


def expected_author_line_pg(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
) -> str:
    session = ctx.row_by_id("sessions", "session_id", session_id)
    workflow = workflow_for_run(ctx, run_id=str(job["run_id"]))
    lane = _json_loads(job["lane_selector_json"]).get("lane_id")
    lane_id = lane if isinstance(lane, str) else None
    attestation = session_lane_attestation(ctx, session_id=session_id)
    author = artifact_author_identity(
        workflow,
        role_id=str(job["role_id"]),
        lane_id=lane_id,
        workflow_job_id=str(job["workflow_job_id"]),
        ordinal=int(session["ordinal"]),
        attested=bool(attestation["attested"]),
        operator_label=session.get("operator_label"),
    )
    line = author["line"]
    if line is None:
        raise ArtifactError("expected artifact author line could not be derived")
    return line


def workflow_declares_fresh_reviewer(workflow: JsonObject) -> bool:
    jobs = workflow.get("jobs")
    if not isinstance(jobs, list):
        return False
    for job in jobs:
        if not isinstance(job, dict) or job.get("type") != "review":
            continue
        if job.get("reviewer_context_policy") == "fresh" or job.get("fresh_session_required") is True:
            return True
    return False


__all__ = [
    "RepoHandlerContext",
    "active_lease_for",
    "after_timestamp",
    "build_packet",
    "canonical_event_hash",
    "enqueue_job",
    "event_row_hash",
    "is_repo_write",
    "latest_verdict",
    "maybe_complete_run",
    "maybe_enqueue_downstream",
    "publish_artifact_pg",
    "resolve_review_posture",
    "session_lane_attestation",
    "transaction",
    "validate_operator_label",
    "verify_required_artifacts",
    "workflow_declares_fresh_reviewer",
    "workflow_for_run",
    "_json_loads",
    "_json_list",
    "_jsonb",
]
