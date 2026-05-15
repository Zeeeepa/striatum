from __future__ import annotations

from pathlib import Path
from typing import Any, cast

import pytest

from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION, MIGRATIONS
from striatum.daemon_apply.apply_service import handle_apply_rpc
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError
from striatum.daemon_rpc.handshake import build_welcome
from striatum.daemon_rpc.registry import METHOD_REGISTRY, METHODS_ETAG, describe_methods
from striatum.daemon_rpc.server import DaemonRpcRouter
from striatum.daemon_rpc.transport_http import validate_loopback_bind
from striatum.daemon_rpc.transport_unix import bind_unix_socket
from striatum.db import connect, init_repo


def test_envelope_v1_round_trip_and_rejects_version_skew() -> None:
    envelope = RpcEnvelope.decode(
        b'{"schema_version":1,"request_id":"018f","method":"daemon.describe","params":{},"deadline_ms":0}'
    )

    assert envelope.method == "daemon.describe"
    assert RpcEnvelope.decode(envelope.encode()) == envelope

    with pytest.raises(RpcError) as exc:
        RpcEnvelope.decode(b'{"schema_version":2,"request_id":"018f","method":"daemon.describe","params":{}}')
    assert exc.value.code == "version_incompatible"
    assert exc.value.exit_code == 10


def test_handshake_returns_methods_etag_and_refuses_unknown_framing() -> None:
    welcome = build_welcome(
        {
            "client": {
                "name": "striatum-cli",
                "version": "1.22.1",
                "supported_envelope": [1],
                "supported_framings": ["json"],
            }
        },
        methods_etag=METHODS_ETAG,
        substrate_schema=2,
        sealed_apply_supported=True,
        key_loaded=False,
    )

    assert welcome["envelope"] == 1
    assert welcome["substrate"] == "postgres"
    assert welcome["substrate_schema"] == 2
    assert welcome["methods_etag"] == METHODS_ETAG
    assert welcome["sealed_apply"]["supported"] is True

    with pytest.raises(RpcError) as exc:
        build_welcome(
            {
                "client": {
                    "name": "old",
                    "version": "0",
                    "supported_envelope": [1],
                    "supported_framings": ["xml"],
                }
            },
            methods_etag=METHODS_ETAG,
            substrate_schema=2,
            sealed_apply_supported=False,
        )
    assert exc.value.code == "version_incompatible"


def test_router_welcome_reports_sealed_apply_unsupported_without_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr("striatum.daemon_rpc.server.key_loaded", lambda: False)
    router = DaemonRpcRouter()
    response = router.handle(
        RpcEnvelope(
            schema_version=1,
            request_id="hello-no-key",
            method="daemon.hello",
            params={
                "client": {
                    "name": "striatum-cli",
                    "version": "1",
                    "supported_envelope": [1],
                    "supported_framings": ["json"],
                }
            },
        )
    )

    assert response.ok is True
    assert response.data["sealed_apply"]["supported"] is False
    assert response.data["sealed_apply"]["key_loaded"] is False


def test_apply_reviewed_patch_ignores_client_supplied_signing_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr("striatum.daemon_apply.apply_service.key_loaded", lambda: False)

    with pytest.raises(RpcError) as exc:
        handle_apply_rpc("apply.reviewed_patch", {"signing_key_loaded": True})

    assert exc.value.code == "sealed_key_missing"


def test_second_connection_must_handshake_even_after_first_connection() -> None:
    router = DaemonRpcRouter()
    hello = RpcEnvelope(
        schema_version=1,
        request_id="hello-a",
        method="daemon.hello",
        params={
            "client": {
                "name": "striatum-cli",
                "version": "1",
                "supported_envelope": [1],
                "supported_framings": ["json"],
            }
        },
    )
    assert router.handle(hello, connection_id="a").ok is True

    same_connection = router.handle(
        RpcEnvelope(schema_version=1, request_id="describe-a", method="daemon.describe", params={}),
        connection_id="a",
    )
    assert same_connection.ok is False
    assert same_connection.data["code"] == "token_missing"

    second_connection = router.handle(
        RpcEnvelope(schema_version=1, request_id="describe-b", method="daemon.describe", params={}),
        connection_id="b",
    )
    assert second_connection.ok is False
    assert second_connection.data["code"] == "version_incompatible"


