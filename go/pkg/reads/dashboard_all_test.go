package reads

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type dashboardAllFakeRunner struct{}

func (dashboardAllFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("dashboard.all must be read-only")
}

func (dashboardAllFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return dashboardAllFakeRow{}
}

func (dashboardAllFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (dashboardAllFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("dashboard.all must not open a transaction")
}

func (dashboardAllFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	now := time.Now().UTC().Truncate(time.Second)
	switch {
	case strings.Contains(sql, "FROM striatumd.repositories"):
		return dashboardAllRowsFromMaps([]map[string]any{
			{"repository_id": "repo_a", "display_name": "A", "repo_root": "/tmp/a", "state": "active"},
			{"repository_id": "repo_b", "display_name": "B", "repo_root": "/tmp/b", "state": "disabled"},
		}), nil
	case strings.Contains(sql, "SELECT r.run_id, r.state, r.branch_name"):
		if args[0] == "repo_a" {
			return dashboardAllRowsFromMaps([]map[string]any{{"run_id": "run_a", "state": "running", "branch_name": "main"}}), nil
		}
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "SELECT j.state, COUNT(*) AS count"):
		if args[0] == "repo_a" {
			return dashboardAllRowsFromMaps([]map[string]any{{"state": "queued", "count": int64(1)}}), nil
		}
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "FROM striatumd.blockers b"):
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "DISTINCT ON (v.job_id)") && strings.Contains(sql, "v.verdict != 'accept'"):
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "FROM striatumd.verdicts v") && strings.Contains(sql, "GROUP BY v.posture"):
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "FROM striatumd.queue_messages q"):
		if args[0] == "repo_a" {
			return dashboardAllRowsFromMaps([]map[string]any{{
				"run_id": "run_a", "job_id": "job_a", "workflow_job_id": "draft",
				"role_id": "author", "lane_id": "codex", "count": int64(1),
			}}), nil
		}
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "WHERE j.repository_id = $1 AND j.state = 'blocked'"):
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "FROM striatumd.sessions s"):
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "FROM striatumd.process_executions p"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"running_count": int64(0), "stale_running_count": int64(0),
			"lost_count": int64(0), "timed_out_count": int64(0),
		}}), nil
	case strings.Contains(sql, "SELECT run_id") && strings.Contains(sql, "state IN ('running', 'blocked')"):
		if args[0] == "repo_a" {
			return dashboardAllRowsFromMaps([]map[string]any{{"run_id": "run_a"}}), nil
		}
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "j.write_scope_json"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"job_id": "job_a", "workflow_job_id": "draft", "job_state": "stale_lease",
			"write_scope_json": map[string]any{"mode": "repo_write"},
			"lease_id":         "lease_a", "owner_session_id": "sess_a", "expires_at": "2026-05-17T00:00:00Z",
			"released_at": nil, "release_reason": nil, "message_id": "msg_a", "message_state": "blocked",
		}}), nil
	case strings.Contains(sql, "w.workflow_json"):
		if args[0] == "repo_a" {
			return dashboardAllRowsFromMaps([]map[string]any{{
				"run_id": "run_a", "state": "running", "paused_at": nil,
				"workflow_snapshot_id": "wfs_a", "repo_root": "/tmp/a",
				"workflow_json": dashboardAllWorkflow(),
			}}), nil
		}
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "SELECT j.job_id, j.workflow_job_id, j.state, j.attempt"):
		return dashboardAllRowsFromMaps([]map[string]any{
			{"job_id": "job_design", "workflow_job_id": "design", "state": "completed", "attempt": int64(1)},
			{"job_id": "job_synth", "workflow_job_id": "synthesize_design", "state": "running", "attempt": int64(1)},
		}), nil
	case strings.Contains(sql, "v.job_id, v.verdict_id, v.verdict"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"job_id": "job_synth", "verdict_id": "verdict_synth",
			"verdict": "accept_with_findings", "posture": "neutral", "created_at": now,
		}}), nil
	case strings.Contains(sql, "COUNT(*) AS candidate_count"):
		return dashboardAllRowsFromMaps([]map[string]any{{"candidate_count": int64(1)}}), nil
	case strings.Contains(sql, "FROM striatumd.process_supervisors ps"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"supervisor_id": "sup_a", "run_id": "run_a", "session_id": "sess_a", "pid": int64(123),
			"supervisor_state": "attached", "supervisor_heartbeat_at": now.Add(-10 * time.Minute),
			"session_last_heartbeat_at": now.Add(-10 * time.Minute),
			"lease_id":                  "lease_active", "job_id": "job_synth",
			"acquired_at":             now.Add(-10 * time.Minute),
			"expires_at":              now.Add(10 * time.Minute),
			"lease_last_heartbeat_at": now.Add(-10 * time.Minute),
			"workflow_job_id":         "synthesize_design", "job_state": "running",
			"message_id": "msg_active", "message_state": "acked",
		}}), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func dashboardAllWorkflow() map[string]any {
	return map[string]any{
		"workflow_id": "wf_dashboard",
		"recovery": map[string]any{
			"auto_finalize": map[string]any{"enabled": true},
		},
		"phases": []any{
			map[string]any{
				"id": "phase_design", "name": "Design",
				"synthesis_job_id": "synthesize_design",
			},
		},
		"jobs": []any{
			map[string]any{"id": "design", "phase_id": "phase_design"},
			map[string]any{"id": "synthesize_design", "phase_id": "phase_design"},
		},
	}
}

type dashboardAllFakeRow struct{}

