package mutations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestPublishArtifactBylineMismatchNamesBothLines covers #73: when a markdown
// artifact's author line does not match the expected work-packet byline, the
// error must name BOTH the submitted (canonical) byline and the expected
// canonical byline so the operator can fix it without reverse-engineering
// attestation state. The seeded review session is unattested with no operator
// label, so the expected byline derives to "author: operator"; the artifact
// carries a different byline.
func TestPublishArtifactBylineMismatchNamesBothLines(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	payload := `---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Finding

author: reviewer-claude-007

The change is correct.
`
	repoID, sessionID, jobID, leaseID := seedReviewFindingFixture(t, ctx, runner, payload)

	_, err := HandlePublishArtifact(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":   sessionID,
		"job_id":       jobID,
		"lease_id":     leaseID,
		"kind":         "finding",
		"logical_name": "finding",
		"path":         "artifacts/review/FINDING.md",
	}))
	if err == nil {
		t.Fatal("expected a byline-mismatch error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does not match the expected work packet author line") {
		t.Fatalf("error does not use the enriched wording: %v", err)
	}
	// Submitted byline (canonical form, lowercased).
	if !strings.Contains(msg, "author: reviewer-claude-007") {
		t.Fatalf("error must name the submitted byline; got: %v", err)
	}
	// Expected canonical byline for an unattested, unlabeled session.
	if !strings.Contains(msg, "author: operator") {
		t.Fatalf("error must name the expected byline; got: %v", err)
	}
}

