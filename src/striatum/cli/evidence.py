"""Legacy evidence CLI compatibility wrappers."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

__all__ = [
    "dependency_summary",
    "evidence_artifact_summaries",
    "evidence_export",
    "evidence_job_summaries",
    "evidence_session_summaries",
    "evidence_snapshot",
]


def evidence_export(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_evidence().evidence_export(*args, **kwargs))


def evidence_snapshot(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_evidence().evidence_snapshot(*args, **kwargs))


def evidence_session_summaries(*args: Any, **kwargs: Any) -> list[JsonObject]:
    return cast(
        list[JsonObject],
        _legacy_evidence().evidence_session_summaries(*args, **kwargs),
    )


def evidence_job_summaries(*args: Any, **kwargs: Any) -> list[JsonObject]:
    return cast(
        list[JsonObject],
        _legacy_evidence().evidence_job_summaries(*args, **kwargs),
    )


def evidence_artifact_summaries(*args: Any, **kwargs: Any) -> list[JsonObject]:
    return cast(
        list[JsonObject],
        _legacy_evidence().evidence_artifact_summaries(*args, **kwargs),
    )


def dependency_summary(*args: Any, **kwargs: Any) -> list[JsonObject]:
    return cast(list[JsonObject], _legacy_evidence().dependency_summary(*args, **kwargs))


def _legacy_evidence() -> Any:
    return import_module("striatum.legacy_sqlite.cli_evidence")
