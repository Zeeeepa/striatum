package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// HandleInterrogationList lists interrogations for a run (RFC 0082).
func HandleInterrogationList(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "interrogation.list requires run_id", nil)
	}
	items, err := collectRows(ctx, runner, `
		SELECT interrogation_id, run_id, interrogator_session_id, target_session_id,
		       topic, state, turn_count, opened_at, closed_at
		  FROM striatumd.interrogations
		 WHERE repository_id = $1 AND run_id = $2
		 ORDER BY opened_at ASC, interrogation_id ASC`,
		repositoryID, runID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": len(items), "run_id": runID, "items": items}, nil
}

// HandleInterrogationShow returns an interrogation plus its ordered, curated
// Q&A turns. Turn bodies carry only authored text and correlation identifiers
// (D028: never raw provider stdout/stderr).
func HandleInterrogationShow(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	interrogationID := stringParam(envelope, "interrogation_id")
	if interrogationID == "" {
		return nil, rpc.NewError("schema_invalid", "interrogation.show requires interrogation_id", nil)
	}
	rows, err := collectRows(ctx, runner, `
		SELECT interrogation_id, run_id, interrogator_session_id, target_session_id,
		       topic, state, turn_count, opened_at, closed_at
		  FROM striatumd.interrogations
		 WHERE repository_id = $1 AND interrogation_id = $2
		 LIMIT 1`,
		repositoryID, interrogationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, rpc.NewError("not_found", "unknown interrogation_id", nil)
	}
	interrogation := rows[0]
	turnRows, err := collectRows(ctx, runner, `
		SELECT message_id, target_session_id, payload_json, created_at
		  FROM striatumd.queue_messages
		 WHERE repository_id = $1
		   AND kind = 'agent_message'
		   AND payload_json->>'interrogation_id' = $2
		 ORDER BY (payload_json->>'turn_index')::int ASC, created_at ASC, message_id ASC`,
		repositoryID, interrogationID)
	if err != nil {
		return nil, err
	}
	turns := make([]map[string]any, 0, len(turnRows))
	for _, row := range turnRows {
		payload, _ := row["payload_json"].(map[string]any)
		turn := map[string]any{
			"message_id":        row["message_id"],
			"target_session_id": row["target_session_id"],
			"created_at":        row["created_at"],
			"turn":              payload["turn"],
			"turn_index":        payload["turn_index"],
			"kind":              payload["kind"],
			"body":              payload["body"],
		}
		turns = append(turns, turn)
	}
	return map[string]any{
		"interrogation": interrogation,
		"turns":         turns,
		"turn_count":    len(turns),
	}, nil
}
