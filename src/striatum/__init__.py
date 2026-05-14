"""Local-first terminal-agent orchestration MVP."""

from __future__ import annotations

from importlib.metadata import PackageNotFoundError, version as _pkg_version

__all__ = ["__version__"]

# Single source of truth: pyproject.toml's [project].version, surfaced via
# the installed distribution metadata. The previous hardcoded literal here
# drifted 7 versions behind pyproject.toml (1.37.0 vs 1.44.1) because
# release commits bumped pyproject.toml but not this file. Reading from
# importlib.metadata eliminates the drift entirely.
try:
    __version__ = _pkg_version("striatum-orchestrator")
except PackageNotFoundError:  # pragma: no cover — only when installing from source without metadata
    __version__ = "0.0.0+local"
