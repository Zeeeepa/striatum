package reads

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type runSummaryFakeRunner struct{}

func (runSummaryFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("run.summary must be read-only")
}

func (runSummaryFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return dashboardAllFakeRow{}
}

func (runSummaryFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("doctor unavailable in run.summary test")
}

func (runSummaryFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("run.summary must not open a transaction")
}

func (runSummaryFakeRunner) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "SELECT w.workflow_json"):
		return dashboardAllRowsFromMaps([]map[string]any{{"workflow_json": map[string]any{
			"operator_mode": "constrained",
		}}}), nil
	case strings.Contains(sql, "FROM striatumd.runs"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"run_id": "run_a", "workflow_snapshot_id": "wfs_a", "repo_root": "/tmp/repo",
			"state": "running", "branch_name": "main",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"workflow_job_id": "draft", "job_id": "job_draft", "role_id": "author",
			"state": "running", "attempt": int64(1),
		}}), nil
	case strings.Contains(sql, "FROM striatumd.artifacts"):
		return dashboardAllRowsFromMaps(nil), nil
	case strings.Contains(sql, "FROM striatumd.verdicts"):
		return dashboardAllRowsFromMaps(nil), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func TestHandleRunSummarySurfacesOperatorMode(t *testing.T) {
	result, err := HandleRunSummary(context.Background(), runSummaryFakeRunner{}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_a", "run_id": "run_a"},
	})
	if err != nil {
		t.Fatalf("HandleRunSummary: %v", err)
	}
	if result["operator_mode"] != "constrained" {
		t.Fatalf("operator_mode = %#v", result["operator_mode"])
	}
}
