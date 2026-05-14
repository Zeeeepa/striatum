"""Parity test for ``evidence.export`` PG handler.

Synthesis L181-189: the handler is read-only over workflow tables and
emits one ``evidence.exported`` event with payload ``{path, sha256}``.
Digest equality with the SQLite path is the load-bearing assertion.
"""

from __future__ import annotations

import hashlib
import inspect


def test_module_registers_evidence_export() -> None:
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    assert hasattr(mod, "handle")
    rpc_method = getattr(mod.handle, "_pg_rpc_method", None)
    if rpc_method is None:
        from striatum.daemon_pg.handlers.registry import resolve_pg_handler

        assert resolve_pg_handler("evidence.export") is mod.handle
    else:
        assert rpc_method == "evidence.export"


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    sig = inspect.signature(mod.handle)
    assert list(sig.parameters) == ["ctx", "params"]


def test_digest_helper_matches_sha256_text() -> None:
    """Source-level pin: the digest must be ``sha256`` of the rendered body's
    UTF-8 bytes. The SQLite path uses :func:`striatum.db.sha256_bytes` on
    ``body.encode("utf-8")``; the PG path's helper must do the same.
    """
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    sample = "test body"
    expected = hashlib.sha256(sample.encode("utf-8")).hexdigest()
    assert mod._sha256_text(sample) == expected


def test_handler_reuses_cli_evidence_redactor() -> None:
    """Synthesis-locked digest equality requires reusing the *same* redaction
    policy code from :mod:`striatum.cli.evidence`. If the PG path forks the
    redactor, the digest will diverge silently when the policy changes.
    """
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    source = inspect.getsource(mod)
    assert "from striatum.cli.evidence import redact_evidence_payload" in source
    assert "render_evidence_markdown" in source


def test_handler_emits_evidence_exported_event() -> None:
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    source = inspect.getsource(mod.handle)
    assert 'event_type="evidence.exported"' in source
    assert '"sha256"' in source and '"path"' in source
