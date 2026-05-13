"""Generated-style daemon RPC method registry."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from typing import Literal

from striatum.db import json_dumps

Capability = Literal["read", "write", "review", "claim", "apply", "admin", "recovery", "surgical_recovery"]
RepositoryScopeMode = Literal["single_repo", "cross_repo", "daemon_global"]
CAPABILITIES: frozenset[str] = frozenset(
    {"read", "write", "review", "claim", "apply", "admin", "recovery", "surgical_recovery"}
)


@dataclass(frozen=True)
class MethodEntry:
    method: str
    required_capability: Capability | None
    repository_scope: bool
    repository_scope_mode: RepositoryScopeMode | None = None
    params_schema_version: int = 1
    audit_class: str = "metadata"
    min_envelope: int = 1
    deprecated: bool = False

    def public_dict(self) -> dict[str, object]:
        return {
            "method": self.method,
            "required_capability": self.required_capability,
            "repository_scope": self.repository_scope,
            "repository_scope_mode": self.effective_repository_scope_mode,
            "params_schema_version": self.params_schema_version,
            "audit_class": self.audit_class,
            "min_envelope": self.min_envelope,
            "deprecated": self.deprecated,
        }

    @property
    def effective_repository_scope_mode(self) -> RepositoryScopeMode:
        if self.repository_scope_mode is not None:
            return self.repository_scope_mode
        return "single_repo" if self.repository_scope else "daemon_global"


_ENTRIES: tuple[MethodEntry, ...] = (
    # ----- Handshake / discovery -----
    MethodEntry("daemon.hello", None, False),
    MethodEntry("daemon.describe", "read", False),
    # ----- Read capability (per-repo) -----
    MethodEntry("status", "read", True),
    MethodEntry("why", "read", True),
    MethodEntry("doctor", "read", True),
    MethodEntry("dashboard", "read", True),
    MethodEntry("evidence.export", "read", True),
    MethodEntry("corpus.export", "read", True),
    MethodEntry("run.summary", "read", True),
    MethodEntry("run.graph", "read", True),
    MethodEntry("workflow.validate", "read", True),
    MethodEntry("workflow.plan", "read", True),
    MethodEntry("workflow.graph", "read", True),
    MethodEntry("workflow.templates.list", "read", True),
    MethodEntry("workflow.templates.show", "read", True),
    MethodEntry("workflow.generate.preview", "read", True),
    MethodEntry("list.runs", "read", True),
    MethodEntry("list.sessions", "read", True),
    MethodEntry("list.jobs", "read", True),
    MethodEntry("list.artifacts", "read", True),
    MethodEntry("list.workflows", "read", True),
    MethodEntry("worktree.list", "read", True),
    # ----- Read capability (daemon-global) -----
    MethodEntry("dashboard.all", "read", False),
    MethodEntry("repo.list", "read", False),
    # ----- Claim capability (per-repo) -----
    MethodEntry("session.register", "claim", True),
    MethodEntry("session.close", "claim", True),
    MethodEntry("work.claim_next", "claim", True),
    MethodEntry("work.ack", "claim", True),
    MethodEntry("work.heartbeat", "claim", True),
    MethodEntry("work.release", "claim", True),
    MethodEntry("supervise.start", "claim", True),
    MethodEntry("supervise.send", "claim", True),
    MethodEntry("supervise.stop", "claim", True),
    MethodEntry("supervise.status", "read", True),
    MethodEntry("supervise.list", "read", True),
    MethodEntry("supervise.reattach_status", "read", True),
    # ----- Write capability (per-repo) -----
    MethodEntry("work.send_message", "write", True),
    MethodEntry("work.block", "write", True),
    MethodEntry("work.complete", "write", True),
    MethodEntry("artifact.publish", "write", True),
    MethodEntry("worktree.create", "write", True),
    MethodEntry("worktree.release", "write", True),
    MethodEntry("workflow.init", "write", True),
    MethodEntry("workflow.generate", "write", True),
    MethodEntry("workflow.upgrade", "write", True),
    MethodEntry("dogfood.publish_on_behalf", "write", True),
    # ----- Review capability (per-repo) -----
    MethodEntry("review.submit", "review", True),
    MethodEntry("review.verdict", "review", True),
    # ----- Admin capability (per-repo) -----
    MethodEntry("review.override", "admin", True),
    MethodEntry("decision.record", "admin", True),
    MethodEntry("checkpoint.resolve", "admin", True),
    MethodEntry("branch.confirm", "admin", True),
    MethodEntry("run.prepare", "admin", True),
    MethodEntry("run.start", "admin", True),
    MethodEntry("run.pause", "admin", True),
    MethodEntry("run.resume", "admin", True),
    MethodEntry("run.cancel", "admin", True),
    MethodEntry("run.retry_job", "admin", True),
    MethodEntry("repo.init", "admin", True),
    # ----- Recovery capability (per-repo) -----
    MethodEntry("recovery.stale_leases", "recovery", True),
    MethodEntry("recovery.requeue_stale", "recovery", True),
    MethodEntry("recovery.cancel_job", "recovery", True),
    MethodEntry("recovery.process_reconcile", "recovery", True),
    MethodEntry("recovery.resume", "recovery", True),
    MethodEntry("recovery.auto", "recovery", True),
    MethodEntry("recovery.watch", "recovery", True),
    # ----- Apply capability (per-repo) -----
    MethodEntry("apply.reviewed_patch", "apply", True),
    MethodEntry("apply.receipt.show", "read", True),
    MethodEntry("apply.receipt.verify", "read", True),
    # ----- Surgical recovery (per-repo) -----
    MethodEntry("dogfood.surgical_recovery", "surgical_recovery", True),
    # ----- Admin capability (daemon-global) -----
    MethodEntry("repo.add", "admin", False),
    MethodEntry("repo.remove", "admin", False),
    MethodEntry("daemon.token.create", "admin", False),
    MethodEntry("daemon.token.revoke", "admin", False),
    MethodEntry("daemon.token.rotate", "admin", False),
    MethodEntry("daemon.key.rotate", "admin", False),
    MethodEntry("daemon.shutdown", "admin", False),
    MethodEntry("daemon.migrate", "admin", False),
    MethodEntry("daemon.migrate_repo_local", "admin", False),
    # ----- Cross-repo coordination -----
    MethodEntry("cross_repo.list", "read", False, repository_scope_mode="cross_repo"),
    MethodEntry("cross_repo.describe", "read", False, repository_scope_mode="cross_repo"),
    MethodEntry("cross_repo.why", "read", False, repository_scope_mode="cross_repo"),
    MethodEntry("cross_repo.cancel", "recovery", False, repository_scope_mode="cross_repo"),
    # ----- Deprecated aliases (RFC 0030 legacy vocabulary) -----
    # Pre-RFC 0043 the registry used undotted names for the repo-local
    # mutation surface. The dotted names above are the RFC 0043 §5
    # canonical methods; the aliases below remain so existing server-side
    # routing (CLI_ROUTES) and integration tests keep resolving while
    # clients migrate. The CLI does not emit these names anymore.
    MethodEntry("ack", "claim", True, deprecated=True),
    MethodEntry("heartbeat", "claim", True, deprecated=True),
    MethodEntry("release", "claim", True, deprecated=True),
    MethodEntry("block", "write", True, deprecated=True),
    MethodEntry("complete", "write", True, deprecated=True),
    MethodEntry("publish_artifact", "write", True, deprecated=True),
    MethodEntry("claim_next", "claim", True, deprecated=True),
    MethodEntry("verdict", "review", True, deprecated=True),
    MethodEntry("submit_review", "review", True, deprecated=True),
)

METHOD_REGISTRY: dict[str, MethodEntry] = {entry.method: entry for entry in _ENTRIES}
METHODS_ETAG = "sha256:" + hashlib.sha256(
    json_dumps([entry.public_dict() for entry in sorted(_ENTRIES, key=lambda item: item.method)]).encode("utf-8")
).hexdigest()


def describe_methods() -> dict[str, object]:
    return {
        "methods_etag": METHODS_ETAG,
        "methods": [entry.public_dict() for entry in sorted(_ENTRIES, key=lambda item: item.method)],
    }
