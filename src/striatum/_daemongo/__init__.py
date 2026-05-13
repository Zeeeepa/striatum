"""Package-data resolver for the shipped Go daemon binary (RFC 0039 Phase 2).

The Go daemon is built per-platform and copied into
``src/striatum/_daemongo/binaries/`` by ``make daemon-go-release`` (cross-
compile) or ``make daemon-go-install`` (host-only). A wheel install ships
those binaries via ``[tool.setuptools.package-data]``. At runtime
``find_binary()`` returns the path to the binary for the current host, or
``None`` if it is missing — in which case the CLI dispatch (Track A) falls
through to ``STRIATUMD_GO_BIN`` and ``go/bin/striatumd``.

This module deliberately imports nothing from the Go source tree; the Go
daemon is a separate compilation unit. The Python side only needs to locate
the binary on disk.
"""

from __future__ import annotations

import platform
import sys
from pathlib import Path

__all__ = ["find_binary", "platform_slug"]


def platform_slug() -> str:
    """Return the ``<sys.platform>-<machine>`` slug for the current host.

    The slug matches the suffix written by ``make daemon-go-release``
    (e.g. ``linux-x86_64``, ``darwin-arm64``).
    """
    return f"{sys.platform}-{platform.machine()}"


def find_binary() -> Path | None:
    """Return the path to the bundled Go daemon binary for this host, or
    ``None`` if one was not shipped.

    Returning ``None`` is the expected case for sdist installs and for
    platforms that did not produce a binary; callers should fall back to
    ``STRIATUMD_GO_BIN`` and ``go/bin/striatumd``.
    """
    base = Path(__file__).resolve().parent / "binaries"
    if not base.is_dir():
        return None
    candidate = base / f"striatumd-{platform_slug()}"
    if candidate.is_file():
        return candidate
    return None
