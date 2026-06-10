package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/reads"
)

// RFC 0118 P0-3 (GH #240): before a run completes cleanly, every
// provenance-required review gate is re-verified against the FROZEN verdict
// stamp (P0-1) — attested or override, never a live probe, never assumed. A
// failing gate routes the run to needs_operator/provenance_gate_failed with
// an escalation the operator can resolve, and resolving re-drives completion.

// seedProvenanceGateRun seeds a running run whose single review job is
// already completed, with one accepting verdict carrying the given stamp
// columns — the state maybeCompleteRun sees at the completion boundary.
func seedProvenanceGateRun(t *testing.T, ctx context.Context, runner db.Runner, repoID string, jobDef map[string]any, stamp map[string]any) (runID, jobID, sessionID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_review_" + repoID
	sessionID = "sess_reviewer_" + repoID
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "provgate_wf",
		"lanes":       map[string]any{"reviewer": map[string]any{"display_model": "Claude"}},
		"jobs":        []any{jobDef},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "reviewer", "reviewer", []string{"review"}, "active")
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, expected_artifacts_json, idempotency_key, created_at,
		  started_at, completed_at, fresh_session_required
		) VALUES ($1,$2,$3,$4,1,'completed','reviewer','Review','review','[]'::jsonb,
		  'idem_'||$2,$5,$5,$5,$6)`,
		repoID, jobID, runID, jobDef["id"], now, jobDef["fresh_session_required"] == true); err != nil {
		t.Fatalf("insert completed review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.verdicts (
		  repository_id, verdict_id, run_id, job_id, session_id, verdict,
		  rationale, created_at, posture,
		  lane_attestation_at_record, review_provenance_override,
		  review_provenance_decision_id, supervisor_id_at_record
		) VALUES ($1,'verdict_'||$2,$3,$2,$4,'accept','seeded',$5,
		  COALESCE($6,'neutral'),$7,$8,$9,$10)`,
		repoID, jobID, runID, sessionID, now,
		stamp["posture"], stamp["lane_attestation_at_record"],
		stamp["review_provenance_override"], stamp["review_provenance_decision_id"],
		stamp["supervisor_id_at_record"]); err != nil {
		t.Fatalf("insert stamped verdict: %v", err)
	}
	return runID, jobID, sessionID
}

func driveMaybeCompleteRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) {
	t.Helper()
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return nil, maybeCompleteRun(ctx, tx, repoID, runID)
	}); err != nil {
		t.Fatalf("maybeCompleteRun: %v", err)
	}
}

func runStateAndStopReason(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) (string, string) {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT state, stop_reason FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	return fmt.Sprint(row["state"]), fmt.Sprint(nullable(row["stop_reason"]))
}

// A provenance-required review whose only accepting verdict froze
// unattested-with-no-override must NOT complete cleanly: the run lands in
// needs_operator with a resolvable escalation enumerating the failing gate.
func TestRunCompletionFailsClosedOnUnattestedProvenanceRequiredGate(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_provgate_failclosed"
	runID, jobID, sessionID := seedProvenanceGateRun(t, ctx, runner, repoID,
		map[string]any{"id": "review", "type": "review", "fresh_session_required": true},
		map[string]any{"lane_attestation_at_record": "unattested", "review_provenance_override": false})

	driveMaybeCompleteRun(t, ctx, runner, repoID, runID)

	state, stopReason := runStateAndStopReason(t, ctx, runner, repoID, runID)
	if state != "needs_operator" || stopReason != "provenance_gate_failed" {
		t.Fatalf("run state/stop_reason = %s/%s, want needs_operator/provenance_gate_failed", state, stopReason)
	}
	blocker, err := oneRow(ctx, runner, `
		SELECT blocker_id, state, payload_json FROM striatumd.blockers
		 WHERE repository_id = $1 AND run_id = $2 AND blocker_kind = 'provenance_gate_failed'`,
		repoID, runID)
	if err != nil {
		t.Fatalf("read provenance gate blocker: %v", err)
	}
	if fmt.Sprint(blocker["state"]) != "open" {
		t.Fatalf("blocker state = %v, want open", blocker["state"])
	}
	payload := asMap(blocker["payload_json"])
	if payload["is_escalation"] != true {
		t.Fatalf("blocker payload is_escalation = %v, want true", payload["is_escalation"])
	}
	gates := asList(payload["failing_gates"])
	if len(gates) != 1 || fmt.Sprint(asMap(gates[0])["job_id"]) != jobID {
		t.Fatalf("failing_gates = %#v, want the seeded review gate", payload["failing_gates"])
	}
	inbox, err := oneRow(ctx, runner, `
		SELECT state FROM striatumd.escalation_inbox
		 WHERE repository_id = $1 AND run_id = $2 AND blocker_kind = 'provenance_gate_failed'`,
		repoID, runID)
	if err != nil {
		t.Fatalf("read escalation inbox: %v", err)
	}
	if fmt.Sprint(inbox["state"]) != "pending" {
		t.Fatalf("inbox state = %v, want pending", inbox["state"])
	}
	// The run is NOT terminal: its session must stay active (resumable).
	if got := intgSessionState(t, ctx, runner, repoID, sessionID); got != "active" {
		t.Fatalf("session state = %q, want active (run is resumable, not terminal)", got)
	}
}

