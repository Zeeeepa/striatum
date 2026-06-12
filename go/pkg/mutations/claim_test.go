package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
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

func TestAwaitNoneEnvelopeTellsLaneToExitIdleSession(t *testing.T) {
	env := awaitNoneEnvelope()
	if env["type"] != "none" || env["status"] != "no_work" {
		t.Fatalf("idle envelope shape = %#v", env)
	}
	if env["idle_behavior"] != "exit_session" {
		t.Fatalf("idle_behavior = %#v, want exit_session", env["idle_behavior"])
	}
	if hint, _ := env["hint"].(string); !strings.Contains(hint, "stop this lane") {
		t.Fatalf("hint = %q, want stop-this-lane guidance", hint)
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

func TestNoTargetsPacketUnchanged(t *testing.T) {
	workflow := map[string]any{
		"jobs": []any{
			map[string]any{"id": "consumer", "type": "review"},
		},
	}
	targets, err := interrogationTargetsForPacket(context.Background(), inertRunner{}, "repo_no_targets", "run_no_targets", workflow, "consumer")
	if err != nil {
		t.Fatalf("interrogationTargetsForPacket: %v", err)
	}
	if targets != nil {
		t.Fatalf("targets = %#v, want nil", targets)
	}
}

func TestDownstreamImplementationEnvelopeForPacketSurfacesReachableWriteScopes(t *testing.T) {
	workflow := map[string]any{
		"jobs": []any{
			map[string]any{"id": "synthesis", "type": "synthesis"},
			map[string]any{"id": "checkpoint", "type": "human_checkpoint"},
			map[string]any{
				"id":      "implement",
				"type":    "implementation",
				"title":   "Build the accepted design",
				"role_id": "builder",
				"lane_id": "codex",
				"write_scope": map[string]any{
					"allowed_paths":   []any{"src/allowed/", "docs/implementation/"},
					"forbidden_paths": []any{".striatum/"},
				},
				"expected_artifacts": []any{
					map[string]any{"logical_name": "implementation_report", "path": "docs/implementation/REPORT.md"},
				},
			},
			map[string]any{
				"id":      "review",
				"type":    "review",
				"role_id": "reviewer",
				"write_scope": map[string]any{
					"allowed_paths": []any{"docs/review/"},
				},
			},
		},
		"edges": []any{
			map[string]any{"from": "synthesis", "to": "checkpoint"},
			map[string]any{"from": "checkpoint", "to": "implement"},
			map[string]any{"from": "implement", "to": "review"},
		},
	}

	got := downstreamImplementationEnvelopeForPacket(workflow, "synthesis")
	if got == nil {
		t.Fatal("implementation envelope missing")
	}
	if got["scope"] != "reachable_downstream_jobs" {
		t.Fatalf("scope = %v", got["scope"])
	}
	if !strings.Contains(fmt.Sprint(got["instruction"]), "frozen scope") {
		t.Fatalf("instruction should warn about frozen scope: %#v", got["instruction"])
	}
	jobs := asList(got["jobs"])
	if len(jobs) != 2 {
		t.Fatalf("jobs = %#v, want implement + review envelopes", jobs)
	}
	implement := asMap(jobs[0])
	if implement["workflow_job_id"] != "implement" || implement["lane_id"] != "codex" {
		t.Fatalf("implement envelope = %#v", implement)
	}
	allowed := asList(asMap(implement["write_scope"])["allowed_paths"])
	if !reflect.DeepEqual(allowed, []any{"src/allowed/", "docs/implementation/"}) {
		t.Fatalf("implement allowed_paths = %#v", allowed)
	}
	if artifacts := asList(implement["expected_artifacts"]); len(artifacts) != 1 {
		t.Fatalf("implement expected_artifacts = %#v", artifacts)
	}
	review := asMap(jobs[1])
	if review["workflow_job_id"] != "review" || asMap(review["write_scope"])["allowed_paths"] == nil {
		t.Fatalf("review envelope = %#v", review)
	}
}

func TestSharedResourcesForPacketSurfacesParallelHazards(t *testing.T) {
	workflow := map[string]any{
		"jobs": []any{
			map[string]any{
				"id":             "privacy_review",
				"type":           "review",
				"title":          "Privacy review",
				"role_id":        "reviewer",
				"lane_id":        "codex",
				"parallel_group": "db_reviews",
				"shared_resources": []any{
					map[string]any{
						"id":          "postgres:test-db",
						"description": "DB-backed validation fixture",
					},
				},
			},
			map[string]any{
				"id":             "authority_review",
				"type":           "review",
				"title":          "Authority review",
				"role_id":        "reviewer",
				"lane_id":        "claude",
				"parallel_group": "db_reviews",
				"shared_resources": []any{
					"postgres:test-db",
				},
			},
			map[string]any{
				"id":             "isolated_review",
				"type":           "review",
				"role_id":        "reviewer",
				"lane_id":        "agy",
				"parallel_group": "db_reviews",
				"shared_resources": []any{map[string]any{
					"id":        "postgres:test-db",
					"mode":      "per_lane_namespace",
					"namespace": "isolated_review",
				}},
			},
		},
	}

	got := sharedResourcesForPacket(workflow, "privacy_review")
	if got == nil {
		t.Fatal("shared_resources packet block missing")
	}
	if got["scope"] != "current_workflow_job" || got["parallel_group"] != "db_reviews" {
		t.Fatalf("shared resource header = %#v", got)
	}
	resources := asList(got["resources"])
	if len(resources) != 1 {
		t.Fatalf("resources = %#v", resources)
	}
	resource := asMap(resources[0])
	if resource["id"] != "postgres:test-db" || resource["mode"] != "exclusive" {
		t.Fatalf("resource = %#v", resource)
	}
	if !strings.Contains(fmt.Sprint(got["instruction"]), "coordinate") {
		t.Fatalf("instruction should mention coordination: %v", got["instruction"])
	}
	related := asList(got["related_parallel_jobs"])
	if len(related) != 2 {
		t.Fatalf("related parallel jobs = %#v, want authority_review + isolated_review because current job is exclusive", related)
	}
	first := asMap(related[0])
	if first["workflow_job_id"] != "authority_review" || first["resource_id"] != "postgres:test-db" {
		t.Fatalf("related job = %#v", first)
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

// TestClaimNextPersistsRenderedPacketWithHash verifies #220: every successful
// work.claim writes exactly the rendered packet JSON and its sha256 into
// striatumd.work_packets, so a dispatched packet is durably auditable later by
// hash and (on explicit request) by body. The queue message payload stays the
// empty object — it is queue metadata, not the packet source of truth — so a
// wrong-packet vs misfollowed-lane dispute is answerable from daemon state.
func TestClaimNextPersistsRenderedPacketWithHash(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_packet_persist"
	runID := "run_claim_packet_persist"
	sessionID := "sess_1"
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

	jobID := "job_1"
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
			repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			title, job_type, idempotency_key, created_at
		) VALUES ($1,$2,$3,'job_w_1',1,'queued',$4,'Job 1','draft','idem_1',NOW())`,
		repoID, jobID, runID, role); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	msgID := "msg_1"
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
			repository_id, message_id, run_id, job_id, kind, state, target_role_id,
			target_lane_id, priority, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',$5,$6,10,NOW(),NOW())`,
		repoID, msgID, runID, jobID, role, lane); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}

	res, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if res["status"] != "claimed" {
		t.Fatalf("status = %v", res["status"])
	}
	packetID, _ := res["packet_id"].(string)
	if packetID == "" {
		t.Fatalf("missing packet_id: %#v", res)
	}

	// The hash stored at claim is computed over json.Marshal(packet) where packet
	// is the exact map returned to the caller, so it must round-trip here.
	wantBytes, err := json.Marshal(res["packet"])
	if err != nil {
		t.Fatalf("marshal returned packet: %v", err)
	}
	wantSum := sha256.Sum256(wantBytes)
	wantSha := hex.EncodeToString(wantSum[:])

	var gotSha string
	var gotJSON []byte
	if err := runner.QueryRow(ctx, `
		SELECT packet_sha256, packet_json::text
		  FROM striatumd.work_packets
		 WHERE repository_id = $1 AND packet_id = $2`, repoID, packetID).Scan(&gotSha, &gotJSON); err != nil {
		t.Fatalf("read work_packets: %v", err)
	}
	if gotSha != wantSha {
		t.Fatalf("packet_sha256 = %s, want %s", gotSha, wantSha)
	}
	// The stored body must deserialize to the same packet that was dispatched.
	var storedPacket, dispatched map[string]any
	if err := json.Unmarshal(gotJSON, &storedPacket); err != nil {
		t.Fatalf("unmarshal stored packet_json: %v", err)
	}
	if err := json.Unmarshal(wantBytes, &dispatched); err != nil {
		t.Fatalf("unmarshal dispatched packet: %v", err)
	}
	if !reflect.DeepEqual(storedPacket, dispatched) {
		t.Fatalf("stored packet_json != dispatched packet\nstored=%#v\ndispatched=%#v", storedPacket, dispatched)
	}

	// The queue message payload is never backfilled with the packet body.
	var payload string
	if err := runner.QueryRow(ctx, `
		SELECT payload_json::text FROM striatumd.queue_messages
		 WHERE repository_id = $1 AND message_id = $2`, repoID, msgID).Scan(&payload); err != nil {
		t.Fatalf("read queue payload: %v", err)
	}
	if strings.TrimSpace(payload) != "{}" {
		t.Fatalf("queue_messages.payload_json = %q, want {} (packet body must not be duplicated here)", payload)
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

func TestClaimNextProjectsExplicitInterrogationTargets(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_interrogation_targets"
	runID := "run_" + repoID
	targetSession := "sess_synth"
	consumerSession := "sess_consumer"
	consumerJob := "job_consumer"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id":  "wf",
		"roles":        map[string]any{"synthesizer": map[string]any{}, "reviewer": map[string]any{}},
		"lanes":        map[string]any{"claude": map[string]any{"display_model": "Claude", "capabilities": []any{"interrogate"}}},
		"context_docs": []any{},
		"jobs": []any{
			map[string]any{"id": "design_synthesis", "interrogable": true},
			map[string]any{"id": "explicit_consumer", "interrogation_targets": []any{map[string]any{"workflow_job_id": "design_synthesis", "required": true}}},
		},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, targetSession, "synthesizer", "claude", nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, targetSession, "claude")
	intgSeedSession(t, ctx, runner, repoID, runID, consumerSession, "reviewer", "claude", []string{"interrogate"}, "active")
	intgAttest(t, ctx, runner, repoID, runID, consumerSession, "claude")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, created_at, completed_at
		) VALUES ($1,'job_synth',$2,'design_synthesis',2,'completed','synthesizer','Synthesis','synthesis','idem_synth','[]'::jsonb,NOW(),NOW())`,
		repoID, runID); err != nil {
		t.Fatalf("insert synth job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.events (repository_id, run_id, event_type, actor_session_id, job_id, payload_json, created_at)
		VALUES ($1,$2,'session.awaiting_interrogation',$3,'job_synth','{"workflow_job_id":"design_synthesis","attempt":2}'::jsonb,NOW())`,
		repoID, runID, targetSession); err != nil {
		t.Fatalf("insert awaiting event: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, created_at
		) VALUES ($1,$2,$3,'explicit_consumer',1,'queued','reviewer','Consumer','build','idem_consumer','[]'::jsonb,NOW())`,
		repoID, consumerJob, runID); err != nil {
		t.Fatalf("insert consumer job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,'msg_consumer',$2,$3,'work','pending',0,'reviewer','claude',NOW(),NOW())`,
		repoID, runID, consumerJob); err != nil {
		t.Fatalf("insert consumer message: %v", err)
	}

	res, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": consumerSession}))
	if err != nil {
		t.Fatalf("claim explicit consumer: %v", err)
	}
	packet := asMap(res["packet"])
	contextBlock := asMap(packet["context"])
	targets := asList(contextBlock["interrogation_targets"])
	if len(targets) != 1 {
		t.Fatalf("packet interrogation_targets = %#v", contextBlock["interrogation_targets"])
	}
	target := asMap(targets[0])
	if target["workflow_job_id"] != "design_synthesis" || target["required"] != true {
		t.Fatalf("target identity = %#v", target)
	}
	if target["state"] != "available" || target["target_session_id"] != targetSession || intValue(target["target_attempt"]) != 2 {
		t.Fatalf("target availability = %#v", target)
	}
	if !strings.Contains(fmt.Sprint(target["instruction"]), "Open interrogation") {
		t.Fatalf("target instruction = %#v", target["instruction"])
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

// TestAwaitPacketDoesNotExpireSessionBeforeSupervisorAttaches guards GH #245:
// the first autonomous await can race with supervise.start's backend attach.
// work.await_packet must wait through that claim race instead of delegating to
// claim_next's direct-operator backend cleanup, which expires the fresh session
// and leaves the lane holding a dead session-bound token.
func TestAwaitPacketDoesNotExpireSessionBeforeSupervisorAttaches(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_await_backend_race"
	runID := "run_await_backend_race"
	sessionID := "sess_backend_race"
	role := "worker"
	lane := "claude"
	jobID := "job_backend_race"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, nil, "active")
	intgSeedClaimableWork(t, ctx, runner, repoID, runID, jobID, "work", role, lane)

	restoreTimeout := awaitPacketTimeout
	restorePoll := awaitPacketPollInterval
	awaitPacketTimeout = 20 * time.Millisecond
	awaitPacketPollInterval = 5 * time.Millisecond
	t.Cleanup(func() {
		awaitPacketTimeout = restoreTimeout
		awaitPacketPollInterval = restorePoll
	})

	env, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	if err != nil {
		t.Fatalf("await_packet should not expire a pre-attach session: %v", err)
	}
	if env["type"] != "none" || env["status"] != "no_work" {
		t.Fatalf("backend-race envelope shape = %#v", env)
	}
	if env["reason"] != "session_backend_not_ready" {
		t.Fatalf("backend-race reason = %#v, want session_backend_not_ready; env=%#v", env["reason"], env)
	}
	if got := intgSessionState(t, ctx, runner, repoID, sessionID); got != "active" {
		t.Fatalf("session state = %q, want active (await must not run claim cleanup)", got)
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued", got)
	}
	if n := activeLeaseCount(t, ctx, runner, repoID, jobID); n != 0 {
		t.Fatalf("active lease count = %d, want 0", n)
	}
}

// TestAwaitPacketStoppedSessionReturnsActionableEnvelope guards GH #245: a
// supervised agent can exit and mark its session stopped before the lane reaches
// its first work.await_packet call. Awaiting that stopped session must not
// strand the lane behind an RPC error, and it must not claim queued work on a
// dead session. The envelope tells the receiver to exit and lets run drive start
// a fresh session.
func TestAwaitPacketStoppedSessionReturnsActionableEnvelope(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_await_stopped_session"
	runID := "run_await_stopped_session"
	sessionID := "sess_stopped_before_await"
	role := "worker"
	lane := "claude"
	jobID := "job_waiting_for_fresh_lane"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{role: map[string]any{}},
		"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, nil, "stopped")
	intgSeedClaimableWork(t, ctx, runner, repoID, runID, jobID, "work", role, lane)

	env, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	if err != nil {
		t.Fatalf("await_packet against stopped session returned error: %v", err)
	}
	if env["type"] != "session_terminal" || env["status"] != "no_work" {
		t.Fatalf("stopped-session envelope shape = %#v", env)
	}
	if env["reason"] != "session_stopped" || env["session_state"] != "stopped" {
		t.Fatalf("stopped-session reason/state = %#v", env)
	}
	if env["idle_behavior"] != "exit_session" || env["next_action"] != "register_fresh_session" {
		t.Fatalf("stopped-session action fields = %#v", env)
	}
	if hint, _ := env["hint"].(string); !strings.Contains(hint, "fresh session") {
		t.Fatalf("hint = %q, want fresh-session guidance", hint)
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued (stopped session must not claim work)", got)
	}
	if n := activeLeaseCount(t, ctx, runner, repoID, jobID); n != 0 {
		t.Fatalf("active lease count = %d, want 0", n)
	}
}

type claimBackendGateExec struct {
	sql  string
	args []any
}

type claimBackendGateFakeTx struct {
	execs []claimBackendGateExec
}

func (tx *claimBackendGateFakeTx) Exec(ctx context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, claimBackendGateExec{sql: sql, args: append([]any{}, args...)})
	return nil
}

