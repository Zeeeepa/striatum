from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from striatum.cross_repo import (
    cancel_cross_repo_run,
    describe_cross_repo_run,
    prepare_cross_repo_run,
    reconcile_cross_repo_preparing,
    start_cross_repo_run,
)


class FakeCursor:
    def __init__(self, conn: FakeConn) -> None:
        self.conn = conn
        self.result: list[dict[str, Any]] = []

    def __enter__(self) -> FakeCursor:
        return self

    def __exit__(self, *args: object) -> None:
        return None

    def execute(self, sql: str, params: tuple[object, ...] = ()) -> None:
        normalized = " ".join(sql.split())
        if normalized.startswith("INSERT INTO striatumd.cross_repo_runs"):
            self.conn.runs[str(params[0])] = {
                "cross_repo_run_id": params[0],
                "workflow_id": params[1],
                "workflow_version": params[2],
                "workflow_snapshot_hash": params[3],
                "primary_repository_id": params[4],
                "state": "preparing",
                "created_at": params[5],
                "reconcile_after": params[6],
                "last_reconcile_error": None,
            }
        elif normalized.startswith("INSERT INTO striatumd.cross_repo_run_repositories"):
            self.conn.participants[(str(params[0]), str(params[1]))] = {
                "cross_repo_run_id": params[0],
                "repository_alias": params[1],
                "repository_id": params[2],
                "local_run_id": None,
                "state": "preparing",
                "last_observed_at": params[3],
            }
        elif normalized.startswith("UPDATE striatumd.cross_repo_run_repositories SET local_run_id"):
            row = self.conn.participants[(str(params[3]), str(params[4]))]
            row["local_run_id"] = params[0]
            row["state"] = "prepared"
        elif normalized.startswith("UPDATE striatumd.cross_repo_run_repositories SET state"):
            row = self.conn.participants[(str(params[2]), str(params[3]))]
            row["state"] = params[0]
        elif normalized.startswith("UPDATE striatumd.cross_repo_runs SET state = 'prepared'"):
            self.conn.runs[str(params[0])]["state"] = "prepared"
        elif normalized.startswith("UPDATE striatumd.cross_repo_runs SET state = 'aborted'"):
            row = self.conn.runs[str(params[1])]
            row["state"] = "aborted"
            row["last_reconcile_error"] = params[0]
        elif normalized.startswith("UPDATE striatumd.cross_repo_runs SET state = 'blocked'"):
            row = self.conn.runs[str(params[1])]
            row["state"] = "blocked"
            row["last_reconcile_error"] = params[0]
        elif normalized.startswith("UPDATE striatumd.cross_repo_runs SET state = 'running'"):
            self.conn.runs[str(params[1])]["state"] = "running"
        elif normalized.startswith("UPDATE striatumd.cross_repo_runs SET state = 'canceling'"):
            self.conn.runs[str(params[0])]["state"] = "canceling"
        elif normalized.startswith("UPDATE striatumd.cross_repo_runs SET state = %s"):
            row = self.conn.runs[str(params[4])]
            row["state"] = params[0]
            row["last_reconcile_error"] = params[3]
        elif normalized.startswith("SELECT cross_repo_run_id FROM striatumd.cross_repo_runs WHERE state = 'preparing'"):
            self.result = [row for row in self.conn.runs.values() if row["state"] == "preparing"]
        elif normalized.startswith("SELECT * FROM striatumd.cross_repo_runs"):
            maybe_row = self.conn.runs.get(str(params[0]))
            self.result = [] if maybe_row is None else [maybe_row]
        elif normalized.startswith("SELECT repository_alias, repository_id, local_run_id, state"):
            rows = [
                row
                for (run_id, _alias), row in self.conn.participants.items()
                if run_id == str(params[0])
            ]
            self.result = sorted(rows, key=lambda item: str(item["repository_alias"]))
        elif normalized.startswith("SELECT cross_repo_run_id, workflow_id"):
            self.result = list(self.conn.runs.values())
        else:  # pragma: no cover - catches query drift in the helper.
            raise AssertionError(normalized)

    def fetchone(self) -> dict[str, Any] | None:
        return self.result[0] if self.result else None

    def fetchall(self) -> list[dict[str, Any]]:
        return self.result


class FakeConn:
    def __init__(self) -> None:
        self.runs: dict[str, dict[str, Any]] = {}
        self.participants: dict[tuple[str, str], dict[str, Any]] = {}

    def cursor(self) -> FakeCursor:
        return FakeCursor(self)


@dataclass
class FakeRunner:
    fail_start_alias: str | None = None
    fail_cancel_alias: str | None = None
    intact: bool = True
    prepared: list[str] = field(default_factory=list)
    started: list[str] = field(default_factory=list)
    canceled: list[str] = field(default_factory=list)
    checkpoints: list[str] = field(default_factory=list)

    def prepare(self, *, repository_id: str, repository_alias: str, cross_repo_run_id: str) -> str:
        self.prepared.append(repository_alias)
        return f"run_{repository_alias}"

    def start(self, *, repository_id: str, local_run_id: str) -> None:
        alias = local_run_id.removeprefix("run_")
        if alias == self.fail_start_alias:
            raise RuntimeError("repo unavailable")
        self.started.append(alias)

    def cancel(self, *, repository_id: str, local_run_id: str, reason: str) -> None:
        alias = local_run_id.removeprefix("run_")
        if alias == self.fail_cancel_alias:
            raise RuntimeError(f"cancel failed for {alias}")
        self.canceled.append(alias)

    def participant_intact(self, *, repository_id: str, local_run_id: str | None) -> bool:
        return self.intact

    def human_checkpoint(self, *, repository_id: str, local_run_id: str | None, reason: str) -> None:
        self.checkpoints.append(reason)


