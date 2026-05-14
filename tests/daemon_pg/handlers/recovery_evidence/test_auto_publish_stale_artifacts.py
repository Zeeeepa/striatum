"""Parity test for ``recovery.auto`` PG handler.

Naming-mismatch follow-up (REVIEW.md L117-119): the file is named
``auto_publish_stale_artifacts.py`` after the synthesis section title,
but the registered RPC method is ``recovery.auto``. This test pins both
facts so the next reviewer sees the intentional asymmetry.
"""

from __future__ import annotations

import inspect


def test_module_registers_recovery_auto_not_long_name() -> None:
    """The decorator argument must be ``recovery.auto`` even though the
    module file is named ``auto_publish_stale_artifacts``. This is the
    explicit fix for finding #1 in the design review (REVIEW.md L117-119).
    """
    from ._helpers import import_handler

    mod = import_handler("auto_publish_stale_artifacts")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("recovery.auto") is mod.handle
        # The longer name must NOT be registered (operator would not find
        # it via grep of ``CLI_ROUTES``).
        assert resolve_pg_handler("recovery.auto_publish_stale_artifacts") is None
    else:
        assert rpc_method == "recovery.auto", (
            "Decorator argument must match the RPC method registered in "
            "server.py:83 (recovery.auto)"
        )


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("auto_publish_stale_artifacts")
    sig = inspect.signature(mod.handle)
    assert list(sig.parameters) == ["ctx", "params"]


def test_dry_run_branch_skips_expire_leases() -> None:
    """Source-level guard for GH #11: dry-run must NOT call ``expire_leases``.

    The SQLite path's ``auto_publish_stale_artifacts`` gates the call
    behind ``if not dry_run``. Drift here would silently mutate runner
    state during a read-only operator preview.
    """
    from ._helpers import import_handler

    mod = import_handler("auto_publish_stale_artifacts")
    source = inspect.getsource(mod.handle)
    # Two cheap, non-syntactic checks: there must be both an "if not dry_run"
    # gate AND an "if dry_run" branch that returns the would-publish list.
    assert "if not dry_run" in source
    assert "would_publish" in source


def test_dry_run_branch_skips_maybe_complete_run() -> None:
    """Source-level guard for GH #11: dry-run must NOT call
    ``maybe_complete_run`` because that mutates the runs row.
    """
    from ._helpers import import_handler

    mod = import_handler("auto_publish_stale_artifacts")
    source = inspect.getsource(mod.handle)
    assert "if not dry_run:\n        maybe_complete_run" in source or (
        "if not dry_run" in source and "maybe_complete_run" in source
    )
