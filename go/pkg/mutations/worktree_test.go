package mutations

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

// gitInit builds a one-commit git repo and returns its root + initial HEAD sha.
func gitInit(t *testing.T, repoRoot string) string {
	t.Helper()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "seed")
	return run("rev-parse", "HEAD")
}

func gitRevParse(t *testing.T, repoRoot, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v\n%s", ref, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitSymbolicHead(t *testing.T, repoRoot string) string {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git symbolic-ref HEAD: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestWorktreeCreateCreatesMissingConfirmedBranchRef is the #183 regression:
// when a run's confirmed branch was recorded (records_only `branch confirm`)
// but the git ref never created, worktree create must mint the ref at the
// recorded base (or HEAD) and then succeed — without moving the operator's
// primary checkout HEAD off `main` (never `git checkout -b`).
func TestWorktreeCreateCreatesMissingConfirmedBranchRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)

	branch := "wf/feature-x"
	// Precondition: the confirmed branch ref does not yet exist.
	if got := mustGitExit(t, repoRoot, "rev-parse", "--verify", "--quiet", branch+"^{commit}"); got == 0 {
		t.Fatalf("precondition: branch %q already exists", branch)
	}

	if err := ensureWorktreeBaseBranchRef(context.Background(), repoRoot, branch, ""); err != nil {
		t.Fatalf("ensureWorktreeBaseBranchRef: %v", err)
	}

	// The ref now exists at the base (HEAD fallback).
	if got := gitRevParse(t, repoRoot, "refs/heads/"+branch); got != baseSHA {
		t.Fatalf("new branch %q = %s, want base %s", branch, got, baseSHA)
	}
	// The operator's primary checkout HEAD is untouched: still on main.
	if head := gitSymbolicHead(t, repoRoot); head != "main" {
		t.Fatalf("primary HEAD moved to %q, want main (must never checkout -b)", head)
	}

	// And the detached worktree add against that confirmed branch now succeeds.
	target := filepath.Join(repoRoot, ".striatum", "worktrees", "wt_183")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := runGitWorktreeCommand(context.Background(), repoRoot, "worktree", "add", "--detach", target, branch)
	if err != nil {
		t.Fatalf("worktree add: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("worktree add exit=%d stderr=%s", res.ExitCode, res.Stderr)
	}
	if head := gitSymbolicHead(t, repoRoot); head != "main" {
		t.Fatalf("primary HEAD moved to %q after worktree add, want main", head)
	}
}

// TestEnsureWorktreeBaseBranchRefIsNoopWhenRefExists guards that an already
// resolvable confirmed branch is left exactly as-is (no second ref, no move).
func TestEnsureWorktreeBaseBranchRefIsNoopWhenRefExists(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)

	branch := "wf/already-there"
	if res, err := runGitWorktreeCommand(context.Background(), repoRoot, "branch", branch, "HEAD"); err != nil || res.ExitCode != 0 {
		t.Fatalf("seed branch: err=%v exit=%d stderr=%s", err, res.ExitCode, res.Stderr)
	}

	if err := ensureWorktreeBaseBranchRef(context.Background(), repoRoot, branch, ""); err != nil {
		t.Fatalf("ensureWorktreeBaseBranchRef on existing ref: %v", err)
	}
	if got := gitRevParse(t, repoRoot, "refs/heads/"+branch); got != baseSHA {
		t.Fatalf("existing branch %q changed to %s, want %s", branch, got, baseSHA)
	}
}

func mustGitExit(t *testing.T, repoRoot string, args ...string) int {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	t.Fatalf("git %v: %v", args, err)
	return -1
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

// TestEnsureWorktreeBaseBranchRefToleratesLostCreateRace guards the concurrent
// worktree-create race: two creates for the same run can both probe the missing
// ref before either runs `git branch`; the loser's create fails "already
// exists" but the ref now resolves, so the goal is met and the create must not
// refuse. A create that fails while the ref is still unresolvable keeps the
// refusal.
func TestEnsureWorktreeBaseBranchRefToleratesLostCreateRace(t *testing.T) {
	prev := runGitWorktreeCommand
	defer func() { runGitWorktreeCommand = prev }()

	probes := 0
	runGitWorktreeCommand = func(_ context.Context, _ string, args ...string) (gitWorktreeResult, error) {
		switch args[0] {
		case "rev-parse":
			probes++
			if probes == 1 {
				// First probe: ref missing.
				return gitWorktreeResult{ExitCode: 1}, nil
			}
			// Re-verify after the lost create: a sibling created the ref.
			return gitWorktreeResult{ExitCode: 0}, nil
		case "branch":
			return gitWorktreeResult{ExitCode: 128, Stderr: "fatal: a branch named 'wf/raced' already exists"}, nil
		default:
			t.Fatalf("unexpected git command %v", args)
			return gitWorktreeResult{}, nil
		}
	}
	if err := ensureWorktreeBaseBranchRef(context.Background(), "/repo", "wf/raced", ""); err != nil {
		t.Fatalf("lost create race must not refuse: %v", err)
	}

	runGitWorktreeCommand = func(_ context.Context, _ string, args ...string) (gitWorktreeResult, error) {
		switch args[0] {
		case "rev-parse":
			// Ref never resolves: missing before and after the failed create.
			return gitWorktreeResult{ExitCode: 1}, nil
		case "branch":
			return gitWorktreeResult{ExitCode: 128, Stderr: "fatal: not a valid object name: 'nope'"}, nil
		default:
			t.Fatalf("unexpected git command %v", args)
			return gitWorktreeResult{}, nil
		}
	}
	if err := ensureWorktreeBaseBranchRef(context.Background(), "/repo", "wf/raced", "nope"); err == nil {
		t.Fatal("real branch-create failure must still refuse")
	}
}
