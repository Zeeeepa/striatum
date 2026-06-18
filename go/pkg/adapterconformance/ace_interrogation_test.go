package adapterconformance

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/workflowgenerate"
)

// RFC 0112 / RFC 0105: ACE explicit interrogation consumers are fixture-backed.
// adjudicated_constraint_extraction has graduated to supported — it is registered
// in ReliabilityFixtureShapes and in workflowtemplates.supportedShapes, with
// shape_tier_guard_test.go enforcing that the two stay in sync.

type aceFixture struct {
	RepositoryID string
	RunID        string
	RepoRoot     string
	Workflow     map[string]any
	Jobs         map[string]map[string]any
	JobIDs       map[string]string
	nextOrdinal  int
}

func TestACEExplicitConsumersHappyPath(t *testing.T) {
	repoID := "repo_ace_explicit_happy"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	fx := seedACEExplicitConsumersRun(t, ctx, h, repoID)

	target := aceCompleteThroughConvenerGate(t, ctx, h, fx)
	secret := "convener-only-fact: zebra-713"

	c1Session, c1Lease, c1Packet := aceClaimAndAck(t, ctx, h, fx, "cross_examiner_1")
	aceAssertTargetProjection(t, c1Packet, target.sessionID, 1, "available", "")
	intg1 := aceOpenAskAnswer(t, ctx, h, fx, c1Session, target.sessionID, secret)
	aceAssertAnswerContains(t, ctx, h, fx, intg1, secret)
	aceCloseInterrogation(t, ctx, h, fx, c1Session, intg1)
	aceCompleteWork(t, ctx, h, fx, "cross_examiner_1", c1Session, c1Lease)
	if got := aceSessionState(t, ctx, h, target.sessionID); got != "active" {
		t.Fatalf("target state after first consumer = %q, want active", got)
	}

	c2Session, c2Lease, c2Packet := aceClaimAndAck(t, ctx, h, fx, "cross_examiner_2")
	aceAssertTargetProjection(t, c2Packet, target.sessionID, 1, "available", "")
	intg2 := aceOpenAskAnswer(t, ctx, h, fx, c2Session, target.sessionID, secret)
	aceAssertAnswerContains(t, ctx, h, fx, intg2, secret)
	aceCloseInterrogation(t, ctx, h, fx, c2Session, intg2)
	aceCompleteWork(t, ctx, h, fx, "cross_examiner_2", c2Session, c2Lease)

	if got := aceSessionCloseReason(t, ctx, h, target.sessionID); got != "interrogation_window_closed" {
		t.Fatalf("target close_reason = %q, want interrogation_window_closed", got)
	}
	if got := aceSessionClosedEventCount(t, ctx, h, fx, target.sessionID, "interrogation_window_closed"); got != 1 {
		t.Fatalf("interrogation_window_closed events = %d, want 1", got)
	}
	aceAssertNoActiveAwaitingWindows(t, ctx, h, fx)
}

