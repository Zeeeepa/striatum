package workflowgenerate

import "testing"

// #287: a generated code_change workflow opts its repo-write jobs into
// source-change publishing (write_scope.publish_source_changes=true) so the
// lane's edits land on the run branch; review-only jobs are not opted in. The
// sibling `review` shape (same draft/apply skeleton, artifact intent) does NOT
// opt in.
func TestGenerateCodeChangeEnablesSourcePublish(t *testing.T) {
	gen := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "code_change",
		"lane_set":         "single_agent",
		"workflow_id":      "t287-cc",
		"name":             "t287-cc",
		"workflow_version": "2026-06-14",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/t287-cc", "allow_dirty": false},
		"scaffold_root":    ".striatum/scratch/t287-cc",
		"artifact_root":    "docs/operator/artifacts/t287-cc",
		"lanes":            map[string]any{"agent": map[string]any{"command": []any{"claude"}}},
		"options":          map[string]any{},
		"lane_modifiers":   []any{},
		"context_docs":     []any{},
	})
	repoWrite, optedIn, reviewOptedIn := 0, 0, 0
	ccJobs, _ := gen.Workflow["jobs"].([]map[string]any)
	for _, job := range ccJobs {
		scope := mapFrom(job["write_scope"])
		if scope["repo_write"] == true {
			repoWrite++
			if scope["publish_source_changes"] == true {
				optedIn++
			}
		} else if scope["publish_source_changes"] == true {
			reviewOptedIn++
		}
	}
	if repoWrite == 0 || optedIn != repoWrite {
		t.Fatalf("code_change repo-write jobs opted in = %d/%d, want all", optedIn, repoWrite)
	}
	if reviewOptedIn != 0 {
		t.Fatalf("review-only jobs must not opt into source publish; got %d", reviewOptedIn)
	}

	// The `review` shape shares the skeleton but must NOT opt in (artifact intent).
	rev := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "review",
		"lane_set":         "single_agent",
		"workflow_id":      "t287-rev",
		"name":             "t287-rev",
		"workflow_version": "2026-06-14",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/t287-rev", "allow_dirty": false},
		"scaffold_root":    ".striatum/scratch/t287-rev",
		"artifact_root":    "docs/operator/artifacts/t287-rev",
		"lanes":            map[string]any{"agent": map[string]any{"command": []any{"claude"}}},
		"options":          map[string]any{},
		"lane_modifiers":   []any{},
		"context_docs":     []any{},
	})
	revJobs, _ := rev.Workflow["jobs"].([]map[string]any)
	for _, job := range revJobs {
		scope := mapFrom(job["write_scope"])
		if scope["publish_source_changes"] == true {
			t.Fatalf("review shape must not opt into source publish")
		}
	}
}
