package adapterconformance

// RFC 0105 reliability fixture for RFC 0106's `divergent_ideation` shape (RFC 0087
// + RFC 0129) — the graduation evidence that lets it carry support_tier=`supported`
// (the shape-tier guard, TestSupportedShapesHaveReliabilityFixture, refuses the
// tier without an entry in ReliabilityFixtureShapes for it). It is deliberately NOT
// a re-run of implementation_panel_test.go with renamed jobs: that fixture proves a
// SINGLE fan-out/join followed by a linear tail, but divergent_ideation's
// distinctive lifecycle is a DOUBLE fan-out/join in series — a diverge fan-out into
// N branches, a converge JOIN on all of them, a deepen fan-out into K branches, and
// a final-synthesis JOIN on all of those (catalog.json graph_preview; no cycles —
// traps are a convergence annotation, not a verdict). This file proves that
// structure drives unattended, and — the load-bearing new coverage no existing
// fixture exercises — that a fault injected into a branch of the SECOND fan-out
// (deepen), after the first join has already fired, self-recovers without the final
// join either firing early or losing the recovered branch.
//
// Every node is driven through the PRODUCTION mutation handlers (work.claim_next /
// work.ack / work.complete / session.close) and the real recovery sweep
// (mutations.SweepRun) against the in-process daemon + isolated pgtest database,
// exactly like the sibling fixtures — the real fan-out/join readiness logic
// (maybeEnqueueDownstream → dependenciesSatisfied), not a shortcut. Each node is a
// distinct (role, lane) so claim_next routes each parallel branch to its own
// driving session deterministically even while several are queued at once.
//
// The RFC 0105 gate (recover-or-escalate, never a silent wedge) for this shape: the
// happy cell drives frame → diverge fan-out → converge join → deepen fan-out →
// final join → completion with no operator; the two fault cells inject a hard dead
// lane into one branch of each fan-out and the production sweep requeues it on the
// same attempt while the downstream join correctly stays blocked, a fresh lane
// finishes it, and the run reaches `completed` unattended with no escalation.
//
// PG-gated exactly like the rest of the conformance suite (NewHarness →
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

// divergentIdeationGraph is RFC 0087's divergent_ideation shape graph
// (catalog.json graph_preview), instantiated at branch_count=3 / deepen_count=2:
// frame_problem fans out to three parallel diverge branches, converge joins on all
// three, then deepen fans out to two parallel jobs, and final_synthesis joins on
// both. No cycles.
var divergentIdeationGraph = []panelNode{
	{id: "frame_problem"},
	{id: "branch_1", dependsOn: []string{"frame_problem"}},
	{id: "branch_2", dependsOn: []string{"frame_problem"}},
	{id: "branch_3", dependsOn: []string{"frame_problem"}},
	{id: "converge", dependsOn: []string{"branch_1", "branch_2", "branch_3"}},
	{id: "deepen_1", dependsOn: []string{"converge"}},
	{id: "deepen_2", dependsOn: []string{"converge"}},
	{id: "final_synthesis", dependsOn: []string{"deepen_1", "deepen_2"}},
}

func diRole(nodeID string) string { return "di_role_" + nodeID }
func diLane(nodeID string) string { return "di_lane_" + nodeID }

// diLC tracks the rows the divergent_ideation lifecycle drives.
type diLC struct {
	repoID   string
	runID    string
	jobIDs   map[string]string // nodeID -> jobID
	sessions int               // monotonic; unique session id + ordinal per drive
}

func (lc *diLC) jobID(nodeID string) string { return lc.jobIDs[nodeID] }

func (lc *diLC) newSession(t *testing.T, ctx context.Context, h *Harness, nodeID string) string {
	t.Helper()
	lc.sessions++
	sessionID := fmt.Sprintf("sess_%s_%s_%d", nodeID, lc.repoID, lc.sessions)
	seedFixtureSession(t, ctx, h.Runner, lc.repoID, lc.runID, sessionID,
		diRole(nodeID), diLane(nodeID), []string{"claim", "write"}, lc.sessions)
	return sessionID
}

