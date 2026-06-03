package adapterconformance

// RFC 0105 — the standing unattended-reliability gate for the FULL multi-lane
// revision lifecycle (the #65/#84/#120/#121/#131 family). Where chaos_test.go
// proves a single-job repo-write lane self-recovers-or-escalates under fault,
// this file proves the same property for the real two-lane, review-gated,
// needs_revision-cycle lifecycle:
//
//   implement (att1) -> review -> needs_revision -> re-open implement (att2)
//   -> re-implement -> re-review -> accept -> run completed
//
// Both lanes are driven through the PRODUCTION mutation handlers (claim / ack /
// complete / submit-verdict) against the same in-process daemon + isolated
// pgtest database the conformance harness boots — the real state machine, not a
// shortcut. (The fake-agent MCP protocol fidelity is covered by the C0..C12
// conformance + chaos suites; this fixture's new coverage is the multi-lane
// revision CYCLE and its fault recovery.) The fault matrix (lane death;
// budget-exhaustion) reuses the deterministic time-warp seam from chaos_test.go
// (injectDeadLane + runSweep).
//
// The gate (RFC 0105 §3): each (shape x fault) cell either self-recovers to
// `completed` with the attempt preserved, OR escalates loudly (needs_operator +
// a recovery_exhausted escalation_inbox row) within budget — NEVER a silent
// wedge. A reverted RFC 0104 run lock turns this red the same way it turns the
// claim/verdict deadlock regression red.
//
// PG-gated exactly like the rest of the conformance suite (NewHarness ->
// pgtest.Pool SKIPs without STRIATUM_PG_TEST_URL).

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// revisionLC identifies the rows the multi-lane revision lifecycle drives.
type revisionLC struct {
	fx               *Fixture // implement job's fixture (role/lane)
	reviewJobID      string
	reviewerRole     string
	reviewerLane     string
	implementerRound int // each implementer session id is unique per attempt
	verdictRound     int // each reviewer session id is unique per round
}

const (
	revisionImplementWorkflowJob = "implement"
	revisionReviewWorkflowJob    = "review_job"
)

