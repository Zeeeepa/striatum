package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestClaimNextExplainsRunNeedsOperator guards #207 (part A): a run frozen in
// needs_operator returned a bare no_work to a polling session, an invisible gate
// (hours of staffing attempts failed against it). claim_next must now surface
// ineligible_reason=run_needs_operator plus the open escalation id, mirroring the
// #107 fresh_session ineligibility shape, so the lane stops polling and the
// operator knows exactly what to resolve.
func TestClaimNextExplainsRunNeedsOperator(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_needs_operator"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)
	preseedExhaustedBudget(t, ctx, runner, repoID, runID, jobID)

	// Drive the run to needs_operator with a real recovery_exhausted escalation.
	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "needs_operator" {
		t.Fatalf("run state = %q, want needs_operator", got)
	}
	escalationID, _ := recoveryExhaustedEscalation(t, ctx, runner, repoID, runID)

	// A fresh, eligible implementer session polls. A pending work message for its
	// (role, lane) is immediately visible; only the needs_operator run state gates
	// it. (seedDeadLaneRepoWriteJob left an acked message — flip it back to pending
	// so the claim would succeed but for the run state.)
	freshSession := "sess_fresh_" + repoID
	// Ordinal 2: the dead session (ordinal 1) already holds implementer/claude.
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, freshSession, "implementer", "claude", []string{"claim", "write"}, "active", 2)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.queue_messages SET state = 'pending'
		 WHERE repository_id = $1 AND run_id = $2 AND kind = 'work'`, repoID, runID); err != nil {
		t.Fatalf("repend work message: %v", err)
	}

	res, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": freshSession}))
	if err != nil {
		t.Fatalf("claim_next: %v", err)
	}
	if got := fmt.Sprint(res["status"]); got != "no_work" {
		t.Fatalf("status = %q, want no_work", got)
	}
	if got := fmt.Sprint(res["ineligible_reason"]); got != "run_needs_operator" {
		t.Fatalf("ineligible_reason = %q, want run_needs_operator (bare no_work is the #207 trap)", got)
	}
	if got := fmt.Sprint(res["blocker_id"]); got != escalationID {
		t.Fatalf("blocker_id = %q, want the open escalation id %q", got, escalationID)
	}
	if got := fmt.Sprint(res["hint"]); got == "" || got == "<nil>" {
		t.Fatalf("hint missing; want operator guidance, got %q", got)
	}
}

// TestResolveAutonomousBlockersClosesMootRecoveryExhausted guards #207 (part C):
// a recovery_exhausted blocker raised against a job that SUBSEQUENTLY completes a
// genuine attempt is moot — recovery exhaustion was a false alarm the job itself
// disproved. The completion path must auto-close ONLY that escalation-by-
// exhaustion blocker (+ its escalation_inbox row + a blocker.resolved_on_completion
// event) and restore the run from needs_operator to running, so the run is not
// frozen forever behind a stale blocker that no door could clear.
func TestResolveAutonomousBlockersClosesMootRecoveryExhausted(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_moot_recovery_exhausted"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)
	preseedExhaustedBudget(t, ctx, runner, repoID, runID, jobID)

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "needs_operator" {
		t.Fatalf("run state = %q, want needs_operator", got)
	}
	escalationID, _ := recoveryExhaustedEscalation(t, ctx, runner, repoID, runID)

	// The job now completes a genuine attempt — simulate the completion-path call.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return nil, resolveAutonomousBlockersOnCompletion(ctx, tx, repoID, runID, jobID, "", now)
	}); err != nil {
		t.Fatalf("resolveAutonomousBlockersOnCompletion: %v", err)
	}

	// Blocker + escalation_inbox row resolved.
	row, err := oneRow(ctx, runner, `
		SELECT ei.state AS ei_state, b.state AS b_state
		  FROM striatumd.escalation_inbox ei
		  JOIN striatumd.blockers b
		    ON b.repository_id = ei.repository_id AND b.blocker_id = ei.blocker_id
		 WHERE ei.repository_id = $1 AND ei.escalation_id = $2`, repoID, escalationID)
	if err != nil {
		t.Fatalf("read escalation after completion: %v", err)
	}
	if got := fmt.Sprint(row["b_state"]); got != "resolved" {
		t.Fatalf("blocker state = %q, want resolved (moot recovery_exhausted should auto-close)", got)
	}
	if got := fmt.Sprint(row["ei_state"]); got != "resolved" {
		t.Fatalf("escalation_inbox state = %q, want resolved", got)
	}

	// Run restored to running.
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "running" {
		t.Fatalf("run state = %q, want running (needs_operator should clear once its sole blocker is moot)", got)
	}

	// A blocker.resolved_on_completion event names the moot reason.
	ev, err := oneRow(ctx, runner, `
		SELECT payload_json
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'blocker.resolved_on_completion'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read resolved-on-completion event: %v", err)
	}
	if got := fmt.Sprint(asMap(ev["payload_json"])["reason"]); got != "recovery_exhausted_moot_after_job_completed" {
		t.Fatalf("event reason = %q, want recovery_exhausted_moot_after_job_completed", got)
	}
}

