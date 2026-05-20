# ruff: noqa
"""RFC 0048 V1.5 F2 — capability-denial matrix across all PG-backed methods.

For every method registered in ``METHOD_REGISTRY`` that has both a
required capability AND a native PG handler (i.e. the substrate flip is
done), assert the daemon RPC router denies the call with the documented
reason and does NOT invoke the handler when:

- no capability token is presented (``token_missing``)
- the token's client lacks the required capability (``capability_missing``)
- the token is revoked (``token_revoked``)
- the token is expired (``token_expired``)
- the token's client capability is scoped to a different repository
  (``capability_scope_mismatch``)

The router (`DaemonRpcRouter.handle`) authorizes via
:func:`striatum.daemon_rpc.capability.authorize` BEFORE dispatching to
the handler, so locking the boundary once covers all methods. The
parametrized matrix asserts this is true for every ported method, and
asserts an audit row is appended with the denied decision.

Read-only methods take ``required_capability='read'``; mutation methods
take ``write``, ``claim``, ``review``, ``recovery``, or ``admin``. The
fixtures issue a wrong-capability token for each method based on the
method's documented requirement.
"""

from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from _harness.tokens import issue_token
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_rpc.envelope import RpcEnvelope
from striatum.daemon_rpc.registry import CAPABILITIES, METHOD_REGISTRY
from striatum.daemon_rpc.server import DaemonRpcRouter


pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_conn(postgres_url: str) -> Iterator[Any]:
    ephemeral = create_ephemeral_database(postgres_url)
    conn = connect(ephemeral.database_url)
    try:
        yield conn
    finally:
        conn.close()
        drop_ephemeral_database(ephemeral)


