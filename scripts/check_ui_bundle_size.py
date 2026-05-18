#!/usr/bin/env python3
"""Check the committed web UI bundle size."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

DEFAULT_MAX_BYTES = 12_000_000
DEFAULT_MAX_FILES = 32
DEFAULT_MAX_SHARED_CHUNKS = 4
IGNORED_NAMES = frozenset({"manifest.sha256"})


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument(
        "--max-bytes",
        type=int,
        default=int(os.environ.get("STRIATUM_UI_BUNDLE_MAX_BYTES", DEFAULT_MAX_BYTES)),
    )
    parser.add_argument(
        "--max-files",
        type=int,
        default=int(os.environ.get("STRIATUM_UI_BUNDLE_MAX_FILES", DEFAULT_MAX_FILES)),
    )
    parser.add_argument(
        "--max-shared-chunks",
        type=int,
        default=int(
            os.environ.get("STRIATUM_UI_BUNDLE_MAX_SHARED_CHUNKS", DEFAULT_MAX_SHARED_CHUNKS)
        ),
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
    files = [path for path in root.rglob("*") if path.is_file() and path.name not in IGNORED_NAMES]
    shared_chunks = list(root.glob("island-shared-*.js"))
    if len(files) > args.max_files:
        print(
            "ui-bundle-size: "
            f"{len(files)} files exceeds limit {args.max_files}; "
            "check for stale generated chunks before raising STRIATUM_UI_BUNDLE_MAX_FILES",
            file=sys.stderr,
        )
        return 1
    if len(shared_chunks) > args.max_shared_chunks:
        print(
            "ui-bundle-size: "
            f"{len(shared_chunks)} island-shared chunks exceeds limit {args.max_shared_chunks}; "
            "group dynamic imports before raising STRIATUM_UI_BUNDLE_MAX_SHARED_CHUNKS",
            file=sys.stderr,
        )
        return 1
    if total > args.max_bytes:
        print(
            "ui-bundle-size: "
            f"{total} bytes exceeds limit {args.max_bytes}; "
            "raise STRIATUM_UI_BUNDLE_MAX_BYTES only with an explicit review note",
            file=sys.stderr,
        )
        return 1
    print(
        "ui-bundle-size: "
        f"{total} bytes <= {args.max_bytes}; "
        f"{len(files)} files <= {args.max_files}; "
        f"{len(shared_chunks)} shared chunks <= {args.max_shared_chunks}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
