package mutations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// clearedACELedgerBody is a cleared adjudicated_constraint_extraction ledger
// (verdict accept_with_findings) carrying one binding, final-review-required
// constraint (C1). The discharge-verifying gate reads its binding constraints
// when a final-review finding claims to discharge them.
const clearedACELedgerBody = `---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "adjudicated_constraint_extraction"
topic: "productive refusal gate"
participants: ["sess_holder", "sess_falsifier", "sess_adjudicator"]
entries:
  - kind: claim
    by: sess_holder
    refs: ["dialogue:1"]
    text: "The proposal is ready."
  - kind: challenge
    by: sess_falsifier
    refs: ["dialogue:2"]
    text: "The proposal lacks an enforceable constraint."
  - kind: rebuttal
    by: sess_holder
    refs: ["dialogue:3"]
    text: "The constraint is now bound."
cycle: 2
verdict: "accept_with_findings"
rationale: "A material challenge landed and the constraint was bound on the record."
findings:
  - id: F1
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: "The implementation did not name the concrete validation gate."
    source_refs: ["dialogue:2"]
constraints:
  - id: C1
    source_finding: F1
    posture: implementation
    severity: high
    kind: gate
    binding: true
    text: "The next revision must add a validation gate for naked needs_revision ledgers."
    source_refs: ["dialogue:2"]
    verification:
      gate: "go -C go test ./pkg/artifactcontracts/..."
    final_review_required: true
---

# Ledger
`

func finalReviewFinding(dischargeBlock string) string {
	return `---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
` + dischargeBlock + `---

# Final Review
`
}

