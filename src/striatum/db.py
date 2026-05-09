"""SQLite helpers and state transitions for the V1 MVP."""

from __future__ import annotations

import hashlib
import json
import sqlite3
import uuid
from contextlib import contextmanager
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any, Iterator, cast

from striatum.errors import (
    StriatumError,
    ArtifactError,
    BranchConfirmationError,
    InvalidTransitionError,
    LeaseError,
    NotFoundError,
)
from striatum.identity import artifact_author_identity
from striatum.migrations import apply_migrations

# JSON columns are intentionally untyped at the SQLite boundary.
JsonObject = dict[str, Any]

STATE_DIR = ".striatum"
DB_NAME = "state.sqlite3"
WORKTREES_SUBDIR = "worktrees"
ADAPTER_ENFORCEMENT_LEVELS = {
    "unsupported": 0,
    "advisory": 1,
    "advisory_strict": 2,
    "enforced": 3,
}


def utc_now() -> str:
    """Return an RFC3339 UTC timestamp."""
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    """Return an opaque stable-enough local id."""
    return f"{prefix}_{uuid.uuid4().hex}"


def json_dumps(value: object) -> str:
    """Serialize JSON deterministically for hashing and storage."""
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def json_loads(value: str) -> JsonObject:
    """Load a JSON object from a SQLite text column."""
    loaded = json.loads(value)
    if not isinstance(loaded, dict):
        raise InvalidTransitionError("stored JSON value is not an object")
    return cast(JsonObject, loaded)


def sha256_bytes(payload: bytes) -> str:
    """Return a hex SHA-256 digest."""
    return hashlib.sha256(payload).hexdigest()


def state_dir(repo: Path) -> Path:
    """Return the repo-local state directory."""
    return repo / STATE_DIR


def db_path(repo: Path) -> Path:
    """Return the repo-local SQLite database path."""
    return state_dir(repo) / DB_NAME


def connect(repo: Path) -> sqlite3.Connection:
    """Connect to the repo-local SQLite database.

    Existing databases are forwarded to the latest schema version through
    :func:`striatum.migrations.apply_migrations` on every connect, so upgrades
    are silent and automatic. Connecting to a database whose schema is newer
    than this install supports raises a clear ``StriatumError``.
    """
    target = db_path(repo)
    already_existed = target.exists()
    conn = sqlite3.connect(target)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute("PRAGMA busy_timeout = 5000")
    if already_existed:
        try:
            apply_migrations(conn)
        except Exception:
            conn.close()
            raise
    return conn


@contextmanager
def transaction(conn: sqlite3.Connection) -> Iterator[sqlite3.Connection]:
    """Run a short write transaction."""
    conn.execute("BEGIN IMMEDIATE")
    try:
        yield conn
    except Exception:
        conn.rollback()
        raise
    else:
        conn.commit()


def init_repo(repo: Path) -> None:
    """Create state storage and initialize schema.

    HARNESS-002 guard: when the repo argument carries a Striatum source
    tree whose ``migrations.LATEST_VERSION`` is higher than the running
    install's, refuse to create a fresh state DB. The alternative is the
    silent failure dogfood-001 hit: ``init`` happily creates a stale-
    schema DB, then the first ``publish-artifact`` for a newer kind
    crashes on the old SQL CHECK with no useful guidance.

    The check only applies to a fresh init (no existing
    ``state.sqlite3``); upgrading an already-initialised DB is handled
    by ``connect``'s ``apply_migrations`` and a stale install would
    raise ``SchemaVersionError`` there only if the install is *newer*
    than the DB. The "install is older than the source tree" foot-gun
    is exactly what HARNESS-002 captured.
    """
    state_dir(repo).mkdir(parents=True, exist_ok=True)
    ignore_path = repo / ".gitignore"
    existing = ignore_path.read_text(encoding="utf-8") if ignore_path.exists() else ""
    if ".striatum/" not in existing.splitlines():
        prefix = "" if existing == "" or existing.endswith("\n") else "\n"
        ignore_path.write_text(f"{existing}{prefix}.striatum/\n", encoding="utf-8")
    if not db_path(repo).exists():
        _refuse_init_when_install_lags_repo(repo)
    with connect(repo) as conn:
        apply_migrations(conn)


def _refuse_init_when_install_lags_repo(repo: Path) -> None:
    """Compare repo source-tree ``LATEST_VERSION`` to the running install."""
    from striatum.migrations import LATEST_VERSION as install_latest

    repo_latest = _read_repo_latest_version(repo)
    if repo_latest is None:
        return
    if install_latest >= repo_latest:
        return
    raise StriatumError(
        "striatum install is older than the repo source tree: "
        f"install LATEST_VERSION={install_latest} < repo LATEST_VERSION={repo_latest}. "
        f"Re-install with `pip install -e {repo}` and try again.",
        exit_code=3,
    )


def _read_repo_latest_version(repo: Path) -> int | None:
    """Read ``LATEST_VERSION`` from ``<repo>/src/striatum/migrations.py``.

    Returns ``None`` if the file is missing (the repo arg may be a
    target repo, not the Striatum source tree itself) or if parsing
    fails — neither case should refuse init. The check is a foot-gun
    guard, not a contract.
    """
    candidate = repo / "src" / "striatum" / "migrations.py"
    if not candidate.is_file():
        return None
    text = candidate.read_text(encoding="utf-8")
    namespace: dict[str, Any] = {}
    try:
        # Standalone parse of the LATEST_VERSION assignment without
        # importing (which would re-import the running install). We
        # only need the integer literal at the end of the file.
        for line in text.splitlines():
            stripped = line.strip()
            if stripped.startswith("LATEST_VERSION") and "=" in stripped:
                exec(compile(line, str(candidate), "exec"), namespace)
                value = namespace.get("LATEST_VERSION")
                if isinstance(value, int):
                    return value
                # Could be `LATEST_VERSION: int = MIGRATIONS[-1].version` —
                # fall through to the harder path.
        # Fall back: scan for `Migration(version=N, ...)` entries and
        # take the max. This is enough to keep the guard correct even
        # when the source uses the dynamic computed form.
        import re

        versions = [int(match.group(1)) for match in re.finditer(r"Migration\(version=(\d+)", text)]
        if versions:
            return max(versions)
    except (SyntaxError, NameError, ValueError, TypeError):
        return None
    return None


def ensure_initialized(repo: Path) -> None:
    """Raise if the repo has not been initialized."""
    if not db_path(repo).exists():
        raise StriatumError("striatum state is not initialized; run striatum init", exit_code=3)


def insert_event(
    conn: sqlite3.Connection,
    *,
    run_id: str | None,
    event_type: str,
    actor_session_id: str | None = None,
    job_id: str | None = None,
    message_id: str | None = None,
    artifact_id: str | None = None,
    lease_id: str | None = None,
    payload: JsonObject | None = None,
) -> int:
    """Insert an append-only event row."""
    cursor = conn.execute(
        """
        INSERT INTO events (
          run_id, event_type, actor_session_id, job_id, message_id,
          artifact_id, lease_id, payload_json, created_at
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        """,
        (
            run_id,
            event_type,
            actor_session_id,
            job_id,
            message_id,
            artifact_id,
            lease_id,
            json_dumps(payload or {}),
            utc_now(),
        ),
    )
    if cursor.lastrowid is None:
        raise InvalidTransitionError("event insert did not return a row id")
    return cursor.lastrowid


