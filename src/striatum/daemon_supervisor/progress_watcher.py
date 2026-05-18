"""Supervised-process progress watcher.

The watcher is intentionally small and injectable: production callers can run
``watch_progress`` in a daemon task, while tests can exercise one polling tick
without sleeping. It polls file metadata only and never reads log contents.
"""

from __future__ import annotations

import fcntl
import os
import threading
import time
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterator

from striatum.repo_policy import state_dir


HeartbeatCallback = Callable[..., object]
IdleCallback = Callable[[str], object]
LostCallback = Callable[[str], object]

_LOCKS: dict[str, threading.Lock] = {}
_LOCKS_GUARD = threading.Lock()


@dataclass(frozen=True)
class ProgressWatcherConfig:
    poll_interval_seconds: float = 30.0
    refresh_threshold_seconds: float = 60.0
    idle_threshold_seconds: float = 600.0
    heartbeat_extend_seconds: int = 900

    @property
    def poll_seconds(self) -> float:
        return self.poll_interval_seconds

    @property
    def active_window_seconds(self) -> float:
        return self.refresh_threshold_seconds


@dataclass(frozen=True)
class SupervisedProgressTarget:
    supervisor_id: str
    session_id: str
    pid: int
    log_path: Path | None = None
    lease_id: str | None = None
    scratch_path: Path | None = None
    run_id: str | None = None
    state: str = "attached"
    job_id: str | None = None


SupervisorRecord = SupervisedProgressTarget


@dataclass(frozen=True)
class ActiveLease:
    lease_id: str
    job_id: str


@dataclass(frozen=True)
class ProgressWatcherResult:
    supervisor_id: str
    status: str
    observed_mtime: float | None = None
    age_seconds: float | None = None
    lease_id: str | None = None
    job_id: str | None = None
    heartbeat_called: bool = False
    warning: str | None = None


@dataclass(frozen=True)
class ProgressCheckResult:
    status: str
    log_mtime: float | None = None
    age_seconds: float | None = None


def newest_log_mtime(*, scratch_path: Path, log_path: Path | None = None) -> float | None:
    """Return the newest relevant ``*.log`` mtime without reading contents."""
    paths: list[Path]
    if log_path is not None:
        paths = [log_path]
    else:
        paths = list(scratch_path.rglob("*.log")) if scratch_path.exists() else []

    newest: float | None = None
    for path in paths:
        try:
            mtime = os.stat(path).st_mtime
        except FileNotFoundError:
            continue
        if newest is None or mtime > newest:
            newest = mtime
    return newest


def pid_alive(pid: int) -> bool:
    return process_alive(pid)


def process_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def progress_lock_path(repo: Path, *, job_id: str) -> Path:
    safe_job_id = job_id.replace("/", "_")
    return state_dir(repo) / "scratch" / "locks" / f"progress-{safe_job_id}.lock"


@contextmanager
def progress_advisory_lock(
    repo: Path, *, job_id: str, blocking: bool = False
) -> Iterator[bool]:
    """Acquire the per-job guard shared by progress refresh and surgical recovery."""
    path = progress_lock_path(repo, job_id=job_id)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(path, os.O_RDWR | os.O_CREAT, 0o600)
    acquired = False
    try:
        flags = fcntl.LOCK_EX
        if not blocking:
            flags |= fcntl.LOCK_NB
        try:
            fcntl.flock(fd, flags)
            acquired = True
        except BlockingIOError:
            yield False
        else:
            yield True
    finally:
        if acquired:
            fcntl.flock(fd, fcntl.LOCK_UN)
        os.close(fd)


