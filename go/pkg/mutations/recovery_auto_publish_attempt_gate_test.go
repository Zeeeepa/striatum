package mutations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestSweepAutoPublishRefusesPriorAttemptArtifact guards #203 (RFC 0095 Goal 2):
// the stale-lease auto-publish pass must NOT complete a re-opened revision-cycle
// job from the PRIOR attempt's artifact. In run_7bc59e7e (the issue's repro) a
// synth job routed for revision cycle 2 (attempt 2) had only the cycle-1
// document on disk — byte-identical to the attempt-1 artifact the reviewer
// rejected. Recovery auto-published it as the "revision", silently discarding
// the needs_revision verdict. The attempt-gate refuses to credit a job from an
// on-disk file that is byte-identical to one of the job's lower-attempt
// artifacts; a fresh lane must perform the real revision.
func TestSweepAutoPublishRefusesPriorAttemptArtifact(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_autopub_attempt_gate"
	runID := "run_" + repoID
	synthJobID := "job_synth_" + repoID
	leaseID := "lease_synth_" + repoID
	msgID := "msg_synth_" + repoID
	sessionID := "sess_synth_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"synthesizer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "synth", "type": "synthesis", "role_id": "synthesizer"}},
	})

	// Point the run at a temp repo root and write the (pre-revision) synthesis on
	// disk. An unattested session's expected byline is `author: operator`.
	repoRoot := t.TempDir()
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET repo_root = $2 WHERE repository_id = $1 AND run_id = $3`,
		repoID, repoRoot, runID); err != nil {
		t.Fatalf("set repo_root: %v", err)
	}
	synthPath := "docs/synthesis.md"
	synthBody := `---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
---
author: operator

The original synthesis the reviewer rejected; no revision happened.
`
	abs := filepath.Join(repoRoot, synthPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(synthBody), 0o644); err != nil {
		t.Fatalf("write synthesis: %v", err)
	}
	digest := sha256Hex([]byte(synthBody))

	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "synthesizer", "claude", []string{"synthesis"}, "active")

	now := time.Now().UTC()
	expArt, err := db.JSONBArg(runner, []any{map[string]any{
		"logical_name": "synthesis", "kind": "synthesis", "path": synthPath, "required": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	// The synth job is at attempt 2 (re-opened once by the revision cycle),
	// running, holding a stale (expired) lease.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'synth',2,'running','synthesizer','Synthesis','synthesis',
		          'idem_synth_'||$1,$4::jsonb,'{"mode":"document_only","allowed_paths":["docs"]}'::jsonb,
		          '{"lane_id":"claude"}'::jsonb,$5,$6,$7,$7)`,
		repoID, synthJobID, runID, expArt, msgID, leaseID, now); err != nil {
		t.Fatalf("insert synth job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'synthesizer','claude',$5,$5)`,
		repoID, msgID, runID, synthJobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'expired',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes',NOW() - INTERVAL '30 minutes','expired')`,
		repoID, leaseID, runID, synthJobID, sessionID); err != nil {
		t.Fatalf("insert stale lease: %v", err)
	}
	// The PRIOR attempt's artifact (attempt 1) — byte-identical to the on-disk
	// file — is what the reviewer reviewed and rejected. Its presence is what the
	// attempt-gate keys on.
	publishArtifactRow(t, ctx, runner, repoID, runID, synthJobID, sessionID, "synthesis", "synthesis", synthPath, digest, now.Add(-time.Hour), 1)

	res, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The attempt-gate refused: the synth job must NOT be completed from the
	// pre-revision artifact.
	if got := jobState(t, ctx, runner, repoID, synthJobID); got == "completed" {
		t.Fatalf("synth job state = completed — recovery auto-published the PRE-revision attempt-1 artifact for an attempt-2 revision job (#203 regression)")
	}

	// No attempt-2 artifact row was created (the stale file was not credited).
	rows := artifactRowsForLogical(t, ctx, runner, repoID, synthJobID, "synthesis")
	for _, row := range rows {
		if intValue(row["attempt"]) >= 2 {
			t.Fatalf("an attempt-2 artifact row was published from the stale prior-attempt file (#203 regression): %#v", row)
		}
	}

	// The refusal is legible: a recovery.auto_publish_refused event was recorded.
	evtRow, err := oneRow(ctx, runner, `
		SELECT count(*) AS n FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'recovery.auto_publish_refused'`,
		repoID, runID)
	if err != nil {
		t.Fatalf("count refusal events: %v", err)
	}
	if intValue(evtRow["n"]) < 1 {
		t.Fatalf("no recovery.auto_publish_refused event recorded; the attempt-gate refusal must be observable in the run history")
	}
	_ = res
}