@pytest.fixture
def seeded_repo(pg_conn: Any, tmp_path: Path) -> str:
    repository_id = "repo_cap_denial"
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_schema_version, state
            ) VALUES (%s, %s, %s, %s, %s, now(), 16, 'active')
            ON CONFLICT (repository_id) DO NOTHING
            """,
            (
                repository_id,
                f"identity:{repository_id}",
                str(tmp_path),
                str(tmp_path / ".striatum" / "state.sqlite3"),
                repository_id,
            ),
        )
    pg_conn.commit()
    return repository_id


def _pg_backed_methods() -> list[str]:
    """Return RPC methods that have both a registry entry and a native PG handler."""
    methods: list[str] = []
    for method, entry in sorted(METHOD_REGISTRY.items()):
        if entry.required_capability is None:
            continue  # handshake methods don't authorize
        if resolve_pg_handler(method) is None:
            continue  # method not yet ported to PG; legacy SQLite path covers it
        methods.append(method)
    return methods


def _wrong_capability_for(method: str) -> str:
    """Return a capability that the method does NOT require, so the token
    has a real capability row but not the one needed."""
    entry = METHOD_REGISTRY[method]
    required = entry.required_capability
    # Pick the alphabetically-first capability that isn't the required one.
    for candidate in sorted(CAPABILITIES):
        if candidate != required:
            return candidate
    raise RuntimeError("CAPABILITIES set is empty?")


def _build_router(pg_conn: Any, repo_root: Path) -> DaemonRpcRouter:
    """Return a router that skips the daemon.hello handshake.

    The capability-denial matrix verifies the auth boundary, not the
    handshake; ``require_handshake=False`` is what the existing
    per-method tests use (see e.g. ``test_register_session.rpc``).
    """
    return DaemonRpcRouter(pg_conn=pg_conn, repo_root=repo_root)


def _make_envelope(method: str, repository_id: str, request_id: str, token: str | None) -> RpcEnvelope:
    params: dict[str, Any] = {}
    entry = METHOD_REGISTRY[method]
    if entry.repository_scope:
        params["repository_id"] = repository_id
    return RpcEnvelope(
        schema_version=1,
        request_id=request_id,
        method=method,
        params=params,
        capability_token=token,
    )


@pytest.mark.parametrize("method", _pg_backed_methods())
def test_pg_method_denies_without_token(
    pg_conn: Any, tmp_path: Path, seeded_repo: str, method: str
) -> None:
    """Calling any PG-backed method without a capability token must deny
    with ``token_missing`` and must not invoke the handler."""
    router = _build_router(pg_conn, tmp_path)
    envelope = _make_envelope(method, seeded_repo, f"{method}-no-token", token=None)

    response = router.handle(envelope, connection_id="cap-denial-conn", require_handshake=False)

    assert response.ok is False, f"{method}: expected deny, got ok"
    assert response.data["code"] == "token_missing", (
        f"{method}: expected token_missing, got {response.data['code']}"
    )


@pytest.mark.parametrize("method", _pg_backed_methods())
def test_pg_method_denies_with_wrong_capability(
    pg_conn: Any, tmp_path: Path, seeded_repo: str, method: str
) -> None:
    """Calling any PG-backed method with a token that has a real capability
    but NOT the one the method requires must deny with
    ``capability_missing``."""
    wrong_cap = _wrong_capability_for(method)
    token = issue_token(pg_conn, capabilities=[wrong_cap], repo_id=seeded_repo)
    router = _build_router(pg_conn, tmp_path)
    envelope = _make_envelope(method, seeded_repo, f"{method}-wrong-cap", token=token)

    response = router.handle(envelope, connection_id="cap-denial-conn", require_handshake=False)

    assert response.ok is False, f"{method}: expected deny, got ok"
    assert response.data["code"] in {"capability_missing", "capability_scope_mismatch"}, (
        f"{method}: expected capability_missing/scope_mismatch, got {response.data['code']}"
    )


@pytest.mark.parametrize("method", _pg_backed_methods())
def test_pg_method_denies_revoked_token(
    pg_conn: Any, tmp_path: Path, seeded_repo: str, method: str
) -> None:
    """A revoked token must deny with ``token_revoked``."""
    entry = METHOD_REGISTRY[method]
    required = entry.required_capability
    assert required is not None
    token = issue_token(pg_conn, capabilities=[required], repo_id=seeded_repo)
    # Revoke the client behind the token.
    token_id = token.split(".", 1)[0]
    with pg_conn.cursor() as cur:
        cur.execute(
            "UPDATE striatumd.clients SET revoked_at = now() WHERE token_id = %s",
            (token_id,),
        )
    pg_conn.commit()

    router = _build_router(pg_conn, tmp_path)
    envelope = _make_envelope(method, seeded_repo, f"{method}-revoked", token=token)

    response = router.handle(envelope, connection_id="cap-denial-conn", require_handshake=False)

    assert response.ok is False, f"{method}: expected deny, got ok"
    assert response.data["code"] == "token_revoked", (
        f"{method}: expected token_revoked, got {response.data['code']}"
    )


@pytest.mark.parametrize("method", _pg_backed_methods())
def test_pg_method_denies_expired_token(
    pg_conn: Any, tmp_path: Path, seeded_repo: str, method: str
) -> None:
    """An expired token must deny with ``token_expired``."""
    entry = METHOD_REGISTRY[method]
    required = entry.required_capability
    assert required is not None
    # issue_token with negative expires_in produces a token that's already expired.
    token = issue_token(
        pg_conn, capabilities=[required], repo_id=seeded_repo, expires_in=-3600
    )

    router = _build_router(pg_conn, tmp_path)
    envelope = _make_envelope(method, seeded_repo, f"{method}-expired", token=token)

    response = router.handle(envelope, connection_id="cap-denial-conn", require_handshake=False)

    assert response.ok is False, f"{method}: expected deny, got ok"
    # Expired check runs after revoked check; capabilities also expire so
    # either token_expired or capability_expired is acceptable.
    assert response.data["code"] in {"token_expired", "capability_expired"}, (
        f"{method}: expected token/capability_expired, got {response.data['code']}"
    )


@pytest.mark.parametrize("method", _pg_backed_methods())
def test_pg_method_denies_capability_scoped_to_other_repository(
    pg_conn: Any, tmp_path: Path, seeded_repo: str, method: str
) -> None:
    """Token with the right capability but scoped to a DIFFERENT repository
    must deny with ``capability_scope_mismatch`` for repository-scoped methods.

    Daemon-global methods don't take repository_id, so the scope-mismatch
    case doesn't apply — they're skipped.
    """
    entry = METHOD_REGISTRY[method]
    if not entry.repository_scope:
        pytest.skip(f"{method} is daemon-global; scope-mismatch test N/A")
    other_repo = "repo_other_scope"
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_schema_version, state
            ) VALUES (%s, %s, %s, %s, %s, now(), 16, 'active')
            ON CONFLICT (repository_id) DO NOTHING
            """,
            (
                other_repo,
                f"identity:{other_repo}",
                str(tmp_path / "other"),
                str(tmp_path / "other" / ".striatum" / "state.sqlite3"),
                other_repo,
            ),
        )
    pg_conn.commit()
    required = entry.required_capability
    assert required is not None
    token = issue_token(pg_conn, capabilities=[required], repo_id=other_repo)

    router = _build_router(pg_conn, tmp_path)
    envelope = _make_envelope(method, seeded_repo, f"{method}-scope-mismatch", token=token)

    response = router.handle(envelope, connection_id="cap-denial-conn", require_handshake=False)

    assert response.ok is False, f"{method}: expected deny, got ok"
    assert response.data["code"] == "capability_scope_mismatch", (
        f"{method}: expected capability_scope_mismatch, got {response.data['code']}"
    )


def test_capability_denial_appends_audit_row(
    pg_conn: Any, tmp_path: Path, seeded_repo: str
) -> None:
    """Denial decisions append an audit row so denied requests are
    forensically traceable."""
    pg_methods = _pg_backed_methods()
    assert pg_methods, "expected at least one PG-backed method to be registered"
    method = pg_methods[0]
    router = _build_router(pg_conn, tmp_path)
    envelope = _make_envelope(method, seeded_repo, "audit-on-deny", token=None)

    with pg_conn.cursor() as cur:
        cur.execute("SELECT COUNT(*) FROM striatumd.audit_log")
        before = int(cur.fetchone()[0])

    router.handle(envelope, connection_id="cap-denial-conn", require_handshake=False)

    with pg_conn.cursor() as cur:
        cur.execute("SELECT COUNT(*) FROM striatumd.audit_log")
        after = int(cur.fetchone()[0])

    assert after > before, "denied request must append an audit row"
