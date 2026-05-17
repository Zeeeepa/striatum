from __future__ import annotations

import pytest

from striatum.web.run_posture_verdicts import (
    RunPostureVerdictsPayloadError,
    run_posture_verdicts_template_context,
)


def test_posture_verdicts_template_context_shapes_daemon_payload() -> None:
    context = run_posture_verdicts_template_context(
        {
            "run": {"run_id": "run_1", "state": "running"},
            "posture": "security",
            "verdicts": [
                {"verdict_id": "v1", "workflow_job_id": "review"},
                "ignored",
            ],
        },
        requested_posture="fallback",
    )

    assert context == {
        "run": {"run_id": "run_1", "state": "running"},
        "posture": "security",
        "verdicts": [{"verdict_id": "v1", "workflow_job_id": "review"}],
    }


def test_posture_verdicts_template_context_uses_requested_posture_fallback() -> None:
    context = run_posture_verdicts_template_context(
        {"run": {"run_id": "run_1"}, "verdicts": []},
        requested_posture="threat_model",
    )

    assert context["posture"] == "threat_model"


@pytest.mark.parametrize(
    "payload",
    [
        {"verdicts": []},
        {"run": [], "verdicts": []},
        {"run": {"run_id": "run_1"}},
        {"run": {"run_id": "run_1"}, "verdicts": {}},
    ],
)
def test_posture_verdicts_template_context_rejects_missing_fields(
    payload: dict[str, object],
) -> None:
    with pytest.raises(
        RunPostureVerdictsPayloadError,
        match="daemon posture verdict DTO missing fields",
    ):
        run_posture_verdicts_template_context(
            payload,
            requested_posture="security",
        )
