"""Local in-process API wrapper for Striatum commands."""

from __future__ import annotations

import sqlite3
from pathlib import Path
from typing import Sequence

from striatum.cli import build_parser, dispatch
from striatum.db import JsonObject
from striatum.errors import StriatumError


def invoke(args: Sequence[str], *, repo: str | Path = ".") -> JsonObject:
    """Run a Striatum command through the same parser and dispatcher as the CLI.

    This is intentionally a wrapper over CLI semantics, not an alternate state
    mutation layer. MCP, TUI, and other local adapters can call this function
    when they need structured results without shelling out.
    """
    parser = build_parser()
    try:
        namespace = parser.parse_args(["--repo", str(repo), *args])
        namespace.json = True
        result = dispatch(namespace)
    except SystemExit as exc:
        code = exc.code if isinstance(exc.code, int) else 2
        return {"ok": False, "error": {"message": "invalid striatum command arguments", "code": code}}
    except StriatumError as exc:
        error: JsonObject = {"message": str(exc), "code": exc.exit_code}
        details = getattr(exc, "details", None)
        if isinstance(details, dict):
            error["details"] = details
        return {"ok": False, "error": error}
    except sqlite3.Error as exc:
        return {"ok": False, "error": {"message": str(exc), "code": 1}}
    return {"ok": True, "data": result}
