package reads

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests cover D205 (doctor integrity legibility P1): default-branch
// *history* preservation (Rule A) clears merged-then-edited/deleted paths to
// clean; a path still live on the default-branch tip with different content
// (Rule B) becomes an artifact_superseded_on_default_branch warning; and a
// curated, sha-bound acknowledged_loss baseline (Rule C) downgrades a reviewed,
// immaterial genuine loss to a warning — while ANY loss not covered by the
// baseline (or covered with a mismatched sha) MUST still red `ok`.

func writeAckLossBaseline(t *testing.T, repoRoot string, entries []acknowledgedLossEntry) {
	t.Helper()
	dir := filepath.Join(repoRoot, filepath.FromSlash("docs/operator"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(acknowledgedLossFile{Schema: acknowledgedLossSchema, Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "doctor-acknowledged-loss.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Rule A: an artifact whose content_sha256 matches a *historical* (not tip)
// revision of its path on the default branch is durably preserved -> clean (no
// problem, no warning). The deliverable was merged, then the path was rewritten
// later, so the recorded content still has a durable home in history.
func TestDoctorArtifactAnchorPreservedInDefaultBranchHistoryIsClean(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot)
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifactPath := "docs/artifact.md"
	originalBody := "merged artifact body v1\n"
	if err := os.WriteFile(filepath.Join(repoRoot, filepath.FromSlash(artifactPath)), []byte(originalBody), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", artifactPath)
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "add artifact v1")
	// Rewrite the path so the recorded content lives only in history, not at tip.
	if err := os.WriteFile(filepath.Join(repoRoot, filepath.FromSlash(artifactPath)), []byte("artifact body v2 (revised post-merge)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", artifactPath)
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "revise artifact to v2")

	row := artifactAnchorRow(repoRoot, "art_history", "run_history", "job_history", "wf/deleted-run-branch", artifactPath, testSHA256(originalBody))
	row["run_state"] = "completed"

	block, problems, records, warnings, warningRecords := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("content preserved in default-branch history must not red ok: problems=%#v records=%#v", problems, records)
	}
	if len(warnings) != 0 || len(warningRecords) != 0 {
		t.Fatalf("history-preserved content must be clean (no warning): warnings=%#v", warnings)
	}
	if block["git_anchor_count"] != 1 {
		t.Fatalf("git_anchor_count = %#v, want 1", block["git_anchor_count"])
	}
}

// An artifact whose recorded body was superseded by a later attempt on the same
// run branch is still durably reconstructable from that run branch's path
// history. The operator may be looking at the revised branch tip, but doctor
// should not red when the original body is still reachable in the run branch.
func TestDoctorArtifactAnchorPreservedInRunBranchHistoryIsClean(t *testing.T) {
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

	row := artifactAnchorRow(repoRoot, "art_run_history", "run_history", "job_history", runBranch, artifactPath, testSHA256(originalBody))
	row["run_state"] = "running"

	_, problems, records, warnings, warningRecords := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("content preserved in run-branch history must not red ok: problems=%#v records=%#v", problems, records)
	}
	if len(warnings) != 0 || len(warningRecords) != 0 {
		t.Fatalf("run-history-preserved content must be clean (no warning): warnings=%#v", warnings)
	}
}

// Rule B: the deliverable's path is still live on the default-branch tip but with
// different content (the recorded sha is an intermediate draft revised before
// merge) -> exactly one artifact_superseded_on_default_branch warning, no problem.
func TestDoctorArtifactSupersededOnDefaultBranchWarns(t *testing.T) {
	requireGit(t)
	repoRoot, _, artifactPath, _ := seedAnchoredArtifact(t, "final merged content\n")
	draftSHA := testSHA256("intermediate draft content\n") // never committed anywhere
	row := artifactAnchorRow(repoRoot, "art_superseded", "run_superseded", "job_superseded", "wf/deleted-run-branch", artifactPath, draftSHA)
	row["run_state"] = "completed"

	_, problems, records, warnings, warningRecords := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("superseded-on-default-branch artifact must not red ok: problems=%#v records=%#v", problems, records)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "artifact_superseded_on_default_branch.art_superseded") {
		t.Fatalf("warnings = %#v, want artifact_superseded_on_default_branch", warnings)
	}
	if len(warningRecords) != 1 || warningRecords[0]["check"] != artifactSupersededOnDefaultBranch {
		t.Fatalf("warningRecords = %#v, want one superseded warning record", warningRecords)
	}
}

// Rule C accept: a genuine-loss artifact (path absent from the default branch)
// whose id+sha are in the curated baseline -> one artifact_acknowledged_loss
// warning (carrying acknowledged_by), no problem; block status = loaded.
func TestDoctorArtifactAcknowledgedLossInBaselineWarns(t *testing.T) {
	requireGit(t)
	repoRoot := t.TempDir()
	readsGitInit(t, repoRoot) // default branch has only the seed; docs/lost.md never exists
	lostSHA := testSHA256("lost dogfood body\n")
	writeAckLossBaseline(t, repoRoot, []acknowledgedLossEntry{{
		ArtifactID:     "art_ack",
		RepoPath:       "docs/lost.md",
		ContentSHA256:  lostSHA,
		Reason:         "Superseded dogfood draft; never merged to main. Immaterial historical loss.",
		AcknowledgedBy: "operator-reviewer-test-1",
		AcknowledgedAt: "2026-06-16",
	}})
	row := artifactAnchorRow(repoRoot, "art_ack", "run_ack", "job_ack", "wf/deleted-run-branch", "docs/lost.md", lostSHA)
	row["run_state"] = "running" // live run: proves the Rule C downgrade, not terminal-debris

	block, problems, records, warnings, warningRecords := doctorArtifactAnchorIntegrity(
		context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("acknowledged loss must not red ok: problems=%#v records=%#v", problems, records)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "artifact_acknowledged_loss.art_ack") {
		t.Fatalf("warnings = %#v, want artifact_acknowledged_loss", warnings)
	}
	if len(warningRecords) != 1 || warningRecords[0]["check"] != artifactAcknowledgedLoss {
		t.Fatalf("warningRecords = %#v, want one acknowledged-loss warning record", warningRecords)
	}
	contextMap := warningRecords[0]["context"].(map[string]any)
	if contextMap["acknowledged_by"] != "operator-reviewer-test-1" {
		t.Fatalf("warning record context = %#v, want acknowledged_by carried through", contextMap)
	}
	if block["acknowledged_loss_status"] != acknowledgedLossLoaded {
		t.Fatalf("acknowledged_loss_status = %#v, want loaded", block["acknowledged_loss_status"])
	}
}

// Rule C safety (load-bearing): a genuine loss NOT covered by the baseline, or
// covered with a mismatched sha, MUST still red `ok`. The acknowledgment binds to
// exact content so a stale/wrong entry can never mask a different future problem.
func TestDoctorArtifactAcknowledgedLossSafetyStillReds(t *testing.T) {
	requireGit(t)
	lostSHA := testSHA256("lost dogfood body\n")

	t.Run("not_in_baseline", func(t *testing.T) {
		repoRoot := t.TempDir()
		readsGitInit(t, repoRoot)
		writeAckLossBaseline(t, repoRoot, []acknowledgedLossEntry{{
			ArtifactID:     "art_other",
			RepoPath:       "docs/other.md",
			ContentSHA256:  testSHA256("a different artifact\n"),
			Reason:         "an unrelated acknowledged loss",
			AcknowledgedBy: "operator-reviewer-test-1",
			AcknowledgedAt: "2026-06-16",
		}})
		row := artifactAnchorRow(repoRoot, "art_lost", "run_lost", "job_lost", "wf/deleted-run-branch", "docs/lost.md", lostSHA)
		row["run_state"] = "running"

		_, problems, _, warnings, _ := doctorArtifactAnchorIntegrity(
			context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
		if !strings.Contains(strings.Join(problems, "\n"), "artifact_anchor_missing_file.art_lost") {
			t.Fatalf("loss not in the baseline must still red ok: problems=%#v", problems)
		}
		if len(warnings) != 0 {
			t.Fatalf("not-in-baseline loss must not be downgraded to a warning: warnings=%#v", warnings)
		}
	})

	t.Run("sha_mismatch", func(t *testing.T) {
		repoRoot := t.TempDir()
		readsGitInit(t, repoRoot)
		writeAckLossBaseline(t, repoRoot, []acknowledgedLossEntry{{
			ArtifactID:     "art_lost", // same id, but...
			RepoPath:       "docs/lost.md",
			ContentSHA256:  testSHA256("a DIFFERENT lost body\n"), // ...mismatched sha
			Reason:         "a stale acknowledgment for different content",
			AcknowledgedBy: "operator-reviewer-test-1",
			AcknowledgedAt: "2026-06-16",
		}})
		row := artifactAnchorRow(repoRoot, "art_lost", "run_lost", "job_lost", "wf/deleted-run-branch", "docs/lost.md", lostSHA)
		row["run_state"] = "running"

		_, problems, _, warnings, _ := doctorArtifactAnchorIntegrity(
			context.Background(), &doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}}, "repo_anchor", healthyBlobBlock())
		if !strings.Contains(strings.Join(problems, "\n"), "artifact_anchor_missing_file.art_lost") {
			t.Fatalf("id match but sha mismatch must still red ok: problems=%#v", problems)
		}
		if len(warnings) != 0 {
			t.Fatalf("sha-mismatched acknowledgment must not be downgraded: warnings=%#v", warnings)
		}
	})
}

