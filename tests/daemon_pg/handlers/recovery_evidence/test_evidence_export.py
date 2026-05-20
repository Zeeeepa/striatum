# ruff: noqa
"""Legacy coverage for the superseded recovery-evidence export module.

Dogfood 060 moved ``evidence.export`` ownership to the read-only PG handler.
The old module keeps helper-level coverage for digest and redaction parity,
but it no longer owns the registry route or emits export workflow events.
"""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import hashlib
import inspect


def test_module_registers_evidence_export() -> None:
    from ._helpers import import_handler
    from striatum.daemon_pg.handlers.reads import evidence_export as read_evidence_export
    from striatum.daemon_pg.handlers.registry import resolve_pg_handler

    mod = import_handler("evidence_export")
    assert hasattr(mod, "handle")
    assert resolve_pg_handler("evidence.export") is read_evidence_export.handle
    assert resolve_pg_handler("evidence.export") is not mod.handle


def test_handler_signature_is_ctx_params() -> None:
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    sig = inspect.signature(mod.handle)
    assert list(sig.parameters) == ["ctx", "params"]


def test_digest_helper_matches_sha256_text() -> None:
    """Source-level pin: the digest must be ``sha256`` of the rendered body's
    UTF-8 bytes. The SQLite path uses :func:`striatum.legacy_sqlite.db.sha256_bytes` on
    ``body.encode("utf-8")``; the PG path's helper must do the same.
    """
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    sample = "test body"
    expected = hashlib.sha256(sample.encode("utf-8")).hexdigest()
    assert mod._sha256_text(sample) == expected


def test_handler_reuses_shared_evidence_presenter() -> None:
    """Synthesis-locked digest equality requires reusing the *same* redaction
    policy code from :mod:`striatum.evidence_presentation`. If the PG path
    forks the redactor, the digest will diverge silently when the policy
    changes.
    """
    from ._helpers import import_handler

    mod = import_handler("evidence_export")
    source = inspect.getsource(mod)
    assert "from striatum.evidence_presentation import redact_evidence_payload" in source
    assert "render_evidence_markdown" in source


def test_read_handler_does_not_emit_evidence_exported_event() -> None:
    from striatum.daemon_pg.handlers.reads import evidence_export as read_evidence_export

    source = inspect.getsource(read_evidence_export)
    assert "append_event" not in source
    assert "INSERT INTO striatumd.events" not in source
    assert "evidence.exported" not in source
