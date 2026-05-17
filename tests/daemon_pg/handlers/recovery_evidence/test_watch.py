from __future__ import annotations

import inspect
from typing import Any

import pytest

from striatum.daemon_rpc.envelope import RpcError


def test_module_registers_recovery_watch() -> None:
    from ._helpers import import_handler

    mod = import_handler("watch")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("recovery.watch") is mod.handle
    else:
        assert rpc_method == "recovery.watch"


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("watch")
    assert list(inspect.signature(mod.handle).parameters) == ["ctx", "params"]


def test_recovery_watch_fails_closed() -> None:
    from ._helpers import import_handler

    mod = import_handler("watch")
    ctx: Any = object()

    with pytest.raises(RpcError) as exc:
        mod.handle(ctx, {"run_id": "run_1"})

    assert exc.value.code == "not_implemented"
    assert "recovery.watch is not supported inside daemon RPC" in str(exc.value)
    assert "recovery.auto" in str(exc.value)
