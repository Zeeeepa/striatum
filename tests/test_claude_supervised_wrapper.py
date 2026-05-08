"""RFC 0010 V2 / HARNESS-001: verify the Claude Code supervised wrapper.

The wrapper at ``.striatum/bin/claude-supervised-wrapper.sh`` is the
reference implementation of RFC 0009's supervised-lane contract for
Claude Code. These tests substitute a stub ``claude`` on ``$PATH`` so
the suite does not depend on the real Claude binary; the goal is to
verify the wrapper's loop semantics, not the inner agent.
"""

from __future__ import annotations

import fcntl
import json
import os
import shutil
import subprocess
import time
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
WRAPPER = ROOT / ".striatum" / "bin" / "claude-supervised-wrapper.sh"


pytestmark = [
    pytest.mark.skipif(
        shutil.which("bash") is None, reason="bash not available"
    ),
    pytest.mark.skipif(
        not WRAPPER.exists(),
        reason="claude-supervised-wrapper.sh not present in this checkout",
    ),
]


def _stub_claude(tmp_path: Path, *, exit_code: int = 0) -> tuple[Path, Path]:
    """Install a stub `claude` on $PATH that records its stdin per call.

    Returns ``(stub_dir, log_path)``. Each stub invocation appends its
    stdin and a ``---END---`` marker to ``log_path``.
    """
    log = tmp_path / "claude.log"
    stub = tmp_path / "claude"
    body = (
        "#!/usr/bin/env bash\n"
        f"cat >> {log}\n"
        f"printf '\\n---END---\\n' >> {log}\n"
        f"exit {exit_code}\n"
    )
    stub.write_text(body, encoding="utf-8")
    stub.chmod(0o755)
    return tmp_path, log


def _spawn_wrapper(
    fifo: Path, *, env_path_prefix: Path
) -> subprocess.Popen[bytes]:
    """Spawn the wrapper with ``fifo`` as stdin without deadlocking.

    Opening a FIFO for read normally blocks until a writer connects,
    which would deadlock the test before the wrapper even starts.
    The ``O_NONBLOCK`` open returns immediately; clearing the flag
    afterward restores normal blocking-read semantics for bash's
    ``read`` builtin inside the child.
    """
    env = os.environ.copy()
    env["PATH"] = f"{env_path_prefix}:{env['PATH']}"
    read_fd = os.open(str(fifo), os.O_RDONLY | os.O_NONBLOCK)
    try:
        flags = fcntl.fcntl(read_fd, fcntl.F_GETFL)
        fcntl.fcntl(read_fd, fcntl.F_SETFL, flags & ~os.O_NONBLOCK)
        proc = subprocess.Popen(
            [str(WRAPPER)],
            stdin=read_fd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            env=env,
        )
    finally:
        # The child has its own copy of the fd; release the parent's.
        os.close(read_fd)
    return proc


def test_wrapper_handles_multiple_packets(tmp_path: Path) -> None:
    """The wrapper must spawn one `claude` per newline-terminated packet."""
    stub_dir, log = _stub_claude(tmp_path)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(fifo, env_path_prefix=stub_dir)
    try:
        with open(fifo, "wb") as writer:
            for i in range(3):
                writer.write((json.dumps({"i": i}) + "\n").encode())
                writer.flush()
                time.sleep(0.05)
        rc = proc.wait(timeout=10)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0
    contents = log.read_text(encoding="utf-8")
    assert contents.count("---END---") == 3, contents
    for i in range(3):
        assert json.dumps({"i": i}) in contents


def test_wrapper_survives_failing_inner_claude(tmp_path: Path) -> None:
    """A non-zero `claude` exit must not kill the wrapper loop."""
    stub_dir, log = _stub_claude(tmp_path, exit_code=1)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(fifo, env_path_prefix=stub_dir)
    try:
        with open(fifo, "wb") as writer:
            for i in range(2):
                writer.write((json.dumps({"i": i}) + "\n").encode())
                writer.flush()
                time.sleep(0.05)
        rc = proc.wait(timeout=10)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0
    assert log.read_text(encoding="utf-8").count("---END---") == 2


def test_wrapper_exits_cleanly_on_writer_eof(tmp_path: Path) -> None:
    """Closing the writer side without sending anything must exit 0."""
    stub_dir, _log = _stub_claude(tmp_path)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(fifo, env_path_prefix=stub_dir)
    try:
        # Open and immediately close the writer side without sending
        # any packets — the empty-input case.
        open(fifo, "wb").close()
        rc = proc.wait(timeout=5)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0


def test_wrapper_exits_cleanly_after_one_packet_then_eof(tmp_path: Path) -> None:
    """Send one packet, then close the writer; wrapper must exit 0.

    Design-review F3: distinguishes "sent some, then closed" from the
    pure-empty case in the test above. Catches a class of bug where the
    loop completes a packet but does not return to ``read`` before the
    EOF arrives.
    """
    stub_dir, log = _stub_claude(tmp_path)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(fifo, env_path_prefix=stub_dir)
    try:
        with open(fifo, "wb") as writer:
            writer.write((json.dumps({"only": True}) + "\n").encode())
            writer.flush()
            time.sleep(0.1)
        rc = proc.wait(timeout=5)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0
    contents = log.read_text(encoding="utf-8")
    assert contents.count("---END---") == 1, contents
    assert json.dumps({"only": True}) in contents
