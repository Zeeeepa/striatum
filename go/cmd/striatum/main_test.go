package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
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

func TestWorkflowValidateAllowsLeadingGlobalOptions(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflow(t, dir, basicWorkflow())
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	exitCode := run([]string{"--repo", dir, "--json", "workflow", "validate", filepath.Base(path), "--allow-same-model-pairing"}, &stdout, &stderr)
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
}

func TestTopLevelHelpAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exitCode := run([]string{"--help"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("help exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "usage: striatum") {
		t.Fatalf("help output = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := run([]string{"self-update"}, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("unknown command exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown command: self-update") {
		t.Fatalf("unknown command stderr = %q", stderr.String())
	}
}

func TestRetiredCLICompatibilityCommandsStayUnavailable(t *testing.T) {
	tests := [][]string{
		{"--no-daemon", "status"},
		{"daemon", "migrate"},
		{"daemon", "migrate-repo-local"},
		{"scaffold", "ddd"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := run(args, &stdout, &stderr)
			if exitCode == 0 {
				t.Fatalf("expected nonzero exit, stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
			if exitCode != 2 && exitCode != 11 && exitCode != 12 {
				t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
			}
		})
	}
}

func TestWorkflowValidateJSONErrorEnvelope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"striatum.workflow.v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
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
	if exitCode != 8 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != false {
		t.Fatalf("payload ok = %#v", payload["ok"])
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != "workflow_invalid" {
		t.Fatalf("error = %#v", errPayload)
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

func TestDaemonRouteDispatchesThroughRPC(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "daemon.sock")
	server := rpc.NewServer()
	server.Register("repo.resolve", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		return map[string]any{"repository_id": "repo_1"}, nil
	})
	server.Register("status", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		if envelope.Params["repository_id"] != "repo_1" || envelope.Params["run_id"] != "run_1" {
			t.Fatalf("params = %#v", envelope.Params)
		}
		return map[string]any{"state": "ok"}, nil
	})
	startTestRPCServer(t, socket, server)
	tokenFile := filepath.Join(t.TempDir(), "client-token")
	if err := os.WriteFile(tokenFile, []byte("tok.secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"--daemon-socket", socket, "--capability-token-file", tokenFile, "--repo", ".", "status", "--run-id", "run_1"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"state":"ok"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestDaemonErrorExitCode(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "daemon.sock")
	server := rpc.NewServer()
	server.Register("repo.resolve", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		return nil, rpc.NewError("repo_not_registered", "missing repo", nil)
	})
	startTestRPCServer(t, socket, server)
	tokenFile := filepath.Join(t.TempDir(), "client-token")
	if err := os.WriteFile(tokenFile, []byte("tok.secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"--daemon-socket", socket, "--capability-token-file", tokenFile, "status"}, &stdout, &stderr)
	if exitCode != 12 {
		t.Fatalf("exit = %d stderr=%s", exitCode, stderr.String())
	}
}

func TestUnknownCommandStillReturnsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"does-not-exist"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exit = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func startTestRPCServer(t *testing.T, socket string, server *rpc.Server) {
	t.Helper()
	listener, err := rpc.ListenUnix(socket)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				_ = server.ServeConn(context.Background(), conn, conn.RemoteAddr().String())
			}(conn)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
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

func TestWorkflowGenerateConversationPreview(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"--repo", dir, "--json", "workflow", "generate",
		"--shape", "conversation", "--option", "topic=trajectory design",
		"--workflow-id", "conv-cli-test"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json: %v; out=%s", err, stdout.String())
	}
	if payload["ok"] != true {
		t.Fatalf("ok = %#v", payload["ok"])
	}
	data := payload["data"].(map[string]any)
	if data["shape"] != "conversation" || data["workflow_id"] != "conv-cli-test" {
		t.Fatalf("data = %#v", data)
	}
	planned, ok := data["planned"].([]any)
	if !ok || len(planned) == 0 {
		t.Fatalf("expected planned files, got %#v", data["planned"])
	}
	// Preview must not write anything to the repo.
	if _, err := os.Stat(filepath.Join(dir, "docs", "operator", "workflows", "conv-cli-test", "workflow.json")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote workflow.json or stat failed: %v", err)
	}
}

func TestWorkflowTemplatesListIncludesConversation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"--json", "workflow", "templates", "list", "--kind", "shape"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json: %v; out=%s", err, stdout.String())
	}
	data := payload["data"].(map[string]any)
	templates := data["templates"].([]any)
	found := false
	for _, entry := range templates {
		if m, ok := entry.(map[string]any); ok && m["template_id"] == "conversation" {
			found = true
		}
	}
	if !found {
		t.Fatalf("conversation shape not listed in templates: %s", stdout.String())
	}
}
