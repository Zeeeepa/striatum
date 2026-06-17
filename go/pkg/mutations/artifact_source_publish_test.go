package mutations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// #287 unit: collectInScopeAuthoredPaths returns the in-scope, attempt-authored
// changed paths (the complement of writeScopeViolationsSinceBaseline) — excluding
// out-of-scope, forbidden, and sibling-ignored paths. With a nil baseline (a
// fresh per-job worktree) every in-scope changed path is the attempt's write.
func TestCollectInScopeAuthoredPaths(t *testing.T) {
	current := []gitPathSnapshot{
		{Path: "go/pkg/feature.go", Hash: "h1"},                  // in scope, new
		{Path: "go/pkg/util.go", Hash: "h2"},                     // in scope, new
		{Path: "docs/notes.md", Hash: "h3"},                      // OUT of scope
		{Path: "go/pkg/secret.go", Hash: "h4"},                   // in allowed but forbidden child
		{Path: "go/pkg/sibling.md", Hash: "h5", Untracked: true}, // sibling-published
	}
	allowed := []string{"go/pkg/"}
	forbidden := []string{"go/pkg/secret.go"}
	ignored := map[string]bool{"go/pkg/sibling.md": true}

	got := collectInScopeAuthoredPaths(current, nil, allowed, forbidden, ignored)
	want := map[string]bool{"go/pkg/feature.go": true, "go/pkg/util.go": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want exactly %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("unexpected path %q in %v (out-of-scope/forbidden/ignored must be excluded)", p, got)
		}
	}

	// A pre-existing baseline-dirty path the attempt left unchanged is not authored.
	baseline := map[string]string{"go/pkg/feature.go": "h1"}
	got2 := collectInScopeAuthoredPaths(current, baseline, allowed, forbidden, ignored)
	for _, p := range got2 {
		if p == "go/pkg/feature.go" {
			t.Fatalf("feature.go unchanged from baseline must not be attributed to the attempt; got %v", got2)
		}
	}
}

// #287 integration: a per-job-isolated repo-write job that opts in via
// write_scope.publish_source_changes=true gets its in-scope source edits
// committed to the worktree HEAD and (after anchoring) onto the run branch — even
// though they were never declared as expected_artifacts. Out-of-scope worktree
// files are NOT committed.
func TestPublishWorktreeSourceChangesLandsOnRunBranch(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "src_publish", true)

	// Opt the job in.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET write_scope_json = jsonb_set(write_scope_json, '{publish_source_changes}', 'true')
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("opt-in publish_source_changes: %v", err)
	}

	// The lane wrote an in-scope SOURCE edit (allowed_paths=docs/) into the
	// worktree — never declared as an artifact — plus an out-of-scope file.
	inScope := "docs/feature_impl.txt"
	srcBody := "the lane's actual implementation edit\n"
	writeWorktreeFile(t, ids.worktreeRoot, inScope, srcBody)
	outOfScope := "elsewhere/stray.txt"
	writeWorktreeFile(t, ids.worktreeRoot, outOfScope, "out of scope; must not be published")

	job, err := rowByID(ctx, runner, ids.repoID, "jobs", "job_id", ids.jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}

	paths, err := publishWorktreeSourceChanges(ctx, runner, ids.repoID, job)
	if err != nil {
		t.Fatalf("publishWorktreeSourceChanges: %v", err)
	}
	if len(paths) != 1 || paths[0] != inScope {
		t.Fatalf("published paths = %v, want exactly [%s] (out-of-scope excluded)", paths, inScope)
	}

	// The in-scope edit is reachable from the worktree HEAD.
	if got := gitRun(t, ids.worktreeRoot, "show", "HEAD:"+inScope); strings.TrimSpace(got) != strings.TrimSpace(srcBody) {
		t.Fatalf("HEAD:%s = %q, want %q", inScope, got, srcBody)
	}
	// The out-of-scope file was NOT committed.
	if out := gitRunAllowFail(t, ids.worktreeRoot, "cat-file", "-e", "HEAD:"+outOfScope); out {
		t.Fatalf("out-of-scope file %s was committed; it must not be published", outOfScope)
	}

	// After anchoring, the source edit lands on the run branch.
	if _, err := anchorActiveWorktreeForJob(ctx, runner, ids.repoID, job); err != nil {
		t.Fatalf("anchorActiveWorktreeForJob: %v", err)
	}
	if got := gitRun(t, repoRoot, "show", ids.runBranch+":"+inScope); strings.TrimSpace(got) != strings.TrimSpace(srcBody) {
		t.Fatalf("run branch %s:%s = %q, want %q (source edit must reach the run branch)", ids.runBranch, inScope, got, srcBody)
	}
}

