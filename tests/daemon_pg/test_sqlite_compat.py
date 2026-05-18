from __future__ import annotations

import hashlib
import sqlite3
from pathlib import Path

from striatum.daemon_pg.sqlite_compat import audit_chain_records, repo_identity
from striatum.primitives import json_dumps


def test_repo_identity_matches_legacy_shape(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    (state_dir / "state.sqlite3").write_bytes(b"sqlite")

    repo_stat = repo.stat()
    state_stat = (state_dir / "state.sqlite3").stat()

    assert repo_identity(repo) == (
        f"inode:{repo_stat.st_dev}:{repo_stat.st_ino}:"
        f"state:{state_stat.st_dev}:{state_stat.st_ino}"
    )


def test_audit_chain_records_accepts_valid_chain() -> None:
    conn = _audit_conn()
    try:
        row_hash = _insert_audit_row(conn, audit_id=1, previous_hash=None)
        conn.execute(
            """
            INSERT INTO audit_segments(segment_id, first_audit_id, last_audit_id,
              first_hash, last_hash, previous_segment_last_hash, state)
            VALUES (1, 1, 1, ?, ?, NULL, 'closed')
            """,
            (row_hash, row_hash),
        )

        assert audit_chain_records(conn) == []
    finally:
        conn.close()


def test_audit_chain_records_reports_broken_previous_hash() -> None:
    conn = _audit_conn()
    try:
        _insert_audit_row(conn, audit_id=1, previous_hash="wrong")

        records = audit_chain_records(conn)

        assert records[0]["check"] == "daemon_audit_chain"
        assert records[0]["context"] == {
            "expected_previous_hash": None,
            "actual_previous_hash": "wrong",
        }
    finally:
        conn.close()


def _audit_conn() -> sqlite3.Connection:
    conn = sqlite3.connect(":memory:")
    conn.row_factory = sqlite3.Row
    conn.executescript(
        """
        CREATE TABLE audit_log(
          audit_id INTEGER PRIMARY KEY,
          timestamp TEXT,
          client_id TEXT,
          repository_id TEXT,
          command TEXT,
          authorization_result TEXT,
          denial_reason TEXT,
          transport TEXT,
          request_id TEXT,
          exit_code INTEGER,
          payload_sha256 TEXT,
          previous_hash TEXT,
          row_hash TEXT,
          segment_id INTEGER
        );
        CREATE TABLE audit_segments(
          segment_id INTEGER PRIMARY KEY,
          first_audit_id INTEGER,
          last_audit_id INTEGER,
          first_hash TEXT,
          last_hash TEXT,
          previous_segment_last_hash TEXT,
          state TEXT
        );
        """
    )
    return conn


def _insert_audit_row(conn: sqlite3.Connection, *, audit_id: int, previous_hash: str | None) -> str:
    material = {
        "timestamp": "2026-05-18T00:00:00Z",
        "client_id": "client_1",
        "repository_id": "repo_1",
        "command": "daemon.hello",
        "authorization_result": "allowed",
        "denial_reason": None,
        "transport": "cli",
        "request_id": "request_1",
        "exit_code": 0,
        "payload_sha256": "payload",
        "previous_hash": previous_hash,
        "segment_id": 1,
    }
    row_hash = hashlib.sha256(json_dumps(material).encode("utf-8")).hexdigest()
    conn.execute(
        """
        INSERT INTO audit_log(audit_id, timestamp, client_id, repository_id,
          command, authorization_result, denial_reason, transport, request_id,
          exit_code, payload_sha256, previous_hash, row_hash, segment_id)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """,
        (
            audit_id,
            material["timestamp"],
            material["client_id"],
            material["repository_id"],
            material["command"],
            material["authorization_result"],
            material["denial_reason"],
            material["transport"],
            material["request_id"],
            material["exit_code"],
            material["payload_sha256"],
            material["previous_hash"],
            row_hash,
            material["segment_id"],
        ),
    )
    return row_hash