// seedDivergentIdeationRun seeds a running divergent_ideation run: frame_problem
// queued (claimable) with a pending work message, every downstream node blocked
// with its job_dependencies, so completing a node drives the real
// maybeEnqueueDownstream readiness logic (it enqueues a blocked job only once ALL
// its dependencies are completed). Nodes are document_only draft jobs with no
// required artifact, so work.complete finalizes them without git or publish — the
// reliability question here is the DAG lifecycle, not the artifact contract.
func seedDivergentIdeationRun(t *testing.T, ctx context.Context, h *Harness, repoID string) *diLC {
	t.Helper()
	repoRoot := t.TempDir()
	runID := "run_" + repoID
	now := time.Now().UTC()

	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'divergent-ideation',$5,16,'active')`,
		repoID, fixtureRepoIdentity, repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("seed repository: %v", err)
	}

	// Workflow snapshot (roles + lanes + jobs + edges) for legibility; the
	// lifecycle is driven by the seeded job rows + job_dependencies below.
	roles := map[string]any{}
	lanes := map[string]any{}
	jobs := []any{}
	edges := []any{}
	for _, n := range divergentIdeationGraph {
		roles[diRole(n.id)] = map[string]any{"summary": n.id}
		lanes[diLane(n.id)] = map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}}
		jobs = append(jobs, map[string]any{"id": n.id, "interrogable": false})
		for _, dep := range n.dependsOn {
			edges = append(edges, map[string]any{"from": dep, "to": n.id})
		}
	}
	seedFixtureRun(t, ctx, h.Runner, repoID, runID, map[string]any{
		"workflow_id": fixtureWorkflowID,
		"roles":       roles,
		"lanes":       lanes,
		"jobs":        jobs,
		"edges":       edges,
		"cycles":      []any{},
	})

	writeScope, err := db.JSONBArg(h.Runner, map[string]any{"mode": "document_only", "allowed_paths": []any{"docs"}})
	if err != nil {
		t.Fatalf("encode write_scope: %v", err)
	}
	emptyArtifacts, err := db.JSONBArg(h.Runner, []any{})
	if err != nil {
		t.Fatalf("encode expected_artifacts: %v", err)
	}

	lc := &diLC{repoID: repoID, runID: runID, jobIDs: map[string]string{}}
	for _, n := range divergentIdeationGraph {
		jobID := "job_" + n.id + "_" + repoID
		lc.jobIDs[n.id] = jobID
		state := "blocked"
		if len(n.dependsOn) == 0 {
			state = "queued"
		}
		laneSel, err := db.JSONBArg(h.Runner, map[string]any{"lane_id": diLane(n.id)})
		if err != nil {
			t.Fatalf("encode lane_selector %s: %v", n.id, err)
		}
		if err := h.Runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
			  lane_selector_json, created_at
			) VALUES ($1,$2,$3,$4,1,$5,$6,$7,'draft','idem_'||$2,$8::jsonb,$9::jsonb,$10::jsonb,$11)`,
			repoID, jobID, runID, n.id, state, diRole(n.id),
			"Divergent "+n.id, writeScope, emptyArtifacts, laneSel, now); err != nil {
			t.Fatalf("seed job %s: %v", n.id, err)
		}
		if state == "queued" {
			if err := h.Runner.Exec(ctx, `
				INSERT INTO striatumd.queue_messages (
				  repository_id, message_id, run_id, job_id, kind, state, priority,
				  target_role_id, target_lane_id, created_at, updated_at
				) VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,$7)`,
				repoID, "msg_"+jobID, runID, jobID, diRole(n.id), diLane(n.id), now); err != nil {
				t.Fatalf("seed queue message %s: %v", n.id, err)
			}
		}
		for _, dep := range n.dependsOn {
			if err := h.Runner.Exec(ctx, `
				INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
				VALUES ($1,$2,$3)`, repoID, jobID, "job_"+dep+"_"+repoID); err != nil {
				t.Fatalf("seed dependency %s<-%s: %v", n.id, dep, err)
			}
		}
	}
	return lc
}

