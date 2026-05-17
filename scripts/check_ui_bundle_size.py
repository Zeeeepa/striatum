#!/usr/bin/env python3
"""Check the committed web UI bundle size."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

DEFAULT_MAX_BYTES = 12_000_000
IGNORED_NAMES = frozenset({"manifest.sha256"})


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument(
        "--max-bytes",
        type=int,
        default=int(os.environ.get("STRIATUM_UI_BUNDLE_MAX_BYTES", DEFAULT_MAX_BYTES)),
    )
    args = parser.parse_args()

    root = args.root
    if not root.is_dir():
        print(f"ui-bundle-size: missing build directory: {root}", file=sys.stderr)
        return 1
    total = sum(
        path.stat().st_size
        for path in root.rglob("*")
        if path.is_file() and path.name not in IGNORED_NAMES
    )
    if total > args.max_bytes:
        print(
            "ui-bundle-size: "
            f"{total} bytes exceeds limit {args.max_bytes}; "
            "raise STRIATUM_UI_BUNDLE_MAX_BYTES only with an explicit review note",
            file=sys.stderr,
        )
        return 1
    print(f"ui-bundle-size: {total} bytes <= {args.max_bytes}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
