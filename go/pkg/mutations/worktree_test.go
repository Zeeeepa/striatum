package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	readspkg "github.com/halbritt/striatum/go/pkg/reads"
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
		{
			name:    "anchor run",
			handler: HandleWorktreeAnchor,
			method:  "worktree.anchor",
			params:  map[string]any{"repository_id": "repo_1", "job_id": "job_1", "worktree_id": "wt_1"},
			message: "run_id must be a non-empty string",
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

func gitRun(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, repoRoot, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestAnchorWorktreeCommitStackFastForwardsRunBranchWithoutCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_git_anchor"
	jobID := "job_git_anchor"
	runBranch := "wf/git-anchor"
	worktreeID := "wt_git_anchor"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.MkdirAll(filepath.Join(worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeRoot, "docs", "out.txt"), []byte("anchored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "docs/out.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := gitRevParse(t, worktreeRoot, "HEAD")

	reachability, err := worktreeHeadReachability(ctx, repoRoot, worktreeRoot, map[string]any{
		"run_id":      runID,
		"job_id":      jobID,
		"base_branch": runBranch,
	})
	if err != nil {
		t.Fatalf("reachability before anchor: %v", err)
	}
	if reachability.Reachable {
		t.Fatalf("worktree HEAD should not be reachable before anchor: %#v", reachability)
	}

	payload, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, jobID, runBranch, map[string]any{
		"worktree_id":   worktreeID,
		"worktree_path": worktreeRel,
	}, 1)
	if err != nil {
		t.Fatalf("anchor worktree commit stack: %v", err)
	}
	if payload["anchor"] != "run_branch_ff" {
		t.Fatalf("anchor payload = %#v, want run_branch_ff", payload)
	}
	if got := gitRevParse(t, repoRoot, "refs/heads/"+runBranch); got != worktreeHead {
		t.Fatalf("run branch = %s, want worktree head %s", got, worktreeHead)
	}
	if head := gitSymbolicHead(t, repoRoot); head != "main" {
		t.Fatalf("primary HEAD moved to %q, want main", head)
	}

	reachability, err = worktreeHeadReachability(ctx, repoRoot, worktreeRoot, map[string]any{
		"run_id":      runID,
		"job_id":      jobID,
		"base_branch": runBranch,
	})
	if err != nil {
		t.Fatalf("reachability after anchor: %v", err)
	}
	if !reachability.Reachable {
		t.Fatalf("worktree HEAD should be reachable after anchor: %#v", reachability)
	}
}

// TestAnchorWorktreeCommitStackMergesDivergedFanInSibling is the #290 fix: a
// fan-in sibling whose worktree forked from an earlier run-branch tip (a
// concurrent sibling has since advanced it) used to be PINNED only and stranded
// — its work never reached the run branch, so a downstream worktree seeded from
// the run branch never saw it. The anchor now integrates the sibling's disjoint
// subtree into the run branch with a conflict-free content merge, so BOTH the
// sibling's file and the run-branch sibling's file are present at the run tip.
func TestAnchorWorktreeCommitStackMergesDivergedFanInSibling(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_git_fanin"
	jobID := "job_git_fanin"
	runBranch := "wf/git-fanin"
	worktreeID := "wt_git_fanin"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "worktree.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "worktree.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := gitRevParse(t, worktreeRoot, "HEAD")

	// A concurrent sibling advanced the run branch to a disjoint path after this
	// worktree forked, so the worktree HEAD can no longer fast-forward it.
	if err := os.WriteFile(filepath.Join(repoRoot, "mainline.txt"), []byte("mainline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "mainline.txt")
	gitRun(t, repoRoot, "commit", "-q", "-m", "sibling run branch change")
	siblingHead := gitRevParse(t, repoRoot, "HEAD")
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, siblingHead)

	payload, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, jobID, runBranch, map[string]any{
		"worktree_id":   worktreeID,
		"worktree_path": worktreeRel,
	}, 1)
	if err != nil {
		t.Fatalf("anchor diverged worktree: %v", err)
	}
	if payload["anchor"] != "run_branch_fanin_merge" {
		t.Fatalf("anchor payload = %#v, want run_branch_fanin_merge", payload)
	}
	merge, _ := payload["fanin_merge"].(map[string]any)
	if merge == nil || merge["integration"] != "merge" {
		t.Fatalf("fanin_merge payload = %#v, want integration=merge", payload["fanin_merge"])
	}
	// The provenance pin still records the exact sibling stack.
	pinRef := "refs/striatum/" + runID + "/" + jobID + "/1"
	if got := gitRevParse(t, repoRoot, pinRef); got != worktreeHead {
		t.Fatalf("pin ref = %s, want worktree head %s", got, worktreeHead)
	}
	// The run branch advanced to a merge commit whose parents are the prior tip
	// and the sibling head — and BOTH files are present at the new tip.
	runTip := gitRevParse(t, repoRoot, "refs/heads/"+runBranch)
	if runTip == siblingHead || runTip == worktreeHead {
		t.Fatalf("run branch did not advance to a merge commit: %s", runTip)
	}
	parents := strings.Fields(gitRun(t, repoRoot, "rev-list", "--parents", "-n", "1", runTip))
	if len(parents) != 3 || parents[1] != siblingHead || parents[2] != worktreeHead {
		t.Fatalf("merge parents = %v, want [%s %s %s]", parents, runTip, siblingHead, worktreeHead)
	}
	tree := gitRun(t, repoRoot, "ls-tree", "-r", "--name-only", "refs/heads/"+runBranch)
	for _, want := range []string{"worktree.txt", "mainline.txt"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("run branch tip missing %q after fan-in merge; tree:\n%s", want, tree)
		}
	}
	// The sibling HEAD is now reachable from the run branch (no longer stranded).
	reachability, err := worktreeHeadReachability(ctx, repoRoot, worktreeRoot, map[string]any{
		"run_id":      runID,
		"job_id":      jobID,
		"base_branch": runBranch,
	})
	if err != nil {
		t.Fatalf("reachability after fan-in merge: %v", err)
	}
	if !reachability.Reachable {
		t.Fatalf("fan-in sibling HEAD should be reachable from the run branch: %#v", reachability)
	}
	if head := gitSymbolicHead(t, repoRoot); head != "main" {
		t.Fatalf("primary HEAD moved to %q, want main", head)
	}
}

// TestAnchorWorktreeCommitStackFanInOverlapErrorsLoudly: when two parallel
// siblings wrote the SAME path with different content (a violation of the
// disjoint-write-scope authoring rule), the fan-in integration must surface the
// conflict loudly rather than silently pick a last writer — which would re-strand
// one sibling's work, the exact failure mode #290 removes.
func TestAnchorWorktreeCommitStackFanInOverlapErrorsLoudly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_git_overlap"
	jobID := "job_git_overlap"
	runBranch := "wf/git-overlap"
	worktreeID := "wt_git_overlap"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "shared.txt"), []byte("from worktree sibling\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "shared.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree writes shared.txt")

	// A concurrent sibling wrote the SAME path (overlapping scope) on the run branch.
	if err := os.WriteFile(filepath.Join(repoRoot, "shared.txt"), []byte("from run-branch sibling\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "shared.txt")
	gitRun(t, repoRoot, "commit", "-q", "-m", "sibling writes shared.txt")
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, gitRevParse(t, repoRoot, "HEAD"))

	_, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, jobID, runBranch, map[string]any{
		"worktree_id":   worktreeID,
		"worktree_path": worktreeRel,
	}, 1)
	if err == nil {
		t.Fatal("expected a loud conflict error when two siblings wrote overlapping paths, got nil")
	}
	if !strings.Contains(err.Error(), "overlapping paths") {
		t.Fatalf("conflict error should name overlapping paths, got: %v", err)
	}
}

