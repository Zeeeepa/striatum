package reads

import (
	"context"
	"strconv"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

// HandleDashboard mirrors reads/dashboard.py.
// Returns the same shape as the legacy SQLite dashboard render: job
// state counts, verdict counts, blocker counts, session list, claimable
// summary, and the last 10 events.
func HandleDashboard(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		// Fall back to the most recent run for the repository so the
		// dashboard verb works without explicit --run-id (the Python
		// path does the same).
		latest, err := collectRows(ctx, runner,
			`SELECT run_id FROM striatumd.runs
			  WHERE repository_id = $1
			  ORDER BY created_at DESC LIMIT 1`,
			repositoryID,
		)
		if err != nil {
			return nil, err
		}
		if len(latest) == 0 {
			return map[string]any{"run_id": nil, "jobs_by_state": map[string]int{}, "verdicts_by_state": map[string]int{}, "blockers": map[string]int{}, "sessions": []any{}, "recent_events": []any{}}, nil
		}
		runID = stringFrom(latest[0], "run_id")
	}

	jobsByState := map[string]int{}
	jobStates, err := collectRows(ctx, runner,
		`SELECT state, COUNT(*) AS c
		   FROM striatumd.jobs
		  WHERE repository_id = $1 AND run_id = $2
		  GROUP BY state`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	for _, r := range jobStates {
		state := stringFrom(r, "state")
		count := 0
		if c, ok := r["c"]; ok {
			switch v := c.(type) {
			case int64:
				count = int(v)
			case int:
				count = v
			case float64:
				count = int(v)
			case string:
				count, _ = strconv.Atoi(v)
			}
		}
		jobsByState[state] = count
	}

	verdictCounts := map[string]int{}
	vrows, err := collectRows(ctx, runner,
		`SELECT verdict, COUNT(*) AS c
		   FROM striatumd.verdicts
		  WHERE repository_id = $1 AND run_id = $2
		  GROUP BY verdict`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	for _, r := range vrows {
		v := stringFrom(r, "verdict")
		count := 0
		if c, ok := r["c"]; ok {
			switch x := c.(type) {
			case int64:
				count = int(x)
			case int:
				count = x
			case float64:
				count = int(x)
			}
		}
		verdictCounts[v] = count
	}

	blockerCounts := map[string]int{}
	brows, err := collectRows(ctx, runner,
		`SELECT blocker_kind, COUNT(*) AS c
		   FROM striatumd.blockers
		  WHERE repository_id = $1 AND run_id = $2 AND state = 'open'
		  GROUP BY blocker_kind`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	for _, r := range brows {
		k := stringFrom(r, "blocker_kind")
		count := 0
		if c, ok := r["c"]; ok {
			switch x := c.(type) {
			case int64:
				count = int(x)
			case int:
				count = x
			case float64:
				count = int(x)
			}
		}
		blockerCounts[k] = count
	}

	sessions, err := collectRows(ctx, runner,
		`SELECT s.session_id, s.role_id, s.lane_id, s.state,
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
		        active_lease.lease_id AS active_lease_id,
		        active_lease.acquired_at AS active_lease_acquired_at,
		        active_lease.expires_at AS active_lease_expires_at,
		        active_lease.last_heartbeat_at AS active_lease_last_heartbeat_at,
		        CASE WHEN ps.supervisor_id IS NOT NULL THEN 'attested' ELSE 'unattested' END AS lane_attestation,
		        CASE WHEN ps.supervisor_id IS NOT NULL THEN NULL ELSE 'no_attached_supervisor' END AS lane_attestation_reason
		   FROM striatumd.sessions s
		   LEFT JOIN striatumd.process_supervisors ps
		     ON ps.repository_id = s.repository_id
		    AND ps.session_id = s.session_id
		    AND ps.state = 'attached'
		   LEFT JOIN LATERAL (
		     SELECT l.lease_id, l.acquired_at, l.expires_at, l.last_heartbeat_at
		       FROM striatumd.leases l
		      WHERE l.repository_id = s.repository_id
		        AND l.owner_session_id = s.session_id
		        AND l.state = 'active'
		      ORDER BY l.acquired_at DESC, l.lease_id DESC
		      LIMIT 1
		   ) active_lease ON true
		  WHERE s.repository_id = $1 AND s.run_id = $2
		  ORDER BY s.registered_at DESC`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, session := range sessions {
		session["liveness"] = sessionliveness.ProjectionFromRow(session, now)
		sessionliveness.RemoveProjectionSourceFields(session)
	}

	events, err := collectRows(ctx, runner,
		`SELECT event_id, run_id, event_type, job_id, message_id,
		        lease_id, created_at, payload_json
		   FROM striatumd.events
		  WHERE repository_id = $1 AND run_id = $2
		  ORDER BY event_id DESC LIMIT 10`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"run_id":            runID,
		"jobs_by_state":     jobsByState,
		"verdicts_by_state": verdictCounts,
		"blockers":          blockerCounts,
		"sessions":          sessions,
		"recent_events":     events,
	}, nil
}
