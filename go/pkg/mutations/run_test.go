package mutations

import (
	"os/exec"
	"strings"
	"testing"
)

// #123: gitBranchExists must correctly report whether a local branch exists.
func TestGitBranchExists(t *testing.T) {
	// Create a real git repo in a temp dir so we can assert on branch presence.
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")
	run("commit", "--allow-empty", "-m", "init")

	if !gitBranchExists(dir, "main") {
		t.Error("gitBranchExists should return true for existing branch 'main'")
	}
	if gitBranchExists(dir, "non-existent-branch") {
		t.Error("gitBranchExists should return false for branch that does not exist")
	}

	// Create another branch and confirm it is detected.
	run("branch", "feature-x")
	if !gitBranchExists(dir, "feature-x") {
		t.Error("gitBranchExists should return true for newly created branch 'feature-x'")
	}
}

// TestAutoConfirmBranchEnsuresRefWithoutCheckout exercises the git helper used
// by branch.confirm/run.prepare: when the suggested branch does not exist, the
// daemon creates the ref without moving the operator's primary checkout.
func TestAutoConfirmBranchEnsuresRefWithoutCheckout(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	currentBranch := func() string {
		t.Helper()
		cmd := exec.Command("git", "branch", "--show-current")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git branch --show-current: %v\n%s", err, out)
		}
		return string(out)
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")
	run("commit", "--allow-empty", "-m", "init")

	// Suggested branch does not exist yet: create it.
	suggested := "striatum/auto-test"
	if gitBranchExists(dir, suggested) {
		t.Fatalf("branch %q must not exist before test", suggested)
	}
	branch, created, err := gitEnsureBranchRef(dir, suggested, "")
	if err != nil {
		t.Fatalf("gitEnsureBranchRef: %v", err)
	}
	if branch != suggested {
		t.Errorf("branch = %q, want %q", branch, suggested)
	}
	if !created {
		t.Error("expected created=true for a new branch")
	}
	if !gitBranchExists(dir, suggested) {
		t.Error("branch must exist after creation")
	}
	if got := strings.TrimSpace(currentBranch()); got != "main" {
		t.Fatalf("current branch moved to %q, want main", got)
	}

	// Second call to the same branch: ref already exists, still no checkout.
	_, created2, err := gitEnsureBranchRef(dir, suggested, "")
	if err != nil {
		t.Fatalf("gitEnsureBranchRef (second call): %v", err)
	}
	if created2 {
		t.Error("second call must not re-create; expected created=false")
	}
	if got := strings.TrimSpace(currentBranch()); got != "main" {
		t.Fatalf("second call moved current branch to %q, want main", got)
	}
}

// #77: run.prepare gates review/phase_synthesis edges on a clearing verdict,
// EXCEPT edges into an adjudicator (phase_synthesis), which stay ungated so the
// adjudicator can absorb a reviewer's needs_revision dissent.
func TestEdgeRequiresClearingVerdictExemptsAdjudicatorInbound(t *testing.T) {
	wf := map[string]any{"jobs": []any{
		map[string]any{"id": "cross_exam", "type": "review"},
		map[string]any{"id": "adjudicate", "type": "phase_synthesis"},
		map[string]any{"id": "implement", "type": "build"},
		map[string]any{"id": "proposal", "type": "draft"},
	}}
	cases := []struct {
		from, to string
		want     bool
	}{
		{"cross_exam", "adjudicate", false}, // review -> adjudicator: ungated (absorbs)
		{"cross_exam", "implement", true},   // review -> build: gated
		{"adjudicate", "implement", true},   // phase_synthesis -> build: gated
		{"adjudicate", "adjudicate", false}, // phase_synthesis -> phase_synthesis: ungated
		{"proposal", "cross_exam", false},   // draft -> review: not verdict-capable source
	}
	for _, tc := range cases {
		if got := edgeRequiresClearingVerdict(wf, tc.from, tc.to); got != tc.want {
			t.Errorf("edgeRequiresClearingVerdict(%s->%s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
