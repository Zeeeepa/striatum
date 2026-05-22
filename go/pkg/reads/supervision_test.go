package reads

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type superviseReadFakeRunner struct {
	listQuery string
	listArgs  []any
	execCount int
}

func (r *superviseReadFakeRunner) Exec(context.Context, string, ...any) error {
	r.execCount++
	return errors.New("supervisor read handlers must be read-only")
}

func (r *superviseReadFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return superviseReadFakeRow{}
}

func (r *superviseReadFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r *superviseReadFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("supervisor read handlers must not open transactions")
}

func (r *superviseReadFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.runs"):
		return dashboardAllRowsFromMaps([]map[string]any{{"run_id": "run_1"}}), nil
	case strings.Contains(sql, "FROM striatumd.sessions") && strings.Contains(sql, "last_mcp_request_at"):
		now := time.Now().UTC()
		return dashboardAllRowsFromMaps([]map[string]any{
			{
				"session_id":              "sess_1",
				"state":                   "active",
				"registered_at":           now.Add(-10 * time.Minute),
				"last_tools_list_at":      now.Add(-9 * time.Minute),
				"last_await_packet_at":    now.Add(-8 * time.Minute),
				"last_mcp_request_at":     now.Add(-6 * time.Minute),
				"liveness_stall_class":    nil,
				"liveness_stall_since":    nil,
				"active_lease_id":         nil,
				"active_lease_expires_at": nil,
			},
		}), nil
	case strings.Contains(sql, "FROM striatumd.sessions") && strings.Contains(sql, "LIMIT 1"):
		return dashboardAllRowsFromMaps([]map[string]any{{"session_id": "sess_1"}}), nil
	case strings.Contains(sql, "LEFT JOIN striatumd.daemon_supervisors ds"):
		rows := superviseReattachRows()
		if len(args) > 1 {
			if supervisorID, ok := args[len(args)-1].(string); ok && strings.HasPrefix(supervisorID, "sup_") {
				rows = filterSuperviseRows(rows, supervisorID)
			}
		}
		return dashboardAllRowsFromMaps(rows), nil
	case strings.Contains(sql, "FROM striatumd.process_supervisors") && strings.Contains(sql, "ORDER BY ps.started_at DESC"):
		r.listQuery = sql
		r.listArgs = args
		if strings.Contains(sql, "LIMIT 1") {
			return dashboardAllRowsFromMaps([]map[string]any{superviseBaseRow("sup_gone", 0, "stale-start-token")}), nil
		}
		return dashboardAllRowsFromMaps([]map[string]any{superviseBaseRow("sup_reattachable", os.Getpid(), currentStartTokenForTest())}), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

type superviseReadFakeRow struct{}

func (superviseReadFakeRow) Scan(...any) error {
	return errors.New("unexpected row scan")
}

func TestHandleSuperviseListScopesByRepositoryRunAndState(t *testing.T) {
	runner := &superviseReadFakeRunner{}
	result, err := HandleSuperviseList(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": "run_1", "state": "attached"},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseList: %v", err)
	}
	if !strings.Contains(runner.listQuery, "ps.repository_id = $1 AND ps.run_id = $2") || !strings.Contains(runner.listQuery, "ps.state = $3") {
		t.Fatalf("query is not repo/run/state scoped: %s", runner.listQuery)
	}
	if len(runner.listArgs) != 3 || runner.listArgs[0] != "repo_1" || runner.listArgs[1] != "run_1" || runner.listArgs[2] != "attached" {
		t.Fatalf("args = %#v", runner.listArgs)
	}
	if result["state"] != "attached" {
		t.Fatalf("state = %#v", result["state"])
	}
	supervisors := result["supervisors"].([]map[string]any)
	if len(supervisors) != 1 || supervisors[0]["supervisor_id"] != "sup_reattachable" {
		t.Fatalf("supervisors = %#v", supervisors)
	}
	tmux := supervisors[0]["tmux"].(map[string]any)
	if tmux["session_name"] != "striatum-run_1-lane_1-sup_reattachable" {
		t.Fatalf("supervisor tmux metadata = %#v", tmux)
	}
	if runner.execCount != 0 {
		t.Fatalf("unexpected exec count %d", runner.execCount)
	}
}

func TestHandleSuperviseReattachStatusSummarizesLivenessIdentityAndRepair(t *testing.T) {
	runner := &superviseReadFakeRunner{}
	result, err := HandleSuperviseReattachStatus(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": "run_1"},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseReattachStatus: %v", err)
	}
	summary := result["summary"].(map[string]int)
	if summary["total"] != 3 || summary["reattachable"] != 1 || summary["lost_candidate"] != 1 || summary["needs_repair"] != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	byID := map[string]map[string]any{}
	for _, row := range result["supervisors"].([]map[string]any) {
		byID[row["supervisor_id"].(string)] = row
	}
	if byID["sup_reattachable"]["pid_identity"] != "matched" || byID["sup_reattachable"]["reattach_state"] != "reattachable" {
		t.Fatalf("reattachable row = %#v", byID["sup_reattachable"])
	}
	if byID["sup_gone"]["reattach_reason"] != "pid_gone" {
		t.Fatalf("gone row = %#v", byID["sup_gone"])
	}
	if byID["sup_repair"]["reattach_reason"] != "pointer_missing" {
		t.Fatalf("repair row = %#v", byID["sup_repair"])
	}
	if runner.execCount != 0 {
		t.Fatalf("unexpected exec count %d", runner.execCount)
	}
}

