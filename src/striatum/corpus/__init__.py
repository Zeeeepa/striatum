"""Deterministic Striatum corpus verification surface."""

from __future__ import annotations

from importlib import import_module
from typing import Any

from striatum.corpus.types import SCHEMA_VERSION

__all__ = ["SCHEMA_VERSION", "verify_corpus_bundle"]


_SYMBOL_MODULES = {
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
