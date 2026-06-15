package reads

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests cover D-doctor-integrity-legibility: un-actionable integrity
// findings (preserved-on-default-branch, terminal-run debris, legacy
// pre-blob-storage artifacts) are reclassified from ok-reddening `problems` to
// non-reddening `warnings`, while genuine loss MUST remain a `problem`.

// Rule 1 (worktree): a worktree HEAD reachable from the default branch is
// durably preserved (run branch merged then deleted) -> warning, not problem.
func TestDoctorWorktreeUnreachableHeadPreservedOnDefaultBranchWarns(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot)
	// Advance the default branch so the worktree HEAD lives on main (as it would
	// after the run branch was merged), then point a detached worktree at it.
	if err := os.WriteFile(filepath.Join(repoRoot, "feature.txt"), []byte("merged feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", "feature.txt")
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "feature merged to main")
	mergedSHA := readsGitRevParse(t, repoRoot, "HEAD")

	worktreeID := "wt_merged"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))
	readsGitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, mergedSHA)

	runner := &doctorWorktreeAnchorFakeRunner{rows: []map[string]any{{
		"worktree_id":     worktreeID,
		"run_id":          "run_merged",
		"job_id":          "job_merged",
		"lease_id":        "lease_merged",
		"base_branch":     "wf/deleted-run-branch",
		"branch_name":     "wf/deleted-run-branch", // deleted post-merge -> unresolvable
		"repo_root":       repoRoot,
		"worktree_path":   worktreeRel,
		"state":           "active",
		"workflow_job_id": "author_draft",
		"job_state":       "completed",
		"run_state":       "completed",
	}}}

	_, problems, records, warnings, warningRecords := doctorWorktreeRefSafety(context.Background(), runner, "repo_wt")
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("preserved-on-default-branch worktree must not red ok: problems=%#v records=%#v", problems, records)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "worktree_unanchored_on_default_branch."+worktreeID) {
		t.Fatalf("warnings = %#v, want worktree_unanchored_on_default_branch", warnings)
	}
	if len(warningRecords) != 1 || warningRecords[0]["check"] != "worktree_unanchored_on_default_branch" {
		t.Fatalf("warningRecords = %#v, want one default-branch warning record", warningRecords)
	}
}

// Rule 2 (worktree): a worktree from a terminal (canceled) run is archived
// debris -> warning, not problem.
func TestDoctorWorktreeTerminalRunDebrisWarns(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	baseSHA := readsGitInit(t, repoRoot)
	runBranch := "wf/canceled"
	readsGitRun(t, repoRoot, "branch", runBranch, baseSHA)

	worktreeID := "wt_canceled"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))
	readsGitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "scratch.txt"), []byte("uncommitted run work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, worktreeRoot, "add", "scratch.txt")
	readsGitRun(t, worktreeRoot, "commit", "-q", "-m", "canceled run work")

	runner := &doctorWorktreeAnchorFakeRunner{rows: []map[string]any{{
		"worktree_id":     worktreeID,
		"run_id":          "run_canceled",
		"job_id":          "job_canceled",
		"lease_id":        "lease_canceled",
		"base_branch":     runBranch,
		"branch_name":     runBranch, // exists but points at base; HEAD not reachable
		"repo_root":       repoRoot,
		"worktree_path":   worktreeRel,
		"state":           "active",
		"workflow_job_id": "author_draft",
		"job_state":       "completed",
		"run_state":       "canceled",
	}}}

	_, problems, records, warnings, warningRecords := doctorWorktreeRefSafety(context.Background(), runner, "repo_wt")
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("canceled-run worktree debris must not red ok: problems=%#v records=%#v", problems, records)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "worktree_debris_terminal_run."+worktreeID) {
		t.Fatalf("warnings = %#v, want worktree_debris_terminal_run", warnings)
	}
	if len(warningRecords) != 1 || warningRecords[0]["check"] != "worktree_debris_terminal_run" {
		t.Fatalf("warningRecords = %#v, want one terminal-debris warning record", warningRecords)
	}
}

// Load-bearing safety (worktree): a worktree HEAD on no durable ref AND not on
// the default branch, from a live run, MUST stay an ok-reddening problem.
func TestDoctorWorktreeGenuineLossStillReds(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	baseSHA := readsGitInit(t, repoRoot)
	runBranch := "wf/live"
	readsGitRun(t, repoRoot, "branch", runBranch, baseSHA)

	worktreeID := "wt_live"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))
	readsGitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "scratch.txt"), []byte("live unflushed work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, worktreeRoot, "add", "scratch.txt")
	readsGitRun(t, worktreeRoot, "commit", "-q", "-m", "live run work")
	worktreeHead := readsGitRevParse(t, worktreeRoot, "HEAD")

	runner := &doctorWorktreeAnchorFakeRunner{rows: []map[string]any{{
		"worktree_id":     worktreeID,
		"run_id":          "run_live",
		"job_id":          "job_live",
		"lease_id":        "lease_live",
		"base_branch":     runBranch,
		"branch_name":     runBranch,
		"repo_root":       repoRoot,
		"worktree_path":   worktreeRel,
		"state":           "active",
		"workflow_job_id": "author_draft",
		"job_state":       "completed",
		"run_state":       "running",
	}}}

	_, problems, _, warnings, _ := doctorWorktreeRefSafety(context.Background(), runner, "repo_wt")
	problemText := strings.Join(problems, "\n")
	if !strings.Contains(problemText, "worktree_head_unreachable."+worktreeID) ||
		!strings.Contains(problemText, "job_completed_without_anchor.job_live") ||
		!strings.Contains(problemText, worktreeHead) {
		t.Fatalf("genuine worktree loss must still red ok: problems=%#v", problems)
	}
	if len(warnings) != 0 {
		t.Fatalf("genuine loss must not be downgraded to a warning: warnings=%#v", warnings)
	}
}

