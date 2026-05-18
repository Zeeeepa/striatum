"""Artifact contract compatibility surface.

Neutral artifact contract helpers live in :mod:`striatum.artifact_contracts`.
The legacy repo-local SQLite publisher remains importable from this module for
compatibility, but it is loaded only when callers invoke those legacy helpers.
"""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.artifact_contracts import (
    ALLOWED_ARTIFACT_KINDS,
    FRONT_MATTER_SCHEMAS,
    MARKDOWN_SUFFIXES,
    FrontMatterField,
    FrontMatterSchema,
    _canonical_byline_form,
    _first_author_line,
    _front_matter_block,
    _front_matter_body,
    _parse_front_matter,
    _strip_markdown_decoration,
    ensure_required_front_matter,
    markdown_title_block_author_lines,
    parse_artifact_front_matter,
    validate_artifact_front_matter,
)

__all__ = [
    "ALLOWED_ARTIFACT_KINDS",
    "FRONT_MATTER_SCHEMAS",
    "MARKDOWN_SUFFIXES",
    "FrontMatterField",
    "FrontMatterSchema",
    "_canonical_byline_form",
    "_first_author_line",
    "_front_matter_block",
    "_front_matter_body",
    "_enforce_required_attestation_for_artifact",
    "_is_operator_byline",
    "_lane_evidence_present",
    "_parse_front_matter",
    "_strip_markdown_decoration",
    "ensure_required_front_matter",
    "expected_author_line",
    "markdown_title_block_author_lines",
    "parse_artifact_front_matter",
    "publish_artifact",
    "validate_artifact_front_matter",
    "validate_optional_markdown_author_line",
]


def publish_artifact(*args: Any, **kwargs: Any) -> dict[str, object]:
    return cast(dict[str, object], _legacy_artifacts().publish_artifact(*args, **kwargs))


def validate_optional_markdown_author_line(*args: Any, **kwargs: Any) -> None:
    _legacy_artifacts().validate_optional_markdown_author_line(*args, **kwargs)


def expected_author_line(*args: Any, **kwargs: Any) -> str:
    return cast(str, _legacy_artifacts().expected_author_line(*args, **kwargs))


def _is_operator_byline(*args: Any, **kwargs: Any) -> bool:
    return cast(bool, _legacy_artifacts()._is_operator_byline(*args, **kwargs))


def _lane_evidence_present(*args: Any, **kwargs: Any) -> bool:
    return cast(bool, _legacy_artifacts()._lane_evidence_present(*args, **kwargs))


def _enforce_required_attestation_for_artifact(*args: Any, **kwargs: Any) -> None:
    _legacy_artifacts()._enforce_required_attestation_for_artifact(*args, **kwargs)


def _legacy_artifacts() -> Any:
    return import_module("striatum.legacy_sqlite.artifacts")