// TestFilterRealFanInConflictsDropsByteIdentical proves the #327 fix: a path that
// is byte-identical between the run tip and the sibling head (an already-integrated
// sibling's output reached via the parallel-group worktree-base race) is dropped
// from the conflict set, while a path with differing content, or present on only
// one side, is kept as a genuine conflict.
func TestFilterRealFanInConflictsDropsByteIdentical(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	base := gitInit(t, repoRoot)

	gitRun(t, repoRoot, "checkout", "-q", "-b", "tipbr", base)
	if err := os.WriteFile(filepath.Join(repoRoot, "shared.txt"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "conflict.txt"), []byte("TIP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "-A")
	gitRun(t, repoRoot, "commit", "-q", "-m", "tip")
	tip := gitRevParse(t, repoRoot, "HEAD")

	gitRun(t, repoRoot, "checkout", "-q", "-b", "headbr", base)
	if err := os.WriteFile(filepath.Join(repoRoot, "shared.txt"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "conflict.txt"), []byte("HEAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "onlyhead.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "-A")
	gitRun(t, repoRoot, "commit", "-q", "-m", "head")
	head := gitRevParse(t, repoRoot, "HEAD")

	got := filterRealFanInConflicts(ctx, repoRoot, tip, head, []string{"shared.txt", "conflict.txt", "onlyhead.txt"})
	keep := map[string]bool{}
	for _, p := range got {
		keep[p] = true
	}
	if keep["shared.txt"] {
		t.Fatalf("byte-identical shared.txt must be dropped, got real=%v", got)
	}
	if !keep["conflict.txt"] || !keep["onlyhead.txt"] {
		t.Fatalf("differing/one-sided paths must be kept, got real=%v", got)
	}
	if len(got) != 2 {
		t.Fatalf("filterRealFanInConflicts = %v, want exactly [conflict.txt onlyhead.txt]", got)
	}
}

// TestFanInIntegrateNonzeroMergeTreeWithoutConflictIsHonest proves the #327
// headline fix: when merge-tree exits non-zero but yields no parseable conflict
// path (a plumbing failure, not a content overlap), the fan-in integration MUST
// surface an honest error rather than mislabeling it as a disjoint-write-scope
// violation ("rejected with 0 conflicting paths").
func TestFanInIntegrateNonzeroMergeTreeWithoutConflictIsHonest(t *testing.T) {
	prev := runGitWorktreeCommand
	defer func() { runGitWorktreeCommand = prev }()
	runGitWorktreeCommand = func(_ context.Context, _ string, args ...string) (gitWorktreeResult, error) {
		switch {
		case len(args) >= 2 && args[0] == "merge-base" && args[1] == "--is-ancestor":
			return gitWorktreeResult{ExitCode: 1}, nil // neither ancestor → force the merge path
		case len(args) >= 1 && args[0] == "merge-tree":
			return gitWorktreeResult{Stdout: "", Stderr: "fatal: simulated merge-tree failure", ExitCode: 128}, nil
		default:
			return gitWorktreeResult{ExitCode: 0}, nil
		}
	}
	_, err := fanInIntegrateRunBranch(context.Background(), "/repo", "refs/heads/run", "tipsha", "headsha", "job_x", 1)
	if err == nil {
		t.Fatal("expected an error when merge-tree fails without a parseable conflict")
	}
	msg := err.Error()
	if strings.Contains(msg, "disjoint write scope") || strings.Contains(msg, "overlapping paths") {
		t.Fatalf("a merge-tree plumbing failure must NOT be mislabeled as a write-scope violation; got: %v", err)
	}
	if !strings.Contains(msg, "no real content conflict") {
		t.Fatalf("expected an honest merge-tree-failure error, got: %v", err)
	}
}

// TestAnchorWorktreeCommitStackFanInAllSiblingsReachableAnyOrder is the core #290
// invariant: with N parallel disjoint siblings, no sibling strands under any
// completion order — every sibling's file is present at the run-branch tip and
// every sibling HEAD is reachable. Models three siblings all forked from the same
// base and anchored in sequence (only the first fast-forwards; the rest merge).
func TestAnchorWorktreeCommitStackFanInAllSiblingsReachableAnyOrder(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_git_nfanin"
	runBranch := "wf/git-nfanin"
	gitRun(t, repoRoot, "branch", runBranch, baseSHA)

	type sibling struct {
		jobID, file, wtRel, wtRoot, head string
	}
	siblings := make([]sibling, 3)
	for i := range siblings {
		jobID := fmt.Sprintf("job_fanin_%d", i)
		wtID := fmt.Sprintf("wt_fanin_%d", i)
		wtRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", wtID))
		wtRoot := filepath.Join(repoRoot, filepath.FromSlash(wtRel))
		// Every sibling forks from the SAME (base) run-branch tip, so only the first
		// to anchor fast-forwards; the others diverge and must merge.
		gitRun(t, repoRoot, "worktree", "add", "--detach", wtRoot, runBranch)
		file := fmt.Sprintf("branches/branch_%d/IDEAS.md", i)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(wtRoot, file)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wtRoot, file), []byte(fmt.Sprintf("ideas %d\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, wtRoot, "add", file)
		gitRun(t, wtRoot, "commit", "-q", "-m", fmt.Sprintf("sibling %d", i))
		siblings[i] = sibling{jobID: jobID, file: file, wtRel: wtRel, wtRoot: wtRoot, head: gitRevParse(t, wtRoot, "HEAD")}
	}

	for _, s := range siblings {
		if _, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, s.jobID, runBranch, map[string]any{
			"worktree_id":   filepath.Base(s.wtRel),
			"worktree_path": s.wtRel,
		}, 1); err != nil {
			t.Fatalf("anchor sibling %s: %v", s.jobID, err)
		}
	}

	tree := gitRun(t, repoRoot, "ls-tree", "-r", "--name-only", "refs/heads/"+runBranch)
	for _, s := range siblings {
		if !strings.Contains(tree, s.file) {
			t.Fatalf("sibling %s stranded: %q missing from run-branch tip; tree:\n%s", s.jobID, s.file, tree)
		}
		reach, err := worktreeHeadReachability(ctx, repoRoot, s.wtRoot, map[string]any{
			"run_id":      runID,
			"job_id":      s.jobID,
			"base_branch": runBranch,
		})
		if err != nil {
			t.Fatalf("reachability for %s: %v", s.jobID, err)
		}
		if !reach.Reachable {
			t.Fatalf("sibling %s HEAD not reachable from the run branch: %#v", s.jobID, reach)
		}
	}
}

func TestAnchorWorktreeCommitStackPinsWhenRunBranchMissing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_git_missing"
	jobID := "job_git_missing"
	runBranch := "wf/git-missing"
	worktreeID := "wt_git_missing"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "worktree.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "worktree.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := gitRevParse(t, worktreeRoot, "HEAD")
	gitRun(t, repoRoot, "update-ref", "-d", "refs/heads/"+runBranch)

	payload, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, jobID, runBranch, map[string]any{
		"worktree_id":   worktreeID,
		"worktree_path": worktreeRel,
	}, 1)
	if err != nil {
		t.Fatalf("anchor missing run branch: %v", err)
	}
	pinRef := "refs/striatum/" + runID + "/" + jobID + "/1"
	if payload["anchor"] != "job_pin" || payload["pin_ref"] != pinRef || payload["run_branch_missing"] != true {
		t.Fatalf("anchor payload = %#v, want missing-branch job_pin at %s", payload, pinRef)
	}
	if got := gitRevParse(t, repoRoot, pinRef); got != worktreeHead {
		t.Fatalf("pin ref = %s, want worktree head %s", got, worktreeHead)
	}
	if got := mustGitExit(t, repoRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+runBranch+"^{commit}"); got == 0 {
		t.Fatalf("run branch %q was recreated; completion should pin instead", runBranch)
	}
	if head := gitSymbolicHead(t, repoRoot); head != "main" {
		t.Fatalf("primary HEAD moved to %q, want main", head)
	}
}

// TestAnchorWorktreeCommitStackNamespacesAttemptsWithoutClobber is the core
// #215 acceptance check: anchoring attempt 2 of a job must not clobber the pin
// that anchored attempt 1. Each attempt's diverged HEAD is preserved under its
// own refs/striatum/<run>/<job>/<attempt> ref.
func TestAnchorWorktreeCommitStackNamespacesAttemptsWithoutClobber(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_attempts"
	jobID := "job_attempts"
	runBranch := "wf/attempts"
	gitRun(t, repoRoot, "branch", runBranch, baseSHA)

	// A run-branch commit the worktree heads diverge from, so each anchor pins
	// rather than fast-forwards.
	if err := os.WriteFile(filepath.Join(repoRoot, "mainline.txt"), []byte("mainline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "mainline.txt")
	gitRun(t, repoRoot, "commit", "-q", "-m", "sibling run branch change")
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, gitRevParse(t, repoRoot, "HEAD"))

	// A revision's worktree forks from the CURRENT run-branch tip (which carries
	// the prior attempt's content), so attempt 2 builds on attempt 1 — i.e. its
	// rewrite is a clean 3-way modify against attempt 1 as the merge base, not an
	// artificial add/add. Each diverged attempt is integrated into the run branch
	// (#290) AND pinned under its own attempt ref (#215).
	anchorAttempt := func(attempt int, base, content string) string {
		worktreeID := fmt.Sprintf("wt_attempt_%d", attempt)
		worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
		worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))
		gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, base)
		if err := os.WriteFile(filepath.Join(worktreeRoot, "attempt.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, worktreeRoot, "add", "attempt.txt")
		gitRun(t, worktreeRoot, "commit", "-q", "-m", "attempt "+content)
		head := gitRevParse(t, worktreeRoot, "HEAD")
		payload, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, jobID, runBranch, map[string]any{
			"worktree_id":   worktreeID,
			"worktree_path": worktreeRel,
		}, attempt)
		if err != nil {
			t.Fatalf("anchor attempt %d: %v", attempt, err)
		}
		wantRef := fmt.Sprintf("refs/striatum/%s/%s/%d", runID, jobID, attempt)
		if payload["pin_ref"] != wantRef {
			t.Fatalf("attempt %d pin_ref = %#v, want %s", attempt, payload["pin_ref"], wantRef)
		}
		gitRun(t, repoRoot, "worktree", "remove", "--force", worktreeRoot)
		return head
	}

	head1 := anchorAttempt(1, baseSHA, "one\n")
	head2 := anchorAttempt(2, head1, "two\n")
	if head1 == head2 {
		t.Fatalf("attempt heads collided: %s", head1)
	}
	// The latest revision's content is what lands on the run branch.
	if got := gitRun(t, repoRoot, "show", "refs/heads/"+runBranch+":attempt.txt"); got != "two" {
		t.Fatalf("run branch attempt.txt = %q, want the latest revision \"two\"", got)
	}

	ref1 := fmt.Sprintf("refs/striatum/%s/%s/1", runID, jobID)
	ref2 := fmt.Sprintf("refs/striatum/%s/%s/2", runID, jobID)
	if got := gitRevParse(t, repoRoot, ref1); got != head1 {
		t.Fatalf("attempt 1 pin = %s, want %s (clobbered by attempt 2?)", got, head1)
	}
	if got := gitRevParse(t, repoRoot, ref2); got != head2 {
		t.Fatalf("attempt 2 pin = %s, want %s", got, head2)
	}
}

