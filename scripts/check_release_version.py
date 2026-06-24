#!/usr/bin/env python3
"""Check release-facing version strings stay in lockstep.

This is intentionally narrow: it only guards the low-noise release surfaces that
must be true before tagging a release. Semantic docs drift still needs operator
review, but VERSION, README's Project Status row, and the CHANGELOG release
header should never disagree mechanically.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path


SEMVER_RE = re.compile(r"^\d+\.\d+\.\d+$")
CHANGELOG_HEADER_RE = re.compile(r"^## v(?P<version>\d+\.\d+\.\d+) — (?P<date>\d{4}-\d{2}-\d{2})$", re.MULTILINE)
README_VERSION_RE = re.compile(
    r"^\| Version \| v(?P<version>\d+\.\d+\.\d+) — latest release published (?P<date>\d{4}-\d{2}-\d{2});",
    re.MULTILINE,
)


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    version = (root / "VERSION").read_text(encoding="utf-8").strip()
    errors: list[str] = []

    if not SEMVER_RE.fullmatch(version):
        errors.append(f"VERSION must be plain semver X.Y.Z, got {version!r}")

    changelog = (root / "CHANGELOG.md").read_text(encoding="utf-8")
    readme = (root / "README.md").read_text(encoding="utf-8")

    changelog_matches = [m for m in CHANGELOG_HEADER_RE.finditer(changelog) if m.group("version") == version]
    if not changelog_matches:
        errors.append(f"CHANGELOG.md is missing a release header for v{version}")
        changelog_date = None
    else:
        changelog_date = changelog_matches[0].group("date")

    readme_match = README_VERSION_RE.search(readme)
    if not readme_match:
        errors.append("README.md Project Status is missing a parseable Version row")
        readme_version = None
        readme_date = None
    else:
        readme_version = readme_match.group("version")
        readme_date = readme_match.group("date")
        if readme_version != version:
            errors.append(f"README.md Version row says v{readme_version}, VERSION says v{version}")

    if changelog_date and readme_date and changelog_date != readme_date:
        errors.append(
            f"README.md latest release date {readme_date} disagrees with CHANGELOG.md v{version} date {changelog_date}"
        )

    if errors:
        for error in errors:
            print(f"[FAIL] {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
