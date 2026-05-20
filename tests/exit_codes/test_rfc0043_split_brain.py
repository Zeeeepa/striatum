# ruff: noqa
"""RFC 0043 V1.6 F-split-brain — ``db.connect`` refuses fresh SQLite when
the repo has already been migrated.

The split-brain risk is: the repo's ``.striatum/state.sqlite3`` has been
tombstoned (Postgres now owns workflow state), but an older/cached CLI
codepath calls ``db.connect`` and would happily create an empty SQLite
file beside the tombstone/sentinel. Future writes would land in the new
empty file, silently diverging from Postgres.

Behavior under test:

* sentinel ``.striatum/state.sqlite3.migrated`` present, source absent →
  refuse with exit code 12 and do **not** create the file.
* tombstone ``.striatum/state.sqlite3.tombstone`` present, source absent →
  refuse with exit code 12 and do **not** create the file.
* neither sentinel nor tombstone → bootstrap behavior preserved; a fresh
  SQLite is created as before.
"""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

from pathlib import Path

import pytest

from striatum.legacy_sqlite.db import connect, db_path
from striatum.errors import StriatumError


def test_split_brain_refuses_when_sentinel_present(tmp_path: Path) -> None:
    state_dir = tmp_path / ".striatum"
    state_dir.mkdir()
    (state_dir / "state.sqlite3.migrated").write_text("{}", encoding="utf-8")

    with pytest.raises(StriatumError) as exc:
        connect(tmp_path)

    assert exc.value.exit_code == 12
    assert "repo_not_migrated" in str(exc.value)
    assert "split-brain" in str(exc.value)
    # Critical: the guard must refuse BEFORE sqlite3.connect creates a
    # zero-byte state.sqlite3 file. Otherwise the split-brain is exactly
    # the scenario this test is supposed to prevent.
    assert not db_path(tmp_path).exists()


def test_split_brain_refuses_when_tombstone_present(tmp_path: Path) -> None:
    state_dir = tmp_path / ".striatum"
    state_dir.mkdir()
    (state_dir / "state.sqlite3.tombstone").write_bytes(b"")

    with pytest.raises(StriatumError) as exc:
        connect(tmp_path)

    assert exc.value.exit_code == 12
    assert "repo_not_migrated" in str(exc.value)
    assert not db_path(tmp_path).exists()


def test_bootstrap_path_preserved_without_checkpoint(tmp_path: Path) -> None:
    """No sentinel, no tombstone, no source — the bootstrap path should
    still create a fresh SQLite as the V1.5 behavior expected. The guard
    only fires when prior-migration evidence is on disk."""
    state_dir = tmp_path / ".striatum"
    state_dir.mkdir()

    conn = connect(tmp_path)
    try:
        assert db_path(tmp_path).exists()
    finally:
        conn.close()


def test_split_brain_guard_does_not_block_existing_source(tmp_path: Path) -> None:
    """If the source SQLite *is* present, ``connect`` returns normally
    even when a sentinel is also present (resume-after-crash path).

    The guard's contract is "do not silently create a fresh file when
    prior migration evidence exists"; it must not interfere with the
    documented crash-resume flow that legitimately holds both files
    momentarily.
    """
    state_dir = tmp_path / ".striatum"
    state_dir.mkdir()
    # Initialize a real SQLite at the canonical path first; this is the
    # state during the "Postgres committed, sentinel written, tombstone
    # not yet renamed" window.
    conn = connect(tmp_path)
    conn.close()
    (state_dir / "state.sqlite3.migrated").write_text("{}", encoding="utf-8")

    # Re-opening with both files present must succeed (the resume path
    # owns finalizing the tombstone, not ``db.connect``).
    conn = connect(tmp_path)
    try:
        assert db_path(tmp_path).exists()
    finally:
        conn.close()