// TestResolveAutonomousBlockersKeepsHumanCheckpointAndOtherEscalations guards the
// safety boundary of #207 (part C): auto-close is narrow. A human_checkpoint
// blocker against the SAME completing job is left open, and a sibling job's open
// escalation keeps the run needs_operator even after the completing job's
// recovery_exhausted blocker is cleared — only a genuinely-moot, sole reason for
// freezing is unfrozen.
func TestResolveAutonomousBlockersKeepsHumanCheckpointAndOtherEscalations(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_keep_human_escalation"
	runID, jobID, _, _, downstreamJobID := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)
	preseedExhaustedBudget(t, ctx, runner, repoID, runID, jobID)

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// A human_checkpoint blocker against the SAME job, and an independent
	// escalation-class blocker against a SIBLING job, are both open.
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id, severity,
		  blocker_kind, description, state, created_at, payload_json
		) VALUES
		  ($1,'blk_human',$2,$3,NULL,'human_checkpoint','ai_self_declared','needs human','open',$4,'{}'::jsonb),
		  ($1,'blk_sibling',$2,$5,NULL,'blocked','ambiguous_goal','sibling escalation','open',$4,'{}'::jsonb)`,
		repoID, runID, jobID, now, downstreamJobID); err != nil {
		t.Fatalf("insert human + sibling blockers: %v", err)
	}

	completedAt := now.Format(time.RFC3339Nano)
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return nil, resolveAutonomousBlockersOnCompletion(ctx, tx, repoID, runID, jobID, "", completedAt)
	}); err != nil {
		t.Fatalf("resolveAutonomousBlockersOnCompletion: %v", err)
	}

	// The human_checkpoint blocker on the completing job stays open.
	if got := blockerStateByID(t, ctx, runner, repoID, "blk_human"); got != "open" {
		t.Fatalf("human_checkpoint blocker state = %q, want open (must never auto-resolve)", got)
	}
	// The sibling escalation stays open.
	if got := blockerStateByID(t, ctx, runner, repoID, "blk_sibling"); got != "open" {
		t.Fatalf("sibling escalation state = %q, want open", got)
	}
	// The recovery_exhausted blocker on the completing job IS resolved (it is moot).
	exhaustedState, err := oneRow(ctx, runner, `
		SELECT state FROM striatumd.blockers
		 WHERE repository_id = $1 AND run_id = $2 AND blocker_kind = 'recovery_exhausted'`, repoID, runID)
	if err != nil {
		t.Fatalf("read recovery_exhausted blocker: %v", err)
	}
	if got := fmt.Sprint(exhaustedState["state"]); got != "resolved" {
		t.Fatalf("recovery_exhausted blocker state = %q, want resolved", got)
	}
	// The run stays needs_operator because a genuine escalation remains.
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "needs_operator" {
		t.Fatalf("run state = %q, want needs_operator (sibling escalation keeps it frozen)", got)
	}
}

func blockerStateByID(t *testing.T, ctx context.Context, runner any, repoID, blockerID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.blockers WHERE repository_id = $1 AND blocker_id = $2`, repoID, blockerID)
	if err != nil {
		t.Fatalf("read blocker %s: %v", blockerID, err)
	}
	return fmt.Sprint(row["state"])
}
