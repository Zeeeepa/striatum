package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// #272 (RFC 0125 D192): a supervised lane that cannot enter the operator-owned
// per-job worktree could not publish, because artifact.publish reads the body
// from the worktree filesystem. The lane may now supply the body over the MCP
// envelope (body_base64); the daemon materializes it at the artifact path and
// the rest of the publish path (sha, durability) is unchanged.
func TestPublishArtifactAcceptsBodyOverEnvelope(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "publish_body", true)

	body := []byte("handoff body delivered over MCP, not the worktree filesystem\n")
	repoPath := "docs/out.txt"
	// Precondition: the lane never wrote the file into the worktree.
	if _, err := os.Stat(filepath.Join(ids.worktreeRoot, filepath.FromSlash(repoPath))); !os.IsNotExist(err) {
		t.Fatalf("precondition: %s must be absent in the worktree", repoPath)
	}

	if _, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"kind":         "handoff",
		"logical_name": "out",
		"path":         repoPath,
		"body_base64":  base64.StdEncoding.EncodeToString(body),
	})); err != nil {
		t.Fatalf("publish with body_base64: %v", err)
	}

	// The daemon materialized the body in the worktree at the artifact path.
	got, err := os.ReadFile(filepath.Join(ids.worktreeRoot, filepath.FromSlash(repoPath)))
	if err != nil {
		t.Fatalf("read materialized worktree body: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("worktree body = %q, want %q", got, body)
	}

	// The artifact row records the matching content sha (so the durability gate
	// and get_content resolve the real bytes).
	sum := sha256.Sum256(body)
	want := hex.EncodeToString(sum[:])
	row, err := oneRow(ctx, runner, `
		SELECT content_sha256 FROM striatumd.artifacts
		 WHERE repository_id = $1 AND job_id = $2 AND logical_name = 'out'
		 ORDER BY created_at DESC LIMIT 1`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("read artifact row: %v", err)
	}
	if got := fmt.Sprint(row["content_sha256"]); got != want {
		t.Fatalf("artifact content_sha256 = %s, want %s", got, want)
	}
}
