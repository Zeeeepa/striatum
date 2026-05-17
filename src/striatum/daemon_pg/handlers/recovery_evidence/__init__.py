"""PG-native handlers for recovery + evidence RPC methods (RFC 0048 Phase A, Track B).

This subpackage owns the 7 ports listed in
``docs/dogfood/057/DESIGN_SYNTHESIS.md`` L121-189. Each module exposes a
single ``handle(ctx, params) -> dict`` function decorated with
``@register_pg_handler("<rpc.method>")``. Importing this subpackage runs
those decorators (synthesis L18), so Track A's
``handlers/__init__.py`` only needs to do ``from . import recovery_evidence``
to wire every Track B handler into the registry.

Method → module index:

    recovery.stale_leases   → stale_leases.handle
    recovery.requeue_stale  → requeue_stale.handle
    recovery.cancel_job     → cancel_job.handle
    recovery.process_reconcile → process_reconcile.handle
    recovery.resume         → resume_blocker.handle
    recovery.auto           → auto_publish_stale_artifacts.handle
    recovery.auto_finalize  → auto_finalize.handle
    evidence.export         → evidence_export.handle
    recovery.watch          → watch.handle (fail closed)

Naming-mismatch follow-up (REVIEW.md L117-119, finding #1):
``recovery.auto_publish_stale_artifacts`` was the synthesis section title
but the canonical daemon RPC method is ``recovery.auto``. Track B's module file is named
``auto_publish_stale_artifacts.py`` to match the synthesis title for
grep-ability, but the ``@register_pg_handler`` argument uses
``"recovery.auto"`` so registry lookups succeed.
"""

from __future__ import annotations

from . import auto_finalize  # noqa: F401
from . import auto_publish_stale_artifacts  # noqa: F401  (decorator side-effect)
from . import cancel_job  # noqa: F401
from . import evidence_export  # noqa: F401
from . import process_reconcile  # noqa: F401
from . import requeue_stale  # noqa: F401
from . import resume_blocker  # noqa: F401
from . import stale_leases  # noqa: F401
from . import watch  # noqa: F401

__all__ = [
    "auto_finalize",
    "auto_publish_stale_artifacts",
    "cancel_job",
    "evidence_export",
    "process_reconcile",
    "requeue_stale",
    "resume_blocker",
    "stale_leases",
    "watch",
]