def test_rpc_denial_writes_audit_and_request_log(monkeypatch: pytest.MonkeyPatch) -> None:
    audit_calls: list[dict[str, Any]] = []
    request_log_calls: list[dict[str, Any]] = []

    monkeypatch.setattr("striatum.daemon_rpc.server.request_id_seen", lambda *args, **kwargs: False)

    def fake_append_audit_row(*args: Any, **kwargs: Any) -> int:
        audit_calls.append(kwargs)
        return 41

    def fake_append_request_log(*args: Any, **kwargs: Any) -> None:
        request_log_calls.append(kwargs)

    monkeypatch.setattr("striatum.daemon_rpc.server.append_audit_row", fake_append_audit_row)
    monkeypatch.setattr("striatum.daemon_rpc.server.append_request_log", fake_append_request_log)

    response = DaemonRpcRouter(pg_conn=object()).handle(
        RpcEnvelope(schema_version=1, request_id="no-hello", method="daemon.describe", params={})
    )

    assert response.ok is False
    assert response.audit_id == "aud_41"
    assert response.data["code"] == "version_incompatible"
    assert audit_calls[0]["auth"].decision == "denied"
    assert audit_calls[0]["auth"].denial_reason == "version_incompatible"
    assert request_log_calls[0]["decision"] == "denied"
    assert request_log_calls[0]["audit_id"] == 41


