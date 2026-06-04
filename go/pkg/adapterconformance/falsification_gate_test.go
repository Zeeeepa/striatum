package adapterconformance

// RFC 0105 reliability fixture for RFC 0106's `falsification_gate` shape — the
// graduation evidence that lets it carry support_tier=`supported` (the shape-tier
// guard, TestSupportedShapesHaveReliabilityFixture, refuses the tier without an
// entry in ReliabilityFixtureShapes for it). It is deliberately NOT a re-run of
// the existing fixtures with renamed jobs:
//
//   - chaos_test.go proves a single-job repo-write lane self-recovers-or-escalates;
//   - lifecycle_revision_test.go proves a two-lane review→needs_revision cycle whose
//     reopen re-blocks ONE downstream job (the review, depth 1);
//   - implementation_panel_test.go proves a forward-only fan-out/join (NO cycle).
//
// falsification_gate's distinctive lifecycle is a needs_revision cycle whose
// reopen must re-cascade a MULTI-JOB linear dialogue chain. Its graph (the
// generator's compileFalsificationGate) is
//
//   holder → falsifier_1 → falsifier_2 → adjudicate → commit → final
//
// where the adjudicator gate is a verdict-capable job (production authors it as a
// `phase_synthesis` workflow-job def, which run.prepare stores as job_type='review'
// since the DB jobs_job_type_check forbids 'phase_synthesis'; the fixture seeds
// that stored shape), and the declared cycle `adjudicate --needs_revision-->
// falsifier_1` re-opens the FIRST dialogue node. The load-bearing new coverage:
//
//  1. Re-opening falsifier_1 must transitively re-block every downstream terminal
//     job — falsifier_2 (depth 1) AND the adjudicate gate itself (depth 2) — in one
//     reopen. This exercises resetDownstreamForRevision's RECURSIVE CTE past the
//     depth-1 base case the single-job lifecycle_revision reopen stops at, and is
//     the exact "re-open every downstream job, not just the one that never voted"
//     mechanism the RFC 0095 panel-revision wedge family (#65) regressed on.
//  2. The revision then re-cascades the whole chain back to a clearing verdict
//     (falsifier_1 → falsifier_2 → adjudicate re-run in order via repeated
//     maybeEnqueueDownstream), and the fault cell injects a hard dead lane into a
//     MIDDLE dialogue job (falsifier_2) DURING that re-cascade — a fault landing
//     mid-chain inside a revision cycle that neither the single-job nor the
//     forward-only-panel fixtures reach.
//
// Both cells are driven through the PRODUCTION mutation handlers (work.claim_next /
// work.ack / work.complete / review.verdict / session.close) and the real recovery
// sweep (mutations.SweepRun) against the in-process daemon + isolated pgtest
// database, exactly like the sibling fixtures — the real cycle-routing
// (routeRevisionCycle → reopenJobForAttempt → resetDownstreamForRevision) and
// fan-in gating (maybeEnqueueDownstream → dependenciesSatisfied), not a shortcut.
// Each node is a distinct (role, lane) so claim_next routes deterministically.
//
// The RFC 0105 gate (recover-or-escalate, never a silent wedge) for this shape:
// the happy cell drives the chain → a needs_revision re-cascade → a clearing
// verdict → completion with no operator; the mid-chain-death cell injects a hard
// dead lane into falsifier_2 during the revision re-cascade and the production
// sweep requeues it on the same attempt while the adjudicate gate correctly stays
// blocked (the re-cascade is not lost), a fresh lane finishes it, and the run
// reaches `completed` unattended.
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

// fgNode is one node of the falsification_gate graph: its job type (which decides
// whether it completes via work.complete or a review.verdict) and the upstream
// nodes it depends on (its job_dependencies).
type fgNode struct {
	id        string
	jobType   string // STORED job_type (DB-legal): "draft" => work.complete; "review" => review.verdict gate
	dependsOn []string
}

