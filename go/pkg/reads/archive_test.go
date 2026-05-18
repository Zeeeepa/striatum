package reads

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSafeArchiveOutputPathRejectsEscapes(t *testing.T) {
	repo := t.TempDir()
	for _, path := range []string{"../archive", "/tmp/archive", ".striatum/archive"} {
		t.Run(path, func(t *testing.T) {
			if _, err := safeArchiveOutputPath(repo, path); err == nil {
				t.Fatalf("expected %q to be rejected", path)
			}
		})
	}
}

func TestSafeEvidenceOutputPathRejectsEscapes(t *testing.T) {
	repo := t.TempDir()
	for _, path := range []string{"../EVIDENCE.md", "/tmp/EVIDENCE.md", ".striatum/EVIDENCE.md"} {
		t.Run(path, func(t *testing.T) {
			if _, err := safeEvidenceOutputPath(repo, path); err == nil {
				t.Fatalf("expected %q to be rejected", path)
			}
		})
	}
}

func TestRenderEvidenceMarkdownIncludesHeader(t *testing.T) {
	body, err := renderEvidenceMarkdown(
		map[string]any{"run_id": "run_1", "state": "running", "branch_name": "main"},
		map[string]any{"runs": []map[string]any{{"run_id": "run_1"}}},
	)
	if err != nil {
		t.Fatalf("render evidence: %v", err)
	}
	if !strings.Contains(body, "# Striatum Evidence Export") {
		t.Fatalf("evidence header missing: %s", body)
	}
	if !strings.Contains(body, "\"run_id\": \"run_1\"") {
		t.Fatalf("evidence snapshot missing run id: %s", body)
	}
}

func TestWriteRunArchiveCreatesVerifiableManifestShape(t *testing.T) {
	repo := t.TempDir()
	out := filepath.Join(repo, "archives", "run_1")
	createdAt := time.Date(2026, 5, 17, 1, 2, 3, 0, time.UTC)

	result, err := writeRunArchive(
		out,
		repo,
		"repo_1",
		"run_1",
		map[string]any{"repository_id": "repo_1", "run_id": "run_1", "created_at": createdAt},
		map[string]any{"repository_id": "repo_1", "workflow_snapshot_id": "wf_1"},
		map[string][]map[string]any{
			"events": {
				{"repository_id": "repo_1", "run_id": "run_1", "payload_json": []byte(`{"ok":true}`)},
			},
		},
	)
	if err != nil {
		t.Fatalf("writeRunArchive: %v", err)
	}
	if result["out"] != "archives/run_1" {
		t.Fatalf("out = %v", result["out"])
	}

	body, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatalf("manifest JSON: %v", err)
	}
	if manifest["schema_version"] != archiveSchemaVersion {
		t.Fatalf("schema_version = %v", manifest["schema_version"])
	}
	rowCounts := manifest["row_counts"].(map[string]any)
	if rowCounts["events"].(float64) != 1 {
		t.Fatalf("events count = %v", rowCounts["events"])
	}
	if rowCounts["command_requests"].(float64) != 0 {
		t.Fatalf("command_requests count = %v", rowCounts["command_requests"])
	}
}
