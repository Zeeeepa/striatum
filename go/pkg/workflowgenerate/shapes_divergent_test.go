package workflowgenerate

import (
	"strings"
	"testing"
)

func divergentRaw(laneSet string, lanes map[string]any, modifiers []any, options map[string]any) map[string]any {
	if options == nil {
		options = map[string]any{}
	}
	if lanes == nil {
		lanes = map[string]any{}
	}
	if modifiers == nil {
		modifiers = []any{}
	}
	return map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "divergent_ideation",
		"lane_set":         laneSet,
		"workflow_id":      "di-test",
		"name":             "di-test",
		"workflow_version": "2026-06-14",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/di-test", "allow_dirty": false},
		"scaffold_root":    "docs/operator/workflows/di-test",
		"artifact_root":    "docs/operator/artifacts/di-test",
		"lanes":            lanes,
		"lane_modifiers":   modifiers,
		"options":          options,
	}
}

func jobsOf(t *testing.T, gen Generated) []map[string]any {
	t.Helper()
	jobs, ok := gen.Workflow["jobs"].([]map[string]any)
	if !ok {
		t.Fatalf("workflow jobs not []map[string]any, got %T", gen.Workflow["jobs"])
	}
	return jobs
}

func jobByID(jobs []map[string]any, id string) map[string]any {
	for _, j := range jobs {
		if j["id"] == id {
			return j
		}
	}
	return nil
}

// TestDivergentIdeationLocalGraph proves the default local fixture generates the
// diverge -> converge -> deepen -> final graph, validates, and warns that the
// single-family run degrades the cross-model signal (RFC 0129).
func TestDivergentIdeationLocalGraph(t *testing.T) {
	spec, err := SpecFromMap(divergentRaw("local", nil, nil, nil))
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	gen, err := Generate(spec)
	if err != nil {
		t.Fatalf("generate (must validate): %v", err)
	}
	if v := WorkflowSchemaVersion; gen.Workflow["schema_version"] != v {
		t.Errorf("schema_version = %v, want %v (divergent_ideation is flat v1)", gen.Workflow["schema_version"], v)
	}
	jobs := jobsOf(t, gen)
	// frame_problem + 5 branches + converge + 3 deepen + final = 11
	if len(jobs) != 11 {
		t.Fatalf("job count = %d, want 11", len(jobs))
	}
	diverge := 0
	deepen := 0
	for _, j := range jobs {
		switch j["parallel_group"] {
		case "diverge":
			diverge++
			if j["fresh_session_required"] != true {
				t.Errorf("diverge branch %v missing fresh_session_required", j["id"])
			}
		case "deepen":
			deepen++
		}
	}
	if diverge != 5 {
		t.Errorf("diverge branches = %d, want 5", diverge)
	}
	if deepen != 3 {
		t.Errorf("deepen jobs = %d, want 3", deepen)
	}
	for _, id := range []string{"frame_problem", "converge", "final_synthesis"} {
		if jobByID(jobs, id) == nil {
			t.Errorf("missing job %q", id)
		}
	}
	if conv := jobByID(jobs, "converge"); conv != nil && conv["expected_artifacts"] != nil {
		arts := conv["expected_artifacts"].([]map[string]any)
		if arts[0]["kind"] != "findings_ledger" {
			t.Errorf("converge artifact kind = %v, want findings_ledger", arts[0]["kind"])
		}
	}
	if !containsSubstr(gen.Warnings, "single model family") {
		t.Errorf("expected single-family degraded warning, got %v", gen.Warnings)
	}
}

// TestDivergentIdeationMultiModelRoundRobin proves a custom claude/codex/agy lane
// set (Opus/GPT-5.5/Gemini) spreads branches across all three models, reports
// three model families, does NOT emit the degraded-signal warning, and validates
// once worktree isolation is requested (the standard RFC 0008 autonomous
// repo-write rule).
func TestDivergentIdeationMultiModelRoundRobin(t *testing.T) {
	lanes := map[string]any{
		"claude": map[string]any{"command": []any{"claude", "--dangerously-skip-permissions"}, "display_model": "Claude Opus 4.8"},
		"codex":  map[string]any{"command": []any{"codex"}, "display_model": "GPT-5.5"},
		"agy":    map[string]any{"command": []any{"agy"}, "display_model": "Gemini"},
	}
	spec, err := SpecFromMap(divergentRaw("custom", lanes, []any{"worktree_isolated"}, nil))
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	gen, err := Generate(spec)
	if err != nil {
		t.Fatalf("generate (must validate with worktree_isolated): %v", err)
	}
	if gen.Metadata["model_families"] != 3 {
		t.Errorf("model_families = %v, want 3", gen.Metadata["model_families"])
	}
	if containsSubstr(gen.Warnings, "single model family") {
		t.Errorf("did not expect degraded-signal warning for 3-family run, got %v", gen.Warnings)
	}
	used := map[string]struct{}{}
	for _, j := range jobsOf(t, gen) {
		if j["parallel_group"] == "diverge" {
			used[j["lane_id"].(string)] = struct{}{}
		}
	}
	for _, lane := range []string{"claude", "codex", "agy"} {
		if _, ok := used[lane]; !ok {
			t.Errorf("expected branches on lane %q (round-robin), used=%v", lane, used)
		}
	}
}

func TestDivergentIdeationOptionBounds(t *testing.T) {
	cases := []struct {
		name    string
		options map[string]any
	}{
		{"branch_count too low", map[string]any{"branch_count": 1}},
		{"branch_count too high", map[string]any{"branch_count": 9}},
		{"deepen_count too high", map[string]any{"deepen_count": 6}},
		{"deepen exceeds branches", map[string]any{"branch_count": 2, "deepen_count": 3}},
		{"bad problem_shape", map[string]any{"problem_shape": "weird"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := SpecFromMap(divergentRaw("local", nil, nil, tc.options))
			if err != nil {
				t.Fatalf("spec: %v", err)
			}
			if _, err := Generate(spec); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func containsSubstr(values []string, want string) bool {
	for _, v := range values {
		if strings.Contains(v, want) {
			return true
		}
	}
	return false
}