func (tx *claimBackendGateFakeTx) QueryRow(ctx context.Context, sql string, args ...any) db.Row {
	return claimBackendGateRow{err: pgx.ErrNoRows}
}

func (tx *claimBackendGateFakeTx) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	return "", nil
}

func (tx *claimBackendGateFakeTx) Commit(ctx context.Context) error {
	return nil
}

func (tx *claimBackendGateFakeTx) Rollback(ctx context.Context) error {
	return nil
}

func (tx *claimBackendGateFakeTx) sawSessionUpdateArg(arg any) bool {
	for _, exec := range tx.execs {
		if !strings.Contains(exec.sql, "UPDATE striatumd.sessions") {
			continue
		}
		for _, item := range exec.args {
			if item == arg {
				return true
			}
		}
	}
	return false
}

type claimBackendGateRow struct {
	err error
}

func (r claimBackendGateRow) Scan(dest ...any) error {
	return r.err
}

// cleanupFakeRunner is a minimal db.Runner that records Exec calls, for asserting
// the backend gate's deferred cleanup (which runs on the pooled runner, not the
// rolled-back work tx).
type cleanupFakeRunner struct {
	execs []claimBackendGateExec
}

func (r *cleanupFakeRunner) Exec(ctx context.Context, sql string, args ...any) error {
	r.execs = append(r.execs, claimBackendGateExec{sql: sql, args: append([]any{}, args...)})
	return nil
}

