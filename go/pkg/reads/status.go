package reads

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

// HandleStatus mirrors src/striatum/daemon_pg/handlers/reads/status.py.
// Returns a snapshot of run, jobs, sessions, queue messages, leases,
// blockers and verdict counts scoped to the caller's repository_id and
// optional run_id filter.
func HandleStatus(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID != "" {
		exists, err := rowExists(ctx, runner,
			`SELECT r.run_id
			   FROM striatumd.runs r
			  WHERE r.repository_id = $1 AND r.run_id = $2
			  LIMIT 1`,
			repositoryID, runID,
		)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, rpc.NewError("not_found", "run not found: "+runID, nil)
		}
	}

	runs, err := statusRuns(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	jobs, err := statusJobCounts(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	sessions, err := statusSessions(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	openBlockers, err := statusBlockers(ctx, runner, repositoryID, runID, "")
	if err != nil {
		return nil, err
	}
	humanCheckpoints, err := statusBlockers(ctx, runner, repositoryID, runID, "human_checkpoint")
	if err != nil {
		return nil, err
	}
	nonAccepting, err := statusLatestNonAccepting(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	verdictsByPosture, err := statusVerdictsByPosture(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	claimable, err := statusClaimableJobs(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	blockedDownstream, err := statusBlockedDownstreamJobs(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	processHealth, err := statusProcessHealth(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	supervisorStalls, err := statusSupervisorStalls(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	hasOrphanSupervisor, err := statusHasLostSupervisorHeldLease(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	hasStaleLeases, err := statusHasStaleLeasesWithOnDiskArtifacts(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	var autoFinalize any
	var provenanceMode any
	result := map[string]any{
		"runs":                                 runs,
		"jobs":                                 jobs,
		"sessions":                             sessions,
		"verdicts_by_posture":                  verdictsByPosture,
		"latest_non_accepting_review_verdicts": nonAccepting,
		"open_blockers":                        openBlockers,
		"claimable_jobs":                       claimable,
		"blocked_downstream_jobs":              blockedDownstream,
		"human_checkpoints":                    humanCheckpoints,
		"process_health":                       processHealth,
		"supervisor_stalls":                    supervisorStalls,
		"auto_finalize_dry_run":                autoFinalize,
		"next_actions":                         statusNextActions(claimable, openBlockers, humanCheckpoints, nonAccepting, hasOrphanSupervisor, hasStaleLeases, processHealth, supervisorStalls, autoFinalize),
		"provenance_mode":                      provenanceMode,
	}
	if runID != "" {
		workflow, err := statusWorkflowForRun(ctx, runner, repositoryID, runID)
		if err != nil {
			return nil, err
		}
		provenanceMode = nullableStringValue(stringValue(workflow["provenance_mode"]))
		autoFinalize, err = dashboardAllAutoFinalizeSummary(ctx, runner, repositoryID, runID, workflow)
		if err != nil {
			return nil, err
		}
		phase, err := dashboardAllPhaseProgress(ctx, runner, repositoryID, runID, workflow)
		if err != nil {
			return nil, err
		}
		result["provenance_mode"] = provenanceMode
		result["auto_finalize_dry_run"] = autoFinalize
		result["current_phase_id"] = phase["current_phase_id"]
		result["phases"] = phase["phases"]
		result["next_actions"] = statusNextActions(claimable, openBlockers, humanCheckpoints, nonAccepting, hasOrphanSupervisor, hasStaleLeases, processHealth, supervisorStalls, autoFinalize)
	}
	return result, nil
}

func statusRuns(ctx context.Context, runner db.Runner, repositoryID, runID string) ([]map[string]any, error) {
	where := "r.repository_id = $1"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND r.run_id = $2"
		args = append(args, runID)
	}
	return collectRows(ctx, runner,
		`SELECT r.run_id, r.state, r.branch_name
		   FROM striatumd.runs r
		  WHERE `+where+`
		  ORDER BY r.created_at, r.run_id`,
		args...,
	)
}

func statusJobCounts(ctx context.Context, runner db.Runner, repositoryID, runID string) (map[string]int, error) {
	where := "j.repository_id = $1"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND j.run_id = $2"
		args = append(args, runID)
	}
	rows, err := collectRows(ctx, runner,
		`SELECT j.state, COUNT(*) AS count
		   FROM striatumd.jobs j
		  WHERE `+where+`
		  GROUP BY j.state
		  ORDER BY j.state`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, row := range rows {
		out[stringFrom(row, "state")] = intFrom(row, "count")
	}
	return out, nil
}

func statusSessions(ctx context.Context, runner db.Runner, repositoryID, runID string) ([]map[string]any, error) {
	where := "s.repository_id = $1"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND s.run_id = $2"
		args = append(args, runID)
	}
	rows, err := collectRows(ctx, runner,
		`SELECT s.session_id, s.run_id, s.role_id, s.lane_id, s.slug,
		        s.ordinal, s.state, s.operator_label,
		        s.registered_at,
		        s.last_mcp_request_at,
		        s.last_tools_list_at,
		        s.last_await_packet_at,
		        s.last_packet_delivered_at,
		        s.last_ack_at,
		        s.last_work_block_at,
		        s.last_work_release_at,
		        s.last_work_complete_at,
		        s.last_work_heartbeat_at,
		        s.last_session_ready_at,
		        s.last_session_heartbeat_at,
		        s.last_session_question_at,
		        s.last_session_escalate_at,
		        s.liveness_stall_class,
		        s.liveness_stall_since,
		        ps.supervisor_id AS supervisor_id, ps.pid AS pid,
		        ptr.metadata_json AS supervisor_metadata_json,
		        active_lease.lease_id AS active_lease_id,
		        active_lease.acquired_at AS active_lease_acquired_at,
		        active_lease.expires_at AS active_lease_expires_at,
		        active_lease.last_heartbeat_at AS active_lease_last_heartbeat_at
		   FROM striatumd.sessions s
		   LEFT JOIN striatumd.process_supervisors ps
		     ON ps.repository_id = s.repository_id
		    AND ps.session_id = s.session_id
		    AND ps.state = 'attached'
		   LEFT JOIN striatumd.process_supervisor_pointers ptr
		     ON ptr.repository_id = ps.repository_id
		    AND ptr.supervisor_id = ps.supervisor_id
		   LEFT JOIN LATERAL (
		     SELECT l.lease_id, l.acquired_at, l.expires_at, l.last_heartbeat_at
		       FROM striatumd.leases l
		      WHERE l.repository_id = s.repository_id
		        AND l.owner_session_id = s.session_id
		        AND l.state = 'active'
		      ORDER BY l.acquired_at DESC, l.lease_id DESC
		      LIMIT 1
		   ) active_lease ON true
		  WHERE `+where+`
		  ORDER BY s.registered_at, s.session_id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, row := range rows {
		row["liveness"] = sessionliveness.ProjectionFromRow(row, now)
		sessionliveness.RemoveProjectionSourceFields(row)
		supervisorID := stringFrom(row, "supervisor_id")
		if supervisorID != "" && DrainHelperEventsHook != nil {
			if tx, err := runner.BeginTx(ctx); err == nil {
				_ = DrainHelperEventsHook(ctx, tx, repositoryID, supervisorID)
				_ = tx.Commit(ctx)
				metaRows, err := collectRows(ctx, runner,
					`SELECT metadata_json FROM striatumd.process_supervisor_pointers
					  WHERE repository_id = $1 AND supervisor_id = $2`,
					repositoryID, supervisorID,
				)
				if err == nil && len(metaRows) > 0 {
					row["supervisor_metadata_json"] = metaRows[0]["metadata_json"]
				}
			}
		}
		metadata := superviseObject(row["supervisor_metadata_json"])
		attachSupervisorTmux(row, "supervisor_metadata_json")
		if stringFrom(row, "supervisor_id") == "" {
			row["lane_attestation"] = "unattested"
			row["lane_attestation_reason"] = "no_attached_supervisor"
			row["pid"] = nil
			row["lane_backend"] = "none"
			row["delivery_state"] = "unknown"
			continue
		}
		pid, hasPID := intValueOptional(row["pid"])
		live := attachTmuxLivenessFromMetadata(ctx, row, metadata, pid, "")
		if !hasPID && live.Backed != "tmux" {
			row["lane_attestation"] = "unattested"
			row["lane_attestation_reason"] = "pid_gone"
			continue
		}
		if !live.Alive {
			row["lane_attestation"] = "unattested"
			row["lane_attestation_reason"] = live.Class
			continue
		}
		if tmuxStartTokenUnverified(live) {
			row["lane_attestation"] = "unattested"
			row["lane_attestation_reason"] = "start_token_unverified"
			continue
		}
		row["lane_attestation"] = "attested"
		row["lane_attestation_reason"] = nil
	}
	return rows, nil
}

func statusBlockers(ctx context.Context, runner db.Runner, repositoryID, runID, severity string) ([]map[string]any, error) {
	where := "b.repository_id = $1 AND b.state = 'open'"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND b.run_id = $" + intPlaceholder(len(args)+1)
		args = append(args, runID)
	}
	if severity != "" {
		where += " AND b.severity = $" + intPlaceholder(len(args)+1)
		args = append(args, severity)
	}
	return collectRows(ctx, runner,
		`SELECT b.blocker_id, b.run_id, b.job_id, b.session_id,
		        b.severity, b.blocker_kind, b.description, b.state,
		        b.created_at, b.payload_json, j.workflow_job_id,
		        j.state AS job_state
		   FROM striatumd.blockers b
		   LEFT JOIN striatumd.jobs j
		     ON j.repository_id = b.repository_id
		    AND j.job_id = b.job_id
		  WHERE `+where+`
		  ORDER BY b.created_at, b.blocker_id`,
		args...,
	)
}

func statusLatestNonAccepting(ctx context.Context, runner db.Runner, repositoryID, runID string) ([]map[string]any, error) {
	where := "v.repository_id = $1 AND v.verdict NOT IN ('accept', 'accept_with_findings')"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND v.run_id = $2"
		args = append(args, runID)
	}
	return collectRows(ctx, runner,
		`SELECT DISTINCT ON (v.job_id)
		        v.verdict_id, v.run_id, v.job_id, j.workflow_job_id,
		        v.verdict, v.posture, v.created_at
		   FROM striatumd.verdicts v
		   JOIN striatumd.jobs j
		     ON j.repository_id = v.repository_id
		    AND j.job_id = v.job_id
		  WHERE `+where+`
		  ORDER BY v.job_id, v.created_at DESC, v.verdict_id DESC`,
		args...,
	)
}

func statusVerdictsByPosture(ctx context.Context, runner db.Runner, repositoryID, runID string) (map[string]map[string]int, error) {
	where := "v.repository_id = $1"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND v.run_id = $2"
		args = append(args, runID)
	}
	rows, err := collectRows(ctx, runner,
		`SELECT v.posture, v.verdict, COUNT(*) AS count
		   FROM striatumd.verdicts v
		  WHERE `+where+`
		  GROUP BY v.posture, v.verdict
		  ORDER BY v.posture, v.verdict`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]int{}
	for _, row := range rows {
		posture := stringFrom(row, "posture")
		verdict := stringFrom(row, "verdict")
		if _, ok := out[posture]; !ok {
			out[posture] = map[string]int{}
		}
		out[posture][verdict] = intFrom(row, "count")
	}
	return out, nil
}

func statusClaimableJobs(ctx context.Context, runner db.Runner, repositoryID, runID string) ([]map[string]any, error) {
	where := "q.repository_id = $1"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND q.run_id = $2"
		args = append(args, runID)
	}
	return collectRows(ctx, runner,
		`SELECT q.run_id, q.job_id, j.workflow_job_id,
		        q.target_role_id AS role_id, q.target_lane_id AS lane_id,
		        COUNT(*) AS count
		   FROM striatumd.queue_messages q
		   JOIN striatumd.jobs j
		     ON j.repository_id = q.repository_id
		    AND j.job_id = q.job_id
		  WHERE `+where+`
		    AND q.state = 'pending'
		    AND (q.visible_after IS NULL OR q.visible_after <= now())
		  GROUP BY q.run_id, q.job_id, j.workflow_job_id,
		           q.target_role_id, q.target_lane_id
		  ORDER BY q.target_role_id, q.target_lane_id, j.workflow_job_id`,
		args...,
	)
}

func statusBlockedDownstreamJobs(ctx context.Context, runner db.Runner, repositoryID, runID string) ([]map[string]any, error) {
	where := "j.repository_id = $1 AND j.state = 'blocked'"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND j.run_id = $2"
		args = append(args, runID)
	}
	return collectRows(ctx, runner,
		`SELECT j.run_id, j.job_id, j.workflow_job_id, j.state
		   FROM striatumd.jobs j
		  WHERE `+where+`
		  ORDER BY j.created_at, j.workflow_job_id`,
		args...,
	)
}

func statusProcessHealth(ctx context.Context, runner db.Runner, repositoryID, runID string) (map[string]any, error) {
	if runID == "" {
		return map[string]any{
			"running_count":       0,
			"stale_running_count": 0,
			"lost_count":          0,
			"timed_out_count":     0,
			"next_actions":        []string{},
		}, nil
	}
	where := "p.repository_id = $1"
	args := []any{repositoryID}
	where += " AND p.run_id = $2"
	args = append(args, runID)
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
		  WHERE `+where,
		args...,
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

func statusSupervisorStalls(ctx context.Context, runner db.Runner, repositoryID, runID string) (map[string]any, error) {
	return dashboardAllSupervisorStalls(ctx, runner, repositoryID, runID)
}

func statusWorkflowForRun(ctx context.Context, runner db.Runner, repositoryID, runID string) (map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT w.workflow_json
		   FROM striatumd.runs r
		   JOIN striatumd.workflow_snapshots w
		     ON w.repository_id = r.repository_id
		    AND w.workflow_snapshot_id = r.workflow_snapshot_id
		  WHERE r.repository_id = $1 AND r.run_id = $2`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[string]any{}, nil
	}
	return objectFromJSONish(rows[0]["workflow_json"]), nil
}

func statusHasLostSupervisorHeldLease(ctx context.Context, runner db.Runner, repositoryID, runID string) (bool, error) {
	where := "s.repository_id = $1 AND s.state = 'lost'"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND s.run_id = $2"
		args = append(args, runID)
	}
	return rowExists(ctx, runner,
		`SELECT 1
		   FROM striatumd.process_supervisors s
		   JOIN striatumd.leases l
		     ON l.repository_id = s.repository_id
		    AND l.owner_session_id = s.session_id
		    AND l.state = 'active'
		    AND l.expires_at > now()
		  WHERE `+where+`
		  LIMIT 1`,
		args...,
	)
}

func statusHasStaleLeasesWithOnDiskArtifacts(ctx context.Context, runner db.Runner, repositoryID, runID string) (bool, error) {
	where := "j.repository_id = $1 AND l.state = 'expired' AND j.state IN ('claimed', 'running', 'stale_lease')"
	args := []any{repositoryID}
	if runID != "" {
		where += " AND j.run_id = $2"
		args = append(args, runID)
	}
	rows, err := collectRows(ctx, runner,
		`SELECT r.repo_root, j.expected_artifacts_json
		   FROM striatumd.jobs j
		   JOIN striatumd.runs r
		     ON r.repository_id = j.repository_id
		    AND r.run_id = j.run_id
		   JOIN striatumd.leases l
		     ON l.repository_id = j.repository_id
		    AND l.lease_id = j.current_lease_id
		  WHERE `+where,
		args...,
	)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		repoRoot := stringFrom(row, "repo_root")
		for _, artifact := range statusExpectedArtifacts(row["expected_artifacts_json"]) {
			required, ok := artifact["required"].(bool)
			if ok && !required {
				continue
			}
			path := stringValue(artifact["path"])
			if path == "" {
				continue
			}
			fullPath := path
			if !filepath.IsAbs(fullPath) {
				fullPath = filepath.Join(repoRoot, path)
			}
			if _, err := os.Stat(fullPath); err == nil {
				return true, nil
			}
		}
	}
	return false, nil
}

func statusExpectedArtifacts(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := []map[string]any{}
		for _, item := range typed {
			if artifact := objectFromJSONish(item); len(artifact) > 0 {
				out = append(out, artifact)
			}
		}
		return out
	case []byte:
		var decoded []map[string]any
		_ = json.Unmarshal(typed, &decoded)
		return decoded
	case string:
		var decoded []map[string]any
		_ = json.Unmarshal([]byte(typed), &decoded)
		return decoded
	default:
		return []map[string]any{}
	}
}

func statusNextActions(
	claimable []map[string]any,
	openBlockers []map[string]any,
	humanCheckpoints []map[string]any,
	nonAcceptingVerdicts []map[string]any,
	hasOrphanSupervisor bool,
	hasStaleLeases bool,
	processHealth map[string]any,
	supervisorStalls map[string]any,
	autoFinalize any,
) []string {
	out := []string{}
	if len(claimable) > 0 {
		out = append(out, "claim_available_work", "inspect_packet_with_inbox")
	}
	if hasOrphanSupervisor {
		out = append(out, "recover_orphan_supervisor")
	}
	if hasStaleLeases {
		out = append(out, "recovery_auto_publish")
	}
	if len(openBlockers) > 0 {
		out = append(out, "inspect_blocker", "export_run_evidence")
	}
	if len(humanCheckpoints) > 0 {
		out = append(out, "resolve_human_checkpoint", "derive_expected_byline")
	}
	if len(nonAcceptingVerdicts) > 0 {
		out = append(out, "revise_workflow_cycle", "derive_expected_byline")
	}
	if summary, ok := autoFinalize.(map[string]any); ok && statusAutoFinalizeLiveAllowed(summary) && (intFrom(summary, "eligible_count") > 0 || intFrom(summary, "candidate_count") > 0) {
		out = appendUnique(out, "recovery_auto_finalize")
	}
	for _, action := range statusStringList(processHealth["next_actions"]) {
		if action != "" {
			out = appendUnique(out, action)
		}
	}
	for _, action := range statusStringList(supervisorStalls["next_actions"]) {
		if action != "" {
			out = appendUnique(out, action)
		}
	}
	return uniqueStrings(out)
}

func statusStringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := []string{}
		for _, item := range typed {
			if text := stringValue(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return []string{}
	}
}

func appendUnique(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func uniqueStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		out = appendUnique(out, item)
	}
	return out
}

func statusAutoFinalizeLiveAllowed(summary map[string]any) bool {
	policy, ok := summary["policy"].(map[string]any)
	return ok && policy["live_allowed"] == true
}

func intPlaceholder(value int) string {
	return strconv.Itoa(value)
}

func stringFrom(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