// diDriveComplete registers a fresh session for the node's role/lane, claims + acks
// the node's queued job, completes it through work.complete, and closes the session
// (a finished lane closes).
func diDriveComplete(t *testing.T, ctx context.Context, h *Harness, lc *diLC, nodeID string) {
	t.Helper()
	sessionID := lc.newSession(t, ctx, h, nodeID)
	leaseID := claimAndAck(t, ctx, h, lc.repoID, sessionID, nodeID)
	if _, err := mutations.HandleCompleteWork(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "work.complete",
		Params: map[string]any{"repository_id": lc.repoID, "session_id": sessionID, "job_id": lc.jobID(nodeID), "lease_id": leaseID, "summary": nodeID + " done"},
	}); err != nil {
		t.Fatalf("complete %s: %v", nodeID, err)
	}
	if _, err := mutations.HandleCloseSession(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "session.close",
		Params: map[string]any{"repository_id": lc.repoID, "session_id": sessionID, "reason": "lane_finished_job"},
	}); err != nil {
		t.Fatalf("close %s: %v", nodeID, err)
	}
}

// diClaimLeaveLive registers a fresh session, claims + acks the node's job, and
// returns the session id WITHOUT completing — a live claimant a fault injector can
// kill.
func diClaimLeaveLive(t *testing.T, ctx context.Context, h *Harness, lc *diLC, nodeID string) string {
	t.Helper()
	sessionID := lc.newSession(t, ctx, h, nodeID)
	claimAndAck(t, ctx, h, lc.repoID, sessionID, nodeID)
	return sessionID
}

// TestDivergentIdeationHappyPathCompletes drives the full divergent_ideation graph
// to `completed` with NO fault and NO operator, asserting BOTH joins hold correctly:
// the diverge fan-out enqueues all three branches at once, converge holds until the
// last branch completes, the deepen fan-out enqueues both deepen jobs at once, and
// final_synthesis holds until the last deepen completes. This is the baseline the
// fault cells perturb.
func TestDivergentIdeationHappyPathCompletes(t *testing.T) {
	repoID := "repo_divideation_happy"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedDivergentIdeationRun(t, ctx, h, repoID)

	// Only frame_problem is claimable; everything downstream is blocked.
	if got := chaosJobState(t, ctx, h, lc.jobID("frame_problem")); got != "queued" {
		t.Fatalf("frame_problem state = %q, want queued", got)
	}
	for _, n := range []string{"branch_1", "branch_2", "branch_3", "converge", "deepen_1", "deepen_2", "final_synthesis"} {
		if got := chaosJobState(t, ctx, h, lc.jobID(n)); got != "blocked" {
			t.Fatalf("%s state = %q, want blocked (pre-fan-out)", n, got)
		}
	}

	// Diverge fan-out: completing frame_problem must enqueue ALL THREE branches.
	diDriveComplete(t, ctx, h, lc, "frame_problem")
	for _, n := range []string{"branch_1", "branch_2", "branch_3"} {
		if got := chaosJobState(t, ctx, h, lc.jobID(n)); got != "queued" {
			t.Fatalf("post-frame %s state = %q, want queued (diverge fan-out must enqueue every branch)", n, got)
		}
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("converge")); got != "blocked" {
		t.Fatalf("converge state = %q, want blocked (join must wait for all branches)", got)
	}

	// Converge join: stays blocked until the LAST branch completes.
	diDriveComplete(t, ctx, h, lc, "branch_1")
	if got := chaosJobState(t, ctx, h, lc.jobID("converge")); got != "blocked" {
		t.Fatalf("converge state = %q after 1/3 branches, want blocked", got)
	}
	diDriveComplete(t, ctx, h, lc, "branch_2")
	if got := chaosJobState(t, ctx, h, lc.jobID("converge")); got != "blocked" {
		t.Fatalf("converge state = %q after 2/3 branches, want blocked", got)
	}
	diDriveComplete(t, ctx, h, lc, "branch_3")
	if got := chaosJobState(t, ctx, h, lc.jobID("converge")); got != "queued" {
		t.Fatalf("converge state = %q after 3/3 branches, want queued (join must fire once all predecessors complete)", got)
	}

	// Deepen fan-out: completing converge must enqueue BOTH deepen jobs at once.
	diDriveComplete(t, ctx, h, lc, "converge")
	for _, n := range []string{"deepen_1", "deepen_2"} {
		if got := chaosJobState(t, ctx, h, lc.jobID(n)); got != "queued" {
			t.Fatalf("post-converge %s state = %q, want queued (deepen fan-out must enqueue every deepen job)", n, got)
		}
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("final_synthesis")); got != "blocked" {
		t.Fatalf("final_synthesis state = %q, want blocked (final join must wait for all deepen jobs)", got)
	}

	// Final join: stays blocked until the LAST deepen completes.
	diDriveComplete(t, ctx, h, lc, "deepen_1")
	if got := chaosJobState(t, ctx, h, lc.jobID("final_synthesis")); got != "blocked" {
		t.Fatalf("final_synthesis state = %q after 1/2 deepen, want blocked", got)
	}
	diDriveComplete(t, ctx, h, lc, "deepen_2")
	if got := chaosJobState(t, ctx, h, lc.jobID("final_synthesis")); got != "queued" {
		t.Fatalf("final_synthesis state = %q after 2/2 deepen, want queued (final join must fire once all deepen complete)", got)
	}

	diDriveComplete(t, ctx, h, lc, "final_synthesis")
	if got := chaosRunState(t, ctx, h, lc.runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (the divergent ideation drove to completion unattended)", got)
	}
}

