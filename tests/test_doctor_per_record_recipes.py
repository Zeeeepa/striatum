from __future__ import annotations

from pathlib import Path

import pytest

from striatum.service import _jinja_env
from striatum.web import doctor as web_doctor


def test_doctor_renders_per_record_recovery_recipe(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def fake_call_repo_method(repo: Path, method: str, params: dict[str, object]) -> dict[str, object]:
        assert repo == tmp_path
        assert method == "doctor"
        assert params == {"verbose": True}
        return {
            "ok": False,
            "problems": ["process is running with expired lease"],
            "problem_records": [
                {
                    "check": "process_running_with_expired_lease",
                    "id": "proc_doctor_recipe",
                    "context": {
                        "run_id": "run_doctor_recipe",
                        "job_id": "job_doctor_recipe",
                    },
                },
            ],
        }

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)

    response = web_doctor.doctor_page_response(tmp_path)
    body = _jinja_env().get_template("doctor.html").render(
        doctor=response.doctor,
        problem_groups=response.problem_groups,
    )

    assert "process_running_with_expired_lease" in body
    assert "striatum recovery process-reconcile --run-id run_doctor_recipe" in body
