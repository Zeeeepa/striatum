package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestClaimNextResultSurfacesPacketIDAndSuperviseSend(t *testing.T) {
	packet := map[string]any{"packet_id": "wp_1"}
	result := claimNextResult("sess_1", "wp_1", packet, false)

	if result["status"] != "claimed" {
		t.Fatalf("status = %v", result["status"])
	}
	if result["packet_id"] != "wp_1" {
		t.Fatalf("packet_id = %v", result["packet_id"])
	}
	if !reflect.DeepEqual(result["packet"], packet) {
		t.Fatalf("packet = %#v", result["packet"])
	}
	nextSteps := result["next_steps"].(map[string]any)
	if nextSteps["supervise_send"] != "striatum supervise send --session-id sess_1 --packet-id wp_1" {
		t.Fatalf("supervise_send = %v", nextSteps["supervise_send"])
	}
}

// Regression for #68: when a self-driving supervisor is attached, the
// supervise_send hint is misleading (the agent self-claims via
// work.await_packet) and must be suppressed in favor of a self-claim note.
func TestClaimNextResultSuppressesSuperviseSendForSelfDrivingSupervisor(t *testing.T) {
	packet := map[string]any{"packet_id": "wp_1"}
	result := claimNextResult("sess_1", "wp_1", packet, true)

	nextSteps := result["next_steps"].(map[string]any)
	if _, ok := nextSteps["supervise_send"]; ok {
		t.Fatalf("supervise_send must be suppressed for a self-driving supervisor: %#v", nextSteps)
	}
	note, _ := nextSteps["self_claim_note"].(string)
	if !strings.Contains(note, "work.await_packet") || !strings.Contains(note, "do not run") {
		t.Fatalf("self_claim_note = %q, want it to mention work.await_packet and not running supervise send", note)
	}
}

func TestAwaitPacketValidatesSessionID(t *testing.T) {
	_, err := HandleAwaitPacket(context.Background(), inertRunner{}, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_await",
		Method:        "work.await_packet",
		Params:        map[string]any{"repository_id": "repo_1"}, // missing session_id
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("err = %v, want schema_invalid", err)
	}
}

func TestPacketTaskPromptResolvesWorkflowLocalPath(t *testing.T) {
	got := packetTaskPrompt(
		map[string]any{"path": "prompts/demo.md"},
		map[string]any{"source_path": "docs/operator/workflows/demo/workflow.json"},
	)

	if got["path"] != "docs/operator/workflows/demo/prompts/demo.md" {
		t.Fatalf("path = %v", got["path"])
	}
	if got["workflow_relative_path"] != "prompts/demo.md" {
		t.Fatalf("workflow_relative_path = %v", got["workflow_relative_path"])
	}
	if got["workflow_source_path"] != "docs/operator/workflows/demo/workflow.json" {
		t.Fatalf("workflow_source_path = %v", got["workflow_source_path"])
	}
}

// Regression for #90: when the prompt path is already repo-relative (already
// prefixed with the workflow directory), packetTaskPrompt must NOT join the
// workflow dir again — that produced striatum/<wf>/striatum/<wf>/prompts/x.md.
func TestPacketTaskPromptDoesNotDuplicateWorkflowDir(t *testing.T) {
	got := packetTaskPrompt(
		map[string]any{"path": "striatum/demo/prompts/demo.md"},
		map[string]any{"source_path": "striatum/demo/workflow.json"},
	)

	if got["path"] != "striatum/demo/prompts/demo.md" {
		t.Fatalf("path = %v, want striatum/demo/prompts/demo.md (no duplication)", got["path"])
	}
	if strings.Contains(got["path"].(string), "striatum/demo/striatum/demo") {
		t.Fatalf("path duplicated the workflow dir: %v", got["path"])
	}
	if got["workflow_relative_path"] != "prompts/demo.md" {
		t.Fatalf("workflow_relative_path = %v, want prompts/demo.md", got["workflow_relative_path"])
	}
	if got["workflow_source_path"] != "striatum/demo/workflow.json" {
		t.Fatalf("workflow_source_path = %v", got["workflow_source_path"])
	}
}

