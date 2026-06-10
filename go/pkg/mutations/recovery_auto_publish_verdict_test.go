package mutations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestSweepAutoPublishReviewRecordsVerdictAndFiresDownstream guards #144: when the
// stale-lease auto-publish pass completes a REVIEW job from its on-disk finding
// artifact, it must record the finding's verdict (accept) so the verdict-gated
// --accepted review--> downstream edge fires. Before the fix the job completed with
// NO verdict row and the downstream findings_ledger stayed blocked forever — the
// run wedged with every job green.
func TestSweepAutoPublishReviewRecordsVerdictAndFiresDownstream(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_autopub_review_verdict"
	runID := "run_" + repoID
	reviewJobID := "job_review_" + repoID
	downstreamJobID := "job_ledger_" + repoID
	leaseID := "lease_review_" + repoID
	msgID := "msg_review_" + repoID
	sessionID := "sess_review_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}, "synthesizer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs": []any{
			map[string]any{"id": "review_job", "type": "review", "role_id": "reviewer"},
			map[string]any{"id": "findings_ledger", "type": "synthesis", "role_id": "synthesizer"},
		},
	})

	// Point the run at a temp repo root and write the finding artifact on disk. An
	// unattested session's expected byline is `author: operator`, so the title block
	// uses that; the front matter declares verdict_intent: accept.
	repoRoot := t.TempDir()
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET repo_root = $2 WHERE repository_id = $1 AND run_id = $3`,
		repoID, repoRoot, runID); err != nil {
		t.Fatalf("set repo_root: %v", err)
	}
	findingPath := "reviews/finding.md"
	finding := `---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept
---
author: operator

The change is correct; accepting.
`
	abs := filepath.Join(repoRoot, findingPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(finding), 0o644); err != nil {
		t.Fatalf("write finding: %v", err)
	}

	// An UNATTESTED reviewer session (ordinal 1, no operator_label, no attestation)
	// => expectedAuthorLine == "author: operator", matching the finding byline.
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "reviewer", "claude", []string{"review"}, "active")

	now := time.Now().UTC()
	expArt, err := db.JSONBArg(runner, []any{map[string]any{
		"logical_name": "finding", "kind": "finding", "path": findingPath, "required": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	// The review job is running with the finding declared REQUIRED, holding a stale
	// (expired) lease: the reviewer wrote its finding then stalled before recording
	// the verdict.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'review_job',1,'running','reviewer','Review','review',
		          'idem_review_'||$1,$4::jsonb,'{"mode":"document_only","allowed_paths":["reviews"]}'::jsonb,
		          '{"lane_id":"claude"}'::jsonb,$5,$6,$7,$7)`,
		repoID, reviewJobID, runID, expArt, msgID, leaseID, now); err != nil {
		t.Fatalf("insert review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'reviewer','claude',$5,$5)`,
		repoID, msgID, runID, reviewJobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'expired',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes',NOW() - INTERVAL '30 minutes','lease_expired')`,
		repoID, leaseID, runID, reviewJobID, sessionID); err != nil {
		t.Fatalf("insert stale lease: %v", err)
	}
	// The downstream findings_ledger job is BLOCKED on the review behind a verdict gate.
	gate, err := db.JSONBArg(runner, map[string]any{"requires_verdict": []any{"accept", "accept_with_findings"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,'findings_ledger',1,'blocked','synthesizer','Ledger','synthesis',
		          'idem_ledger_'||$1,'[]'::jsonb,'{}'::jsonb,'{"lane_id":"claude"}'::jsonb,$4)`,
		repoID, downstreamJobID, runID, now); err != nil {
		t.Fatalf("insert downstream job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id, gate_json)
		VALUES ($1,$2,$3,$4::jsonb)`, repoID, downstreamJobID, reviewJobID, gate); err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The review job is completed (auto-published).
	if got := jobState(t, ctx, runner, repoID, reviewJobID); got != "completed" {
		t.Fatalf("review job state = %q, want completed (auto-published)", got)
	}
	// The #144 fix: a verdict row was recorded from the on-disk finding's verdict_intent.
	v, err := latestVerdict(ctx, runner, repoID, reviewJobID)
	if err != nil {
		t.Fatalf("latest verdict: %v", err)
	}
	if v != "accept" {
		t.Fatalf("recorded verdict = %q, want accept (auto-published review must record its on-disk verdict_intent)", v)
	}
	// The verdict-gated downstream edge fired: the findings_ledger is no longer blocked.
	if got := jobState(t, ctx, runner, repoID, downstreamJobID); got == "blocked" {
		t.Fatalf("downstream findings_ledger still blocked — the --accepted review--> edge did not fire (the #144 wedge)")
	}
	// RFC 0118 P0-2 classification: the #144 sweep is a lane-evidence
	// TRANSCRIPTION (the stalled lane authored the verdict_intent; recovery
	// merely records it), so its stamp is the truthful unattested state with NO
	// override basis — the run-completion gate fail-closes it on
	// provenance-required reviews and passes it on ordinary ones.
	stamp, err := oneRow(ctx, runner, `
		SELECT lane_attestation_at_record, review_provenance_override
		  FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2`, repoID, reviewJobID)
	if err != nil {
		t.Fatalf("read sweep verdict stamp: %v", err)
	}
	if got := fmt.Sprint(stamp["lane_attestation_at_record"]); got != "unattested" {
		t.Fatalf("lane_attestation_at_record = %q, want unattested (stale lane, truthful stamp)", got)
	}
	if stamp["review_provenance_override"] != false {
		t.Fatalf("review_provenance_override = %v, want false (transcription, not an operator override)", stamp["review_provenance_override"])
	}
}
