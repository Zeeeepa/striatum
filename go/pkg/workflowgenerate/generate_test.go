package workflowgenerate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPreviewReturnsPlannedWritesWithoutWriting(t *testing.T) {
	repo := t.TempDir()
	generated := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "review",
		"lane_set":         "local",
		"workflow_id":      "demo",
		"name":             "Demo",
		"workflow_version": "2026-05-17",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": false},
		"scaffold_root":    "workflows/demo",
		"artifact_root":    "striatum/demo",
		"lanes":            map[string]any{},
		"options":          map[string]any{},
		"lane_modifiers":   []any{},
		"context_docs":     []any{},
	})
	planned, err := Preview(repo, generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) == 0 {
		t.Fatal("expected planned writes")
	}
	if planned[0]["status"] != "would_create" {
		t.Fatalf("first status = %#v", planned[0])
	}
	if _, err := os.Stat(filepath.Join(repo, "workflows", "demo", "workflow.json")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote workflow.json or stat failed: %v", err)
	}
}

func TestWriteCreatesOnlySafeRepoRelativeTargets(t *testing.T) {
	repo := t.TempDir()
	generated := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "minimal",
		"lane_set":         "local",
		"workflow_id":      "demo",
		"name":             "Demo",
		"workflow_version": "2026-05-17",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": false},
		"scaffold_root":    "workflows/demo",
		"artifact_root":    "striatum/demo",
		"lanes":            map[string]any{},
		"options":          map[string]any{},
	})
	result, err := Write(repo, generated)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "created" {
		t.Fatalf("status = %#v", result["status"])
	}
	if _, err := os.Stat(filepath.Join(repo, "workflows", "demo", "workflow.json")); err != nil {
		t.Fatalf("workflow.json not created: %v", err)
	}
	if _, err := Write(repo, generated); err == nil {
		t.Fatal("second write should refuse overwrite")
	}
}

func TestTraversalAndScratchPathsRejected(t *testing.T) {
	repo := t.TempDir()
	generated := Generated{
		Files:    []map[string]any{{"path": "../evil.md", "content": "bad\n"}},
		Metadata: map[string]any{"workflow_path": "../evil.md", "scaffold_root": ".."},
	}
	if _, err := Preview(repo, generated); err == nil || !strings.Contains(err.Error(), "path must not escape") {
		t.Fatalf("traversal preview error = %v", err)
	}
	generated.Files[0]["path"] = ".striatum/workflow.json"
	if _, err := Write(repo, generated); err == nil || !strings.Contains(err.Error(), ".git/.striatum") {
		t.Fatalf("scratch write error = %v", err)
	}
}

func TestUpgradeAddPhasesFailsClosedWithoutSQLiteFallback(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "workflow.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"striatum.workflow.v1","workflow_id":"wf"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Upgrade(context.Background(), failingQueryer{}, "repo_1", repo, UpgradeOptions{Path: "workflow.json", AddPhases: true})
	if err == nil {
		t.Fatal("expected fail-closed add_phases refusal")
	}
	var genErr *Error
	if !errors.As(err, &genErr) {
		t.Fatalf("error = %T %v", err, err)
	}
	if genErr.FieldPath != "add_phases" || strings.Contains(strings.ToLower(genErr.Message), "not_implemented") {
		t.Fatalf("unexpected guard error: %#v", genErr)
	}
}

type failingQueryer struct{}

func (failingQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query should not be reached")
}

func mustGenerate(t *testing.T, spec map[string]any) Generated {
	t.Helper()
	generated, err := GenerateFromMap(spec)
	if err != nil {
		t.Fatal(err)
	}
	return generated
}
