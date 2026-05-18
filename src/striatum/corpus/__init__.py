"""Deterministic Striatum corpus export surface.

The package-level compatibility exports are lazy so storage-neutral corpus
helpers can be imported without loading the legacy SQLite export path.
"""

from __future__ import annotations

from importlib import import_module
from typing import Any

from striatum.corpus.types import SCHEMA_VERSION

__all__ = ["SCHEMA_VERSION", "export_corpus_bundle", "verify_corpus_bundle"]


_SYMBOL_MODULES = {
    "export_corpus_bundle": "striatum.corpus.export",
    "verify_corpus_bundle": "striatum.corpus.verify",
}


def __getattr__(name: str) -> Any:
    module_name = _SYMBOL_MODULES.get(name)
    if module_name is None:
        raise AttributeError(f"module 'striatum.corpus' has no attribute {name!r}")
    module = import_module(module_name)
    value = getattr(module, name)
    globals()[name] = value
    return value
