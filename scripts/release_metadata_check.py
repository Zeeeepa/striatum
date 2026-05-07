"""Check installed package metadata needed for release readiness."""

from __future__ import annotations

from importlib import metadata

EXPECTED_NAME = "striatum"
EXPECTED_VERSION = "0.1.0"
EXPECTED_LICENSE = "Apache-2.0"
EXPECTED_CONSOLE_SCRIPT = "striatum"
EXPECTED_ENTRY_POINT = "striatum.cli:main"


def main() -> None:
    dist = metadata.distribution(EXPECTED_NAME)
    meta = dist.metadata
    entry_points = {
        entry_point.name: entry_point.value
        for entry_point in metadata.entry_points(group="console_scripts")
    }

    checks = {
        "Name": meta["Name"] == EXPECTED_NAME,
        "Version": meta["Version"] == EXPECTED_VERSION,
        "License-Expression": meta["License-Expression"] == EXPECTED_LICENSE,
        "console_scripts.striatum": entry_points.get(EXPECTED_CONSOLE_SCRIPT)
        == EXPECTED_ENTRY_POINT,
    }
    failed = [name for name, ok in checks.items() if not ok]
    if failed:
        raise SystemExit(f"release metadata check failed: {', '.join(failed)}")


if __name__ == "__main__":
    main()