def test_duplicate_request_id_is_refused_before_audit(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr("striatum.daemon_rpc.server.request_id_seen", lambda *args, **kwargs: True)
    monkeypatch.setattr(
        "striatum.daemon_rpc.server.append_audit_row",
        lambda *args, **kwargs: pytest.fail("duplicate request must not append audit"),
    )
    monkeypatch.setattr(
        "striatum.daemon_rpc.server.append_request_log",
        lambda *args, **kwargs: pytest.fail("duplicate request must not append request log"),
    )

    response = DaemonRpcRouter(pg_conn=object()).handle(
        RpcEnvelope(schema_version=1, request_id="dupe", method="daemon.describe", params={})
    )

    assert response.ok is False
    assert response.data["code"] == "duplicate_request"


def test_hello_success_is_audited_and_request_logged(monkeypatch: pytest.MonkeyPatch) -> None:
    audit_calls: list[dict[str, Any]] = []
    request_log_calls: list[dict[str, Any]] = []
    monkeypatch.setattr("striatum.daemon_rpc.server.key_loaded", lambda: False)
    monkeypatch.setattr("striatum.daemon_rpc.server.request_id_seen", lambda *args, **kwargs: False)

    def fake_append_audit_row(*args: Any, **kwargs: Any) -> int:
        audit_calls.append(kwargs)
        return 42

    monkeypatch.setattr("striatum.daemon_rpc.server.append_audit_row", fake_append_audit_row)
    monkeypatch.setattr(
        "striatum.daemon_rpc.server.append_request_log",
        lambda *args, **kwargs: request_log_calls.append(kwargs),
    )

    response = DaemonRpcRouter(pg_conn=object()).handle(
        RpcEnvelope(
            schema_version=1,
            request_id="hello-audit",
            method="daemon.hello",
            params={
                "client": {
                    "name": "striatum-cli",
                    "version": "1",
                    "supported_envelope": [1],
                    "supported_framings": ["json"],
                }
            },
        )
    )

    assert response.ok is True
    assert audit_calls[0]["auth"].decision == "allowed"
    assert request_log_calls[0]["method"] == "daemon.hello"
    assert request_log_calls[0]["decision"] == "allowed"


def test_repo_scoped_rpc_refuses_unregistered_repo(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    class FakeCursor:
        def __enter__(self) -> "FakeCursor":
            return self

        def __exit__(self, *_args: object) -> None:
            return None

        def execute(self, _sql: str, _params: object = None) -> None:
            return None

        def fetchone(self) -> None:
            return None

    class FakeConn:
        def cursor(self) -> FakeCursor:
            return FakeCursor()

    monkeypatch.setattr("striatum.daemon_rpc.server.request_id_seen", lambda *args, **kwargs: False)
    monkeypatch.setattr(
        "striatum.daemon_rpc.server.authorize",
        lambda *args, **kwargs: RpcAuthContext("client", "token", "repo_a", "read", "allowed"),
    )
    monkeypatch.setattr("striatum.daemon_rpc.server.append_audit_row", lambda *args, **kwargs: 43)
    monkeypatch.setattr("striatum.daemon_rpc.server.append_request_log", lambda *args, **kwargs: None)
    router = DaemonRpcRouter(pg_conn=FakeConn(), repo_root=tmp_path / "repo-a")
    router._handshaken_connections.add("c")

    response = router.handle(
        RpcEnvelope(schema_version=1, request_id="repo-bind", method="status", params={"repository_id": "repo_a"}),
        connection_id="c",
    )

    assert response.ok is False
    assert response.data["code"] == "repo_not_registered"


def test_method_registry_declares_expected_capabilities() -> None:
    assert METHOD_REGISTRY["daemon.hello"].required_capability is None
    assert METHOD_REGISTRY["daemon.describe"].required_capability == "read"
    assert METHOD_REGISTRY["claim_next"].required_capability == "claim"
    assert METHOD_REGISTRY["submit_review"].required_capability == "review"
    assert METHOD_REGISTRY["apply.reviewed_patch"].required_capability == "apply"
    assert METHOD_REGISTRY["recovery.cancel_job"].required_capability == "recovery"
    assert METHOD_REGISTRY["dogfood.publish_on_behalf"].required_capability == "write"
    assert METHOD_REGISTRY["dogfood.surgical_recovery"].required_capability == "surgical_recovery"
    assert METHOD_REGISTRY["daemon.key.rotate"].required_capability == "admin"

    described = describe_methods()
    assert described["methods_etag"] == METHODS_ETAG
    described_methods = cast(list[dict[str, Any]], described["methods"])
    methods = {row["method"] for row in described_methods}
    assert {
        "daemon.hello",
        "daemon.describe",
        "supervise.start",
        "apply.reviewed_patch",
        "cross_repo.describe",
        "dogfood.publish_on_behalf",
        "dogfood.surgical_recovery",
    } <= methods


def test_rpc_transports_are_owner_local_only(tmp_path: Path) -> None:
    validate_loopback_bind("127.0.0.1")
    validate_loopback_bind("::1")
    with pytest.raises(RpcError) as exc:
        validate_loopback_bind("0.0.0.0")
    assert exc.value.code == "non_loopback_refused"

    runtime = tmp_path / "runtime"
    runtime.mkdir(mode=0o700)
    sock = bind_unix_socket(runtime / "daemon.sock")
    try:
        assert (runtime / "daemon.sock").stat().st_mode & 0o777 == 0o600
    finally:
        sock.close()


def test_daemon_pg_migrations_name_rpc_supervisor_apply_and_repo_local_tables() -> None:
    assert LATEST_DAEMON_DB_VERSION == 5
    sql = "\n".join(migration.sql for migration in MIGRATIONS)

    for table in (
        "striatumd.rpc_methods",
        "striatumd.daemon_supervisors",
        "striatumd.apply_receipts",
        "striatumd.repositories",
        "striatumd.runs",
        "striatumd.process_supervisor_pointers",
    ):
        assert table in sql
    assert "cross_repo_runs" in sql
    assert "surgical_recovery" in sql
    assert "mutation_queue" not in sql


def test_repo_local_migration_adds_daemon_supervisor_pointer_table(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    init_repo(repo)

    with connect(repo) as conn:
        row = conn.execute(
            "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'process_supervisor_pointers'"
        ).fetchone()
        assert row is not None
        version = conn.execute("PRAGMA user_version").fetchone()
        assert int(version[0]) >= 14
        row = conn.execute("PRAGMA table_info(runs)").fetchall()
        assert "cross_repo_run_id" in {item[1] for item in row}