// TestPublishArtifactLogicalNameConflictGivesRecoveryGuidance covers #94: a
// still-running job that re-publishes the same logical_name with corrected
// content gets the immutable-for-this-attempt recovery guidance, not the bare
// "already exists with different content" line.
func TestPublishArtifactLogicalNameConflictGivesRecoveryGuidance(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedReviewFindingFixture(t, ctx, runner, findingArtifactPayload("accept"))

	// Pre-seed an artifact under logical_name "finding" at the same path but
	// with a content_sha256 that does NOT match the on-disk body, so the
	// re-publish trips the content-mismatch branch.
	now := time.Now().UTC()
	runID := "run_" + repoID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode, created_at
		) VALUES ($1,'art_prior_94',$2,$3,$4,'finding','finding',
		  'artifacts/review/FINDING.md','sha_does_not_match_disk',1,'create',$5)`,
		repoID, runID, jobID, sessionID, now); err != nil {
		t.Fatalf("seed prior artifact: %v", err)
	}

	_, err := HandlePublishArtifact(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":   sessionID,
		"job_id":       jobID,
		"lease_id":     leaseID,
		"kind":         "finding",
		"logical_name": "finding",
		"path":         "artifacts/review/FINDING.md",
	}))
	if err == nil {
		t.Fatal("expected a logical-name conflict error, got nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"immutable for this attempt",
		"Publish a distinct logical_name",
		"revision cycle",
		"#84",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("conflict error missing recovery guidance %q; got: %v", want, err)
		}
	}
}

// TestSubmitReviewInfersSoleExpectedArtifactIdentity covers #96 part 1: when the
// job has exactly one required expected artifact and the caller omits
// logical_name/kind, the submit infers them from that sole expected artifact
// (here logical_name=test_gate_report, kind=finding) instead of defaulting to
// "review"/"finding" and failing the precheck.
func TestSubmitReviewInfersSoleExpectedArtifactIdentity(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedReviewExpectedArtifactFixture(t, ctx, runner, []map[string]any{{
		"logical_name": "test_gate_report",
		"kind":         "finding",
		"path":         "artifacts/review/FINDING.md",
		"required":     true,
	}})

	result, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"path":       "artifacts/review/FINDING.md",
		"verdict":    "accept",
		"rationale":  "gate report verified",
		// NOTE: no logical_name / kind passed -> inference must fire.
	}))
	if err != nil {
		t.Fatalf("submit-review with inferred identity: %v", err)
	}
	if result["job_state"] != "completed" {
		t.Fatalf("job_state = %#v; result=%#v", result["job_state"], result)
	}
	// The published artifact must carry the inferred logical_name, not "review".
	logical, err := runner.QueryScalar(ctx, `
		SELECT logical_name FROM striatumd.artifacts
		 WHERE repository_id = $1 AND job_id = $2 LIMIT 1`, repoID, jobID)
	if err != nil {
		t.Fatalf("query artifact logical_name: %v", err)
	}
	if logical != "test_gate_report" {
		t.Fatalf("published logical_name = %q, want test_gate_report (inferred)", logical)
	}
}

// TestSubmitReviewInferenceUnitSoleExpected is a fast unit check of the
// inference helper itself: a sole required expected artifact supplies the
// missing logical_name + kind; explicit values pass through untouched.
func TestSubmitReviewInferenceUnitSoleExpected(t *testing.T) {
	job := map[string]any{
		"attempt": 1,
		"expected_artifacts_json": []any{map[string]any{
			"logical_name": "test_gate_report",
			"kind":         "finding",
			"path":         "artifacts/review/FINDING.md",
			"required":     true,
		}},
	}
	// Caller omitted both -> infer both.
	gotLogical, gotKind := inferSubmitReviewArtifactIdentity(job, "artifacts/review/FINDING.md", "", "", false, false)
	if gotLogical != "test_gate_report" || gotKind != "finding" {
		t.Fatalf("inference = (%q,%q), want (test_gate_report, finding)", gotLogical, gotKind)
	}
	// Caller passed explicit values -> untouched even with a sole expected.
	gotLogical, gotKind = inferSubmitReviewArtifactIdentity(job, "artifacts/review/FINDING.md", "explicit", "synthesis", true, true)
	if gotLogical != "explicit" || gotKind != "synthesis" {
		t.Fatalf("explicit values mutated: (%q,%q)", gotLogical, gotKind)
	}
}

// TestSubmitReviewInferenceUnitAmbiguous covers #96's ambiguity guard: with
// several required expected artifacts, inference does NOT fire and the caller's
// (empty) values pass through unchanged, so the handler falls back to the
// historical defaults and the precheck demands explicit flags.
func TestSubmitReviewInferenceUnitAmbiguous(t *testing.T) {
	job := map[string]any{
		"attempt": 1,
		"expected_artifacts_json": []any{
			map[string]any{"logical_name": "report_a", "kind": "finding", "path": "a.md", "required": true},
			map[string]any{"logical_name": "report_b", "kind": "finding", "path": "b.md", "required": true},
		},
	}
	gotLogical, gotKind := inferSubmitReviewArtifactIdentity(job, "a.md", "", "", false, false)
	if gotLogical != "" || gotKind != "" {
		t.Fatalf("ambiguous expected artifacts must not infer; got (%q,%q)", gotLogical, gotKind)
	}
}

// TestSubmitReviewAmbiguousExpectedNamesSubmittedAndExpected covers #96 part 2:
// when the artifact would still be missing after submit (multiple required
// expected artifacts, so no inference), the precheck error names BOTH the
// submitted tuple and the expected tuple so the operator can pass the right
// flags.
func TestSubmitReviewAmbiguousExpectedNamesSubmittedAndExpected(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedReviewExpectedArtifactFixture(t, ctx, runner, []map[string]any{
		{"logical_name": "report_a", "kind": "finding", "path": "artifacts/review/A.md", "required": true},
		{"logical_name": "report_b", "kind": "finding", "path": "artifacts/review/B.md", "required": true},
	})

	_, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"path":       "artifacts/review/FINDING.md",
		"verdict":    "accept",
		"rationale":  "no matching expected artifact",
		// no logical_name / kind -> defaults to review/finding, ambiguous so no inference.
	}))
	if err == nil {
		t.Fatal("expected a missing-required-artifact precheck error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "submitted (logical_name=") || !strings.Contains(msg, "expected (logical_name=") {
		t.Fatalf("error must name submitted + expected tuples; got: %v", err)
	}
	// Submitted defaults surface.
	if !strings.Contains(msg, `logical_name="review"`) || !strings.Contains(msg, `path="artifacts/review/FINDING.md"`) {
		t.Fatalf("error must name the submitted default tuple; got: %v", err)
	}
	// At least one expected logical name is named.
	if !strings.Contains(msg, "report_a") && !strings.Contains(msg, "report_b") {
		t.Fatalf("error must name an expected logical_name; got: %v", err)
	}
}

// seedReviewExpectedArtifactFixture seeds a repo + run (with an on-disk repo
// root) + an active reviewer session + a running review job with the supplied
// expected artifacts, an active lease, and writes a no-author-line finding body
// to every expected `path` plus artifacts/review/FINDING.md. It returns the ids
// the review handlers need. The bodies carry no author line so the markdown
// author-line check is skipped (the fixture is independent of attestation).
func seedReviewExpectedArtifactFixture(
	t *testing.T,
	ctx context.Context,
	runner db.Runner,
	expected []map[string]any,
) (repoID, sessionID, jobID, leaseID string) {
	t.Helper()
	repoID = "repo_erg96_" + strings.NewReplacer("/", "_", " ", "_", "-", "_").Replace(t.Name())
	runID := "run_" + repoID
	sessionID = "sess_reviewer_" + repoID
	jobID = "job_review_" + repoID
	leaseID = "lease_" + repoID
	repoRoot := t.TempDir()
	body := findingArtifactPayload("accept")
	writeBody := func(rel string) {
		full := filepath.Join(repoRoot, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeBody("artifacts/review/FINDING.md")
	for _, e := range expected {
		if p, _ := e["path"].(string); p != "" {
			writeBody(p)
		}
	}

	intgSeedRepo(t, ctx, runner, repoID)
	workflow := map[string]any{
		"workflow_id": "review_wf",
		"lanes": map[string]any{
			"reviewer": map[string]any{"display_model": "Claude"},
		},
		"jobs": []any{
			map[string]any{"id": "review", "type": "review"},
		},
	}
	intgSeedRun(t, ctx, runner, repoID, runID, workflow)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET repo_root = $1
		 WHERE repository_id = $2 AND run_id = $3`, repoRoot, repoID, runID); err != nil {
		t.Fatalf("update run repo root: %v", err)
	}
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "reviewer", "reviewer", []string{"review"}, "active")
	now := time.Now().UTC()
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":            "review_only_artifact",
		"repo_write":      false,
		"allowed_paths":   []string{"artifacts/review/"},
		"forbidden_paths": []string{".striatum/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedArg, err := db.JSONBArg(runner, expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, lane_selector_json, write_scope_json,
		  expected_artifacts_json, idempotency_key, created_at, started_at
		) VALUES ($1,$2,$3,'review',1,'running','reviewer','Review','review',
		  $4::jsonb,$5::jsonb,$6::jsonb,'idem_'||$2,$7,$7)`,
		repoID, jobID, runID, laneSelectorArg, writeScopeArg, expectedArg, now); err != nil {
		t.Fatalf("insert review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	return repoID, sessionID, jobID, leaseID
}
