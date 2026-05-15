"""Daemon-owned cross-repository workflow lifecycle helpers.

The dogfood-035 slice is intentionally unit-testable: callers inject the
repo-local operations so this module can exercise daemon coordination without
requiring the deferred multi-repo daemon harness.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Mapping, Protocol

from striatum.db import json_dumps, new_id, sha256_bytes, utc_now
from striatum.errors import InvalidTransitionError, NotFoundError


def _dict_cursor(conn: Any) -> Any:
    """Return a cursor yielding dict rows regardless of the connection's
    default row_factory.

    Production daemon connections set ``row_factory = psycopg.rows.dict_row``
    at the connection level (daemon.py daemon-start path); tests that call
    these helpers via ``daemon_pg.connection.connect`` directly get the
    default tuple_row factory, which makes the module's ``_row_dict`` /
    ``_row_value`` helpers fail. Forcing dict rows per-cursor keeps both
    call sites correct without changing the global default (which would
    break positional access in test fixtures).
    """
    try:
        from psycopg.rows import dict_row
    except ImportError:
        return conn.cursor()
    try:
        return conn.cursor(row_factory=dict_row)
    except TypeError:
        # FakeConn-style test doubles don't accept the row_factory kwarg.
        return conn.cursor()


def _atomic(conn: Any) -> Any:
    """Return a context manager that wraps the next block in a transaction
    when the connection supports it; otherwise a no-op.

    Production psycopg connections expose ``conn.transaction()`` which
    creates an explicit transaction (or savepoint when nested). Unit-test
    fakes (e.g. ``tests/test_cross_repo_lifecycle.py::FakeConn``) don't
    implement it. Use a nullcontext fallback so the same call site works
    for both shapes — the atomicity contract holds in production and the
    unit-test fakes already enforce in-memory semantics directly.
    """
    if hasattr(conn, "transaction"):
        return conn.transaction()
    from contextlib import nullcontext
    return nullcontext()


TERMINAL_STATES = frozenset({"canceled", "completed", "failed", "aborted"})


class CrossRepoLocalRunner(Protocol):
    """Repo-local operations used by daemon cross-repo orchestration."""

    def prepare(self, *, repository_id: str, repository_alias: str, cross_repo_run_id: str) -> str:
        """Prepare a local run and return its local run id."""

    def start(self, *, repository_id: str, local_run_id: str) -> None:
        """Start an already-prepared local participant run."""

    def cancel(self, *, repository_id: str, local_run_id: str, reason: str) -> None:
        """Cancel a local participant run."""

    def participant_intact(self, *, repository_id: str, local_run_id: str | None) -> bool:
        """Return whether a participant's repo-local run row still exists."""

    def human_checkpoint(self, *, repository_id: str, local_run_id: str | None, reason: str) -> None:
        """Record a primary-repo human checkpoint for operator repair."""


@dataclass(frozen=True)
class CrossRepoParticipant:
    repository_alias: str
    repository_id: str
    local_run_id: str | None
    state: str


