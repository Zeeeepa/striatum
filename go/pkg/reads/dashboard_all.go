package reads

import (
	"context"
	"encoding/json"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const protocolVersion = 1

// HandleDashboardAll returns the daemon-global dashboard projection over the
// daemon-owned PostgreSQL registry and per-repository workflow tables.
//
// This is intentionally SELECT-only. Unlike the legacy Python/SQLite path, it
// does not lazily expire leases while rendering a dashboard frame.
func HandleDashboardAll(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	if runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "dashboard.all requires daemon PostgreSQL", nil)
	}
	repos, err := collectRows(ctx, runner,
		`SELECT repository_id, display_name, repo_root, state
		   FROM striatumd.repositories
		  WHERE state != 'removed'
		  ORDER BY repository_id`)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(repos))
	for _, repo := range repos {
		repositoryID := stringFrom(repo, "repository_id")
		entry := map[string]any{
			"repository_id": repositoryID,
			"display_name":  repo["display_name"],
			"repo_root":     repo["repo_root"],
			"state":         repo["state"],
		}
		status, err := dashboardAllStatus(ctx, runner, repositoryID)
		if err != nil {
			entry["state"] = "degraded"
			entry["error"] = err.Error()
			result = append(result, entry)
			continue
		}
		staleLeases, err := dashboardAllStaleLeases(ctx, runner, repositoryID)
		if err != nil {
			entry["state"] = "degraded"
			entry["error"] = err.Error()
			result = append(result, entry)
			continue
		}
		entry["status"] = status
		entry["stale_leases"] = staleLeases
		result = append(result, entry)
	}
	return map[string]any{
		"mode":             "daemon",
		"protocol_version": protocolVersion,
		"repositories":     result,
	}, nil
}

