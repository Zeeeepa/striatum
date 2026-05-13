from __future__ import annotations

import sqlite3
from pathlib import Path

from striatum.migrations import apply_migrations


HERE = Path(__file__).resolve().parent
FIXTURE = HERE / "state.sqlite3"
TS = "2026-05-13T00:00:00Z"


def main() -> None:
    if FIXTURE.exists():
        FIXTURE.unlink()
    conn = sqlite3.connect(FIXTURE)
    conn.row_factory = sqlite3.Row
    apply_migrations(conn)
    conn.executescript(
        f"""
        INSERT INTO workflow_snapshots(workflow_snapshot_id, workflow_id,
          workflow_version, source_path, content_sha256, workflow_json, loaded_at)
        VALUES ('ws_1', 'wf_fixture', '1', 'workflow.json', 'sha-workflow',
          '{{"schema_version":"striatum.workflow.v1","jobs":[]}}', '{TS}');

        INSERT INTO runs(run_id, workflow_snapshot_id, repo_root, state,
          branch_name, branch_base, branch_confirmed_at, branch_confirmed_by,
          created_at, started_at, completed_at, stop_reason, paused_at,
          paused_reason, cross_repo_run_id)
        VALUES ('run_1', 'ws_1', '/tmp/fixture-repo', 'running', 'main',
          'base', '{TS}', 'operator', '{TS}', '{TS}', NULL, NULL, NULL,
          NULL, 'cross_1');

        INSERT INTO sessions(session_id, run_id, role_id, lane_id, slug,
          ordinal, capabilities_json, parent_session_id, first_class,
          fresh_context, state, registered_at, last_heartbeat_at, expires_at,
          non_fresh_reason, closed_at, close_reason, operator_label)
        VALUES ('sess_1', 'run_1', 'implementer', 'codex', 'implementer-codex-1',
          1, '["write"]', NULL, 1, 0, 'active', '{TS}', '{TS}', NULL,
          NULL, NULL, NULL, NULL);

        INSERT INTO jobs(job_id, run_id, workflow_job_id, title, job_type,
          role_id, lane_selector_json, capability_requirements_json, state,
          attempt, max_attempts, fresh_session_required, write_scope_json,
          expected_artifacts_json, idempotency_key, created_at, ready_at,
          started_at, completed_at, current_message_id, current_lease_id)
        VALUES ('job_1', 'run_1', 'implement', 'Implement', 'build',
          'implementer', '{{"lane_id":"codex"}}', '["write"]', 'running',
          1, 1, 0, '{{"allowed_paths":["src/"]}}',
          '[{{"path":"HANDOFF.md"}}]', 'idem_1', '{TS}', '{TS}', '{TS}',
          NULL, 'msg_1', 'lease_1');

        INSERT INTO job_dependencies(job_id, depends_on_job_id, gate_json)
        VALUES ('job_1', 'job_1', '{{"self_fixture":true}}');

        INSERT INTO queue_messages(message_id, run_id, job_id, kind, state,
          priority, target_session_id, target_role_id, target_lane_id,
          dedupe_key, payload_json, visible_after, claim_count, max_claims,
          created_at, updated_at, claimed_at, acked_at, completed_at,
          current_lease_id)
        VALUES ('msg_1', 'run_1', 'job_1', 'work', 'acked', 10, 'sess_1',
          'implementer', 'codex', 'dedupe_1', '{{"hello":"world"}}',
          '{TS}', 1, 2, '{TS}', '{TS}', '{TS}', '{TS}', NULL, 'lease_1');

        INSERT INTO leases(lease_id, run_id, resource_type, resource_id,
          owner_session_id, state, acquired_at, expires_at, last_heartbeat_at,
          released_at, release_reason)
        VALUES ('lease_1', 'run_1', 'job', 'job_1', 'sess_1', 'active',
          '{TS}', '2026-05-13T01:00:00Z', '{TS}', NULL, NULL);

        INSERT INTO work_packets(packet_id, run_id, job_id, message_id,
          lease_id, session_id, packet_json, packet_sha256, created_at)
        VALUES ('wp_1', 'run_1', 'job_1', 'msg_1', 'lease_1', 'sess_1',
          '{{"packet_id":"wp_1"}}', 'sha-packet', '{TS}');

        INSERT INTO artifacts(artifact_id, run_id, job_id, session_id,
          logical_name, artifact_kind, repo_path, content_sha256, size_bytes,
          publish_mode, created_at, author_line)
        VALUES ('art_1', 'run_1', 'job_1', 'sess_1', 'handoff', 'handoff',
          'HANDOFF.md', 'sha-artifact', 123, 'create', '{TS}',
          'author: implementer-unknown-model-001');

        INSERT INTO verdicts(verdict_id, run_id, job_id, session_id, verdict,
          rationale, findings_artifact_id, created_at, posture)
        VALUES ('verdict_1', 'run_1', 'job_1', 'sess_1', 'accept',
          'fixture', 'art_1', '{TS}', 'neutral');

        INSERT INTO blockers(blocker_id, run_id, job_id, session_id, severity,
          blocker_kind, description, state, created_at, resolved_at,
          payload_json)
        VALUES ('block_1', 'run_1', 'job_1', 'sess_1', 'info', 'fixture',
          'fixture blocker', 'resolved', '{TS}', '{TS}', '{{"ok":true}}');

        INSERT INTO command_requests(request_id, run_id, session_id,
          command_name, payload_sha256, response_json, state, created_at,
          completed_at)
        VALUES ('cmd_1', 'run_1', 'sess_1', 'fixture.command', 'sha-cmd',
          '{{"ok":true}}', 'completed', '{TS}', '{TS}');

        INSERT INTO process_executions(process_id, run_id, job_id, session_id,
          lease_id, packet_id, adapter, command_json, cwd, scratch_path,
          stdin_mode, stdio_mode, pid, state, exit_code, started_at, ended_at)
        VALUES ('proc_1', 'run_1', 'job_1', 'sess_1', 'lease_1', 'wp_1',
          'process', '["true"]', '/tmp/fixture-repo',
          '/tmp/fixture-repo/.striatum/scratch', 'packet', 'suppressed',
          4242, 'exited', 0, '{TS}', '{TS}');

        INSERT INTO events(event_id, run_id, event_type, actor_session_id,
          job_id, message_id, artifact_id, lease_id, payload_json, created_at)
        VALUES (1, 'run_1', 'fixture.created', 'sess_1', 'job_1', 'msg_1',
          'art_1', 'lease_1', '{{"event":1}}', '{TS}');

        INSERT INTO job_worktrees(worktree_id, run_id, job_id, lease_id,
          base_branch, worktree_path, state, created_at, released_at,
          removed_at)
        VALUES ('wt_1', 'run_1', 'job_1', 'lease_1', 'main',
          '/tmp/fixture-repo/.striatum/worktrees/wt_1', 'active', '{TS}',
          NULL, NULL);

        INSERT INTO process_supervisors(supervisor_id, run_id, session_id,
          adapter, command_json, cwd, scratch_path, stdin_pipe_path, pid,
          state, started_at, heartbeat_at, ended_at, stop_reason,
          pid_start_time)
        VALUES ('sup_1', 'run_1', 'sess_1', 'process', '["sleep","1"]',
          '/tmp/fixture-repo', '/tmp/fixture-repo/.striatum/scratch/sup_1',
          '/tmp/fixture-repo/.striatum/scratch/sup_1/stdin.pipe', 4243,
          'attached', '{TS}', '{TS}', NULL, NULL, '12345');

        INSERT INTO process_supervisor_pointers(supervisor_id,
          daemon_supervisor_id, run_id, session_id, pid, pid_start_time,
          state, updated_at, metadata_json)
        VALUES ('sup_1', 'dsup_1', 'run_1', 'sess_1', 4243, '12345',
          'attached', '{TS}', '{{"daemon":true}}');
        """
    )
    conn.commit()
    conn.close()


if __name__ == "__main__":
    main()
