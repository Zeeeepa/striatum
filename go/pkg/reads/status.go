package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
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

	args := []any{repositoryID}
	runWhere := ""
	if runID != "" {
		runWhere = " AND run_id = $2"
		args = append(args, runID)
	}

	runs, err := collectRows(ctx, runner,
		`SELECT run_id, workflow_snapshot_id, repo_root, state, branch_name,
		        branch_confirmed_at, branch_confirmed_by, created_at,
		        started_at, completed_at, stop_reason, paused_at,
		        paused_reason
		   FROM striatumd.runs
		  WHERE repository_id = $1`+runWhere+
			` ORDER BY created_at DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}

	jobs, err := collectRows(ctx, runner,
		`SELECT job_id, run_id, workflow_job_id, title, job_type, role_id,
		        state, attempt, max_attempts, current_message_id,
		        current_lease_id
		   FROM striatumd.jobs
		  WHERE repository_id = $1`+runWhere+
			` ORDER BY created_at DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}

	sessions, err := collectRows(ctx, runner,
		`SELECT session_id, run_id, role_id, lane_id, state, slug,
		        capabilities_json, lane_attestation, lane_attestation_reason,
		        registered_at, closed_at, closed_reason
		   FROM striatumd.sessions
		  WHERE repository_id = $1`+runWhere+
			` ORDER BY registered_at DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}

	verdicts, err := collectRows(ctx, runner,
		`SELECT v.verdict_id, v.run_id, v.job_id, v.session_id,
		        v.verdict, v.rationale, v.review_posture, v.recorded_at,
		        j.workflow_job_id, j.state AS job_state
		   FROM striatumd.verdicts v
		   LEFT JOIN striatumd.jobs j
		     ON j.repository_id = v.repository_id AND j.job_id = v.job_id
		  WHERE v.repository_id = $1`+
			func() string {
				if runID == "" {
					return ""
				}
				return " AND v.run_id = $2"
			}()+
			` ORDER BY v.recorded_at DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}

	blockers, err := collectRows(ctx, runner,
		`SELECT blocker_id, run_id, blocker_kind, description, state,
		        opened_at, resolved_at, resolution
		   FROM striatumd.blockers
		  WHERE repository_id = $1 AND state = 'open'`+
			func() string {
				if runID == "" {
					return ""
				}
				return " AND run_id = $2"
			}()+
			` ORDER BY opened_at DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}

	claimable := computeClaimable(jobs)
	blockedDownstream := computeBlockedDownstream(jobs)

	return map[string]any{
		"runs":                                 runs,
		"jobs":                                 jobs,
		"sessions":                             sessions,
		"verdicts_by_posture":                  verdictsByPosture(verdicts),
		"latest_non_accepting_review_verdicts": latestNonAccepting(verdicts),
		"open_blockers":                        blockers,
		"claimable_jobs":                       claimable,
		"blocked_downstream_jobs":              blockedDownstream,
		"human_checkpoints":                    []any{},
		"process_health": map[string]any{
			"running_count":       0,
			"lost_count":          0,
			"timed_out_count":     0,
			"stale_running_count": 0,
			"next_actions":        []any{},
		},
		"next_actions":    computeNextActions(claimable, blockers),
		"provenance_mode": "advisory",
	}, nil
}

func computeClaimable(jobs []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, j := range jobs {
		if j["state"] == "ready" || j["state"] == "queued" {
			out = append(out, map[string]any{
				"lane_id":          stringFrom(j, "lane_id"),
				"role_id":          stringFrom(j, "role_id"),
				"count":            1,
				"workflow_job_ids": []string{stringFrom(j, "workflow_job_id")},
			})
		}
	}
	return out
}

func computeBlockedDownstream(jobs []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, j := range jobs {
		if j["state"] == "blocked" {
			out = append(out, map[string]any{
				"job_id":          stringFrom(j, "job_id"),
				"workflow_job_id": stringFrom(j, "workflow_job_id"),
				"role_id":         stringFrom(j, "role_id"),
				"state":           stringFrom(j, "state"),
			})
		}
	}
	return out
}

func verdictsByPosture(verdicts []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, v := range verdicts {
		posture := stringFrom(v, "review_posture")
		if posture == "" {
			continue
		}
		counts[posture]++
	}
	return counts
}

func latestNonAccepting(verdicts []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, v := range verdicts {
		verdict := stringFrom(v, "verdict")
		if verdict == "needs_revision" || verdict == "reject" {
			out = append(out, v)
		}
	}
	return out
}

func computeNextActions(claimable []map[string]any, blockers []map[string]any) []string {
	out := []string{}
	if len(claimable) > 0 {
		out = append(out, "claim_available_work")
	}
	if len(blockers) > 0 {
		out = append(out, "inspect_blocker", "resolve_human_checkpoint")
	}
	out = append(out, "export_run_evidence")
	return out
}

func stringFrom(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