func dashboardAllStatus(ctx context.Context, runner db.Runner, repositoryID string) (map[string]any, error) {
	runs, err := collectRows(ctx, runner,
		`SELECT r.run_id, r.state, r.branch_name
		   FROM striatumd.runs r
		  WHERE r.repository_id = $1
		  ORDER BY r.created_at, r.run_id`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	jobRows, err := collectRows(ctx, runner,
		`SELECT j.state, COUNT(*) AS count
		   FROM striatumd.jobs j
		  WHERE j.repository_id = $1
		  GROUP BY j.state
		  ORDER BY j.state`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	jobs := map[string]int{}
	for _, row := range jobRows {
		jobs[stringFrom(row, "state")] = intFrom(row, "count")
	}
	openBlockers, err := dashboardAllBlockers(ctx, runner, repositoryID, "")
	if err != nil {
		return nil, err
	}
	humanCheckpoints, err := dashboardAllBlockers(ctx, runner, repositoryID, "human_checkpoint")
	if err != nil {
		return nil, err
	}
	nonAccepting, err := collectRows(ctx, runner,
		`SELECT DISTINCT ON (v.job_id)
		        v.verdict_id, v.run_id, v.job_id, j.workflow_job_id,
		        v.verdict, v.posture, v.created_at
		   FROM striatumd.verdicts v
		   JOIN striatumd.jobs j
		     ON j.repository_id = v.repository_id
		    AND j.job_id = v.job_id
		  WHERE v.repository_id = $1 AND v.verdict != 'accept'
		  ORDER BY v.job_id, v.created_at DESC, v.verdict_id DESC`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	verdictPostures, err := collectRows(ctx, runner,
		`SELECT v.posture, v.verdict, COUNT(*) AS count
		   FROM striatumd.verdicts v
		  WHERE v.repository_id = $1
		  GROUP BY v.posture, v.verdict
		  ORDER BY v.posture, v.verdict`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	verdictsByPosture := map[string]map[string]int{}
	for _, row := range verdictPostures {
		posture := stringFrom(row, "posture")
		verdict := stringFrom(row, "verdict")
		if _, ok := verdictsByPosture[posture]; !ok {
			verdictsByPosture[posture] = map[string]int{}
		}
		verdictsByPosture[posture][verdict] = intFrom(row, "count")
	}
	claimable, err := collectRows(ctx, runner,
		`SELECT q.run_id, q.job_id, j.workflow_job_id,
		        q.target_role_id AS role_id, q.target_lane_id AS lane_id,
		        COUNT(*) AS count
		   FROM striatumd.queue_messages q
		   JOIN striatumd.jobs j
		     ON j.repository_id = q.repository_id
		    AND j.job_id = q.job_id
		  WHERE q.repository_id = $1
		    AND q.state = 'pending'
		    AND (q.visible_after IS NULL OR q.visible_after <= now())
		  GROUP BY q.run_id, q.job_id, j.workflow_job_id,
		           q.target_role_id, q.target_lane_id
		  ORDER BY q.target_role_id, q.target_lane_id, j.workflow_job_id`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	blockedDownstream, err := collectRows(ctx, runner,
		`SELECT j.run_id, j.job_id, j.workflow_job_id, j.state
		   FROM striatumd.jobs j
		  WHERE j.repository_id = $1 AND j.state = 'blocked'
		  ORDER BY j.created_at, j.workflow_job_id`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	sessions, err := collectRows(ctx, runner,
		`SELECT s.session_id, s.run_id, s.role_id, s.lane_id, s.slug,
		        s.ordinal, s.state, s.operator_label,
		        ps.supervisor_id AS supervisor_id
		   FROM striatumd.sessions s
		   LEFT JOIN striatumd.process_supervisors ps
		     ON ps.repository_id = s.repository_id
		    AND ps.session_id = s.session_id
		    AND ps.state = 'attached'
		  WHERE s.repository_id = $1
		  ORDER BY s.registered_at, s.session_id`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	processHealth, err := dashboardAllProcessHealth(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"runs":                                 runs,
		"provenance_mode":                      nil,
		"sessions":                             sessions,
		"jobs":                                 jobs,
		"open_blockers":                        openBlockers,
		"human_checkpoints":                    humanCheckpoints,
		"latest_non_accepting_review_verdicts": nonAccepting,
		"verdicts_by_posture":                  verdictsByPosture,
		"claimable_jobs":                       claimable,
		"blocked_downstream_jobs":              blockedDownstream,
		"process_health":                       processHealth,
		"next_actions":                         computeNextActions(claimable, openBlockers),
	}, nil
}

func dashboardAllBlockers(ctx context.Context, runner db.Runner, repositoryID string, severity string) ([]map[string]any, error) {
	args := []any{repositoryID}
	where := "b.repository_id = $1 AND b.state = 'open'"
	if severity != "" {
		where += " AND b.severity = $2"
		args = append(args, severity)
	}
	return collectRows(ctx, runner,
		`SELECT b.blocker_id, b.run_id, b.job_id, b.session_id, b.severity,
		        b.blocker_kind, b.description, b.state,
		        j.workflow_job_id, j.state AS job_state
		   FROM striatumd.blockers b
		   LEFT JOIN striatumd.jobs j
		     ON j.repository_id = b.repository_id
		    AND j.job_id = b.job_id
		  WHERE `+where+`
		  ORDER BY b.created_at, b.blocker_id`,
		args...,
	)
}

func dashboardAllProcessHealth(ctx context.Context, runner db.Runner, repositoryID string) (map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT
		    COUNT(*) FILTER (WHERE p.state = 'running') AS running_count,
		    COUNT(*) FILTER (
		      WHERE p.state = 'running' AND l.state = 'expired'
		    ) AS stale_running_count,
		    COUNT(*) FILTER (WHERE p.state = 'lost') AS lost_count,
		    COUNT(*) FILTER (WHERE p.state = 'timed_out') AS timed_out_count
		   FROM striatumd.process_executions p
		   LEFT JOIN striatumd.leases l
		     ON l.repository_id = p.repository_id
		    AND l.lease_id = p.lease_id
		  WHERE p.repository_id = $1`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	row := map[string]any{}
	if len(rows) > 0 {
		row = rows[0]
	}
	nextActions := []string{}
	if intFrom(row, "stale_running_count") > 0 {
		nextActions = append(nextActions, "recovery_process_reconcile")
	}
	return map[string]any{
		"running_count":       intFrom(row, "running_count"),
		"stale_running_count": intFrom(row, "stale_running_count"),
		"lost_count":          intFrom(row, "lost_count"),
		"timed_out_count":     intFrom(row, "timed_out_count"),
		"next_actions":        nextActions,
	}, nil
}

func dashboardAllStaleLeases(ctx context.Context, runner db.Runner, repositoryID string) ([]map[string]any, error) {
	runRows, err := collectRows(ctx, runner,
		`SELECT run_id
		   FROM striatumd.runs
		  WHERE repository_id = $1 AND state IN ('running', 'blocked')
		  ORDER BY created_at, run_id`,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(runRows))
	for _, run := range runRows {
		payload, err := dashboardAllStaleLeasesForRun(ctx, runner, repositoryID, stringFrom(run, "run_id"))
		if err != nil {
			return nil, err
		}
		out = append(out, payload)
	}
	return out, nil
}

func dashboardAllStaleLeasesForRun(ctx context.Context, runner db.Runner, repositoryID string, runID string) (map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT j.job_id, j.workflow_job_id, j.state AS job_state,
		        j.write_scope_json,
		        l.lease_id, l.owner_session_id, l.acquired_at,
		        l.expires_at, l.released_at, l.release_reason,
		        qm.message_id, qm.state AS message_state
		   FROM striatumd.jobs j
		   LEFT JOIN striatumd.leases l
		     ON l.repository_id = j.repository_id
		    AND (l.lease_id = j.current_lease_id
		         OR (l.resource_id = j.job_id AND l.state = 'expired'))
		   LEFT JOIN striatumd.queue_messages qm
		     ON qm.repository_id = j.repository_id
		    AND qm.message_id = j.current_message_id
		  WHERE j.repository_id = $1
		    AND j.run_id = $2
		    AND (j.state = 'stale_lease' OR l.state = 'expired')
		  ORDER BY j.workflow_job_id, l.expires_at`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	entries := []map[string]any{}
	seen := map[string]bool{}
	for _, row := range rows {
		key := stringFrom(row, "job_id") + "/" + stringFrom(row, "lease_id")
		if seen[key] {
			continue
		}
		seen[key] = true
		repoWrite := isRepoWriteScope(row["write_scope_json"])
		policy := "safe_to_reclaim_when_pending"
		actions := []string{"register_or_select_session", "claim_available_work"}
		if repoWrite {
			policy = "manual_inspection_required"
			actions = []string{"inspect_worktree_and_artifacts", "decide_requeue_or_cancel"}
		}
		entries = append(entries, map[string]any{
			"job_id":           row["job_id"],
			"workflow_job_id":  row["workflow_job_id"],
			"job_state":        row["job_state"],
			"lease_id":         row["lease_id"],
			"owner_session_id": row["owner_session_id"],
			"expires_at":       row["expires_at"],
			"released_at":      row["released_at"],
			"release_reason":   row["release_reason"],
			"message_id":       row["message_id"],
			"message_state":    row["message_state"],
			"repo_write":       repoWrite,
			"recovery_policy":  policy,
			"next_actions":     actions,
		})
	}
	nextActions := []string{}
	if len(entries) > 0 {
		nextActions = []string{"inspect_worktree_and_artifacts", "decide_requeue_or_cancel"}
	}
	return map[string]any{
		"run_id":       runID,
		"stale_count":  len(entries),
		"stale_leases": entries,
		"next_actions": nextActions,
	}, nil
}

func isRepoWriteScope(value any) bool {
	scope := map[string]any{}
	switch v := value.(type) {
	case map[string]any:
		scope = v
	case []byte:
		_ = json.Unmarshal(v, &scope)
	case string:
		_ = json.Unmarshal([]byte(v), &scope)
	default:
		return false
	}
	if mode, _ := scope["mode"].(string); mode == "repo_write" {
		return true
	}
	flag, _ := scope["repo_write"].(bool)
	return flag
}