func (r *cleanupFakeRunner) QueryRow(ctx context.Context, sql string, args ...any) db.Row {
	return claimBackendGateRow{err: pgx.ErrNoRows}
}

func (r *cleanupFakeRunner) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	return "", nil
}

func (r *cleanupFakeRunner) BeginTx(ctx context.Context) (db.TxRunner, error) {
	return &claimBackendGateFakeTx{}, nil
}

func (r *cleanupFakeRunner) sawSessionUpdateArg(arg any) bool {
	for _, exec := range r.execs {
		if !strings.Contains(exec.sql, "UPDATE striatumd.sessions") {
			continue
		}
		for _, item := range exec.args {
			if item == arg {
				return true
			}
		}
	}
	return false
}

func TestEnsureWorkSessionBackendExpiresMissingSupervisor(t *testing.T) {
	tx := &claimBackendGateFakeTx{}

	err := ensureWorkSessionBackend(context.Background(), tx, "repo_1", "sess_1", "claim")
	var gate *sessionBackendGateError
	if !errors.As(err, &gate) {
		t.Fatalf("backend gate err = %v, want *sessionBackendGateError", err)
	}
	if gate.rpcErr == nil || gate.rpcErr.Code != "invalid_transition" {
		t.Fatalf("gate rpcErr = %v, want invalid_transition", gate.rpcErr)
	}
	// The gate must NOT mutate the work tx — that write would be rolled back with
	// the refusal. The expiry is deferred to cleanup on the pooled runner.
	if tx.sawSessionUpdateArg("expired") {
		t.Fatalf("gate must not expire inside the work tx; execs=%#v", tx.execs)
	}
	if gate.cleanup == nil {
		t.Fatalf("gate cleanup must be set to commit the session expiry")
	}
	cleanup := &cleanupFakeRunner{}
	if err := gate.cleanup(context.Background(), cleanup); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if !cleanup.sawSessionUpdateArg("expired") {
		t.Fatalf("cleanup missing session expired update, execs=%#v", cleanup.execs)
	}
	if !cleanup.sawSessionUpdateArg("claim refused: no_attached_supervisor") {
		t.Fatalf("cleanup missing close_reason update, execs=%#v", cleanup.execs)
	}
}