def prepare_cross_repo_run(
    conn: Any,
    *,
    workflow: Mapping[str, Any],
    local_runner: CrossRepoLocalRunner,
    cross_repo_run_id: str | None = None,
) -> dict[str, Any]:
    repositories = _repositories(workflow)
    primary_alias = str(workflow["primary_repository"])
    run_id = cross_repo_run_id or new_id("xrun")
    now = utc_now()
    snapshot = json_dumps(workflow)
    # The whole INSERT-cross-repo-runs + INSERT-participants block has to
    # be atomic: if a participant FK fails (the workflow names an
    # unregistered repository) the cross_repo_runs row must not survive.
    # Under autocommit=True (production daemon connection setting) each
    # statement would commit independently and leave a stranded 'preparing'
    # row. ``_atomic(conn)`` brackets the whole block as a single
    # transaction regardless of autocommit; unit-test fakes that don't
    # implement ``conn.transaction()`` fall through to a no-op context.
    with _atomic(conn), _dict_cursor(conn) as cur:
        cur.execute(
            """
            INSERT INTO striatumd.cross_repo_runs (
              cross_repo_run_id, workflow_id, workflow_version,
              workflow_snapshot_hash, primary_repository_id, state, created_at,
              reconcile_after
            )
            VALUES (%s, %s, %s, %s, %s, 'preparing', %s, %s)
            """,
            (
                run_id,
                str(workflow["workflow_id"]),
                workflow.get("workflow_version"),
                sha256_bytes(snapshot.encode("utf-8")),
                repositories[primary_alias],
                now,
                now,
            ),
        )
        for alias, repository_id in repositories.items():
            cur.execute(
                """
                INSERT INTO striatumd.cross_repo_run_repositories (
                  cross_repo_run_id, repository_alias, repository_id, state,
                  last_observed_at
                )
                VALUES (%s, %s, %s, 'preparing', %s)
                """,
                (run_id, alias, repository_id, now),
            )
    participants: list[dict[str, Any]] = []
    try:
        for alias in sorted(repositories):
            repository_id = repositories[alias]
            local_run_id = local_runner.prepare(
                repository_id=repository_id,
                repository_alias=alias,
                cross_repo_run_id=run_id,
            )
            participants.append(
                {"repository_alias": alias, "repository_id": repository_id, "local_run_id": local_run_id}
            )
            with _dict_cursor(conn) as cur:
                cur.execute(
                    """
                    UPDATE striatumd.cross_repo_run_repositories
                    SET local_run_id = %s, state = 'prepared', prepared_at = %s,
                        last_observed_at = %s
                    WHERE cross_repo_run_id = %s AND repository_alias = %s
                    """,
                    (local_run_id, utc_now(), utc_now(), run_id, alias),
                )
    except Exception as exc:
        with _dict_cursor(conn) as cur:
            cur.execute(
                """
                UPDATE striatumd.cross_repo_runs
                SET state = 'aborted', last_reconcile_error = %s
                WHERE cross_repo_run_id = %s
                """,
                (str(exc), run_id),
            )
        raise
    with _dict_cursor(conn) as cur:
        cur.execute(
            "UPDATE striatumd.cross_repo_runs SET state = 'prepared' WHERE cross_repo_run_id = %s",
            (run_id,),
        )
    return {"cross_repo_run_id": run_id, "state": "prepared", "participants": participants}


