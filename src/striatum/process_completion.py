"""Process-adapter completion envelope helpers and compatibility wrappers."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

ENVELOPE_VERSION = "striatum.process_adapter.envelope.v1"

__all__ = [
    "ENVELOPE_VERSION",
    "block_job_with_envelope",
    "build_diagnostic_envelope",
    "build_recovery_commands",
    "evaluate_and_block_after_reconcile",
    "evaluate_and_block_inline",
    "pick_inline_blocker_kind",
    "validate_outputs",
]


def build_diagnostic_envelope(
    *,
    process_id: str,
    command: list[str],
    exit_code: int | None,
    duration_seconds: float,
    timeout_seconds: int | None,
    missing_artifact_paths: list[str],
    review_verdict_missing: bool,
    recovery_commands: list[str],
) -> JsonObject:
    """Assemble the privacy-safe V1 diagnostic envelope."""
    return {
        "envelope_version": ENVELOPE_VERSION,
        "process_id": process_id,
        "command": list(command),
        "exit_code": exit_code,
        "duration_seconds": round(duration_seconds, 3),
        "timeout_seconds": timeout_seconds,
        "missing_artifact_paths": list(missing_artifact_paths),
        "review_verdict_missing": review_verdict_missing,
        "recovery_commands": list(recovery_commands),
    }


def pick_inline_blocker_kind(
    *,
    exit_code: int | None,
    timed_out: bool,
    missing_artifact_paths: list[str],
    review_verdict_missing: bool,
) -> str | None:
    """Return the highest-priority blocker kind for an inline failure."""
    if exit_code is not None and exit_code != 0:
        return "process_exit_nonzero"
    if timed_out:
        return "process_timeout_exceeded"
    if review_verdict_missing:
        return "process_review_verdict_missing"
    if missing_artifact_paths:
        return "process_outputs_missing"
    return None


def build_recovery_commands(
    *,
    run_id: str,
    job_id: str,
    session_id: str,
    blocker_kind: str,
    missing_artifact_paths: list[str],
    review_verdict_missing: bool,
    blocker_id: str | None = None,
) -> list[str]:
    """Return shell-string operator suggestions for the given failure mode."""
    cmds: list[str] = []
    for path in missing_artifact_paths:
        cmds.append(
            "striatum publish-artifact "
            f"--session-id {session_id} --job-id {job_id} "
            f"--lease-id <lease_id> --kind <kind> "
            f"--logical-name <logical_name> --path {path}"
        )
    if review_verdict_missing:
        cmds.append(
            "striatum verdict "
            f"--session-id {session_id} --job-id {job_id} "
            f"--lease-id <lease_id> --verdict <accept|accept_with_findings|needs_revision|reject>"
        )
    blocker_arg = blocker_id if blocker_id is not None else "<blocker_id>"
    force_arg = (
        " --force"
        if blocker_kind in {"process_exit_nonzero", "process_timeout_exceeded"}
        else ""
    )
    cmds.append(f"striatum recovery resume --blocker-id {blocker_arg}{force_arg}")
    if not review_verdict_missing:
        cmds.append(
            f"striatum recovery resume --blocker-id {blocker_arg}{force_arg} "
            f"--complete --session-id {session_id} --summary \"<summary>\""
        )
    if blocker_kind == "process_lost_with_outputs_missing":
        cmds.append(f"striatum recovery process-reconcile --run-id {run_id}")
    return cmds


def validate_outputs(*args: Any, **kwargs: Any) -> tuple[list[str], bool]:
    return cast(
        tuple[list[str], bool],
        _legacy_process_completion().validate_outputs(*args, **kwargs),
    )


def block_job_with_envelope(*args: Any, **kwargs: Any) -> str:
    return cast(
        str,
        _legacy_process_completion().block_job_with_envelope(*args, **kwargs),
    )


def evaluate_and_block_inline(
    *args: Any, **kwargs: Any
) -> tuple[str | None, JsonObject | None]:
    return cast(
        tuple[str | None, JsonObject | None],
        _legacy_process_completion().evaluate_and_block_inline(*args, **kwargs),
    )


def evaluate_and_block_after_reconcile(
    *args: Any, **kwargs: Any
) -> tuple[str | None, JsonObject | None]:
    return cast(
        tuple[str | None, JsonObject | None],
        _legacy_process_completion().evaluate_and_block_after_reconcile(*args, **kwargs),
    )


def _legacy_process_completion() -> Any:
    return import_module("striatum.legacy_sqlite.process_completion")