class SupervisedProgressWatcher:
    def __init__(
        self,
        *,
        heartbeat_callback: HeartbeatCallback,
        conn: Any | None = None,
        repo: Path | None = None,
        config: ProgressWatcherConfig | None = None,
        clock: Callable[[], float] = time.time,
        warning_callback: Callable[[ProgressWatcherResult], None] | None = None,
    ) -> None:
        self.heartbeat_callback = heartbeat_callback
        self.conn = conn
        self.repo = repo
        self.config = config or ProgressWatcherConfig()
        self.clock = clock
        self.warning_callback = warning_callback

    def tick(self, supervisor: SupervisorRecord) -> ProgressWatcherResult:
        if supervisor.state != "attached":
            return ProgressWatcherResult(supervisor.supervisor_id, "inactive_supervisor")
        if not process_alive(supervisor.pid):
            return ProgressWatcherResult(
                supervisor.supervisor_id,
                "process_gone",
                warning="supervised process is not alive",
            )

        scratch_path = supervisor.scratch_path or (
            supervisor.log_path.parent if supervisor.log_path is not None else Path(".")
        )
        mtime = newest_log_mtime(scratch_path=scratch_path, log_path=supervisor.log_path)
        if mtime is None:
            return ProgressWatcherResult(
                supervisor.supervisor_id,
                "no_log",
                warning="no supervised progress log found",
            )

        age = max(0.0, self.clock() - mtime)
        lease = self._active_lease(supervisor)
        if lease is None:
            return ProgressWatcherResult(
                supervisor.supervisor_id,
                "no_active_lease",
                observed_mtime=mtime,
                age_seconds=age,
            )

        if age <= self.config.refresh_threshold_seconds:
            if self.repo is not None:
                with progress_advisory_lock(self.repo, job_id=lease.job_id) as acquired:
                    if not acquired:
                        return ProgressWatcherResult(
                            supervisor.supervisor_id,
                            "lock_busy",
                            observed_mtime=mtime,
                            age_seconds=age,
                            lease_id=lease.lease_id,
                            job_id=lease.job_id,
                        )
                    self._heartbeat(supervisor.session_id, lease.lease_id)
            else:
                self._heartbeat(supervisor.session_id, lease.lease_id)
            return ProgressWatcherResult(
                supervisor.supervisor_id,
                "heartbeat",
                observed_mtime=mtime,
                age_seconds=age,
                lease_id=lease.lease_id,
                job_id=lease.job_id,
                heartbeat_called=True,
            )

        if age > self.config.idle_threshold_seconds:
            result = ProgressWatcherResult(
                supervisor.supervisor_id,
                "idle",
                observed_mtime=mtime,
                age_seconds=age,
                lease_id=lease.lease_id,
                job_id=lease.job_id,
                warning="supervised progress log is idle",
            )
            if self.warning_callback is not None:
                self.warning_callback(result)
            return result

        return ProgressWatcherResult(
            supervisor.supervisor_id,
            "no_recent_progress",
            observed_mtime=mtime,
            age_seconds=age,
            lease_id=lease.lease_id,
            job_id=lease.job_id,
        )

    def _heartbeat(self, session_id: str, lease_id: str) -> None:
        try:
            self.heartbeat_callback(
                session_id=session_id,
                lease_id=lease_id,
                extend_seconds=self.config.heartbeat_extend_seconds,
            )
        except TypeError:
            self.heartbeat_callback(session_id, lease_id)

    def _active_lease(self, supervisor: SupervisorRecord) -> ActiveLease | None:
        if self.conn is None:
            if supervisor.lease_id is None:
                return None
            return ActiveLease(supervisor.lease_id, supervisor.job_id or supervisor.lease_id)

        params: list[str] = [supervisor.session_id]
        lease_clause = ""
        if supervisor.lease_id is not None:
            lease_clause = "AND lease_id = ?"
            params.append(supervisor.lease_id)
        row = self.conn.execute(
            f"""
            SELECT lease_id, resource_id
            FROM leases
            WHERE owner_session_id = ?
              AND state = 'active'
              AND resource_type = 'job'
              {lease_clause}
            ORDER BY acquired_at DESC
            LIMIT 1
            """,
            params,
        ).fetchone()
        if row is None:
            return None
        return ActiveLease(str(row["lease_id"]), str(row["resource_id"]))


def check_progress_once(
    target: SupervisedProgressTarget,
    *,
    heartbeat: HeartbeatCallback,
    config: ProgressWatcherConfig = ProgressWatcherConfig(),
    now: float | None = None,
    on_idle: IdleCallback | None = None,
    on_lost: LostCallback | None = None,
) -> ProgressCheckResult:
    """Inspect one supervisor log path and heartbeat when progress is fresh."""
    observed_now = time.time() if now is None else now
    if not process_alive(target.pid):
        if on_lost is not None:
            on_lost(target.supervisor_id)
        return ProgressCheckResult("process_lost")

    if target.log_path is None:
        return ProgressCheckResult("log_missing")
    try:
        stat = os.stat(target.log_path)
    except FileNotFoundError:
        return ProgressCheckResult("log_missing")

    age = max(0.0, observed_now - stat.st_mtime)
    if age <= config.refresh_threshold_seconds:
        lock = _lock_for(target.supervisor_id)
        if not lock.acquire(blocking=False):
            return ProgressCheckResult("busy", stat.st_mtime, age)
        try:
            heartbeat(target.session_id, target.lease_id)
        finally:
            lock.release()
        return ProgressCheckResult("heartbeat", stat.st_mtime, age)

    if age > config.idle_threshold_seconds:
        if on_idle is not None:
            on_idle(target.supervisor_id)
        return ProgressCheckResult("idle", stat.st_mtime, age)
    return ProgressCheckResult("quiet", stat.st_mtime, age)


def watch_progress(
    target: SupervisedProgressTarget,
    *,
    heartbeat: HeartbeatCallback,
    config: ProgressWatcherConfig = ProgressWatcherConfig(),
    stop_event: threading.Event | None = None,
    on_idle: IdleCallback | None = None,
    on_lost: LostCallback | None = None,
) -> None:
    """Poll until stopped or until the supervised process is lost."""
    stopper = stop_event or threading.Event()
    while not stopper.is_set():
        result = check_progress_once(
            target,
            heartbeat=heartbeat,
            config=config,
            on_idle=on_idle,
            on_lost=on_lost,
        )
        if result.status == "process_lost":
            return
        stopper.wait(config.poll_interval_seconds)


def _lock_for(supervisor_id: str) -> threading.Lock:
    with _LOCKS_GUARD:
        lock = _LOCKS.get(supervisor_id)
        if lock is None:
            lock = threading.Lock()
            _LOCKS[supervisor_id] = lock
        return lock


__all__ = [
    "ActiveLease",
    "HeartbeatCallback",
    "ProgressCheckResult",
    "ProgressWatcherConfig",
    "ProgressWatcherResult",
    "SupervisorRecord",
    "SupervisedProgressTarget",
    "SupervisedProgressWatcher",
    "check_progress_once",
    "newest_log_mtime",
    "pid_alive",
    "process_alive",
    "progress_advisory_lock",
    "progress_lock_path",
    "watch_progress",
]