def row_by_id(conn: sqlite3.Connection, table: str, column: str, value: str) -> sqlite3.Row:
    """Fetch a row or raise NotFoundError."""
    cursor = conn.execute(f"SELECT * FROM {table} WHERE {column} = ?", (value,))
    row = cursor.fetchone()
    if row is None:
        raise NotFoundError(f"could not find {table} row for {column}={value!r}")
    return cast(sqlite3.Row, row)


def repo_relative_path(repo: Path, path_text: str) -> Path:
    """Resolve a repo-relative path and reject escapes."""
    return _repo_relative_path(repo, path_text, allow_state=False)


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


def is_repo_write(job: sqlite3.Row) -> bool:
    """Return whether a job can write non-review artifacts."""
    scope = json_loads(str(job["write_scope_json"]))
    return scope.get("repo_write") is True or scope.get("mode") == "repo_write"


def lane_worktree_isolation(workflow: JsonObject, lane_id: str | None) -> str:
    """Return the lane's declared worktree isolation mode.

    Defaults to ``"off"`` when the lane is missing, the field is absent, or
    the field is not a recognized string. RFC 0008 V1 only supports
    ``"off"`` and ``"per_job"``.
    """
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


def job_lane_id(job: sqlite3.Row) -> str | None:
    """Return the lane id recorded on a job row."""
    selector = json_loads(str(job["lane_selector_json"]))
    lane = selector.get("lane_id")
    return lane if isinstance(lane, str) else None


