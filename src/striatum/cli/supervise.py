"""CLI shims around ``striatum.supervisor``.

The supervisor implementation lives in ``striatum.supervisor``; the CLI
dispatch handlers are thin wrappers that re-export those names so the
package layout reads as one handler module per concern.
"""

from __future__ import annotations

from striatum.supervisor import (
    SUPERVISOR_ACTIVE_STATES,
    supervise_list,
    supervise_send,
    supervise_start,
    supervise_status,
    supervise_stop,
)

__all__ = [
    "SUPERVISOR_ACTIVE_STATES",
    "supervise_list",
    "supervise_send",
    "supervise_start",
    "supervise_status",
    "supervise_stop",
]