func TestACEExplicitConsumersRevisionReopenFreshWindow(t *testing.T) {
	repoID := "repo_ace_explicit_revision"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	fx := seedACEExplicitConsumersRun(t, ctx, h, repoID)

	target1 := aceCompleteThroughConvenerGate(t, ctx, h, fx)
	aceCompleteCrossExaminer(t, ctx, h, fx, "cross_examiner_1", target1.sessionID, "cycle-1-secret")
	aceCompleteCrossExaminer(t, ctx, h, fx, "cross_examiner_2", target1.sessionID, "cycle-1-secret")
	aceRecordVerdict(t, ctx, h, fx, "cross_exam_synthesis", "accept")
	aceCompleteWorkClaimed(t, ctx, h, fx, "adjudication_intake")
	routed := aceRecordVerdict(t, ctx, h, fx, "adjudicate", "needs_revision")
	if got := fmt.Sprint(routed["status"]); got != "revision_routed" {
		t.Fatalf("needs_revision result status = %q, want revision_routed; result=%#v", got, routed)
	}
	if got := aceSessionCloseReason(t, ctx, h, target1.sessionID); got != "revision_reopened" {
		t.Fatalf("prior target close_reason = %q, want revision_reopened", got)
	}
	if got := chaosJobAttempt(t, ctx, h, fx.JobIDs["convener_draft"]); got != 2 {
		t.Fatalf("convener_draft attempt = %d, want 2", got)
	}
	if got := chaosJobState(t, ctx, h, fx.JobIDs["cross_examiner_1"]); got != "blocked" {
		t.Fatalf("cross_examiner_1 state after re-open = %q, want blocked", got)
	}

	target2 := aceCompleteThroughConvenerGate(t, ctx, h, fx)
	if target2.sessionID == target1.sessionID {
		t.Fatalf("revision reused stale target session %s", target2.sessionID)
	}
	_, _, c1Packet := aceClaimAndAck(t, ctx, h, fx, "cross_examiner_1")
	aceAssertTargetProjection(t, c1Packet, target2.sessionID, 2, "available", "")
}

func TestACEExplicitConsumersDeadLaneDuringReCascade(t *testing.T) {
	repoID := "repo_ace_explicit_dead_lane"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	fx := seedACEExplicitConsumersRun(t, ctx, h, repoID)

	target1 := aceCompleteThroughConvenerGate(t, ctx, h, fx)
	aceCompleteCrossExaminer(t, ctx, h, fx, "cross_examiner_1", target1.sessionID, "cycle-1-secret")
	aceCompleteCrossExaminer(t, ctx, h, fx, "cross_examiner_2", target1.sessionID, "cycle-1-secret")
	aceRecordVerdict(t, ctx, h, fx, "cross_exam_synthesis", "accept")
	aceCompleteWorkClaimed(t, ctx, h, fx, "adjudication_intake")
	aceRecordVerdict(t, ctx, h, fx, "adjudicate", "needs_revision")

	target2 := aceCompleteThroughConvenerGate(t, ctx, h, fx)
	deadSession, _, _ := aceClaimAndAck(t, ctx, h, fx, "cross_examiner_1")
	priorAttempt := chaosJobAttempt(t, ctx, h, fx.JobIDs["cross_examiner_1"])
	injectDeadLane(t, ctx, h.Runner, repoID, deadSession, fx.JobIDs["cross_examiner_1"])

	summary := runSweep(t, ctx, h, fx.RunID)
	if summary.escalationCount != 0 {
		t.Fatalf("dead-lane sweep escalated, want same-attempt requeue; result=%#v", summary.result)
	}
	// Regression coverage for the released-lease same-second tiebreak: without
	// the current_lease_id preference in recoverStuckJobs' lease resolution, the
	// scan can resolve the PRIOR attempt's released lease (whose owner session is
	// still active) and skip the genuinely dead current owner.
	if got := chaosJobState(t, ctx, h, fx.JobIDs["cross_examiner_1"]); got != "queued" {
		t.Fatalf("cross_examiner_1 after sweep = %q, want queued; sweep=%#v", got, summary.result)
	}
	if got := chaosJobAttempt(t, ctx, h, fx.JobIDs["cross_examiner_1"]); got != priorAttempt {
		t.Fatalf("cross_examiner_1 attempt = %d, want %d", got, priorAttempt)
	}
	if got := aceSessionState(t, ctx, h, target2.sessionID); got != "active" {
		t.Fatalf("target state during re-cascade recovery = %q, want active", got)
	}
	if got := chaosJobState(t, ctx, h, fx.JobIDs["cross_exam_synthesis"]); got != "blocked" {
		t.Fatalf("join state during recovery = %q, want blocked", got)
	}

	aceCompleteCrossExaminer(t, ctx, h, fx, "cross_examiner_1", target2.sessionID, "cycle-2-secret")
	aceCompleteCrossExaminer(t, ctx, h, fx, "cross_examiner_2", target2.sessionID, "cycle-2-secret")
	aceRecordVerdict(t, ctx, h, fx, "cross_exam_synthesis", "accept")
	aceCompleteWorkClaimed(t, ctx, h, fx, "adjudication_intake")
	accepted := aceRecordVerdict(t, ctx, h, fx, "adjudicate", "accept")
	if got := fmt.Sprint(accepted["status"]); got != "completed" {
		t.Fatalf("accept result status = %q, want completed; result=%#v", got, accepted)
	}
	if got := chaosRunState(t, ctx, h, fx.RunID); got != "completed" {
		t.Fatalf("run state = %q, want completed", got)
	}
}

