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

type listArtifactsFakeRunner struct{}

func (listArtifactsFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("list artifacts must be read-only")
}

func (listArtifactsFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return dashboardAllFakeRow{}
}

func (listArtifactsFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (listArtifactsFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("list artifacts must not open a transaction")
}

func (listArtifactsFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, " AND kind =") ||
		strings.Contains(sql, " session_id, kind,") ||
		strings.Contains(sql, " ORDER BY published_at") {
		return nil, errors.New("list artifacts selected legacy artifact column names")
	}
	if !strings.Contains(sql, "artifact_kind AS kind") ||
		!strings.Contains(sql, "repo_path AS path") ||
		!strings.Contains(sql, "author_line AS byline") ||
		!strings.Contains(sql, "created_at AS published_at") ||
		!strings.Contains(sql, "AND artifact_kind = $3") {
		return nil, errors.New("list artifacts did not project current schema columns")
	}
	if len(args) != 3 || args[0] != "repo_a" || args[1] != "run_a" || args[2] != "synthesis" {
		return nil, errors.New("unexpected args")
	}
	return dashboardAllRowsFromMaps([]map[string]any{{
		"artifact_id":    "art_1",
		"run_id":         "run_a",
		"job_id":         "job_a",
		"session_id":     "sess_a",
		"kind":           "synthesis",
		"logical_name":   "summary",
		"path":           "docs/SUMMARY.md",
		"content_sha256": "abc",
		"byline":         "author: tester",
		"published_at":   "2026-05-21T00:00:00Z",
	}}), nil
}

type listWorkflowsFakeRunner struct{}

func (listWorkflowsFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("list workflows must be read-only")
}

func (listWorkflowsFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return dashboardAllFakeRow{}
}

func (listWorkflowsFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (listWorkflowsFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("list workflows must not open a transaction")
}

func (listWorkflowsFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	// #142: workflow_snapshots has content_sha256 / loaded_at, not
	// snapshot_sha256 / captured_at. Reject the legacy projection that 42703'd.
	if strings.Contains(sql, " snapshot_sha256, captured_at") ||
		strings.Contains(sql, "ORDER BY captured_at") {
		return nil, errors.New("list workflows selected legacy snapshot column names")
	}
	if !strings.Contains(sql, "content_sha256 AS snapshot_sha256") ||
		!strings.Contains(sql, "loaded_at AS captured_at") ||
		!strings.Contains(sql, "ORDER BY loaded_at") {
		return nil, errors.New("list workflows did not project current schema columns")
	}
	if len(args) != 1 || args[0] != "repo_a" {
		return nil, errors.New("unexpected args")
	}
	return dashboardAllRowsFromMaps([]map[string]any{{
		"workflow_snapshot_id": "wfs_1",
		"workflow_id":          "demo-flow",
		"workflow_version":     "2026-06-01",
		"snapshot_sha256":      "abc",
		"captured_at":          "2026-06-01T00:00:00Z",
	}}), nil
}

func TestHandleListWorkflowsUsesCurrentSchemaColumnNames(t *testing.T) {
	result, err := HandleListWorkflows(context.Background(), listWorkflowsFakeRunner{}, rpc.Envelope{
		Params: map[string]any{
			"repository_id": "repo_a",
		},
	})
	if err != nil {
		t.Fatalf("HandleListWorkflows: %v", err)
	}
	if result["count"] != 1 {
		t.Fatalf("count = %#v", result["count"])
	}
	items := result["items"].([]map[string]any)
	if items[0]["workflow_id"] != "demo-flow" || items[0]["snapshot_sha256"] != "abc" {
		t.Fatalf("workflow row = %#v", items[0])
	}
}

func TestHandleListArtifactsUsesCurrentSchemaColumnNames(t *testing.T) {
	result, err := HandleListArtifacts(context.Background(), listArtifactsFakeRunner{}, rpc.Envelope{
		Params: map[string]any{
			"repository_id": "repo_a",
			"run_id":        "run_a",
			"kind":          "synthesis",
		},
	})
	if err != nil {
		t.Fatalf("HandleListArtifacts: %v", err)
	}
	if result["count"] != 1 {
		t.Fatalf("count = %#v", result["count"])
	}
	items := result["items"].([]map[string]any)
	if items[0]["kind"] != "synthesis" || items[0]["path"] != "docs/SUMMARY.md" {
		t.Fatalf("artifact row = %#v", items[0])
	}
}
