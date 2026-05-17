"""Template-context shaping for run posture verdict drill-down pages."""

from __future__ import annotations

from typing import Any, Mapping


class RunPostureVerdictsPayloadError(ValueError):
    """Raised when the daemon posture-verdict DTO is missing required fields."""


def run_posture_verdicts_template_context(
    payload: Mapping[str, Any],
    *,
    requested_posture: str,
) -> dict[str, Any]:
    """Return the Jinja context for ``run_posture_verdicts.html``."""

    run_raw = payload.get("run")
    verdicts_raw = payload.get("verdicts")
    if not isinstance(run_raw, Mapping) or not isinstance(verdicts_raw, list):
        raise RunPostureVerdictsPayloadError(
            "daemon posture verdict DTO missing fields"
        )

    return {
        "run": dict(run_raw),
        "posture": str(payload.get("posture") or requested_posture),
        "verdicts": [
            dict(verdict)
            for verdict in verdicts_raw
            if isinstance(verdict, Mapping)
        ],
    }


__all__ = [
    "RunPostureVerdictsPayloadError",
    "run_posture_verdicts_template_context",
]
