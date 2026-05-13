"""RFC 0043 V1.6 F-lock — ``migrate-repo-local`` refuses on lock contention.

These tests exercise the lock surface directly without touching Postgres
or a live SQLite fixture, so they run under ``pytest -m 'not multi_repo'``.

The two concurrent-invocation guarantees the V1.6 synthesis asks for:

1. While one invocation holds the exclusive ``fcntl.flock``, a second
   refuses with :class:`MigrationInProgressError` (exit code 8) and a
   message naming the source SQLite path.
2. The lock is released once the body returns (or raises), so a
   subsequent invocation succeeds.

Closes gemini A3.
"""

from __future__ import annotations

import errno
import fcntl
import os
from pathlib import Path

import pytest

from striatum.daemon_pg.repo_local_migration import (
    MigrationInProgressError,
    RepoLocalMigrationOptions,
    _exclusive_migrate_lock,
    migrate_repo_local,
)
from striatum.errors import StriatumError


def _prepare_repo(tmp_path: Path) -> Path:
    repo = tmp_path / "repo"
    (repo / ".striatum").mkdir(parents=True)
    # Empty source SQLite so the migrate body, if it ever runs past the
    # lock, exits on the read-only open rather than wandering further.
    (repo / ".striatum" / "state.sqlite3").write_bytes(b"")
    return repo


def _hold_external_lock(repo: Path) -> int:
    lock_path = repo / ".striatum" / "state.sqlite3.migrate.lock"
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(str(lock_path), os.O_RDWR | os.O_CREAT, 0o600)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError:
        os.close(fd)
        raise
    return fd


def _release_external_lock(fd: int) -> None:
    try:
        fcntl.flock(fd, fcntl.LOCK_UN)
    finally:
        os.close(fd)


def test_lock_contention_refuses_with_exit_code_8(tmp_path: Path) -> None:
    """While an external holder owns the lock, ``migrate_repo_local``
    must refuse immediately with the V1.6 ``migrate_in_progress`` shape
    rather than waiting or proceeding to the Postgres connection."""
    repo = _prepare_repo(tmp_path)
    lock_fd = _hold_external_lock(repo)
    try:
        with pytest.raises(MigrationInProgressError) as exc:
            migrate_repo_local(
                RepoLocalMigrationOptions(
                    repo=repo,
                    postgres_url="postgresql://invalid-host-for-this-test/none",
                )
            )
        # Synthesis: exit code 8 (existing V1.5 migrate-repo-local refusal
        # code) — no new exit code introduced.
        assert exc.value.exit_code == 8
        message = str(exc.value)
        assert "migrate_in_progress" in message
        # Operator-facing remediation: name the source SQLite so the
        # message correlates with the repo's state file.
        assert "state.sqlite3" in message
        # Also a StriatumError (so dispatch handles it via the standard
        # JSON envelope path).
        assert isinstance(exc.value, StriatumError)
    finally:
        _release_external_lock(lock_fd)


def test_lock_released_after_context_manager_exit(tmp_path: Path) -> None:
    """The lock context manager must release the flock on normal exit
    so a subsequent migrate invocation can acquire it.

    Without this guarantee the first invocation would permanently wedge
    the lock for the lifetime of the calling process.
    """
    repo = _prepare_repo(tmp_path)
    with _exclusive_migrate_lock(repo):
        pass

    # If the lock were still held, the second acquisition would raise
    # OSError(EWOULDBLOCK). Reaching the inner block means the lock was
    # cleanly released.
    with _exclusive_migrate_lock(repo):
        pass


def test_lock_released_after_context_manager_exception(tmp_path: Path) -> None:
    """The release happens even when the body raises."""
    repo = _prepare_repo(tmp_path)

    class _Sentinel(Exception):
        pass

    with pytest.raises(_Sentinel):
        with _exclusive_migrate_lock(repo):
            raise _Sentinel

    with _exclusive_migrate_lock(repo):
        pass


def test_lock_path_is_sidecar_not_source(tmp_path: Path) -> None:
    """The lock file must be a sidecar (``state.sqlite3.migrate.lock``),
    not the source SQLite itself, so it survives the source-file rename
    during finalization and does not contend with SQLite's own
    POSIX byte-range locks."""
    repo = _prepare_repo(tmp_path)
    with _exclusive_migrate_lock(repo) as lock_path:
        assert lock_path == repo / ".striatum" / "state.sqlite3.migrate.lock"
        assert lock_path.exists()


def test_concurrent_invocation_via_forked_process(tmp_path: Path) -> None:
    """End-to-end: fork a child that holds the lock for a moment, then
    assert the parent's call refuses with exit code 8.

    This is the most direct shape of the synthesis ask ("forks two
    processes and asserts one wins, one refuses"). Forking keeps the
    lock semantics honest — flock semantics are per open file
    description, so a single process opening the file twice already
    contends, but forking exercises the more realistic shape: two
    independent processes racing on the same lock file.
    """
    repo = _prepare_repo(tmp_path)
    ready_r, ready_w = os.pipe()
    proceed_r, proceed_w = os.pipe()

    pid = os.fork()
    if pid == 0:  # pragma: no cover (child)
        os.close(ready_r)
        os.close(proceed_w)
        try:
            with _exclusive_migrate_lock(repo):
                os.write(ready_w, b"ready")
                os.read(proceed_r, 8)
        finally:
            os._exit(0)

    os.close(ready_w)
    os.close(proceed_r)
    try:
        marker = os.read(ready_r, 8)
        assert marker == b"ready"

        with pytest.raises(MigrationInProgressError) as exc:
            migrate_repo_local(
                RepoLocalMigrationOptions(
                    repo=repo,
                    postgres_url="postgresql://invalid-host-for-this-test/none",
                )
            )
        assert exc.value.exit_code == 8
    finally:
        os.write(proceed_w, b"go")
        os.close(proceed_w)
        os.close(ready_r)
        os.waitpid(pid, 0)


def test_lock_acquire_fast_path_does_not_clobber_existing_lock(
    tmp_path: Path,
) -> None:
    """Defensive: ensure ``_exclusive_migrate_lock`` does not silently
    win when another holder has the file locked even briefly (i.e. the
    flag combination is correct — ``LOCK_EX | LOCK_NB``).
    """
    repo = _prepare_repo(tmp_path)
    fd = _hold_external_lock(repo)
    try:
        with pytest.raises(MigrationInProgressError):
            with _exclusive_migrate_lock(repo):
                pytest.fail(
                    "_exclusive_migrate_lock acquired the lock while another "
                    "fd held LOCK_EX — would silently allow concurrent "
                    "migrations"
                )
    finally:
        _release_external_lock(fd)


def test_external_helpers_self_check(tmp_path: Path) -> None:
    """Smoke check the test helpers themselves so a regression in
    ``_hold_external_lock`` does not silently turn the contention tests
    above into no-ops."""
    repo = _prepare_repo(tmp_path)
    fd = _hold_external_lock(repo)
    try:
        lock_path = repo / ".striatum" / "state.sqlite3.migrate.lock"
        second_fd = os.open(str(lock_path), os.O_RDWR)
        try:
            with pytest.raises(OSError) as exc:
                fcntl.flock(second_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            assert exc.value.errno in (errno.EWOULDBLOCK, errno.EAGAIN, errno.EACCES)
        finally:
            os.close(second_fd)
    finally:
        _release_external_lock(fd)


