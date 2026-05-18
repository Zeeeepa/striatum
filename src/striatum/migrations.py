"""Lazy compatibility facade for retired repo-local SQLite migrations."""

from __future__ import annotations

from importlib import import_module
import sys
from typing import Any
from types import ModuleType

_LEGACY_MODULE = "striatum.legacy_sqlite.migrations"
_PROTECTED_NAMES = frozenset(
    {
        "__builtins__",
        "__cached__",
        "__class__",
        "__dict__",
        "__doc__",
        "__file__",
        "__loader__",
        "__name__",
        "__package__",
        "__spec__",
    }
)


def __getattr__(name: str) -> Any:
    return getattr(import_module(_LEGACY_MODULE), name)


def __dir__() -> list[str]:
    module = import_module(_LEGACY_MODULE)
    return sorted(set(globals()) | set(dir(module)))


class _LegacyFacadeModule(ModuleType):
    def __getattr__(self, name: str) -> Any:
        return __getattr__(name)

    def __setattr__(self, name: str, value: object) -> None:
        if name.startswith("_") or name in _PROTECTED_NAMES:
            super().__setattr__(name, value)
            return
        setattr(import_module(_LEGACY_MODULE), name, value)
        super().__setattr__(name, value)

    def __delattr__(self, name: str) -> None:
        if not name.startswith("_") and name not in _PROTECTED_NAMES:
            legacy_module = import_module(_LEGACY_MODULE)
            if hasattr(legacy_module, name):
                delattr(legacy_module, name)
        super().__delattr__(name)

    def __dir__(self) -> list[str]:
        return sorted(set(super().__dir__()) | set(__dir__()))


sys.modules[__name__].__class__ = _LegacyFacadeModule