func TestClaimNextExpiresUnattestedSessionBeforeClaim(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_unattested"
	runID := "run_claim_unattested"
	sessionID := "sess_unattested"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"worker": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "worker", "claude", nil, "active")
	intgSeedClaimableWork(t, ctx, runner, repoID, runID, "job_unattested", "work", "worker", "claude")

	_, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("claim_next err = %v, want invalid_transition", err)
	}
	if got := intgSessionState(t, ctx, runner, repoID, sessionID); got != "expired" {
		t.Fatalf("session state = %q, want expired", got)
	}
	if got := jobState(t, ctx, runner, repoID, "job_unattested"); got != "queued" {
		t.Fatalf("job state = %q, want queued", got)
	}
	if n := activeLeaseCount(t, ctx, runner, repoID, "job_unattested"); n != 0 {
		t.Fatalf("active lease count = %d, want 0", n)
	}
}

func TestClaimNextMarksDeadAttachedSupervisorLostBeforeClaim(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_claim_dead_supervisor"
	runID := "run_claim_dead_supervisor"
	sessionID := "sess_dead_supervisor"
	supervisorID := "sup_dead_supervisor"
	daemonSupervisorID := "dsup_dead_supervisor"
	deadPID := 99999999
	now := time.Now().UTC()

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"worker": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "worker", "claude", nil, "active")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, state, started_at
		) VALUES ($1,$2,$3,$4,'process','[]'::jsonb,'/tmp','/tmp/scratch',$5,'attached',$6)`,
		repoID, supervisorID, runID, sessionID, deadPID, now); err != nil {
		t.Fatalf("insert supervisor: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,'attached',$7,'{}'::jsonb)`,
		repoID, supervisorID, daemonSupervisorID, runID, sessionID, deadPID, now); err != nil {
		t.Fatalf("insert pointer: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_supervisors (
		  daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
		  daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
		  state, started_at, heartbeat_at
		) VALUES ($1,$2,$3,$4,$5,'inst','process','[]'::jsonb,'sha','/tmp',$6,'attached',$7,$7)`,
		daemonSupervisorID, repoID, runID, sessionID, supervisorID, deadPID, now); err != nil {
		t.Fatalf("insert daemon supervisor: %v", err)
	}
	intgSeedClaimableWork(t, ctx, runner, repoID, runID, "job_dead_supervisor", "work", "worker", "claude")

	_, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("claim_next err = %v, want invalid_transition", err)
	}
	if got := intgSessionState(t, ctx, runner, repoID, sessionID); got != "lost" {
		t.Fatalf("session state = %q, want lost", got)
	}
	row, err := oneRow(ctx, runner, `
		SELECT ps.state AS supervisor_state, ptr.state AS pointer_state, ds.state AS daemon_state
		  FROM striatumd.process_supervisors ps
		  JOIN striatumd.process_supervisor_pointers ptr
		    ON ptr.repository_id = ps.repository_id AND ptr.supervisor_id = ps.supervisor_id
		  JOIN striatumd.daemon_supervisors ds
		    ON ds.repository_id = ps.repository_id AND ds.daemon_supervisor_id = ptr.daemon_supervisor_id
		 WHERE ps.repository_id = $1 AND ps.supervisor_id = $2`,
		repoID, supervisorID)
	if err != nil {
		t.Fatalf("read supervisor states: %v", err)
	}
	if row["supervisor_state"] != "lost" || row["pointer_state"] != "lost" || row["daemon_state"] != "lost" {
		t.Fatalf("supervisor states = %#v, want all lost", row)
	}
}

func TestCompleteWorkRejectsUnattestedSession(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_complete_unattested"
	runID := "run_complete_unattested"
	sessionID := "sess_complete_unattested"
	jobID := "job_complete_unattested"
	leaseID := "lease_complete_unattested"
	now := time.Now().UTC()

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"worker": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "worker", "claude", nil, "active")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  state, idempotency_key, created_at, started_at, current_lease_id
		) VALUES ($1,$2,$3,'work','Work','build','worker','running','idem_complete_unattested',$4,$4,$5)`,
		repoID, jobID, runID, now, leaseID); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}

	_, err := HandleCompleteWork(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"summary":    "done",
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("complete err = %v, want invalid_transition", err)
	}
	if got := intgSessionState(t, ctx, runner, repoID, sessionID); got != "expired" {
		t.Fatalf("session state = %q, want expired", got)
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "running" {
		t.Fatalf("job state = %q, want running", got)
	}
	if n := activeLeaseCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("active lease count = %d, want 1", n)
	}
}

