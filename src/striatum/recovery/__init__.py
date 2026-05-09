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

from striatum.recovery.auto import run_auto_sweep
from striatum.recovery.hooks import (
    run_marker_file_hook,
    run_shell_hook,
    run_webhook_hook,
)
from striatum.recovery.policy import (
    DEFAULT_POLICY,
    resolve_policy,
    validate_recovery_policy,
)

__all__ = [
    "DEFAULT_POLICY",
    "resolve_policy",
    "run_auto_sweep",
    "run_marker_file_hook",
    "run_shell_hook",
    "run_webhook_hook",
    "validate_recovery_policy",
]