func TestACEExplicitConsumersAdvisoryEvidenceAndWaitingHuman(t *testing.T) {
	repoID := "repo_ace_explicit_evidence"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	fx := seedACEExplicitConsumersRun(t, ctx, h, repoID)

	target := aceCompleteThroughConvenerGate(t, ctx, h, fx)
	c1Session, c1Lease, _ := aceClaimAndAck(t, ctx, h, fx, "cross_examiner_1")
	if _, err := mutations.HandleBlockWork(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "work.block",
		Params: map[string]any{
			"repository_id": fx.RepositoryID,
			"session_id":    c1Session,
			"job_id":        fx.JobIDs["cross_examiner_1"],
			"lease_id":      c1Lease,
			"severity":      "human_checkpoint",
			"kind":          "needs_operator_decision",
			"description":   "fixture checkpoint",
		},
	}); err != nil {
		t.Fatalf("work.block human checkpoint: %v", err)
	}
	if got := aceSessionState(t, ctx, h, target.sessionID); got != "active" {
		t.Fatalf("target state with sibling consumer pending = %q, want active", got)
	}
	if got := aceEventCount(t, ctx, h, fx, "interrogation.required_skipped"); got != 1 {
		t.Fatalf("required_skipped after waiting_human = %d, want 1", got)
	}

	c2Session, c2Lease, _ := aceClaimAndAck(t, ctx, h, fx, "cross_examiner_2")
	aceCompleteWork(t, ctx, h, fx, "cross_examiner_2", c2Session, c2Lease)
	if got := aceSessionCloseReason(t, ctx, h, target.sessionID); got != "interrogation_window_closed" {
		t.Fatalf("target close_reason after sibling completed = %q, want interrogation_window_closed", got)
	}
	if got := aceEventCount(t, ctx, h, fx, "interrogation.required_skipped"); got != 2 {
		t.Fatalf("required_skipped events = %d, want 2", got)
	}

	res, err := mutations.HandleInterrogationOpen(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.open",
		Params: map[string]any{
			"repository_id":     fx.RepositoryID,
			"session_id":        c1Session,
			"target_session_id": target.sessionID,
			"topic":             "post-close unavailable signal",
		},
	})
	if err != nil {
		t.Fatalf("interrogation.open unavailable signal: %v", err)
	}
	if got := fmt.Sprint(res["status"]); got != "interrogation_unavailable" {
		t.Fatalf("unavailable result status = %q, want interrogation_unavailable; result=%#v", got, res)
	}
	if got := aceEventCount(t, ctx, h, fx, "interrogation.unavailable_signaled"); got != 1 {
		t.Fatalf("unavailable_signaled events = %d, want 1", got)
	}
	aceAssertNoActiveAwaitingWindows(t, ctx, h, fx)
}

type aceTargetWindow struct {
	sessionID string
	attempt   int
}

