package reads

import (
	"context"
	"strconv"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func limitClause(envelope rpc.Envelope, defaultLimit int) (string, int) {
	limit := defaultLimit
	switch v := envelope.Params["limit"].(type) {
	case float64:
		limit = int(v)
	case int:
		limit = v
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}
	return " LIMIT " + strconv.Itoa(limit), limit
}

// HandleListRuns mirrors reads/list_runs.py.
func HandleListRuns(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	state := stringParam(envelope, "state")
	args := []any{repositoryID}
	where := "WHERE repository_id = $1"
	if state != "" {
		where += " AND state = $2"
		args = append(args, state)
	}
	limit, count := limitClause(envelope, 200)
	items, err := collectRows(ctx, runner,
		`SELECT run_id, workflow_snapshot_id, repo_root, state, branch_name,
		        created_at, started_at, completed_at, stop_reason
		   FROM striatumd.runs `+where+
			` ORDER BY created_at DESC`+limit,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": len(items), "limit": count, "items": items}, nil
}

// HandleListSessions mirrors reads/list_sessions.py.
func HandleListSessions(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	role := stringParam(envelope, "role")
	lane := stringParam(envelope, "lane")
	args := []any{repositoryID}
	where := "WHERE repository_id = $1"
	if runID != "" {
		args = append(args, runID)
		where += " AND run_id = $" + strconv.Itoa(len(args))
	}
	if role != "" {
		args = append(args, role)
		where += " AND role_id = $" + strconv.Itoa(len(args))
	}
	if lane != "" {
		args = append(args, lane)
		where += " AND lane_id = $" + strconv.Itoa(len(args))
	}
	limit, count := limitClause(envelope, 200)
	items, err := collectRows(ctx, runner,
		`SELECT session_id, run_id, role_id, lane_id, state, slug,
		        capabilities_json, lane_attestation, lane_attestation_reason,
		        registered_at, closed_at, closed_reason
		   FROM striatumd.sessions `+where+
			` ORDER BY registered_at DESC`+limit,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": len(items), "limit": count, "items": items}, nil
}

// HandleListJobs mirrors reads/list_jobs.py.
func HandleListJobs(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	state := stringParam(envelope, "state")
	workflowJobID := stringParam(envelope, "workflow_job_id")
	args := []any{repositoryID}
	where := "WHERE repository_id = $1"
	if runID != "" {
		args = append(args, runID)
		where += " AND run_id = $" + strconv.Itoa(len(args))
	}
	if state != "" {
		args = append(args, state)
		where += " AND state = $" + strconv.Itoa(len(args))
	}
	if workflowJobID != "" {
		args = append(args, workflowJobID)
		where += " AND workflow_job_id = $" + strconv.Itoa(len(args))
	}
	limit, count := limitClause(envelope, 500)
	items, err := collectRows(ctx, runner,
		`SELECT job_id, run_id, workflow_job_id, title, job_type, role_id,
		        state, attempt, max_attempts, current_message_id,
		        current_lease_id, ready_at, started_at, completed_at
		   FROM striatumd.jobs `+where+
			` ORDER BY created_at DESC`+limit,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": len(items), "limit": count, "items": items}, nil
}

// HandleListArtifacts mirrors reads/list_artifacts.py.
func HandleListArtifacts(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	kind := stringParam(envelope, "kind")
	args := []any{repositoryID}
	where := "WHERE repository_id = $1"
	if runID != "" {
		args = append(args, runID)
		where += " AND run_id = $" + strconv.Itoa(len(args))
	}
	if kind != "" {
		args = append(args, kind)
		where += " AND kind = $" + strconv.Itoa(len(args))
	}
	limit, count := limitClause(envelope, 500)
	items, err := collectRows(ctx, runner,
		`SELECT artifact_id, run_id, job_id, session_id, kind, logical_name,
		        path, content_sha256, byline, published_at
		   FROM striatumd.artifacts `+where+
			` ORDER BY published_at DESC`+limit,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": len(items), "limit": count, "items": items}, nil
}

// HandleListWorkflows mirrors reads/list_workflows.py.
func HandleListWorkflows(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	limit, count := limitClause(envelope, 200)
	items, err := collectRows(ctx, runner,
		`SELECT workflow_snapshot_id, workflow_id, workflow_version,
		        snapshot_sha256, captured_at
		   FROM striatumd.workflow_snapshots
		  WHERE repository_id = $1
		  ORDER BY captured_at DESC`+limit,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": len(items), "limit": count, "items": items}, nil
}
