package reads

import (
	"context"
	"errors"
	"strings"
	"testing"

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
	case strings.Contains(sql, "DISTINCT ON (v.job_id)"):
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
	default:
		return nil, errors.New("unexpected query: " + sql)
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
