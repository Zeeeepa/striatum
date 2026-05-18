"""Check installed package metadata needed for release readiness.

Sources the expected distribution name and version from ``pyproject.toml`` so
the check stays in lockstep with package-name and version metadata. An
``STRIATUM_RELEASE_VERSION`` env var overrides the version for CI matrices
that need to pin a specific build.
"""

from __future__ import annotations

import os
import re
import tomllib
from importlib import metadata
from pathlib import Path


EXPECTED_LICENSE = "Apache-2.0"
EXPECTED_CONSOLE_SCRIPT = "striatum"
EXPECTED_ENTRY_POINT = "striatum.cli:main"


def _project_metadata() -> tuple[str, str]:
    override = os.environ.get("STRIATUM_RELEASE_VERSION")
    pyproject = Path(__file__).resolve().parents[1] / "pyproject.toml"
    if not pyproject.is_file():
        raise SystemExit("release metadata check failed: pyproject.toml missing")
    data = tomllib.loads(pyproject.read_text(encoding="utf-8"))
    project = data.get("project", {})
    name = project.get("name")
    if not isinstance(name, str) or not name:
        raise SystemExit("release metadata check failed: invalid pyproject project name")
    version = override or project.get("version")
    if not isinstance(version, str) or not re.fullmatch(
        r"\d+\.\d+\.\d+(?:[+-].+)?", version
    ):
        raise SystemExit(
            f"release metadata check failed: invalid pyproject version {version!r}"
        )
    return name, version


def main() -> None:
    expected_name, expected_version = _project_metadata()
    dist = metadata.distribution(expected_name)
    meta = dist.metadata
    entry_points = {
        entry_point.name: entry_point.value
        for entry_point in metadata.entry_points(group="console_scripts")
    }

    checks = {
        "Name": meta["Name"] == expected_name,
        "Version": meta["Version"] == expected_version,
        "License-Expression": meta["License-Expression"] == EXPECTED_LICENSE,
        "console_scripts.striatum": entry_points.get(EXPECTED_CONSOLE_SCRIPT)
        == EXPECTED_ENTRY_POINT,
    }
    failed = [name for name, ok in checks.items() if not ok]
    if failed:
        raise SystemExit(f"release metadata check failed: {', '.join(failed)}")


if __name__ == "__main__":
    main()