// #326 integration: in-scope source publish is the DEFAULT for a bounded
// repo-write scope (no opt-in flag). A multi-file slice's undeclared in-scope
// edits — including a MODIFICATION to a pre-existing tracked file — land on the
// run branch; expected_artifacts are presence assertions, not an allowlist.
// (Regression of #297: previously these stranded in the per-job worktree.)
func TestSourceChangesPublishedByDefault(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "src_default", true)

	// A pre-existing tracked in-scope file: commit it on the worktree HEAD, then
	// MODIFY it — the new content must reach the run branch (not present-but-stale).
	preExisting := "docs/preexisting.txt"
	writeWorktreeFile(t, ids.worktreeRoot, preExisting, "old content\n")
	gitRun(t, ids.worktreeRoot, "add", preExisting)
	gitRun(t, ids.worktreeRoot, "commit", "-q", "-m", "seed pre-existing tracked file")
	writeWorktreeFile(t, ids.worktreeRoot, preExisting, "new content\n")
	// An undeclared in-scope NEW file (a test/migration the slice wrote).
	undeclared := "docs/new_impl.txt"
	writeWorktreeFile(t, ids.worktreeRoot, undeclared, "undeclared but in-scope\n")
	// Out-of-scope file must NOT be published.
	writeWorktreeFile(t, ids.worktreeRoot, "elsewhere/stray.txt", "out of scope")

	job, err := rowByID(ctx, runner, ids.repoID, "jobs", "job_id", ids.jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	paths, err := publishWorktreeSourceChanges(ctx, runner, ids.repoID, job)
	if err != nil {
		t.Fatalf("publishWorktreeSourceChanges: %v", err)
	}
	got := map[string]bool{}
	for _, p := range paths {
		got[p] = true
	}
	if !got[undeclared] || !got[preExisting] {
		t.Fatalf("published %v; want both the undeclared new file and the modified pre-existing file (default-on)", paths)
	}
	if got["elsewhere/stray.txt"] {
		t.Fatalf("out-of-scope file was published: %v", paths)
	}
	if _, err := anchorActiveWorktreeForJob(ctx, runner, ids.repoID, job); err != nil {
		t.Fatalf("anchor: %v", err)
	}
	if body := gitRun(t, repoRoot, "show", ids.runBranch+":"+preExisting); strings.TrimSpace(body) != "new content" {
		t.Fatalf("run branch %s = %q, want \"new content\" (pre-existing-file edit must not be dropped/stale)", preExisting, body)
	}
	if body := gitRun(t, repoRoot, "show", ids.runBranch+":"+undeclared); strings.TrimSpace(body) != "undeclared but in-scope" {
		t.Fatalf("undeclared in-scope file did not reach the run branch: %q", body)
	}
}

// #326: a job may still explicitly opt OUT (publish_source_changes:false) for
// deliberately artifact-only publication.
func TestSourceChangesNotPublishedWhenOptedOut(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "src_optout", true)

	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET write_scope_json = jsonb_set(write_scope_json, '{publish_source_changes}', 'false')
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("opt-out publish_source_changes: %v", err)
	}
	writeWorktreeFile(t, ids.worktreeRoot, "docs/impl.txt", "uncommitted source edit")

	job, err := rowByID(ctx, runner, ids.repoID, "jobs", "job_id", ids.jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	paths, err := publishWorktreeSourceChanges(ctx, runner, ids.repoID, job)
	if err != nil {
		t.Fatalf("publishWorktreeSourceChanges: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("published %v despite explicit opt-out; want nothing", paths)
	}
	if gitRunAllowFail(t, ids.worktreeRoot, "cat-file", "-e", "HEAD:docs/impl.txt") {
		t.Fatalf("source edit committed despite publish_source_changes:false")
	}
}

func writeWorktreeFile(t *testing.T, worktreeRoot, relPath, body string) {
	t.Helper()
	abs := filepath.Join(worktreeRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitRunAllowFail runs a git command and reports whether it succeeded (exit 0).
func gitRunAllowFail(t *testing.T, repoRoot string, args ...string) bool {
	t.Helper()
	full := append([]string{"-C", repoRoot}, args...)
	cmd := exec.Command("git", full...)
	return cmd.Run() == nil
}
