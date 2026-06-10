package mutations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// RFC 0118 P0-1 (GH #240): every verdict row freezes its provenance basis at
// record time — the lane-attestation state the admission gate decided on, the
// supervising process identity, and any operator override decision — so the
// run-completion gate and post-incident audit can read a verdict's provenance
// from the frozen row alone. Assertions read the verdicts row back via SQL;
// no live lanehealth probe in the assertion path.

func readVerdictProvenanceStamp(t *testing.T, ctx context.Context, runner any, repoID, jobID string) map[string]any {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT lane_attestation_at_record, review_provenance_override,
		       review_provenance_decision_id, supervisor_id_at_record
		  FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("read verdict provenance stamp: %v", err)
	}
	return row
}

// RFC 0118 reconciliation note 1: a session that claimed its job via the
// admin work.claim_override escape (#222) is by construction an
// operator-escaped, non-independent claimant. Any verdict it later records
// must carry the claim's authorizing decision onto the frozen stamp — even
// when the lane is attested at record time — so the run-completion gate can
// never count it toward a clean lanes_attested completion.
func TestRecordVerdictCarriesClaimOverrideDecisionOntoStamp(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedReviewFindingFixture(t, ctx, runner, findingArtifactPayload("accept"))
	runID := "run_" + repoID
	claimDecisionID := "dec_claim_override_" + strings.ReplaceAll(t.Name(), "/", "_")
	if _, err := appendEvent(ctx, runner, repoID, runID, "work.claim_overridden", sessionID, jobID, nil, nil, nil, map[string]any{
		"decision_id": claimDecisionID,
		"override":    "fresh_review_process_lineage",
	}); err != nil {
		t.Fatalf("seed claim_overridden event: %v", err)
	}

	if _, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"verdict":    "accept",
		"rationale":  "override-claimed reviewer accepts",
	})); err != nil {
		t.Fatalf("record accept verdict: %v", err)
	}

	stamp := readVerdictProvenanceStamp(t, ctx, runner, repoID, jobID)
	if got := fmt.Sprint(stamp["lane_attestation_at_record"]); got != "attested" {
		t.Fatalf("lane_attestation_at_record = %q, want attested", got)
	}
	if stamp["review_provenance_override"] != true {
		t.Fatalf("review_provenance_override = %v, want true (claim_override carried forward)", stamp["review_provenance_override"])
	}
	if got := fmt.Sprint(stamp["review_provenance_decision_id"]); got != claimDecisionID {
		t.Fatalf("review_provenance_decision_id = %q, want claim decision %q", got, claimDecisionID)
	}
}

// An unattested fresh review accepted under a verdict-time provenance
// decision stamps the override and the authorizing decision, and freezes the
// unattested lane state the gate actually saw.
func TestRecordVerdictStampsOverrideProvenance(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedUnattestedReviewFindingFixture(t, ctx, runner, findingArtifactPayload("accept"))
	published, err := HandlePublishArtifact(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":   sessionID,
		"job_id":       jobID,
		"lease_id":     leaseID,
		"kind":         "finding",
		"logical_name": "review",
		"path":         "artifacts/review/FINDING.md",
	}))
	if err != nil {
		t.Fatalf("publish finding: %v", err)
	}
	markReviewJobFresh(t, ctx, runner, repoID, jobID)
	runID := "run_" + repoID
	decisionID := "dec_stamp_override_" + strings.ReplaceAll(t.Name(), "/", "_")
	seedReviewProvenanceDecision(t, ctx, runner, repoID, runID, decisionID)

	if _, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":                    sessionID,
		"job_id":                        jobID,
		"lease_id":                      leaseID,
		"verdict":                       "accept",
		"findings_artifact_id":          published["artifact_id"],
		"rationale":                     "operator accepted unattested fresh review",
		"review_provenance_decision_id": decisionID,
	})); err != nil {
		t.Fatalf("record verdict with provenance decision: %v", err)
	}

	stamp := readVerdictProvenanceStamp(t, ctx, runner, repoID, jobID)
	if got := fmt.Sprint(stamp["lane_attestation_at_record"]); got != "unattested" {
		t.Fatalf("lane_attestation_at_record = %q, want unattested", got)
	}
	if stamp["review_provenance_override"] != true {
		t.Fatalf("review_provenance_override = %v, want true", stamp["review_provenance_override"])
	}
	if got := fmt.Sprint(stamp["review_provenance_decision_id"]); got != decisionID {
		t.Fatalf("review_provenance_decision_id = %q, want %q", got, decisionID)
	}
	if stamp["supervisor_id_at_record"] != nil {
		t.Fatalf("supervisor_id_at_record = %v, want NULL", stamp["supervisor_id_at_record"])
	}
}

func TestRecordVerdictStampsAttestedProvenance(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, sessionID, jobID, leaseID := seedReviewFindingFixture(t, ctx, runner, findingArtifactPayload("accept"))

	if _, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"verdict":    "accept",
		"rationale":  "attested reviewer accepts",
	})); err != nil {
		t.Fatalf("record accept verdict: %v", err)
	}

	stamp := readVerdictProvenanceStamp(t, ctx, runner, repoID, jobID)
	if got := fmt.Sprint(stamp["lane_attestation_at_record"]); got != "attested" {
		t.Fatalf("lane_attestation_at_record = %q, want attested", got)
	}
	if stamp["review_provenance_override"] != false {
		t.Fatalf("review_provenance_override = %v, want false", stamp["review_provenance_override"])
	}
	if stamp["review_provenance_decision_id"] != nil {
		t.Fatalf("review_provenance_decision_id = %v, want NULL", stamp["review_provenance_decision_id"])
	}
	if got := fmt.Sprint(stamp["supervisor_id_at_record"]); got != "sup_"+sessionID {
		t.Fatalf("supervisor_id_at_record = %q, want %q", got, "sup_"+sessionID)
	}
}