// TestAnchorWorktreeCommitStackFallsBackToLegacyPin covers the upgrade window:
// a pre-#215 run already holds a job-only pin at refs/striatum/<run>/<job>, so
// git cannot create the attempt directory beneath it (ref D/F conflict). The
// anchor must not error or migrate; it falls back to the legacy job-only ref.
func TestAnchorWorktreeCommitStackFallsBackToLegacyPin(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_legacy"
	jobID := "job_legacy"
	runBranch := "wf/legacy"
	worktreeID := "wt_legacy"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	gitRun(t, repoRoot, "branch", runBranch, baseSHA)

	// Simulate a pre-#215 legacy job-only pin occupying the <job> ref name.
	legacyRef := "refs/striatum/" + runID + "/" + jobID
	gitRun(t, repoRoot, "update-ref", legacyRef, baseSHA)

	// Diverge the run branch so completion pins instead of fast-forwarding.
	if err := os.WriteFile(filepath.Join(repoRoot, "mainline.txt"), []byte("mainline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "mainline.txt")
	gitRun(t, repoRoot, "commit", "-q", "-m", "sibling run branch change")
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, gitRevParse(t, repoRoot, "HEAD"))

	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, baseSHA)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "attempt.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "attempt.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "attempt 2")
	worktreeHead := gitRevParse(t, worktreeRoot, "HEAD")

	payload, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, jobID, runBranch, map[string]any{
		"worktree_id":   worktreeID,
		"worktree_path": worktreeRel,
	}, 2)
	if err != nil {
		t.Fatalf("anchor with legacy pin present: %v", err)
	}
	if payload["pin_ref"] != legacyRef || payload["pin_shape"] != "legacy" {
		t.Fatalf("anchor payload = %#v, want legacy fallback at %s", payload, legacyRef)
	}
	if got := gitRevParse(t, repoRoot, legacyRef); got != worktreeHead {
		t.Fatalf("legacy pin = %s, want worktree head %s", got, worktreeHead)
	}
	// Reachability must still resolve a legacy-shaped pin.
	reachability, err := worktreeHeadReachability(ctx, repoRoot, worktreeRoot, map[string]any{
		"run_id":      runID,
		"job_id":      jobID,
		"base_branch": runBranch,
	})
	if err != nil {
		t.Fatalf("reachability with legacy pin: %v", err)
	}
	if !reachability.Reachable {
		t.Fatalf("legacy-pinned worktree HEAD should be reachable: %#v", reachability)
	}
}

