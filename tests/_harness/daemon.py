"""Daemon process helpers for multi-repo tests."""

from __future__ import annotations

import os
import shutil
import signal
import subprocess
import time
from dataclasses import dataclass, field
from pathlib import Path
from types import FrameType
from typing import Literal

import striatum.daemon as daemon

ROOT = Path(__file__).resolve().parents[2]

DaemonCore = Literal["go"]

_GO_BIN_ENV = "STRIATUMD_GO_BIN"
_DEFAULT_GO_BIN = ROOT / "go" / "bin" / "striatumd"
_GO_DIR = ROOT / "go"


def _resolve_go_binary() -> Path:
    """Locate the Go daemon binary, building it via `make -C go build` if missing."""
    override = os.environ.get(_GO_BIN_ENV)
    if override:
        binary = Path(override)
        if not binary.exists():
            raise RuntimeError(
                f"{_GO_BIN_ENV}={override} but the binary is not present on disk"
            )
        return binary
    binary = _DEFAULT_GO_BIN
    if binary.exists():
        return binary
    if not _GO_DIR.is_dir():
        raise RuntimeError(
            f"Go daemon source tree is missing at {_GO_DIR}; cannot build striatumd"
        )
    if shutil.which("make") is None or shutil.which("go") is None:
        raise RuntimeError(
            "make and go must be available to build the striatumd binary; "
            f"set {_GO_BIN_ENV} to a prebuilt binary to skip the build step"
        )
    subprocess.run(
        ["make", "-C", str(_GO_DIR), "build"],
        check=True,
        cwd=ROOT,
    )
    if not binary.exists():
        raise RuntimeError(
            f"`make -C {_GO_DIR} build` completed but {binary} is missing"
        )
    return binary


@dataclass
class DaemonProcess:
    scratch_dir: Path
    postgres_url: str
    daemon_core: DaemonCore = "go"
    env: dict[str, str] = field(init=False)
    process: subprocess.Popen[str] | None = field(default=None, init=False)
    _socket_path: Path = field(init=False)

    def __post_init__(self) -> None:
        if self.daemon_core != "go":
            raise ValueError(f"unknown daemon_core: {self.daemon_core!r}")
        runtime = self.scratch_dir / "runtime"
        registry = self.scratch_dir / "daemon-registry.sqlite3"
        self.env = os.environ.copy()
        self.env["PYTHONPATH"] = str(ROOT / "src")
        self.env[daemon.ENV_REGISTRY] = str(registry)
        self.env[daemon.ENV_RUNTIME] = str(runtime)
        # The test harness creates ephemeral databases as the local DB owner;
        # the daemon connects as that same owner, which has implicit
        # UPDATE/DELETE on the audit tables that the production doctor refuses
        # as `unsafe_privileges`. The opt-in below tells doctor to treat that
        # as an acceptable harness-only state. Production deployments never
        # set this; see src/striatum/daemon_pg/connection.py::doctor.
        self.env["STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK"] = "1"
        self._socket_path = runtime / "striatumd.sock"

    @property
    def socket_path(self) -> Path:
        return self._socket_path

    def start(self) -> None:
        if self.process is not None and self.process.poll() is None:
            raise RuntimeError("daemon process is already running")
        self.socket_path.parent.mkdir(parents=True, exist_ok=True)
        self._start_go()

    def _start_go(self) -> None:
        binary = _resolve_go_binary()
        migrations_dir = ROOT / "src" / "striatum" / "daemon_pg" / "sql"
        cmd = [
            str(binary),
            "--socket",
            str(self.socket_path),
            "--postgres-url",
            self.postgres_url,
            "--migrations-sha-source",
            str(migrations_dir),
        ]
        self.process = subprocess.Popen(
            cmd,
            cwd=ROOT,
            env=self.env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        self._wait_for_socket()

    def _wait_for_socket(self) -> None:
        proc = self.process
        assert proc is not None
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            if proc.poll() is not None:
                stdout, stderr = proc.communicate(timeout=1)
                raise RuntimeError(
                    f"daemon ({self.daemon_core}) exited during startup\n"
                    f"stdout={stdout}\nstderr={stderr}"
                )
            if self.socket_path.exists():
                return
            time.sleep(0.05)
        raise RuntimeError("daemon socket did not appear before startup timeout")

    def stop(self) -> None:
        proc = self.process
        if proc is None:
            return
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=5)
        self.process = None

    def kill(self, sig: signal.Signals = signal.SIGKILL) -> None:
        proc = self.process
        if proc is None or proc.poll() is not None:
            return
        proc.send_signal(sig)
        proc.wait(timeout=5)
        self.process = None


class PauseHook:
    """No-op deterministic hook placeholder until the daemon accept loop matures."""

    def __init__(self, stage: str) -> None:
        self.stage = stage

    def __enter__(self) -> "PauseHook":
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: FrameType | None,
    ) -> None:
        return None
