#!/usr/bin/env python3
"""Guard against stale docs by checking mechanically-detectable rot.

Two repo-agnostic, low-false-positive checks:

1. Markdown link integrity: every inline ``[text](target)`` / ``![alt](target)``
   whose target is a repo-relative path must resolve to a file or directory that
   exists inside this repo. URLs, ``mailto:``, anchors, globbed paths, and any
   target that resolves outside the repo root are out of scope (the last avoids
   false positives on links into sibling checkouts).

2. Version consistency: if ``scripts/check_release_version.py`` exists, run it so
   VERSION / README / CHANGELOG stay in lockstep.

Semantic staleness — a status doc or board that contradicts the actual state of
the repo — is a human/agent responsibility per AGENTS.md, not something this
guard can detect reliably; cross-repo decision/commit references make automated
ID resolution unsafe here. This catches broken links and version drift only.

Adapted from engram's ``scripts/check_artifact_refs.py`` (reference-integrity
idea), generalized to plain markdown links.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
SKIP_PREFIXES = ("http://", "https://", "mailto:", "tel:", "file:", "data:", "#", "//")
SKIP_DIRS = frozenset(
    {".git", "__pycache__", ".venv", "node_modules", ".pytest_cache", ".ruff_cache", ".striatum"}
)
IGNORE_FILE = ".check-docs-ignore"


def ignore_prefixes(root: Path) -> tuple[str, ...]:
    """Repo-relative path prefixes to skip, from an optional ``.check-docs-ignore``.

    Lets a repo exclude frozen provenance (accepted RFCs, archives, review
    records) whose outbound links are point-in-time and must not be rewritten.
    """
    ignore = root / IGNORE_FILE
    if not ignore.is_file():
        return ()
    prefixes = []
    for line in ignore.read_text(encoding="utf-8").splitlines():
        entry = line.strip()
        if entry and not entry.startswith("#"):
            prefixes.append(entry.rstrip("/") + "/")
    return tuple(prefixes)


def tracked_markdown(root: Path, skip: tuple[str, ...]) -> list[Path]:
    """Return tracked ``*.md`` files, falling back to a filtered walk."""
    try:
        out = subprocess.run(
            ["git", "-C", str(root), "ls-files"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout
        rels = [line for line in out.splitlines() if line.endswith(".md")]
        if rels:
            return sorted({root / rel for rel in rels if not rel.startswith(skip)})
    except (subprocess.CalledProcessError, FileNotFoundError):
        pass
    out_paths = []
    for path in root.rglob("*.md"):
        rel = path.relative_to(root)
        if any(part in SKIP_DIRS for part in rel.parts):
            continue
        if str(rel).startswith(skip):
            continue
        out_paths.append(path)
    return sorted(out_paths)


def link_target(raw: str) -> str | None:
    """Return a checkable repo-relative path from a markdown link, or None."""
    text = raw.strip()
    if text.startswith("<") and ">" in text:
        text = text[1 : text.index(">")]
    else:
        text = text.split()[0] if text.split() else ""
    if not text or text.startswith(SKIP_PREFIXES) or text.startswith(("~", "/")):
        return None
    if "*" in text:
        return None
    text = text.split("#", 1)[0].split("?", 1)[0]
    return text or None


def check_links(root: Path) -> list[str]:
    errors: list[str] = []
    skip = ignore_prefixes(root)
    for md in tracked_markdown(root, skip):
        try:
            lines = md.read_text(encoding="utf-8").splitlines()
        except (OSError, UnicodeDecodeError):
            continue
        for lineno, line in enumerate(lines, start=1):
            for match in LINK_RE.finditer(line):
                target = link_target(match.group(1))
                if target is None:
                    continue
                dest = (md.parent / target).resolve()
                try:
                    dest.relative_to(root)
                except ValueError:
                    continue  # outside the repo root — out of scope
                if not dest.exists():
                    errors.append(f"{md.relative_to(root)}:{lineno}: broken link -> {target}")
    return errors


def check_version(root: Path) -> list[str]:
    script = root / "scripts" / "check_release_version.py"
    if not script.exists():
        return []
    proc = subprocess.run([sys.executable, str(script)], capture_output=True, text=True)
    if proc.returncode != 0:
        return [f"version consistency failed: {(proc.stdout + proc.stderr).strip()}"]
    return []


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Check docs for broken local references.")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args(argv)
    root = args.root.resolve()

    errors = check_links(root) + check_version(root)
    for error in errors:
        print(f"[FAIL] {error}")
    if errors:
        print(f"\ncheck-docs: {len(errors)} problem(s)")
        return 1
    print("check-docs: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