// The scope rule (no false-positive regression): a review that is neither
// require_attested_lane nor fresh-required legitimately completes from an
// unattested neutral verdict — the run completes with
// completion_mode='lanes_attested'.
func TestRunCompletionPassesUnattestedNonRequiredGate(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_provgate_scope"
	runID, _, _ := seedProvenanceGateRun(t, ctx, runner, repoID,
		map[string]any{"id": "review", "type": "review"},
		map[string]any{"lane_attestation_at_record": "unattested", "review_provenance_override": false})

	driveMaybeCompleteRun(t, ctx, runner, repoID, runID)

	state, _ := runStateAndStopReason(t, ctx, runner, repoID, runID)
	if state != "completed" {
		t.Fatalf("run state = %s, want completed (non-required gate, shipped admission rule)", state)
	}
	event, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.completed'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read run.completed event: %v", err)
	}
	payload := asMap(event["payload_json"])
	if payload["completion_mode"] != "lanes_attested" {
		t.Fatalf("completion_mode = %v, want lanes_attested", payload["completion_mode"])
	}
	if len(asList(payload["provenance_gate"])) != 1 {
		t.Fatalf("provenance_gate ledger = %#v, want 1 entry", payload["provenance_gate"])
	}
}

// Decision #1 (fail-closed-after-migration): a pre-migration verdict row with
// NULL stamps on a provenance-required gate is treated as NOT attested.
func TestRunCompletionNullStampFailsClosed(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_provgate_nullstamp"
	runID, _, _ := seedProvenanceGateRun(t, ctx, runner, repoID,
		map[string]any{"id": "review", "type": "review", "fresh_session_required": true},
		map[string]any{})

	driveMaybeCompleteRun(t, ctx, runner, repoID, runID)

	state, stopReason := runStateAndStopReason(t, ctx, runner, repoID, runID)
	if state != "needs_operator" || stopReason != "provenance_gate_failed" {
		t.Fatalf("run state/stop_reason = %s/%s, want needs_operator/provenance_gate_failed (NULL stamp fail-closed)", state, stopReason)
	}
}

// An override-cleared provenance-required gate completes the run as
// operator_override and lists the authorizing decision in the ledger.
func TestRunCompletionOverrideBasisCompletesAsOperatorOverride(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_provgate_override"
	runID, _, _ := seedProvenanceGateRun(t, ctx, runner, repoID,
		map[string]any{"id": "review", "type": "review", "fresh_session_required": true},
		map[string]any{
			"lane_attestation_at_record":    "unattested",
			"review_provenance_override":    true,
			"review_provenance_decision_id": "dec_gate_override",
		})

	driveMaybeCompleteRun(t, ctx, runner, repoID, runID)

	state, _ := runStateAndStopReason(t, ctx, runner, repoID, runID)
	if state != "completed" {
		t.Fatalf("run state = %s, want completed", state)
	}
	event, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.completed'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read run.completed event: %v", err)
	}
	payload := asMap(event["payload_json"])
	if payload["completion_mode"] != "operator_override" {
		t.Fatalf("completion_mode = %v, want operator_override", payload["completion_mode"])
	}
	gate := asMap(asList(payload["provenance_gate"])[0])
	if gate["basis"] != "override" || fmt.Sprint(gate["override_decision_id"]) != "dec_gate_override" {
		t.Fatalf("ledger entry = %#v, want override basis bound to dec_gate_override", gate)
	}
}

