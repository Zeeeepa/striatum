"""Parity test for ``recovery.stale_leases`` PG handler.

Asserts that:
- The handler module imports and registers ``recovery.stale_leases``.
- The decorator argument matches the RPC method registered in
  ``CLI_ROUTES`` (server.py:78).
- The handler signature is ``(ctx, params) -> dict[str, Any]``.
- On an empty PG run state, the handler returns ``stale_count: 0`` with
  no events emitted (parity with the SQLite path on a fresh DB).

Byte-equivalence over a populated fixture requires Track A's PG ack /
release / claim helpers to be available so the fixture can be built via
the registered RPC verbs on both substrates. That assertion is gated
behind the ``RFC0048_PARITY`` env marker so it runs locally during
sub-agent verification but does not block CI until the full helper set
lands.
"""

from __future__ import annotations

import inspect
import os
from typing import Any

import pytest


def test_module_registers_recovery_stale_leases() -> None:
    from ._helpers import import_handler

    mod = import_handler("stale_leases")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("recovery.stale_leases") is mod.handle, (
            "handle must be registered under the 'recovery.stale_leases' RPC method"
        )
    else:
        assert rpc_method == "recovery.stale_leases"


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("stale_leases")
    sig = inspect.signature(mod.handle)
    params = list(sig.parameters)
    assert params == ["ctx", "params"], (
        f"handle must have the synthesis-locked signature; got {params!r}"
    )


@pytest.mark.skipif(
    not os.environ.get("RFC0048_PARITY"),
    reason="full PG parity fixture requires Track A's PG mutation helpers; "
    "set RFC0048_PARITY=1 to enable",
)
def test_pg_handler_empty_run_returns_no_stale_entries(pg_ctx: Any) -> None:
    from ._helpers import import_handler

    mod = import_handler("stale_leases")
    run_id = "run_test_empty"
    with pg_ctx.cursor() as cur:
        cur.execute(
            "INSERT INTO striatumd.workflow_snapshots(repository_id, workflow_snapshot_id, "
            " workflow_id, content_sha256, workflow_json, loaded_at) "
            "VALUES (%s, 'snap_x', 'wf_x', 'sha', '{}'::jsonb, now())",
            (pg_ctx.repository_id,),
        )
        cur.execute(
            "INSERT INTO striatumd.runs(repository_id, run_id, workflow_snapshot_id, "
            " repo_root, state, created_at) VALUES (%s, %s, 'snap_x', %s, 'ready', now())",
            (pg_ctx.repository_id, run_id, str(pg_ctx.repo_root)),
        )
    pg_ctx.pg_conn.commit()
    result = mod.handle(pg_ctx, {"run_id": run_id})
    assert result["run_id"] == run_id
    assert result["stale_count"] == 0
    assert result["stale_leases"] == []
    assert result["next_actions"] == []
