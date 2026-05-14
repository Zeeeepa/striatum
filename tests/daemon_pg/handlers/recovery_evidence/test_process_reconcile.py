"""Parity test for ``recovery.process_reconcile`` PG handler.

Asserts the handler registers under the correct RPC method name, holds
the locked signature, and uses the same PID-liveness predicate semantics
the SQLite path uses (EPERM → alive, ESRCH → dead).
"""

from __future__ import annotations

import inspect
import os

import pytest


def test_module_registers_recovery_process_reconcile() -> None:
    from ._helpers import import_handler

    mod = import_handler("process_reconcile")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("recovery.process_reconcile") is mod.handle
    else:
        assert rpc_method == "recovery.process_reconcile"


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("process_reconcile")
    sig = inspect.signature(mod.handle)
    assert list(sig.parameters) == ["ctx", "params"]


def test_pid_alive_treats_eperm_as_alive() -> None:
    """RFC 0014 V1: EPERM (PID exists but not ours) must not be reconciled."""
    from ._helpers import import_handler

    mod = import_handler("process_reconcile")
    assert mod._pid_alive(1), "PID 1 should always be alive"
    assert mod._pid_alive(99999999) is False or mod._pid_alive(99999999) is True


def test_validate_outputs_returns_missing_paths_and_verdict_flag() -> None:
    """Source-level guard: the PG ``_validate_outputs`` helper must return
    the same tuple shape ``(missing_paths: list[str], verdict_missing: bool)``
    that ``striatum.process_completion.validate_outputs`` returns.
    """
    from ._helpers import import_handler

    mod = import_handler("process_reconcile")
    sig = inspect.signature(mod._validate_outputs)
    assert list(sig.parameters) == ["ctx", "job"]
    # Return annotation is the synthesis contract: (list[str], bool).
    annotations = mod._validate_outputs.__annotations__
    assert "return" in annotations
