package reads

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// seedRecoveryGateRepo seeds a repo + run + a job_recovery_state row with the
// given confidence-gate state, for the RFC 0131 131-D (#337) escape-valve doctor
// invariant and the run.summary recovery_gate legibility block.
func seedRecoveryGateRepo(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID string, runState string, silentSweeps int, escalationPending bool) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,35,'active')
		ON CONFLICT (repository_id) DO NOTHING`,
		repoID, "ident_"+repoID, "/tmp/"+repoID, "/tmp/"+repoID+"/.striatum", now); err != nil {
		t.Fatalf("insert repository %s: %v", repoID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha','{}'::jsonb,$3)
		ON CONFLICT (repository_id, workflow_snapshot_id) DO NOTHING`,
		repoID, "snap_"+repoID, now); err != nil {
		t.Fatalf("insert workflow snapshot %s: %v", repoID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at, started_at
		) VALUES ($1,$2,$3,$4,$5,$6,$6)`,
		repoID, runID, "snap_"+repoID, "/tmp/"+repoID, runState, now); err != nil {
		t.Fatalf("insert run %s: %v", runID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,'review',1,'running','reviewer','Review','review',
		          'idem_'||$2,'[]'::jsonb,'{}'::jsonb,'{}'::jsonb,$4)`,
		repoID, jobID, runID, now); err != nil {
		t.Fatalf("insert job %s: %v", jobID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id, transfer_count,
		  consecutive_silent_sweeps, misfire_evidence_score, last_probe_basis,
		  escalation_pending, last_recovery_action, last_recovery_at, created_at, updated_at
		) VALUES ($1,$2,$3,3,$4,$5,'deadline_elapsed_only',$6,'transfer_requeue',$7,$7,$7)`,
		repoID, runID, jobID, silentSweeps, silentSweeps, escalationPending, now); err != nil {
		t.Fatalf("insert job_recovery_state %s: %v", jobID, err)
	}
}

// TestDoctorFlagsUncappedEscapeValveBreach asserts the RFC 0131 131-D escape-valve
// safety-floor invariant: a job at/over its silent-sweep cap (default 7) that is
// NOT escalation_pending on a still-running run is a hard RED breach (the cap must
// always eventually fire — never an un-escalatable lane).
func TestDoctorFlagsUncappedEscapeValveBreach(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_131d_escape_valve_breach"
	// 8 silent sweeps >= default cap 7 (maxRequeues default 2 → (2*2)+3), not escalated.
	seedRecoveryGateRepo(t, ctx, runner, repoID, "run_breach", "job_breach", "running", 8, false)

	block, problems, records := doctorRecoveryGateIntegrity(ctx, runner, repoID)
	if block["checked"] != true {
		t.Fatalf("block not checked: %#v", block)
	}
	if block["breach_count"] != 1 {
		t.Fatalf("breach_count = %v, want 1; block=%#v", block["breach_count"], block)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "recovery_escape_valve_uncapped.job_breach") {
		t.Fatalf("problems = %#v, want the uncapped breach", problems)
	}
	if len(records) != 1 || records[0]["silent_sweep_cap"] != 7 {
		t.Fatalf("records = %#v, want one record with cap 7", records)
	}

	// Full doctor must red.
	result, err := HandleDoctor(ctx, runner, rpc.Envelope{Params: map[string]any{"repository_id": repoID, "verbose": true}})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	if result["ok"] == true {
		t.Fatalf("doctor ok = true, want false on an escape-valve breach")
	}
	gateBlock := result["recovery_escape_valve"].(map[string]any)
	if gateBlock["breach_count"] != 1 {
		t.Fatalf("recovery_escape_valve block = %#v, want one breach", gateBlock)
	}
}

// TestDoctorRecoveryGateQuietWhenWithinCapOrEscalated asserts the invariant does
// NOT false-positive: a job below its cap, an already-escalation_pending job at the
// cap, and a terminal run are all healthy and produce no breach.
func TestDoctorRecoveryGateQuietWhenWithinCapOrEscalated(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_131d_escape_valve_quiet"
	// Below cap (3 < 7), not escalated: healthy debounce.
	seedRecoveryGateRepo(t, ctx, runner, repoID, "run_below", "job_below", "running", 3, false)
	// At/over cap (9 >= 7) BUT escalation_pending=true: the cap fired correctly.
	seedRecoveryGateRepo(t, ctx, runner, repoID, "run_escalated", "job_escalated", "running", 9, true)
	// Over cap (10) but the run is terminal: past the recovery sweep, not a breach.
	seedRecoveryGateRepo(t, ctx, runner, repoID, "run_terminal", "job_terminal", "completed", 10, false)

	block, problems, records := doctorRecoveryGateIntegrity(ctx, runner, repoID)
	if len(problems) != 0 || len(records) != 0 {
		t.Fatalf("healthy gate state produced problems=%#v records=%#v", problems, records)
	}
	if block["breach_count"] != 0 {
		t.Fatalf("breach_count = %v, want 0; block=%#v", block["breach_count"], block)
	}
}

// TestRecoveryGateSummaryProjectsGateState asserts the RFC 0131 131-D run.summary
// escalation-decision legibility block: a debounced lane (silent sweeps, not yet
// escalated) and an escalated lane are both projected with their gate state.
func TestRecoveryGateSummaryProjectsGateState(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_131d_gate_summary"
	runID := "run_summary"
	seedRecoveryGateRepo(t, ctx, runner, repoID, runID, "job_debounced", "running", 2, false)
	// A second job on the same run that already escalated.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, created_at
		) VALUES ($1,'job_escalated',$2,'build',1,'running','builder','Build','build',
		          'idem_build','[]'::jsonb,'{}'::jsonb,'{}'::jsonb, now())`,
		repoID, runID); err != nil {
		t.Fatalf("insert second job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id, consecutive_silent_sweeps,
		  misfire_evidence_score, last_probe_basis, escalation_pending,
		  last_recovery_action, last_recovery_at, created_at, updated_at
		) VALUES ($1,$2,'job_escalated',7,7,'deadline_elapsed_only',true,
		          'transfer_requeue_budget_exhausted', now(), now(), now())`,
		repoID, runID); err != nil {
		t.Fatalf("insert escalated gate state: %v", err)
	}

	summary := recoveryGateSummary(ctx, runner, repoID, runID)
	if summary["checked"] != true {
		t.Fatalf("summary not checked: %#v", summary)
	}
	if summary["debounced_count"] != 1 {
		t.Fatalf("debounced_count = %v, want 1; %#v", summary["debounced_count"], summary)
	}
	if summary["escalated_count"] != 1 {
		t.Fatalf("escalated_count = %v, want 1; %#v", summary["escalated_count"], summary)
	}
	gates := summary["gates"].([]map[string]any)
	if len(gates) != 2 {
		t.Fatalf("gates = %#v, want 2", gates)
	}
	states := map[string]string{}
	for _, g := range gates {
		states[g["job_id"].(string)] = g["gate_state"].(string)
	}
	if states["job_debounced"] != "debounced" || states["job_escalated"] != "escalated" {
		t.Fatalf("gate_state projection wrong: %#v", states)
	}
}
