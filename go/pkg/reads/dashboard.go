package reads

import (
	"context"
	"strconv"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
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
		`SELECT session_id, role_id, lane_id, state, lane_attestation
		   FROM striatumd.sessions
		  WHERE repository_id = $1 AND run_id = $2
		  ORDER BY registered_at DESC`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
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
