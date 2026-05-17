"""Template environment helpers for the local web UI."""

from __future__ import annotations

from typing import Any

__all__ = ["escape_html", "jinja_env", "jinja_env_factory"]


def escape_html(s: str) -> str:
    return (
        s.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
        .replace("'", "&#x27;")
    )


def jinja_env() -> Any:
    """Return a Jinja2 environment that loads bundled web templates."""
    return jinja_env_factory()


def jinja_env_factory() -> Any:
    from functools import lru_cache

    @lru_cache(maxsize=1)
    def _build() -> Any:
        from jinja2 import Environment, PackageLoader, select_autoescape

        return Environment(
            loader=PackageLoader("striatum.web", "templates"),
            autoescape=select_autoescape(["html"]),
            keep_trailing_newline=False,
        )

    return _build()
