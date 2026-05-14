"""Local helpers for Track B handler tests.

Cannot be named ``conftest`` because the test runner finds the top-level
``tests/conftest.py`` first when test files do ``from conftest import ...``;
explicit module imports must use a unique name.
"""

from __future__ import annotations

import importlib
import sys
from types import ModuleType


def _stub_missing_workflow_loop_modules() -> None:
    """Pre-populate ``sys.modules`` with empty stubs for Track A handler
    submodules that don't exist yet, so the parent package's ``__init__``
    can finish importing.

    Once Track A lands ``record_verdict``, ``submit_review``, and
    ``override_review_verdict``, the stubs are harmlessly shadowed.
    """
    parent = "striatum.daemon_pg.handlers.workflow_loop"
    for missing in ("record_verdict", "submit_review", "override_review_verdict"):
        name = f"{parent}.{missing}"
        if name not in sys.modules:
            sys.modules[name] = ModuleType(name)


def import_handler(module_name: str) -> ModuleType:
    """Import a Track B handler module by its short name."""
    _stub_missing_workflow_loop_modules()
    full = f"striatum.daemon_pg.handlers.recovery_evidence.{module_name}"
    return importlib.import_module(full)


__all__ = ["import_handler"]