// TestSweepRunPinsDeletesIntegratedRetainsDivergent covers the #214 acceptance
// checks (RFC 0117 Open Question 1): an integrated pin (reachable from the run
// branch) is deleted, a divergent pin is retained with a reason, both the legacy
// and attempt-namespaced ref shapes are handled, and a second sweep is
// idempotent (no unexpected changes).
func TestSweepRunPinsDeletesIntegratedRetainsDivergent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_sweep"
	runBranch := "wf/sweep"
	baseBranch := "main"
	gitRun(t, repoRoot, "branch", runBranch, baseSHA)

	// Build two commits in a worktree: C1 (integrated onto the run branch) and
	// C2 (a child of C1 that the run branch never advances to, so it diverges).
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", "wt_sweep"))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, baseSHA)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "c1.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "c1.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "c1")
	c1 := gitRevParse(t, worktreeRoot, "HEAD")
	if err := os.WriteFile(filepath.Join(worktreeRoot, "c2.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "c2.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "c2")
	c2 := gitRevParse(t, worktreeRoot, "HEAD")
	gitRun(t, repoRoot, "worktree", "remove", "--force", worktreeRoot)

	// Integrate C1 onto the run branch.
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, c1)

	// Legacy-shaped pin at the integrated commit (should be deleted).
	legacyRef := "refs/striatum/" + runID + "/jobA"
	gitRun(t, repoRoot, "update-ref", legacyRef, c1)
	// Attempt-namespaced pin at the divergent commit (should be retained).
	attemptRef := "refs/striatum/" + runID + "/jobB/2"
	gitRun(t, repoRoot, "update-ref", attemptRef, c2)

	deleted, retained, err := sweepRunPins(ctx, repoRoot, runID, runBranch, baseBranch)
	if err != nil {
		t.Fatalf("sweepRunPins: %v", err)
	}
	if len(deleted) != 1 || deleted[0]["ref"] != legacyRef {
		t.Fatalf("deleted = %#v, want [%s]", deleted, legacyRef)
	}
	if deleted[0]["reason"] != "reachable_from" {
		t.Fatalf("deleted reason = %#v, want reachable_from", deleted[0]["reason"])
	}
	if len(retained) != 1 || retained[0]["ref"] != attemptRef || retained[0]["reason"] != "divergent" {
		t.Fatalf("retained = %#v, want divergent [%s]", retained, attemptRef)
	}
	// The integrated pin is gone; the divergent pin survives.
	if got := mustGitExit(t, repoRoot, "rev-parse", "--verify", "--quiet", legacyRef); got == 0 {
		t.Fatalf("legacy pin %q should be deleted", legacyRef)
	}
	if got := gitRevParse(t, repoRoot, attemptRef); got != c2 {
		t.Fatalf("attempt pin = %s, want %s (divergent pin must be retained)", got, c2)
	}

	// Idempotent: a second sweep deletes nothing new and still retains the
	// divergent pin.
	deleted2, retained2, err := sweepRunPins(ctx, repoRoot, runID, runBranch, baseBranch)
	if err != nil {
		t.Fatalf("second sweepRunPins: %v", err)
	}
	if len(deleted2) != 0 {
		t.Fatalf("second sweep deleted = %#v, want none", deleted2)
	}
	if len(retained2) != 1 || retained2[0]["ref"] != attemptRef {
		t.Fatalf("second sweep retained = %#v, want [%s]", retained2, attemptRef)
	}
}

func TestWorktreeCompleteAnchorsCommitStack(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_anchor"
	jobID := "job_anchor"
	sessionID := "sess_anchor"
	leaseID := "lease_anchor"
	messageID := "msg_anchor"
	worktreeID := "wt_anchor"
	runBranch := "wf/anchor"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.MkdirAll(filepath.Join(worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeRoot, "docs", "out.txt"), []byte("anchored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "docs/out.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := gitRevParse(t, worktreeRoot, "HEAD")

	now := time.Now().UTC()
	workflowArg, err := db.JSONBArg(runner, map[string]any{
		"workflow_id": "wf_anchor",
		"lanes": map[string]any{
			"codex": map[string]any{"worktree_isolation": "per_job"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "codex"})
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":       "repo_write",
		"repo_write": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	emptyJSONArg, err := db.JSONBArg(runner, []any{})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_anchor','ident_anchor',$1,$2,'repo',$3,16,'active')`,
		repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_anchor','snap_anchor','wf_anchor','sha',$1::jsonb,$2)`, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at
		) VALUES ('repo_anchor',$1,'snap_anchor',$2,'running',$3,$4,$5,'human',$5)`,
		runID, repoRoot, runBranch, baseSHA, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal, state, registered_at
		) VALUES ('repo_anchor',$1,$2,'author','codex','author-codex-1',1,'active',$3)`,
		sessionID, runID, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	intgAttest(t, ctx, runner, "repo_anchor", runID, sessionID, "codex")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key, expected_artifacts_json,
		  write_scope_json, current_message_id, created_at, started_at, current_lease_id
		) VALUES ('repo_anchor',$1,$2,'author_draft',1,'running','author',
		  $3::jsonb,'Author draft','build','idem_anchor',$4::jsonb,$5::jsonb,$6,$7,$7,$8)`,
		jobID, runID, laneArg, emptyJSONArg, writeScopeArg, messageID, now, leaseID); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_session_id, target_role_id, target_lane_id, current_lease_id,
		  created_at, updated_at
		) VALUES ('repo_anchor',$1,$2,$3,'work','acked',0,$4,'author','codex',$5,$6,$6)`,
		messageID, runID, jobID, sessionID, leaseID, now); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ('repo_anchor',$1,$2,'job',$3,$4,'active',$5,$6)`,
		leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_worktrees (
		  repository_id, worktree_id, run_id, job_id, lease_id,
		  base_branch, worktree_path, state, created_at
		) VALUES ('repo_anchor',$1,$2,$3,$4,$5,$6,'active',$7)`,
		worktreeID, runID, jobID, leaseID, runBranch, worktreeRel, now); err != nil {
		t.Fatalf("insert worktree: %v", err)
	}

	result, err := HandleCompleteWork(ctx, runner, intgEnv("repo_anchor", map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"summary":    "done",
	}))
	if err != nil {
		t.Fatalf("complete work: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("complete result = %#v", result)
	}
	if got := gitRevParse(t, repoRoot, "refs/heads/"+runBranch); got != worktreeHead {
		t.Fatalf("run branch was not fast-forwarded to worktree HEAD: got %s want %s", got, worktreeHead)
	}
	row, err := oneRow(ctx, runner, `
		SELECT payload_json
		  FROM striatumd.events
		 WHERE repository_id = 'repo_anchor'
		   AND run_id = $1
		   AND job_id = $2
		   AND event_type = 'job.commits_anchored'`,
		runID, jobID)
	if err != nil {
		t.Fatalf("missing job.commits_anchored event: %v", err)
	}
	payload := asMap(row["payload_json"])
	if payload["anchor"] != "run_branch_ff" || payload["head"] != worktreeHead {
		t.Fatalf("anchor payload = %#v, want run_branch_ff at %s", payload, worktreeHead)
	}
	if head := gitSymbolicHead(t, repoRoot); head != "main" {
		t.Fatalf("primary HEAD moved to %q, want main", head)
	}
}