// falsificationGateGraph mirrors the generator's compileFalsificationGate graph:
// a linear holder→falsifiers dialogue chain into the adjudicator gate, then a
// commit→final tail. The bounded cycle (declared in the seeded workflow snapshot)
// routes adjudicate --needs_revision--> falsifier_1. The adjudicator is a
// verdict-capable gate stored as job_type='review' — production authors it as a
// `phase_synthesis` workflow-job def, but run.prepare stores it as 'review'
// (HandleRecordVerdict accepts both; the DB jobs_job_type_check forbids
// 'phase_synthesis'), so seeding 'review' is the faithful stored shape.
var falsificationGateGraph = []fgNode{
	{id: "holder", jobType: "draft"},
	{id: "falsifier_1", jobType: "draft", dependsOn: []string{"holder"}},
	{id: "falsifier_2", jobType: "draft", dependsOn: []string{"falsifier_1"}},
	{id: "adjudicate", jobType: "review", dependsOn: []string{"falsifier_2"}},
	{id: "commit", jobType: "draft", dependsOn: []string{"adjudicate"}},
	{id: "final", jobType: "draft", dependsOn: []string{"commit"}},
}

// fgCycleTarget / fgGate name the two cycle endpoints; fgCycleMaxIterations bounds
// the declared revision cycle (>1 so a single needs_revision stays within budget).
const (
	fgCycleTarget        = "falsifier_1"
	fgGate               = "adjudicate"
	fgCycleMaxIterations = 2
)

func fgRole(nodeID string) string { return "role_" + nodeID }
func fgLane(nodeID string) string { return "lane_" + nodeID }

// fgNodeByID resolves a node so a driver can pick the right completion path.
func fgNodeByID(nodeID string) fgNode {
	for _, n := range falsificationGateGraph {
		if n.id == nodeID {
			return n
		}
	}
	panic("unknown falsification_gate node " + nodeID)
}

// fgLC tracks the rows the falsification_gate lifecycle drives.
type fgLC struct {
	repoID   string
	runID    string
	jobIDs   map[string]string // nodeID -> jobID
	sessions int               // monotonic; unique session id + ordinal per drive
}

func (lc *fgLC) jobID(nodeID string) string { return lc.jobIDs[nodeID] }

// newSession registers a fresh session for the node's role/lane. Verdict-capable
// gate nodes get the `review` capability (mirroring lifecycle_revision's reviewer);
// every other node gets `write`.
func (lc *fgLC) newSession(t *testing.T, ctx context.Context, h *Harness, nodeID string) string {
	t.Helper()
	lc.sessions++
	caps := []string{"claim", "write"}
	if fgNodeByID(nodeID).jobType == "review" {
		caps = []string{"claim", "review"}
	}
	sessionID := fmt.Sprintf("sess_%s_%s_%d", nodeID, lc.repoID, lc.sessions)
	seedFixtureSession(t, ctx, h.Runner, lc.repoID, lc.runID, sessionID,
		fgRole(nodeID), fgLane(nodeID), caps, lc.sessions)
	return sessionID
}