func TestPacketTaskPromptLeavesRootRelativePath(t *testing.T) {
	got := packetTaskPrompt(
		map[string]any{"path": "prompts/demo.md"},
		map[string]any{"source_path": "workflow.json"},
	)

	if !reflect.DeepEqual(got, map[string]any{"path": "prompts/demo.md"}) {
		t.Fatalf("task prompt = %#v", got)
	}
}

func TestAugmentationReferencesInspectLocalCorpusBundle(t *testing.T) {
	repoRoot := t.TempDir()
	bundle := filepath.Join(repoRoot, "exports", "corpus")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"schema_version":           "striatum.corpus_export.v1",
		"corpus_contract_version":  2,
		"corpus_id":                "striatum:" + strings.Repeat("a", 64),
		"redaction_tier":           "public",
		"verification_depth":       "deep_chain",
		"bundle_sha256":            strings.Repeat("b", 64),
		"row_counts":               map[string]any{"rfc": float64(2)},
		"repo_root":                "/absolute/path/not-exposed",
		"incremental_export_token": "not surfaced",
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	workflow := map[string]any{
		"augmentation": map[string]any{
			"mode":                    "reference_only",
			"required":                false,
			"budget_per_packet_lines": 25,
			"sources": []any{
				map[string]any{
					"id":          "local-corpus",
					"kind":        "corpus_bundle",
					"path":        "exports/corpus",
					"description": "Local corpus bundle",
				},
				map[string]any{"id": "missing", "kind": "corpus_bundle", "path": "exports/missing"},
			},
			"jobs": []any{"draft"},
		},
	}

	got := augmentationReferences(workflow, "draft", repoRoot)

	if got["mode"] != "reference_only" || got["required"] != false {
		t.Fatalf("augmentation policy = %#v", got)
	}
	if got["budget_per_packet_lines"] != 25 {
		t.Fatalf("budget = %v", got["budget_per_packet_lines"])
	}
	sources := got["sources"].([]any)
	first := sources[0].(map[string]any)
	if first["status"] != "available" || first["available"] != true {
		t.Fatalf("first source = %#v", first)
	}
	summary := first["manifest"].(map[string]any)
	if summary["corpus_id"] != "striatum:"+strings.Repeat("a", 64) {
		t.Fatalf("summary = %#v", summary)
	}
	if _, ok := summary["repo_root"]; ok {
		t.Fatalf("absolute repo_root leaked into summary: %#v", summary)
	}
	second := sources[1].(map[string]any)
	if second["status"] != "missing" || second["reason"] != "bundle_not_found" {
		t.Fatalf("second source = %#v", second)
	}
}

func TestAugmentationReferencesOmittedForNonOptedInJob(t *testing.T) {
	workflow := map[string]any{
		"augmentation": map[string]any{
			"mode":    "reference_only",
			"sources": []any{map[string]any{"id": "local", "kind": "corpus_bundle", "path": "exports/corpus"}},
			"jobs":    []any{"draft"},
		},
	}

	if got := augmentationReferences(workflow, "review", t.TempDir()); got != nil {
		t.Fatalf("augmentation references = %#v, want nil", got)
	}
}

