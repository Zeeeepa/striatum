"""Process-adapter neutral helpers and legacy compatibility wrappers."""

from __future__ import annotations

import string
from collections.abc import Mapping

__all__ = [
    "_expand_lane_env_values",
]


def _expand_lane_env_values(
    raw_env: Mapping[str, str], expansion: Mapping[str, str]
) -> dict[str, str]:
    """Expand ``${VAR}`` / ``$VAR`` references in lane env values."""
    return {
        key: string.Template(value).safe_substitute(expansion)
        for key, value in raw_env.items()
    }
