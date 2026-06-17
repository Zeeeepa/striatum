package reads

import (
	"context"
	"strings"
	"testing"
)

// debrisArtifactRow builds an artifact row whose repo_path is absent from every
// ref (a genuinely lost deliverable) on a terminal-debris run, so the doctor
// classifier reclassifies it to the artifact_debris_terminal_run warning — the
// exact shape `recovery prune-debris` is designed to tombstone.
func debrisArtifactRow(t *testing.T, artifactID string) map[string]any {
	t.Helper()
	repoRoot := t.TempDir()
	baseSHA := readsGitInit(t, repoRoot)
	runBranch := "wf/" + artifactID
	readsGitRun(t, repoRoot, "branch", runBranch, baseSHA)
	row := artifactAnchorRow(repoRoot, artifactID, "run_debris", "job_debris", runBranch, "docs/gone.md", testSHA256("gone body\n"))
	row["run_state"] = "failed"
	return row
}

// TestDoctorArtifactAnchorIntegrityClassifiesTerminalDebris confirms the
// precondition the prune verb relies on: a lost artifact on a failed run is a
// debris WARNING, not a problem.
func TestDoctorArtifactAnchorIntegrityClassifiesTerminalDebris(t *testing.T) {
	row := debrisArtifactRow(t, "art_debris")
	_, problems, _, warnings, _ := doctorArtifactAnchorIntegrity(
		context.Background(),
		&doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}},
		"repo_anchor", healthyBlobBlock())
	if len(problems) != 0 {
		t.Fatalf("problems = %#v, want none (terminal debris is a warning)", problems)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), artifactDebrisTerminalRun+".art_debris") {
		t.Fatalf("warnings = %#v, want %s", warnings, artifactDebrisTerminalRun)
	}
}

// TestDoctorArtifactAnchorIntegritySuppressesTombstonedDebris is the #303
// suppression behavior: once an artifact has a recovery.debris_pruned tombstone,
// the doctor pass skips it entirely — no problem, no warning — so the report
// reads ok again.
func TestDoctorArtifactAnchorIntegritySuppressesTombstonedDebris(t *testing.T) {
	row := debrisArtifactRow(t, "art_debris")
	block, problems, _, warnings, _ := doctorArtifactAnchorIntegrity(
		context.Background(),
		&doctorArtifactAnchorRunner{
			artifactRows:  []map[string]any{row},
			tombstonedIDs: []string{"art_debris"},
		},
		"repo_anchor", healthyBlobBlock())
	if len(problems) != 0 {
		t.Fatalf("problems = %#v, want none after tombstone", problems)
	}
	if joined := strings.Join(warnings, "\n"); strings.Contains(joined, "art_debris") {
		t.Fatalf("warnings = %#v, want art_debris suppressed", warnings)
	}
	if block["debris_pruned_count"] != 1 {
		t.Fatalf("debris_pruned_count = %#v, want 1", block["debris_pruned_count"])
	}
	if block["artifact_count"] != 1 {
		t.Fatalf("artifact_count = %#v, want 1 (the row is still scanned, just skipped)", block["artifact_count"])
	}
}

// TestArtifactDebrisVerdictsEligibleOnTerminalLoss confirms the exported helper
// the prune handler consumes flags a lost terminal-run artifact as eligible with
// the debris code.
func TestArtifactDebrisVerdictsEligibleOnTerminalLoss(t *testing.T) {
	row := debrisArtifactRow(t, "art_debris")
	verdicts, err := ArtifactDebrisVerdicts(
		context.Background(),
		&doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}},
		"repo_anchor", "run_debris")
	if err != nil {
		t.Fatalf("ArtifactDebrisVerdicts: %v", err)
	}
	if len(verdicts) != 1 {
		t.Fatalf("verdicts = %#v, want one", verdicts)
	}
	v := verdicts[0]
	if !v.Eligible || v.Code != artifactDebrisTerminalRun {
		t.Fatalf("verdict = %#v, want eligible debris", v)
	}
	if v.ArtifactID != "art_debris" || v.RunID != "run_debris" || v.JobID != "job_debris" || v.RepoPath != "docs/gone.md" {
		t.Fatalf("verdict identity = %#v", v)
	}
}

// TestArtifactDebrisVerdictsRetainsAnchoredContent confirms a deliverable still
// present at its repo_path on the default branch is NOT eligible (it is durably
// preserved, not debris) even when its run is terminal — the prune verb must
// never tombstone preserved content.
func TestArtifactDebrisVerdictsRetainsAnchoredContent(t *testing.T) {
	body := "preserved on default\n"
	repoRoot, runBranch, artifactPath, contentSHA := seedAnchoredArtifact(t, body)
	row := artifactAnchorRow(repoRoot, "art_keep", "run_keep", "job_keep", runBranch, artifactPath, contentSHA)
	row["run_state"] = "failed"

	verdicts, err := ArtifactDebrisVerdicts(
		context.Background(),
		&doctorArtifactAnchorRunner{artifactRows: []map[string]any{row}},
		"repo_anchor", "run_keep")
	if err != nil {
		t.Fatalf("ArtifactDebrisVerdicts: %v", err)
	}
	if len(verdicts) != 1 {
		t.Fatalf("verdicts = %#v, want one", verdicts)
	}
	if verdicts[0].Eligible {
		t.Fatalf("verdict = %#v, want NOT eligible (content preserved on default branch)", verdicts[0])
	}
}
