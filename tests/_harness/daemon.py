"""Daemon process helpers for multi-repo tests."""

from __future__ import annotations

import os
import signal
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from types import FrameType

from striatum import daemon

ROOT = Path(__file__).resolve().parents[2]


@dataclass
class DaemonProcess:
    scratch_dir: Path
    postgres_url: str
    env: dict[str, str] = field(init=False)
    process: subprocess.Popen[str] | None = field(default=None, init=False)

    def __post_init__(self) -> None:
        registry = self.scratch_dir / "daemon-registry.sqlite3"
        runtime = self.scratch_dir / "runtime"
        self.env = os.environ.copy()
        self.env["PYTHONPATH"] = str(ROOT / "src")
        self.env[daemon.ENV_REGISTRY] = str(registry)
        self.env[daemon.ENV_RUNTIME] = str(runtime)

    @property
    def socket_path(self) -> Path:
        return Path(self.env[daemon.ENV_RUNTIME]) / "striatumd.sock"

    def start(self) -> None:
        if self.process is not None and self.process.poll() is None:
            raise RuntimeError("daemon process is already running")
        self.socket_path.parent.mkdir(parents=True, exist_ok=True)
        self.process = subprocess.Popen(
            [
                sys.executable,
                "-m",
                "striatum.cli",
                "daemon",
                "start",
                "--sweep-interval-seconds",
                "3600",
                "--json",
            ],
            cwd=ROOT,
            env=self.env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            if self.process.poll() is not None:
                stdout, stderr = self.process.communicate(timeout=1)
                raise RuntimeError(f"daemon exited during startup\nstdout={stdout}\nstderr={stderr}")
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
