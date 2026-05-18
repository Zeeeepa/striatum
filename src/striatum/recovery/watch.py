"""RFC 0020 step 3: long-lived ``recovery watch`` scheduler.

Production ``recovery watch`` keeps the pidfile, signal handling, sleep loop,
and JSONL stream in the foreground CLI process while every state mutation goes
through daemon RPC ``recovery.sweep``.

The watcher contributes scheduling, signal handling, and output formatting,
nothing else.
"""

from __future__ import annotations

import errno
import json
import os
import signal
import sys
import threading
from pathlib import Path
from typing import Any, Callable, Mapping, TextIO

from striatum.primitives import utc_now

TERMINAL_RUN_STATES: frozenset[str] = frozenset(
    {"completed", "failed", "canceled"}
)
PIDFILE_COLLISION_EXIT_CODE = 4
DaemonMethodCaller = Callable[[Path, str, Mapping[str, Any]], dict[str, Any]]


def pidfile_path(repo: Path, run_id: str) -> Path:
    """Return the pidfile path for a watcher on ``run_id``."""
    return repo.resolve() / ".striatum" / "scratch" / f"recovery-watch-{run_id}.pid"


def run_daemon_watch(
    repo: Path,
    *,
    run_id: str,
    interval_seconds: float = 60.0,
    exit_on_terminal: bool = True,
    max_sweeps: int | None = None,
    cli_overrides: Mapping[str, Any] | None = None,
    json_output: bool = False,
    stdout: TextIO | None = None,
    now: Callable[[], str] = utc_now,
    install_signal_handlers: bool = True,
    call_method: DaemonMethodCaller | None = None,
) -> int:
    """Run daemon-backed recovery sweeps until terminal / max_sweeps / signal.

    This is the production daemon-required watcher. It keeps scheduling,
    pidfile, signal, and JSONL behavior in the foreground CLI process while
    sending each state mutation through daemon RPC ``recovery.sweep``.
    """
    if call_method is None:
        from striatum.service_daemon import call_repo_method

        call_method = call_repo_method
    out = stdout if stdout is not None else sys.stdout
    pidfile = pidfile_path(repo, run_id)
    acquired = _acquire_pidfile(pidfile)
    if not acquired:
        existing = _read_pidfile(pidfile)
        message = (
            f"another recovery watch is active for {run_id} "
            f"(pid {existing if existing is not None else '<unknown>'})"
        )
        _emit_line(
            out,
            json_output,
            {
                "event": "watch_collision",
                "run_id": run_id,
                "pid": existing,
                "message": message,
            },
            human=f"recovery watch refused: {message}",
        )
        return PIDFILE_COLLISION_EXIT_CODE

    stop_event = threading.Event()

    def _handler(_signum: int, _frame: object) -> None:
        stop_event.set()

    if install_signal_handlers:
        try:
            signal.signal(signal.SIGTERM, _handler)
            signal.signal(signal.SIGINT, _handler)
        except (ValueError, OSError):
            pass

    swept = 0
    exit_reason = "clean"
    sweep_params = _daemon_sweep_params(run_id=run_id, cli_overrides=cli_overrides)
    try:
        while not stop_event.is_set():
            envelope = call_method(repo, "recovery.sweep", sweep_params)
            _emit_envelope(out, json_output, envelope)
            swept += 1

            if exit_on_terminal and _sweep_envelope_terminal(envelope):
                exit_reason = "run_terminal"
                break

            if max_sweeps is not None and swept >= max_sweeps:
                exit_reason = "max_sweeps_reached"
                break

            if stop_event.wait(interval_seconds):
                exit_reason = "signal"
                break
    except Exception:
        exit_reason = "error"
        raise
    finally:
        try:
            pidfile.unlink()
        except FileNotFoundError:
            pass
        _emit_line(
            out,
            json_output,
            {
                "event": "watch_exit",
                "run_id": run_id,
                "reason": exit_reason,
                "swept_total": swept,
                "exited_at": now(),
            },
            human=f"recovery watch exit: reason={exit_reason} swept={swept}",
        )
    return 0


def _daemon_sweep_params(
    *,
    run_id: str,
    cli_overrides: Mapping[str, Any] | None,
) -> dict[str, Any]:
    params: dict[str, Any] = {"run_id": run_id}
    for key, value in dict(cli_overrides or {}).items():
        if value is not None:
            params[key] = value
    return params


def _sweep_envelope_terminal(envelope: Mapping[str, Any]) -> bool:
    return str(envelope.get("run_state") or "") in TERMINAL_RUN_STATES


def _acquire_pidfile(pidfile: Path) -> bool:
    """Attempt O_EXCL pidfile open; clean a stale pidfile and retry."""
    pidfile.parent.mkdir(parents=True, exist_ok=True)
    for _ in range(2):
        try:
            fd = os.open(str(pidfile), os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o644)
        except FileExistsError:
            existing = _read_pidfile(pidfile)
            if existing is None or not _pid_alive(existing):
                # Stale; remove and retry.
                try:
                    pidfile.unlink()
                except FileNotFoundError:
                    pass
                continue
            return False
        else:
            try:
                os.write(fd, f"{os.getpid()}\n".encode("ascii"))
            finally:
                os.close(fd)
            return True
    return False


def _read_pidfile(pidfile: Path) -> int | None:
    try:
        text = pidfile.read_text(encoding="ascii").strip()
    except (FileNotFoundError, OSError):
        return None
    try:
        return int(text)
    except ValueError:
        return None


def _pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        # Process exists but is owned by someone else; treat as alive.
        return True
    except OSError as exc:
        if exc.errno == errno.ESRCH:
            return False
        return True
    return True


def _emit_envelope(stdout: TextIO, json_output: bool, envelope: dict[str, Any]) -> None:
    if json_output:
        stdout.write(json.dumps(envelope) + "\n")
    else:
        actions = len(envelope.get("actions", []))
        escalations = len(envelope.get("escalations", []))
        still_stuck = len(envelope.get("still_stuck", []))
        stdout.write(
            f"swept_at={envelope.get('swept_at', '')} "
            f"actions={actions} escalations={escalations} "
            f"still_stuck={still_stuck}\n"
        )
    stdout.flush()


def _emit_line(
    stdout: TextIO, json_output: bool, payload: dict[str, Any], *, human: str
) -> None:
    if json_output:
        stdout.write(json.dumps(payload) + "\n")
    else:
        stdout.write(human + "\n")
    stdout.flush()


__all__ = [
    "PIDFILE_COLLISION_EXIT_CODE",
    "TERMINAL_RUN_STATES",
    "pidfile_path",
    "run_daemon_watch",
]
