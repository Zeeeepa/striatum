"""RFC 0043 §3 daemon-required CLI plumbing.

V1.5: daemon-required enforcement is the default. The two refusal paths:

- :data:`EXIT_DAEMON_UNREACHABLE` (11) — raised when the daemon socket
  cannot be reached. Stderr names the socket path tried plus platform
  remediation (Linux systemd, macOS launchd, ``striatumd --foreground``,
  Postgres install hint). The JSON error envelope carries a short ``hint``.
- :data:`EXIT_REPO_NOT_MIGRATED` (12) — raised when the daemon is reachable
  but the target repo has no ``repo_migrations`` row yet. Stderr names
  ``striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>``.

Opt-out: V1.6 narrows this to test-only. Setting
:envvar:`STRIATUM_DAEMON_REQUIRED` ``= "0"`` opts out **only when paired
with** :envvar:`STRIATUM_TEST_HARNESS` ``= "1"`` (set by
``tests/conftest.py``). The bare env var no longer bypasses
enforcement — closes the codex threat-model finding from dogfood-050
that flagged the documented operator escape path.
"""

from __future__ import annotations

import os
import socket
import sys
from dataclasses import dataclass
from pathlib import Path

from striatum.errors import DaemonUnreachableError, RepoNotMigratedError

ENV_DAEMON_REQUIRED = "STRIATUM_DAEMON_REQUIRED"
ENV_DAEMON_SOCKET = "STRIATUM_DAEMON_SOCKET"
ENV_TEST_HARNESS = "STRIATUM_TEST_HARNESS"

# Commands that legitimately run without a reachable daemon. ``--help`` and
# ``--version`` short-circuit inside argparse and never enter dispatch.
# ``daemon`` lifecycle commands manage the daemon itself; ``init`` is the
# bootstrap path; ``skills`` and ``plugin`` touch installer files only.
DAEMON_OPTIONAL_COMMANDS: frozenset[str] = frozenset(
    {
        "daemon",
        "init",
        "skills",
        "plugin",
        "self-update",
    }
)


@dataclass(frozen=True)
class DaemonRequirement:
    """Resolved daemon-required policy for this CLI invocation."""

    enforced: bool
    socket_path: Path


def resolve_requirement(command: str | None) -> DaemonRequirement | None:
    """Decide whether the daemon-required check applies to *command*.

    V1.6: enforcement is the default. Returns ``None`` only when
    *command* is on the lifecycle allowlist OR the operator has
    explicitly opted out via :envvar:`STRIATUM_DAEMON_REQUIRED` ``= "0"``
    **paired with** :envvar:`STRIATUM_TEST_HARNESS` ``= "1"``.
    The bare ``STRIATUM_DAEMON_REQUIRED=0`` opt-out (without the test
    harness marker) no longer bypasses enforcement — closes the codex
    dogfood-050 threat-model finding that documented the bare env var
    as an operator migration path.
    """
    if command in DAEMON_OPTIONAL_COMMANDS:
        return None
    if (
        os.environ.get(ENV_DAEMON_REQUIRED) == "0"
        and os.environ.get(ENV_TEST_HARNESS) == "1"
    ):
        return None
    return DaemonRequirement(enforced=True, socket_path=resolve_socket_path())


def resolve_socket_path() -> Path:
    """Return the configured daemon socket path."""
    override = os.environ.get(ENV_DAEMON_SOCKET)
    if override:
        return Path(override).expanduser()
    if sys.platform == "darwin":
        base = Path.home() / "Library" / "Caches" / "striatum" / "runtime"
    else:
        runtime_root = os.environ.get("XDG_RUNTIME_DIR")
        base = Path(runtime_root) / "striatum" if runtime_root else (
            Path.home() / ".local" / "state" / "striatum" / "runtime"
        )
    return base / "striatumd.sock"