func TestWorktreeAnchorRemediationAnchorsCompletedJobAfterLeaseInactive(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "anchor_remediation", true)
	worktreeHead := commitWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", "anchored after inactive lease\n")
	completeSeededWorktreeJobWithInactiveLease(t, ctx, runner, ids)

	beforeProblems := doctorProblemsForRepo(t, ctx, runner, ids.repoID)
	assertContainsString(t, beforeProblems, "worktree_head_unreachable."+ids.worktreeID)
	assertContainsString(t, beforeProblems, "job_completed_without_anchor."+ids.jobID)

	result, err := HandleWorktreeAnchor(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":      ids.runID,
		"job_id":      ids.jobID,
		"worktree_id": ids.worktreeID,
	}))
	if err != nil {
		t.Fatalf("worktree anchor: %v", err)
	}
	if result["status"] != "anchored" {
		t.Fatalf("anchor result = %#v", result)
	}
	if got := gitRevParse(t, repoRoot, "refs/heads/"+ids.runBranch); got != worktreeHead {
		t.Fatalf("run branch = %s, want anchored worktree head %s", got, worktreeHead)
	}

	afterProblems := doctorProblemsForRepo(t, ctx, runner, ids.repoID)
	assertNotContainsString(t, afterProblems, "worktree_head_unreachable."+ids.worktreeID)
	assertNotContainsString(t, afterProblems, "job_completed_without_anchor."+ids.jobID)

	row, err := oneRow(ctx, runner, `
		SELECT payload_json
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND job_id = $3
		   AND event_type = 'worktree.anchored'`,
		ids.repoID, ids.runID, ids.jobID)
	if err != nil {
		t.Fatalf("missing worktree.anchored event: %v", err)
	}
	payload := asMap(row["payload_json"])
	if payload["head"] != worktreeHead || payload["remediation"] != "worktree.anchor" {
		t.Fatalf("anchor event payload = %#v, want head %s", payload, worktreeHead)
	}
}

func TestWorktreeAnchorRefusesNonCompletedJob(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "anchor_running", true)

	_, err := HandleWorktreeAnchor(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":      ids.runID,
		"job_id":      ids.jobID,
		"worktree_id": ids.worktreeID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" || !strings.Contains(rpcErr.Message, "requires completed job") {
		t.Fatalf("worktree anchor error = %v, want invalid_transition for non-completed job", err)
	}
}

func TestWorktreeAnchorRefusesMissingWorktreePath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "anchor_missing", true)
	commitWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", "missing worktree\n")
	completeSeededWorktreeJobWithInactiveLease(t, ctx, runner, ids)
	gitRun(t, repoRoot, "worktree", "remove", "--force", ids.worktreeRoot)

	_, err := HandleWorktreeAnchor(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":      ids.runID,
		"job_id":      ids.jobID,
		"worktree_id": ids.worktreeID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" || !strings.Contains(rpcErr.Message, "path is missing on disk") {
		t.Fatalf("worktree anchor error = %v, want invalid_transition for missing path", err)
	}
}

func TestWorktreeRequiredJobRefusesPrimaryCheckoutSurfacesWithoutActiveWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "missing_wt", false)

	check := func(name string, err error) {
		t.Helper()
		var rpcErr *rpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != "worktree_required" {
			t.Fatalf("%s error = %v, want worktree_required", name, err)
		}
	}

	_, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"kind":         "handoff",
		"logical_name": "handoff",
		"path":         "docs/out.md",
	}))
	check("artifact.publish", err)

	_, err = HandleRepoWrite(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"path":       "docs/out.md",
		"content":    "body\n",
	}))
	check("repo.write", err)

	_, err = HandleProcessRun(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"command_json": []any{"true"},
	}))
	check("process.run", err)

	_, err = HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	check("work.complete", err)
}

