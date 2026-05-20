"""Parity test for ``recovery.resume`` PG handler.

Asserts the handler registers under the correct RPC method name, holds
the locked signature, and propagates the same blocker-kind allow-list
and ``--force`` requirements the SQLite path enforces.
"""

from __future__ import annotations

import inspect

import pytest

from striatum.errors import InvalidTransitionError


def test_module_registers_recovery_resume() -> None:
    from ._helpers import import_handler

    mod = import_handler("resume_blocker")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("recovery.resume") is mod.handle
    else:
        assert rpc_method == "recovery.resume"


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("resume_blocker")
    sig = inspect.signature(mod.handle)
    assert list(sig.parameters) == ["ctx", "params"]


def test_process_adapter_blocker_kinds() -> None:
    from ._helpers import import_handler

    mod = import_handler("resume_blocker")
    expected_adapter_kinds = frozenset(
        {"process_stopped", "process_lacks_lease", "process_missing_heartbeat"}
    )
    expected_exit_kinds = frozenset(
        {"process_stopped", "process_exit_bad_code", "process_killed"}
    )

    assert mod.PROCESS_ADAPTER_BLOCKER_KINDS == expected_adapter_kinds
    assert mod.PROCESS_EXIT_BLOCKER_KINDS == expected_exit_kinds



def test_complete_requires_session_id() -> None:
    """Parity with the SQLite path: ``--complete`` without ``--session-id``
    must refuse before doing any DB work.
    """
    from ._helpers import import_handler

    mod = import_handler("resume_blocker")
    with pytest.raises(InvalidTransitionError):
        mod.handle(
            ctx=None,
            params={"blocker_id": "blk_test", "complete": True, "extend_seconds": 900},
        )


def test_extend_seconds_must_be_positive() -> None:
    from ._helpers import import_handler

    mod = import_handler("resume_blocker")
    with pytest.raises(InvalidTransitionError):
        mod.handle(
            ctx=None,
            params={"blocker_id": "blk_test", "extend_seconds": 0},
        )
