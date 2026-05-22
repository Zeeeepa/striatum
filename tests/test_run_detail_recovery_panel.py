from __future__ import annotations

from striatum.web.template_env import jinja_env


def test_run_detail_template_renders_recovery_panel_with_blocker_recipe() -> None:
    blocker = {
        "run_id": "run_1",
        "job_id": "job_1",
        "workflow_job_id": "build",
        "severity": "blocked",
        "blocker_kind": "process_outputs_missing",
        "description": "missing output",
        "recipes": ["striatum recovery process-reconcile --run-id run_1"],
    }

    html = jinja_env().get_template("run_detail.html").render(
        run={"run_id": "run_1", "state": "running", "branch_name": "main"},
        jobs=[],
        graph_svg="<svg></svg>",
        next_actions=[],
        recovery_panel={
            "run_id": "run_1",
            "recipes": [],
            "blockers": [blocker],
            "human_checkpoints": [],
            "blocked": [blocker],
            "auto_publish_recipe": "striatum recovery auto-publish --run-id run_1 --dry-run",
        },
        verdicts_by_posture={},
        sessions=[],
        phase_progress=[],
        current_phase_id=None,
        suggested_branch_name="",
        allow_dirty=False,
    )

    assert "recovery-panel" in html
    assert 'id="island-recovery-panel"' in html
    assert "/static/build/island-recovery-panel.js" in html
    assert "process_outputs_missing" in html
    assert "striatum recovery process-reconcile" in html
    assert "data-copy=" in html
    assert "striatum recovery auto-publish" in html


def test_recovery_panel_template_renders_auto_finalize_projection() -> None:
    template = jinja_env().from_string(
        "{% import '_recovery_panel.html' as recovery_ui %}"
        "{{ recovery_ui.recovery_panel(panel) }}"
    )
    html = template.render(
        panel={
            "run_id": "run_123",
            "blockers": [],
            "human_checkpoints": [],
            "blocked": [],
            "recipes": [],
            "auto_publish_recipe": None,
            "auto_finalize_dry_run": {
                "eligible_count": 1,
                "lane_finalization_summary": {
                    "auto_from_artifact": 1,
                    "manual_publish": 0,
                    "pending": 0,
                },
                "eligible": [
                    {
                        "workflow_job_id": "review_docs",
                        "lane_finalization": "auto_from_artifact",
                        "artifacts": [{"path": "docs/review.md"}],
                    }
                ],
                "skipped_count": 2,
                "skipped": [
                    {
                        "workflow_job_id": "review_api",
                        "reason": "expected_artifact validation refused",
                        "artifacts": [
                            {
                                "path": "docs/api.md",
                                "reason": "finding artifact front matter is required for auto-finalize",
                            }
                        ],
                    },
                    {
                        "workflow_job_id": "review_live",
                        "reason": "auto-finalize circuit breaker is open until 2026-05-22T00:10:00Z",
                        "cause": "circuit_breaker_open",
                        "circuit_breaker": {
                            "cause": "finalize_publish_failed",
                            "open_until": "2026-05-22T00:10:00Z",
                        },
                    },
                ],
                "policy": {
                    "live_allowed": False,
                    "circuit_breaker": {
                        "state": "open",
                        "open": [{"workflow_job_id": "review_live"}],
                    },
                },
            },
        }
    )

    assert "Auto-finalize dry run" in html
    assert "1 eligible" in html
    assert "2 refused" in html
    assert "live allowed: no" in html
    assert "breakers open: 1" in html
    assert "circuit breaker" in html
    assert "auto: 1" in html
    assert "auto_from_artifact" in html
    assert "review_docs" in html
    assert "docs/review.md" in html
    assert "finding artifact front matter is required for auto-finalize" in html