def start_cross_repo_run(
    conn: Any, *, cross_repo_run_id: str, local_runner: CrossRepoLocalRunner
) -> dict[str, Any]:
    run = _cross_repo_run(conn, cross_repo_run_id)
    if run["state"] != "prepared":
        raise InvalidTransitionError(
            f"cross-repo run {cross_repo_run_id} must be prepared before start"
        )
    participants = _participants(conn, cross_repo_run_id)
    primary = str(run["primary_repository_id"])
    # Start the primary repo first, then the rest alphabetically. The primary
    # is the orchestrator's home repo; if any non-primary start fails, the
    # primary should already be running so the human checkpoint has somewhere
    # to record the blocker.
    participants = sorted(
        participants,
        key=lambda p: (p.repository_id != primary, p.repository_alias),
    )
    started: list[str] = []
    try:
        for participant in participants:
            if participant.local_run_id is None:
                raise InvalidTransitionError(
                    f"participant {participant.repository_alias} has no local_run_id"
                )
            local_runner.start(
                repository_id=participant.repository_id,
                local_run_id=participant.local_run_id,
            )
            started.append(participant.repository_alias)
            _update_participant(conn, cross_repo_run_id, participant.repository_alias, "running")
    except Exception as exc:
        local_runner.human_checkpoint(
            repository_id=primary,
            local_run_id=_primary_local_run_id(participants, primary),
            reason=str(exc),
        )
        with _dict_cursor(conn) as cur:
            cur.execute(
                """
                UPDATE striatumd.cross_repo_runs
                SET state = 'blocked', last_reconcile_error = %s
                WHERE cross_repo_run_id = %s
                """,
                (str(exc), cross_repo_run_id),
            )
        return {"cross_repo_run_id": cross_repo_run_id, "state": "blocked", "started": started}
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            UPDATE striatumd.cross_repo_runs
            SET state = 'running', started_at = %s
            WHERE cross_repo_run_id = %s
            """,
            (utc_now(), cross_repo_run_id),
        )
    return {"cross_repo_run_id": cross_repo_run_id, "state": "running", "started": started}


def cancel_cross_repo_run(
    conn: Any,
    *,
    cross_repo_run_id: str,
    local_runner: CrossRepoLocalRunner,
    reason: str,
) -> dict[str, Any]:
    run = _cross_repo_run(conn, cross_repo_run_id)
    if str(run["state"]) in TERMINAL_STATES:
        return {"cross_repo_run_id": cross_repo_run_id, "state": run["state"], "canceled": []}
    canceled: list[str] = []
    blocked: list[str] = []
    with _dict_cursor(conn) as cur:
        cur.execute(
            "UPDATE striatumd.cross_repo_runs SET state = 'canceling' WHERE cross_repo_run_id = %s",
            (cross_repo_run_id,),
        )
    for participant in _participants(conn, cross_repo_run_id):
        if participant.local_run_id is None:
            blocked.append(participant.repository_alias)
            continue
        try:
            local_runner.cancel(
                repository_id=participant.repository_id,
                local_run_id=participant.local_run_id,
                reason=reason,
            )
            canceled.append(participant.repository_alias)
            _update_participant(conn, cross_repo_run_id, participant.repository_alias, "canceled")
        except Exception:
            blocked.append(participant.repository_alias)
            _update_participant(conn, cross_repo_run_id, participant.repository_alias, "blocked")
    state = "blocked" if blocked else "canceled"
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            UPDATE striatumd.cross_repo_runs
            SET state = %s, canceled_at = CASE WHEN %s = 'canceled' THEN %s ELSE canceled_at END
            WHERE cross_repo_run_id = %s
            """,
            (state, state, utc_now(), cross_repo_run_id),
        )
    return {"cross_repo_run_id": cross_repo_run_id, "state": state, "canceled": canceled, "blocked": blocked}


def reconcile_cross_repo_preparing(
    conn: Any, *, local_runner: CrossRepoLocalRunner
) -> dict[str, Any]:
    reconciled: list[str] = []
    aborted: list[str] = []
    with _dict_cursor(conn) as cur:
        cur.execute(
            "SELECT cross_repo_run_id FROM striatumd.cross_repo_runs WHERE state = 'preparing'"
        )
        rows = cur.fetchall()
    for row in rows:
        run_id = str(_row_value(row, "cross_repo_run_id"))
        participants = _participants(conn, run_id)
        if all(
            item.local_run_id is not None
            and local_runner.participant_intact(
                repository_id=item.repository_id, local_run_id=item.local_run_id
            )
            for item in participants
        ):
            with _dict_cursor(conn) as cur:
                cur.execute(
                    "UPDATE striatumd.cross_repo_runs SET state = 'prepared' WHERE cross_repo_run_id = %s",
                    (run_id,),
                )
            reconciled.append(run_id)
        else:
            with _dict_cursor(conn) as cur:
                cur.execute(
                    """
                    UPDATE striatumd.cross_repo_runs
                    SET state = 'aborted', last_reconcile_error = %s
                    WHERE cross_repo_run_id = %s
                    """,
                    ("participant local run missing during reconcile", run_id),
                )
            aborted.append(run_id)
    return {"prepared": reconciled, "aborted": aborted}