def workflow_for_run(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Return the workflow snapshot JSON associated with a run."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    return json_loads(str(snapshot["workflow_json"]))


def active_worktree_for_job(
    conn: sqlite3.Connection, *, job_id: str
) -> sqlite3.Row | None:
    """Return the active worktree row for a job, if any."""
    row = conn.execute(
        "SELECT * FROM job_worktrees WHERE job_id = ? AND state = 'active' LIMIT 1",
        (job_id,),
    ).fetchone()
    if row is None:
        return None
    return cast(sqlite3.Row, row)


def active_lease_for(
    conn: sqlite3.Connection,
    *,
    lease_id: str,
    session_id: str,
    job_id: str | None = None,
) -> sqlite3.Row:
    """Fetch an active lease and validate ownership."""
    lease = row_by_id(conn, "leases", "lease_id", lease_id)
    if lease["state"] != "active":
        raise LeaseError("lease is not active")
    if lease["owner_session_id"] != session_id:
        raise LeaseError("lease is owned by another session")
    if job_id is not None and lease["resource_id"] != job_id:
        raise LeaseError("lease does not belong to the job")
    if str(lease["expires_at"]) < utc_now():
        raise LeaseError("lease is expired")
    return lease


def expire_leases(conn: sqlite3.Connection, *, run_id: str) -> None:
    """Expire stale leases lazily during CLI mutations."""
    # Local import: supervisor.py imports from db.py, so a top-level import
    # would create a cycle. The recovery hook is small and well-isolated.
    from striatum.supervisor import mark_supervisor_lost_for_lease

    now = utc_now()
    rows = conn.execute(
        "SELECT * FROM leases WHERE run_id = ? AND state = 'active' AND expires_at < ?",
        (run_id, now),
    ).fetchall()
    for lease in rows:
        job = row_by_id(conn, "jobs", "job_id", str(lease["resource_id"]))
        message_id = job["current_message_id"]
        if is_repo_write(job):
            job_state = "stale_lease"
            message_state = "blocked"
        else:
            job_state = "queued"
            message_state = "pending"
        conn.execute(
            """
            UPDATE leases
            SET state = 'expired', released_at = ?, release_reason = 'expired'
            WHERE lease_id = ?
            """,
            (now, lease["lease_id"]),
        )
        conn.execute(
            """
            UPDATE jobs
            SET state = ?, current_lease_id = NULL
            WHERE job_id = ?
            """,
            (job_state, job["job_id"]),
        )
        if message_id is not None:
            conn.execute(
                """
                UPDATE queue_messages
                SET state = ?, current_lease_id = NULL, updated_at = ?
                WHERE message_id = ?
                """,
                (message_state, now, message_id),
            )
        insert_event(
            conn,
            run_id=run_id,
            event_type="lease.expired",
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=str(lease["lease_id"]),
            payload={"job_state": job_state, "message_state": message_state},
        )
        # If the session that just lost its lease has an attached supervisor,
        # mark the supervisor lost without auto-killing the OS process. RFC
        # 0009 keeps the same operator-inspection policy as stale leases for
        # repo-write work: state transitions are explicit, not automatic.
        mark_supervisor_lost_for_lease(
            conn,
            run_id=run_id,
            session_id=str(lease["owner_session_id"]),
            lease_id=str(lease["lease_id"]),
        )
        worktree = active_worktree_for_job(conn, job_id=str(job["job_id"]))
        if worktree is not None and str(worktree["lease_id"]) == str(lease["lease_id"]):
            conn.execute(
                """
                UPDATE job_worktrees
                SET state = 'abandoned'
                WHERE worktree_id = ?
                """,
                (str(worktree["worktree_id"]),),
            )
            insert_event(
                conn,
                run_id=run_id,
                event_type="worktree.abandoned",
                job_id=str(job["job_id"]),
                lease_id=str(lease["lease_id"]),
                payload={
                    "worktree_id": str(worktree["worktree_id"]),
                    "worktree_path": str(worktree["worktree_path"]),
                    "reason": "lease_expired",
                },
            )


def enqueue_job(conn: sqlite3.Connection, *, job_id: str) -> str:
    """Enqueue a work message for a queued job."""
    job = row_by_id(conn, "jobs", "job_id", job_id)
    if job["state"] not in ("blocked", "queued"):
        raise InvalidTransitionError("job is not enqueueable")
    now = utc_now()
    message_id = new_id("msg")
    lane_selector = json_loads(str(job["lane_selector_json"]))
    target_lane = lane_selector.get("lane_id")
    if target_lane is not None and not isinstance(target_lane, str):
        target_lane = None
    conn.execute(
        """
        INSERT INTO queue_messages (
          message_id, run_id, job_id, kind, state, priority, target_role_id,
          target_lane_id, payload_json, claim_count, max_claims, created_at, updated_at
        )
        VALUES (?, ?, ?, 'work', 'pending', 0, ?, ?, '{}', 0, ?, ?, ?)
        """,
        (
            message_id,
            job["run_id"],
            job_id,
            job["role_id"],
            target_lane,
            job["max_attempts"],
            now,
            now,
        ),
    )
    conn.execute(
        """
        UPDATE jobs
        SET state = 'queued', ready_at = ?, current_message_id = ?
        WHERE job_id = ?
        """,
        (now, message_id, job_id),
    )
    insert_event(
        conn,
        run_id=str(job["run_id"]),
        event_type="queue.message_enqueued",
        job_id=job_id,
        message_id=message_id,
        payload={"workflow_job_id": job["workflow_job_id"]},
    )
    return message_id


def maybe_enqueue_downstream(conn: sqlite3.Connection, *, completed_job_id: str) -> None:
    """Enqueue jobs whose dependencies are satisfied."""
    dependents = conn.execute(
        "SELECT job_id FROM job_dependencies WHERE depends_on_job_id = ?",
        (completed_job_id,),
    ).fetchall()
    for dependent in dependents:
        job_id = str(dependent["job_id"])
        job = row_by_id(conn, "jobs", "job_id", job_id)
        if job["state"] != "blocked":
            continue
        if dependencies_satisfied(conn, job_id=job_id):
            enqueue_job(conn, job_id=job_id)


def dependencies_satisfied(conn: sqlite3.Connection, *, job_id: str) -> bool:
    """Return whether all materialized dependency gates are satisfied."""
    dependencies = conn.execute(
        "SELECT * FROM job_dependencies WHERE job_id = ?",
        (job_id,),
    ).fetchall()
    for dependency in dependencies:
        upstream = row_by_id(conn, "jobs", "job_id", str(dependency["depends_on_job_id"]))
        try:
            gate = json_loads(str(dependency["gate_json"]))
        except (json.JSONDecodeError, InvalidTransitionError):
            return False
        if upstream["state"] != "completed":
            return False
        required = gate.get("requires_verdict")
        if required is None:
            continue
        if not isinstance(required, list) or not all(isinstance(item, str) for item in required):
            return False
        verdict = latest_verdict(conn, job_id=str(upstream["job_id"]))
        if verdict not in set(required):
            return False
    return True


def latest_verdict(conn: sqlite3.Connection, *, job_id: str) -> str | None:
    """Return the most recent verdict string for a review job."""
    row = conn.execute(
        "SELECT verdict FROM verdicts WHERE job_id = ? ORDER BY created_at DESC, verdict_id DESC LIMIT 1",
        (job_id,),
    ).fetchone()
    return str(row["verdict"]) if row is not None else None


def maybe_complete_run(conn: sqlite3.Connection, *, run_id: str) -> None:
    """Mark a run completed, failed, or canceled when terminal job states require it.

    On any terminal transition, calls :func:`close_remaining_sessions`
    so any still-active sessions are closed in the same transaction
    (RFC 0011). Auto-close skips sessions with active leases; the
    existing recovery flow remains the path for those.

    Run state is determined by the worst-case job state:

    * any ``failed`` job → ``failed`` (preserves the previous behavior)
    * else any non-terminal job remaining → return without transition
    * else any ``completed`` job → ``completed`` (partial success counts)
    * else (every job is ``canceled``/``skipped``) → ``canceled``

    The third branch resolves the D055 follow-up: ``recovery cancel-job
    --cascade`` over an entire run no longer produces a ``completed``
    run state. Auto-close uses ``source="run_canceled"`` for that path.
    """
    failed = conn.execute(
        "SELECT 1 FROM jobs WHERE run_id = ? AND state = 'failed' LIMIT 1",
        (run_id,),
    ).fetchone()
    run = row_by_id(conn, "runs", "run_id", run_id)
    if failed is not None and run["state"] == "running":
        now = utc_now()
        conn.execute(
            "UPDATE runs SET state = 'failed', completed_at = ?, stop_reason = ? WHERE run_id = ?",
            (now, "job_failed", run_id),
        )
        insert_event(conn, run_id=run_id, event_type="run.failed", payload={"reason": "job_failed"})
        close_remaining_sessions(
            conn, run_id=run_id, source="run_failed", reason="run_failed"
        )
        return
    remaining = conn.execute(
        """
        SELECT 1 FROM jobs
        WHERE run_id = ? AND state NOT IN ('completed','skipped','canceled')
        LIMIT 1
        """,
        (run_id,),
    ).fetchone()
    if remaining is not None:
        return
    if run["state"] != "running":
        return
    has_completed = conn.execute(
        "SELECT 1 FROM jobs WHERE run_id = ? AND state = 'completed' LIMIT 1",
        (run_id,),
    ).fetchone()
    now = utc_now()
    if has_completed is None:
        # Every job is canceled or skipped — no work was completed; the
        # run did not finish, it was canceled. Source enum value matches
        # RFC 0011's reserved ``"run_canceled"`` for auto-close.
        conn.execute(
            "UPDATE runs SET state = 'canceled', completed_at = ?, stop_reason = ? WHERE run_id = ?",
            (now, "all_jobs_canceled", run_id),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="run.canceled",
            payload={"reason": "all_jobs_canceled"},
        )
        close_remaining_sessions(
            conn, run_id=run_id, source="run_canceled", reason="run_canceled"
        )
        return
    conn.execute(
        "UPDATE runs SET state = 'completed', completed_at = ? WHERE run_id = ?",
        (now, run_id),
    )
    insert_event(conn, run_id=run_id, event_type="run.completed")
    close_remaining_sessions(
        conn, run_id=run_id, source="run_completed", reason="run_completed"
    )


def pause_run(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    reason: str | None = None,
) -> JsonObject:
    """RFC 0024 V4: suspend claim-next on the run without changing state.

    Sets ``runs.paused_at`` + ``runs.paused_reason`` if not already set.
    Idempotent: re-pausing returns the current state without re-emitting.
    Refuses terminal states (completed/failed/canceled).

    Caller is responsible for being inside a transaction.
    """
    from striatum.errors import InvalidTransitionError, NotFoundError

    run = conn.execute(
        "SELECT * FROM runs WHERE run_id = ?", (run_id,)
    ).fetchone()
    if run is None:
        raise NotFoundError(f"run not found: {run_id}")
    state = str(run["state"])
    if state in ("completed", "failed", "canceled"):
        raise InvalidTransitionError(
            f"run is in terminal state {state!r} and cannot be paused"
        )
    if run["paused_at"] is not None:
        return {"run_id": run_id, "state": state, "paused_at": run["paused_at"], "status": "already_paused"}
    now = utc_now()
    paused_reason = reason or "operator_paused"
    conn.execute(
        "UPDATE runs SET paused_at = ?, paused_reason = ? WHERE run_id = ? AND paused_at IS NULL",
        (now, paused_reason, run_id),
    )
    insert_event(
        conn,
        run_id=run_id,
        event_type="run.paused",
        payload={"reason": paused_reason},
    )
    return {"run_id": run_id, "state": state, "paused_at": now, "status": "paused"}


def resume_run(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """RFC 0024 V4: clear the paused flag so claim-next resumes work.

    Idempotent: resuming a non-paused run returns current state without
    re-emitting. Refuses terminal states (use ``retry_job`` to revive).

    Caller is responsible for being inside a transaction.
    """
    from striatum.errors import InvalidTransitionError, NotFoundError

    run = conn.execute(
        "SELECT * FROM runs WHERE run_id = ?", (run_id,)
    ).fetchone()
    if run is None:
        raise NotFoundError(f"run not found: {run_id}")
    state = str(run["state"])
    if state in ("completed", "failed", "canceled"):
        raise InvalidTransitionError(
            f"run is in terminal state {state!r}; use retry_job to revive"
        )
    if run["paused_at"] is None:
        return {"run_id": run_id, "state": state, "paused_at": None, "status": "not_paused"}
    conn.execute(
        "UPDATE runs SET paused_at = NULL, paused_reason = NULL WHERE run_id = ?",
        (run_id,),
    )
    insert_event(
        conn,
        run_id=run_id,
        event_type="run.resumed",
        payload={},
    )
    return {"run_id": run_id, "state": state, "paused_at": None, "status": "resumed"}


def retry_job(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    job_id: str,
) -> JsonObject:
    """RFC 0024 V4: reset a failed/canceled/blocked job and re-enqueue.

    If the run is in a terminal failed/canceled state, transitions it
    back to ``running`` and emits ``run.revived`` (per design-review F1
    option C: explicit + loud).

    Caller is responsible for being inside a transaction.
    """
    from striatum.errors import InvalidTransitionError, NotFoundError

    run = conn.execute(
        "SELECT * FROM runs WHERE run_id = ?", (run_id,)
    ).fetchone()
    if run is None:
        raise NotFoundError(f"run not found: {run_id}")
    if str(run["state"]) == "completed":
        raise InvalidTransitionError(
            "run is completed; retry would revive a finished run"
        )
    job = conn.execute(
        "SELECT * FROM jobs WHERE job_id = ?", (job_id,)
    ).fetchone()
    if job is None:
        raise NotFoundError(f"job not found: {job_id}")
    if str(job["run_id"]) != run_id:
        raise InvalidTransitionError("job does not belong to the requested run")
    previous_state = str(job["state"])
    if previous_state not in ("failed", "canceled", "blocked"):
        raise InvalidTransitionError(
            f"job state {previous_state!r} is not retriable"
            " (must be failed, canceled, or blocked)"
        )
    # 1. Reset job row. Clear current_message_id too so deleting prior
    # queue_messages row doesn't break the FK.
    conn.execute(
        """
        UPDATE jobs
        SET state = 'queued',
            started_at = NULL,
            completed_at = NULL,
            current_lease_id = NULL,
            current_message_id = NULL,
            attempt = attempt + 1
        WHERE job_id = ?
        """,
        (job_id,),
    )
    # 2. Clear any unresolved blockers tied to this job.
    conn.execute(
        "DELETE FROM blockers WHERE job_id = ? AND resolved_at IS NULL",
        (job_id,),
    )
    # 3. Mark prior queue_messages rows as canceled. The partial unique
    # index `uq_active_work_message_per_job` only covers
    # state IN ('pending','claimed','acked'); marking them canceled
    # lets the new enqueue insert a fresh row without FK collisions.
    conn.execute(
        """
        UPDATE queue_messages
        SET state = 'canceled', updated_at = ?
        WHERE job_id = ? AND state IN ('pending','claimed','acked')
        """,
        (utc_now(), job_id),
    )
    # 4. Re-enqueue.
    enqueue_job(conn, job_id=job_id)
    # 4. Revive the run if needed (F1 option C).
    run_revived = False
    if str(run["state"]) in ("failed", "canceled"):
        conn.execute(
            """
            UPDATE runs
            SET state = 'running', completed_at = NULL, stop_reason = NULL
            WHERE run_id = ?
            """,
            (run_id,),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="run.revived",
            payload={"trigger_job_id": job_id, "previous_run_state": str(run["state"])},
        )
        run_revived = True
    insert_event(
        conn,
        run_id=run_id,
        event_type="job.retried",
        job_id=job_id,
        payload={"previous_state": previous_state, "attempt": int(job["attempt"]) + 1},
    )
    return {
        "run_id": run_id,
        "job_id": job_id,
        "previous_state": previous_state,
        "new_state": "queued",
        "run_revived": run_revived,
    }


def cancel_run(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    reason: str | None = None,
) -> JsonObject:
    """RFC 0024 V3: top-down cancel.

    Releases active leases, marks in-flight jobs canceled, transitions
    the run to ``canceled``, emits ``run.canceled`` event, and closes
    remaining sessions. Idempotent: re-cancelling an already-``canceled``
    run is a no-op (returns the current state).

    Allowed source states: ``prepared``, ``needs_branch_confirmation``,
    ``ready``, ``running``. Already-``canceled`` → no-op.
    Other terminal states (``completed``, ``failed``) →
    :class:`InvalidTransitionError`.

    Caller is responsible for being inside a transaction.
    """
    from striatum.errors import InvalidTransitionError, NotFoundError

    run = conn.execute(
        "SELECT * FROM runs WHERE run_id = ?", (run_id,)
    ).fetchone()
    if run is None:
        raise NotFoundError(f"run not found: {run_id}")
    state = str(run["state"])
    if state == "canceled":
        return {"run_id": run_id, "state": "canceled", "status": "already_canceled"}
    if state in ("completed", "failed"):
        raise InvalidTransitionError(
            f"run is in terminal state {state!r} and cannot be canceled"
        )
    now = utc_now()
    stop_reason = reason or "operator_canceled"
    # 1) Mark in-flight jobs canceled. Include 'claimed' (between
    #    claim-next and ack) so a racing ack hits the canceled state.
    conn.execute(
        """
        UPDATE jobs
        SET state = 'canceled', completed_at = ?
        WHERE run_id = ?
          AND state IN ('queued', 'running', 'blocked', 'ready', 'claimed')
        """,
        (now, run_id),
    )
    # 2) Release active leases held by this run's sessions.
    conn.execute(
        """
        UPDATE leases
        SET state = 'released', released_at = ?, release_reason = 'run_canceled'
        WHERE owner_session_id IN (
            SELECT session_id FROM sessions WHERE run_id = ?
        )
        AND state = 'active'
        """,
        (now, run_id),
    )
    # 3) Transition run to canceled.
    conn.execute(
        """
        UPDATE runs
        SET state = 'canceled', completed_at = ?, stop_reason = ?
        WHERE run_id = ?
        """,
        (now, stop_reason, run_id),
    )
    insert_event(
        conn,
        run_id=run_id,
        event_type="run.canceled",
        payload={"reason": stop_reason},
    )
    close_remaining_sessions(
        conn, run_id=run_id, source="run_canceled", reason="run_canceled"
    )
    return {"run_id": run_id, "state": "canceled", "status": "canceled"}


def close_remaining_sessions(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    source: str,
    reason: str,
) -> list[JsonObject]:
    """Auto-close every still-active session on ``run_id`` (RFC 0011).

    Skips any session that holds an active lease — the run reaching a
    terminal state with held leases is itself anomalous and the
    existing expire-lease/recovery flow handles that surface;
    auto-close should not paper over it. Returns one summary dict per
    closed session.

    Caller is responsible for being inside a transaction (every
    existing call site is). The function does not open or close
    transactions.
    """
    now = utc_now()
    rows = conn.execute(
        """
        SELECT s.session_id, s.role_id, s.lane_id
        FROM sessions s
        LEFT JOIN leases l
          ON l.owner_session_id = s.session_id AND l.state = 'active'
        WHERE s.run_id = ? AND s.state = 'active' AND l.lease_id IS NULL
        ORDER BY s.registered_at
        """,
        (run_id,),
    ).fetchall()
    closed: list[JsonObject] = []
    for row in rows:
        sid = str(row["session_id"])
        conn.execute(
            """
            UPDATE sessions
            SET state = 'closed', closed_at = ?, close_reason = ?
            WHERE session_id = ?
            """,
            (now, reason, sid),
        )
        insert_event(
            conn,
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
        closed.append(
            {
                "session_id": sid,
                "role_id": str(row["role_id"]),
                "lane_id": str(row["lane_id"]),
                "closed_at": now,
                "close_reason": reason,
                "source": source,
            }
        )
    return closed


def claim_next(conn: sqlite3.Connection, *, repo: Path, session_id: str, lease_seconds: int) -> JsonObject:
    """Claim the next eligible work item for a registered session."""
    with transaction(conn):
        session = row_by_id(conn, "sessions", "session_id", session_id)
        run = row_by_id(conn, "runs", "run_id", str(session["run_id"]))
        expire_leases(conn, run_id=str(run["run_id"]))
        if run["state"] in ("needs_branch_confirmation", "ready"):
            raise BranchConfirmationError("branch confirmation and run start are required before claims")
        if run["state"] != "running":
            return {"status": "no_work"}
        # RFC 0024 V4: pause gate. Active leases keep ticking;
        # expire_leases above handles paused-with-stale-leases.
        try:
            paused_at = run["paused_at"]
        except (IndexError, KeyError):
            paused_at = None
        if paused_at is not None:
            return {"status": "no_work", "paused": True}
        chosen = conn.execute(
            """
            SELECT qm.*
            FROM queue_messages qm
            JOIN jobs j ON j.job_id = qm.job_id
            WHERE qm.kind = 'work'
              AND qm.state = 'pending'
              AND qm.target_role_id = ?
              AND (qm.target_lane_id IS NULL OR qm.target_lane_id = ?)
              AND (
                j.fresh_session_required = 0
                OR NOT EXISTS (
                  SELECT 1 FROM work_packets wp
                  WHERE wp.run_id = qm.run_id
                    AND wp.session_id = ?
                )
              )
              AND qm.run_id = ?
            ORDER BY qm.priority DESC, qm.created_at ASC
            LIMIT 1
            """,
            (session["role_id"], session["lane_id"], session_id, run["run_id"]),
        ).fetchone()
        if chosen is None:
            return {"status": "no_work"}
        job = row_by_id(conn, "jobs", "job_id", str(chosen["job_id"]))
        now = utc_now()
        lease_id = new_id("lease")
        packet_id = new_id("wp")
        expires_at = (
            datetime.now(UTC) + timedelta(seconds=lease_seconds)
        ).replace(microsecond=0).isoformat().replace("+00:00", "Z")
        conn.execute(
            """
            INSERT INTO leases (
              lease_id, run_id, resource_type, resource_id, owner_session_id,
              state, acquired_at, expires_at, last_heartbeat_at
            )
            VALUES (?, ?, 'job', ?, ?, 'active', ?, ?, ?)
            """,
            (lease_id, run["run_id"], job["job_id"], session_id, now, expires_at, now),
        )
        conn.execute(
            """
            UPDATE queue_messages
            SET state = 'claimed', claimed_at = ?, updated_at = ?,
                current_lease_id = ?, claim_count = claim_count + 1
            WHERE message_id = ?
            """,
            (now, now, lease_id, chosen["message_id"]),
        )
        conn.execute(
            """
            UPDATE jobs
            SET state = 'claimed', current_lease_id = ?, started_at = ?
            WHERE job_id = ?
            """,
            (lease_id, now, job["job_id"]),
        )
        packet = build_packet(
            conn=conn,
            repo=repo,
            run=run,
            session=session,
            job=job,
            message_id=str(chosen["message_id"]),
            lease_id=lease_id,
            lease_expires_at=expires_at,
            packet_id=packet_id,
        )
        packet_json = json_dumps(packet)
        conn.execute(
            """
            INSERT INTO work_packets (
              packet_id, run_id, job_id, message_id, lease_id, session_id,
              packet_json, packet_sha256, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                packet_id,
                run["run_id"],
                job["job_id"],
                chosen["message_id"],
                lease_id,
                session_id,
                packet_json,
                sha256_bytes(packet_json.encode("utf-8")),
                now,
            ),
        )
        insert_event(
            conn,
            run_id=str(run["run_id"]),
            event_type="queue.claimed",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=str(chosen["message_id"]),
            lease_id=lease_id,
        )
        # Auto-route the packet through any attached supervisor for this
        # session so the same OS process keeps receiving packets across
        # turns. Deferred import avoids a circular dependency between
        # ``striatum.db`` and ``striatum.supervisor``.
        from striatum.supervisor import deliver_packet_to_attached_supervisor

        delivery = deliver_packet_to_attached_supervisor(
            conn,
            run_id=str(run["run_id"]),
            session_id=session_id,
            packet_id=packet_id,
            packet_json=packet_json,
        )
        response: JsonObject = {"status": "claimed", "packet": packet}
        if delivery is not None:
            response["supervisor_delivery"] = delivery
        return response


def build_packet(
    *,
    conn: sqlite3.Connection,
    repo: Path,
    run: sqlite3.Row,
    session: sqlite3.Row,
    job: sqlite3.Row,
    message_id: str,
    lease_id: str,
    lease_expires_at: str,
    packet_id: str,
) -> JsonObject:
    """Build a structured work packet from stored workflow state."""
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    workflow = json_loads(str(snapshot["workflow_json"]))
    roles = cast(JsonObject, workflow.get("roles", {}))
    role_def = roles.get(str(job["role_id"]), {})
    context_docs = workflow.get("context_docs", [])
    write_scope = json_loads(str(job["write_scope_json"]))
    expected_artifacts = json.loads(str(job["expected_artifacts_json"]))
    lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
    lane_id = lane if isinstance(lane, str) else None
    lanes = cast(JsonObject, workflow.get("lanes", {}))
    lane_config = lanes.get(lane_id, {}) if lane_id is not None else {}
    adapter_constraints = build_adapter_constraints(lane_config if isinstance(lane_config, dict) else {})
    worktree_isolation = lane_worktree_isolation(workflow, lane_id)
    worktree_required = worktree_isolation == "per_job" and is_repo_write(job)
    author = artifact_author_identity(
        workflow,
        role_id=str(job["role_id"]),
        lane_id=lane_id,
        workflow_job_id=str(job["workflow_job_id"]),
        ordinal=int(session["ordinal"]),
    )
    author_line = author["line"]
    if author_line is None:
        raise InvalidTransitionError("session author line could not be derived")
    review_policy = _build_review_policy(workflow, workflow_job_id=str(job["workflow_job_id"]))
    packet: JsonObject = {
        "packet_version": "striatum.work-packet.v1",
        "packet_id": packet_id,
        "run": {
            "run_id": run["run_id"],
            "workflow_id": workflow.get("workflow_id"),
            "repo_root": str(repo),
            "branch": {"name": run["branch_name"], "confirmed": run["branch_confirmed_at"] is not None},
        },
        "session": {
            "session_id": session["session_id"],
            "slug": session["slug"],
            "role_id": session["role_id"],
            "lane_id": session["lane_id"],
            "capabilities": json.loads(str(session["capabilities_json"])),
        },
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
            "objective": json_loads(str(job["capability_requirements_json"])).get("objective"),
            "fresh_session_required": job["fresh_session_required"] == 1,
        },
        "role": {
            "role_id": job["role_id"],
            "definition_path": role_def.get("definition_path") if isinstance(role_def, dict) else None,
            "inline_summary": role_def.get("summary") if isinstance(role_def, dict) else None,
        },
        "context": {"docs": context_docs, "content_mode": "references"},
        "task_prompt": json_loads(str(job["capability_requirements_json"])).get("task_prompt", {}),
        "inputs": json_loads(str(job["capability_requirements_json"])).get("inputs", []),
        "write_scope": write_scope,
        "adapter_constraints": adapter_constraints,
        "expected_artifacts": expected_artifacts_with_author(expected_artifacts, author_line=author_line),
        "worktree_isolation": worktree_isolation,
        "worktree_required": worktree_required,
        "commands": _build_packet_commands(
            session_id=str(session["session_id"]),
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
            worktree_required=worktree_required,
        ),
        "artifact_policy": {"publish_transcripts": False, "curated_artifacts_only": True},
    }
    if review_policy is not None:
        packet["review_policy"] = review_policy
    profile_view = _harness_profile_view(workflow, lane_id=lane_id)
    if profile_view is not None:
        packet["harness_profile"] = profile_view
    return packet


def _harness_profile_view(
    workflow: JsonObject, *, lane_id: str | None
) -> JsonObject | None:
    """Project a lane's RFC 0010 harness profile into the work packet.

    Returns the profile body keyed by ``profile_id`` plus all declared fields
    (passthrough projection). Returns ``None`` when the lane has no profile
    reference, the reference is unrecognised, or either side is malformed.
    """
    if lane_id is None:
        return None
    lanes = workflow.get("lanes")
    if not isinstance(lanes, dict):
        return None
    lane_value = lanes.get(lane_id)
    if not isinstance(lane_value, dict):
        return None
    profile_id = lane_value.get("harness_profile_id")
    if not isinstance(profile_id, str) or not profile_id:
        return None
    profiles = workflow.get("harness_profiles")
    if not isinstance(profiles, dict):
        return None
    body = profiles.get(profile_id)
    if not isinstance(body, dict):
        return None
    view: JsonObject = {"profile_id": profile_id}
    view.update(body)
    return view


_REVIEWER_ACCESS_INSTRUCTIONS = {
    "document_only": (
        "Read only the target documents listed in inputs. Do not consult other "
        "artifacts, ledgers, reports, or repository contents beyond inputs."
    ),
    "artifact_augmented": (
        "You may read the target documents AND the supporting artifacts/reports/"
        "ledgers listed in inputs. Do not inspect other repository contents."
    ),
    "repo_level": (
        "You may inspect the repository within the job's declared write_scope."
        "allowed_paths/forbidden_paths. Stay within that scope."
    ),
}

_REVIEWER_CONTEXT_INSTRUCTIONS = {
    "fresh": (
        " This is a fresh-context review. Do not rely on prior thread state from "
        "earlier rounds."
    ),
    "cross_round": (
        " You may retain prior context to verify whether previously raised issues "
        "were resolved."
    ),
}


def _build_review_policy(
    workflow: JsonObject, *, workflow_job_id: str
) -> JsonObject | None:
    """Return the RFC 0002 / RFC 0018 review-policy block for a review job.

    The block is added to a work packet only when the workflow declares
    ``reviewer_access_scope``, ``reviewer_context_policy``, or
    ``review_posture`` on the matching review job. Access and context
    default through ``document_only`` and ``cross_round`` so the rendered
    ``instruction`` text always describes the intended reviewer behavior.
    Posture defaults to ``"neutral"`` when omitted; the ``posture`` key is
    included in the block only when explicitly declared so that a workflow
    declaring only access/context is byte-identical to today.
    """
    from striatum.workflow import ALLOWED_POSTURES, POSTURE_INSTRUCTIONS

    jobs = workflow.get("jobs", [])
    if not isinstance(jobs, list):
        return None
    job_def: JsonObject | None = None
    for entry in jobs:
        if not isinstance(entry, dict):
            continue
        if entry.get("id") == workflow_job_id and entry.get("type") == "review":
            job_def = cast(JsonObject, entry)
            break
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
    if not isinstance(access, str) or access not in _REVIEWER_ACCESS_INSTRUCTIONS:
        return None
    if not isinstance(context, str) or context not in _REVIEWER_CONTEXT_INSTRUCTIONS:
        return None
    if not isinstance(posture, str):
        return None
    posture_sentence = (
        POSTURE_INSTRUCTIONS[posture] if posture in ALLOWED_POSTURES else ""
    )
    instruction = (
        _REVIEWER_ACCESS_INSTRUCTIONS[access]
        + _REVIEWER_CONTEXT_INSTRUCTIONS[context]
        + posture_sentence
    )
    block: JsonObject = {
        "access_scope": access,
        "context_policy": context,
        "instruction": instruction,
    }
    if has_posture:
        block["posture"] = posture
    return block


def _build_packet_commands(
    *,
    session_id: str,
    job_id: str,
    message_id: str,
    lease_id: str,
    worktree_required: bool,
) -> JsonObject:
    """Return the command map embedded in a work packet."""
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
            f"striatum worktree create --session-id {session_id} "
            f"--job-id {job_id} --lease-id {lease_id}"
        )
    return commands


def expected_artifacts_with_author(expected_artifacts: object, *, author_line: str) -> list[object]:
    """Attach the exact artifact author line to work-packet artifact specs."""
    if not isinstance(expected_artifacts, list):
        return []
    enriched: list[object] = []
    for artifact in expected_artifacts:
        if isinstance(artifact, dict):
            artifact_copy = dict(artifact)
            artifact_copy["author_line"] = author_line
            enriched.append(artifact_copy)
        else:
            enriched.append(artifact)
    return enriched


def build_adapter_constraints(lane_config: JsonObject) -> JsonObject:
    """Return requested lane constraints and V1 enforcement status."""
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
        required_level = required.get(key)
        required_text = required_level if isinstance(required_level, str) else None
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


def adapter_constraint_enforcement(adapter: object, *, constraint: str, requested: str) -> str:
    """Return the enforcement level an adapter can provide for a requested constraint.

    `advisory_strict` is a best-effort enforcement that the runner actively
    sets up but cannot fully prevent; e.g. for `network=forbidden` the process
    adapter scrubs proxy env vars and sets a sentinel `STRIATUM_NETWORK_POLICY`,
    but a child process that ignores the policy can still open sockets.
    """
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


def complete_job(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    summary: str | None,
) -> JsonObject:
    """Complete a running job after required artifacts are present."""
    with transaction(conn):
        job = row_by_id(conn, "jobs", "job_id", job_id)
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        if job["state"] != "running":
            raise InvalidTransitionError("job must be running before completion")
        verify_required_artifacts(conn, job_id=job_id)
        now = utc_now()
        message_id = job["current_message_id"]
        conn.execute(
            "UPDATE jobs SET state = 'completed', completed_at = ?, current_lease_id = NULL WHERE job_id = ?",
            (now, job_id),
        )
        if message_id is not None:
            conn.execute(
                """
                UPDATE queue_messages
                SET state = 'completed', completed_at = ?, updated_at = ?,
                    current_lease_id = NULL
                WHERE message_id = ?
                """,
                (now, now, message_id),
            )
        conn.execute(
            """
            UPDATE leases
            SET state = 'released', released_at = ?, release_reason = 'completed'
            WHERE lease_id = ?
            """,
            (now, lease_id),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="job.completed",
            actor_session_id=session_id,
            job_id=job_id,
            message_id=message_id,
            lease_id=lease_id,
            payload={"summary": summary},
        )
        maybe_enqueue_downstream(conn, completed_job_id=job_id)
        maybe_complete_run(conn, run_id=str(job["run_id"]))
        return {"status": "completed", "job_id": job_id}


def verify_required_artifacts(conn: sqlite3.Connection, *, job_id: str) -> None:
    """Ensure all required artifacts for a job were published."""
    job = row_by_id(conn, "jobs", "job_id", job_id)
    expected = json.loads(str(job["expected_artifacts_json"]))
    if not isinstance(expected, list):
        raise InvalidTransitionError("expected artifacts must be a list")
    for item in expected:
        if not isinstance(item, dict) or item.get("required") is not True:
            continue
        logical_name = item.get("logical_name")
        kind = item.get("kind")
        path = item.get("path")
        found = conn.execute(
            """
            SELECT 1 FROM artifacts
            WHERE job_id = ? AND logical_name = ? AND artifact_kind = ? AND repo_path = ?
            LIMIT 1
            """,
            (job_id, logical_name, kind, path),
        ).fetchone()
        if found is None:
            raise InvalidTransitionError(
                "required artifact is missing: "
                f"logical_name={logical_name!r}, kind={kind!r}, path={path!r}"
            )


def _resolve_review_posture(
    conn: sqlite3.Connection, *, job: sqlite3.Row
) -> str:
    """RFC 0018 step 3 (V1.5): resolve a review job's posture from its
    workflow snapshot.

    Returns the literal value of the job's ``review_posture`` field if
    declared and valid; otherwise ``"neutral"``. Posture lives only in
    the immutable workflow snapshot, not on the live workflow file or
    on any per-job table column, so this lookup is the source of truth.
    """
    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    workflow = json_loads(str(snapshot["workflow_json"]))
    if not isinstance(workflow, dict):
        return "neutral"
    jobs = workflow.get("jobs", [])
    if not isinstance(jobs, list):
        return "neutral"
    workflow_job_id = str(job["workflow_job_id"])
    for entry in jobs:
        if not isinstance(entry, dict):
            continue
        if entry.get("id") != workflow_job_id:
            continue
        if entry.get("type") != "review":
            return "neutral"
        declared = entry.get("review_posture")
        if isinstance(declared, str) and declared:
            return declared
        return "neutral"
    return "neutral"


def record_review_verdict(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    verdict: str,
    findings_artifact_id: str | None,
    rationale: str | None,
) -> JsonObject:
    """Record a review verdict and apply its workflow transition."""
    with transaction(conn):
        job = row_by_id(conn, "jobs", "job_id", job_id)
        if job["job_type"] != "review":
            raise InvalidTransitionError("verdict is valid only for review jobs")
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        if job["state"] != "running":
            raise InvalidTransitionError("review job must be running before verdict")
        verify_required_artifacts(conn, job_id=job_id)
        verdict_id = new_id("verdict")
        now = utc_now()
        posture = _resolve_review_posture(conn, job=job)
        conn.execute(
            """
            INSERT INTO verdicts (
              verdict_id, run_id, job_id, session_id, verdict, rationale,
              findings_artifact_id, created_at, posture
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (verdict_id, job["run_id"], job_id, session_id, verdict,
             rationale, findings_artifact_id, now, posture),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="verdict.recorded",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={"verdict": verdict, "posture": posture},
        )
        if verdict in ("accept", "accept_with_findings"):
            _complete_review_job(conn, job=job, session_id=session_id, lease_id=lease_id, summary=verdict)
            maybe_enqueue_downstream(conn, completed_job_id=job_id)
            maybe_complete_run(conn, run_id=str(job["run_id"]))
            return {"status": "completed", "job_id": job_id, "verdict": verdict, "verdict_id": verdict_id}
        if verdict == "needs_revision":
            result = request_revision_for_cycle(conn, review_job=job, session_id=session_id, lease_id=lease_id)
            result["verdict_id"] = verdict_id
            return result
        if verdict == "reject":
            _fail_review_job(conn, job=job, session_id=session_id, lease_id=lease_id)
            maybe_complete_run(conn, run_id=str(job["run_id"]))
            return {"status": "failed", "job_id": job_id, "verdict": verdict, "verdict_id": verdict_id}
        raise InvalidTransitionError(f"unknown verdict {verdict!r}")


def request_revision_for_cycle(
    conn: sqlite3.Connection,
    *,
    review_job: sqlite3.Row,
    session_id: str,
    lease_id: str,
) -> JsonObject:
    """Route a needs_revision verdict through a declared bounded cycle."""
    workflow = _workflow_for_run(conn, run_id=str(review_job["run_id"]))
    cycle = _matching_revision_cycle(workflow, workflow_job_id=str(review_job["workflow_job_id"]))
    if cycle is None:
        policy = workflow.get("review_revision_policy", {})
        description = "needs_revision verdict has no matching workflow cycle"
        if isinstance(policy, dict) and policy.get("root_review_needs_revision") == "human_checkpoint":
            configured = policy.get("description")
            description = (
                configured
                if isinstance(configured, str) and configured != ""
                else "needs_revision routed to configured human checkpoint"
            )
        blocker_id = _open_human_checkpoint(
            conn,
            job=review_job,
            session_id=session_id,
            lease_id=lease_id,
            description=description,
        )
        return {"status": "waiting_human", "job_id": review_job["job_id"], "verdict": "needs_revision", "blocker_id": blocker_id}
    target_workflow_job_id = str(cycle["to"])
    max_iterations = int(cycle["max_iterations"])
    completed_attempts = conn.execute(
        "SELECT COUNT(*) AS count FROM jobs WHERE run_id = ? AND workflow_job_id = ? AND attempt > 1",
        (review_job["run_id"], target_workflow_job_id),
    ).fetchone()
    if int(completed_attempts["count"]) >= max_iterations:
        blocker_id = _open_human_checkpoint(
            conn,
            job=review_job,
            session_id=session_id,
            lease_id=lease_id,
            description="needs_revision cycle exhausted max_iterations",
        )
        return {"status": "waiting_human", "job_id": review_job["job_id"], "verdict": "needs_revision", "blocker_id": blocker_id}

    attempt = int(review_job["attempt"]) + 1
    target_job = _latest_job_for_workflow_id(
        conn,
        run_id=str(review_job["run_id"]),
        workflow_job_id=target_workflow_job_id,
    )
    next_target_id = _clone_job_attempt(conn, source=target_job, attempt=attempt)
    next_review_id = _clone_job_attempt(conn, source=review_job, attempt=attempt)
    conn.execute(
        """
        INSERT INTO job_dependencies(job_id, depends_on_job_id, gate_json)
        VALUES (?, ?, ?)
        """,
        (
            next_review_id,
            next_target_id,
            json_dumps({"on": "completed", "from": target_workflow_job_id, "to": review_job["workflow_job_id"]}),
        ),
    )
    # Re-route any downstream dependencies from the original review attempt to
    # the new review attempt so downstream jobs gate on the latest verdict.
    conn.execute(
        """
        UPDATE job_dependencies SET depends_on_job_id = ?
        WHERE depends_on_job_id = ?
        """,
        (next_review_id, review_job["job_id"]),
    )
    _complete_review_job(conn, job=review_job, session_id=session_id, lease_id=lease_id, summary="needs_revision")
    enqueue_job(conn, job_id=next_target_id)
    insert_event(
        conn,
        run_id=str(review_job["run_id"]),
        event_type="revision.requested",
        actor_session_id=session_id,
        job_id=str(review_job["job_id"]),
        lease_id=lease_id,
        payload={"next_job_id": next_target_id, "next_review_job_id": next_review_id, "attempt": attempt},
    )
    return {
        "status": "revision_requested",
        "job_id": review_job["job_id"],
        "verdict": "needs_revision",
        "next_job_id": next_target_id,
    }


def _complete_review_job(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    lease_id: str,
    summary: str,
) -> None:
    """Complete a review job after verdict-specific handling chooses that path."""
    now = utc_now()
    message_id = job["current_message_id"]
    conn.execute(
        "UPDATE jobs SET state = 'completed', completed_at = ?, current_lease_id = NULL WHERE job_id = ?",
        (now, job["job_id"]),
    )
    if message_id is not None:
        conn.execute(
            """
            UPDATE queue_messages
            SET state = 'completed', completed_at = ?, updated_at = ?, current_lease_id = NULL
            WHERE message_id = ?
            """,
            (now, now, message_id),
        )
    conn.execute(
        "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'verdict' WHERE lease_id = ?",
        (now, lease_id),
    )
    insert_event(
        conn,
        run_id=str(job["run_id"]),
        event_type="job.completed",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        message_id=message_id,
        lease_id=lease_id,
        payload={"summary": summary},
    )


def _fail_review_job(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    lease_id: str,
) -> None:
    """Fail a review job after a reject verdict."""
    now = utc_now()
    message_id = job["current_message_id"]
    conn.execute(
        "UPDATE jobs SET state = 'failed', completed_at = ?, current_lease_id = NULL WHERE job_id = ?",
        (now, job["job_id"]),
    )
    if message_id is not None:
        conn.execute(
            """
            UPDATE queue_messages
            SET state = 'completed', completed_at = ?, updated_at = ?, current_lease_id = NULL
            WHERE message_id = ?
            """,
            (now, now, message_id),
        )
    conn.execute(
        "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'reject' WHERE lease_id = ?",
        (now, lease_id),
    )
    insert_event(
        conn,
        run_id=str(job["run_id"]),
        event_type="job.failed",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        message_id=message_id,
        lease_id=lease_id,
        payload={"reason": "reject"},
    )


def _open_human_checkpoint(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    lease_id: str,
    description: str,
) -> str:
    """Open a human checkpoint and move the review job to waiting_human."""
    now = utc_now()
    blocker_id = new_id("blk")
    conn.execute(
        """
        INSERT INTO blockers (
          blocker_id, run_id, job_id, session_id, severity, blocker_kind,
          description, state, created_at
        )
        VALUES (?, ?, ?, ?, 'human_checkpoint', 'revision_routing', ?, 'open', ?)
        """,
        (blocker_id, job["run_id"], job["job_id"], session_id, description, now),
    )
    conn.execute(
        "UPDATE jobs SET state = 'waiting_human', current_lease_id = NULL WHERE job_id = ?",
        (job["job_id"],),
    )
    if job["current_message_id"] is not None:
        conn.execute(
            """
            UPDATE queue_messages
            SET state = 'blocked', current_lease_id = NULL, updated_at = ?
            WHERE message_id = ?
            """,
            (now, job["current_message_id"]),
        )
    conn.execute(
        "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'needs_revision' WHERE lease_id = ?",
        (now, lease_id),
    )
    insert_event(
        conn,
        run_id=str(job["run_id"]),
        event_type="human_checkpoint.opened",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        lease_id=lease_id,
        payload={"blocker_id": blocker_id, "description": description},
    )
    return blocker_id


def _workflow_for_run(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Return the workflow snapshot JSON for a run."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    snapshot = row_by_id(conn, "workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"]))
    return json_loads(str(snapshot["workflow_json"]))


def _matching_revision_cycle(workflow: JsonObject, *, workflow_job_id: str) -> JsonObject | None:
    """Find the declared needs_revision cycle for a review workflow job."""
    cycles = workflow.get("cycles", [])
    if not isinstance(cycles, list):
        return None
    for cycle in cycles:
        if not isinstance(cycle, dict):
            continue
        if cycle.get("from") == workflow_job_id and cycle.get("on_verdict") == "needs_revision":
            return cast(JsonObject, cycle)
    return None


def _latest_job_for_workflow_id(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    workflow_job_id: str,
) -> sqlite3.Row:
    """Return the latest attempt for a workflow job id."""
    row = conn.execute(
        """
        SELECT * FROM jobs
        WHERE run_id = ? AND workflow_job_id = ?
        ORDER BY attempt DESC
        LIMIT 1
        """,
        (run_id, workflow_job_id),
    ).fetchone()
    if row is None:
        raise NotFoundError(f"could not find job for workflow_job_id={workflow_job_id!r}")
    return cast(sqlite3.Row, row)


def _clone_job_attempt(conn: sqlite3.Connection, *, source: sqlite3.Row, attempt: int) -> str:
    """Create a blocked clone for the next bounded revision attempt."""
    job_id = f"job_{source['run_id']}_{source['workflow_job_id']}_a{attempt}"
    now = utc_now()
    conn.execute(
        """
        INSERT INTO jobs (
          job_id, run_id, workflow_job_id, title, job_type, role_id,
          lane_selector_json, capability_requirements_json, state, attempt,
          max_attempts, fresh_session_required, write_scope_json,
          expected_artifacts_json, idempotency_key, created_at
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'blocked', ?, ?, ?, ?, ?, ?, ?)
        """,
        (
            job_id,
            source["run_id"],
            source["workflow_job_id"],
            source["title"],
            source["job_type"],
            source["role_id"],
            source["lane_selector_json"],
            source["capability_requirements_json"],
            attempt,
            source["max_attempts"],
            source["fresh_session_required"],
            source["write_scope_json"],
            source["expected_artifacts_json"],
            f"{source['run_id']}:{source['workflow_job_id']}:{attempt}",
            now,
        ),
    )
    return job_id
