"""RFC 0020 V1: recovery_policy block validator + defaults."""

from __future__ import annotations

from typing import Any, Mapping

from striatum.errors import WorkflowError

ALLOWED_HOOK_KINDS: frozenset[str] = frozenset({"marker_file", "webhook", "shell"})

# Runner-level defaults. A workflow that omits `recovery_policy`
# inherits these; an `auto` invocation against such a workflow is
# diagnostic-only (autonomous_* default off).
DEFAULT_POLICY: dict[str, Any] = {
    "autonomous_review_requeue": False,
    "autonomous_process_reconcile": False,
    "checkpoint_timeout_seconds": 14400,   # 4 hours
    "eligible_after_seconds": 600,         # 10 minutes
    "max_requeues_per_sweep": 8,
    "max_total_requeues_per_job": 3,
    "escalation_hook": None,
}


def validate_recovery_policy(payload: Any, *, workflow_id: str | None = None) -> None:
    """Raise :class:`WorkflowError` when ``payload`` is malformed.

    The block is optional. ``None`` is a no-op.
    """
    if payload is None:
        return
    if not isinstance(payload, Mapping):
        raise WorkflowError("recovery_policy must be an object")
    bool_fields = ("autonomous_review_requeue", "autonomous_process_reconcile")
    for field in bool_fields:
        if field in payload and not isinstance(payload[field], bool):
            raise WorkflowError(f"recovery_policy.{field} must be a boolean")
    int_fields = (
        ("checkpoint_timeout_seconds", 0),
        ("eligible_after_seconds", 0),
        ("max_requeues_per_sweep", 0),
        ("max_total_requeues_per_job", 0),
    )
    for field, lower in int_fields:
        if field in payload:
            value = payload[field]
            if not isinstance(value, int) or isinstance(value, bool) or value < lower:
                raise WorkflowError(
                    f"recovery_policy.{field} must be a non-negative integer"
                )
    hook = payload.get("escalation_hook")
    if hook is None:
        return
    if not isinstance(hook, Mapping):
        raise WorkflowError("recovery_policy.escalation_hook must be an object")
    kind = hook.get("kind")
    if kind not in ALLOWED_HOOK_KINDS:
        allowed = ", ".join(sorted(ALLOWED_HOOK_KINDS))
        raise WorkflowError(
            f"recovery_policy.escalation_hook.kind must be one of: {allowed}"
        )
    if kind == "marker_file":
        path = hook.get("path")
        if not isinstance(path, str) or not path:
            raise WorkflowError("escalation_hook.path is required for marker_file")
        if path.startswith(".striatum/") or path.startswith("/"):
            raise WorkflowError(
                "escalation_hook.path must be repo-relative and outside .striatum/"
            )
        if ".." in path.split("/"):
            raise WorkflowError("escalation_hook.path may not contain '..'")
    elif kind == "webhook":
        url = hook.get("url")
        if not isinstance(url, str) or not (
            url.startswith("http://") or url.startswith("https://")
        ):
            raise WorkflowError(
                "escalation_hook.url must start with http:// or https://"
            )
    elif kind == "shell":
        command = hook.get("command")
        if not isinstance(command, list) or not command:
            raise WorkflowError(
                "escalation_hook.command must be a non-empty list of strings"
            )
        for part in command:
            if not isinstance(part, str):
                raise WorkflowError(
                    "escalation_hook.command must contain only strings"
                )


def resolve_policy(
    *,
    workflow_payload: Any,
    cli_overrides: Mapping[str, Any] | None = None,
) -> dict[str, Any]:
    """Merge defaults + workflow.recovery_policy + CLI overrides.

    Returns a freshly-allocated dict with every default-or-overridden
    field present. ``policy_source`` is set for envelope reporting.
    """
    resolved: dict[str, Any] = dict(DEFAULT_POLICY)
    source = "default"
    if isinstance(workflow_payload, Mapping):
        resolved.update({k: v for k, v in workflow_payload.items() if k in resolved or k == "escalation_hook"})
        source = "workflow"
    if cli_overrides:
        for key, value in cli_overrides.items():
            if value is None:
                continue
            resolved[key] = value
        if any(v is not None for v in cli_overrides.values()):
            source = "cli_override"
    resolved["policy_source"] = source
    return resolved
