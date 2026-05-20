from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
import os
import signal
import subprocess
import sys
import time
from copy import deepcopy
from pathlib import Path
from typing import Any, Callable, Mapping, cast

from striatum.legacy_sqlite.db import connect


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


JsonDict = dict[str, Any]


# ---------------------------------------------------------------------------
# CLI / fixture helpers (kept local; the supervisor flow does not need the
# wider test_cli_mvp scaffolding).
# ---------------------------------------------------------------------------


def run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(
            f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}"
        )
    if result.stdout.strip() == "":
        return {}
    payload = cast(JsonDict, json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def init_repo(repo: Path) -> None:
    from striatum.legacy_sqlite.db import init_repo as legacy_init_repo

    legacy_init_repo(repo)


def temporary_workflow(tmp_path: Path, workflow: JsonDict) -> Path:
    path = tmp_path / "workflow.json"
    path.write_text(json.dumps(workflow), encoding="utf-8")
    return path


def example_workflow() -> JsonDict:
    loaded = cast(JsonDict, json.loads(WORKFLOW.read_text(encoding="utf-8")))
    assert isinstance(loaded, dict)
    return loaded


def long_running_command(received_path: Path) -> list[str]:
    """Return a python command that loops on stdin appending each line to a file.

    The supervised payload reads stdin line by line until EOF, writes each
    line to ``received_path`` (creating the parent directory if needed),
    flushes, then continues. This mirrors the contract real agent CLIs follow:
    one work-packet per turn delivered as a JSON line on stdin.
    """
    script = (
        "import sys, pathlib, os\n"
        "p = pathlib.Path(os.environ['SUPERVISE_RECEIVED_PATH'])\n"
        "p.parent.mkdir(parents=True, exist_ok=True)\n"
        "f = p.open('a', encoding='utf-8')\n"
        "for line in sys.stdin:\n"
        "    f.write(line)\n"
        "    f.flush()\n"
        "f.close()\n"
    )
    return [sys.executable, "-c", script]


def supervise_workflow(tmp_path: Path, received_path: Path) -> Path:
    """Build a workflow whose codex lane uses a long-running stdin reader.

    The fixture workflow's ``codex`` lane is rewritten to run the long
    python loop above; the lane keeps its other declared properties so
    ``register-session`` and ``claim-next`` keep working as in the rest of
    the test suite.
    """
    workflow = deepcopy(example_workflow())
    lanes = workflow["lanes"]
    assert isinstance(lanes, dict)
    codex = lanes["codex"]
    assert isinstance(codex, dict)
    codex["command"] = long_running_command(received_path)
    return temporary_workflow(tmp_path, workflow)


def prepare_started_run(
    repo: Path,
    workflow_path: Path,
    *,
    extra_env: dict[str, str] | None = None,
) -> str:
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def register(repo: Path, run_id: str, role: str, lane: str) -> str:
    args: list[str] = [
        "register-session",
        "--run-id",
        run_id,
        "--role",
        role,
        "--lane",
        lane,
        "--capability",
        "write",
    ]
    # HARNESS-003: same-operator reviewer registration needs the
    # explicit override.
    if role == "reviewer":
        args += ["--force-non-fresh", "--reason", "test fixture"]
    payload = data(run_cli(repo, *args))
    return str(payload["session_id"])


def claim(repo: Path, session_id: str) -> JsonDict:
    payload = data(run_cli(repo, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    return cast(JsonDict, packet)


def supervise_start_with_env(
    repo: Path, session_id: str, *, received_path: Path
) -> JsonDict:
    """Run ``supervise start`` with ``SUPERVISE_RECEIVED_PATH`` exported."""
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    env["SUPERVISE_RECEIVED_PATH"] = str(received_path)
    proc = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(repo),
            "supervise",
            "start",
            "--session-id",
            session_id,
            "--json",
        ],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    assert proc.returncode == 0, (
        f"supervise start failed: stdout={proc.stdout} stderr={proc.stderr}"
    )
    payload = cast(JsonDict, json.loads(proc.stdout))
    return data(payload)


def wait_for(
    condition: Callable[[], bool], *, timeout: float = 5.0, interval: float = 0.05
) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if condition():
            return True
        time.sleep(interval)
    return False


def pid_alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


def supervisor_row(repo: Path, supervisor_id: str) -> Mapping[str, Any] | None:
    conn = connect(repo)
    try:
        row = conn.execute(
            "SELECT * FROM process_supervisors WHERE supervisor_id = ?",
            (supervisor_id,),
        ).fetchone()
    finally:
        conn.close()
    return cast(Mapping[str, Any] | None, row)


def set_supervisor_fields(repo: Path, supervisor_id: str, **fields: object) -> None:
    assert fields
    assignments = ", ".join(f"{name} = ?" for name in fields)
    values = [*fields.values(), supervisor_id]
    conn = connect(repo)
    try:
        conn.execute(
            f"UPDATE process_supervisors SET {assignments} WHERE supervisor_id = ?",
            values,
        )
        conn.commit()
    finally:
        conn.close()


def supervisor_events(repo: Path, supervisor_id: str) -> list[tuple[str, JsonDict]]:
    conn = connect(repo)
    try:
        rows = conn.execute(
            "SELECT event_type, payload_json FROM events WHERE event_type LIKE 'supervisor.%' ORDER BY event_id"
        ).fetchall()
    finally:
        conn.close()
    out: list[tuple[str, JsonDict]] = []
    for event_type, payload_text in rows:
        payload = cast(JsonDict, json.loads(payload_text))
        if payload.get("supervisor_id") == supervisor_id:
            out.append((event_type, payload))
    return out


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_supervise_start_creates_attached_session(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        assert started["state"] == "attached"
        pid = int(started["pid"])
        assert pid_alive(pid), "supervised pid should be alive after start"
        pipe_path = Path(str(started["stdin_pipe_path"]))
        assert pipe_path.exists(), f"stdin pipe must exist on disk: {pipe_path}"
        # Sanity: the row matches what we returned.
        row = supervisor_row(tmp_path, str(started["supervisor_id"]))
        assert row is not None
        assert row["state"] == "attached"
        assert row["pid"] == pid
        assert row["pid_start_time"] is not None
        assert started["lane_attestation"] == "attested"
        packet = claim(tmp_path, author)
        author_info = packet["job"]["author"]
        assert isinstance(author_info, dict)
        assert author_info["line"] == "author: author-codex-gpt-5.5-001"
        # Lifecycle events recorded.
        events = supervisor_events(tmp_path, str(started["supervisor_id"]))
        types = [event_type for event_type, _ in events]
        assert "supervisor.starting" in types
        assert "supervisor.started" in types
    finally:
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_lane_attestation_requires_pid_identity_match(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        set_supervisor_fields(
            tmp_path,
            str(started["supervisor_id"]),
            pid_start_time="not-the-recorded-process",
        )
        packet = claim(tmp_path, author)
        assert packet["lane_attestation"]["state"] == "unattested"
        assert packet["lane_attestation"]["reason"] == "pid_identity_mismatch"
        author_info = packet["job"]["author"]
        assert isinstance(author_info, dict)
        assert author_info["line"] == "author: operator"
        row = supervisor_row(tmp_path, str(started["supervisor_id"]))
        assert row is not None
        assert row["state"] == "attached"
    finally:
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_lane_attestation_requires_snapshot_lane_command_match(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        set_supervisor_fields(
            tmp_path,
            str(started["supervisor_id"]),
            command_json=json.dumps([sys.executable, "-c", "import time; time.sleep(60)"]),
        )
        packet = claim(tmp_path, author)
        assert packet["lane_attestation"]["state"] == "unattested"
        assert packet["lane_attestation"]["reason"] == "lane_command_mismatch"
        author_info = packet["job"]["author"]
        assert isinstance(author_info, dict)
        assert author_info["line"] == "author: operator"
    finally:
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_starting_supervisor_does_not_attest_lane(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        set_supervisor_fields(tmp_path, str(started["supervisor_id"]), state="starting")
        packet = claim(tmp_path, author)
        assert packet["lane_attestation"]["state"] == "unattested"
        assert packet["lane_attestation"]["reason"] == "no_attached_supervisor"
        author_info = packet["job"]["author"]
        assert isinstance(author_info, dict)
        assert author_info["line"] == "author: operator"
    finally:
        set_supervisor_fields(tmp_path, str(started["supervisor_id"]), state="attached")
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_supervise_send_delivers_packet_to_running_process(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    packet_id = str(packet["packet_id"])

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        result = data(
            run_cli(
                tmp_path,
                "supervise",
                "send",
                "--session-id",
                author,
                "--packet-id",
                packet_id,
            )
        )
        assert result["packet_id"] == packet_id
        assert int(result["bytes"]) > 0

        # Wait for the child to flush the packet to disk.
        assert wait_for(lambda: received.exists() and received.stat().st_size > 0)
        delivered_text = received.read_text(encoding="utf-8").splitlines()
        assert len(delivered_text) >= 1
        delivered = json.loads(delivered_text[0])
        assert delivered["packet_id"] == packet_id

        events = supervisor_events(tmp_path, str(started["supervisor_id"]))
        delivered_events = [
            payload
            for event_type, payload in events
            if event_type == "supervisor.packet_delivered"
        ]
        assert len(delivered_events) == 1
        assert delivered_events[0]["packet_id"] == packet_id
        assert int(delivered_events[0]["bytes_written"]) == int(result["bytes"])
    finally:
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_supervise_send_two_packets_in_a_row(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")
    first_packet = claim(tmp_path, author)
    first_id = str(first_packet["packet_id"])
    # Inject a second synthetic work packet for the same session via a fresh
    # synthetic queue_message + lease so the test does not depend on the
    # workflow having two queued jobs for the author lane. supervise send
    # only requires a packet row owned by this session; constructing one
    # directly keeps the test focused on supervisor mechanics.
    job_id = str(first_packet["job"]["job_id"])
    second_id = first_id + "_b"
    second_payload = json.dumps({"packet_id": second_id, "synthetic": True})
    second_message_id = "msg_supervise_test_second"
    second_lease_id = "lease_supervise_test_second"
    conn = connect(tmp_path)
    try:
        # Use a kind other than 'work' so the partial unique index on
        # work-state messages by job is not tripped — supervise send only
        # cares about a work_packets row, not about the queue message that
        # produced it.
        conn.execute(
            """
            INSERT INTO queue_messages (
              message_id, run_id, job_id, kind, state, priority,
              target_role_id, target_lane_id, payload_json, claim_count,
              max_claims, created_at, updated_at, claimed_at, current_lease_id
            )
            VALUES (?, ?, ?, 'agent_message', 'acked', 0, ?, ?, '{}', 1, 1,
                    datetime('now'), datetime('now'), datetime('now'), ?)
            """,
            (
                second_message_id,
                run_id,
                job_id,
                "author",
                "codex",
                second_lease_id,
            ),
        )
        # Use a non-active state so the partial unique index on
        # (resource_type, resource_id) WHERE state='active' tolerates the
        # synthetic lease alongside the real one held by the active session.
        conn.execute(
            """
            INSERT INTO leases (
              lease_id, run_id, resource_type, resource_id, owner_session_id,
              state, acquired_at, expires_at, last_heartbeat_at, released_at,
              release_reason
            )
            VALUES (?, ?, 'job', ?, ?, 'released',
                    datetime('now'), datetime('now', '+1 hour'),
                    datetime('now'), datetime('now'), 'test fixture')
            """,
            (second_lease_id, run_id, job_id, author),
        )
        conn.execute(
            """
            INSERT INTO work_packets (
              packet_id, run_id, job_id, message_id, lease_id, session_id,
              packet_json, packet_sha256, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
            """,
            (
                second_id,
                run_id,
                job_id,
                second_message_id,
                second_lease_id,
                author,
                second_payload,
                "0" * 64,
            ),
        )
        conn.commit()
    finally:
        conn.close()

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        run_cli(
            tmp_path,
            "supervise",
            "send",
            "--session-id",
            author,
            "--packet-id",
            first_id,
        )
        run_cli(
            tmp_path,
            "supervise",
            "send",
            "--session-id",
            author,
            "--packet-id",
            second_id,
        )

        assert wait_for(
            lambda: received.exists()
            and len(received.read_text(encoding="utf-8").splitlines()) >= 2,
            timeout=10.0,
        )
        lines = received.read_text(encoding="utf-8").splitlines()
        first_received = json.loads(lines[0])
        second_received = json.loads(lines[1])
        assert first_received["packet_id"] == first_id
        assert second_received["packet_id"] == second_id

        events = supervisor_events(tmp_path, str(started["supervisor_id"]))
        delivered_ids = [
            payload["packet_id"]
            for event_type, payload in events
            if event_type == "supervisor.packet_delivered"
        ]
        assert delivered_ids == [first_id, second_id]
    finally:
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_supervise_stop_terminates_child_and_cleans_pipe(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    pid = int(started["pid"])
    pipe_path = Path(str(started["stdin_pipe_path"]))
    assert pid_alive(pid)
    assert pipe_path.exists()

    stopped = data(
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "operator stop",
        )
    )
    assert stopped["state"] == "stopped"
    # Wait for the kernel to reap the child after SIGTERM.
    assert wait_for(lambda: not pid_alive(pid), timeout=5.0)
    assert not pipe_path.exists(), "stop must remove the named pipe"

    row = supervisor_row(tmp_path, str(started["supervisor_id"]))
    assert row is not None
    assert row["state"] == "stopped"
    assert row["stop_reason"] == "operator stop"
    assert row["ended_at"] is not None

    events = supervisor_events(tmp_path, str(started["supervisor_id"]))
    types = [event_type for event_type, _ in events]
    assert "supervisor.stopped" in types


def test_supervise_status_marks_lost_when_pid_disappears(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    pid = int(started["pid"])
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    assert wait_for(lambda: not pid_alive(pid), timeout=5.0)

    status = data(
        run_cli(
            tmp_path,
            "supervise",
            "status",
            "--session-id",
            author,
        )
    )
    assert status["state"] == "lost"
    assert status["liveness"] == "gone"

    events = supervisor_events(tmp_path, str(started["supervisor_id"]))
    types = [event_type for event_type, _ in events]
    assert "supervisor.lost" in types


def test_supervise_refuses_second_start_for_same_session(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        env = os.environ.copy()
        env["PYTHONPATH"] = str(ROOT / "src")
        env["SUPERVISE_RECEIVED_PATH"] = str(received)
        result = subprocess.run(
            [
                sys.executable,
                "-m",
                "striatum.cli",
                "--repo",
                str(tmp_path),
                "supervise",
                "start",
                "--session-id",
                author,
                "--json",
            ],
            cwd=tmp_path,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )
        assert result.returncode != 0, (
            f"second start must fail: stdout={result.stdout} stderr={result.stderr}"
        )
        payload = json.loads(result.stdout)
        assert payload["ok"] is False
        assert "already has an active supervisor" in str(payload["error"]["message"])
    finally:
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_claim_next_auto_delivers_to_attached_supervisor(tmp_path: Path) -> None:
    """``claim-next`` routes packets through an attached supervisor's pipe.

    With a supervisor started before the claim, the runner writes the freshly
    built work packet to the supervisor's stdin pipe inside the same
    transaction, records a ``supervisor.packet_delivered`` event, and includes
    a ``supervisor_delivery`` summary in the CLI response. The agent's child
    process observes the packet exactly as if ``supervise send`` had been
    invoked separately.
    """
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    try:
        payload = data(run_cli(tmp_path, "claim-next", "--session-id", author))
        assert payload["status"] == "claimed"
        assert "supervisor_delivery" in payload, payload
        delivery = payload["supervisor_delivery"]
        assert isinstance(delivery, dict)
        assert delivery["supervisor_id"] == started["supervisor_id"]
        assert int(delivery["bytes_written"]) > 0

        packet = payload["packet"]
        assert isinstance(packet, dict)
        packet_id = str(packet["packet_id"])

        # The supervised child must observe the packet on stdin (it appends
        # received lines to ``received_path``).
        assert wait_for(lambda: received.exists() and received.stat().st_size > 0)
        delivered_lines = received.read_text(encoding="utf-8").splitlines()
        assert len(delivered_lines) >= 1
        delivered = json.loads(delivered_lines[0])
        assert delivered["packet_id"] == packet_id

        events = supervisor_events(tmp_path, str(started["supervisor_id"]))
        delivered_events = [
            evt_payload
            for event_type, evt_payload in events
            if event_type == "supervisor.packet_delivered"
        ]
        assert len(delivered_events) == 1
        assert delivered_events[0]["packet_id"] == packet_id
        assert int(delivered_events[0]["bytes_written"]) == int(delivery["bytes_written"])
        assert delivered_events[0].get("via") == "claim_next_auto_delivery"
    finally:
        run_cli(
            tmp_path,
            "supervise",
            "stop",
            "--session-id",
            author,
            "--reason",
            "test cleanup",
            check=False,
        )


def test_claim_next_without_supervisor_unchanged(tmp_path: Path) -> None:
    """Sessions without a supervisor see no ``supervisor_delivery`` field.

    The claim payload still contains ``status`` and ``packet``, exactly as
    before RFC 0009 auto-delivery landed; this preserves backwards
    compatibility for callers and tests that do not interact with the
    supervisor flow.
    """
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    payload = data(run_cli(tmp_path, "claim-next", "--session-id", author))
    assert payload["status"] == "claimed"
    assert "supervisor_delivery" not in payload, payload


def test_claim_next_marks_supervisor_lost_when_pipe_missing(tmp_path: Path) -> None:
    """A vanished pipe transitions the supervisor to ``lost`` on claim.

    The packet itself is still returned so the caller can recover (re-start
    the supervisor and call ``supervise send`` for the packet id), but the
    supervisor row is no longer ``attached`` and a ``supervisor.lost`` event
    is recorded with a ``stdin_pipe_missing`` reason.
    """
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    pid = int(started["pid"])
    pipe_path = Path(str(started["stdin_pipe_path"]))
    try:
        # External actor (or a crashed environment) removed the FIFO.
        assert pipe_path.exists()
        pipe_path.unlink()
        assert not pipe_path.exists()

        payload = data(run_cli(tmp_path, "claim-next", "--session-id", author))
        assert payload["status"] == "claimed"
        assert "packet" in payload, payload
        delivery = payload.get("supervisor_delivery")
        assert isinstance(delivery, dict), payload
        assert delivery["supervisor_id"] == started["supervisor_id"]
        assert delivery.get("error") == "stdin_pipe_missing"
        assert "bytes_written" not in delivery

        row = supervisor_row(tmp_path, str(started["supervisor_id"]))
        assert row is not None
        assert row["state"] == "lost"
        assert row["ended_at"] is not None

        events = supervisor_events(tmp_path, str(started["supervisor_id"]))
        types = [event_type for event_type, _ in events]
        assert "supervisor.lost" in types
    finally:
        # The supervisor row is already lost; reap the orphaned child.
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


def test_doctor_flags_supervisor_with_dead_pid(tmp_path: Path) -> None:
    received = tmp_path / "received.txt"
    workflow_path = supervise_workflow(tmp_path, received)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")

    started = supervise_start_with_env(tmp_path, author, received_path=received)
    pid = int(started["pid"])
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    assert wait_for(lambda: not pid_alive(pid), timeout=5.0)

    report = data(run_cli(tmp_path, "doctor"))
    problems = report["problems"]
    assert isinstance(problems, list)
    expected = f"supervisor pid is gone: {started['supervisor_id']}"
    assert expected in problems
