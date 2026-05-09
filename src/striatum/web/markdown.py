"""RFC 0023 V1: Markdown rendering for chat output, file view, and
inline artifact rendering.

Uses ``markdown-it-py`` configured in CommonMark mode with raw HTML
disabled at the parser level so no ``<script>`` / ``<iframe>`` tags
slip through; no separate sanitizer is needed for V1. Tables and
strikethrough are enabled because striatum's own dogfood artifacts use
GFM tables.
"""

from __future__ import annotations

from typing import Any

__all__ = ["render"]


_MD: Any | None = None


def _get_md() -> Any:
    global _MD
    if _MD is None:
        from markdown_it import MarkdownIt

        md = MarkdownIt(
            "commonmark",
            {
                "breaks": False,
                "html": False,
                "linkify": False,
                "typographer": False,
            },
        )
        md.enable("table")
        md.enable("strikethrough")
        _MD = md
    return _MD


def render(source: str) -> str:
    """Render Markdown source to HTML.

    The parser is configured with ``html: False`` so any literal HTML
    in the source is escaped rather than emitted; this is V1's
    sanitization story per RFC 0023 § "Markdown library choice."
    """
    if not isinstance(source, str):
        return ""
    return str(_get_md().render(source))
