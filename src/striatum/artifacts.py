"""Artifact contract compatibility surface.

Neutral artifact contract helpers live in :mod:`striatum.artifact_contracts`.
The legacy repo-local SQLite publisher remains importable from this module for
compatibility, but it is loaded only when callers invoke those legacy helpers.
"""

from __future__ import annotations


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
    "_parse_front_matter",
    "_strip_markdown_decoration",
    "ensure_required_front_matter",
    "markdown_title_block_author_lines",
    "parse_artifact_front_matter",
    "validate_artifact_front_matter",
]
