package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func TestSendMessageRecordsCompletedAgentMessage(t *testing.T) {
	tx := &sendMessageFakeTx{runID: "run_1"}
	runner := &sendMessageFakeRunner{tx: tx}

	result, err := HandleSendMessage(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_send",
		Method:        "work.send_message",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_author",
			"kind":          "note",
			"body_json":     `{"summary":"working"}`,
		},
	})
	if err != nil {
		t.Fatalf("HandleSendMessage: %v", err)
	}
	messageID, ok := result["message_id"].(string)
	if !ok || !strings.HasPrefix(messageID, "msg_") {
		t.Fatalf("message_id = %#v, want msg_* string", result["message_id"])
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("transaction commit/rollback = %v/%v", tx.committed, tx.rolledBack)
	}

	queueInsert := tx.execContaining("INSERT INTO striatumd.queue_messages")
	if queueInsert == nil {
		t.Fatalf("queue message insert was not executed: %#v", tx.execs)
	}
	if queueInsert.args[0] != "repo_1" || queueInsert.args[1] != messageID || queueInsert.args[2] != "run_1" {
		t.Fatalf("queue insert scope args = %#v", queueInsert.args[:3])
	}
	payload, ok := queueInsert.args[3].(map[string]any)
	if !ok {
		t.Fatalf("queue payload arg = %#v", queueInsert.args[3])
	}
	if payload["kind"] != "note" {
		t.Fatalf("queue payload kind = %#v", payload)
	}
	body, ok := payload["body"].(map[string]any)
	if !ok || body["summary"] != "working" {
		t.Fatalf("queue payload body = %#v", payload["body"])
	}

	eventInsert := tx.execContaining("INSERT INTO striatumd.events")
	if eventInsert == nil {
		t.Fatalf("event insert was not executed: %#v", tx.execs)
	}
	if eventInsert.args[3] != "message.sent" {
		t.Fatalf("event type arg = %#v", eventInsert.args[3])
	}
	if eventInsert.args[4] != "sess_author" {
		t.Fatalf("event actor_session_id arg = %#v", eventInsert.args[4])
	}
	if eventInsert.args[6] != messageID {
		t.Fatalf("event message_id arg = %#v", eventInsert.args[6])
	}
	eventPayload, ok := eventInsert.args[9].(map[string]any)
	if !ok || eventPayload["kind"] != "note" {
		t.Fatalf("event payload = %#v", eventInsert.args[9])
	}
}

func TestSendMessageAcceptsBodyObjectParam(t *testing.T) {
	original := map[string]any{"summary": "direct"}
	body, err := sendMessageBody(map[string]any{
		"body": original,
	})
	if err != nil {
		t.Fatalf("sendMessageBody: %v", err)
	}
	if body["summary"] != "direct" {
		t.Fatalf("body = %#v", body)
	}
	body["summary"] = "changed"
	if original["summary"] != "direct" {
		t.Fatalf("body map was aliased: %#v", original)
	}
}

func TestSendMessageRejectsNonObjectBodyBeforeTransaction(t *testing.T) {
	runner := &sendMessageFakeRunner{tx: &sendMessageFakeTx{runID: "run_1"}}
	_, err := HandleSendMessage(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_send_bad_body",
		Method:        "work.send_message",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_author",
			"kind":          "note",
			"body_json":     `[]`,
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("error = %v, want invalid_transition", err)
	}
	if rpcErr.Message != "message body must be a JSON object" {
		t.Fatalf("error message = %q", rpcErr.Message)
	}
	if runner.beginCount != 0 {
		t.Fatalf("BeginTx called for invalid body: %d", runner.beginCount)
	}
}

type sendMessageFakeRunner struct {
	tx         *sendMessageFakeTx
	beginCount int
}

func (r *sendMessageFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *sendMessageFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return sendMessageFakeRow{err: errors.New("unexpected runner query outside tx")}
}

func (r *sendMessageFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected runner query scalar outside tx")
}

func (r *sendMessageFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	r.beginCount++
	return r.tx, nil
}

type sendMessageFakeTx struct {
	runID      string
	nextEvent  int64
	execs      []sendMessageExec
	committed  bool
	rolledBack bool
}

type sendMessageExec struct {
	sql  string
	args []any
}

func (tx *sendMessageFakeTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, sendMessageExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *sendMessageFakeTx) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	switch {
	case strings.Contains(sql, "FROM striatumd.sessions"):
		if args[0] != "repo_1" || args[1] != "sess_author" {
			return sendMessageFakeRow{err: errors.New("unexpected session query args")}
		}
		return sendMessageFakeRow{values: []any{tx.runID}}
	case strings.Contains(sql, "repo_event_chain_heads"):
		return sendMessageFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		tx.nextEvent++
		return sendMessageFakeRow{values: []any{tx.nextEvent}}
	default:
		return sendMessageFakeRow{err: errors.New("unexpected query: " + sql)}
	}
}

func (tx *sendMessageFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *sendMessageFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *sendMessageFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *sendMessageFakeTx) execContaining(part string) *sendMessageExec {
	for i := range tx.execs {
		if strings.Contains(tx.execs[i].sql, part) {
			return &tx.execs[i]
		}
	}
	return nil
}

type sendMessageFakeRow struct {
	values []any
	err    error
}

func (r sendMessageFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = value.(string)
		case **string:
			if value == nil {
				*target = nil
			} else {
				text := value.(string)
				*target = &text
			}
		case *int64:
			*target = value.(int64)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
