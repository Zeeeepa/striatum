"""Generated-style daemon RPC method registry."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from typing import Literal

from striatum.db import json_dumps

Capability = Literal["read", "write", "review", "claim", "apply", "admin"]
CAPABILITIES: frozenset[str] = frozenset({"read", "write", "review", "claim", "apply", "admin"})


@dataclass(frozen=True)
class MethodEntry:
    method: str
    required_capability: Capability | None
    repository_scope: bool
    params_schema_version: int = 1
    audit_class: str = "metadata"
    min_envelope: int = 1
    deprecated: bool = False

    def public_dict(self) -> dict[str, object]:
        return {
            "method": self.method,
            "required_capability": self.required_capability,
            "repository_scope": self.repository_scope,
            "params_schema_version": self.params_schema_version,
            "audit_class": self.audit_class,
            "min_envelope": self.min_envelope,
            "deprecated": self.deprecated,
        }


_ENTRIES: tuple[MethodEntry, ...] = (
    MethodEntry("daemon.hello", None, False),
    MethodEntry("daemon.describe", "read", False),
    MethodEntry("status", "read", True),
    MethodEntry("why", "read", True),
    MethodEntry("doctor", "read", True),
    MethodEntry("dashboard", "read", True),
    MethodEntry("dashboard.all", "read", False),
    MethodEntry("evidence.export", "read", True),
    MethodEntry("workflow.validate", "write", True),
    MethodEntry("run.prepare", "write", True),
    MethodEntry("run.start", "write", True),
    MethodEntry("session.register", "write", True),
    MethodEntry("ack", "write", True),
    MethodEntry("block", "write", True),
    MethodEntry("heartbeat", "write", True),
    MethodEntry("publish_artifact", "write", True),
    MethodEntry("complete", "write", True),
    MethodEntry("release", "write", True),
    MethodEntry("claim_next", "claim", True),
    MethodEntry("verdict", "review", True),
    MethodEntry("submit_review", "review", True),
    MethodEntry("supervise.start", "write", True),
    MethodEntry("supervise.send", "write", True),
    MethodEntry("supervise.stop", "write", True),
    MethodEntry("supervise.status", "read", True),
    MethodEntry("supervise.list", "read", True),
    MethodEntry("supervise.reattach_status", "read", True),
    MethodEntry("apply.reviewed_patch", "apply", True),
    MethodEntry("apply.receipt.show", "read", True),
    MethodEntry("apply.receipt.verify", "read", True),
    MethodEntry("repo.add", "admin", False),
    MethodEntry("repo.remove", "admin", False),
    MethodEntry("daemon.token.create", "admin", False),
    MethodEntry("daemon.token.revoke", "admin", False),
    MethodEntry("daemon.key.rotate", "admin", False),
    MethodEntry("daemon.shutdown", "admin", False),
    MethodEntry("daemon.migrate", "admin", False),
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
