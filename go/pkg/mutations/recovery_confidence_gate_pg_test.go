package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedSilentPipeLaneJob seeds the RFC 0131 #311 incident shape: a STILL-ACTIVE
// pipe-transport lane (agy/Gemini, no pty_helper oracle) whose owning session is
// silent past the protocol-idle deadline, so the classifier reports ProtocolStalled
// with a deadline_elapsed_only basis (no confirmed-dead oracle). The job holds no
// active lease (the silent lane's lease lapsed), so the decision tree's CASE-2
// transfer path applies. The transfer budget is preseeded at max so the next sweep
// is the over-budget escalation candidate that the 131-C confidence gate must
// debounce rather than immediately escalate. A pipe supervisor pointer makes
// TransportFromRow report `pipe`.
func seedSilentPipeLaneJob(t *testing.T, ctx context.Context, runner db.Runner, repoID string, preseedTransfers int) (runID, jobID, sessionID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_review_" + repoID
	sessionID = "sess_pipe_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}},
		"lanes":       map[string]any{"agy": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "review", "type": "review", "role_id": "reviewer"}},
	})
	// The owning session is still ACTIVE (the pipe lane did not close itself) —
	// this is the crux: it is not a dead-lane CASE-1, it is a silent-but-present
	// lane the confidence gate must not over-eagerly escalate.
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "reviewer", "agy", []string{"write"}, "active")

	now := time.Now().UTC()
	// Age the session past the protocol-idle deadline: registered long ago and the
	// only recorded activity (its last MCP request) is old, so Classify reports
	// ProtocolStalled. No PTY activity (it is a pipe lane).
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET registered_at = NOW() - INTERVAL '30 minutes',
		       last_mcp_request_at = NOW() - INTERVAL '20 minutes'
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID); err != nil {
		t.Fatalf("age session activity: %v", err)
	}

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, created_at, started_at
		) VALUES ($1,$2,$3,'review',1,'running','reviewer','Review','review',
		          'idem_review_'||$1,'[]'::jsonb,'{}'::jsonb,'{"lane_id":"agy"}'::jsonb,
		          NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes')`,
		repoID, jobID, runID); err != nil {
		t.Fatalf("insert silent pipe-lane job: %v", err)
	}
	// A released lease owned by the (still-active) session: no ACTIVE lease, so the
	// CASE-2 protocol-idle transfer path applies (leaseClearedOrGone).
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',
		          NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '25 minutes',
		          NOW() - INTERVAL '25 minutes','lapsed')`,
		repoID, "lease_pipe_"+repoID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}
	// A PIPE supervisor pointer so TransportFromRow reports `pipe` (no PTY oracle).
	// 'attached' with no live PID makes supervisedAgentConfirmedDead indeterminate
	// (not confirmed dead), so the lane stays the CASE-2 silent-stall path.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,'','attached',$7,'{"transport":"pipe"}'::jsonb)`,
		repoID, "sup_"+sessionID, "dsup_"+sessionID, runID, sessionID, 0, now); err != nil {
		t.Fatalf("insert pipe supervisor pointer: %v", err)
	}
	// Preseed the transfer budget at max so the next sweep is over-budget.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id, transfer_count, last_recovery_action,
		  last_recovery_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'transfer_requeue',
		          NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '10 minutes')`,
		repoID, runID, jobID, preseedTransfers); err != nil {
		t.Fatalf("preseed transfer budget: %v", err)
	}
	return runID, jobID, sessionID
}

func gateColumns(t *testing.T, ctx context.Context, runner any, repoID, jobID string) (silentSweeps, misfire int, lastBasis string) {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT consecutive_silent_sweeps, misfire_evidence_score, last_probe_basis
		  FROM striatumd.job_recovery_state
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("read gate columns: %v", err)
	}
	return intFromAny(row["consecutive_silent_sweeps"], -1),
		intFromAny(row["misfire_evidence_score"], -1),
		fmt.Sprint(nullable(row["last_probe_basis"]))
}

// TestConfidenceGateDebouncesThenEscalatesSilentPipeLane is the RFC 0131 131-C
// end-to-end gate: a budget-exhausted, still-active, silent PIPE lane on a
// deadline_elapsed_only basis is DEBOUNCED on the first over-budget sweep (the run
// stays running, a recovery.escalation_debounced event is written, the silent-sweep
// counter advances) and ESCALATES on the second consecutive silent sweep (the
// two-sweep bar clears → run flips to needs_operator). This is the #311 misfire
// the gate exists to prevent: a single elapsed deadline on a no-oracle pipe lane
// must not discard the run.
func TestConfidenceGateDebouncesThenEscalatesSilentPipeLane(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_131c_gate_debounce"
	runID, jobID, _ := seedSilentPipeLaneJob(t, ctx, runner, repoID, defaultMaxTransfers)

	if got := runStateOf(t, ctx, runner, repoID, runID); got != "running" {
		t.Fatalf("pre-sweep run state = %q, want running", got)
	}

	// Sweep 1: DEBOUNCE. The over-budget deadline_elapsed_only pipe lane must NOT
	// escalate on a single sweep.
	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep 1: %v", err)
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "running" {
		t.Fatalf("after sweep 1 run state = %q, want running (debounced, NOT escalated)", got)
	}
	if n := eventCount(t, ctx, runner, repoID, runID, "recovery.escalation_debounced"); n != 1 {
		t.Fatalf("recovery.escalation_debounced count after sweep 1 = %d, want 1", n)
	}
	if n := eventCount(t, ctx, runner, repoID, runID, "recovery.budget_exhausted"); n != 0 {
		t.Fatalf("recovery.budget_exhausted count after sweep 1 = %d, want 0 (debounced, not escalated)", n)
	}
	silent, misfire, basis := gateColumns(t, ctx, runner, repoID, jobID)
	if silent != 1 || misfire != 1 {
		t.Fatalf("after sweep 1 gate columns = silent:%d misfire:%d, want 1/1", silent, misfire)
	}
	if basis != "deadline_elapsed_only" {
		t.Fatalf("after sweep 1 last_probe_basis = %q, want deadline_elapsed_only", basis)
	}

	// Sweep 2: ESCALATE via the two-consecutive-sweep bar. The prior sweep recorded
	// deadline_elapsed_only, so the second silent sweep clears the bar.
	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep 2: %v", err)
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "needs_operator" {
		t.Fatalf("after sweep 2 run state = %q, want needs_operator (two-sweep bar cleared)", got)
	}
	if n := eventCount(t, ctx, runner, repoID, runID, "recovery.budget_exhausted"); n != 1 {
		t.Fatalf("recovery.budget_exhausted count after sweep 2 = %d, want 1 (escalated)", n)
	}
	// The escalation event records WHY it fired (confidence_gated, the two-sweep
	// bar — not the cap).
	ev, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'recovery.budget_exhausted'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read budget_exhausted event: %v", err)
	}
	payload := asMap(ev["payload_json"])
	if payload["confidence_gated"] != true {
		t.Fatalf("escalation event must record confidence_gated=true; %#v", payload)
	}
	if payload["cap_fired"] != false {
		t.Fatalf("the two-sweep-bar escalation must record cap_fired=false; %#v", payload)
	}
	if fmt.Sprint(payload["probe_basis"]) != "deadline_elapsed_only" {
		t.Fatalf("escalation probe_basis = %v, want deadline_elapsed_only", payload["probe_basis"])
	}
}

// TestConfidenceGateResetByForgeryResistantProgress is the #311 misfire replayed:
// a silent pipe lane that had accrued a silent sweep but then SEALED real progress
// (a published artifact — daemon-recorded sealed work a dead loop cannot forge) is
// RESET and deferred, not escalated. Proves the forgery-resistant reset signal.
func TestConfidenceGateResetByForgeryResistantProgress(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_131c_gate_reset"
	runID, jobID, sessionID := seedSilentPipeLaneJob(t, ctx, runner, repoID, defaultMaxTransfers)

	// Sweep 1: debounce, recording 1 silent sweep + last_probe_basis.
	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep 1: %v", err)
	}
	if silent, _, _ := gateColumns(t, ctx, runner, repoID, jobID); silent != 1 {
		t.Fatalf("after sweep 1 silent sweeps = %d, want 1", silent)
	}

	// Now the lane seals real forgery-resistant progress: a published artifact whose
	// created_at is AFTER the prior sweep's last_recovery_at.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode, created_at
		) VALUES ($1,$2,$3,$4,$5,'review_finding','finding','findings/review.md',
		          'deadbeef',42,'create',NOW())`,
		repoID, "art_"+jobID, runID, jobID, sessionID); err != nil {
		t.Fatalf("seal artifact progress: %v", err)
	}

	// Sweep 2: the gate sees forgery-resistant progress and RESETS — the run stays
	// running and the silent-sweep counter resets to 0.
	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep 2: %v", err)
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "running" {
		t.Fatalf("after progress-reset sweep run state = %q, want running (reset, NOT escalated)", got)
	}
	silent, misfire, _ := gateColumns(t, ctx, runner, repoID, jobID)
	if silent != 0 || misfire != 0 {
		t.Fatalf("after progress reset gate columns = silent:%d misfire:%d, want 0/0", silent, misfire)
	}
	// A second debounce event was written for the reset sweep (progress_reset=true).
	if n := eventCount(t, ctx, runner, repoID, runID, "recovery.escalation_debounced"); n != 2 {
		t.Fatalf("recovery.escalation_debounced count = %d, want 2", n)
	}
}
