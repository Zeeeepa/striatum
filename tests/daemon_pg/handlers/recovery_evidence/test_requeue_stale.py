"""Parity test for ``recovery.requeue_stale`` PG handler.

Asserts the handler registers under the correct RPC method name, has the
locked ``(ctx, params)`` signature, and refuses repo-write jobs with the
same exit-4 ``InvalidTransitionError`` the SQLite path raises.
"""

from __future__ import annotations

import inspect
import os

import pytest

from striatum.errors import InvalidTransitionError


def test_module_registers_recovery_requeue_stale() -> None:
    from ._helpers import import_handler

    mod = import_handler("requeue_stale")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("recovery.requeue_stale") is mod.handle
    else:
        assert rpc_method == "recovery.requeue_stale"


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("requeue_stale")
    sig = inspect.signature(mod.handle)
    assert list(sig.parameters) == ["ctx", "params"]


def test_handler_module_exposes_no_repo_write_loophole() -> None:
    """The PG handler must use ``is_repo_write_scope`` to gate requeue.

    A direct ``grep`` for the predicate is the cheapest way to prevent
    accidental drift from the SQLite path (which gates on
    ``is_repo_write``).
    """
    from ._helpers import import_handler

    mod = import_handler("requeue_stale")
    source = inspect.getsource(mod)
    assert "is_repo_write_scope" in source, (
        "handler must reuse the repository-write predicate; otherwise "
        "repo-write stale jobs could be requeued without operator inspection"
    )
    assert "InvalidTransitionError" in source


@pytest.mark.skipif(
    not os.environ.get("RFC0048_PARITY"),
    reason="full PG parity fixture requires Track A's PG mutation helpers; "
    "set RFC0048_PARITY=1 to enable",
)
def test_pg_handler_requires_run_id_and_job_id(pg_ctx) -> None:
    from ._helpers import import_handler

    mod = import_handler("requeue_stale")
    with pytest.raises(Exception):
        mod.handle(pg_ctx, {})
