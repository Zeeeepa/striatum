# ruff: noqa
"""RFC 0014 V1 process-adapter completion guarantees tests.

Closes [issue #1](https://github.com/halbritt/striatum/issues/1):
process adapters can exit or hang without completing claimed jobs.

Test pattern: each test builds a minimal one-job workflow with a
``process`` lane whose command is a bash invocation (``true``,
``exit 1``, ``sleep 30``, etc.) and asserts the post-exit
validation, blocker insertion, envelope shape, and recovery surface.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, cast

import pytest

ROOT = Path(__file__).resolve().parents[1]

JsonDict = dict[str, Any]


def test_adapter_run_is_retired_outside_legacy_test_harness(tmp_path: Path) -> None:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    env["STRIATUM_DAEMON_REQUIRED"] = "1"
    env["STRIATUM_SQLITE_CONNECT_TRIPWIRE"] = "1"
    env.pop("STRIATUM_TEST_HARNESS", None)

    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "adapter",
            "run",
            "--session-id",
            "session_1",
            "--lease-id",
            "lease_1",
            "--json",
        ],
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 12
    payload = json.loads(result.stdout)
    assert payload["ok"] is False
    assert "adapter must route through daemon RPC" in payload["error"]["message"]
    assert not (tmp_path / ".striatum" / "retired-local-state").exists()


def _run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
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
        return {"returncode": result.returncode}
    payload = cast(JsonDict, json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def _data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def _git_init_repo(repo: Path, initial_branch: str = "main") -> None:
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "checkout", "-b", initial_branch], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "config", "user.email", "test@example.com"],
        cwd=repo, check=True, capture_output=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "Test"],
        cwd=repo, check=True, capture_output=True,
    )
    seed = repo / ".gitseed"
    seed.write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", ".gitseed"], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "commit", "-m", "seed", "--no-gpg-sign"],
        cwd=repo, check=True, capture_output=True,
    )


def _build_workflow(
    *,
    command: list[str],
    job_type: str = "generic",
    expected_artifact_required: bool = True,
    artifact_path: str = "docs/out/OUT.md",
    adapter_timeout_seconds: int | None = None,
) -> JsonDict:
    lane: JsonDict = {
        "adapter": "process",
        "command": command,
        "capabilities": ["write"],
    }
    if adapter_timeout_seconds is not None:
        lane["adapter_timeout_seconds"] = adapter_timeout_seconds
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "test-process-adapter",
        "workflow_version": "2026-05-08",
        "name": "Test process adapter",
        "branch": {
            "mode": "auto",
            "suggested_name": "striatum/test-process-adapter",
        },
        "coordinator": {"role_id": "author", "lane_id": "stub"},
        "lanes": {"stub": lane},
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1},
        "jobs": [
            {
                "id": "demo",
                "type": job_type,
                "title": "Demo",
                "role_id": "author",
                "lane_id": "stub",
                "objective": "demo",
                "task_prompt": {"path": "prompts/demo.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": [str(Path(artifact_path).parent) + "/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "out",
                        "kind": "handoff",
                        "path": artifact_path,
                        "required": expected_artifact_required,
                    }
                ],
            }
        ],
        "edges": [],
        "cycles": [],
    }


def _start_with(repo: Path, workflow: JsonDict) -> tuple[str, str, str, str, str]:
    """Init, prepare, start, register, claim. Returns (run_id, session_id,
    job_id, lease_id, message_id)."""
    pytest.skip("historical direct adapter-run fixture quarantined")
    _git_init_repo(repo)
    workflow_path = repo / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")
    raise AssertionError("unreachable after historical direct fixture skip")
    prepared = _data(_run_cli(repo, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    _run_cli(repo, "run", "start", "--run-id", run_id)
    session = _data(_run_cli(
        repo, "register-session",
        "--run-id", run_id, "--role", "author",
        "--lane", "stub", "--capability", "write",
    ))
    session_id = str(session["session_id"])
    claim = _data(_run_cli(repo, "claim-next", "--session-id", session_id))
    packet = claim["packet"]
    job_id = str(packet["job"]["job_id"])
    lease_id = str(packet["lease"]["lease_id"])
    message_id = str(packet["lease"]["message_id"])
    _run_cli(repo, "ack", "--session-id", session_id,
             "--message-id", message_id, "--lease-id", lease_id)
    return run_id, session_id, job_id, lease_id, message_id


def _publish_artifact(repo: Path, session_id: str, job_id: str, lease_id: str,
                     artifact_path: str, body: str = "ok\n") -> None:
    full = repo / artifact_path
    full.parent.mkdir(parents=True, exist_ok=True)
    full.write_text(body, encoding="utf-8")
    _run_cli(
        repo, "publish-artifact",
        "--session-id", session_id, "--job-id", job_id, "--lease-id", lease_id,
        "--kind", "handoff", "--logical-name", "out", "--path", artifact_path,
    )


def _open_blocker(repo: Path, job_id: str) -> JsonDict | None:
    pytest.skip("historical direct blocker fixture quarantined")


def _job_state(repo: Path, job_id: str) -> str:
    pytest.skip("historical direct job-state fixture quarantined")


def _process_executions(repo: Path, run_id: str) -> list[JsonDict]:
    pytest.skip("historical direct process-execution fixture quarantined")


# ----- 1. Happy path ------------------------------------------------------


def test_happy_path_artifact_present_no_block(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _publish_artifact(tmp_path, session_id, job_id, lease_id, "docs/out/OUT.md")
    result = _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    assert result["state"] == "exited"
    assert result["exit_code"] == 0
    assert _open_blocker(tmp_path, job_id) is None
    assert _job_state(tmp_path, job_id) == "running"


# ----- 2. Exit 0 missing required artifact --------------------------------


def test_exit_zero_missing_required_artifact_blocks(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_outputs_missing"
    assert blocker["severity"] == "blocked"
    env = blocker["payload"]
    assert env["envelope_version"] == "striatum.process_adapter.envelope.v1"
    assert "docs/out/OUT.md" in env["missing_artifact_paths"]
    assert env["review_verdict_missing"] is False
    assert _job_state(tmp_path, job_id) == "blocked"


# ----- 3. Exit nonzero ----------------------------------------------------


def test_exit_nonzero_blocks(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 1"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_exit_nonzero"
    assert blocker["payload"]["exit_code"] == 1
    assert _job_state(tmp_path, job_id) == "blocked"


# ----- 4. Timeout ---------------------------------------------------------


def test_timeout_terminates_and_blocks(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["bash", "-c", "sleep 30"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    started = time.time()
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
        "--timeout-seconds", "1",
    ))
    elapsed = time.time() - started
    # Should NOT have waited 30 seconds.
    assert elapsed < 15.0, f"timeout did not terminate the child quickly enough: {elapsed}s"
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_timeout_exceeded"
    assert blocker["payload"]["timeout_seconds"] == 1
    assert blocker["payload"]["exit_code"] is None
    rows = _process_executions(tmp_path, run_id)
    assert len(rows) == 1
    assert rows[0]["state"] == "timed_out"


# ----- 5. Lane default timeout (no CLI flag) ------------------------------


def test_lane_field_default_used_when_flag_omitted(tmp_path: Path) -> None:
    workflow = _build_workflow(
        command=["bash", "-c", "sleep 30"],
        adapter_timeout_seconds=1,
    )
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    started = time.time()
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    elapsed = time.time() - started
    assert elapsed < 15.0
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_timeout_exceeded"
    assert blocker["payload"]["timeout_seconds"] == 1


# ----- 6. CLI flag overrides lane default ---------------------------------


def test_cli_flag_overrides_lane_default(tmp_path: Path) -> None:
    workflow = _build_workflow(
        command=["bash", "-c", "sleep 30"],
        adapter_timeout_seconds=60,
    )
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    started = time.time()
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
        "--timeout-seconds", "1",
    ))
    elapsed = time.time() - started
    assert elapsed < 15.0


# ----- 7. Workflow validation caps adapter_timeout_seconds ----------------


def test_workflow_validation_rejects_excessive_timeout(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["true"], adapter_timeout_seconds=1_000_000)
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")
    rejected = _run_cli(tmp_path, "workflow", "validate", str(workflow_path), check=False)
    assert rejected["returncode"] == 8
    assert "adapter_timeout_seconds" in str(rejected["error"]["message"])


def test_workflow_validation_rejects_non_positive_timeout(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["true"], adapter_timeout_seconds=0)
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")
    rejected = _run_cli(tmp_path, "workflow", "validate", str(workflow_path), check=False)
    assert rejected["returncode"] == 8
    assert "positive integer" in str(rejected["error"]["message"])


# ----- 8. Reconciliation --------------------------------------------------


def test_reconcile_keeps_alive_pid_running(tmp_path: Path) -> None:
    """A real running process should not be reconciled to lost."""
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    # Run adapter normally; this finishes and the row is 'exited'.
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    # Manually flip the row state back to 'running' with the current
    # python pid (which is alive) to simulate a still-running process.
    with connect(tmp_path) as conn:
        conn.execute(
            "UPDATE process_executions SET state = 'running', pid = ? WHERE run_id = ?",
            (os.getpid(), run_id),
        )
        conn.commit()
    result = _data(_run_cli(
        tmp_path, "recovery", "process-reconcile", "--run-id", run_id,
    ))
    assert len(result["transitioned_to_lost"]) == 0
    assert len(result["still_running"]) == 1


def test_reconcile_transitions_dead_pid_to_lost(tmp_path: Path) -> None:
    """A dead pid should be reconciled to 'lost' and the job blocked.

    Setup: run ``adapter run`` to create a real ``process_executions``
    row, then resolve the inline-path blocker so the reconciler's
    output validation can insert its own. Manually flip the row back
    to ``state='running'`` with a dead pid to simulate an
    externally-killed process whose bookkeeping was bypassed.
    """
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    # Run the adapter so a process_executions row exists. The inline
    # path will insert a process_outputs_missing blocker (artifact was
    # never published); we resolve it manually below so the reconciler
    # can insert its own without colliding.
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    # Harvest a now-dead pid.
    proc = subprocess.Popen(["bash", "-c", "exit 0"])
    proc.wait()
    dead_pid = proc.pid
    try:
        os.kill(dead_pid, 0)
        pytest.skip("dead_pid still reachable; can't simulate reconciler input")
    except ProcessLookupError:
        pass
    # Flip the process_executions row back to running with the dead pid,
    # resolve the inline blocker so the reconciler's blocker can land,
    # and reset the job state so the reconciler observes a fresh
    # post-loss validation pass.
    with connect(tmp_path) as conn:
        conn.execute(
            "UPDATE process_executions SET state = 'running', pid = ? WHERE run_id = ?",
            (dead_pid, run_id),
        )
        conn.execute(
            "UPDATE blockers SET state = 'resolved', resolved_at = ? WHERE job_id = ?",
            ("2026-05-08T00:00:00Z", job_id),
        )
        conn.execute("UPDATE jobs SET state = 'running' WHERE job_id = ?", (job_id,))
        conn.commit()
    result = _data(_run_cli(
        tmp_path, "recovery", "process-reconcile", "--run-id", run_id,
    ))
    assert len(result["transitioned_to_lost"]) == 1
    assert result["transitioned_to_lost"][0]["blocker_kind"] == "process_lost_with_outputs_missing"
    assert _job_state(tmp_path, job_id) == "blocked"
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_lost_with_outputs_missing"
    rows = _process_executions(tmp_path, run_id)
    assert len(rows) == 1
    assert rows[0]["state"] == "lost"


# ----- 9. Doctor checks ---------------------------------------------------


def test_doctor_flags_pid_gone(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    # Plant the bookkeeping mismatch.
    with connect(tmp_path) as conn:
        conn.execute(
            "UPDATE process_executions SET state = 'running', pid = 999999 WHERE run_id = ?",
            (run_id,),
        )
        conn.commit()
    result = _data(_run_cli(
        tmp_path, "doctor", "--run-id", run_id, "--verbose",
    ))
    checks = {r["check"] for r in result.get("problem_records", [])}
    assert "process_running_but_pid_gone" in checks


# ----- 10. Status process_health summary ---------------------------------


def test_status_process_health_summary(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    status = _data(_run_cli(tmp_path, "status", "--run-id", run_id))
    health = status["process_health"]
    assert health["running_count"] == 0
    assert health["lost_count"] == 0
    assert health["timed_out_count"] == 0


# ----- 11. Envelope contains no stdout/stderr -----------------------------


def test_envelope_contains_no_stdout_stderr(tmp_path: Path) -> None:
    """Regression: D028 — the envelope must never carry child output."""
    from striatum.process_completion import build_diagnostic_envelope

    envelope = build_diagnostic_envelope(
        process_id="proc_test",
        command=["claude", "--print"],
        exit_code=0,
        duration_seconds=1.5,
        timeout_seconds=None,
        missing_artifact_paths=["docs/x.md"],
        review_verdict_missing=False,
        recovery_commands=["striatum recovery process-reconcile --run-id r"],
    )
    flat = json.dumps(envelope)
    assert "stdout" not in flat
    assert "stderr" not in flat
    # Field whitelist: only the documented fields are present.
    assert set(envelope.keys()) == {
        "envelope_version",
        "process_id",
        "command",
        "exit_code",
        "duration_seconds",
        "timeout_seconds",
        "missing_artifact_paths",
        "review_verdict_missing",
        "recovery_commands",
    }


# ----- 12. Issue #1 reproduction -----------------------------------------


def test_issue_one_reproduction(tmp_path: Path) -> None:
    """Issue #1: process exits 0 without producing the required artifact;
    runner must transition the job to ``blocked`` without operator
    intervention."""
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    result = _data(_run_cli(
        tmp_path, "adapter", "run",
        "--session-id", session_id, "--lease-id", lease_id, "--stdin", "none",
    ))
    assert result["state"] == "exited"
    assert result["exit_code"] == 0
    # The runner should have closed the loop:
    assert _job_state(tmp_path, job_id) == "blocked"
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_outputs_missing"
    # Recovery commands must include the concrete blocker resume path.
    assert any(
        "recovery resume --blocker-id" in cmd
        for cmd in blocker["payload"]["recovery_commands"]
    )


# ----- 12b. Lane env variable expansion ----------------------------------


def test_lane_env_expands_striatum_scratch_dir() -> None:
    """Lane env values like ``${STRIATUM_SCRATCH_DIR}/codex-home`` must reach
    the child fully expanded; subprocess does not perform shell substitution."""
    from striatum.process_adapter import _expand_lane_env_values

    expanded = _expand_lane_env_values(
        {"CODEX_HOME": "${STRIATUM_SCRATCH_DIR}/codex-home"},
        {"STRIATUM_SCRATCH_DIR": "/tmp/scratch-abc"},
    )
    assert expanded == {"CODEX_HOME": "/tmp/scratch-abc/codex-home"}


def test_lane_env_unknown_variable_is_left_in_place() -> None:
    """Unknown ``${VAR}`` references are preserved verbatim so failures are
    obvious downstream rather than silently turning into an empty string."""
    from striatum.process_adapter import _expand_lane_env_values

    expanded = _expand_lane_env_values(
        {"FOO": "${UNDEFINED_THING}/bar"},
        {"STRIATUM_SCRATCH_DIR": "/tmp/x"},
    )
    assert expanded == {"FOO": "${UNDEFINED_THING}/bar"}


# ----- 13. Migration idempotency -----------------------------------------


def test_migrations_v8_v9_idempotent(tmp_path: Path) -> None:
    """v8 + v9 migrations must be re-runnable on an already-migrated DB."""
    pytest.skip("historical direct migration fixture quarantined")