// seedRevisionLifecycleRun seeds a two-lane revision-cycle run: a document_only
// `implement` job (queued, claimable by the implementer lane) and a `review`
// job (blocked, depends on implement) gated behind it, plus a declared workflow
// cycle (review -> implement on needs_revision, max_iterations 1). The implement
// artifact is non-required so the att2 re-completion does not depend on the
// separately-tested per-attempt artifact-freshness contract (RFC 0095/0100); the
// focus here is lifecycle reliability under fault.
func seedRevisionLifecycleRun(t *testing.T, ctx context.Context, h *Harness, repoID string) *revisionLC {
	t.Helper()
	repoRoot := t.TempDir()
	role := "implementer"
	lane := "claude_code"
	reviewerRole := "reviewer"
	reviewerLane := "reviewer_lane"
	runID := "run_" + repoID
	jobID := "job_implement_" + repoID
	reviewJobID := "job_review_" + repoID

	artifactRepoPath := "docs/revision-note.md"
	writeRepoFile(t, repoRoot, artifactRepoPath, fixtureArtifactBody())

	now := time.Now().UTC()
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'revision-lifecycle',$5,16,'active')`,
		repoID, fixtureRepoIdentity, repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("seed repository: %v", err)
	}

	writeScope := map[string]any{"mode": "document_only", "allowed_paths": []any{"docs"}}
	expectedArtifacts := []any{map[string]any{
		"logical_name": "revision_note", "kind": "progress_note", "path": artifactRepoPath, "required": false,
	}}
	workflow := map[string]any{
		"workflow_id": fixtureWorkflowID,
		"roles": map[string]any{
			role:         map[string]any{"summary": "implement a small change"},
			reviewerRole: map[string]any{"summary": "review the change"},
		},
		"lanes": map[string]any{
			lane:         map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}},
			reviewerLane: map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "review"}},
		},
		"jobs": []any{
			map[string]any{"id": revisionImplementWorkflowJob, "interrogable": false},
			map[string]any{"id": revisionReviewWorkflowJob, "type": "review", "role_id": reviewerRole},
		},
		"cycles": []any{map[string]any{
			"from": revisionReviewWorkflowJob, "to": revisionImplementWorkflowJob,
			"on_verdict": "needs_revision", "max_iterations": 1, "allow_same_lane": true,
		}},
	}

	seedFixtureRun(t, ctx, h.Runner, repoID, runID, workflow)
	seedFixtureJob(t, ctx, h.Runner, repoID, runID, jobID, role, lane, writeScope, expectedArtifacts, now)
	// Point the implement job at the cycle target workflow_job_id.
	if err := h.Runner.Exec(ctx, `UPDATE striatumd.jobs SET workflow_job_id=$3 WHERE repository_id=$1 AND job_id=$2`,
		repoID, jobID, revisionImplementWorkflowJob); err != nil {
		t.Fatalf("set implement workflow_job_id: %v", err)
	}
	// The review job: blocked, verdict-capable, depends on implement, so the
	// implementer's work.complete -> maybeEnqueueDownstream enqueues it.
	reviewLaneSel, err := db.JSONBArg(h.Runner, map[string]any{"lane_id": reviewerLane})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,$4,1,'blocked',$5,'Review','review','idem_'||$2,'{}'::jsonb,'[]'::jsonb,$6::jsonb,$7)`,
		repoID, reviewJobID, runID, revisionReviewWorkflowJob, reviewerRole, reviewLaneSel, now); err != nil {
		t.Fatalf("seed review job: %v", err)
	}
	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`, repoID, reviewJobID, jobID); err != nil {
		t.Fatalf("seed job dependency: %v", err)
	}

	return &revisionLC{
		fx: &Fixture{
			RepositoryID: repoID, RunID: runID, Role: role, Lane: lane, JobID: jobID, RepoRoot: repoRoot,
			ArtifactKind: "progress_note", ArtifactLogicalName: "revision_note",
			ArtifactRepoPath: artifactRepoPath, ArtifactAbsPath: filepath.Join(repoRoot, artifactRepoPath),
		},
		reviewJobID: reviewJobID, reviewerRole: reviewerRole, reviewerLane: reviewerLane,
	}
}

// claimAndAck claims the one queued job matching a session's role/lane and acks
// it through the production handlers, returning the lease id.
func claimAndAck(t *testing.T, ctx context.Context, h *Harness, repoID, sessionID, who string) (leaseID string) {
	t.Helper()
	claim, err := mutations.HandleClaimNext(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "work.claim_next",
		Params: map[string]any{"repository_id": repoID, "session_id": sessionID},
	})
	if err != nil {
		t.Fatalf("%s claim: %v", who, err)
	}
	if fmt.Sprint(claim["status"]) != "claimed" {
		t.Fatalf("%s claim returned %v, want claimed (work was not enqueued)", who, claim["status"])
	}
	packet, _ := claim["packet"].(map[string]any)
	lease, _ := packet["lease"].(map[string]any)
	leaseID = fmt.Sprint(lease["lease_id"])
	messageID := fmt.Sprint(lease["message_id"])
	if _, err := mutations.HandleAckWork(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "work.ack",
		Params: map[string]any{"repository_id": repoID, "session_id": sessionID, "message_id": messageID, "lease_id": leaseID},
	}); err != nil {
		t.Fatalf("%s ack: %v", who, err)
	}
	return leaseID
}

// driveImplementer registers a fresh implementer session and drives it through
// claim -> ack (-> complete) via the production handlers (the implement job id
// is the stable lc.fx.JobID across attempts — reopenJobForAttempt re-uses the
// row). With complete=false it stops after ack (a live claimant the fault
// injector can kill). Returns the session id.
func driveImplementer(t *testing.T, ctx context.Context, h *Harness, lc *revisionLC, complete bool) (sessionID string) {
	t.Helper()
	lc.implementerRound++
	sessionID = fmt.Sprintf("sess_impl_%s_%d", lc.fx.RepositoryID, lc.implementerRound)
	seedFixtureSession(t, ctx, h.Runner, lc.fx.RepositoryID, lc.fx.RunID, sessionID,
		lc.fx.Role, lc.fx.Lane, []string{"claim", "write"}, lc.implementerRound)
	leaseID := claimAndAck(t, ctx, h, lc.fx.RepositoryID, sessionID, "implementer")
	if !complete {
		return sessionID
	}
	if _, err := mutations.HandleCompleteWork(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "work.complete",
		Params: map[string]any{"repository_id": lc.fx.RepositoryID, "session_id": sessionID, "job_id": lc.fx.JobID, "lease_id": leaseID, "summary": "implemented"},
	}); err != nil {
		t.Fatalf("implementer complete (round %d): %v", lc.implementerRound, err)
	}
	// A completed implementer lane closes its session (a fresh session does any
	// revision re-work). This also keeps the recovery decision tree's dead-lane
	// resolution deterministic: a re-opened job carries released leases from every
	// prior attempt's session, so leaving a prior session 'active' could make the
	// "latest lease owner" resolve to a live session and mask the dead att2 lane.
	if _, err := mutations.HandleCloseSession(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "session.close",
		Params: map[string]any{"repository_id": lc.fx.RepositoryID, "session_id": sessionID, "reason": "lane_finished_job"},
	}); err != nil {
		t.Fatalf("implementer close (round %d): %v", lc.implementerRound, err)
	}
	return sessionID
}

// driveReviewerVerdict registers a fresh reviewer session, claims + acks the
// enqueued review job, and records the verdict (needs_revision routes the cycle;
// accept completes the run).
func driveReviewerVerdict(t *testing.T, ctx context.Context, h *Harness, lc *revisionLC, verdict string) map[string]any {
	t.Helper()
	lc.verdictRound++
	sessionID := fmt.Sprintf("sess_reviewer_%s_%d", lc.fx.RepositoryID, lc.verdictRound)
	seedFixtureSession(t, ctx, h.Runner, lc.fx.RepositoryID, lc.fx.RunID, sessionID,
		lc.reviewerRole, lc.reviewerLane, []string{"claim", "review"}, lc.verdictRound)
	leaseID := claimAndAck(t, ctx, h, lc.fx.RepositoryID, sessionID, "reviewer")
	result, err := mutations.HandleRecordVerdict(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "review.verdict",
		Params: map[string]any{
			"repository_id": lc.fx.RepositoryID, "session_id": sessionID,
			"job_id": lc.reviewJobID, "lease_id": leaseID, "verdict": verdict,
		},
	})
	if err != nil {
		t.Fatalf("reviewer verdict %q (round %d): %v", verdict, lc.verdictRound, err)
	}
	return result
}

// TestRevisionLifecycleHappyPathCompletes drives the full two-lane,
// review-gated, needs_revision-cycle lifecycle to `completed` with NO fault and
// NO operator action — the baseline the fault cells perturb.
func TestRevisionLifecycleHappyPathCompletes(t *testing.T) {
	repoID := "repo_revlife_happy"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedRevisionLifecycleRun(t, ctx, h, repoID)

	// att1: implementer completes -> review enqueued -> needs_revision routes back.
	driveImplementer(t, ctx, h, lc, true)
	routed := driveReviewerVerdict(t, ctx, h, lc, "needs_revision")
	if got := fmt.Sprint(routed["status"]); got != "revision_routed" {
		t.Fatalf("needs_revision status = %q, want revision_routed (the cycle did not fire)", got)
	}
	if got := chaosJobState(t, ctx, h, lc.fx.JobID); got != "queued" {
		t.Fatalf("post-needs_revision implement state = %q, want queued (re-opened for att2)", got)
	}

	// att2: re-implement -> re-review -> accept -> run completes.
	driveImplementer(t, ctx, h, lc, true)
	accepted := driveReviewerVerdict(t, ctx, h, lc, "accept")
	if got := fmt.Sprint(accepted["status"]); got != "completed" {
		t.Fatalf("accept status = %q, want completed", got)
	}
	if got := chaosRunState(t, ctx, h, lc.fx.RunID); got != "completed" {
		t.Fatalf("run state = %q, want completed (unattended, no operator)", got)
	}
}

// TestRevisionLifecycleLaneDeathSelfRecovers is the fault-matrix self-recover
// cell: the att2 implementer (re-working after a needs_revision) dies mid-task;
// the production recovery sweep requeues its job on the SAME attempt (no
// operator, no escalation), a fresh implementer completes the revision, and the
// re-review accepts — the run reaches `completed` unattended.
func TestRevisionLifecycleLaneDeathSelfRecovers(t *testing.T) {
	repoID := "repo_revlife_death"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedRevisionLifecycleRun(t, ctx, h, repoID)

	driveImplementer(t, ctx, h, lc, true)
	driveReviewerVerdict(t, ctx, h, lc, "needs_revision")
	priorAttempt := chaosJobAttempt(t, ctx, h, lc.fx.JobID)

	// att2 implementer claims + acks the re-opened job, then its lane dies.
	deadSession := driveImplementer(t, ctx, h, lc, false)
	injectDeadLane(t, ctx, h.Runner, repoID, deadSession, lc.fx.JobID)
	if got := chaosJobState(t, ctx, h, lc.fx.JobID); got != "running" {
		t.Fatalf("pre-sweep implement state = %q, want running (limbo after lane death)", got)
	}

	summary := runSweep(t, ctx, h, lc.fx.RunID)
	if summary.escalationCount != 0 {
		t.Fatalf("first dead-lane recovery raised %d escalations, want 0 (self-recover); result=%#v", summary.escalationCount, summary.result)
	}
	if got := chaosJobState(t, ctx, h, lc.fx.JobID); got != "queued" {
		t.Fatalf("post-sweep implement state = %q, want queued (requeued); sweep acted_count=%d", got, summary.actedCount)
	}
	if got := chaosJobAttempt(t, ctx, h, lc.fx.JobID); got != priorAttempt {
		t.Fatalf("attempt = %d, want %d (autonomous requeue must not bump attempt)", got, priorAttempt)
	}
	if got := chaosRunState(t, ctx, h, lc.fx.RunID); got != "running" {
		t.Fatalf("run = %q, want running (no operator needed)", got)
	}

	// A fresh implementer completes the revision; the re-review accepts.
	driveImplementer(t, ctx, h, lc, true)
	accepted := driveReviewerVerdict(t, ctx, h, lc, "accept")
	if got := fmt.Sprint(accepted["status"]); got != "completed" {
		t.Fatalf("accept status = %q, want completed", got)
	}
	if got := chaosRunState(t, ctx, h, lc.fx.RunID); got != "completed" {
		t.Fatalf("run = %q, want completed (self-recovered, no operator)", got)
	}
}

// TestRevisionLifecycleUnrecoverableEscalatesLoudly is the fault-matrix
// escalate-loud cell: the att2 implementer dies repeatedly past the per-job
// requeue budget. Within a bounded number of sweeps the run must flip to
// needs_operator with a recovery_exhausted escalation_inbox row — NEVER a silent
// 'running'. The loop is capped so a non-escalating (silent-wedge) regression
// fails loudly via the cap rather than hanging.
func TestRevisionLifecycleUnrecoverableEscalatesLoudly(t *testing.T) {
	repoID := "repo_revlife_escalate"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedRevisionLifecycleRun(t, ctx, h, repoID)

	driveImplementer(t, ctx, h, lc, true)
	driveReviewerVerdict(t, ctx, h, lc, "needs_revision")

	const maxCycles = 6 // default max_requeues=2 -> escalate by cycle 3; cap guards a silent wedge.
	escalated := false
	for cycle := 1; cycle <= maxCycles; cycle++ {
		deadSession := driveImplementer(t, ctx, h, lc, false)
		injectDeadLane(t, ctx, h.Runner, repoID, deadSession, lc.fx.JobID)
		summary := runSweep(t, ctx, h, lc.fx.RunID)
		if summary.escalationCount > 0 || chaosRunState(t, ctx, h, lc.fx.RunID) == "needs_operator" {
			escalated = true
			break
		}
		if got := chaosJobState(t, ctx, h, lc.fx.JobID); got != "queued" {
			t.Fatalf("cycle %d: implement state = %q, want queued (still self-recovering, pre-budget)", cycle, got)
		}
	}
	if !escalated {
		t.Fatalf("revision run never escalated after %d dead-lane cycles: a budget-exhausted run sat silently (the silent-wedge regression)", maxCycles)
	}
	if got := chaosRunState(t, ctx, h, lc.fx.RunID); got != "needs_operator" {
		t.Fatalf("run = %q, want needs_operator after budget exhaustion", got)
	}
	escs := chaosRecoveryExhaustedEscalations(t, ctx, h, lc.fx.RunID)
	if len(escs) != 1 {
		t.Fatalf("recovery_exhausted escalation count = %d, want 1", len(escs))
	}
	if got := fmt.Sprint(escs[0]["ei_state"]); got != "pending" {
		t.Fatalf("escalation state = %q, want pending", got)
	}
}

// ReliabilityFixtureShapes lists the workflow-generator shape names that have a
// green RFC 0105 reliability fixture in this package, so RFC 0106's graduation
// guard can refuse a `supported` tier for any shape without one (the tier cannot
// lie). Coverage:
//   - the single-job repo-write lane (chaos_test.go) underwrites `minimal` and
//     `code_change`;
//   - the two-lane review-gated needs_revision-cycle lifecycle (this file)
//     underwrites `review` and `multi_review_synthesis`.
//
// The collaboration / interrogation shapes (implementation_panel, conversation,
// falsification_gate, cross_examination, adjudicated_constraint_extraction,
// multi_phase, human_checkpoint, evidence_backed, custom) deliberately have NO
// entry here and therefore stay `experimental` until a fixture proves them.
var ReliabilityFixtureShapes = map[string]bool{
	"minimal":                true,
	"review":                 true,
	"code_change":            true,
	"multi_review_synthesis": true,
}