// #105: a role/context path declared relative to the workflow dir resolves to a
// repo-root-relative path so a lane running from the repo root opens it on the
// first try, and the explicit workflow_root is derived from the snapshot.
func TestResolveWorkflowRelativePathAndRoot(t *testing.T) {
	root := workflowRootDir(map[string]any{
		"source_path": "striatum/rfc-0070-semantic-context-quiz-panel-2026-05-30/workflow.json",
	})
	if root != "striatum/rfc-0070-semantic-context-quiz-panel-2026-05-30" {
		t.Fatalf("workflow root = %q", root)
	}

	// Bare workflow-relative path -> repo-root-relative + original kept.
	resolved, rel := resolveWorkflowRelativePath("roles/final_reviewer.md", root)
	if resolved != root+"/roles/final_reviewer.md" {
		t.Fatalf("resolved = %q, want %q", resolved, root+"/roles/final_reviewer.md")
	}
	if rel != "roles/final_reviewer.md" {
		t.Fatalf("workflow-relative = %q", rel)
	}

	// Already repo-root-relative: not joined twice.
	resolved2, rel2 := resolveWorkflowRelativePath(root+"/roles/x.md", root)
	if resolved2 != root+"/roles/x.md" || rel2 != "roles/x.md" {
		t.Fatalf("already-prefixed resolved=%q rel=%q", resolved2, rel2)
	}

	// Absolute path and empty root pass through unchanged.
	if got, rel := resolveWorkflowRelativePath("/abs/x.md", root); got != "/abs/x.md" || rel != "" {
		t.Fatalf("absolute path mutated: %q %q", got, rel)
	}
	if got, _ := resolveWorkflowRelativePath("roles/x.md", ""); got != "roles/x.md" {
		t.Fatalf("empty root should pass through, got %q", got)
	}

	// A workflow at the repo root has no distinct workflow_root.
	if got := workflowRootDir(map[string]any{"source_path": "workflow.json"}); got != "" {
		t.Fatalf("repo-root workflow should yield empty root, got %q", got)
	}
}

