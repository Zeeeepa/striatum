package workflowgenerate

import (
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
)

// #288 papercut A: scaffold_root (and generated scaffold targets) may live under
// `.striatum/scratch/` — operator scratch that `run prepare` snapshots — while
// every other `.striatum/` subdirectory, `.git`, and traversal stay rejected.
func TestSafeScaffoldRootAllowsStriatumScratch(t *testing.T) {
	okCases := []string{
		".striatum/scratch/t288",
		".striatum/scratch/nested/deeper",
		"docs/operator/workflows/demo",
	}
	for _, value := range okCases {
		if _, err := SafeScaffoldRoot(value, "spec.scaffold_root"); err != nil {
			t.Errorf("SafeScaffoldRoot(%q) = error %v, want allowed", value, err)
		}
	}

	rejectCases := []string{
		".striatum",
		".striatum/cache/x",
		".striatum/worktrees/x",
		".git/hooks",
		"../escape",
		"../.striatum/scratch/x",
	}
	for _, value := range rejectCases {
		if _, err := SafeScaffoldRoot(value, "spec.scaffold_root"); err == nil {
			t.Errorf("SafeScaffoldRoot(%q) = allowed, want rejected", value)
		}
	}

	// SafeRelativePath (artifact_root and durable paths) stays strict: scratch is
	// only acceptable for throwaway scaffold output, not durable artifacts.
	if _, err := SafeRelativePath(".striatum/scratch/t288", "spec.artifact_root"); err == nil {
		t.Errorf("SafeRelativePath allowed .striatum/scratch; durable paths must stay strict")
	}
}

// #288 structural fix: a generated single_agent code_change scaffold must validate
// out of the box — the supervised repo-write lane carries per-job worktree
// isolation and the workflow records the structural same-model review acceptance,
// so neither hand-edit (worktree_isolation: per_job, allow_same_model_review_pairing)
// is required of the operator.
func TestGenerateSingleAgentCodeChangeValidatesOutOfBox(t *testing.T) {
	generated := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "code_change",
		"lane_set":         "single_agent",
		"workflow_id":      "t288-single",
		"name":             "t288-single",
		"workflow_version": "2026-06-14",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/t288-single", "allow_dirty": false},
		"scaffold_root":    ".striatum/scratch/t288-single",
		"artifact_root":    "docs/operator/artifacts/t288-single",
		"lanes": map[string]any{
			"agent": map[string]any{"command": []any{"claude", "--dangerously-skip-permissions"}},
		},
		"options":        map[string]any{},
		"lane_modifiers": []any{},
		"context_docs":   []any{},
	})

	lanes := mapFrom(generated.Workflow["lanes"])
	agent := mapFrom(lanes["agent"])
	if agent["worktree_isolation"] != "per_job" {
		t.Fatalf("agent lane worktree_isolation = %#v, want per_job", agent["worktree_isolation"])
	}
	if generated.Workflow["allow_same_model_review_pairing"] != true {
		t.Fatalf("allow_same_model_review_pairing = %#v, want true", generated.Workflow["allow_same_model_review_pairing"])
	}

	// Mirror the two `workflow validate` gates an operator hits.
	if err := workflowauthoring.RefuseAutonomousSharedCheckoutRepoWrite(generated.Workflow); err != nil {
		t.Fatalf("RefuseAutonomousSharedCheckoutRepoWrite = %v, want nil", err)
	}
	if generated.Workflow["allow_same_model_review_pairing"] != true {
		// Defensive: the same-model refusal short-circuits on this flag.
		t.Fatalf("missing same-model override; validate would refuse the scaffold")
	}
}

// #288 papercut C: when a lane set's commands are missing, the generator reports
// every required lane in one error with a copy-pasteable JSON-array hint, instead
// of one round-trip per lane.
func TestCompileLanesBatchesMissingCommandErrors(t *testing.T) {
	_, err := GenerateFromMap(map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "code_change",
		"lane_set":         "author_reviewer",
		"workflow_id":      "t288-batch",
		"name":             "t288-batch",
		"workflow_version": "2026-06-14",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/t288-batch", "allow_dirty": false},
		"scaffold_root":    "docs/operator/workflows/t288-batch",
		"artifact_root":    "docs/operator/artifacts/t288-batch",
		"lanes":            map[string]any{},
		"options":          map[string]any{},
		"lane_modifiers":   []any{},
		"context_docs":     []any{},
	})
	if err == nil {
		t.Fatal("expected an error for the missing author_reviewer lane commands")
	}
	if !strings.Contains(err.Error(), "author") || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("error should list both missing lanes in one message, got %q", err.Error())
	}
	var genErr *Error
	if !errors.As(err, &genErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if !strings.Contains(genErr.Hint, "lanes.author.command") || !strings.Contains(genErr.Hint, "lanes.reviewer.command") {
		t.Fatalf("hint should show the JSON-array --option for each lane, got %q", genErr.Hint)
	}
	if !strings.Contains(genErr.Hint, "[") {
		t.Fatalf("hint should include a JSON array example, got %q", genErr.Hint)
	}
}
