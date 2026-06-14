package mutations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

// #289 unit: deadAgentExitedUnsealed separates "engaged the work protocol +
// produced output, never sealed" from a hard crash or a sealed session.
func TestDeadAgentExitedUnsealedPredicate(t *testing.T) {
	now := time.Now().UTC()
	tp := func(d time.Duration) *time.Time { v := now.Add(d); return &v }

	cases := []struct {
		name string
		act  sessionliveness.Activity
		want bool
	}{
		{
			name: "tool call + pty, no work.complete -> unsealed",
			act:  sessionliveness.Activity{LastToolCallStartedAt: tp(-time.Minute), LastPTYActivityAt: tp(-time.Second)},
			want: true,
		},
		{
			name: "finished tool call + pty, no work.complete -> unsealed",
			act:  sessionliveness.Activity{LastToolCallFinishedAt: tp(-time.Minute), LastPTYActivityAt: tp(-time.Second)},
			want: true,
		},
		{
			name: "did seal (work.complete set) -> not unsealed",
			act:  sessionliveness.Activity{LastWorkCompleteAt: tp(-time.Minute), LastToolCallFinishedAt: tp(-2 * time.Minute), LastPTYActivityAt: tp(-time.Minute)},
			want: false,
		},
		{
			name: "pty only, no tool call -> not unsealed (no protocol engagement)",
			act:  sessionliveness.Activity{LastPTYActivityAt: tp(-time.Second)},
			want: false,
		},
		{
			name: "tool call only, no pty -> not unsealed (no output)",
			act:  sessionliveness.Activity{LastToolCallStartedAt: tp(-time.Minute)},
			want: false,
		},
		{
			name: "nothing -> not unsealed (hard early crash)",
			act:  sessionliveness.Activity{},
			want: false,
		},
	}
	for _, tc := range cases {
		if got := deadAgentExitedUnsealed(tc.act); got != tc.want {
			t.Errorf("%s: deadAgentExitedUnsealed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// #289 unit: the recovery policy parses max_unsealed_requeues and defaults it.
func TestRecoveryPolicyUnsealedBudget(t *testing.T) {
	def := recoveryPolicyFromWorkflow(map[string]any{})
	if def.maxUnsealedRequeues != defaultMaxUnsealedRequeues {
		t.Fatalf("default maxUnsealedRequeues = %d, want %d", def.maxUnsealedRequeues, defaultMaxUnsealedRequeues)
	}
	if defaultMaxUnsealedRequeues >= defaultMaxRequeues {
		t.Fatalf("unsealed default (%d) must be smaller than the hard-crash requeue default (%d)", defaultMaxUnsealedRequeues, defaultMaxRequeues)
	}
	p := recoveryPolicyFromWorkflow(map[string]any{
		"recovery_policy": map[string]any{"max_unsealed_requeues": float64(0)},
	})
	if p.maxUnsealedRequeues != 0 {
		t.Fatalf("override maxUnsealedRequeues = %d, want 0", p.maxUnsealedRequeues)
	}
	neg := recoveryPolicyFromWorkflow(map[string]any{
		"recovery_policy": map[string]any{"max_unsealed_requeues": float64(-3)},
	})
	if neg.maxUnsealedRequeues != 0 {
		t.Fatalf("negative override clamps to 0, got %d", neg.maxUnsealedRequeues)
	}
}

// #289 unit: the escalation remediation is class-specific — the unsealed class
// points the operator at the worktree (the deliverable may already be there).
func TestSuggestedOperatorActionsUnsealed(t *testing.T) {
	unsealed := fmt.Sprint(suggestedOperatorActions(stallClassAgentExitedUnsealed))
	if !strings.Contains(unsealed, "complete-but-unsealed") && !strings.Contains(unsealed, "before work.complete") {
		t.Fatalf("unsealed remediation should mention the unsealed deliverable; got %s", unsealed)
	}
	hard := fmt.Sprint(suggestedOperatorActions(stallClassAgentPIDDead))
	if !strings.Contains(hard, "write_scope") {
		t.Fatalf("hard-crash remediation should be the generic set; got %s", hard)
	}
	if unsealed == hard {
		t.Fatalf("unsealed and hard-crash remediations must differ")
	}
}

// jobLastStallClass reads the recorded stall class for a job's recovery row.
func jobLastStallClass(t *testing.T, ctx context.Context, runner any, repoID, jobID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT last_stall_class FROM striatumd.job_recovery_state
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		if isNoRows(err) {
			return ""
		}
		t.Fatalf("read last_stall_class: %v", err)
	}
	return fmt.Sprint(nullable(row["last_stall_class"]))
}

// makeConfirmedDeadActiveSessionWithOutput turns the stalled-active fixture into a
// CASE-3 confirmed-dead lane that LOOKS alive by protocol (so CASE 1/2 do not
// fire), with the unsealed-output activity signature: a finished tool call + PTY
// output, but no work.complete. It seeds a supervisor pointer and stubs the
// liveness probe to report the agent process dead.
func makeConfirmedDeadActiveSessionWithOutput(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID string, withOutput bool) {
	t.Helper()
	now := time.Now().UTC()
	// Fresh protocol/heartbeat signals so the session is neither dead nor stalled
	// (forces the default branch where supervisedAgentConfirmedDead is the signal).
	var tool, pty any
	if withOutput {
		tool = now
		pty = now
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET registered_at = $3, last_mcp_request_at = $3,
		       last_session_heartbeat_at = $3, last_work_heartbeat_at = $3,
		       last_tool_call_started_at = $4, last_tool_call_finished_at = $4,
		       last_pty_activity_at = $5, last_work_complete_at = NULL
		 WHERE repository_id = $1 AND session_id = $2`,
		repoID, sessionID, now, tool, pty); err != nil {
		t.Fatalf("set confirmed-dead-active activity: %v", err)
	}
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,4242,'','attached',$6,'{}'::jsonb)`,
		repoID, supID, "dsup_"+sessionID, runID, sessionID, now); err != nil {
		t.Fatalf("seed supervisor pointer: %v", err)
	}
}

func stubDeadProbe(t *testing.T) {
	t.Helper()
	restore := probeLaneLiveness
	probeLaneLiveness = func(pctx context.Context, metadata map[string]any, pid int, expectedStart string) gosupervisor.LaneLiveness {
		return gosupervisor.LaneLiveness{Backed: "plain_pty", Alive: false, Class: "pid_gone", ObservedPID: pid}
	}
	t.Cleanup(func() { probeLaneLiveness = restore })
}

// #289 integration: a confirmed-dead agent that produced output but never sealed
// is classified agent_exited_unsealed (distinct from agent_pid_dead), requeued on
// the first sweep.
func TestSweepClassifiesUnsealedExitDistinctly(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_unsealed_class"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoID, runID, sessionID, true)
	stubDeadProbe(t)

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 1 {
		t.Fatalf("acted_count = %v, want 1 (unsealed dead agent requeued once); %#v", summary["acted_count"], summary)
	}
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassAgentExitedUnsealed {
		t.Fatalf("last_stall_class = %q, want %q", got, stallClassAgentExitedUnsealed)
	}
	requeue, _, escalation := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeue != 1 {
		t.Fatalf("requeue_count = %d, want 1", requeue)
	}
	if escalation {
		t.Fatalf("escalation_pending = true, want false on the first unsealed requeue")
	}
}

// #289 integration: the unsealed class escalates after a SMALLER budget than a
// hard crash. At requeue_count=1 the unsealed class (limit 1) escalates, while a
// hard agent_pid_dead (limit 2) would still requeue — proving the budget
// divergence — and the escalation payload carries the inspect-the-worktree
// remediation.
func TestSweepUnsealedExitEscalatesOnSmallerBudget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	// Unsealed: at count 1 (== default maxUnsealedRequeues) the sweep escalates.
	repoU := "repo_sweep_unsealed_budget"
	runU, jobU, _, _, sessU := seedStalledSessionActiveLane(t, ctx, runner, repoU)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoU, runU, sessU, true)
	stubDeadProbe(t)
	preseedRequeueBudget(t, ctx, runner, repoU, runU, jobU, 1)

	resU, err := SweepRun(ctx, runner, repoU, runU, "")
	if err != nil {
		t.Fatalf("unsealed sweep: %v", err)
	}
	sumU := recoveryActionsFromSweep(t, resU)
	if intValue(sumU["escalation_pending_count"]) != 1 {
		t.Fatalf("unsealed escalation_pending_count = %v, want 1 (limit 1 reached); %#v", sumU["escalation_pending_count"], sumU)
	}
	if got := jobState(t, ctx, runner, repoU, jobU); got != "running" {
		t.Fatalf("unsealed job state = %q, want running (escalated, not requeued)", got)
	}

	// Hard crash: same count 1, but limit 2 -> still requeues (NOT escalated).
	repoH := "repo_sweep_hardcrash_budget"
	runH, jobH, _, _, sessH := seedStalledSessionActiveLane(t, ctx, runner, repoH)
	makeConfirmedDeadActiveSessionWithOutput(t, ctx, runner, repoH, runH, sessH, false)
	preseedRequeueBudget(t, ctx, runner, repoH, runH, jobH, 1)

	resH, err := SweepRun(ctx, runner, repoH, runH, "")
	if err != nil {
		t.Fatalf("hard-crash sweep: %v", err)
	}
	sumH := recoveryActionsFromSweep(t, resH)
	if intValue(sumH["acted_count"]) != 1 {
		t.Fatalf("hard-crash acted_count = %v, want 1 (limit 2 not yet reached at count 1); %#v", sumH["acted_count"], sumH)
	}
	if got := jobLastStallClass(t, ctx, runner, repoH, jobH); got != stallClassAgentPIDDead {
		t.Fatalf("hard-crash last_stall_class = %q, want %q", got, stallClassAgentPIDDead)
	}

	// The unsealed escalation payload carries the distinct remediation.
	row, err := oneRow(ctx, runner, `
		SELECT payload_json::text AS payload FROM striatumd.escalation_inbox
		 WHERE repository_id = $1 AND run_id = $2`, repoU, runU)
	if err != nil {
		t.Fatalf("read escalation payload: %v", err)
	}
	payload := fmt.Sprint(row["payload"])
	if !strings.Contains(payload, stallClassAgentExitedUnsealed) {
		t.Fatalf("escalation payload missing the unsealed stall class: %s", payload)
	}
	if !strings.Contains(payload, "complete-but-unsealed") {
		t.Fatalf("escalation payload missing inspect-the-worktree remediation: %s", payload)
	}
}

// #289 integration (finding #1 lock): a confirmed-dead agent whose activity has
// AGED past the liveness deadlines — so Classify would call it ProtocolStalled —
// must still be handled as a dead lane (requeue_same_attempt + unsealed class),
// NOT mis-routed to the CASE 2 stalled-but-alive transfer. A dead agent's
// timestamps freeze at death, so a delayed sweep would otherwise transfer it on
// the wrong (larger) budget with no inspect-the-worktree remediation.
func TestSweepConfirmedDeadStalledRoutesToUnsealedNotTransfer(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_dead_stalled_unsealed"
	runID, jobID, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)

	// AGED output signature: a tool call + PTY output, but well past every liveness
	// deadline (so protocol classifies stalled), and no work.complete.
	aged := time.Now().UTC().Add(-30 * time.Minute)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET last_tool_call_started_at = $3, last_tool_call_finished_at = $3,
		       last_pty_activity_at = $3, last_work_complete_at = NULL
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID, aged); err != nil {
		t.Fatalf("set aged unsealed activity: %v", err)
	}
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,4242,'','attached',$6,'{}'::jsonb)`,
		repoID, supID, "dsup_"+sessionID, runID, sessionID, time.Now().UTC()); err != nil {
		t.Fatalf("seed supervisor pointer: %v", err)
	}
	stubDeadProbe(t)

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	acts, _ := summary["actions"].([]map[string]any)
	if len(acts) != 1 || acts[0]["action"] != "requeue_same_attempt" {
		t.Fatalf("expected one requeue_same_attempt (dead lane), got %#v", summary["actions"])
	}
	if got := jobLastStallClass(t, ctx, runner, repoID, jobID); got != stallClassAgentExitedUnsealed {
		t.Fatalf("last_stall_class = %q, want %q (must not be a generic stalled transfer)", got, stallClassAgentExitedUnsealed)
	}
	requeue, transfer, _ := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeue != 1 || transfer != 0 {
		t.Fatalf("budgets requeue=%d transfer=%d, want requeue=1 transfer=0 (dead-lane path, not transfer)", requeue, transfer)
	}
}

func preseedRequeueBudget(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID string, count int) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id, requeue_count, last_recovery_action,
		  last_recovery_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'requeue_same_attempt',$5,$5,$5)`,
		repoID, runID, jobID, count, now); err != nil {
		t.Fatalf("preseed requeue budget: %v", err)
	}
}
