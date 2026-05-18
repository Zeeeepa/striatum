"""Console entry point for the ``striatumd`` Go daemon shim."""

from __future__ import annotations

import sys
from collections.abc import Sequence


def main(argv: Sequence[str] | None = None) -> int:
    """Launch the supported daemon without importing the legacy Python daemon."""

    args = list(sys.argv[1:] if argv is None else argv)
    if args[:1] == ["--foreground"]:
        args = args[1:]
    from striatum.cli import main as cli_main

    return cli_main(["daemon", "start", *args])