func TestHandleSuperviseStatusKeepsGonePIDAsReadProjection(t *testing.T) {
	runner := &superviseReadFakeRunner{}
	result, err := HandleSuperviseStatus(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "session_id": "sess_1"},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStatus: %v", err)
	}
	if result["state"] != "attached" || result["liveness"] != "gone" {
		t.Fatalf("status projection = %#v", result)
	}
	tmux := result["tmux"].(map[string]any)
	if tmux["attach_command"] != "tmux attach-session -t striatum-run_1-lane_1-sup_gone" {
		t.Fatalf("status tmux metadata = %#v", tmux)
	}
	protocolLiveness, ok := result["protocol_liveness"].(map[string]any)
	if !ok || protocolLiveness["stall_class"] != "agent_protocol_idle_stall" {
		t.Fatalf("protocol liveness = %#v", result["protocol_liveness"])
	}
	if result["lane_attestation"] != "unattested" || result["lane_attestation_reason"] != "no_live_attached_supervisor" {
		t.Fatalf("lane attestation = %#v", result)
	}
	if runner.execCount != 0 {
		t.Fatalf("unexpected exec count %d", runner.execCount)
	}
}

func TestSuperviseReadHandlersValidateParamsBeforeQuery(t *testing.T) {
	tests := []struct {
		name    string
		handler handlerFn
		params  map[string]any
	}{
		{
			name:    "status session",
			handler: HandleSuperviseStatus,
			params:  map[string]any{"repository_id": "repo_1"},
		},
		{
			name:    "list run",
			handler: HandleSuperviseList,
			params:  map[string]any{"repository_id": "repo_1"},
		},
		{
			name:    "reattach run id",
			handler: HandleSuperviseReattachStatus,
			params:  map[string]any{"repository_id": "repo_1", "run_id": 42},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.handler(context.Background(), nil, rpc.Envelope{Params: tc.params})
			var rpcErr *rpc.Error
			if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
				t.Fatalf("error = %v, want schema_invalid", err)
			}
		})
	}
}

func superviseReattachRows() []map[string]any {
	token := currentStartTokenForTest()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	row := func(supervisorID string, pid int, pidStart string) map[string]any {
		base := superviseBaseRow(supervisorID, pid, pidStart)
		base["started_at"] = now
		base["pointer_daemon_supervisor_id"] = "dsup_" + supervisorID
		base["pointer_pid"] = pid
		base["pointer_pid_start_time"] = pidStart
		base["pointer_state"] = "attached"
		base["pointer_updated_at"] = now
		base["pointer_metadata_json"] = map[string]any{"source": "test"}
		base["daemon_supervisor_id"] = "dsup_" + supervisorID
		base["daemon_instance_id"] = "daemon_1"
		base["daemon_pid"] = pid
		base["daemon_pid_start_time"] = pidStart
		base["daemon_state"] = "attached"
		base["daemon_heartbeat_at"] = now
		base["daemon_ended_at"] = nil
		base["daemon_stop_reason"] = nil
		return base
	}
	reattachable := row("sup_reattachable", os.Getpid(), token)
	gone := row("sup_gone", 0, "stale-start-token")
	repair := row("sup_repair", os.Getpid(), token)
	repair["pointer_daemon_supervisor_id"] = nil
	repair["daemon_supervisor_id"] = nil
	return []map[string]any{reattachable, gone, repair}
}

func filterSuperviseRows(rows []map[string]any, supervisorID string) []map[string]any {
	filtered := []map[string]any{}
	for _, row := range rows {
		if row["supervisor_id"] == supervisorID {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func superviseBaseRow(supervisorID string, pid int, pidStart string) map[string]any {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	return map[string]any{
		"repository_id":   "repo_1",
		"supervisor_id":   supervisorID,
		"run_id":          "run_1",
		"session_id":      "sess_1",
		"adapter":         "process",
		"command_json":    []any{"sleep", "600"},
		"cwd":             "/tmp/repo",
		"scratch_path":    "/tmp/repo/.striatum/scratch/" + supervisorID,
		"stdin_pipe_path": "/tmp/repo/.striatum/scratch/" + supervisorID + "/stdin.pipe",
		"pid":             pid,
		"pid_start_time":  pidStart,
		"state":           "attached",
		"started_at":      now,
		"heartbeat_at":    now,
		"ended_at":        nil,
		"stop_reason":     nil,
		"pointer_metadata_json": map[string]any{
			"tmux": map[string]any{
				"session_name":   "striatum-run_1-lane_1-" + supervisorID,
				"attach_command": "tmux attach-session -t striatum-run_1-lane_1-" + supervisorID,
			},
		},
	}
}

func currentStartTokenForTest() string {
	token, ok := processStartToken(os.Getpid())
	if !ok {
		return ""
	}
	return token
}