func seedACEExplicitConsumersRun(t *testing.T, ctx context.Context, h *Harness, repoID string) *aceFixture {
	t.Helper()
	generated, err := workflowgenerate.GenerateFromMap(aceConformanceGeneratorSpec())
	if err != nil {
		t.Fatalf("GenerateFromMap(ACE): %v", err)
	}
	workflow := generated.Workflow
	jobs := aceJobsByID(workflow["jobs"])
	for _, id := range []string{"cross_examiner_1", "cross_examiner_2"} {
		targets := aceList(jobs[id]["interrogation_targets"])
		if len(targets) != 1 || fmt.Sprint(aceMap(targets[0])["workflow_job_id"]) != "convener_draft" {
			t.Fatalf("%s interrogation_targets = %#v, want convener_draft", id, targets)
		}
	}
	if aceHasEdge(workflow["edges"], "convener_draft", "cross_examiner_1") {
		t.Fatal("ACE fixture must not use fake convener_draft -> cross_examiner_1 edge")
	}

	repoRoot := t.TempDir()
	// The generated ACE jobs carry repo_write scopes, so work.complete runs the
	// write-scope git guard against repo_root; it must be a real git work tree.
	initGitRepo(t, repoRoot)
	now := time.Now().UTC()
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'ace-explicit-consumers',$5,16,'active')`,
		repoID, fixtureRepoIdentity, repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	workflowArg, err := db.JSONBArg(h.Runner, workflow)
	if err != nil {
		t.Fatalf("encode workflow: %v", err)
	}
	runID := "run_" + repoID
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,$3,'sha_ace',$4::jsonb,$5)`,
		repoID, "snap_"+runID, workflow["workflow_id"], workflowArg, now); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,$2,$3,$4,'running',$5)`,
		repoID, runID, "snap_"+runID, repoRoot, now); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	seedIDs := []string{
		"convener_draft",
		"convener_synthesis",
		"cross_examiner_1",
		"cross_examiner_2",
		"cross_exam_synthesis",
		"adjudication_intake",
		"adjudicate",
	}
	jobIDs := map[string]string{}
	for _, workflowJobID := range seedIDs {
		jobIDs[workflowJobID] = "job_" + repoID + "_" + workflowJobID
	}
	fx := &aceFixture{
		RepositoryID: repoID,
		RunID:        runID,
		RepoRoot:     repoRoot,
		Workflow:     workflow,
		Jobs:         jobs,
		JobIDs:       jobIDs,
	}
	for _, workflowJobID := range seedIDs {
		state := "blocked"
		if workflowJobID == "convener_draft" {
			state = "queued"
		}
		aceSeedJob(t, ctx, h, fx, workflowJobID, state, now)
	}
	for _, item := range aceList(workflow["edges"]) {
		edge := aceMap(item)
		from := fmt.Sprint(edge["from"])
		to := fmt.Sprint(edge["to"])
		if fx.JobIDs[from] == "" || fx.JobIDs[to] == "" {
			continue
		}
		if err := h.Runner.Exec(ctx, `
			INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
			VALUES ($1,$2,$3)`, repoID, fx.JobIDs[to], fx.JobIDs[from]); err != nil {
			t.Fatalf("seed dependency %s -> %s: %v", from, to, err)
		}
	}
	aceSeedQueueMessage(t, ctx, h, fx, "convener_draft", now)
	return fx
}

func aceConformanceGeneratorSpec() map[string]any {
	lane := func(name string) map[string]any {
		return map[string]any{
			"command":       []any{name, "run"},
			"display_model": name,
			"capabilities":  []any{"claim", "write", "review", "synthesis", "interrogate"},
		}
	}
	return map[string]any{
		"schema_version":   workflowgenerate.GeneratorSchemaVersion,
		"shape":            "adjudicated_constraint_extraction",
		"lane_set":         "multi_review",
		"workflow_id":      "ace-explicit-consumers",
		"name":             "ACE explicit consumers",
		"workflow_version": "2026-06-05",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/ace-explicit-consumers", "allow_dirty": false},
		"scaffold_root":    "workflows/ace-explicit-consumers",
		"artifact_root":    "striatum/ace-explicit-consumers",
		"lanes": map[string]any{
			"author":     lane("author"),
			"reviewer_1": lane("reviewer_1"),
			"reviewer_2": lane("reviewer_2"),
			"reviewer_3": lane("reviewer_3"),
		},
		"options": map[string]any{
			"topic":               "explicit interrogation consumers",
			"reviewer_count":      3,
			"review_postures":     []any{"product", "implementation"},
			"max_revision_cycles": 2,
		},
	}
}

func aceSeedJob(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID, state string, now time.Time) {
	t.Helper()
	job := fx.Jobs[workflowJobID]
	writeScope := map[string]any{"mode": "document_only", "allowed_paths": []any{"striatum/ace-explicit-consumers"}}
	if raw := aceMap(job["write_scope"]); len(raw) > 0 {
		writeScope = raw
	}
	writeScopeArg, err := db.JSONBArg(h.Runner, writeScope)
	if err != nil {
		t.Fatalf("encode write scope: %v", err)
	}
	expectedArg, err := db.JSONBArg(h.Runner, []any{})
	if err != nil {
		t.Fatalf("encode expected artifacts: %v", err)
	}
	laneID := fmt.Sprint(job["lane_id"])
	laneSelectorArg, err := db.JSONBArg(h.Runner, map[string]any{"lane_id": laneID})
	if err != nil {
		t.Fatalf("encode lane selector: %v", err)
	}
	requirementsArg, err := db.JSONBArg(h.Runner, map[string]any{"objective": job["objective"]})
	if err != nil {
		t.Fatalf("encode capability requirements: %v", err)
	}
	// Mirror run.prepare's stored-type mapping: phase_synthesis workflow defs
	// are stored as job_type='review' (jobs_job_type_check forbids the def name).
	jobType := fmt.Sprint(job["type"])
	if jobType == "" || jobType == "<nil>" {
		jobType = "generic"
	}
	if jobType == "phase_synthesis" {
		jobType = "review"
	}
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, capability_requirements_json, created_at
		) VALUES ($1,$2,$3,$4,1,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12::jsonb,$13::jsonb,$14)`,
		fx.RepositoryID, fx.JobIDs[workflowJobID], fx.RunID, workflowJobID, state,
		job["role_id"], job["title"], jobType, "idem_"+fx.JobIDs[workflowJobID],
		writeScopeArg, expectedArg, laneSelectorArg, requirementsArg, now); err != nil {
		t.Fatalf("seed job %s: %v", workflowJobID, err)
	}
}

