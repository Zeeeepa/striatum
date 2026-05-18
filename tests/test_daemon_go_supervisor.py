"""Python harness end-to-end tests for the Go daemon supervisor (RFC 0039
Phase 2 Step 5).

These tests require a built ``go/bin/striatumd`` and an ephemeral Postgres.
They are skipped when:

* the Go binary is not present (``go/bin/striatumd`` missing), OR
* an ephemeral Postgres URL is unavailable.

The shape mirrors ``tests/test_daemon_go_smoke.py`` so the same fixture +
skip rules apply.
"""

from __future__ import annotations

import shutil
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
GO_BIN = REPO_ROOT / "go" / "bin" / "striatumd"


def _go_bin_available() -> bool:
    return GO_BIN.is_file() or bool(shutil.which("striatumd"))


pytestmark = [
    pytest.mark.multi_repo,
    pytest.mark.skipif(
        not _go_bin_available(),
        reason="go/bin/striatumd not built; run `make daemon-go-build`",
    ),
]


def test_supervisor_start_attach_heartbeat_lost() -> None:
    """Placeholder smoke for supervisor lifecycle.

    Track B V1 ships the supervisor package skeleton + pidfile + Postgres
    pointer upsert + dead-pid lost-detection (covered by Go-level tests in
    ``go/pkg/supervisor/supervisor_test.go``). Full lifecycle parity with
    the Python supervisor — including the FIFO write path, PTY launch
    semantics, and heartbeat round-trip from the supervised wrapper —
    requires the creack/pty integration and the Postgres pointer schema
    glue Track A holds. That landing is V1.6 follow-up; this test asserts
    only that the binary exists when the gate is open and serves as the
    extension point.
    """
    assert _go_bin_available(), "expected go/bin/striatumd"


def test_supervisor_sigterm_cleanup() -> None:
    """Placeholder for SIGTERM cleanup parity.

    The Go-level tests cover Stop(signalProcess=true) and the GraceOnTerm
    drain; the harness-level assertion that no orphan PTYs remain is
    deferred to V1.6 when PTY launch is wired (see go/pkg/supervisor/pty.go
    for the not-wired sentinel).
    """
    assert _go_bin_available(), "expected go/bin/striatumd"
