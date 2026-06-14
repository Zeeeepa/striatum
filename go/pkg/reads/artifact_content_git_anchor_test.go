package reads

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// #275: when the working-tree copy of an artifact body is absent (the operator's
// checkout is on a different branch than the run branch the artifact was
// published to), getContentFromGitAnchor must still resolve the body from the
// durable run-branch git anchor and verify its content sha — so a live workflow
// input stays retrievable without ad hoc `git show <branch>:<path>` probing.
func TestGetContentFromGitAnchorResolvesBodyFromRunBranch(t *testing.T) {
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot) // creates `main` with seed.txt

	// Publish the artifact on a separate run branch, then leave the working tree on main.
	readsGitRun(t, repoRoot, "checkout", "-q", "-b", "run-branch")
	body := []byte("# synthesis\nreconstructable body\n")
	repoPath := "artifacts/synth.md"
	if err := os.MkdirAll(filepath.Join(repoRoot, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, repoPath), body, 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", ".")
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "publish synthesis")

	// Back to main, where artifacts/synth.md does NOT exist on disk (the #275 shape).
	readsGitRun(t, repoRoot, "checkout", "-q", "main")
	if _, err := os.Stat(filepath.Join(repoRoot, repoPath)); !os.IsNotExist(err) {
		t.Fatalf("expected %s absent on the main working tree; stat err = %v", repoPath, err)
	}

	sum := sha256.Sum256(body)
	contentSha := hex.EncodeToString(sum[:])
	row := map[string]any{
		"repo_root":   repoRoot,
		"repo_path":   repoPath,
		"branch_name": "run-branch",
		"run_id":      "run_x",
		"job_id":      "job_x",
		"placement":   "git_publication",
	}

	res, ok, err := getContentFromGitAnchor(context.Background(), nil, "repo_a", "art_1", row, contentSha)
	if err != nil {
		t.Fatalf("getContentFromGitAnchor: %v", err)
	}
	if !ok {
		t.Fatalf("expected the git-anchor fallback to find the body on run-branch")
	}
	if res["source"] != "git_anchor" {
		t.Fatalf("source = %v, want git_anchor", res["source"])
	}
	if res["anchor_ref"] != "refs/heads/run-branch" {
		t.Fatalf("anchor_ref = %v, want refs/heads/run-branch", res["anchor_ref"])
	}
	gotBody, err := base64.StdEncoding.DecodeString(res["body_base64"].(string))
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if string(gotBody) != string(body) {
		t.Fatalf("body mismatch: got %q want %q", gotBody, body)
	}
	if res["sha256"] != contentSha || res["verified"] != true {
		t.Fatalf("sha/verified mismatch: %#v", res)
	}
}

// A content sha that matches no anchored body yields found=false, so the caller
// reports the missing body rather than returning a wrong one.
func TestGetContentFromGitAnchorRejectsShaMismatch(t *testing.T) {
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot)
	readsGitRun(t, repoRoot, "checkout", "-q", "-b", "run-branch")
	if err := os.MkdirAll(filepath.Join(repoRoot, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "artifacts/synth.md"), []byte("real body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", ".")
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "publish")
	readsGitRun(t, repoRoot, "checkout", "-q", "main")

	row := map[string]any{
		"repo_root":   repoRoot,
		"repo_path":   "artifacts/synth.md",
		"branch_name": "run-branch",
		"run_id":      "run_x",
		"job_id":      "job_x",
	}
	_, ok, err := getContentFromGitAnchor(context.Background(), nil, "repo_a", "art_1", row, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		t.Fatalf("getContentFromGitAnchor: %v", err)
	}
	if ok {
		t.Fatalf("expected no anchor to match a non-existent content sha")
	}
}
