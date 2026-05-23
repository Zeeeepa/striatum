from __future__ import annotations

from striatum.dashboard import render_frame


def test_dashboard_renders_tmux_attach_and_liveness_without_pane_text() -> None:
    output = render_frame(
        {
            "run": {"run_id": "run_1", "branch_name": "main", "state": "running"},
            "status": {
                "jobs": {"running": 1},
                "sessions": [
                    {
                        "role_id": "author",
                        "lane_id": "codex",
                        "state": "active",
                        "lane_attestation": "attested",
                        "liveness": {
                            "protocol": "stalled",
                            "lease": "no_lease",
                            "stall_class": "agent_mcp_discovery_stall",
                            "deadline_name": "mcp_discovery",
                            "deadline_seconds": 60,
                            "last_session_report_kind": "heartbeat",
                        },
                        "tmux": {
                            "state": "attachable",
                            "session_name": "striatum-run_1-codex-sup_1",
                            "window_id": "@1",
                            "pane_id": "%2",
                            "attach_command": (
                                "tmux attach-session -t striatum-run_1-codex-sup_1"
                            ),
                            "pane_text": "terminal bytes must not render",
                        },
                    },
                    {
                        "role_id": "reviewer",
                        "lane_id": "gemini",
                        "state": "active",
                        "lane_attestation": "unattested",
                        "lane_attestation_reason": "no_attached_supervisor",
                        "liveness": {"protocol": "live", "lease": "no_lease"},
                        "tmux": {
                            "state": "unavailable",
                            "unavailable_reason": "tmux_not_found",
                            "remediation": (
                                "install tmux or unset supervision.require_tmux "
                                "for non-interactive lanes"
                            ),
                        },
                    },
                ],
                "open_blockers": [],
                "human_checkpoints": [],
                "latest_non_accepting_review_verdicts": [],
                "claimable_jobs": [],
                "next_actions": [],
            },
            "events": [],
            "updated_at": "2026-05-23T00:00:00Z",
        },
        terminal_width=160,
    )

    assert "liveness protocol=stalled lease=no_lease" in output
    assert "stall=agent_mcp_discovery_stall deadline=mcp_discovery/60s" in output
    assert "last_report=heartbeat" in output
    assert "tmux attach: tmux attach-session -t striatum-run_1-codex-sup_1" in output
    assert "window=@1 pane=%2" in output
    assert "tmux unavailable: tmux_not_found; install tmux" in output
    assert "terminal bytes must not render" not in output