// Rule 1 (artifact): a git-anchor artifact whose content matches its repo_path
// at the default-branch tip is durably preserved -> fully clean (the prompt
// scopes artifact preservation to "not a problem"; nothing for the operator to
// anchor, so it is not even a warning).
func TestDoctorArtifactAnchorPreservedOnDefaultBranchIsClean(t *testing.T) {
	requireGit(t)
	repoRoot, _, artifactPath, contentSHA := seedAnchoredArtifact(t, "merged artifact body\n")
	row := artifactAnchorRow(repoRoot, "art_merged", "run_merged", "job_merged", "wf/deleted-run-branch", artifactPath, contentSHA)
	row["run_state"] = "completed"

	block, problems, records, warnings, warningRecords := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("artifact preserved on default branch must not red ok: problems=%#v records=%#v", problems, records)
	}
	if len(warnings) != 0 || len(warningRecords) != 0 {
		t.Fatalf("artifact preserved on default branch must be clean (no warning): warnings=%#v", warnings)
	}
	if block["git_anchor_count"] != 1 {
		t.Fatalf("block git_anchor_count = %#v, want 1", block["git_anchor_count"])
	}
}

// Rule 3 (artifact): a blob-placement artifact with an empty blob_key predates
// RFC 0125 blob storage; if its content is verifiable on the default branch it
// is a legacy warning, not an artifact_blob_metadata_missing problem.
func TestDoctorArtifactLegacyBlobKeyPreservedOnDefaultBranchWarns(t *testing.T) {
	requireGit(t)
	repoRoot, _, artifactPath, contentSHA := seedAnchoredArtifact(t, "legacy artifact body\n")
	row := artifactAnchorRow(repoRoot, "art_legacy", "run_legacy", "job_legacy", "wf/deleted-run-branch", artifactPath, contentSHA)
	row["artifact_kind"] = "synthesis"
	row["placement"] = "blob_exhaust"
	row["blob_key"] = "" // legacy: predates blob storage
	row["run_state"] = "completed"

	block, problems, records, warnings, warningRecords := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("legacy preserved artifact must not red ok: problems=%#v records=%#v", problems, records)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "artifact_legacy_unverifiable.art_legacy") {
		t.Fatalf("warnings = %#v, want artifact_legacy_unverifiable", warnings)
	}
	if len(warningRecords) != 1 || warningRecords[0]["check"] != artifactLegacyUnverifiable {
		t.Fatalf("warningRecords = %#v, want one legacy warning record", warningRecords)
	}
	if block["warning_count"] != 1 || block["blob_exhaust_count"] != 1 {
		t.Fatalf("block counts = %#v, want warning_count=1 blob_exhaust_count=1", block)
	}
}

// Load-bearing safety (artifact): content on no durable ref AND not on the
// default branch, from a live run, MUST stay an ok-reddening problem.
func TestDoctorArtifactGenuineLossStillReds(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot) // default branch has only the seed; no artifact anywhere
	row := artifactAnchorRow(repoRoot, "art_lost", "run_lost", "job_lost", "wf/deleted-run-branch", "docs/lost.md", testSHA256("lost body\n"))
	row["run_state"] = "running"

	_, problems, records, warnings, warningRecords := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if !strings.Contains(strings.Join(problems, "\n"), "artifact_anchor_missing_file.art_lost") {
		t.Fatalf("genuine artifact loss must still red ok: problems=%#v", problems)
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v, want one problem record", records)
	}
	if len(warnings) != 0 || len(warningRecords) != 0 {
		t.Fatalf("genuine loss must not be downgraded to a warning: warnings=%#v", warnings)
	}
}

// Load-bearing safety (legacy artifact): an empty blob_key whose content is
// also absent everywhere is genuine loss, not a legacy warning.
func TestDoctorArtifactLegacyBlobKeyGenuineLossStillReds(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot)
	row := artifactAnchorRow(repoRoot, "art_legacy_lost", "run_legacy_lost", "job_legacy_lost", "wf/deleted-run-branch", "docs/legacy-lost.md", testSHA256("legacy lost body\n"))
	row["artifact_kind"] = "synthesis"
	row["placement"] = "blob_exhaust"
	row["blob_key"] = ""
	row["run_state"] = "running"

	_, problems, _, warnings, _ := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if !strings.Contains(strings.Join(problems, "\n"), "artifact_blob_metadata_missing.art_legacy_lost") {
		t.Fatalf("legacy artifact with no preserved content must still red ok: problems=%#v", problems)
	}
	if len(warnings) != 0 {
		t.Fatalf("genuine legacy loss must not be downgraded to a warning: warnings=%#v", warnings)
	}
}

// readGitDefaultBranchRef must degrade safely on a repo with no resolvable
// default branch rather than crashing or hanging.
func TestReadGitDefaultBranchRefDegradesSafely(t *testing.T) {
	requireGit(t)
	if ref := readGitDefaultBranchRef(context.Background(), ""); ref != "" {
		t.Fatalf("empty repo root: ref = %q, want \"\"", ref)
	}
	if ref := readGitDefaultBranchRef(context.Background(), filepath.Join(t.TempDir(), "not-a-repo")); ref != "" {
		t.Fatalf("non-repo path: ref = %q, want \"\"", ref)
	}
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot)
	if ref := readGitDefaultBranchRef(context.Background(), repoRoot); ref != "refs/heads/main" {
		t.Fatalf("local main repo: ref = %q, want refs/heads/main", ref)
	}
}
