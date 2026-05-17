"""RFC 0010 V2 / HARNESS-001: verify supervised wrapper fixtures.

The wrappers under ``.striatum/bin`` implement RFC 0009's supervised-lane
contract for provider CLIs. These tests substitute provider-command stubs on
``$PATH`` so the suite does not depend on real agent binaries; the goal is to
verify wrapper loop semantics and non-interactive invocation flags.
"""

from __future__ import annotations

import fcntl
import json
import os
import shutil
import shlex
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import cast

import pytest

ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class WrapperCase:
    name: str
    command: str
    wrapper: Path
    expected_args: tuple[str, ...]


WRAPPERS = (
    WrapperCase(
        name="claude",
        command="claude",
        wrapper=ROOT / ".striatum" / "bin" / "claude-supervised-wrapper.sh",
        expected_args=(
            "--print",
            "--permission-mode",
            "acceptEdits",
            "--allowedTools",
            "Bash",
        ),
    ),
    WrapperCase(
        name="codex",
        command="codex",
        wrapper=ROOT / ".striatum" / "bin" / "codex-supervised-wrapper.sh",
        expected_args=(
            "exec",
            "--json",
            "--ephemeral",
            "--skip-git-repo-check",
            "--dangerously-bypass-approvals-and-sandbox",
            "-c",
            "approval_policy=never",
            "--ignore-user-config",
            "-",
        ),
    ),
    WrapperCase(
        name="gemini",
        command="gemini",
        wrapper=ROOT / ".striatum" / "bin" / "gemini-supervised-wrapper.sh",
        expected_args=(
            "--prompt",
            "-",
            "--output-format",
            "stream-json",
            "--approval-mode",
            "yolo",
        ),
    ),
)


pytestmark = [
    pytest.mark.skipif(
        shutil.which("bash") is None, reason="bash not available"
    )
]


@pytest.fixture(
    params=[
        pytest.param(
            case,
            id=case.name,
            marks=pytest.mark.skipif(
                not case.wrapper.exists(),
                reason=f"{case.wrapper.name} not present in this checkout",
            ),
        )
        for case in WRAPPERS
    ]
)
def wrapper_case(request: pytest.FixtureRequest) -> WrapperCase:
    return cast(WrapperCase, request.param)


def _stub_command(
    tmp_path: Path, case: WrapperCase, *, exit_code: int = 0
) -> tuple[Path, Path]:
    """Install a provider-command stub on $PATH.

    Returns ``(stub_dir, log_path)``. Each stub invocation appends its
    argv, stdin, and a ``---END---`` marker to ``log_path``.
    """
    log = tmp_path / f"{case.name}.log"
    stub = tmp_path / case.command
    body = (
        "#!/usr/bin/env bash\n"
        "{\n"
        "  printf 'ARGV:'\n"
        "  for arg in \"$@\"; do\n"
        "    printf ' [%s]' \"$arg\"\n"
        "  done\n"
        "  printf '\\nSTDIN:\\n'\n"
        "  cat\n"
        "  printf '\\n---END---\\n'\n"
        f"}} >> {shlex.quote(str(log))}\n"
        f"exit {exit_code}\n"
    )
    stub.write_text(body, encoding="utf-8")
    stub.chmod(0o755)
    return tmp_path, log


def _spawn_wrapper(
    fifo: Path,
    *,
    case: WrapperCase,
    env_path_prefix: Path,
    scratch_dir: Path,
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
    env["STRIATUM_SCRATCH_DIR"] = str(scratch_dir)
    scratch_dir.mkdir()
    read_fd = os.open(str(fifo), os.O_RDONLY | os.O_NONBLOCK)
    try:
        flags = fcntl.fcntl(read_fd, fcntl.F_GETFL)
        fcntl.fcntl(read_fd, fcntl.F_SETFL, flags & ~os.O_NONBLOCK)
        proc = subprocess.Popen(
            [str(case.wrapper)],
            stdin=read_fd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            env=env,
            cwd=scratch_dir.parent,
        )
    finally:
        # The child has its own copy of the fd; release the parent's.
        os.close(read_fd)
    return proc


def test_wrapper_handles_multiple_packets(
    tmp_path: Path, wrapper_case: WrapperCase
) -> None:
    """The wrapper must spawn one provider process per packet."""
    stub_dir, log = _stub_command(tmp_path, wrapper_case)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(
        fifo,
        case=wrapper_case,
        env_path_prefix=stub_dir,
        scratch_dir=tmp_path / "scratch",
    )
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
    for arg in wrapper_case.expected_args:
        assert f"[{arg}]" in contents


def test_wrapper_survives_failing_inner_command(
    tmp_path: Path, wrapper_case: WrapperCase
) -> None:
    """A non-zero provider exit must not kill the wrapper loop."""
    stub_dir, log = _stub_command(tmp_path, wrapper_case, exit_code=1)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(
        fifo,
        case=wrapper_case,
        env_path_prefix=stub_dir,
        scratch_dir=tmp_path / "scratch",
    )
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


def test_wrapper_exits_cleanly_on_writer_eof(
    tmp_path: Path, wrapper_case: WrapperCase
) -> None:
    """Closing the writer side without sending anything must exit 0."""
    stub_dir, _log = _stub_command(tmp_path, wrapper_case)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(
        fifo,
        case=wrapper_case,
        env_path_prefix=stub_dir,
        scratch_dir=tmp_path / "scratch",
    )
    try:
        # Open and immediately close the writer side without sending
        # any packets -- the empty-input case.
        open(fifo, "wb").close()
        rc = proc.wait(timeout=5)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0


def test_wrapper_exits_cleanly_after_one_packet_then_eof(
    tmp_path: Path, wrapper_case: WrapperCase
) -> None:
    """Send one packet, then close the writer; wrapper must exit 0.

    Design-review F3: distinguishes "sent some, then closed" from the
    pure-empty case in the test above. Catches a class of bug where the
    loop completes a packet but does not return to ``read`` before the
    EOF arrives.
    """
    stub_dir, log = _stub_command(tmp_path, wrapper_case)
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    proc = _spawn_wrapper(
        fifo,
        case=wrapper_case,
        env_path_prefix=stub_dir,
        scratch_dir=tmp_path / "scratch",
    )
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
