package reads

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestArtifactGetContentValidatesParams(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		code   string
	}{
		{
			name:   "missing repository_id",
			params: map[string]any{},
			code:   "repo_not_registered",
		},
		{
			name:   "missing artifact_id",
			params: map[string]any{"repository_id": "repo_1"},
			code:   "schema_invalid",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := HandleArtifactGetContent(context.Background(), nil, rpc.Envelope{Params: c.params})
			if err == nil {
				t.Fatalf("expected error code %q, got nil", c.code)
			}
			rpcErr, ok := err.(*rpc.Error)
			if !ok {
				t.Fatalf("expected *rpc.Error, got %T: %v", err, err)
			}
			if rpcErr.Code != c.code {
				t.Fatalf("expected code %q, got %q", c.code, rpcErr.Code)
			}
		})
	}
}

func TestArtifactListForRunValidatesParams(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		code   string
	}{
		{
			name:   "missing repository_id",
			params: map[string]any{},
			code:   "repo_not_registered",
		},
		{
			name:   "missing run_id",
			params: map[string]any{"repository_id": "repo_1"},
			code:   "schema_invalid",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := HandleArtifactListForRun(context.Background(), nil, rpc.Envelope{Params: c.params})
			if err == nil {
				t.Fatalf("expected error code %q, got nil", c.code)
			}
			rpcErr, ok := err.(*rpc.Error)
			if !ok {
				t.Fatalf("expected *rpc.Error, got %T: %v", err, err)
			}
			if rpcErr.Code != c.code {
				t.Fatalf("expected code %q, got %q", c.code, rpcErr.Code)
			}
		})
	}
}

func TestArtifactGetContentFallsBackToRunBranchHistory(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot)
	runBranch := "wf/revision-run-branch"
	readsGitRun(t, repoRoot, "checkout", "-q", "-b", runBranch)
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifactPath := "docs/FINDING.md"
	originalBody := "attempt 1 finding body\n"
	if err := os.WriteFile(filepath.Join(repoRoot, filepath.FromSlash(artifactPath)), []byte(originalBody), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", artifactPath)
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "publish attempt 1 finding")
	if err := os.WriteFile(filepath.Join(repoRoot, filepath.FromSlash(artifactPath)), []byte("attempt 2 revised finding body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", artifactPath)
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "publish attempt 2 finding")
	readsGitRun(t, repoRoot, "checkout", "-q", "main")

	contentSHA := testSHA256(originalBody)
	row := artifactAnchorRow(repoRoot, "art_run_history", "run_history", "job_history", runBranch, artifactPath, contentSHA)
	result, found, err := getContentFromGitAnchor(context.Background(), nil, "repo_anchor", "art_run_history", row, contentSHA)
	if err != nil {
		t.Fatalf("getContentFromGitAnchor: %v", err)
	}
	if !found {
		t.Fatal("expected artifact content to be found in run-branch history")
	}
	body, err := base64.StdEncoding.DecodeString(result["body_base64"].(string))
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if string(body) != originalBody {
		t.Fatalf("body = %q, want %q", body, originalBody)
	}
	if result["source"] != "git_anchor" || result["verified"] != true {
		t.Fatalf("result = %#v, want verified git_anchor", result)
	}
}
