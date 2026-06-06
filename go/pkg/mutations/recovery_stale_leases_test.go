package mutations

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// staleLeaseEntryIDs extracts the job_ids reported as stale by
// recovery.stale_leases.
func staleLeaseEntryIDs(t *testing.T, result map[string]any) []string {
	t.Helper()
	entries, ok := result["stale_leases"].([]map[string]any)
	if !ok {
		t.Fatalf("stale_leases missing or wrong type: %#v", result["stale_leases"])
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, asString(e["job_id"]))
	}
	return ids
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// TestRecoveryStaleLeasesExcludesReleasedTransferredLeases is the #179
// regression: recovery.stale_leases reported already-released historical leases
// as stale. A lease that recovery transferred/requeued away is stamped
// state='expired' with release_reason='recovery_transfer' (its job has since
// moved to a fresh lease/attempt); the bare `l.state = 'expired'` join kept
// matching it, so every later recovery.stale_leases call re-reported the same
// resolved lease as stale forever.
//
// This seeds two jobs in one run:
//   - job_transferred: moved on (state 'running', current_lease_id NULL) whose
//     only expired lease is a historical recovery_transfer release — must NOT be
//     reported stale.
//   - job_stale: genuinely stuck (state 'stale_lease') with an expired lease
//     released by the ordinary expiry path (release_reason='expired') — MUST
//     still be reported stale (over-filter guard).
func TestRecoveryStaleLeasesExcludesReleasedTransferredLeases(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_stale_lease_179"
	runID := "run_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs": []any{
			map[string]any{"id": "transferred", "type": "build", "role_id": "implementer"},
			map[string]any{"id": "stale", "type": "build", "role_id": "implementer"},
		},
	})

	// The owning session has since closed (the lane that held the leases is gone);
	// leases.owner_session_id is NOT NULL with an FK to sessions, so reference it.
	ownerSession := "sess_owner_" + repoID
	intgSeedSession(t, ctx, runner, repoID, runID, ownerSession, "implementer", "claude", []string{"write"}, "closed")

	now := time.Now().UTC()
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"src/"}})
	if err != nil {
		t.Fatal(err)
	}

	seedJob := func(jobID, workflowJobID, state string) {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
			  lane_selector_json, current_lease_id, created_at, started_at
			) VALUES ($1,$2,$3,$4,1,$5,'implementer','Implement','build',
			          'idem_'||$2,'[]'::jsonb,$6::jsonb,'{"lane_id":"claude"}'::jsonb,NULL,$7,$7)`,
			repoID, jobID, runID, workflowJobID, state, wsArg, now); err != nil {
			t.Fatalf("insert job %s: %v", jobID, err)
		}
	}

	// job_transferred: moved on, only an already-transferred expired lease remains.
	jobTransferred := "job_transferred_" + repoID
	seedJob(jobTransferred, "transferred", "running")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'expired',
		          NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour',
		          NOW() - INTERVAL '30 minutes','recovery_transfer')`,
		repoID, "lease_transferred_"+repoID, runID, jobTransferred, ownerSession); err != nil {
		t.Fatalf("insert transferred lease: %v", err)
	}

	// job_stale: genuinely stuck — stale_lease job + an expired lease released by
	// the ordinary expiry path.
	jobStale := "job_stale_" + repoID
	seedJob(jobStale, "stale", "stale_lease")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'expired',
		          NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour',
		          NOW() - INTERVAL '30 minutes','expired')`,
		repoID, "lease_stale_"+repoID, runID, jobStale, ownerSession); err != nil {
		t.Fatalf("insert stale lease: %v", err)
	}

	result, err := HandleRecoveryStaleLeases(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
	}))
	if err != nil {
		t.Fatalf("recovery.stale_leases: %v", err)
	}

	ids := staleLeaseEntryIDs(t, result)
	for _, id := range ids {
		if id == jobTransferred {
			t.Fatalf("#179: recovery.stale_leases reported the already-transferred "+
				"(release_reason='recovery_transfer') historical lease for %s as stale; "+
				"stale entries=%v", jobTransferred, ids)
		}
	}

	// Over-filter guard: the genuinely stale job must still be reported.
	foundStale := false
	for _, id := range ids {
		if id == jobStale {
			foundStale = true
		}
	}
	if !foundStale {
		t.Fatalf("genuinely stale job %s must still be reported; stale entries=%v", jobStale, ids)
	}

	if intValue(result["stale_count"]) != 1 {
		t.Fatalf("stale_count = %v, want 1 (only the genuinely stale job); entries=%v",
			result["stale_count"], ids)
	}
}
