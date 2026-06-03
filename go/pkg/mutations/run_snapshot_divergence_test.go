package mutations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
)

// divergenceTestWorkflow returns a fully-valid workflow map (LoadFile validates
// required fields, so the on-disk file the divergence check re-loads must be a
// real workflow). laneCommand lets a test produce a semantically-different edit.
func divergenceTestWorkflow(laneCommand ...string) map[string]any {
	cmd := make([]any, len(laneCommand))
	for i, c := range laneCommand {
		cmd[i] = c
	}
	return map[string]any{
		"schema_version":   workflowauthoring.SchemaV1,
		"workflow_id":      "divergence-wf",
		"workflow_version": "1",
		"name":             "Divergence WF",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/divergence"},
		"coordinator":      map[string]any{"role_id": "author", "lane_id": "codex"},
		"lanes":            map[string]any{"codex": map[string]any{"adapter": "process", "command": cmd}},
		"roles":            map[string]any{"author": map[string]any{}, "reviewer": map[string]any{}},
		"context_docs":     []any{},
		"parallelism":      map[string]any{"mode": "declared", "max_active_jobs": 1},
		"edges":            []any{map[string]any{"from": "draft", "to": "review", "on": "completed"}},
		"cycles":           []any{map[string]any{"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": 1}},
		"jobs": []any{
			map[string]any{
				"id": "draft", "type": "draft", "role_id": "author", "lane_id": "codex",
				"write_scope":        map[string]any{"mode": "repo_write", "allowed_paths": []any{"src/"}, "forbidden_paths": []any{".striatum/"}},
				"expected_artifacts": []any{map[string]any{"logical_name": "draft", "kind": "handoff", "path": "src/draft.md", "required": true}},
			},
			map[string]any{
				"id": "review", "type": "review", "role_id": "reviewer", "lane_id": "codex", "fresh_session_required": true,
				"write_scope":        map[string]any{"mode": "review_only_artifact", "allowed_paths": []any{"reviews/"}, "forbidden_paths": []any{".striatum/"}},
				"expected_artifacts": []any{map[string]any{"logical_name": "review", "kind": "finding", "path": "reviews/REVIEW.md", "required": true}},
			},
		},
	}
}

func writeWorkflowFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

// TestWorkflowSnapshotDivergence guards #115: a run is pinned to its frozen
// workflow snapshot, so on-disk edits are inert. workflowSnapshotDivergence must
// warn (naming the file + "prepare a NEW run") when the on-disk workflow's
// canonical sha differs from the snapshot's, stay silent when they match, ignore
// cosmetic-only edits, and never warn when it cannot positively determine a
// mismatch.
func TestWorkflowSnapshotDivergence(t *testing.T) {
	repoRoot := t.TempDir()
	sourcePath := filepath.Join(repoRoot, "workflow.json")
	original := divergenceTestWorkflow("claude")
	canonical, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	writeWorkflowFile(t, sourcePath, canonical)
	sum := sha256.Sum256(canonical)
	matchSha := hex.EncodeToString(sum[:])

	t.Run("no divergence when the on-disk sha matches the snapshot", func(t *testing.T) {
		if w := workflowSnapshotDivergence(repoRoot, sourcePath, matchSha); w != "" {
			t.Fatalf("unexpected divergence warning for a matching file: %q", w)
		}
	})

	t.Run("cosmetic-only edits do not trigger divergence", func(t *testing.T) {
		// Same workflow, indented + reordered bytes: LoadFile parses it back to the
		// same map, so the canonical-sha comparison treats it as unchanged.
		indented, err := json.MarshalIndent(original, "", "  ")
		if err != nil {
			t.Fatalf("marshal indent: %v", err)
		}
		writeWorkflowFile(t, sourcePath, indented)
		if w := workflowSnapshotDivergence(repoRoot, sourcePath, matchSha); w != "" {
			t.Fatalf("cosmetic edit should not diverge; got: %q", w)
		}
	})

	t.Run("a semantic edit diverges and names the file + remedy", func(t *testing.T) {
		edited, err := json.Marshal(divergenceTestWorkflow("claude", "--print"))
		if err != nil {
			t.Fatalf("marshal edited: %v", err)
		}
		writeWorkflowFile(t, sourcePath, edited)
		w := workflowSnapshotDivergence(repoRoot, sourcePath, matchSha)
		if w == "" {
			t.Fatal("expected a divergence warning after a semantic edit")
		}
		for _, want := range []string{sourcePath, "prepare a NEW run", "#115"} {
			if !strings.Contains(w, want) {
				t.Fatalf("warning must mention %q; got: %q", want, w)
			}
		}
	})

	t.Run("undeterminable inputs stay silent (never a false warning)", func(t *testing.T) {
		writeWorkflowFile(t, sourcePath, canonical) // restore a valid, matching file
		cases := [][3]string{
			{repoRoot, filepath.Join(repoRoot, "missing.json"), matchSha}, // file gone
			{repoRoot, "", matchSha},                                      // no source_path
			{repoRoot, sourcePath, ""},                                    // no snapshot sha
			{repoRoot, sourcePath, "<nil>"},                               // nullable rendered
		}
		for _, c := range cases {
			if w := workflowSnapshotDivergence(c[0], c[1], c[2]); w != "" {
				t.Fatalf("expected silence for inputs %v; got: %q", c, w)
			}
		}
	})
}