func TestWorktreeCompletionDirtyCheckUsesActiveWorktreeNotPrimaryCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "dirty_root", true)
	if err := os.WriteFile(filepath.Join(repoRoot, "outside-root.txt"), []byte("operator dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	if err != nil {
		t.Fatalf("complete with dirty primary checkout: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("complete result = %#v", result)
	}
}

// RFC 0125 daemon-as-porter (#278/#281, D192): a published artifact whose body
// the lane wrote into the per-job worktree but could not commit itself
// (git.commit_apply refuses the detached HEAD; `git add` skips a declared path
// the repo ignores) is now made durable by the DAEMON at completion — it
// force-adds and commits the published path on the worktree HEAD — so
// work.complete SUCCEEDS rather than refusing. (Before RFC 0125 this refused
// with a durability error; the porter removes that lane burden. The body-absent
// case — the lane could not write the worktree at all, #272 — still refuses; see
// TestPorterLeavesResidualWhenBodyAbsent.)
func TestWorktreeCompletePortersUncommittedPublishedArtifact(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "artifact_uncommitted", true)
	payload := []byte("published but not committed\n")
	if err := os.MkdirAll(filepath.Join(ids.worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, "docs", "out.txt"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	seedPublishedArtifact(t, ctx, runner, ids, "art_uncommitted", "out", "docs/out.txt", payload, nil)

	result, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	if err != nil {
		t.Fatalf("complete should succeed via the daemon-porter, got: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("complete result = %#v, want completed", result)
	}
	if got := jobState(t, ctx, runner, ids.repoID, ids.jobID); got != "completed" {
		t.Fatalf("job state = %q, want completed after porter remediation", got)
	}
	// The porter made the published body durable in the worktree HEAD.
	if got := gitRun(t, ids.worktreeRoot, "show", "HEAD:docs/out.txt"); strings.TrimSpace(got) != strings.TrimSpace(string(payload)) {
		t.Fatalf("HEAD:docs/out.txt = %q, want %q (porter should have committed it)", got, payload)
	}
}

func TestWorktreeCompleteAllowsPublishedArtifactCommittedInWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "artifact_committed", true)
	payload := []byte("published and committed\n")
	if err := os.MkdirAll(filepath.Join(ids.worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, "docs", "out.txt"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, ids.worktreeRoot, "add", "docs/out.txt")
	gitRun(t, ids.worktreeRoot, "commit", "-q", "-m", "artifact")
	seedPublishedArtifact(t, ctx, runner, ids, "art_committed", "out", "docs/out.txt", payload, nil)

	result, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	if err != nil {
		t.Fatalf("complete with committed published artifact: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("complete result = %#v", result)
	}
}

func TestClosedSessionRequeueLetsFreshSessionPublishExistingWorktreeArtifact(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "closed_requeue_publish", true)
	artifactPath := "docs/recovered.md"
	payload := []byte(findingArtifactPayload("accept"))
	if err := os.MkdirAll(filepath.Join(ids.worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, filepath.FromSlash(artifactPath)), payload, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = NOW(), release_reason = 'operator_release_before_close'
		 WHERE repository_id = $1 AND lease_id = $2`, ids.repoID, ids.leaseID); err != nil {
		t.Fatalf("release old lease: %v", err)
	}
	closeResult, err := HandleCloseSession(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":  ids.sessionID,
		"reason":      "lane closed after writing artifact; requeue same attempt",
		"requeue_job": true,
	}))
	if err != nil {
		t.Fatalf("session close --requeue-job: %v", err)
	}
	if closeResult["state"] != "closed" || closeResult["requeued_job"] == nil {
		t.Fatalf("close result = %#v, want closed with requeued job", closeResult)
	}

	freshSession := "sess_fresh_" + ids.repoID
	intgSeedSessionOrdinal(t, ctx, runner, ids.repoID, ids.runID, freshSession, "author", "codex", []string{"write"}, "active", 2)
	intgAttest(t, ctx, runner, ids.repoID, ids.runID, freshSession, "codex")
	claim, err := HandleClaimNext(ctx, runner, intgEnv(ids.repoID, map[string]any{"session_id": freshSession}))
	if err != nil {
		t.Fatalf("fresh claim: %v", err)
	}
	if claim["status"] != "claimed" {
		t.Fatalf("fresh claim = %#v, want claimed", claim)
	}
	leaseRow, err := oneRow(ctx, runner, `
		SELECT lease_id
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND owner_session_id = $2 AND state = 'active'
		 LIMIT 1`, ids.repoID, freshSession)
	if err != nil {
		t.Fatalf("fresh active lease: %v", err)
	}

	published, err := HandlePublishArtifact(boundCtx(ctx, ids.repoID, freshSession), runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   freshSession,
		"job_id":       ids.jobID,
		"lease_id":     fmt.Sprint(leaseRow["lease_id"]),
		"kind":         "finding",
		"logical_name": "recovered",
		"path":         artifactPath,
	}))
	if err != nil {
		t.Fatalf("fresh publish of recovered worktree artifact: %v", err)
	}
	if published["artifact_id"] == nil {
		t.Fatalf("publish result = %#v, want artifact_id", published)
	}
}

func TestWorktreeReleaseRefusesPublishedArtifactOnlyInUncommittedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "release_artifact_uncommitted", true)
	payload := []byte("published release artifact\n")
	if err := os.MkdirAll(filepath.Join(ids.worktreeRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, "docs", "out.txt"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	seedPublishedArtifact(t, ctx, runner, ids, "art_release_uncommitted", "out", "docs/out.txt", payload, nil)
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("complete job fixture: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'completed'
		 WHERE repository_id = $2 AND lease_id = $3`, now, ids.repoID, ids.leaseID); err != nil {
		t.Fatalf("release lease fixture: %v", err)
	}

	_, err := HandleWorktreeRelease(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"worktree_id": ids.worktreeID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" ||
		!strings.Contains(rpcErr.Message, "published artifact content is not durable") {
		t.Fatalf("release error = %v, want artifact durability refusal", err)
	}
	if _, err := os.Stat(ids.worktreeRoot); err != nil {
		t.Fatalf("worktree path should remain after refused release: %v", err)
	}
}

func TestWorktreeReleaseRefusesMissingActiveWorktreePathWithoutForce(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "release_missing_path", true)
	gitRun(t, repoRoot, "worktree", "remove", "--force", ids.worktreeRoot)

	_, err := HandleWorktreeRelease(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"worktree_id": ids.worktreeID,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" ||
		!strings.Contains(rpcErr.Message, "path is missing on disk") {
		t.Fatalf("release error = %v, want missing path refusal", err)
	}
	row, err := oneRow(ctx, runner, `
		SELECT state FROM striatumd.job_worktrees
		 WHERE repository_id = $1 AND worktree_id = $2`, ids.repoID, ids.worktreeID)
	if err != nil {
		t.Fatalf("read worktree row: %v", err)
	}
	if row["state"] != "active" {
		t.Fatalf("worktree state = %#v, want active", row["state"])
	}
}

func TestWorktreeReleaseForceRefusesMissingNonTerminalWorktreePath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "release_missing_non_terminal", true)
	gitRun(t, repoRoot, "worktree", "remove", "--force", ids.worktreeRoot)

	_, err := HandleWorktreeRelease(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"worktree_id": ids.worktreeID,
		"force":       true,
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" ||
		!strings.Contains(rpcErr.Message, "requires a terminal job") {
		t.Fatalf("release error = %v, want terminal-job refusal", err)
	}
	row, err := oneRow(ctx, runner, `
		SELECT state FROM striatumd.job_worktrees
		 WHERE repository_id = $1 AND worktree_id = $2`, ids.repoID, ids.worktreeID)
	if err != nil {
		t.Fatalf("read worktree row: %v", err)
	}
	if row["state"] != "active" {
		t.Fatalf("worktree state = %#v, want active", row["state"])
	}
}

func TestWorktreeReleaseForceRemovesMissingTerminalWorktreePath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "release_missing_terminal", true)
	gitRun(t, repoRoot, "worktree", "remove", "--force", ids.worktreeRoot)
	completeSeededWorktreeJobWithInactiveLease(t, ctx, runner, ids)

	result, err := HandleWorktreeRelease(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"worktree_id": ids.worktreeID,
		"force":       true,
	}))
	if err != nil {
		t.Fatalf("HandleWorktreeRelease: %v", err)
	}
	if result["status"] != "force_released" || result["missing_on_disk"] != true {
		t.Fatalf("release result = %#v, want force_released missing_on_disk", result)
	}
	row, err := oneRow(ctx, runner, `
		SELECT state, released_at, removed_at FROM striatumd.job_worktrees
		 WHERE repository_id = $1 AND worktree_id = $2`, ids.repoID, ids.worktreeID)
	if err != nil {
		t.Fatalf("read worktree row: %v", err)
	}
	if row["state"] != "removed" || row["released_at"] == nil || row["removed_at"] == nil {
		t.Fatalf("worktree row = %#v, want removed with release timestamps", row)
	}
	event, err := oneRow(ctx, runner, `
		SELECT payload_json
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND job_id = $3
		   AND event_type = 'worktree.force_released'`, ids.repoID, ids.runID, ids.jobID)
	if err != nil {
		t.Fatalf("missing worktree.force_released event: %v", err)
	}
	payload := asMap(event["payload_json"])
	if payload["missing_on_disk"] != true || payload["worktree_id"] != ids.worktreeID || payload["reachable"] != false {
		t.Fatalf("event payload = %#v, want missing-on-disk force release", payload)
	}
	problems := doctorProblemsForRepo(t, ctx, runner, ids.repoID)
	assertNotContainsString(t, problems, "worktree_head_unreachable")
}