// setupConstraintDischargeRun inserts a repository, workflow snapshot, run, the
// final-review check job (type review, role final_reviewer, phase final_review),
// a session, and an active lease. It then writes a cleared ACE ledger file and
// registers it as a published collaboration_ledger artifact for the run, so the
// gate has binding constraints to verify against. It returns the repo root.
func setupConstraintDischargeRun(t *testing.T, ctx context.Context, runner db.Runner) string {
	t.Helper()
	repoRoot := t.TempDir()
	base := filepath.Join(repoRoot, "docs", "ace")
	if err := os.MkdirAll(filepath.Join(base, "final_review", "check"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "adjudication"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()

	workflow := map[string]any{
		"workflow_id": "wf_ace",
		"lanes":       map[string]any{"author": map[string]any{"display_model": "Claude"}},
		"phases": []any{
			map[string]any{"id": "final_review", "name": "Final review", "synthesis_job_id": "final_review_synthesis"},
		},
		"jobs": []any{
			map[string]any{"id": "final_discharge_check", "type": "review", "role_id": "final_reviewer", "phase_id": "final_review"},
		},
	}
	workflowArg, err := db.JSONBArg(runner, workflow)
	if err != nil {
		t.Fatal(err)
	}
	writeScopeArg, err := db.JSONBArg(runner, map[string]any{
		"mode":          "repo_write",
		"repo_write":    true,
		"allowed_paths": []string{"docs/ace/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "author"})
	if err != nil {
		t.Fatal(err)
	}

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_ace','ident_ace',$1,$2,'repo',$3,14,'active')`,
		repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_ace','snap_ace','wf_ace','sha',$1::jsonb,$2)`, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ('repo_ace','run_ace','snap_ace',$1,'running',$2)`, repoRoot, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  state, registered_at
		) VALUES ('repo_ace','sess_final','run_ace','final_reviewer','author','final-reviewer-1',1,'active',$1)`, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  lane_selector_json, state, write_scope_json, idempotency_key, created_at
		) VALUES ('repo_ace','job_final','run_ace','final_discharge_check','Final review','review','final_reviewer',
		  $1::jsonb,'running',$2::jsonb,'idem_final',$3)`, laneSelectorArg, writeScopeArg, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ('repo_ace','lease_final','run_ace','job','job_final','sess_final','active',$1,$2)`,
		now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}

	// Publish the cleared ACE ledger as an artifact of the run.
	ledgerPath := filepath.Join(base, "adjudication", "LEDGER.md")
	if err := os.WriteFile(ledgerPath, []byte(clearedACELedgerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode, created_at
		) VALUES ('repo_ace','art_ledger','run_ace','job_final','sess_final','forum_ledger',
		  'collaboration_ledger','docs/ace/adjudication/LEDGER.md','sha_ledger',10,'create',$1)`, now); err != nil {
		t.Fatalf("insert ledger artifact: %v", err)
	}
	return repoRoot
}

func publishFinalReviewFinding(ctx context.Context, runner db.Runner, body string) (map[string]any, error) {
	return HandlePublishArtifact(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_final_publish",
		Method:        "artifact.publish",
		Params: map[string]any{
			"repository_id": "repo_ace",
			"session_id":    "sess_final",
			"job_id":        "job_final",
			"lease_id":      "lease_final",
			"kind":          "finding",
			"logical_name":  "constraint_discharge",
			"path":          "docs/ace/final_review/check/CONSTRAINT_DISCHARGE.md",
		},
	})
}

// TestConstraintDischargeGatePassesWhenAllBindingConstraintsDischarged is the
// RFC 0098 Acceptance Criteria #5 happy path: a final-review finding whose
// constraint_discharge[] table marks every binding constraint discharged passes
// the gate (the artifact publishes).
func TestConstraintDischargeGatePassesWhenAllBindingConstraintsDischarged(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupConstraintDischargeRun(t, ctx, runner)

	body := finalReviewFinding(`constraint_discharge:
  - constraint_id: C1
    status: discharged
    evidence: "RFC 0069 §2.1, validation gate added"
`)
	findingPath := filepath.Join(repoRoot, "docs", "ace", "final_review", "check", "CONSTRAINT_DISCHARGE.md")
	if err := os.WriteFile(findingPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := publishFinalReviewFinding(ctx, runner, body)
	if err != nil {
		t.Fatalf("publish should pass when all binding constraints discharged: %v", err)
	}
	if result["status"] != "published" {
		t.Fatalf("publish result = %#v, want published", result)
	}
}

// TestConstraintDischargeGatePassesWhenAcceptedRisk verifies the second passing
// status: accepted_risk with owner+stage clears the typecheck.
func TestConstraintDischargeGatePassesWhenAcceptedRisk(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupConstraintDischargeRun(t, ctx, runner)

	body := finalReviewFinding(`constraint_discharge:
  - constraint_id: C1
    status: accepted_risk
    owner: maintainer
    stage: "Stage 6"
`)
	findingPath := filepath.Join(repoRoot, "docs", "ace", "final_review", "check", "CONSTRAINT_DISCHARGE.md")
	if err := os.WriteFile(findingPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := publishFinalReviewFinding(ctx, runner, body)
	if err != nil {
		t.Fatalf("publish should pass when binding constraint accepted_risk: %v", err)
	}
	if result["status"] != "published" {
		t.Fatalf("publish result = %#v, want published", result)
	}
}

// TestConstraintDischargeGateFailsClosedWhenConstraintMissing is the RFC 0098
// Acceptance Criteria #5 fail-closed path: a final-review finding that omits a
// binding constraint's discharge row is rejected, and the error names the
// offending constraint id.
func TestConstraintDischargeGateFailsClosedWhenConstraintMissing(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupConstraintDischargeRun(t, ctx, runner)

	// The discharge table reports an unrelated constraint, so binding C1 has no
	// discharge row at all.
	body := finalReviewFinding(`constraint_discharge:
  - constraint_id: C2-OTHER
    status: discharged
`)
	findingPath := filepath.Join(repoRoot, "docs", "ace", "final_review", "check", "CONSTRAINT_DISCHARGE.md")
	if err := os.WriteFile(findingPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := publishFinalReviewFinding(ctx, runner, body)
	if err == nil {
		t.Fatal("publish should fail closed when a binding constraint is undischarged")
	}
	if !strings.Contains(err.Error(), "fails closed") || !strings.Contains(err.Error(), "C1") {
		t.Fatalf("error must name the undischarged constraint C1: %v", err)
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "artifact_error" {
		t.Fatalf("error code = %#v, want artifact_error (exit code 6)", err)
	}
}

// TestConstraintDischargeGateFailsClosedOnUnacceptedPartial verifies the §5
// invariant: a `partial` status that is not modeled as accepted_risk fails
// closed.
func TestConstraintDischargeGateFailsClosedOnUnacceptedPartial(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupConstraintDischargeRun(t, ctx, runner)

	body := finalReviewFinding(`constraint_discharge:
  - constraint_id: C1
    status: partial
    evidence: "only half landed"
`)
	findingPath := filepath.Join(repoRoot, "docs", "ace", "final_review", "check", "CONSTRAINT_DISCHARGE.md")
	if err := os.WriteFile(findingPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := publishFinalReviewFinding(ctx, runner, body)
	if err == nil {
		t.Fatal("publish should fail closed when a binding constraint is only partial")
	}
	if !strings.Contains(err.Error(), "C1") || !strings.Contains(err.Error(), "partial") {
		t.Fatalf("error must name C1 partial: %v", err)
	}
}

// TestConstraintDischargeGateInertWithoutDischargeBlock verifies the gate is a
// no-op for an ordinary finding that does not claim to discharge constraints,
// even when the run has a cleared ACE ledger with binding constraints.
func TestConstraintDischargeGateInertWithoutDischargeBlock(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := setupConstraintDischargeRun(t, ctx, runner)

	body := finalReviewFinding("")
	findingPath := filepath.Join(repoRoot, "docs", "ace", "final_review", "check", "CONSTRAINT_DISCHARGE.md")
	if err := os.WriteFile(findingPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := publishFinalReviewFinding(ctx, runner, body)
	if err != nil {
		t.Fatalf("plain finding without a discharge block should publish: %v", err)
	}
	if result["status"] != "published" {
		t.Fatalf("publish result = %#v, want published", result)
	}
}
