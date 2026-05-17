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

type runGraphFakeRunner struct{}

func (runGraphFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("run.graph must be read-only")
}

func (runGraphFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return runGraphFakeRow{}
}

func (runGraphFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (runGraphFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("run.graph must not open a transaction")
}

func (runGraphFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "workflow_json"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"workflow_id":      "workflow-graph",
			"workflow_version": "1",
			"workflow_json": map[string]any{
				"workflow_id":      "workflow-graph",
				"workflow_version": "1",
				"jobs": []any{
					map[string]any{"id": "draft", "type": "draft", "role_id": "author", "lane_id": "codex"},
					map[string]any{"id": "review", "type": "review", "role_id": "reviewer", "lane_id": "codex"},
				},
				"edges":  []any{map[string]any{"from": "draft", "to": "review", "on": "completed"}},
				"cycles": []any{},
			},
		}}), nil
	case strings.Contains(sql, "FROM striatumd.job_dependencies"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"from_id": "draft",
			"to_id":   "review",
			"gate_json": map[string]any{
				"on": "completed",
			},
		}}), nil
	case strings.Contains(sql, "FROM striatumd.jobs j"):
		return dashboardAllRowsFromMaps([]map[string]any{
			{"job_id": "job_draft_2", "workflow_job_id": "draft", "state": "completed", "attempt": int64(2), "job_type": "draft"},
			{"job_id": "job_review", "workflow_job_id": "review", "state": "completed", "attempt": int64(1), "job_type": "review"},
		}), nil
	case strings.Contains(sql, "FROM striatumd.verdicts v"):
		return dashboardAllRowsFromMaps([]map[string]any{{
			"job_id": "job_review", "verdict_id": "verdict_1", "verdict": "accept",
			"rationale": "ok", "findings_artifact_id": nil, "posture": "neutral",
		}}), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

type runGraphFakeRow struct{}

func (runGraphFakeRow) Scan(...any) error {
	return errors.New("unexpected row scan")
}

func TestHandleRunGraphJSONUsesPGDependenciesAndLatestState(t *testing.T) {
	result, err := HandleRunGraph(context.Background(), runGraphFakeRunner{}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": "run_1", "format": "json"},
	})
	if err != nil {
		t.Fatalf("HandleRunGraph: %v", err)
	}
	if result["workflow_id"] != "workflow-graph" || result["run_id"] != "run_1" {
		t.Fatalf("result header = %#v", result)
	}
	graph := result["graph"].(map[string]any)
	nodes := graph["nodes"].([]map[string]any)
	if nodes[0]["current_state"] != "completed" || nodes[0]["attempt"] != int64(2) {
		t.Fatalf("draft node = %#v", nodes[0])
	}
	if nodes[1]["latest_verdict"].(map[string]any)["verdict"] != "accept" {
		t.Fatalf("review node = %#v", nodes[1])
	}
	edges := graph["edges"].([]map[string]any)
	if len(edges) != 1 || edges[0]["from"] != "draft" || edges[0]["to"] != "review" {
		t.Fatalf("edges = %#v", edges)
	}
}

func TestHandleRunGraphMermaidIncludesStateClasses(t *testing.T) {
	result, err := HandleRunGraph(context.Background(), runGraphFakeRunner{}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": "run_1", "format": "mermaid"},
	})
	if err != nil {
		t.Fatalf("HandleRunGraph: %v", err)
	}
	source := result["source"].(string)
	for _, needle := range []string{"flowchart TD", "-->|completed|", "classDef state-completed", "class n0 state-completed"} {
		if !strings.Contains(source, needle) {
			t.Fatalf("source missing %q:\n%s", needle, source)
		}
	}
}