func aceSeedQueueMessage(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID string, now time.Time) {
	t.Helper()
	job := fx.Jobs[workflowJobID]
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,$7)`,
		fx.RepositoryID, "msg_"+fx.JobIDs[workflowJobID], fx.RunID, fx.JobIDs[workflowJobID],
		job["role_id"], job["lane_id"], now); err != nil {
		t.Fatalf("seed queue message %s: %v", workflowJobID, err)
	}
}

func aceCompleteThroughConvenerGate(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture) aceTargetWindow {
	t.Helper()
	sessionID, leaseID, _ := aceClaimAndAck(t, ctx, h, fx, "convener_draft")
	aceCompleteWork(t, ctx, h, fx, "convener_draft", sessionID, leaseID)
	target := aceLatestTargetWindow(t, ctx, h, fx, "convener_draft")
	if target.sessionID != sessionID {
		t.Fatalf("awaiting target session = %s, want %s", target.sessionID, sessionID)
	}
	// convener_synthesis is a phase_synthesis def stored as job_type='review';
	// production gates complete via review.verdict, not work.complete (#127).
	aceRecordVerdict(t, ctx, h, fx, "convener_synthesis", "accept")
	return target
}

func aceCompleteCrossExaminer(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID, targetSessionID, secret string) {
	t.Helper()
	sessionID, leaseID, packet := aceClaimAndAck(t, ctx, h, fx, workflowJobID)
	attempt := chaosJobAttempt(t, ctx, h, fx.JobIDs["convener_draft"])
	aceAssertTargetProjection(t, packet, targetSessionID, attempt, "available", "")
	// Deliberately leaves the interrogation thread open: the revision cells
	// prove a needs_revision reopen retires a still-open window + threads with
	// reason revision_reopened (HappyPath covers the well-behaved close path).
	intg := aceOpenAskAnswer(t, ctx, h, fx, sessionID, targetSessionID, secret)
	aceAssertAnswerContains(t, ctx, h, fx, intg, secret)
	aceCompleteWork(t, ctx, h, fx, workflowJobID, sessionID, leaseID)
}

func aceCloseInterrogation(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, sessionID, interrogationID string) {
	t.Helper()
	if _, err := mutations.HandleInterrogationClose(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.close",
		Params: map[string]any{
			"repository_id":    fx.RepositoryID,
			"session_id":       sessionID,
			"interrogation_id": interrogationID,
		},
	}); err != nil {
		t.Fatalf("interrogation.close %s: %v", interrogationID, err)
	}
}

func aceCompleteWorkClaimed(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID string) {
	t.Helper()
	sessionID, leaseID, _ := aceClaimAndAck(t, ctx, h, fx, workflowJobID)
	aceCompleteWork(t, ctx, h, fx, workflowJobID, sessionID, leaseID)
}

func aceClaimAndAck(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID string) (string, string, map[string]any) {
	t.Helper()
	jobDef := fx.Jobs[workflowJobID]
	fx.nextOrdinal++
	sessionID := fmt.Sprintf("sess_%s_%s_%d", fx.RepositoryID, workflowJobID, fx.nextOrdinal)
	seedFixtureSession(t, ctx, h.Runner, fx.RepositoryID, fx.RunID, sessionID,
		fmt.Sprint(jobDef["role_id"]), fmt.Sprint(jobDef["lane_id"]),
		[]string{"claim", "write", "review", "synthesis", "interrogate"}, fx.nextOrdinal)
	claim, err := mutations.HandleClaimNext(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "work.claim_next",
		Params:        map[string]any{"repository_id": fx.RepositoryID, "session_id": sessionID},
	})
	if err != nil {
		t.Fatalf("%s claim: %v", workflowJobID, err)
	}
	if got := fmt.Sprint(claim["status"]); got != "claimed" {
		t.Fatalf("%s claim status = %q, want claimed; result=%#v", workflowJobID, got, claim)
	}
	packet := aceMap(claim["packet"])
	packetJob := aceMap(packet["job"])
	if got := fmt.Sprint(packetJob["workflow_job_id"]); got != workflowJobID {
		t.Fatalf("claimed workflow_job_id = %q, want %q; packet=%#v", got, workflowJobID, packetJob)
	}
	lease := aceMap(packet["lease"])
	leaseID := fmt.Sprint(lease["lease_id"])
	messageID := fmt.Sprint(lease["message_id"])
	if _, err := mutations.HandleAckWork(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "work.ack",
		Params: map[string]any{
			"repository_id": fx.RepositoryID,
			"session_id":    sessionID,
			"message_id":    messageID,
			"lease_id":      leaseID,
		},
	}); err != nil {
		t.Fatalf("%s ack: %v", workflowJobID, err)
	}
	return sessionID, leaseID, packet
}

func aceCompleteWork(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID, sessionID, leaseID string) {
	t.Helper()
	if _, err := mutations.HandleCompleteWork(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "work.complete",
		Params: map[string]any{
			"repository_id": fx.RepositoryID,
			"session_id":    sessionID,
			"job_id":        fx.JobIDs[workflowJobID],
			"lease_id":      leaseID,
			"summary":       "fixture completed " + workflowJobID,
		},
	}); err != nil {
		t.Fatalf("%s complete: %v", workflowJobID, err)
	}
}

func aceRecordVerdict(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID, verdict string) map[string]any {
	t.Helper()
	sessionID, leaseID, _ := aceClaimAndAck(t, ctx, h, fx, workflowJobID)
	result, err := mutations.HandleRecordVerdict(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "review.verdict",
		Params: map[string]any{
			"repository_id": fx.RepositoryID,
			"session_id":    sessionID,
			"job_id":        fx.JobIDs[workflowJobID],
			"lease_id":      leaseID,
			"verdict":       verdict,
			"rationale":     "fixture verdict",
		},
	})
	if err != nil {
		t.Fatalf("%s verdict %s: %v", workflowJobID, verdict, err)
	}
	return result
}

func aceOpenAskAnswer(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, interrogatorSessionID, targetSessionID, secret string) string {
	t.Helper()
	openRes, err := mutations.HandleInterrogationOpen(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.open",
		Params: map[string]any{
			"repository_id":     fx.RepositoryID,
			"session_id":        interrogatorSessionID,
			"target_session_id": targetSessionID,
			"topic":             "ACE preserved-context fixture",
		},
	})
	if err != nil {
		t.Fatalf("interrogation.open: %v", err)
	}
	interrogationID := fmt.Sprint(openRes["interrogation_id"])
	if _, err := mutations.HandleInterrogationAsk(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.ask",
		Params: map[string]any{
			"repository_id":    fx.RepositoryID,
			"session_id":       interrogatorSessionID,
			"interrogation_id": interrogationID,
			"body":             "state the convener-only fact",
		},
	}); err != nil {
		t.Fatalf("interrogation.ask: %v", err)
	}
	if _, err := mutations.HandleInterrogationAnswer(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.answer",
		Params: map[string]any{
			"repository_id":    fx.RepositoryID,
			"session_id":       targetSessionID,
			"interrogation_id": interrogationID,
			"body":             "answer from preserved context: " + secret,
		},
	}); err != nil {
		t.Fatalf("interrogation.answer: %v", err)
	}
	return interrogationID
}

func aceLatestTargetWindow(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, workflowJobID string) aceTargetWindow {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `
		SELECT e.actor_session_id, COALESCE((e.payload_json->>'attempt')::int, j.attempt) AS attempt
		  FROM striatumd.events e
		  JOIN striatumd.jobs j ON j.repository_id = e.repository_id AND j.job_id = e.job_id
		 WHERE e.repository_id = $1
		   AND e.run_id = $2
		   AND e.event_type = 'session.awaiting_interrogation'
		   AND j.workflow_job_id = $3
		 ORDER BY e.event_id DESC
		 LIMIT 1`, fx.RepositoryID, fx.RunID, workflowJobID)
	if len(rows) != 1 {
		t.Fatalf("awaiting window rows for %s = %d, want 1", workflowJobID, len(rows))
	}
	return aceTargetWindow{sessionID: fmt.Sprint(rows[0]["actor_session_id"]), attempt: intFromMap(rows[0], "attempt")}
}

func aceAssertTargetProjection(t *testing.T, packet map[string]any, targetSessionID string, attempt int, state, reason string) {
	t.Helper()
	contextBlock := aceMap(packet["context"])
	targets := aceList(contextBlock["interrogation_targets"])
	if len(targets) != 1 {
		t.Fatalf("interrogation_targets = %#v, want 1 entry", contextBlock["interrogation_targets"])
	}
	target := aceMap(targets[0])
	if got := fmt.Sprint(target["workflow_job_id"]); got != "convener_draft" {
		t.Fatalf("target workflow_job_id = %q, want convener_draft; target=%#v", got, target)
	}
	if got := fmt.Sprint(target["state"]); got != state {
		t.Fatalf("target state = %q, want %q; target=%#v", got, state, target)
	}
	if got := fmt.Sprint(target["target_session_id"]); got != targetSessionID {
		t.Fatalf("target_session_id = %q, want %q; target=%#v", got, targetSessionID, target)
	}
	if got := intFromMap(target, "target_attempt"); got != attempt {
		t.Fatalf("target_attempt = %d, want %d; target=%#v", got, attempt, target)
	}
	if reason != "" && fmt.Sprint(target["reason"]) != reason {
		t.Fatalf("target reason = %q, want %q; target=%#v", target["reason"], reason, target)
	}
}

func aceAssertAnswerContains(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, interrogationID, expected string) {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `
		SELECT payload_json->>'body' AS body
		  FROM striatumd.queue_messages
		 WHERE repository_id = $1
		   AND payload_json->>'interrogation_id' = $2
		   AND payload_json->>'turn' = 'answer'
		 ORDER BY created_at DESC
		 LIMIT 1`, fx.RepositoryID, interrogationID)
	if len(rows) != 1 {
		t.Fatalf("answer rows for %s = %d, want 1", interrogationID, len(rows))
	}
	if got := fmt.Sprint(rows[0]["body"]); !strings.Contains(got, expected) {
		t.Fatalf("answer body %q does not contain preserved-context secret %q", got, expected)
	}
}

func aceAssertNoActiveAwaitingWindows(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture) {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `
		SELECT e.actor_session_id
		  FROM striatumd.events e
		  JOIN striatumd.sessions s ON s.repository_id = e.repository_id AND s.session_id = e.actor_session_id
		 WHERE e.repository_id = $1
		   AND e.run_id = $2
		   AND e.event_type = 'session.awaiting_interrogation'
		   AND s.state = 'active'`, fx.RepositoryID, fx.RunID)
	if len(rows) != 0 {
		t.Fatalf("active awaiting_interrogation sessions remain: %#v", rows)
	}
}

func aceSessionState(t *testing.T, ctx context.Context, h *Harness, sessionID string) string {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `SELECT state FROM striatumd.sessions WHERE repository_id=$1 AND session_id=$2`, h.RepositoryID, sessionID)
	if len(rows) != 1 {
		t.Fatalf("session %s rows = %d, want 1", sessionID, len(rows))
	}
	return fmt.Sprint(rows[0]["state"])
}

func aceSessionCloseReason(t *testing.T, ctx context.Context, h *Harness, sessionID string) string {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `SELECT close_reason FROM striatumd.sessions WHERE repository_id=$1 AND session_id=$2`, h.RepositoryID, sessionID)
	if len(rows) != 1 {
		t.Fatalf("session %s rows = %d, want 1", sessionID, len(rows))
	}
	return fmt.Sprint(rows[0]["close_reason"])
}

func aceSessionClosedEventCount(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, sessionID, reason string) int {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `
		SELECT count(*) AS n
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND event_type = 'session.closed'
		   AND actor_session_id = $3
		   AND payload_json->>'reason' = $4`, fx.RepositoryID, fx.RunID, sessionID, reason)
	return intFromMap(rows[0], "n")
}

func aceEventCount(t *testing.T, ctx context.Context, h *Harness, fx *aceFixture, eventType string) int {
	t.Helper()
	rows := chaosQuery(t, ctx, h, `
		SELECT count(*) AS n
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND event_type = $3`, fx.RepositoryID, fx.RunID, eventType)
	return intFromMap(rows[0], "n")
}

func aceJobsByID(value any) map[string]map[string]any {
	jobs := map[string]map[string]any{}
	for _, item := range aceList(value) {
		job := aceMap(item)
		jobs[fmt.Sprint(job["id"])] = job
	}
	return jobs
}

func aceHasEdge(value any, from, to string) bool {
	for _, item := range aceList(value) {
		edge := aceMap(item)
		if edge["from"] == from && edge["to"] == to {
			return true
		}
	}
	return false
}

func aceMap(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case nil:
		return map[string]any{}
	default:
		return map[string]any{}
	}
}

func aceList(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}
