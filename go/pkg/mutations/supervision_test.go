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

func TestSuperviseReportRecordsAgentExit(t *testing.T) {
	tx := &superviseReportFakeTx{
		supervisor: supervisorReportRow{
			SupervisorID:       "sup_1",
			RunID:              "run_1",
			SessionID:          "sess_1",
			State:              "attached",
			DaemonSupervisorID: "dsup_1",
		},
	}
	runner := &superviseReportFakeRunner{tx: tx}

	result, err := HandleSuperviseReport(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_supervise_report",
		Method:        "supervise.report",
		Params: map[string]any{
			"repository_id": "repo_1",
			"supervisor_id": "sup_1",
			"session_id":    "sess_1",
			"event_type":    "agent_exited",
			"payload":       map[string]any{"exit_code": 7},
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseReport: %v", err)
	}
	if result["state"] != "stopped" {
		t.Fatalf("state = %v, want stopped", result["state"])
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("transaction commit/rollback = %v/%v", tx.committed, tx.rolledBack)
	}
	if !tx.sawExec("UPDATE striatumd.process_supervisors", "state = 'stopped'") {
		t.Fatalf("process supervisor stop update was not executed: %#v", tx.execs)
	}
	if !tx.sawExec("UPDATE striatumd.process_supervisor_pointers", "state = 'stopped'") {
		t.Fatalf("pointer stop update was not executed: %#v", tx.execs)
	}
	if !tx.sawExec("UPDATE striatumd.daemon_supervisors", "state = 'stopped'") {
		t.Fatalf("daemon supervisor stop update was not executed: %#v", tx.execs)
	}

	event := tx.eventInsert()
	if event == nil {
		t.Fatalf("event insert was not executed: %#v", tx.execs)
	}
	if got := event.args[3]; got != "supervisor.agent_exited" {
		t.Fatalf("event_type arg = %v, want supervisor.agent_exited", got)
	}
	payload, ok := event.args[9].(map[string]any)
	if !ok {
		t.Fatalf("event payload arg = %#v", event.args[9])
	}
	if payload["daemon_supervisor_id"] != "dsup_1" {
		t.Fatalf("daemon_supervisor_id payload = %#v", payload)
	}
	nested, ok := payload["payload"].(map[string]any)
	if !ok || nested["exit_code"] != 7 {
		t.Fatalf("nested payload = %#v", payload["payload"])
	}
}

func TestSuperviseReportValidatesHelperBatchBeforeTransaction(t *testing.T) {
	runner := &superviseReportFakeRunner{tx: &superviseReportFakeTx{}}
	_, err := HandleSuperviseReport(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_supervise_report_bad_batch",
		Method:        "supervise.report",
		Params: map[string]any{
			"repository_id": "repo_1",
			"supervisor_id": "sup_1",
			"events_jsonl":  "{\"schema_version\":\"striatum.supervisor_helper.event.v1\",\"event_type\":\"progress\",\"supervisor_id\":\"sup_1\"}\n{\"event_type\":",
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("error = %v, want schema_invalid", err)
	}
	if runner.beginCount != 0 {
		t.Fatalf("BeginTx called for malformed batch: %d", runner.beginCount)
	}
}

func TestSuperviseReportRecordsHelperBatch(t *testing.T) {
	tx := &superviseReportFakeTx{
		supervisor: supervisorReportRow{
			SupervisorID:       "sup_1",
			RunID:              "run_1",
			SessionID:          "sess_1",
			State:              "attached",
			DaemonSupervisorID: "dsup_1",
		},
	}
	runner := &superviseReportFakeRunner{tx: tx}

	result, err := HandleSuperviseReport(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_supervise_report_batch",
		Method:        "supervise.report",
		Params: map[string]any{
			"repository_id": "repo_1",
			"supervisor_id": "sup_1",
			"events": []any{
				map[string]any{
					"schema_version": "striatum.supervisor_helper.event.v1",
					"event_type":     "packet_accepted",
					"packet_id":      "packet_1",
					"timestamp":      "2026-05-17T00:00:00Z",
					"payload":        map[string]any{"bytes_read": 123},
				},
				map[string]any{
					"schema_version": "striatum.supervisor_helper.event.v1",
					"event_type":     "progress",
					"payload":        map[string]any{"kind": "heartbeat"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseReport batch: %v", err)
	}
	if result["events_recorded"] != 2 {
		t.Fatalf("events_recorded = %v, want 2", result["events_recorded"])
	}
	if len(tx.eventInserts()) != 2 {
		t.Fatalf("event inserts = %d, want 2", len(tx.eventInserts()))
	}
	if result["state"] != "attached" {
		t.Fatalf("batch state = %v, want attached", result["state"])
	}
}

type superviseReportFakeRunner struct {
	tx         *superviseReportFakeTx
	beginCount int
}

func (r *superviseReportFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *superviseReportFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return superviseReportFakeRow{err: errors.New("unexpected runner query outside tx")}
}

func (r *superviseReportFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected runner query scalar outside tx")
}

func (r *superviseReportFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	r.beginCount++
	return r.tx, nil
}

type superviseReportFakeTx struct {
	supervisor supervisorReportRow
	nextEvent  int64
	execs      []superviseReportExec
	committed  bool
	rolledBack bool
}

type superviseReportExec struct {
	sql  string
	args []any
}

func (tx *superviseReportFakeTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, superviseReportExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *superviseReportFakeTx) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	switch {
	case strings.Contains(sql, "FROM striatumd.process_supervisors"):
		dsup := tx.supervisor.DaemonSupervisorID
		return superviseReportFakeRow{values: []any{
			tx.supervisor.SupervisorID,
			tx.supervisor.RunID,
			tx.supervisor.SessionID,
			tx.supervisor.State,
			&dsup,
		}}
	case strings.Contains(sql, "repo_event_chain_heads"):
		return superviseReportFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		tx.nextEvent++
		return superviseReportFakeRow{values: []any{tx.nextEvent}}
	default:
		return superviseReportFakeRow{err: errors.New("unexpected query: " + sql)}
	}
}

func (tx *superviseReportFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *superviseReportFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *superviseReportFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *superviseReportFakeTx) sawExec(parts ...string) bool {
	for _, exec := range tx.execs {
		ok := true
		for _, part := range parts {
			if !strings.Contains(exec.sql, part) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func (tx *superviseReportFakeTx) eventInsert() *superviseReportExec {
	events := tx.eventInserts()
	if len(events) == 0 {
		return nil
	}
	return &events[0]
}

func (tx *superviseReportFakeTx) eventInserts() []superviseReportExec {
	events := []superviseReportExec{}
	for _, exec := range tx.execs {
		if strings.Contains(exec.sql, "INSERT INTO striatumd.events") {
			events = append(events, exec)
		}
	}
	return events
}

type superviseReportFakeRow struct {
	values []any
	err    error
}

func (r superviseReportFakeRow) Scan(dest ...any) error {
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
			} else if ptr, ok := value.(*string); ok {
				*target = ptr
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