// #107: when the only remaining queued work for a role is fresh_session_required
// and the current session is already spent, claim_next returns no_work WITH a
// structured ineligibility reason (not a bare no_work), so the lane can stop and
// the coordinator can register a fresh session.
func TestClaimNextExplainsFreshSessionIneligibility(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_fresh_ineligible"
	runID := "run_" + repoID
	sess := "sess_spent_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"capabilities": []any{"write"}}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sess, "implementer", "claude", []string{"claim", "write"}, "active")
	intgAttest(t, ctx, runner, repoID, runID, sess, "claude")

	seedQueuedImplJob := func(jobID, wfJobID string, fresh bool) {
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, fresh_session_required, idempotency_key,
			  expected_artifacts_json, created_at
			) VALUES ($1,$2,$3,$4,1,'queued','implementer','Impl','build',$5,'idem_'||$2,'[]'::jsonb,NOW())`,
			repoID, jobID, runID, wfJobID, fresh); err != nil {
			t.Fatalf("insert job %s: %v", jobID, err)
		}
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.queue_messages (
			  repository_id, message_id, run_id, job_id, kind, state, priority,
			  target_role_id, target_lane_id, created_at, updated_at
			) VALUES ($1,'msg_'||$2,$3,$2,'work','pending',0,'implementer','claude',NOW(),NOW())`,
			repoID, jobID, runID); err != nil {
			t.Fatalf("insert message %s: %v", jobID, err)
		}
	}

	// Slice 1: claimable; the session claims it (becoming "spent").
	seedQueuedImplJob("job_slice1", "slice1", false)
	res1, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sess}))
	if err != nil {
		t.Fatalf("claim slice1: %v", err)
	}
	if res1["status"] != "claimed" {
		t.Fatalf("expected slice1 claimed, got %#v", res1)
	}

	// Slice 2: queued, fresh_session_required -> the spent session is ineligible.
	seedQueuedImplJob("job_slice2", "slice2_ui_diagnostics", true)
	res2, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sess}))
	if err != nil {
		t.Fatalf("claim slice2: %v", err)
	}
	if res2["status"] != "no_work" {
		t.Fatalf("expected no_work for the spent session, got %#v", res2)
	}
	if res2["ineligible_reason"] != "fresh_session_required" {
		t.Fatalf("#107: expected ineligible_reason=fresh_session_required, got %#v", res2)
	}
	if res2["workflow_job_id"] != "slice2_ui_diagnostics" {
		t.Fatalf("#107: expected the queued workflow_job_id named, got %#v", res2)
	}
	if hint, _ := res2["hint"].(string); !strings.Contains(hint, "fresh session") {
		t.Fatalf("#107: expected a fresh-session hint, got %#v", res2)
	}
}
