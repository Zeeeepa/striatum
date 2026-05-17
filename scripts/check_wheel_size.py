#!/usr/bin/env python3
"""Check built wheel size."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

DEFAULT_MAX_BYTES = 25_000_000


def _find_wheel(path: Path) -> Path | None:
    if path.is_file() and path.suffix == ".whl":
        return path
    if not path.is_dir():
        return None
    wheels = sorted(path.glob("*.whl"))
    if len(wheels) != 1:
        return None
    return wheels[0]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--wheel", type=Path, required=True)
    parser.add_argument(
        "--max-bytes",
        type=int,
        default=int(os.environ.get("STRIATUM_WHEEL_MAX_BYTES", DEFAULT_MAX_BYTES)),
    )
    args = parser.parse_args()

    wheel = _find_wheel(args.wheel)
    if wheel is None:
        print(
            f"wheel-size: expected one wheel file at or under {args.wheel}",
            file=sys.stderr,
        )
        return 1

    total = wheel.stat().st_size
    if total > args.max_bytes:
        print(
            "wheel-size: "
            f"{wheel.name} is {total} bytes, exceeds limit {args.max_bytes}; "
            "raise STRIATUM_WHEEL_MAX_BYTES only with an explicit review note",
            file=sys.stderr,
        )
        return 1
    print(f"wheel-size: {wheel.name} is {total} bytes <= {args.max_bytes}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