func TestWorktreeCompletionStillRejectsOutOfScopeDirtInsideActiveWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "dirty_worktree", true)
	if err := os.WriteFile(filepath.Join(ids.worktreeRoot, "outside-worktree.txt"), []byte("bad dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "write_scope_drift" || !strings.Contains(rpcErr.Message, "outside-worktree.txt") {
		t.Fatalf("complete error = %v, want write_scope violation for outside-worktree.txt", err)
	}
}

type worktreeRequiredFixtureIDs struct {
	repoID       string
	runID        string
	jobID        string
	sessionID    string
	leaseID      string
	messageID    string
	worktreeID   string
	worktreeRel  string
	worktreeRoot string
	runBranch    string
}

func seedPublishedArtifact(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs, artifactID, logicalName, repoPath string, payload []byte, blobKey any) {
	t.Helper()
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
		  created_at, author_line, blob_key, attempt
		) VALUES ($1,$2,$3,$4,$5,$6,'handoff',$7,$8,$9,'create',$10,NULL,$11,1)`,
		ids.repoID, artifactID, ids.runID, ids.jobID, ids.sessionID, logicalName,
		repoPath, digest, len(payload), time.Now().UTC(), blobKey); err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
}

func commitWorktreeFile(t *testing.T, worktreeRoot, relPath, body string) string {
	t.Helper()
	target := filepath.Join(worktreeRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", relPath)
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree anchor remediation")
	return gitRevParse(t, worktreeRoot, "HEAD")
}

func completeSeededWorktreeJobWithInactiveLease(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`,
		now, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("complete seeded job: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.queue_messages
		   SET state = 'completed', completed_at = $1, updated_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND message_id = $3`,
		now, ids.repoID, ids.messageID); err != nil {
		t.Fatalf("complete seeded message: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released', released_at = $1, release_reason = 'completed'
		 WHERE repository_id = $2 AND lease_id = $3`,
		now, ids.repoID, ids.leaseID); err != nil {
		t.Fatalf("release seeded lease: %v", err)
	}
}

func doctorProblemsForRepo(t *testing.T, ctx context.Context, runner db.Runner, repoID string) []string {
	t.Helper()
	report, err := readspkg.HandleDoctor(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_doctor_" + repoID,
		Method:        "doctor",
		Params:        map[string]any{"repository_id": repoID, "verbose": true},
	})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	problems, ok := report["problems"].([]string)
	if !ok {
		t.Fatalf("doctor problems = %#v, want []string", report["problems"])
	}
	return problems
}

func assertContainsString(t *testing.T, items []string, needle string) {
	t.Helper()
	for _, item := range items {
		if strings.Contains(item, needle) {
			return
		}
	}
	t.Fatalf("missing %q in %#v", needle, items)
}

func assertNotContainsString(t *testing.T, items []string, needle string) {
	t.Helper()
	for _, item := range items {
		if strings.Contains(item, needle) {
			t.Fatalf("unexpected %q in %#v", needle, items)
		}
	}
}

func seedWorktreeRequiredJob(t *testing.T, ctx context.Context, runner db.Runner, repoRoot, suffix string, withWorktree bool) worktreeRequiredFixtureIDs {
	t.Helper()
	baseSHA := gitInit(t, repoRoot)
	ids := worktreeRequiredFixtureIDs{
		repoID:      "repo_" + suffix,
		runID:       "run_" + suffix,
		jobID:       "job_" + suffix,
		sessionID:   "sess_" + suffix,
		leaseID:     "lease_" + suffix,
		messageID:   "msg_" + suffix,
		worktreeID:  "wt_" + suffix,
		runBranch:   "wf/" + suffix,
		worktreeRel: filepath.ToSlash(filepath.Join(".striatum", "worktrees", "wt_"+suffix)),
	}
	ids.worktreeRoot = filepath.Join(repoRoot, filepath.FromSlash(ids.worktreeRel))
	gitRun(t, repoRoot, "branch", ids.runBranch, baseSHA)
	if withWorktree {
		gitRun(t, repoRoot, "worktree", "add", "--detach", ids.worktreeRoot, ids.runBranch)
	}

	now := time.Now().UTC()
	workflowArg, err := db.JSONBArg(runner, map[string]any{
		"workflow_id": "wf_" + suffix,
		"lanes": map[string]any{
			"codex": map[string]any{"worktree_isolation": "per_job"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "codex"})
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":            "repo_write",
		"repo_write":      true,
		"allowed_paths":   []any{"docs/"},
		"forbidden_paths": []any{".striatum/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedArg, err := db.JSONBArg(runner, []any{})
	if err != nil {
		t.Fatal(err)
	}
	capReqArg, err := db.JSONBArg(runner, map[string]any{"process_execution": true})
	if err != nil {
		t.Fatal(err)
	}
	packetArg, err := db.JSONBArg(runner, map[string]any{"packet_id": "wp_" + suffix})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,16,'active')`,
		ids.repoID, "ident_"+suffix, repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,$3,'sha',$4::jsonb,$5)`, ids.repoID, "snap_"+suffix, "wf_"+suffix, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at
		) VALUES ($1,$2,$3,$4,'running',$5,$6,$7,'human',$7)`,
		ids.repoID, ids.runID, "snap_"+suffix, repoRoot, ids.runBranch, baseSHA, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal, state, registered_at
		) VALUES ($1,$2,$3,'author','codex','author-codex-1',1,'active',$4)`,
		ids.repoID, ids.sessionID, ids.runID, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	intgAttest(t, ctx, runner, ids.repoID, ids.runID, ids.sessionID, "codex")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  lane_selector_json, title, job_type, idempotency_key, expected_artifacts_json,
		  capability_requirements_json, write_scope_json, current_message_id,
		  created_at, started_at, current_lease_id
		) VALUES ($1,$2,$3,'author_draft',1,'running','author',
		  $4::jsonb,'Author draft','build',$5,$6::jsonb,$7::jsonb,$8::jsonb,$9,$10,$10,$11)`,
		ids.repoID, ids.jobID, ids.runID, laneArg, "idem_"+suffix, expectedArg, capReqArg, writeScopeArg, ids.messageID, now, ids.leaseID); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_session_id, target_role_id, target_lane_id, current_lease_id,
		  created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,$5,'author','codex',$6,$7,$7)`,
		ids.repoID, ids.messageID, ids.runID, ids.jobID, ids.sessionID, ids.leaseID, now); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		ids.repoID, ids.leaseID, ids.runID, ids.jobID, ids.sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.work_packets (
		  repository_id, packet_id, run_id, job_id, message_id, lease_id,
		  session_id, packet_json, packet_sha256, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,'packet-sha',$9)`,
		ids.repoID, "wp_"+suffix, ids.runID, ids.jobID, ids.messageID, ids.leaseID, ids.sessionID, packetArg, now); err != nil {
		t.Fatalf("insert packet: %v", err)
	}
	if withWorktree {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.job_worktrees (
			  repository_id, worktree_id, run_id, job_id, lease_id,
			  base_branch, worktree_path, state, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8)`,
			ids.repoID, ids.worktreeID, ids.runID, ids.jobID, ids.leaseID, ids.runBranch, ids.worktreeRel, now); err != nil {
			t.Fatalf("insert worktree: %v", err)
		}
	}
	return ids
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

