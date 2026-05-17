package mutations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestWorktreeHandlersValidateRequiredParamsBeforeDB(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler handlerFn
		method  string
		params  map[string]any
		message string
	}{
		{
			name:    "create session",
			handler: HandleWorktreeCreate,
			method:  "worktree.create",
			params:  map[string]any{"repository_id": "repo_1", "job_id": "job_1", "lease_id": "lease_1"},
			message: "session_id must be a non-empty string",
		},
		{
			name:    "create job",
			handler: HandleWorktreeCreate,
			method:  "worktree.create",
			params:  map[string]any{"repository_id": "repo_1", "session_id": "sess_1", "lease_id": "lease_1"},
			message: "job_id must be a non-empty string",
		},
		{
			name:    "create lease",
			handler: HandleWorktreeCreate,
			method:  "worktree.create",
			params:  map[string]any{"repository_id": "repo_1", "session_id": "sess_1", "job_id": "job_1"},
			message: "lease_id must be a non-empty string",
		},
		{
			name:    "release worktree",
			handler: HandleWorktreeRelease,
			method:  "worktree.release",
			params:  map[string]any{"repository_id": "repo_1"},
			message: "worktree_id must be a non-empty string",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.handler(context.Background(), inertRunner{}, rpc.Envelope{
				SchemaVersion: rpc.SupportedEnvelopeVersion,
				RequestID:     "req_" + strings.ReplaceAll(tc.name, " ", "_"),
				Method:        tc.method,
				Params:        tc.params,
			})
			var rpcErr *rpc.Error
			if !errors.As(err, &rpcErr) {
				t.Fatalf("error = %v, want rpc error", err)
			}
			if rpcErr.Code != "schema_invalid" {
				t.Fatalf("error code = %q, want schema_invalid", rpcErr.Code)
			}
			if rpcErr.Message != tc.message {
				t.Fatalf("error message = %q, want %q", rpcErr.Message, tc.message)
			}
		})
	}
}

func TestWorktreeTargetConfinesPathToStateWorktrees(t *testing.T) {
	repoRoot := t.TempDir()
	got, err := worktreeTarget(repoRoot, ".striatum/worktrees/wt_1")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(repoRoot, ".striatum", "worktrees", "wt_1")
	if got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}

	got, err = worktreeTarget(repoRoot, filepath.Join(repoRoot, ".striatum", "worktrees", "wt_2"))
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Join(repoRoot, ".striatum", "worktrees", "wt_2")
	if got != want {
		t.Fatalf("absolute target = %q, want %q", got, want)
	}

	for _, path := range []string{
		"docs/not-a-worktree",
		".striatum/../docs/not-a-worktree",
		filepath.Join(filepath.Dir(repoRoot), "outside"),
	} {
		t.Run(path, func(t *testing.T) {
			_, err := worktreeTarget(repoRoot, path)
			var rpcErr *rpc.Error
			if !errors.As(err, &rpcErr) {
				t.Fatalf("error = %v, want rpc error", err)
			}
			if rpcErr.Code != "invalid_transition" {
				t.Fatalf("error code = %q, want invalid_transition", rpcErr.Code)
			}
		})
	}
}

func TestWorktreeTargetRejectsSymlinkedScratchComponents(t *testing.T) {
	repoRoot := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".striatum"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repoRoot, ".striatum", "worktrees")); err != nil {
		t.Fatal(err)
	}

	_, err := worktreeTarget(repoRoot, ".striatum/worktrees/wt_1")

	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %v, want rpc error", err)
	}
	if rpcErr.Code != "invalid_transition" {
		t.Fatalf("error code = %q, want invalid_transition", rpcErr.Code)
	}
	if rpcErr.Message != "worktree path must not traverse symlinks" {
		t.Fatalf("error message = %q", rpcErr.Message)
	}
}

func TestGitWorktreeErrorMessageTruncatesCommandOutput(t *testing.T) {
	longOutput := strings.Repeat("x", 250)
	message := gitWorktreeErrorMessage("git worktree add failed", gitWorktreeResult{Stderr: longOutput})
	if !strings.HasPrefix(message, "git worktree add failed: ") {
		t.Fatalf("message prefix = %q", message)
	}
	if !strings.HasSuffix(message, "...") {
		t.Fatalf("message should be truncated with ellipsis: %q", message)
	}
	if len(strings.TrimPrefix(message, "git worktree add failed: ")) != 203 {
		t.Fatalf("truncated detail length = %d", len(strings.TrimPrefix(message, "git worktree add failed: ")))
	}
}
