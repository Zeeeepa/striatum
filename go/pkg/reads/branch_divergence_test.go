package reads

import (
	"strings"
	"testing"
)

// TestBranchDivergenceWarnsWhenCheckoutDiffers covers #71: a run whose stored
// branch_name (branch.mode=auto metadata) differs from the target repository's
// actual checkout branch gets an advisory branch_divergence warning naming both
// branches. The git repo is checked out on "main"; the run claims "feature/x".
func TestBranchDivergenceWarnsWhenCheckoutDiffers(t *testing.T) {
	repo := initGitRepo(t) // checked out on "main"
	row := map[string]any{
		"run_id":      "run_a",
		"state":       "running",
		"branch_name": "feature/x",
		"repo_root":   repo,
	}
	decorateBranchDivergence(row)

	warn, ok := row["branch_divergence"].(map[string]any)
	if !ok {
		t.Fatalf("expected branch_divergence warning, got %#v", row["branch_divergence"])
	}
	if warn["expected_branch"] != "feature/x" {
		t.Fatalf("expected_branch = %#v, want feature/x", warn["expected_branch"])
	}
	if warn["actual_branch"] != "main" {
		t.Fatalf("actual_branch = %#v, want main", warn["actual_branch"])
	}
	msg, _ := warn["warning"].(string)
	if !strings.Contains(msg, "feature/x") || !strings.Contains(msg, "main") {
		t.Fatalf("warning must name both branches; got %q", msg)
	}
}

// TestBranchDivergenceSilentWhenBranchesMatch covers the #71 no-warning case:
// when the stored branch_name equals the actual checkout branch, no warning is
// surfaced (advisory only, never a false positive).
func TestBranchDivergenceSilentWhenBranchesMatch(t *testing.T) {
	repo := initGitRepo(t) // checked out on "main"
	row := map[string]any{
		"run_id":      "run_a",
		"state":       "running",
		"branch_name": "main",
		"repo_root":   repo,
	}
	decorateBranchDivergence(row)
	if _, present := row["branch_divergence"]; present {
		t.Fatalf("matching branches must not warn; got %#v", row["branch_divergence"])
	}
}

// TestBranchDivergenceSilentWhenUnknown covers the #71 advisory contract: when
// either side is unknown (no repo_root, no stored branch_name, or a non-git
// repo_root) the check stays silent rather than fabricating a warning or
// failing the read.
func TestBranchDivergenceSilentWhenUnknown(t *testing.T) {
	cases := []map[string]any{
		{"branch_name": "main"},                           // no repo_root
		{"repo_root": t.TempDir()},                        // no branch_name
		{"branch_name": "main", "repo_root": t.TempDir()}, // repo_root is not a git repo
	}
	for i, row := range cases {
		decorateBranchDivergence(row)
		if _, present := row["branch_divergence"]; present {
			t.Fatalf("case %d: expected no warning, got %#v", i, row["branch_divergence"])
		}
	}
}