// A fully attested provenance-required gate completes as lanes_attested.
func TestRunCompletionAttestedRequiredGateCompletesLanesAttested(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_provgate_attested"
	runID, _, _ := seedProvenanceGateRun(t, ctx, runner, repoID,
		map[string]any{"id": "review", "type": "review", "fresh_session_required": true},
		map[string]any{
			"lane_attestation_at_record": "attested",
			"review_provenance_override": false,
			"supervisor_id_at_record":    "sup_seeded",
		})

	driveMaybeCompleteRun(t, ctx, runner, repoID, runID)

	state, _ := runStateAndStopReason(t, ctx, runner, repoID, runID)
	if state != "completed" {
		t.Fatalf("run state = %s, want completed", state)
	}
	event, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.completed'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read run.completed event: %v", err)
	}
	if payload := asMap(event["payload_json"]); payload["completion_mode"] != "lanes_attested" {
		t.Fatalf("completion_mode = %v, want lanes_attested", payload["completion_mode"])
	}
}

// The acceptance-check-1 resume path: a fail-closed run is resumed by
// recording a run-level review_provenance escape decision and resolving the
// escalation — the resolve re-drives completion (via
// reads.RunCompletionRedriveHook, wired exactly as mutations.Register wires
// it) and the run terminates as operator_override, not stuck.
func TestProvenanceGateEscalationResolveRedrivesCompletion(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_provgate_resume"
	runID, _, _ := seedProvenanceGateRun(t, ctx, runner, repoID,
		map[string]any{"id": "review", "type": "review", "fresh_session_required": true},
		map[string]any{"lane_attestation_at_record": "unattested", "review_provenance_override": false})

	driveMaybeCompleteRun(t, ctx, runner, repoID, runID)
	if state, _ := runStateAndStopReason(t, ctx, runner, repoID, runID); state != "needs_operator" {
		t.Fatalf("run state = %s, want needs_operator before resume", state)
	}
	blocker, err := oneRow(ctx, runner, `
		SELECT blocker_id FROM striatumd.blockers
		 WHERE repository_id = $1 AND run_id = $2
		   AND blocker_kind = 'provenance_gate_failed' AND state = 'open'`, repoID, runID)
	if err != nil {
		t.Fatalf("read open blocker: %v", err)
	}

	decisionID := "dec_provgate_resume_escape"
	seedReviewProvenanceDecision(t, ctx, runner, repoID, runID, decisionID)

	prevHook := reads.RunCompletionRedriveHook
	reads.RunCompletionRedriveHook = func(ctx context.Context, tx db.TxRunner, repositoryID, hookRunID string) error {
		return maybeCompleteRun(ctx, tx, repositoryID, hookRunID)
	}
	defer func() { reads.RunCompletionRedriveHook = prevHook }()

	if _, err := reads.HandleEscalationResolve(ctx, runner, intgEnv(repoID, map[string]any{
		"escalation_id": fmt.Sprint(blocker["blocker_id"]),
		"decision_id":   decisionID,
	})); err != nil {
		t.Fatalf("escalation resolve: %v", err)
	}

	state, stopReason := runStateAndStopReason(t, ctx, runner, repoID, runID)
	if state != "completed" || stopReason != "<nil>" {
		t.Fatalf("run state/stop_reason = %s/%s, want completed/<nil> after resolve re-drive", state, stopReason)
	}
	event, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.completed'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read run.completed event: %v", err)
	}
	payload := asMap(event["payload_json"])
	if payload["completion_mode"] != "operator_override" {
		t.Fatalf("completion_mode = %v, want operator_override", payload["completion_mode"])
	}
	gate := asMap(asList(payload["provenance_gate"])[0])
	if gate["basis"] != "override" || fmt.Sprint(gate["override_decision_id"]) != decisionID {
		t.Fatalf("ledger entry = %#v, want override bound to %s", gate, decisionID)
	}
}