// TestDivergentIdeationDivergeBranchDeathSelfRecovers is the RFC 0105 fault cell for
// the FIRST fan-out/join: two of three diverge branches finish, the third's lane
// dies hard mid-flight, and the production recovery sweep requeues that branch on
// the same attempt with no operator and no escalation. The converge join must NOT
// fire while the recovered branch is down, and once a fresh lane completes it the
// join fires and the run reaches `completed` unattended.
func TestDivergentIdeationDivergeBranchDeathSelfRecovers(t *testing.T) {
	repoID := "repo_divideation_diverge_death"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedDivergentIdeationRun(t, ctx, h, repoID)

	diDriveComplete(t, ctx, h, lc, "frame_problem")
	diDriveComplete(t, ctx, h, lc, "branch_1")
	diDriveComplete(t, ctx, h, lc, "branch_3")
	if got := chaosJobState(t, ctx, h, lc.jobID("converge")); got != "blocked" {
		t.Fatalf("converge state = %q, want blocked (branch_2 still in flight)", got)
	}

	// branch_2's lane claims + acks, then dies hard.
	priorAttempt := chaosJobAttempt(t, ctx, h, lc.jobID("branch_2"))
	deadSession := diClaimLeaveLive(t, ctx, h, lc, "branch_2")
	injectDeadLane(t, ctx, h.Runner, repoID, deadSession, lc.jobID("branch_2"))
	if got := chaosJobState(t, ctx, h, lc.jobID("branch_2")); got != "running" {
		t.Fatalf("pre-sweep branch_2 state = %q, want running (limbo after lane death)", got)
	}

	summary := runSweep(t, ctx, h, lc.runID)
	if summary.escalationCount != 0 {
		t.Fatalf("diverge-branch death raised %d escalations, want 0 (self-recover); result=%#v", summary.escalationCount, summary.result)
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("branch_2")); got != "queued" {
		t.Fatalf("post-sweep branch_2 state = %q, want queued (requeued); acted_count=%d", got, summary.actedCount)
	}
	if got := chaosJobAttempt(t, ctx, h, lc.jobID("branch_2")); got != priorAttempt {
		t.Fatalf("branch_2 attempt = %d, want %d (autonomous requeue must not bump attempt)", got, priorAttempt)
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("converge")); got != "blocked" {
		t.Fatalf("converge state = %q during branch_2 recovery, want blocked (the join must not lose the recovered branch)", got)
	}

	// A fresh lane completes the recovered branch; the run drives to completion.
	diDriveComplete(t, ctx, h, lc, "branch_2")
	if got := chaosJobState(t, ctx, h, lc.jobID("converge")); got != "queued" {
		t.Fatalf("converge state = %q after recovered branch_2 completed, want queued (join must fire post-recovery)", got)
	}
	diDriveComplete(t, ctx, h, lc, "converge")
	diDriveComplete(t, ctx, h, lc, "deepen_1")
	diDriveComplete(t, ctx, h, lc, "deepen_2")
	diDriveComplete(t, ctx, h, lc, "final_synthesis")
	if got := chaosRunState(t, ctx, h, lc.runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (self-recovered from a diverge-branch death, no operator)", got)
	}
	if escs := chaosRecoveryExhaustedEscalations(t, ctx, h, lc.runID); len(escs) != 0 {
		t.Fatalf("a self-recovering run raised %d recovery_exhausted escalations, want 0", len(escs))
	}
}