def _workflow() -> dict[str, Any]:
    return {
        "workflow_id": "wf",
        "workflow_version": "1",
        "repositories": {
            "primary": {"repo_id": "repo_primary"},
            "consumer": {"repo_id": "repo_consumer"},
        },
        "primary_repository": "primary",
    }


def test_prepare_start_cancel_and_describe_cross_repo_run() -> None:
    conn = FakeConn()
    runner = FakeRunner()

    prepared = prepare_cross_repo_run(conn, workflow=_workflow(), local_runner=runner, cross_repo_run_id="xrun_1")
    assert prepared["state"] == "prepared"
    assert runner.prepared == ["consumer", "primary"]

    started = start_cross_repo_run(conn, cross_repo_run_id="xrun_1", local_runner=runner)
    assert started["state"] == "running"
    # Primary starts first (the orchestrator's home repo), then the rest
    # alphabetically — see the sort key in start_cross_repo_run.
    assert runner.started == ["primary", "consumer"]

    described = describe_cross_repo_run(conn, cross_repo_run_id="xrun_1")
    assert described["state"] == "running"
    assert {row["repository_alias"] for row in described["participants"]} == {"primary", "consumer"}

    canceled = cancel_cross_repo_run(conn, cross_repo_run_id="xrun_1", local_runner=runner, reason="test")
    assert canceled["state"] == "canceled"
    assert set(runner.canceled) == {"primary", "consumer"}


def test_cancel_skips_terminal_participants() -> None:
    conn = FakeConn()
    runner = FakeRunner()
    prepare_cross_repo_run(
        conn,
        workflow=_workflow(),
        local_runner=runner,
        cross_repo_run_id="xrun_1",
    )
    start_cross_repo_run(conn, cross_repo_run_id="xrun_1", local_runner=runner)
    conn.participants[("xrun_1", "consumer")]["state"] = "completed"
    runner.canceled.clear()

    canceled = cancel_cross_repo_run(
        conn,
        cross_repo_run_id="xrun_1",
        local_runner=runner,
        reason="test",
    )

    assert canceled["state"] == "canceled"
    assert canceled["skipped_terminal"] == ["consumer"]
    assert runner.canceled == ["primary"]
    states = {
        alias: row["state"]
        for (_run_id, alias), row in conn.participants.items()
    }
    assert states == {"consumer": "completed", "primary": "canceled"}


def test_cancel_skips_preparing_participants_without_local_run() -> None:
    conn = FakeConn()
    runner = FakeRunner()
    prepare_cross_repo_run(
        conn,
        workflow=_workflow(),
        local_runner=runner,
        cross_repo_run_id="xrun_1",
    )
    conn.runs["xrun_1"]["state"] = "preparing"
    conn.participants[("xrun_1", "consumer")]["state"] = "preparing"
    conn.participants[("xrun_1", "consumer")]["local_run_id"] = None
    runner.canceled.clear()

    canceled = cancel_cross_repo_run(
        conn,
        cross_repo_run_id="xrun_1",
        local_runner=runner,
        reason="operator request",
    )

    assert canceled["state"] == "canceled"
    assert canceled["skipped_no_local_run"] == ["consumer"]
    assert runner.canceled == ["primary"]
    states = {
        alias: row["state"]
        for (_run_id, alias), row in conn.participants.items()
    }
    assert states == {"consumer": "canceled", "primary": "canceled"}


def test_cancel_records_blocked_error_details() -> None:
    conn = FakeConn()
    runner = FakeRunner(fail_cancel_alias="consumer")
    prepare_cross_repo_run(
        conn,
        workflow=_workflow(),
        local_runner=runner,
        cross_repo_run_id="xrun_1",
    )
    start_cross_repo_run(conn, cross_repo_run_id="xrun_1", local_runner=runner)

    canceled = cancel_cross_repo_run(
        conn,
        cross_repo_run_id="xrun_1",
        local_runner=runner,
        reason="operator request",
    )

    assert canceled["state"] == "blocked"
    assert canceled["blocked"] == ["consumer"]
    assert canceled["blocked_errors"] == {"consumer": "cancel failed for consumer"}
    assert conn.runs["xrun_1"]["last_reconcile_error"] == "consumer: cancel failed for consumer"


def test_start_failure_blocks_and_records_primary_checkpoint() -> None:
    conn = FakeConn()
    runner = FakeRunner(fail_start_alias="consumer")
    prepare_cross_repo_run(conn, workflow=_workflow(), local_runner=runner, cross_repo_run_id="xrun_1")

    result = start_cross_repo_run(conn, cross_repo_run_id="xrun_1", local_runner=runner)

    assert result["state"] == "blocked"
    assert runner.checkpoints == ["repo unavailable"]


def test_reconcile_preparing_marks_prepared_or_aborted() -> None:
    conn = FakeConn()
    runner = FakeRunner()
    prepare_cross_repo_run(conn, workflow=_workflow(), local_runner=runner, cross_repo_run_id="xrun_1")
    conn.runs["xrun_1"]["state"] = "preparing"

    assert reconcile_cross_repo_preparing(conn, local_runner=runner) == {"prepared": ["xrun_1"], "aborted": []}

    conn.runs["xrun_1"]["state"] = "preparing"
    runner.intact = False
    assert reconcile_cross_repo_preparing(conn, local_runner=runner) == {"prepared": [], "aborted": ["xrun_1"]}