func (dashboardAllFakeRow) Scan(...any) error {
	return errors.New("unexpected row scan")
}

func TestHandleDashboardAllBuildsGlobalProjectionReadOnly(t *testing.T) {
	result, err := HandleDashboardAll(context.Background(), dashboardAllFakeRunner{}, rpc.Envelope{})
	if err != nil {
		t.Fatalf("HandleDashboardAll: %v", err)
	}
	if result["mode"] != "daemon" || result["protocol_version"] != protocolVersion {
		t.Fatalf("unexpected envelope fields: %#v", result)
	}
	repos, ok := result["repositories"].([]map[string]any)
	if !ok || len(repos) != 2 {
		t.Fatalf("repositories = %#v", result["repositories"])
	}
	first := repos[0]
	if first["repository_id"] != "repo_a" || first["state"] != "active" {
		t.Fatalf("first repo = %#v", first)
	}
	status := first["status"].(map[string]any)
	jobs := status["jobs"].(map[string]int)
	if jobs["queued"] != 1 {
		t.Fatalf("jobs = %#v", jobs)
	}
	staleRuns := first["stale_leases"].([]map[string]any)
	if len(staleRuns) != 1 || staleRuns[0]["stale_count"] != 1 {
		t.Fatalf("stale leases = %#v", staleRuns)
	}
	staleEntry := staleRuns[0]["stale_leases"].([]map[string]any)[0]
	if staleEntry["repo_write"] != true || staleEntry["recovery_policy"] != "manual_inspection_required" {
		t.Fatalf("stale entry = %#v", staleEntry)
	}
	progress := first["run_progress"].([]map[string]any)
	if len(progress) != 1 || progress[0]["run_id"] != "run_a" {
		t.Fatalf("run progress = %#v", progress)
	}
	phases := progress[0]["phases"].([]map[string]any)
	if len(phases) != 1 || phases[0]["state"] != "active" || phases[0]["jobs_completed"] != 1 {
		t.Fatalf("phase progress = %#v", phases)
	}
	synthesisVerdict := phases[0]["synthesis_verdict"].(map[string]any)
	if synthesisVerdict["verdict"] != "accept_with_findings" {
		t.Fatalf("synthesis verdict = %#v", synthesisVerdict)
	}
	autoFinalize := progress[0]["auto_finalize_dry_run"].(map[string]any)
	if autoFinalize["candidate_count"] != 1 || autoFinalize["dry_run"] != true {
		t.Fatalf("auto-finalize summary = %#v", autoFinalize)
	}
	policy := autoFinalize["policy"].(map[string]any)
	if policy["workflow_enabled"] != true || policy["live_allowed"] != true {
		t.Fatalf("auto-finalize policy = %#v", policy)
	}
	supervisorStalls := progress[0]["supervisor_stalls"].(map[string]any)
	if supervisorStalls["stalled_count"] != 1 || supervisorStalls["warning_count"] != 1 {
		t.Fatalf("supervisor stalls = %#v", supervisorStalls)
	}
}

func TestHandleDashboardAllRegisters(t *testing.T) {
	server := rpc.NewServer()
	Register(server, dashboardAllFakeRunner{})
	handler, ok := server.Handlers["dashboard.all"]
	if !ok {
		t.Fatal("dashboard.all was not registered")
	}
	_, err := handler(context.Background(), rpc.Envelope{})
	if err != nil {
		t.Fatalf("dashboard.all handler returned error: %v", err)
	}
}

type dashboardAllFakeRows struct {
	fields []string
	rows   [][]any
	index  int
}

func dashboardAllRowsFromMaps(items []map[string]any) pgx.Rows {
	fields := []string{}
	if len(items) > 0 {
		for key := range items[0] {
			fields = append(fields, key)
		}
	}
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		row := make([]any, 0, len(fields))
		for _, field := range fields {
			row = append(row, item[field])
		}
		rows = append(rows, row)
	}
	return &dashboardAllFakeRows{fields: fields, rows: rows, index: -1}
}

func (r *dashboardAllFakeRows) Close() {}

func (r *dashboardAllFakeRows) Err() error { return nil }

func (r *dashboardAllFakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *dashboardAllFakeRows) FieldDescriptions() []pgconn.FieldDescription {
	out := make([]pgconn.FieldDescription, 0, len(r.fields))
	for _, field := range r.fields {
		out = append(out, pgconn.FieldDescription{Name: field})
	}
	return out
}

func (r *dashboardAllFakeRows) Next() bool {
	r.index++
	return r.index < len(r.rows)
}

func (r *dashboardAllFakeRows) Scan(dest ...any) error {
	if len(dest) == 1 {
		if scanner, ok := dest[0].(pgx.RowScanner); ok {
			return scanner.ScanRow(r)
		}
	}
	values, err := r.Values()
	if err != nil {
		return err
	}
	for i := range dest {
		if i >= len(values) {
			break
		}
		switch target := dest[i].(type) {
		case *any:
			*target = values[i]
		case *string:
			if value, ok := values[i].(string); ok {
				*target = value
			}
		case *int64:
			if value, ok := values[i].(int64); ok {
				*target = value
			}
		}
	}
	return nil
}

func (r *dashboardAllFakeRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.rows) {
		return nil, errors.New("no current row")
	}
	return r.rows[r.index], nil
}

func (r *dashboardAllFakeRows) RawValues() [][]byte { return nil }

func (r *dashboardAllFakeRows) Conn() *pgx.Conn { return nil }
