from __future__ import annotations

import sqlite3
from pathlib import Path

import pytest

from striatum import daemon
from striatum.db import init_repo
from striatum.mcp import DaemonRpcServer


def _env(tmp_path: Path) -> dict[str, str]:
    return {
        daemon.ENV_REGISTRY: str(tmp_path / "daemon" / "striatumd.sqlite3"),
        daemon.ENV_RUNTIME: str(tmp_path / "runtime"),
        "XDG_CONFIG_HOME": str(tmp_path / "config"),
        "STRIATUM_DAEMON_DB_URL": "",
    }


def _init_repo(repo: Path) -> None:
    repo.mkdir(parents=True, exist_ok=True)
    init_repo(repo)


def test_repo_add_refuses_path_traversal(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    
    repo = tmp_path / "repo"
    _init_repo(repo)
    
    # Path with traversal
    traversal_path = repo / ".." / "repo"
    
    with pytest.raises(daemon.DaemonCapabilityError, match="repo path traversal is not allowed"):
        daemon.repo_add(traversal_path)


def test_authorize_refuses_malformed_tokens(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    
    conn = daemon.connect_registry()
    
    # No separator
    auth = daemon._authorize(conn, required="read", repository_id=None, token="notoken")
    assert auth.authorization_result == "denied"
    assert auth.denial_reason == "token_malformed"
    
    # Empty parts
    auth = daemon._authorize(conn, required="read", repository_id=None, token=".")
    assert auth.authorization_result == "denied"
    assert auth.denial_reason == "token_malformed"
    
    # Missing secret
    auth = daemon._authorize(conn, required="read", repository_id=None, token="dtok_abc.")
    assert auth.authorization_result == "denied"
    assert auth.denial_reason == "token_malformed"


def test_runtime_token_ignores_plaintext_env_var(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    monkeypatch.setenv("STRIATUM_DAEMON_TOKEN", "dtok_forbidden.secret")

    assert daemon.read_runtime_token() is None


def test_audit_chain_tamper_detection(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    
    repo = tmp_path / "repo"
    _init_repo(repo)
    daemon.repo_add(repo)
    
    conn = daemon.connect_registry()
    
    # Verify chain is OK
    records = daemon._audit_chain_records(conn)
    assert records == []
    
    # Tamper with an audit row
    conn.execute("DROP TRIGGER audit_no_update")
    conn.execute("UPDATE audit_log SET command = 'tampered' WHERE audit_id = 1")
    conn.commit()
    
    records = daemon._audit_chain_records(conn)
    assert len(records) > 0
    assert "daemon audit row hash is invalid" in records[0]["message"]
    
    with pytest.raises(sqlite3.Error, match="daemon audit rows are append-only"):
        conn.execute("DELETE FROM audit_log WHERE audit_id = 1")


def test_socket_permissions_are_owner_only(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    
    # Run daemon foreground for one sweep
    daemon.run_daemon_foreground(max_sweeps=1)
    
    reg_path = daemon.registry_path()
    assert reg_path.exists()
    mode = reg_path.stat().st_mode & 0o777
    assert mode == 0o600


def test_daemon_mcp_denies_invoke(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    
    server = DaemonRpcServer()
    # Attempt to use striatum/invoke which is present in LocalRpcServer but should be absent in DaemonRpcServer
    resp = server.handle_request({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "striatum/invoke",
        "params": {"args": ["status"]}
    })
    assert resp is not None
    assert resp["error"]["code"] == -32601 # Method not found
