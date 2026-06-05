package reads

const pgReadScopeBroadRuntimeSelect = "broad_runtime_select"

// pgReadScopeDoctorBlock reports the RFC 0110 / #164 read-scope posture. It is
// deliberately separate from pg_write_boundary: RFC 0110's shipped guarantees
// close mutation paths, not private reads by a live runtime credential.
func pgReadScopeDoctorBlock() map[string]any {
	return map[string]any{
		"posture":                   pgReadScopeBroadRuntimeSelect,
		"runtime_role_select_scope": "broad",
		"private_read_denial":       false,
		"partial_projection_gates": []map[string]any{
			{
				"surface":         "clients",
				"denied_columns":  []string{"token_hash", "token_salt"},
				"owner_bundle":    5,
				"authority_stamp": "auth_projection_read",
			},
		},
		"bounded_by": []string{
			"L0 runtime credential rotation makes captured DSN strings stale after daemon restart",
			"L2 lane isolation prevents sandboxed lanes from reaching PostgreSQL once adopted",
		},
		"representative_sensitive_surfaces": []string{
			"artifacts",
			"blockers",
			"client_capabilities",
			"client_sessions",
			"clients",
			"command_requests",
			"conversations",
			"cross_repo_run_repositories",
			"cross_repo_runs",
			"daemon_supervisors",
			"escalation_inbox",
			"events",
			"interrogations",
			"job_dependencies",
			"job_recovery_state",
			"job_worktrees",
			"jobs",
			"leases",
			"principal_clients",
			"principals",
			"process_executions",
			"process_supervisor_pointers",
			"process_supervisors",
			"queue_messages",
			"repositories",
			"rpc_request_log",
			"runs",
			"scheduler_cursors",
			"sessions",
			"trajectory_segments",
			"verdicts",
			"work_packets",
			"workflow_accepted_risks",
			"workflow_snapshots",
		},
		"inventory_source":        "go/pkg/db/read_authority_inventory.go",
		"sensitive_surface_count": 34,
		"note": "RFC 0110 does not claim read confidentiality against a leaked live runtime credential; " +
			"RFC 0113 R1 reduces token-secret columns first, but #164 remains open until every sensitive read surface is bounded.",
	}
}