// seedFalsificationGateRun seeds a running falsification_gate run: the holder job
// queued (claimable) with a pending work message, every downstream node blocked
// with its job_dependencies, and the declared revision cycle in the workflow
// snapshot. Completing a node drives the real maybeEnqueueDownstream gating;
// recording needs_revision on the adjudicate gate drives the real
// routeRevisionCycle. Nodes are document_only jobs with no required artifact, so
// the lifecycle question is the cycle/cascade, not the artifact contract (which
// the publisher's own suites cover) — exactly the sibling fixtures' stance.
func seedFalsificationGateRun(t *testing.T, ctx context.Context, h *Harness, repoID string) *fgLC {
	t.Helper()
	repoRoot := t.TempDir()
	runID := "run_" + repoID
	now := time.Now().UTC()

	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'falsification-gate',$5,16,'active')`,
		repoID, fixtureRepoIdentity, repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("seed repository: %v", err)
	}

	// Workflow snapshot (roles + lanes + jobs + edges + the cycle). The cycle is
	// the load-bearing part workflowCyclesForJob reads; the rest is legibility.
	roles := map[string]any{}
	lanes := map[string]any{}
	jobs := []any{}
	edges := []any{}
	for _, n := range falsificationGateGraph {
		roles[fgRole(n.id)] = map[string]any{"summary": n.id}
		lanes[fgLane(n.id)] = map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}}
		jobs = append(jobs, map[string]any{"id": n.id})
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
		"cycles": []any{map[string]any{
			"from": fgGate, "to": fgCycleTarget,
			"on_verdict": "needs_revision", "max_iterations": fgCycleMaxIterations, "allow_same_lane": true,
		}},
	})

	writeScope, err := db.JSONBArg(h.Runner, map[string]any{"mode": "document_only", "allowed_paths": []any{"docs"}})
	if err != nil {
		t.Fatalf("encode write_scope: %v", err)
	}
	emptyArtifacts, err := db.JSONBArg(h.Runner, []any{})
	if err != nil {
		t.Fatalf("encode expected_artifacts: %v", err)
	}

	lc := &fgLC{repoID: repoID, runID: runID, jobIDs: map[string]string{}}
	for _, n := range falsificationGateGraph {
		jobID := "job_" + n.id + "_" + repoID
		lc.jobIDs[n.id] = jobID
		state := "blocked"
		if len(n.dependsOn) == 0 {
			state = "queued"
		}
		laneSel, err := db.JSONBArg(h.Runner, map[string]any{"lane_id": fgLane(n.id)})
		if err != nil {
			t.Fatalf("encode lane_selector %s: %v", n.id, err)
		}
		if err := h.Runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
			  lane_selector_json, created_at
			) VALUES ($1,$2,$3,$4,1,$5,$6,$7,$8,'idem_'||$2,$9::jsonb,$10::jsonb,$11::jsonb,$12)`,
			repoID, jobID, runID, n.id, state, fgRole(n.id),
			"Falsification "+n.id, n.jobType, writeScope, emptyArtifacts, laneSel, now); err != nil {
			t.Fatalf("seed job %s: %v", n.id, err)
		}
		if state == "queued" {
			if err := h.Runner.Exec(ctx, `
				INSERT INTO striatumd.queue_messages (
				  repository_id, message_id, run_id, job_id, kind, state, priority,
				  target_role_id, target_lane_id, created_at, updated_at
				) VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,$7)`,
				repoID, "msg_"+jobID, runID, jobID, fgRole(n.id), fgLane(n.id), now); err != nil {
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

// fgDriveComplete registers a fresh session for a non-verdict node, claims + acks
// its queued job, completes it through work.complete, and closes the session (a
// finished lane closes — mirroring driveImplementer; closing keeps the re-opened
// chain's recovery deterministic across the revision cycle).
func fgDriveComplete(t *testing.T, ctx context.Context, h *Harness, lc *fgLC, nodeID string) {
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

// fgDriveVerdict registers a fresh session for the verdict-capable gate node,
// claims + acks the enqueued gate job, and records the verdict through the
// production review.verdict handler (needs_revision routes the cycle; a clearing
// verdict completes the gate and enqueues the commit tail).
func fgDriveVerdict(t *testing.T, ctx context.Context, h *Harness, lc *fgLC, nodeID, verdict string) map[string]any {
	t.Helper()
	sessionID := lc.newSession(t, ctx, h, nodeID)
	leaseID := claimAndAck(t, ctx, h, lc.repoID, sessionID, nodeID)
	result, err := mutations.HandleRecordVerdict(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion, Method: "review.verdict",
		Params: map[string]any{
			"repository_id": lc.repoID, "session_id": sessionID,
			"job_id": lc.jobID(nodeID), "lease_id": leaseID, "verdict": verdict,
		},
	})
	if err != nil {
		t.Fatalf("verdict %q on %s: %v", verdict, nodeID, err)
	}
	return result
}

// fgClaimLeaveLive registers a fresh session, claims + acks the node's job, and
// returns the session id WITHOUT completing — a live claimant a fault injector can
// kill.
func fgClaimLeaveLive(t *testing.T, ctx context.Context, h *Harness, lc *fgLC, nodeID string) string {
	t.Helper()
	sessionID := lc.newSession(t, ctx, h, nodeID)
	claimAndAck(t, ctx, h, lc.repoID, sessionID, nodeID)
	return sessionID
}

// TestFalsificationGateHappyPathReCascades drives the full falsification_gate graph
// through ONE needs_revision cycle to `completed` with NO fault and NO operator.
// The load-bearing assertion is the multi-job re-cascade: a needs_revision verdict
// on the adjudicate gate must transitively re-block falsifier_2 (depth 1) AND the
// adjudicate gate itself (depth 2) and re-open falsifier_1, then the whole chain
// re-runs to a clearing verdict and the commit tail completes the run — coverage
// the single-job lifecycle_revision reopen (depth 1) does not reach.
func TestFalsificationGateHappyPathReCascades(t *testing.T) {
	repoID := "repo_falsgate_happy"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedFalsificationGateRun(t, ctx, h, repoID)

	// Only holder is claimable; everything downstream is blocked.
	if got := chaosJobState(t, ctx, h, lc.jobID("holder")); got != "queued" {
		t.Fatalf("holder state = %q, want queued", got)
	}
	for _, n := range []string{"falsifier_1", "falsifier_2", "adjudicate", "commit", "final"} {
		if got := chaosJobState(t, ctx, h, lc.jobID(n)); got != "blocked" {
			t.Fatalf("%s state = %q, want blocked (pre-dialogue)", n, got)
		}
	}

	// Drive the linear dialogue chain to the gate.
	fgDriveComplete(t, ctx, h, lc, "holder")
	fgDriveComplete(t, ctx, h, lc, "falsifier_1")
	fgDriveComplete(t, ctx, h, lc, "falsifier_2")
	if got := chaosJobState(t, ctx, h, lc.jobID("adjudicate")); got != "queued" {
		t.Fatalf("adjudicate state = %q, want queued (chain reached the gate)", got)
	}
	priorTargetAttempt := chaosJobAttempt(t, ctx, h, lc.jobID("falsifier_1"))
	priorGateAttempt := chaosJobAttempt(t, ctx, h, lc.jobID("adjudicate"))

	// needs_revision: the gate re-opens the WHOLE dialogue chain.
	routed := fgDriveVerdict(t, ctx, h, lc, "adjudicate", "needs_revision")
	if got := fmt.Sprint(routed["status"]); got != "revision_routed" {
		t.Fatalf("needs_revision status = %q, want revision_routed (the cycle did not fire)", got)
	}
	// The cycle target re-opens for a fresh attempt...
	if got := chaosJobState(t, ctx, h, lc.jobID("falsifier_1")); got != "queued" {
		t.Fatalf("falsifier_1 state = %q, want queued (cycle target re-opened)", got)
	}
	if got := chaosJobAttempt(t, ctx, h, lc.jobID("falsifier_1")); got != priorTargetAttempt+1 {
		t.Fatalf("falsifier_1 attempt = %d, want %d (revision bumps the target attempt)", got, priorTargetAttempt+1)
	}
	// ...and EVERY downstream terminal job is transitively re-blocked: falsifier_2
	// at depth 1 AND the adjudicate gate at depth 2 (the recursive-CTE coverage).
	for _, n := range []string{"falsifier_2", "adjudicate"} {
		if got := chaosJobState(t, ctx, h, lc.jobID(n)); got != "blocked" {
			t.Fatalf("%s state = %q after needs_revision, want blocked (transitive downstream re-block)", n, got)
		}
	}
	if got := chaosJobAttempt(t, ctx, h, lc.jobID("adjudicate")); got != priorGateAttempt+1 {
		t.Fatalf("adjudicate (depth-2) attempt = %d, want %d (transitive reopen must bump the gate too)", got, priorGateAttempt+1)
	}
	if got := chaosRunState(t, ctx, h, lc.runID); got != "running" {
		t.Fatalf("run state = %q, want running (mid-revision)", got)
	}

	// The revision re-cascades the whole chain back to the gate; this time it clears.
	fgDriveComplete(t, ctx, h, lc, "falsifier_1")
	if got := chaosJobState(t, ctx, h, lc.jobID("falsifier_2")); got != "queued" {
		t.Fatalf("falsifier_2 state = %q after re-completing falsifier_1, want queued (re-cascade)", got)
	}
	fgDriveComplete(t, ctx, h, lc, "falsifier_2")
	if got := chaosJobState(t, ctx, h, lc.jobID("adjudicate")); got != "queued" {
		t.Fatalf("adjudicate state = %q after re-completing the chain, want queued (gate re-enqueued)", got)
	}
	cleared := fgDriveVerdict(t, ctx, h, lc, "adjudicate", "accept")
	if got := fmt.Sprint(cleared["status"]); got != "completed" {
		t.Fatalf("clearing verdict status = %q, want completed", got)
	}

	// The commit tail finishes the run unattended.
	fgDriveComplete(t, ctx, h, lc, "commit")
	fgDriveComplete(t, ctx, h, lc, "final")
	if got := chaosRunState(t, ctx, h, lc.runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (falsification_gate drove to completion unattended)", got)
	}
}

// TestFalsificationGateMidCascadeDeathSelfRecovers is the RFC 0105 fault cell for
// this shape: after a needs_revision re-opens the dialogue chain, a MIDDLE node of
// the re-cascade (falsifier_2) has its lane die hard mid-flight (the #121 dead-pane
// shape) DURING the revision cycle. The production recovery sweep must requeue that
// node on the same attempt with no operator and no escalation, the adjudicate gate
// must stay blocked (the re-cascade must not be lost), and once a fresh lane
// completes falsifier_2 the gate re-enqueues, a clearing verdict lands, and the run
// reaches `completed` unattended.
func TestFalsificationGateMidCascadeDeathSelfRecovers(t *testing.T) {
	repoID := "repo_falsgate_death"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedFalsificationGateRun(t, ctx, h, repoID)

	// Drive the chain to the gate and route one revision back through it.
	fgDriveComplete(t, ctx, h, lc, "holder")
	fgDriveComplete(t, ctx, h, lc, "falsifier_1")
	fgDriveComplete(t, ctx, h, lc, "falsifier_2")
	fgDriveVerdict(t, ctx, h, lc, "adjudicate", "needs_revision")
	if got := chaosJobState(t, ctx, h, lc.jobID("falsifier_1")); got != "queued" {
		t.Fatalf("falsifier_1 state = %q, want queued (cycle re-opened it)", got)
	}

	// Re-cascade one hop: falsifier_1 re-completes, enqueueing falsifier_2 (att2).
	fgDriveComplete(t, ctx, h, lc, "falsifier_1")
	if got := chaosJobState(t, ctx, h, lc.jobID("falsifier_2")); got != "queued" {
		t.Fatalf("falsifier_2 state = %q, want queued (mid-cascade)", got)
	}
	priorAttempt := chaosJobAttempt(t, ctx, h, lc.jobID("falsifier_2"))

	// falsifier_2's lane claims + acks, then dies hard mid-revision-cascade.
	deadSession := fgClaimLeaveLive(t, ctx, h, lc, "falsifier_2")
	injectDeadLane(t, ctx, h.Runner, repoID, deadSession, lc.jobID("falsifier_2"))
	if got := chaosJobState(t, ctx, h, lc.jobID("falsifier_2")); got != "running" {
		t.Fatalf("pre-sweep falsifier_2 state = %q, want running (limbo after lane death)", got)
	}

	// The production recovery sweep requeues the dead mid-chain node on the same attempt.
	summary := runSweep(t, ctx, h, lc.runID)
	if summary.escalationCount != 0 {
		t.Fatalf("mid-cascade death raised %d escalations, want 0 (self-recover); result=%#v", summary.escalationCount, summary.result)
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("falsifier_2")); got != "queued" {
		t.Fatalf("post-sweep falsifier_2 state = %q, want queued (requeued); acted_count=%d", got, summary.actedCount)
	}
	if got := chaosJobAttempt(t, ctx, h, lc.jobID("falsifier_2")); got != priorAttempt {
		t.Fatalf("falsifier_2 attempt = %d, want %d (autonomous requeue must not bump attempt)", got, priorAttempt)
	}
	// The gate must NOT have been enqueued while the recovered node was down.
	if got := chaosJobState(t, ctx, h, lc.jobID("adjudicate")); got != "blocked" {
		t.Fatalf("adjudicate state = %q during falsifier_2 recovery, want blocked (the re-cascade must not be lost)", got)
	}
	if got := chaosRunState(t, ctx, h, lc.runID); got != "running" {
		t.Fatalf("run state = %q, want running (no operator needed)", got)
	}

	// A fresh lane completes the recovered node; the gate then clears and the run
	// drives to completion unattended.
	fgDriveComplete(t, ctx, h, lc, "falsifier_2")
	if got := chaosJobState(t, ctx, h, lc.jobID("adjudicate")); got != "queued" {
		t.Fatalf("adjudicate state = %q after recovered falsifier_2 completed, want queued (gate re-enqueued post-recovery)", got)
	}
	cleared := fgDriveVerdict(t, ctx, h, lc, "adjudicate", "accept")
	if got := fmt.Sprint(cleared["status"]); got != "completed" {
		t.Fatalf("clearing verdict status = %q, want completed", got)
	}
	fgDriveComplete(t, ctx, h, lc, "commit")
	fgDriveComplete(t, ctx, h, lc, "final")
	if got := chaosRunState(t, ctx, h, lc.runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (self-recovered from a mid-cascade death, no operator)", got)
	}
	if escs := chaosRecoveryExhaustedEscalations(t, ctx, h, lc.runID); len(escs) != 0 {
		t.Fatalf("a self-recovering falsification_gate raised %d recovery_exhausted escalations, want 0", len(escs))
	}
}
