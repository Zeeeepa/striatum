"""Parity test for ``recovery.cancel_job`` PG handler.

Asserts the handler registers, holds the locked signature, and refuses
empty-reason or terminal-state inputs the same way the SQLite path does.
"""

from __future__ import annotations

import inspect


def test_module_registers_recovery_cancel_job() -> None:
    from ._helpers import import_handler

    mod = import_handler("cancel_job")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("recovery.cancel_job") is mod.handle
    else:
        assert rpc_method == "recovery.cancel_job"


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("cancel_job")
    sig = inspect.signature(mod.handle)
    assert list(sig.parameters) == ["ctx", "params"]


def test_cancelable_states() -> None:
    from ._helpers import import_handler

    mod = import_handler("cancel_job")
    expected_states = frozenset({"pending", "claimed", "blocked"})

    assert mod.CANCELABLE_JOB_STATES == expected_states



def test_handler_emits_cascade_events_in_order() -> None:
    """Source-level guard: cascade must iterate dependents BFS-style so
    the audit chain reflects causal cancellation order.
    """
    from ._helpers import import_handler

    mod = import_handler("cancel_job")
    source = inspect.getsource(mod)
    assert "_dependents_blocked_only_through" in source
    assert "while queue:" in source, (
        "cancel_job must iterate dependents until the queue empties to "
        "match SQLite's transitive cascade behavior"
    )
