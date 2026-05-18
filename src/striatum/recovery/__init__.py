"""RFC 0020 V1: autonomous stalled-run recovery.

Public surface:

- :func:`run_auto_sweep` performs a single autonomous-recovery
  sweep against a run. Composable with cron / systemd timers.
- :func:`resolve_policy` merges a workflow's ``recovery_policy``
  block with runner defaults and CLI overrides.
- The hook runners (:func:`run_marker_file_hook`,
  :func:`run_webhook_hook`, :func:`run_shell_hook`) are exposed
  for test injection.
"""

from __future__ import annotations

from importlib import import_module
from typing import Any

_SYMBOL_MODULES = {
    "DEFAULT_POLICY": "striatum.recovery.policy",
    "PIDFILE_COLLISION_EXIT_CODE": "striatum.recovery.watch",
    "pidfile_path": "striatum.recovery.watch",
    "resolve_policy": "striatum.recovery.policy",
    "run_auto_sweep": "striatum.recovery.auto",
    "run_daemon_watch": "striatum.recovery.watch",
    "run_marker_file_hook": "striatum.recovery.hooks",
    "run_shell_hook": "striatum.recovery.hooks",
    "run_webhook_hook": "striatum.recovery.hooks",
    "validate_recovery_policy": "striatum.recovery.policy",
}

__all__ = [
    "DEFAULT_POLICY",
    "PIDFILE_COLLISION_EXIT_CODE",
    "pidfile_path",
    "resolve_policy",
    "run_auto_sweep",
    "run_daemon_watch",
    "run_marker_file_hook",
    "run_shell_hook",
    "run_webhook_hook",
    "validate_recovery_policy",
]


def __getattr__(name: str) -> Any:
    module_name = _SYMBOL_MODULES.get(name)
    if module_name is None:
        raise AttributeError(f"module 'striatum.recovery' has no attribute {name!r}")
    module = import_module(module_name)
    value = getattr(module, name)
    globals()[name] = value
    return value