// TestDivergentIdeationDeepenBranchDeathSelfRecovers is the load-bearing NEW
// coverage: a fault in the SECOND fan-out/join (deepen → final_synthesis), AFTER the
// first join (converge) has already fired. No existing fixture exercises a fault in
// a second serial fan-out. One deepen branch's lane dies hard; the sweep requeues it
// on the same attempt with no escalation; final_synthesis must stay blocked until a
// fresh lane completes the recovered deepen branch, then the run reaches `completed`
// unattended.
func TestDivergentIdeationDeepenBranchDeathSelfRecovers(t *testing.T) {
	repoID := "repo_divideation_deepen_death"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedDivergentIdeationRun(t, ctx, h, repoID)

	// Drive the whole diverge phase + converge so the deepen fan-out is live.
	diDriveComplete(t, ctx, h, lc, "frame_problem")
	diDriveComplete(t, ctx, h, lc, "branch_1")
	diDriveComplete(t, ctx, h, lc, "branch_2")
	diDriveComplete(t, ctx, h, lc, "branch_3")
	diDriveComplete(t, ctx, h, lc, "converge")
	// Deepen fan-out is live: deepen_1 finishes; deepen_2's lane will die.
	diDriveComplete(t, ctx, h, lc, "deepen_1")
	if got := chaosJobState(t, ctx, h, lc.jobID("final_synthesis")); got != "blocked" {
		t.Fatalf("final_synthesis state = %q, want blocked (deepen_2 still in flight)", got)
	}

	priorAttempt := chaosJobAttempt(t, ctx, h, lc.jobID("deepen_2"))
	deadSession := diClaimLeaveLive(t, ctx, h, lc, "deepen_2")
	injectDeadLane(t, ctx, h.Runner, repoID, deadSession, lc.jobID("deepen_2"))

	summary := runSweep(t, ctx, h, lc.runID)
	if summary.escalationCount != 0 {
		t.Fatalf("deepen-branch death raised %d escalations, want 0 (self-recover); result=%#v", summary.escalationCount, summary.result)
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("deepen_2")); got != "queued" {
		t.Fatalf("post-sweep deepen_2 state = %q, want queued (requeued); acted_count=%d", got, summary.actedCount)
	}
	if got := chaosJobAttempt(t, ctx, h, lc.jobID("deepen_2")); got != priorAttempt {
		t.Fatalf("deepen_2 attempt = %d, want %d (autonomous requeue must not bump attempt)", got, priorAttempt)
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("final_synthesis")); got != "blocked" {
		t.Fatalf("final_synthesis state = %q during deepen_2 recovery, want blocked (the final join must not lose the recovered deepen branch)", got)
	}

	diDriveComplete(t, ctx, h, lc, "deepen_2")
	if got := chaosJobState(t, ctx, h, lc.jobID("final_synthesis")); got != "queued" {
		t.Fatalf("final_synthesis state = %q after recovered deepen_2 completed, want queued (final join must fire post-recovery)", got)
	}
	diDriveComplete(t, ctx, h, lc, "final_synthesis")
	if got := chaosRunState(t, ctx, h, lc.runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (self-recovered from a deepen-branch death in the second fan-out, no operator)", got)
	}
	if escs := chaosRecoveryExhaustedEscalations(t, ctx, h, lc.runID); len(escs) != 0 {
		t.Fatalf("a self-recovering run raised %d recovery_exhausted escalations, want 0", len(escs))
	}
}
