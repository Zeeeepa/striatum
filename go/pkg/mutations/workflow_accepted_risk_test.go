package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
	"github.com/jackc/pgx/v5"
)

type acceptedRiskRunner struct {
	tx *acceptedRiskTx
}

func (r *acceptedRiskRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec")
}

func (r *acceptedRiskRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (r *acceptedRiskRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (r *acceptedRiskRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}

type acceptedRiskTx struct {
	execs      []acceptedRiskExec
	committed  bool
	rolledBack bool
}

type acceptedRiskExec struct {
	sql  string
	args []any
}

func (tx *acceptedRiskTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, acceptedRiskExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *acceptedRiskTx) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (tx *acceptedRiskTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (tx *acceptedRiskTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *acceptedRiskTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func TestWorkflowAcceptRiskPersistsDecisionLinkedFingerprintBinding(t *testing.T) {
	workflow := acceptedRiskWorkflow()
	lintPayload, err := workflowauthoring.Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	warning := lintPayload["warnings"].([]map[string]any)[0]
	fingerprint := warning["fingerprint"].(string)
	tx := &acceptedRiskTx{}
	runner := &acceptedRiskRunner{tx: tx}

	result, err := HandleWorkflowAcceptRisk(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{
			"repository_id":         "repo_1",
			"workflow_json":         workflow,
			"source_path":           "workflow.json",
			"finding_fingerprints":  []any{fingerprint},
			"decision_artifact_ref": "docs/decisions/D001.md",
			"rationale":             "accepted for fixture coverage",
			"accepted_by":           "operator",
		},
	})
	if err != nil {
		t.Fatalf("HandleWorkflowAcceptRisk: %v", err)
	}
	if result["count"] != 1 || result["accepted_risk_binding"] != "workflow_fingerprint" {
		t.Fatalf("result = %#v", result)
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
	if len(tx.execs) != 1 || !strings.Contains(tx.execs[0].sql, "INSERT INTO striatumd.workflow_accepted_risks") {
		t.Fatalf("execs = %#v", tx.execs)
	}
	args := tx.execs[0].args
	if args[0] != "repo_1" || args[10] != "docs/decisions/D001.md" || args[12] != "accepted for fixture coverage" {
		t.Fatalf("insert args = %#v", args)
	}
}

func TestWorkflowAcceptRiskRefusesStaleFingerprint(t *testing.T) {
	tx := &acceptedRiskTx{}
	runner := &acceptedRiskRunner{tx: tx}

	_, err := HandleWorkflowAcceptRisk(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{
			"repository_id":         "repo_1",
			"workflow_json":         acceptedRiskWorkflow(),
			"finding_fingerprints":  []any{"missing-fingerprint"},
			"decision_artifact_ref": "docs/decisions/D001.md",
			"rationale":             "accepted for fixture coverage",
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("error = %v, want invalid_transition", err)
	}
	if len(tx.execs) != 0 {
		t.Fatalf("stale fingerprint wrote rows: %#v", tx.execs)
	}
}

func acceptedRiskWorkflow() map[string]any {
	return map[string]any{
		"schema_version":   "striatum.workflow.v1",
		"workflow_id":      "accepted-risk-fixture",
		"workflow_version": "1",
		"name":             "Accepted Risk Fixture",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/accepted-risk"},
		"coordinator":      map[string]any{"role_id": "author", "lane_id": "author"},
		"lanes": map[string]any{
			"author":   map[string]any{"adapter": "process", "command": []any{"true"}, "display_model": "codex-gpt-5"},
			"reviewer": map[string]any{"adapter": "process", "command": []any{"true"}, "display_model": "codex-gpt-5.1"},
		},
		"roles":        map[string]any{"author": map[string]any{}, "reviewer": map[string]any{}},
		"context_docs": []any{},
		"parallelism":  map[string]any{"mode": "declared", "max_active_jobs": 1},
		"jobs": []any{
			map[string]any{
				"id":      "draft",
				"type":    "build",
				"role_id": "author",
				"lane_id": "author",
				"write_scope": map[string]any{
					"mode":            "repo_write",
					"repo_write":      true,
					"allowed_paths":   []any{"."},
					"forbidden_paths": []any{".striatum/"},
				},
				"expected_artifacts": []any{map[string]any{
					"logical_name": "draft",
					"kind":         "handoff",
					"path":         "docs/DRAFT.md",
				}},
			},
			map[string]any{
				"id":      "review",
				"type":    "review",
				"role_id": "reviewer",
				"lane_id": "reviewer",
				"write_scope": map[string]any{
					"mode":            "review_only_artifact",
					"allowed_paths":   []any{"docs/reviews/"},
					"forbidden_paths": []any{".striatum/"},
				},
				"expected_artifacts": []any{map[string]any{
					"logical_name": "review",
					"kind":         "finding",
					"path":         "docs/reviews/FINDING.md",
				}},
			},
		},
		"edges":  []any{map[string]any{"from": "draft", "to": "review", "on": "completed"}},
		"cycles": []any{},
	}
}