def render_daemon_unreachable_message(socket_path: Path) -> str:
    """Compose the stderr block for RFC 0043 §3 exit code 11.

    Lists the socket path that was tried plus platform remediation
    matching the design synthesis (Linux systemd, macOS launchd,
    foreground reminder, Postgres install hint).
    """
    return (
        f"daemon_unreachable: could not connect to Striatum daemon at {socket_path}\n"
        "Start the daemon and verify PostgreSQL configuration.\n"
        "Linux systemd: systemctl --user start striatumd\n"
        "macOS launchd: launchctl bootstrap gui/$UID "
        "~/Library/LaunchAgents/io.striatum.striatumd.plist\n"
        "Foreground: striatumd --foreground\n"
        "Postgres: run `striatum daemon doctor --postgres-url <url>` or "
        "set STRIATUM_DAEMON_DB_URL."
    )


def render_daemon_unreachable_hint(socket_path: Path) -> str:
    """Compose the structured ``hint`` field for the JSON error envelope."""
    return (
        f"start striatumd; run striatum daemon doctor --postgres-url <url> "
        f"(socket: {socket_path})"
    )


def render_repo_not_migrated_message(repo_path: Path) -> str:
    """Compose the stderr block for RFC 0043 §3 exit code 12."""
    return (
        f"repo_not_migrated: {repo_path} has not been migrated to daemon "
        "PostgreSQL state\n"
        f"Run: striatum daemon migrate-repo-local --from sqlite --to pg "
        f"--repo {repo_path}"
    )


def render_repo_not_migrated_hint(repo_path: Path) -> str:
    """Compose the structured ``hint`` field for the JSON error envelope."""
    return (
        f"striatum daemon migrate-repo-local --from sqlite --to pg "
        f"--repo {repo_path}"
    )


def build_daemon_unreachable_error(socket_path: Path) -> DaemonUnreachableError:
    """Construct the canonical exit-code-11 error.

    Returning the constructed exception keeps call sites explicit about
    the ``raise`` they intend; the dispatcher decides how to format the
    error (text vs JSON envelope).
    """
    return DaemonUnreachableError(
        render_daemon_unreachable_message(socket_path),
        hint=render_daemon_unreachable_hint(socket_path),
    )


def build_repo_not_migrated_error(repo_path: Path) -> RepoNotMigratedError:
    """Construct the canonical exit-code-12 error."""
    return RepoNotMigratedError(
        render_repo_not_migrated_message(repo_path),
        hint=render_repo_not_migrated_hint(repo_path),
    )


def daemon_socket_is_reachable(socket_path: Path, *, timeout_seconds: float = 1.0) -> bool:
    """Probe the daemon socket. Returns ``True`` only on a successful connect.

    A missing path, refused connection, timeout, or permission denial all
    return ``False`` so the caller can construct the exit-code-11 refusal
    without distinguishing failure modes (RFC 0043 §3 collapses them all
    into ``daemon_unreachable``).
    """
    if not socket_path.exists():
        return False
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
            sock.settimeout(timeout_seconds)
            sock.connect(str(socket_path))
    except OSError:
        return False
    return True


def repo_is_migrated(repo_path: Path) -> bool:
    """Best-effort migration check.

    Track A owns the authoritative ``striatumd.repo_migrations`` lookup; Track B
    surfaces the refusal shape so callers can opt in once Track A lands.
    For now the helper treats the presence of a ``.striatum/state.sqlite3``
    file without a sibling ``.striatum/state.sqlite3.tombstone`` as the
    "pre-cutover" signal; ``True`` otherwise.
    """
    state_db = repo_path / ".striatum" / "state.sqlite3"
    tombstone = repo_path / ".striatum" / "state.sqlite3.tombstone"
    if not state_db.exists():
        return True
    if tombstone.exists():
        return True
    return False


def enforce_daemon_required(command: str | None, repo: Path) -> None:
    """Raise the RFC 0043 §3 refusals when enforcement is enabled.

    The dispatcher calls this before the SQLite/in-process fallback so
    the operator sees codes 11/12 immediately instead of the legacy
    ``state.sqlite3 not found`` failure mode.
    """
    requirement = resolve_requirement(command)
    if requirement is None:
        return
    if not daemon_socket_is_reachable(requirement.socket_path):
        raise build_daemon_unreachable_error(requirement.socket_path)
    if not repo_is_migrated(repo):
        raise build_repo_not_migrated_error(repo)
