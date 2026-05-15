"""Local helpers for Track B handler tests.

Cannot be named ``conftest`` because the test runner finds the top-level
``tests/conftest.py`` first when test files do ``from conftest import …``;
explicit module imports must use a unique name.
"""

from __future__ import annotations

import importlib
from types import ModuleType


def import_handler(module_name: str) -> ModuleType:
    """Import a Track B handler module by its short name.

    Returns ``striatum.daemon_pg.handlers.recovery_evidence.<module_name>``.
    """
    full = f"striatum.daemon_pg.handlers.recovery_evidence.{module_name}"
    return importlib.import_module(full)


__all__ = ["import_handler"]
