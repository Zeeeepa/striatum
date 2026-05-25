package reads

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type workflowAuthoringFakeRunner struct {
	repoRoot      string
	acceptedRisks []map[string]any
}

func (r workflowAuthoringFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("workflow authoring handler must be read-only")
}

func (r workflowAuthoringFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return workflowAuthoringFakeRow{}
}

func (r workflowAuthoringFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r workflowAuthoringFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("workflow authoring handler must not open a transaction")
}

func (r workflowAuthoringFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM striatumd.repositories") {
		if len(args) != 1 || args[0] != "repo_1" {
			return dashboardAllRowsFromMaps(nil), nil
		}
		return dashboardAllRowsFromMaps([]map[string]any{{"repo_root": r.repoRoot}}), nil
	}
	if strings.Contains(sql, "FROM striatumd.workflow_accepted_risks") {
		return dashboardAllRowsFromMaps(r.acceptedRisks), nil
	}
	return nil, errors.New("unexpected query: " + sql)
}

type workflowAuthoringFakeRow struct{}

func (workflowAuthoringFakeRow) Scan(...any) error {
	return errors.New("unexpected row scan")
}

func TestWorkflowAuthoringHandlersUseRepoRelativeFiles(t *testing.T) {
	repo := t.TempDir()
	writeAuthoringWorkflow(t, filepath.Join(repo, "workflows", "demo", "workflow.json"), authoringWorkflow())
	runner := workflowAuthoringFakeRunner{repoRoot: repo}
	envelope := rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "workflow_path": "workflows/demo/workflow.json"},
	}

	validated, err := HandleWorkflowValidate(context.Background(), runner, envelope)
	if err != nil {
		t.Fatalf("HandleWorkflowValidate: %v", err)
	}
	if validated["workflow_id"] != "handler-authoring" || validated["valid"] != true {
		t.Fatalf("validated = %#v", validated)
	}

	planned, err := HandleWorkflowPlan(context.Background(), runner, envelope)
	if err != nil {
		t.Fatalf("HandleWorkflowPlan: %v", err)
	}
	summary := planned["summary"].(map[string]any)
	if summary["jobs"] != 2 || summary["claim_steps"] != 2 {
		t.Fatalf("plan summary = %#v", summary)
	}

	graph, err := HandleWorkflowGraph(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "workflow_path": "workflows/demo/workflow.json", "format": "dot"},
	})
	if err != nil {
		t.Fatalf("HandleWorkflowGraph: %v", err)
	}
	if graph["format"] != "dot" || !strings.Contains(graph["source"].(string), "digraph striatum_workflow") {
		t.Fatalf("graph = %#v", graph)
	}
}

func TestWorkflowLintReturnsFingerprintsAndAcceptedRiskAnnotations(t *testing.T) {
	repo := t.TempDir()
	workflow := authoringWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["codex"] = map[string]any{"adapter": "process", "command": []any{"true"}, "display_model": "codex-gpt-5"}
	writeAuthoringWorkflow(t, filepath.Join(repo, "workflow.json"), workflow)

	initial, err := HandleWorkflowLint(context.Background(), workflowAuthoringFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "workflow_path": "workflow.json"},
	})
	if err != nil {
		t.Fatalf("HandleWorkflowLint initial: %v", err)
	}
	warnings := initial["warnings"].([]map[string]any)
	if len(warnings) == 0 {
		t.Fatalf("expected lint warnings: %#v", initial)
	}
	first := warnings[0]
	fingerprint := first["fingerprint"].(string)
	runner := workflowAuthoringFakeRunner{
		repoRoot: repo,
		acceptedRisks: []map[string]any{{
			"accepted_risk_id":            "war_1",
			"workflow_fingerprint_sha256": initial["workflow_fingerprint_sha256"],
			"lint_rule":                   first["rule"],
			"finding_fingerprint_sha256":  fingerprint,
			"finding_json":                first,
			"decision_artifact_ref":       "docs/decisions/D001.md",
			"accepted_by":                 "operator",
			"rationale":                   "fixture",
		}},
	}

	payload, err := HandleWorkflowLint(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "workflow_path": "workflow.json"},
	})
	if err != nil {
		t.Fatalf("HandleWorkflowLint annotated: %v", err)
	}
	if payload["workflow_fingerprint_sha256"] == "" {
		t.Fatalf("missing workflow fingerprint: %#v", payload)
	}
	annotated := payload["warnings"].([]map[string]any)[0]
	if annotated["accepted"] != true {
		t.Fatalf("accepted warning was not annotated: %#v", annotated)
	}
	ids := annotated["accepted_risk_ids"].([]any)
	if len(ids) != 1 || ids[0] != "war_1" {
		t.Fatalf("accepted ids = %#v", ids)
	}
}