// TestWorkspaceTargetConfinesPathToStateWorkspaces is the RFC 0127 P0 path-safety
// guard for plain-dir workspaces: the resolved path must stay under
// .striatum/workspaces and reject repo-escaping or worktree-namespace paths.
func TestWorkspaceTargetConfinesPathToStateWorkspaces(t *testing.T) {
	repoRoot := t.TempDir()
	got, err := workspaceTarget(repoRoot, ".striatum/workspaces/ws_1")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(repoRoot, ".striatum", "workspaces", "ws_1")
	if got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
	for _, path := range []string{
		"docs/not-a-workspace",
		".striatum/worktrees/wt_1", // a worktree path is not a workspace path
		".striatum/../docs/escape",
		filepath.Join(filepath.Dir(repoRoot), "outside"),
	} {
		t.Run(path, func(t *testing.T) {
			_, err := workspaceTarget(repoRoot, path)
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

// TestStagePlainDirBaseContentStagesTreeWithoutGit is the RFC 0127 P0 staging
// proof (hermetic, no DB): the daemon stages the run-branch base content into a
// plain directory, returns the exact tree sha it staged, and leaves no .git
// behind — the lane sees a pure byte directory.
func TestStagePlainDirBaseContentStagesTreeWithoutGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	repoRoot := t.TempDir()
	gitInit(t, repoRoot) // seeds seed.txt
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "docs", "nested.md"), []byte("nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", ".")
	gitRun(t, repoRoot, "commit", "-q", "-m", "nested")
	runBranch := "wf/plain-dir"
	gitRun(t, repoRoot, "branch", runBranch, "HEAD")
	wantTree := gitRevParse(t, repoRoot, runBranch+"^{tree}")

	target := filepath.Join(repoRoot, ".striatum", "workspaces", "ws_stage")
	gotTree, err := stagePlainDirBaseContent(ctx, repoRoot, runBranch, target)
	if err != nil {
		t.Fatalf("stagePlainDirBaseContent: %v", err)
	}
	if gotTree != wantTree {
		t.Fatalf("base tree sha = %s, want %s", gotTree, wantTree)
	}
	if got, err := os.ReadFile(filepath.Join(target, "seed.txt")); err != nil || string(got) != "seed\n" {
		t.Fatalf("seed.txt = %q err=%v, want seed", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "docs", "nested.md")); err != nil || string(got) != "nested\n" {
		t.Fatalf("docs/nested.md = %q err=%v, want nested", got, err)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".git must not exist in a plain-dir workspace: err=%v", err)
	}
}

// TestHandleWorktreeCreateStagesPlainDirWorkspace is the RFC 0127 P0 acceptance
// test through the full handler: a job created with workspace_kind=plain_dir gets
// a .git-free directory with the base content staged and the base tree sha
// recorded in job_workspaces — and writes no job_worktrees row.
func TestHandleWorktreeCreateStagesPlainDirWorkspace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "plain_dir", false)
	wantTree := gitRevParse(t, repoRoot, ids.runBranch+"^{tree}")

	result, err := HandleWorktreeCreate(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":     ids.sessionID,
		"job_id":         ids.jobID,
		"lease_id":       ids.leaseID,
		"workspace_kind": "plain_dir",
	}))
	if err != nil {
		t.Fatalf("HandleWorktreeCreate plain_dir: %v", err)
	}
	if result["workspace_kind"] != "plain_dir" {
		t.Fatalf("workspace_kind = %#v, want plain_dir", result["workspace_kind"])
	}
	if result["base_tree_sha"] != wantTree {
		t.Fatalf("base_tree_sha = %#v, want %s", result["base_tree_sha"], wantTree)
	}
	wsPath, _ := result["workspace_path"].(string)
	if !strings.HasPrefix(wsPath, ".striatum/workspaces/") {
		t.Fatalf("workspace_path = %q, want under .striatum/workspaces/", wsPath)
	}

	abs := filepath.Join(repoRoot, filepath.FromSlash(wsPath))
	if got, err := os.ReadFile(filepath.Join(abs, "seed.txt")); err != nil || string(got) != "seed\n" {
		t.Fatalf("staged seed.txt = %q err=%v, want seed", got, err)
	}
	if _, err := os.Stat(filepath.Join(abs, ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".git must not exist in a plain-dir workspace: err=%v", err)
	}

	row, err := oneRow(ctx, runner, `
		SELECT workspace_kind, base_tree_sha, workspace_path, state
		  FROM striatumd.job_workspaces
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("job_workspaces row: %v", err)
	}
	if row["workspace_kind"] != "plain_dir" || row["base_tree_sha"] != wantTree || row["state"] != "active" {
		t.Fatalf("job_workspaces row = %#v, want plain_dir/%s/active", row, wantTree)
	}

	// The plain-dir opt-in must not touch the legacy git-worktree table.
	count, err := runner.QueryScalar(ctx, `SELECT count(*) FROM striatumd.job_worktrees WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("job_worktrees count: %v", err)
	}
	if strings.TrimSpace(count) != "0" {
		t.Fatalf("plain-dir create wrote job_worktrees rows: count=%s", count)
	}
}

// TestHandleWorktreeCreateDefaultStillCreatesGitWorktree guards that the legacy
// per-job git-worktree path is the unchanged default when workspace_kind is
// omitted (RFC 0127 P0 reversibility: no in-flight per_job run breaks).
func TestHandleWorktreeCreateDefaultStillCreatesGitWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "default_worktree", false)

	result, err := HandleWorktreeCreate(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
	}))
	if err != nil {
		t.Fatalf("HandleWorktreeCreate default: %v", err)
	}
	wtPath, _ := result["worktree_path"].(string)
	if !strings.HasPrefix(wtPath, ".striatum/worktrees/") {
		t.Fatalf("worktree_path = %q, want under .striatum/worktrees/", wtPath)
	}
	// A real git worktree has a .git pointer file at its root.
	abs := filepath.Join(repoRoot, filepath.FromSlash(wtPath))
	if _, err := os.Stat(filepath.Join(abs, ".git")); err != nil {
		t.Fatalf("default worktree must contain a .git pointer: %v", err)
	}
	count, err := runner.QueryScalar(ctx, `SELECT count(*) FROM striatumd.job_worktrees WHERE repository_id = $1 AND job_id = $2 AND state = 'active'`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("job_worktrees count: %v", err)
	}
	if strings.TrimSpace(count) != "1" {
		t.Fatalf("default create must record one active job_worktrees row: count=%s", count)
	}
}

// TestHandleWorktreeCreateRejectsUnknownWorkspaceKind keeps the opt-in selector a
// closed enum.
func TestHandleWorktreeCreateRejectsUnknownWorkspaceKind(t *testing.T) {
	_, err := HandleWorktreeCreate(context.Background(), inertRunner{}, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_bad_workspace_kind",
		Method:        "worktree.create",
		Params: map[string]any{
			"repository_id":  "repo_1",
			"session_id":     "sess_1",
			"job_id":         "job_1",
			"lease_id":       "lease_1",
			"workspace_kind": "overlayfs",
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("error = %v, want schema_invalid for unknown workspace_kind", err)
	}
}
