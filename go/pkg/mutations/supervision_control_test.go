package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func TestSuperviseStartInsertsAndAttachesProcessSupervisor(t *testing.T) {
	origMkfifo := supervisionMkfifo
	origLaunch := supervisionLaunch
	defer func() {
		supervisionMkfifo = origMkfifo
		supervisionLaunch = origLaunch
	}()
	supervisionMkfifo = func(path string) error {
		return os.WriteFile(path, nil, 0o600)
	}
	supervisionLaunch = func(context.Context, supervisionStartConfig, string, string, string, string) (supervisionLaunchResult, error) {
		return supervisionLaunchResult{PID: os.Getpid(), PIDStartTime: "start-token"}, nil
	}

	repoRoot := t.TempDir()
	tx1 := &superviseControlFakeTx{}
	tx2 := &superviseControlFakeTx{}
	runner := &superviseControlFakeRunner{
		repoRoot: repoRoot,
		txs:      []*superviseControlFakeTx{tx1, tx2},
	}
	result, err := HandleSuperviseStart(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_start",
		Method:        "supervise.start",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStart: %v", err)
	}
	if result["state"] != "attached" || result["session_id"] != "sess_1" || result["run_id"] != "run_1" {
		t.Fatalf("start result = %#v", result)
	}
	if result["lane_attestation"] != "attested" {
		t.Fatalf("lane_attestation = %v", result["lane_attestation"])
	}
	if !tx1.sawExec("INSERT INTO striatumd.process_supervisors") {
		t.Fatalf("missing process_supervisors insert: %#v", tx1.execs)
	}
	if !tx1.sawExec("INSERT INTO striatumd.daemon_supervisors") {
		t.Fatalf("missing daemon_supervisors insert: %#v", tx1.execs)
	}
	if !tx2.sawExec("UPDATE striatumd.process_supervisors", "state = $1") {
		t.Fatalf("missing attached process update: %#v", tx2.execs)
	}
	if len(tx1.eventInserts()) != 1 || len(tx2.eventInserts()) != 1 {
		t.Fatalf("event inserts tx1/tx2 = %d/%d", len(tx1.eventInserts()), len(tx2.eventInserts()))
	}
}

func TestSuperviseSendDeliversPacketUnacknowledged(t *testing.T) {
	dir := t.TempDir()
	pipePath := dir + "/stdin.pipe"
	if err := syscall.Mkfifo(pipePath, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	readDone := make(chan []byte, 1)
	go func() {
		data, _ := os.ReadFile(pipePath)
		readDone <- data
	}()

	tx := &superviseControlFakeTx{pipePath: pipePath, pid: os.Getpid()}
	runner := &superviseControlFakeRunner{txs: []*superviseControlFakeTx{tx}}
	result, err := HandleSuperviseSend(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_send",
		Method:        "supervise.send",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"packet_id":     "packet_1",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseSend: %v", err)
	}
	if result["delivery_state"] != "delivered_unacknowledged" || result["control_ack_expected"] != true {
		t.Fatalf("send result = %#v", result)
	}
	body := <-readDone
	var packet map[string]any
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatalf("packet json = %q: %v", string(body), err)
	}
	if packet["packet"] != "body" {
		t.Fatalf("delivered packet = %#v", packet)
	}
	if !tx.sawExec("UPDATE striatumd.process_supervisors", "heartbeat_at") {
		t.Fatalf("missing heartbeat update: %#v", tx.execs)
	}
	event := tx.lastEventInsert()
	if event == nil || event.args[3] != "supervisor.packet_delivered" {
		t.Fatalf("event insert = %#v", event)
	}
	payload := event.args[9].(map[string]any)
	if payload["packet_id"] != "packet_1" || payload["stdin_delivery"] != stdinDeliveryPersistentFIFO {
		t.Fatalf("event payload = %#v", payload)
	}
}

func TestSuperviseSendWrongKindPacketIDPointsAtClaimNextPacketID(t *testing.T) {
	tx := &superviseControlFakeTx{}
	runner := &superviseControlFakeRunner{txs: []*superviseControlFakeTx{tx}}
	_, err := HandleSuperviseSend(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_send_wrong_id",
		Method:        "supervise.send",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"packet_id":     "msg_123",
		},
	})
	if err == nil {
		t.Fatalf("expected supervise send to reject wrong-kind packet id")
	}
	rpcErr, ok := err.(*rpc.Error)
	if !ok || rpcErr.Code != "not_found" {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(rpcErr.Message, "msg_123 is a message id, not a work packet id") ||
		!strings.Contains(rpcErr.Message, "data.packet_id") ||
		!strings.Contains(rpcErr.Message, "data.packet.packet_id") {
		t.Fatalf("message = %q", rpcErr.Message)
	}
}

func TestSuperviseStopMarksSupervisorStoppedAndUnlinksPipe(t *testing.T) {
	dir := t.TempDir()
	pipePath := dir + "/stdin.pipe"
	if err := os.WriteFile(pipePath, nil, 0o600); err != nil {
		t.Fatalf("write pipe placeholder: %v", err)
	}
	tx := &superviseControlFakeTx{pipePath: pipePath}
	runner := &superviseControlFakeRunner{txs: []*superviseControlFakeTx{tx}, pipePath: pipePath}
	result, err := HandleSuperviseStop(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_stop",
		Method:        "supervise.stop",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_1",
			"reason":        "operator_requested",
		},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStop: %v", err)
	}
	if result["state"] != "stopped" || result["stop_reason"] != "operator_requested" {
		t.Fatalf("stop result = %#v", result)
	}
	if _, err := os.Stat(pipePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pipe still exists or unexpected stat err: %v", err)
	}
	if !tx.sawExec("UPDATE striatumd.process_supervisors", "ended_at") {
		t.Fatalf("missing stopped update: %#v", tx.execs)
	}
	event := tx.lastEventInsert()
	if event == nil || event.args[3] != "supervisor.stopped" {
		t.Fatalf("event insert = %#v", event)
	}
}

