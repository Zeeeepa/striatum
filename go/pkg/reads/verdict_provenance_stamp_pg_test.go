package reads

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// RFC 0118 P0-1 (GH #240): run.summary surfaces each verdict's frozen
// provenance stamp so a terminal run's review provenance is auditable from
// the summary alone — no live lanehealth probe, no daemon archaeology.
func TestRunSummaryVerdictsCarryFrozenProvenanceStamp(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_stamp_summary"
	runID := "run_stamp"
	now := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	crSeedRepo(t, ctx, runner, repoID, now)
	crSeedRun(t, ctx, runner, repoID, runID, "striatum/stamp", now, false, nil)

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.verdicts (
		  repository_id, verdict_id, run_id, job_id, session_id, verdict,
		  rationale, created_at, posture,
		  lane_attestation_at_record, review_provenance_override,
		  review_provenance_decision_id, supervisor_id_at_record
		) VALUES ($1,'verdict_stamp',$2,$3,$4,'accept','ok',$5,'override',
		  'unattested',true,'dec_stamp','sup_stamp')`,
		repoID, runID, "job_"+runID, "sess_"+runID, now); err != nil {
		t.Fatalf("seed stamped verdict: %v", err)
	}

	result, err := HandleRunSummary(ctx, runner, rpc.Envelope{
		Params: map[string]any{"repository_id": repoID, "run_id": runID},
	})
	if err != nil {
		t.Fatalf("HandleRunSummary: %v", err)
	}
	verdicts, ok := result["verdicts"].([]map[string]any)
	if !ok || len(verdicts) != 1 {
		t.Fatalf("verdicts = %#v, want exactly one row", result["verdicts"])
	}
	row := verdicts[0]
	if got := fmt.Sprint(row["lane_attestation_at_record"]); got != "unattested" {
		t.Fatalf("lane_attestation_at_record = %q, want unattested", got)
	}
	if row["review_provenance_override"] != true {
		t.Fatalf("review_provenance_override = %v, want true", row["review_provenance_override"])
	}
	if got := fmt.Sprint(row["review_provenance_decision_id"]); got != "dec_stamp" {
		t.Fatalf("review_provenance_decision_id = %q, want dec_stamp", got)
	}
	if got := fmt.Sprint(row["supervisor_id_at_record"]); got != "sup_stamp" {
		t.Fatalf("supervisor_id_at_record = %q, want sup_stamp", got)
	}
}