func TestClaimNextReclaimRequeuedJobWithFreshSessionRequired(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_reclaim"
	runID := "run_claim_reclaim"
	sessionID := "sess_1"
	role := "worker"
	lane := "claude"

	// 1. Seed Repo and Run
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
	})

	// 2. Seed Session
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, sessionID, lane)

	// 3. Seed Job 1 with fresh_session_required = true
	jobID1 := "job_1"
	err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
			repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			title, job_type, fresh_session_required, idempotency_key, created_at
		) VALUES ($1, $2, $3, 'job_w_1', 1, 'queued', $4, 'Job 1', 'draft', true, 'idem_1', NOW())`,
		repoID, jobID1, runID, role)
	if err != nil {
		t.Fatalf("failed to insert job 1: %v", err)
	}

	// 4. Seed Queue Message for Job 1
	msgID1 := "msg_1"
	err = runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
			repository_id, message_id, run_id, job_id, kind, state, target_role_id, target_lane_id, priority, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'work', 'pending', $5, $6, 10, NOW(), NOW())`,
		repoID, msgID1, runID, jobID1, role, lane)
	if err != nil {
		t.Fatalf("failed to insert queue message 1: %v", err)
	}

	// 5. First Claim: Claim the job
	res1, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
	}))
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if res1["status"] != "claimed" {
		t.Fatalf("expected first claim status to be claimed, got %v", res1["status"])
	}

	// 6. Simulate job re-queueing (after checkpoint resolution, etc.)
	// Release the job, release the lease, and reset queue message to pending
	err = runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'queued', current_lease_id = NULL
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID1)
	if err != nil {
		t.Fatalf("failed to reset job 1 state: %v", err)
	}
	err = runner.Exec(ctx, `
		UPDATE striatumd.leases
		   SET state = 'released'
		 WHERE repository_id = $1 AND resource_type = 'job' AND resource_id = $2`, repoID, jobID1)
	if err != nil {
		t.Fatalf("failed to release job 1 lease: %v", err)
	}
	err = runner.Exec(ctx, `
		UPDATE striatumd.queue_messages
		   SET state = 'pending', current_lease_id = NULL
		 WHERE repository_id = $1 AND message_id = $2`, repoID, msgID1)
	if err != nil {
		t.Fatalf("failed to reset queue message 1 state: %v", err)
	}

	// 7. Second Claim: Try to reclaim the same job with the same session
	res2, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
	}))
	if err != nil {
		t.Fatalf("second claim failed: %v", err)
	}
	if res2["status"] != "claimed" {
		t.Fatalf("expected second claim status to be claimed, got %v", res2["status"])
	}

	// 8. Seed a DIFFERENT Job 2 with fresh_session_required = true under the same run
	jobID2 := "job_2"
	err = runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
			repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			title, job_type, fresh_session_required, idempotency_key, created_at
		) VALUES ($1, $2, $3, 'job_w_2', 1, 'queued', $4, 'Job 2', 'draft', true, 'idem_2', NOW())`,
		repoID, jobID2, runID, role)
	if err != nil {
		t.Fatalf("failed to insert job 2: %v", err)
	}

	msgID2 := "msg_2"
	err = runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
			repository_id, message_id, run_id, job_id, kind, state, target_role_id, target_lane_id, priority, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'work', 'pending', $5, $6, 10, NOW(), NOW())`,
		repoID, msgID2, runID, jobID2, role, lane)
	if err != nil {
		t.Fatalf("failed to insert queue message 2: %v", err)
	}

	// 9. Third Claim: Try to claim Job 2 (different job) with the same session.
	// Since Job 1 was already claimed and fresh_session_required is true for Job 2,
	// this session cannot claim Job 2 because it's a different job in the same run.
	res3, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
	}))
	if err != nil {
		t.Fatalf("third claim failed: %v", err)
	}
	if res3["status"] != "no_work" {
		t.Fatalf("expected third claim status to be no_work, got %v", res3["status"])
	}
}