func TestWorkflowAuthoringHandlerRejectsInvalidWorkflowAsRPCError(t *testing.T) {
	repo := t.TempDir()
	workflow := authoringWorkflow()
	workflow["edges"] = []any{map[string]any{"from": "draft", "to": "missing", "on": "completed"}}
	writeAuthoringWorkflow(t, filepath.Join(repo, "workflow.json"), workflow)

	_, err := HandleWorkflowValidate(context.Background(), workflowAuthoringFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "workflow_path": "workflow.json"},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "workflow_error" {
		t.Fatalf("error = %v, want workflow_error", err)
	}
}

func TestWorkflowAuthoringHandlerRejectsTraversal(t *testing.T) {
	repo := t.TempDir()
	_, err := HandleWorkflowPlan(context.Background(), workflowAuthoringFakeRunner{repoRoot: repo}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "workflow_path": "../workflow.json"},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "workflow_error" || !strings.Contains(rpcErr.Message, "inside the repository") {
		t.Fatalf("error = %#v, want traversal workflow_error", err)
	}
}

func writeAuthoringWorkflow(t *testing.T, path string, workflow map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	raw, err := json.Marshal(workflow)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

func authoringWorkflow() map[string]any {
	return map[string]any{
		"schema_version":   "striatum.workflow.v1",
		"workflow_id":      "handler-authoring",
		"workflow_version": "1",
		"name":             "Handler Authoring",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/handler-authoring"},
		"coordinator":      map[string]any{"role_id": "author", "lane_id": "codex"},
		"lanes":            map[string]any{"codex": map[string]any{"adapter": "process", "command": []any{"true"}}},
		"roles":            map[string]any{"author": map[string]any{}, "reviewer": map[string]any{}},
		"context_docs":     []any{},
		"parallelism":      map[string]any{"mode": "declared", "max_active_jobs": 1},
		"edges":            []any{map[string]any{"from": "draft", "to": "review", "on": "completed"}},
		"cycles":           []any{},
		"jobs": []any{
			map[string]any{
				"id":      "draft",
				"type":    "draft",
				"role_id": "author",
				"lane_id": "codex",
				"write_scope": map[string]any{
					"mode":            "repo_write",
					"allowed_paths":   []any{"src/"},
					"forbidden_paths": []any{".striatum/"},
				},
				"expected_artifacts": []any{map[string]any{
					"logical_name": "draft",
					"kind":         "handoff",
					"path":         "src/draft.md",
					"required":     true,
				}},
			},
			map[string]any{
				"id":      "review",
				"type":    "review",
				"role_id": "reviewer",
				"lane_id": "codex",
				"write_scope": map[string]any{
					"mode":            "review_only_artifact",
					"allowed_paths":   []any{"reviews/"},
					"forbidden_paths": []any{".striatum/"},
				},
				"expected_artifacts": []any{map[string]any{
					"logical_name": "review",
					"kind":         "finding",
					"path":         "reviews/REVIEW.md",
					"required":     true,
				}},
			},
		},
	}
}
