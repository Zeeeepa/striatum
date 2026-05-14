"""Native daemon PostgreSQL handlers."""

from __future__ import annotations

from . import workflow_loop as workflow_loop

try:  # Track B owns this package; import it when present.
    from . import recovery_evidence as recovery_evidence  # type: ignore[attr-defined]
except ImportError:  # pragma: no cover - absent until Track B lands.
    recovery_evidence = None
