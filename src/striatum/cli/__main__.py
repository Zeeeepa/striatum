"""Module entry point so ``python -m striatum.cli`` keeps working."""

from __future__ import annotations

from striatum.cli import main

if __name__ == "__main__":
    raise SystemExit(main())