def describe_cross_repo_run(conn: Any, *, cross_repo_run_id: str) -> dict[str, Any]:
    run = _cross_repo_run(conn, cross_repo_run_id)
    return {
        "cross_repo_run_id": cross_repo_run_id,
        "workflow_id": run["workflow_id"],
        "workflow_version": run.get("workflow_version"),
        "primary_repository_id": run["primary_repository_id"],
        "state": run["state"],
        "participants": [
            {
                "repository_alias": item.repository_alias,
                "repository_id": item.repository_id,
                "local_run_id": item.local_run_id,
                "state": item.state,
            }
            for item in _participants(conn, cross_repo_run_id)
        ],
    }


def list_cross_repo_runs(conn: Any) -> dict[str, Any]:
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            SELECT cross_repo_run_id, workflow_id, workflow_version,
                   primary_repository_id, state
            FROM striatumd.cross_repo_runs
            ORDER BY created_at DESC
            """
        )
        rows = cur.fetchall()
    return {"cross_repo_runs": [_row_dict(row) for row in rows]}


def _repositories(workflow: Mapping[str, Any]) -> dict[str, str]:
    raw = workflow.get("repositories")
    if not isinstance(raw, dict) or not raw:
        raise InvalidTransitionError("workflow is not a cross-repo workflow")
    result: dict[str, str] = {}
    for alias, body in raw.items():
        if not isinstance(body, dict):
            raise InvalidTransitionError("workflow repositories block is invalid")
        result[str(alias)] = str(body["repo_id"])
    if workflow.get("primary_repository") not in result:
        raise InvalidTransitionError("cross-repo workflow primary_repository is invalid")
    return result


def _cross_repo_run(conn: Any, cross_repo_run_id: str) -> dict[str, Any]:
    with _dict_cursor(conn) as cur:
        cur.execute(
            "SELECT * FROM striatumd.cross_repo_runs WHERE cross_repo_run_id = %s",
            (cross_repo_run_id,),
        )
        row = cur.fetchone()
    if row is None:
        raise NotFoundError(f"cross-repo run not found: {cross_repo_run_id}")
    return _row_dict(row)


def _participants(conn: Any, cross_repo_run_id: str) -> list[CrossRepoParticipant]:
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            SELECT repository_alias, repository_id, local_run_id, state
            FROM striatumd.cross_repo_run_repositories
            WHERE cross_repo_run_id = %s
            ORDER BY repository_alias
            """,
            (cross_repo_run_id,),
        )
        rows = cur.fetchall()
    return [
        CrossRepoParticipant(
            repository_alias=str(_row_value(row, "repository_alias")),
            repository_id=str(_row_value(row, "repository_id")),
            local_run_id=(
                None
                if _row_value(row, "local_run_id", default=None) is None
                else str(_row_value(row, "local_run_id"))
            ),
            state=str(_row_value(row, "state")),
        )
        for row in rows
    ]


def _update_participant(conn: Any, cross_repo_run_id: str, alias: str, state: str) -> None:
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            UPDATE striatumd.cross_repo_run_repositories
            SET state = %s, last_observed_at = %s
            WHERE cross_repo_run_id = %s AND repository_alias = %s
            """,
            (state, utc_now(), cross_repo_run_id, alias),
        )


def _primary_local_run_id(participants: list[CrossRepoParticipant], primary_repository_id: str) -> str | None:
    for item in participants:
        if item.repository_id == primary_repository_id:
            return item.local_run_id
    return None


def _row_dict(row: Any) -> dict[str, Any]:
    if isinstance(row, dict):
        return dict(row)
    if hasattr(row, "keys"):
        return {key: row[key] for key in row.keys()}
    raise TypeError("database row must expose mapping-like keys")


def _row_value(row: Any, key: str, *, default: Any = ... ) -> Any:
    if isinstance(row, dict):
        if default is ...:
            return row[key]
        return row.get(key, default)
    if hasattr(row, "keys"):
        keys = row.keys()
        if key in keys:
            return row[key]
        if default is not ...:
            return default
    raise TypeError("database row must expose mapping-like keys")
