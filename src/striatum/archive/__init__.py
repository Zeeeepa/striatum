"""Run archive creation and verification.

Compatibility exports are lazy so archive verification and daemon handler
registration do not import more run-state helpers than they need.
"""

from __future__ import annotations

from importlib import import_module
from typing import Any

__all__ = [
    "ARCHIVE_JSON_FILES",
    "ARCHIVE_JSONL_FILES",
    "ARCHIVE_SCHEMA_VERSION",
    "verify_run_archive",
    "write_run_archive",
]


_SYMBOL_MODULES = {
    "ARCHIVE_JSON_FILES": "striatum.archive.writer",
    "ARCHIVE_JSONL_FILES": "striatum.archive.writer",
    "ARCHIVE_SCHEMA_VERSION": "striatum.archive.writer",
    "verify_run_archive": "striatum.archive.verify",
    "write_run_archive": "striatum.archive.writer",
}


def __getattr__(name: str) -> Any:
    module_name = _SYMBOL_MODULES.get(name)
    if module_name is None:
        raise AttributeError(f"module 'striatum.archive' has no attribute {name!r}")
    module = import_module(module_name)
    value = getattr(module, name)
    globals()[name] = value
    return value
