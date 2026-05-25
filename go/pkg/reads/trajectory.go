package reads

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// HandleTrajectoryExport projects ordered trajectory records for a run.
func HandleTrajectoryExport(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "trajectory.export requires run_id", nil)
	}
	profile := stringParam(envelope, "profile")
	if profile == "" {
		profile = "dialogue"
	}
	if profile != "dialogue" && profile != "provenance" {
		return nil, rpc.NewError("schema_invalid", "invalid profile: "+profile, nil)
	}

	records, err := fetchTrajectory(ctx, runner, repositoryID, runID, profile, 0)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"repository_id": repositoryID,
		"run_id":        runID,
		"profile":       profile,
		"records":       records,
	}, nil
}

// HandleTrajectoryWatch tails trajectory records for a run since a sequence.
func HandleTrajectoryWatch(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "trajectory.watch requires run_id", nil)
	}
	profile := stringParam(envelope, "profile")
	if profile == "" {
		profile = "dialogue"
	}
	sinceSeq := int64(intFrom(envelope.Params, "since_seq"))

	records, err := fetchTrajectory(ctx, runner, repositoryID, runID, profile, sinceSeq)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"repository_id": repositoryID,
		"run_id":        runID,
		"profile":       profile,
		"since_seq":     sinceSeq,
		"records":       records,
	}, nil
}

func fetchTrajectory(ctx context.Context, runner db.Runner, repositoryID, runID, profile string, sinceSeq int64) ([]map[string]any, error) {
	// We build a UNION query to pull from messages, events, artifacts, verdicts, and blockers.
	// Each branch is filtered by profile.

	var queries []string
	var args []any
	args = append(args, repositoryID, runID, sinceSeq)

	// 1. Messages (Dialogue + Provenance)
	// For dialogue, we only want agent_message and coordinator_message.
	// For provenance, we also want work (claim/ack/complete) and human_checkpoint.
	messageFilter := "'agent_message', 'coordinator_message'"
	if profile == "provenance" {
		messageFilter = "'agent_message', 'coordinator_message', 'work', 'human_checkpoint', 'commit_request'"
	}

	queries = append(queries, fmt.Sprintf(`
		SELECT run_event_seq AS seq, created_at AS ts, kind,
		       target_session_id AS session_id, target_role_id AS role_id, target_lane_id AS lane_id,
		       payload_json->>'parent_message_id' AS parent_message_id,
		       payload_json AS body,
		       NULL::jsonb AS references
		  FROM striatumd.queue_messages
		 WHERE repository_id = $1 AND run_id = $2 AND run_event_seq > $3
		   AND kind IN (%s)`, messageFilter))

	// 2. Events (Provenance only)
	if profile == "provenance" {
		queries = append(queries, `
			SELECT run_event_seq AS seq, created_at AS ts, event_type AS kind,
			       actor_session_id AS session_id, NULL AS role_id, NULL AS lane_id,
			       NULL AS parent_message_id,
			       payload_json AS body,
			       NULL::jsonb AS references
			  FROM striatumd.events
			 WHERE repository_id = $1 AND run_id = $2 AND run_event_seq > $3`)
	}

	// 3. Artifacts (Dialogue + Provenance)
	// We project these as 'artifact_published' kind.
	queries = append(queries, `
		SELECT run_event_seq AS seq, created_at AS ts, 'artifact_published' AS kind,
		       session_id, NULL AS role_id, NULL AS lane_id,
		       NULL AS parent_message_id,
		       jsonb_build_object('artifact_id', artifact_id, 'logical_name', logical_name, 'kind', artifact_kind, 'path', repo_path) AS body,
		       NULL::jsonb AS references
		  FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND run_event_seq > $3`)

	// 4. Verdicts (Provenance only)
	if profile == "provenance" {
		queries = append(queries, `
			SELECT run_event_seq AS seq, created_at AS ts, 'verdict' AS kind,
			       session_id, NULL AS role_id, NULL AS lane_id,
			       NULL AS parent_message_id,
			       jsonb_build_object('verdict_id', verdict_id, 'verdict', verdict, 'rationale', rationale, 'posture', posture) AS body,
			       NULL::jsonb AS references
			  FROM striatumd.verdicts
			 WHERE repository_id = $1 AND run_id = $2 AND run_event_seq > $3`)
	}

	// 5. Blockers (Provenance only)
	if profile == "provenance" {
		queries = append(queries, `
			SELECT run_event_seq AS seq, created_at AS ts, 'blocker' AS kind,
			       session_id, NULL AS role_id, NULL AS lane_id,
			       NULL AS parent_message_id,
			       jsonb_build_object('blocker_id', blocker_id, 'severity', severity, 'kind', blocker_kind, 'description', description, 'state', state) AS body,
			       NULL::jsonb AS references
			  FROM striatumd.blockers
			 WHERE repository_id = $1 AND run_id = $2 AND run_event_seq > $3`)
	}

	fullQuery := ""
	for i, q := range queries {
		if i > 0 {
			fullQuery += " UNION ALL "
		}
		fullQuery += q
	}
	fullQuery += " ORDER BY seq ASC"

	rows, err := collectRows(ctx, runner, fullQuery, args...)
	if err != nil {
		return nil, err
	}

	// Curate bodies to ensure D028 compliance.
	for _, row := range rows {
		kind := row["kind"].(string)
		body, _ := row["body"].(map[string]any)
		if kind == "agent_message" || kind == "coordinator_message" {
			// Curate chat body.
			curated := map[string]any{}
			if text, ok := body["text"].(string); ok {
				curated["text"] = text
			}
			if topic, ok := body["topic"].(string); ok {
				curated["topic"] = topic
			}
			row["body"] = curated
		}
	}

	return rows, nil
}
