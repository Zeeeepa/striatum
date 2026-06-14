package mutations

import (
	"context"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// RFC 0125 P1-1 (GH #286): a completed run's write-once completion record must
// carry a self-contained RUN_LEDGER — per verdict-capable gate, the verdict +
// frozen attestation and every required artifact body's reconstructability
// provenance — and the record's sha256 must be anchored in the terminal event
// so a retrospective reconstructs the run offline from the hash alone.
func TestRunCompletionRecordCarriesRunLedger(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedReconReviewRun(t, ctx, runner, "runledger", "decision", "git_publication", nil)

	commit := anchorBodyOnRunBranch(t, fx.repoRoot, fx.runBranch, fx.repoPath, fx.body)
	gitRun(t, fx.repoRoot, "update-ref", attemptPinRef(fx.runID, fx.jobID, 1), commit)

	driveMaybeCompleteRun(t, ctx, runner, fx.repoID, fx.runID)

	state, _ := runStateAndStopReason(t, ctx, runner, fx.repoID, fx.runID)
	if state != "completed" {
		t.Fatalf("run state = %s, want completed", state)
	}

	row, err := oneRow(ctx, runner, `
		SELECT completion_record_json FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, fx.repoID, fx.runID)
	if err != nil {
		t.Fatalf("read completion record: %v", err)
	}
	rec := asMap(row["completion_record_json"])
	ledger := asList(rec["run_ledger"])
	if len(ledger) != 1 {
		t.Fatalf("run_ledger = %#v, want exactly the seeded gate", rec["run_ledger"])
	}
	gate := asMap(ledger[0])
	if fmt.Sprint(gate["job_id"]) != fx.jobID {
		t.Fatalf("run_ledger gate job_id = %v, want %s", gate["job_id"], fx.jobID)
	}
	if fmt.Sprint(gate["lane_attestation_at_record"]) != "attested" {
		t.Fatalf("run_ledger lane_attestation_at_record = %v, want attested", gate["lane_attestation_at_record"])
	}
	if gate["verdict_id"] == nil || fmt.Sprint(gate["verdict_id"]) == "" {
		t.Fatalf("run_ledger gate missing verdict_id: %#v", gate)
	}
	arts := asList(gate["artifacts"])
	if len(arts) != 1 {
		t.Fatalf("run_ledger artifacts[] = %#v, want 1", gate["artifacts"])
	}
	art := asMap(arts[0])
	if fmt.Sprint(art["content_sha256"]) != fx.contentSHA {
		t.Fatalf("run_ledger artifact content_sha256 = %v, want %s", art["content_sha256"], fx.contentSHA)
	}
	if art["readback_verified"] != true {
		t.Fatalf("run_ledger artifact readback_verified = %v, want true", art["readback_verified"])
	}
	if fmt.Sprint(art["placement"]) != "git_publication" {
		t.Fatalf("run_ledger artifact placement = %v, want git_publication", art["placement"])
	}
	if fmt.Sprint(art["git_anchor_ref"]) != attemptPinRef(fx.runID, fx.jobID, 1) {
		t.Fatalf("run_ledger artifact git_anchor_ref = %v, want the attempt pin", art["git_anchor_ref"])
	}

	// The record's sha256 is anchored in the append-only terminal event, so the
	// ledger is offline-verifiable from the hash alone.
	ev, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.completed'
		 ORDER BY event_id DESC LIMIT 1`, fx.repoID, fx.runID)
	if err != nil {
		t.Fatalf("read run.completed event: %v", err)
	}
	if anchor := fmt.Sprint(asMap(ev["payload_json"])["completion_record_sha256"]); anchor == "" || anchor == "<nil>" {
		t.Fatalf("run.completed event missing completion_record_sha256 anchor")
	}
}