// The baseline reader safe-degrades: an absent file -> empty "absent" set with no
// downgrades; malformed/wrong-schema -> "parse_error" empty set; a valid file ->
// "loaded" with sha-bound honor that accepts an exact match and rejects mismatch.
func TestLoadAcknowledgedLossSetSafeDegrades(t *testing.T) {
	if set := loadAcknowledgedLossSet(""); set.status != acknowledgedLossAbsent || len(set.byID) != 0 {
		t.Fatalf("empty repo root: status=%q n=%d, want absent/0", set.status, len(set.byID))
	}

	repoRoot := t.TempDir()
	set := loadAcknowledgedLossSet(repoRoot)
	if set.status != acknowledgedLossAbsent || len(set.byID) != 0 {
		t.Fatalf("missing file: status=%q n=%d, want absent/0", set.status, len(set.byID))
	}
	if _, ok := set.honor("art_x", testSHA256("x")); ok {
		t.Fatal("an absent baseline must honor nothing")
	}

	dir := filepath.Join(repoRoot, filepath.FromSlash("docs/operator"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(dir, "doctor-acknowledged-loss.json")
	if err := os.WriteFile(baselinePath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if set := loadAcknowledgedLossSet(repoRoot); set.status != acknowledgedLossParseError || len(set.byID) != 0 {
		t.Fatalf("malformed file: status=%q n=%d, want parse_error/0", set.status, len(set.byID))
	}
	if err := os.WriteFile(baselinePath, []byte(`{"schema":"wrong.schema","entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if set := loadAcknowledgedLossSet(repoRoot); set.status != acknowledgedLossParseError {
		t.Fatalf("wrong schema: status=%q, want parse_error", set.status)
	}

	sha := testSHA256("acknowledged body\n")
	writeAckLossBaseline(t, repoRoot, []acknowledgedLossEntry{{
		ArtifactID: "art_v", RepoPath: "docs/v.md", ContentSHA256: sha,
		Reason: "reviewed", AcknowledgedBy: "operator-reviewer-test-1", AcknowledgedAt: "2026-06-16",
	}})
	valid := loadAcknowledgedLossSet(repoRoot)
	if valid.status != acknowledgedLossLoaded || len(valid.byID) != 1 {
		t.Fatalf("valid file: status=%q n=%d, want loaded/1", valid.status, len(valid.byID))
	}
	if _, ok := valid.honor("art_v", sha); !ok {
		t.Fatal("a valid baseline must honor an exact id+sha match")
	}
	if _, ok := valid.honor("art_v", testSHA256("other body\n")); ok {
		t.Fatal("sha-bound honor must reject a sha mismatch")
	}
	if _, ok := valid.honor("art_missing", sha); ok {
		t.Fatal("honor must reject an unknown artifact id")
	}
}