type superviseControlFakeRunner struct {
	mu       sync.Mutex
	repoRoot string
	pipePath string
	txs      []*superviseControlFakeTx
}

func (r *superviseControlFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *superviseControlFakeRunner) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	return r.fakeRow(sql, args...)
}

func (r *superviseControlFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (r *superviseControlFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.txs) == 0 {
		return nil, errors.New("unexpected BeginTx")
	}
	tx := r.txs[0]
	r.txs = r.txs[1:]
	if tx.pipePath == "" {
		tx.pipePath = r.pipePath
	}
	return tx, nil
}

func (r *superviseControlFakeRunner) fakeRow(sql string, args ...any) db.Row {
	switch {
	case strings.Contains(sql, "SELECT s.session_id, s.run_id"):
		return superviseControlFakeRow{values: []any{"sess_1", "run_1", "lane_1", "active", "snap_1", r.repoRoot}}
	case strings.Contains(sql, "SELECT state FROM striatumd.sessions"):
		return superviseControlFakeRow{values: []any{"active"}}
	case strings.Contains(sql, "SELECT workflow_json"):
		return superviseControlFakeRow{values: []any{map[string]any{
			"lanes": map[string]any{
				"lane_1": map[string]any{
					"adapter": "process",
					"command": []any{"/bin/cat"},
				},
			},
		}}}
	case strings.Contains(sql, "SELECT supervisor_id, state") && strings.Contains(sql, "state = ANY"):
		return superviseControlFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "SELECT session_id") && strings.Contains(sql, "FROM striatumd.sessions"):
		return superviseControlFakeRow{values: []any{"sess_1"}}
	case strings.Contains(sql, "SELECT ps.supervisor_id"):
		return superviseControlFakeRow{values: []any{"sup_1", "run_1", "sess_1", "attached", r.pipePath, nil, "", "dsup_1", map[string]any{"stdin_delivery": stdinDeliveryPersistentFIFO}}}
	default:
		return superviseControlFakeRow{err: errors.New("unexpected runner query: " + sql)}
	}
}

type superviseControlFakeTx struct {
	pipePath   string
	pid        int
	nextEvent  int64
	execs      []superviseControlExec
	committed  bool
	rolledBack bool
}

type superviseControlExec struct {
	sql  string
	args []any
}

func (tx *superviseControlFakeTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, superviseControlExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *superviseControlFakeTx) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	switch {
	case strings.Contains(sql, "SELECT supervisor_id, state") && strings.Contains(sql, "state = ANY"):
		return superviseControlFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "SELECT ps.supervisor_id"):
		var pid any
		if tx.pid > 0 {
			pid = tx.pid
		}
		return superviseControlFakeRow{values: []any{"sup_1", "run_1", "sess_1", "attached", tx.pipePath, pid, "", "dsup_1", map[string]any{"stdin_delivery": stdinDeliveryPersistentFIFO}}}
	case strings.Contains(sql, "FROM striatumd.work_packets"):
		return superviseControlFakeRow{values: []any{"packet_1", "run_1", "job_1", "lease_1", "sess_1", map[string]any{"packet": "body"}}}
	case strings.Contains(sql, "FROM striatumd.leases"):
		return superviseControlFakeRow{values: []any{"active", "sess_1", "job_1", "2999-01-01T00:00:00Z"}}
	case strings.Contains(sql, "SELECT state, daemon_supervisor_id"):
		dsup := "dsup_1"
		return superviseControlFakeRow{values: []any{"attached", &dsup}}
	case strings.Contains(sql, "FROM striatumd.daemon_supervisors") && strings.Contains(sql, "SELECT state"):
		return superviseControlFakeRow{values: []any{"attached"}}
	case strings.Contains(sql, "SELECT metadata_json"):
		return superviseControlFakeRow{values: []any{map[string]any{"stdin_delivery": stdinDeliveryPersistentFIFO}}}
	case strings.Contains(sql, "repo_event_chain_heads"):
		return superviseControlFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		tx.nextEvent++
		return superviseControlFakeRow{values: []any{tx.nextEvent}}
	default:
		return superviseControlFakeRow{err: errors.New("unexpected tx query: " + sql)}
	}
}

func (tx *superviseControlFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *superviseControlFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *superviseControlFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *superviseControlFakeTx) sawExec(parts ...string) bool {
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

func (tx *superviseControlFakeTx) eventInserts() []superviseControlExec {
	var events []superviseControlExec
	for _, exec := range tx.execs {
		if strings.Contains(exec.sql, "INSERT INTO striatumd.events") {
			events = append(events, exec)
		}
	}
	return events
}

func (tx *superviseControlFakeTx) lastEventInsert() *superviseControlExec {
	events := tx.eventInserts()
	if len(events) == 0 {
		return nil
	}
	return &events[len(events)-1]
}

type superviseControlFakeRow struct {
	values []any
	err    error
}

func (r superviseControlFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > len(r.values) {
		return errors.New("not enough fake row values")
	}
	for i, value := range r.values[:len(dest)] {
		switch target := dest[i].(type) {
		case *string:
			if value == nil {
				*target = ""
			} else {
				*target = value.(string)
			}
		case **string:
			if value == nil {
				*target = nil
			} else if ptr, ok := value.(*string); ok {
				*target = ptr
			} else {
				text := value.(string)
				*target = &text
			}
		case **int:
			if value == nil {
				*target = nil
			} else {
				typed := value.(int)
				*target = &typed
			}
		case *int64:
			*target = value.(int64)
		case *any:
			*target = value
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
