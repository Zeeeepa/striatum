package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowValidateJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflow(t, dir, basicWorkflow())
	var stdout, stderr bytes.Buffer
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	exitCode := run([]string{"workflow", "validate", "--json", filepath.Base(path)}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != true {
		t.Fatalf("payload ok = %#v", payload["ok"])
	}
	data := payload["data"].(map[string]any)
	if data["workflow_id"] != "go-cli-test" {
		t.Fatalf("workflow_id = %#v", data["workflow_id"])
	}
}

func TestWorkflowValidateRefusesSameModelPairingUnlessAllowed(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflow(t, dir, sameModelWorkflow())
	var stdout, stderr bytes.Buffer
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	exitCode := run([]string{"workflow", "validate", filepath.Base(path)}, &stdout, &stderr)
	if exitCode != 8 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "same model family") {
		t.Fatalf("stderr = %s", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"workflow", "validate", "--allow-same-model-pairing", filepath.Base(path)}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("allowed exit = %d, stderr = %s", exitCode, stderr.String())
	}
}

func writeWorkflow(t *testing.T, dir string, body string) string {
	t.Helper()
	path := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func basicWorkflow() string {
	return `{
  "schema_version": "striatum.workflow.v1",
  "workflow_id": "go-cli-test",
  "workflow_version": "test",
  "name": "Go CLI Test",
  "context_docs": [],
  "coordinator": {"role_id": "coordinator", "lane_id": "codex"},
  "parallelism": {"mode": "declared", "max_active_jobs": 1},
  "branch": {"mode": "confirm", "suggested_name": "main"},
  "lanes": {"codex": {"adapter": "process", "command": ["true"], "model": "codex"}},
  "roles": {"coordinator": {"description": "Coordinator"}, "worker": {"description": "Worker"}},
  "jobs": [{
    "id": "build",
    "type": "build",
    "role_id": "worker",
    "lane_id": "codex",
    "task_prompt": {"inline": "do work"},
    "write_scope": {"mode": "repo_write", "repo_write": true, "allowed_paths": ["out/"], "forbidden_paths": []},
    "expected_artifacts": []
  }],
  "edges": [],
  "cycles": []
}`
}

func sameModelWorkflow() string {
	return `{
  "schema_version": "striatum.workflow.v1",
  "workflow_id": "go-cli-test",
  "workflow_version": "test",
  "name": "Go CLI Test",
  "context_docs": [],
  "coordinator": {"role_id": "coordinator", "lane_id": "codex"},
  "parallelism": {"mode": "declared", "max_active_jobs": 1},
  "branch": {"mode": "confirm", "suggested_name": "main"},
  "lanes": {"codex": {"adapter": "process", "command": ["true"], "model": "codex", "display_model": "Codex"}},
  "roles": {"coordinator": {"description": "Coordinator"}, "worker": {"description": "Worker"}, "reviewer": {"description": "Reviewer"}},
  "jobs": [
    {
      "id": "build",
      "type": "build",
      "role_id": "worker",
      "lane_id": "codex",
      "task_prompt": {"inline": "do work"},
      "write_scope": {"mode": "repo_write", "repo_write": true, "allowed_paths": ["out/"], "forbidden_paths": []},
      "expected_artifacts": []
    },
    {
      "id": "review",
      "type": "review",
      "role_id": "reviewer",
      "lane_id": "codex",
      "task_prompt": {"inline": "review"},
      "write_scope": {"mode": "review_only_artifact", "repo_write": false, "allowed_paths": ["reviews/"], "forbidden_paths": []},
      "expected_artifacts": []
    }
  ],
  "edges": [{"from": "build", "to": "review", "on": "completed"}],
  "cycles": []
}`
}