// TestClaimNextSuppressesSuperviseSendWithSelfDrivingSupervisor verifies #68
// end-to-end: when an attached supervisor is recorded in agent_loop_mode
// self_driving, HandleClaimNext must NOT emit a supervise_send hint (which
// would invite an operator double-claim) and must surface the self-claim note.
func TestClaimNextSuppressesSuperviseSendWithSelfDrivingSupervisor(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_selfdrive"
	runID := "run_claim_selfdrive"
	sessionID := "sess_selfdrive"
	role := "worker"
	lane := "claude"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, sessionID, lane)

	// Mark the attached supervisor pointer as agent_loop self_driving.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET metadata_json = '{"agent_loop_mode":"self_driving"}'::jsonb
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID); err != nil {
		t.Fatalf("set self_driving metadata: %v", err)
	}

	jobID := "job_selfdrive"
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
			repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			title, job_type, fresh_session_required, idempotency_key, created_at
		) VALUES ($1, $2, $3, 'job_w_1', 1, 'queued', $4, 'Job', 'draft', false, 'idem_sd', NOW())`,
		repoID, jobID, runID, role); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
			repository_id, message_id, run_id, job_id, kind, state, target_role_id, target_lane_id, priority, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'work', 'pending', $5, $6, 10, NOW(), NOW())`,
		repoID, "msg_sd", runID, jobID, role, lane); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}

	res, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	if err != nil {
		t.Fatalf("claim_next: %v", err)
	}
	if res["status"] != "claimed" {
		t.Fatalf("status = %v, want claimed", res["status"])
	}
	nextSteps, _ := res["next_steps"].(map[string]any)
	if nextSteps == nil {
		t.Fatalf("missing next_steps: %#v", res)
	}
	if _, ok := nextSteps["supervise_send"]; ok {
		t.Fatalf("supervise_send must be suppressed for self-driving supervisor: %#v", nextSteps)
	}
	if _, ok := nextSteps["self_claim_note"]; !ok {
		t.Fatalf("expected self_claim_note: %#v", nextSteps)
	}
}

// TestClaimNextRefusesClosedSession verifies RFC 0095 §4 (F-I/#81): a session
// closed with close_reason interrogation_window_closed (process still alive)
// must never be granted a revision-cycle job via work.claim_next /
// work.await_packet. Both must refuse with a clear "register a fresh session"
// error instead of letting the prior author rewrite its own challenged work.
func TestClaimNextRefusesClosedSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_closed_session"
	runID := "run_claim_closed_session"
	sessionID := "sess_closed"
	role := "worker"
	lane := "claude"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
	})

	// The session is closed (window closed) but its supervised process is alive.
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, nil, "active")
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET state = 'closed', closed_at = NOW(), close_reason = 'interrogation_window_closed'
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID); err != nil {
		t.Fatalf("close session: %v", err)
	}

	// A revision-cycle job is queued and addressed to this (role, lane).
	jobID := "job_revision"
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
			repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			title, job_type, fresh_session_required, idempotency_key, created_at
		) VALUES ($1, $2, $3, 'job_w_1', 2, 'queued', $4, 'Revision', 'draft', true, 'idem_rev', NOW())`,
		repoID, jobID, runID, role); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
			repository_id, message_id, run_id, job_id, kind, state, target_role_id, target_lane_id, priority, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'work', 'pending', $5, $6, 10, NOW(), NOW())`,
		repoID, "msg_rev", runID, jobID, role, lane); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}

	// work.claim_next must refuse the closed session.
	_, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("claim_next err = %v, want invalid_transition", err)
	}
	if !strings.Contains(rpcErr.Message, "register a fresh session") {
		t.Fatalf("claim_next message = %q, want it to mention 'register a fresh session'", rpcErr.Message)
	}

	// work.await_packet must also refuse it (no delivery of work/interrogation).
	_, err = HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("await_packet err = %v, want invalid_transition", err)
	}
	if !strings.Contains(rpcErr.Message, "register a fresh session") {
		t.Fatalf("await_packet message = %q, want it to mention 'register a fresh session'", rpcErr.Message)
	}

	// The job must remain queued (not reclaimed by the closed session).
	jobRow, err := oneRow(ctx, runner, `SELECT state FROM striatumd.jobs WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil || fmt.Sprint(jobRow["state"]) != "queued" {
		t.Fatalf("job should remain queued, got state: %v, err: %v", jobRow["state"], err)
	}
}