// TestSweepAutoPublishExcludesRequeuedExpiredLease guards #203 layer (a): the
// auto-publish candidate scan's expired-lease disjunct (both the join and the
// WHERE) must exclude a lease that recovery already requeued/transferred away
// (release_reason in recovery_requeue / recovery_transfer / operator_transfer).
// Otherwise that historical lease re-matches and the job — whose work has since
// moved to a fresh attempt/lease — is auto-published from the prior round's
// on-disk file. This mirrors the #179 exclusion now shared via
// db.ExpiredLeaseStillStalePredicate.
func TestSweepAutoPublishExcludesRequeuedExpiredLease(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_autopub_requeued_lease"
	runID := "run_" + repoID
	synthJobID := "job_synth_" + repoID
	leaseID := "lease_synth_" + repoID
	msgID := "msg_synth_" + repoID
	sessionID := "sess_synth_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"synthesizer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "synth", "type": "synthesis", "role_id": "synthesizer"}},
	})

	repoRoot := t.TempDir()
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET repo_root = $2 WHERE repository_id = $1 AND run_id = $3`,
		repoID, repoRoot, runID); err != nil {
		t.Fatalf("set repo_root: %v", err)
	}
	synthPath := "docs/synthesis.md"
	synthBody := `---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
---
author: operator

A fresh-attempt document on disk; the requeued historical lease must NOT credit it.
`
	abs := filepath.Join(repoRoot, synthPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(synthBody), 0o644); err != nil {
		t.Fatalf("write synthesis: %v", err)
	}

	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "synthesizer", "claude", []string{"synthesis"}, "active")

	now := time.Now().UTC()
	expArt, err := db.JSONBArg(runner, []any{map[string]any{
		"logical_name": "synthesis", "kind": "synthesis", "path": synthPath, "required": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	// The job is stale_lease with a CURRENT message but its prior lease was
	// already released by recovery with release_reason='recovery_requeue'. The
	// candidate scan must skip this lease entirely.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'synth',1,'stale_lease','synthesizer','Synthesis','synthesis',
		          'idem_synth_'||$1,$4::jsonb,'{"mode":"document_only","allowed_paths":["docs"]}'::jsonb,
		          '{"lane_id":"claude"}'::jsonb,$5,NULL,$6,$6)`,
		repoID, synthJobID, runID, expArt, msgID, now); err != nil {
		t.Fatalf("insert synth job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,'synthesizer','claude',$5,$5)`,
		repoID, msgID, runID, synthJobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'expired',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes',NOW() - INTERVAL '30 minutes','recovery_requeue')`,
		repoID, leaseID, runID, synthJobID, sessionID); err != nil {
		t.Fatalf("insert requeued lease: %v", err)
	}

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The job must NOT be auto-published/completed from the requeued historical
	// lease — its work belongs to a fresh attempt the requeue already opened.
	if got := jobState(t, ctx, runner, repoID, synthJobID); got == "completed" {
		t.Fatalf("synth job state = completed — the auto-publish scan matched an already-requeued (recovery_requeue) expired lease (#203 layer-a regression)")
	}
}
