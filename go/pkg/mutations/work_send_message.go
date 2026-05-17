package mutations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func HandleSendMessage(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	kind := stringParam(envelope, "kind")
	if sessionID == "" || kind == "" {
		return nil, rpc.NewError("schema_invalid", "work.send_message requires session_id and kind", nil)
	}
	body, err := sendMessageBody(envelope.Params)
	if err != nil {
		return nil, err
	}

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		runID, err := sendMessageSessionRunID(ctx, tx, repositoryID, sessionID)
		if err != nil {
			return nil, err
		}
		messageID, err := newID("msg")
		if err != nil {
			return nil, err
		}
		now := nowString()
		payload := map[string]any{"kind": kind, "body": body}
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.queue_messages (
			  repository_id, message_id, run_id, kind, state, payload_json,
			  created_at, updated_at
			)
			VALUES ($1,$2,$3,'agent_message','completed',$4,$5,$6)`,
			repositoryID,
			messageID,
			runID,
			payload,
			now,
			now,
		); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "message.sent", sessionID, nil, messageID, nil, nil, map[string]any{
			"kind": kind,
		}); err != nil {
			return nil, err
		}
		return map[string]any{"message_id": messageID}, nil
	})
}

func sendMessageSessionRunID(ctx context.Context, runner any, repositoryID, sessionID string) (string, error) {
	rower, ok := runner.(interface {
		QueryRow(context.Context, string, ...any) db.Row
	})
	if !ok {
		return "", fmt.Errorf("runner does not support query row")
	}
	var runID string
	err := rower.QueryRow(ctx, `
		SELECT run_id
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2
		 FOR UPDATE`,
		repositoryID,
		sessionID,
	).Scan(&runID)
	return runID, err
}

func sendMessageBody(params map[string]any) (map[string]any, error) {
	if value, exists := params["body"]; exists {
		if body, ok := value.(map[string]any); ok {
			return sendMessageCopyMap(body), nil
		}
		return nil, rpc.NewError("invalid_transition", "message body must be a JSON object", nil)
	}

	bodyJSON := "{}"
	if value, exists := params["body_json"]; exists {
		bodyJSON = fmt.Sprint(value)
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewBufferString(bodyJSON))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, rpc.NewError("schema_invalid", "work.send_message body_json must be valid JSON", map[string]any{
			"error": err.Error(),
		})
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return nil, rpc.NewError("schema_invalid", "work.send_message body_json must be valid JSON", map[string]any{
			"error": err.Error(),
		})
	}
	body, ok := decoded.(map[string]any)
	if !ok {
		return nil, rpc.NewError("invalid_transition", "message body must be a JSON object", nil)
	}
	return body, nil
}

func sendMessageCopyMap(value map[string]any) map[string]any {
	copied := make(map[string]any, len(value))
	for key, item := range value {
		copied[key] = item
	}
	return copied
}
