package adapterconformance

// RFC 0105 reliability fixture for RFC 0106's `implementation_panel` shape — the
// graduation evidence that lets it carry support_tier=`supported` (the shape-tier
// guard, TestSupportedShapesHaveReliabilityFixture, refuses the tier without a
// fixture here). It is deliberately NOT a re-run of the existing fixtures with
// renamed jobs: chaos_test.go proves a single-job repo-write lane, and
// lifecycle_revision_test.go proves a two-lane review→needs_revision cycle, but
// NEITHER exercises implementation_panel's distinctive lifecycle — a 3-way
// FAN-OUT of parallel proposals from a framing job and a 3-predecessor JOIN at the
// scorecards before a linear arbitration→dissent→decision tail (catalog.json
// graph_preview; no cycles). This file proves that structure drives unattended,
// and — the load-bearing new coverage — that a fault injected into ONE parallel
// branch self-recovers without the join either firing early or losing the
// recovered branch.
//
// Both lanes/cells are driven through the PRODUCTION mutation handlers
// (work.claim_next / work.ack / work.complete / session.close) and the real
// recovery sweep (mutations.SweepRun) against the in-process daemon + isolated
// pgtest database, exactly like the sibling fixtures — the real fan-out/join
// readiness logic (maybeEnqueueDownstream → dependenciesSatisfied), not a
// shortcut. Each node is a distinct (role, lane) so claim_next routes each parallel
// proposal to its own driving session deterministically even while all three are
// queued at once.
//
// The RFC 0105 gate (recover-or-escalate, never a silent wedge) for this shape:
// the happy cell drives fan-out → join → completion with no operator; the
// parallel-branch-death cell injects a hard dead lane into one proposal and the
// production sweep requeues it on the same attempt while the join correctly stays
// blocked, a fresh lane finishes it, and the panel reaches `completed` unattended.
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

// panelNode is one node of the implementation_panel graph and the upstream nodes
// it joins on (its job_dependencies).
type panelNode struct {
	id        string
	dependsOn []string
}

// implementationPanelGraph is RFC 0106's implementation_panel shape graph
// (catalog.json graph_preview): frame fans out to three parallel proposals, the
// scorecards join waits on all three, then a linear arbitration→dissent→decision
// tail. No cycles.
var implementationPanelGraph = []panelNode{
	{id: "frame"},
	{id: "proposal_a", dependsOn: []string{"frame"}},
	{id: "proposal_b", dependsOn: []string{"frame"}},
	{id: "proposal_c", dependsOn: []string{"frame"}},
	{id: "scorecards", dependsOn: []string{"proposal_a", "proposal_b", "proposal_c"}},
	{id: "arbitration", dependsOn: []string{"scorecards"}},
	{id: "dissent", dependsOn: []string{"arbitration"}},
	{id: "decision", dependsOn: []string{"dissent"}},
}

func panelRole(nodeID string) string { return "role_" + nodeID }
func panelLane(nodeID string) string { return "lane_" + nodeID }

// panelLC tracks the rows the implementation_panel lifecycle drives.
type panelLC struct {
	repoID   string
	runID    string
	jobIDs   map[string]string // nodeID -> jobID
	sessions int               // monotonic; unique session id + ordinal per drive
}

func (lc *panelLC) jobID(nodeID string) string { return lc.jobIDs[nodeID] }

func (lc *panelLC) newSession(t *testing.T, ctx context.Context, h *Harness, nodeID string) string {
	t.Helper()
	lc.sessions++
	sessionID := fmt.Sprintf("sess_%s_%s_%d", nodeID, lc.repoID, lc.sessions)
	seedFixtureSession(t, ctx, h.Runner, lc.repoID, lc.runID, sessionID,
		panelRole(nodeID), panelLane(nodeID), []string{"claim", "write"}, lc.sessions)
	return sessionID
}

// seedImplementationPanelRun seeds a running implementation_panel run: the frame
// job queued (claimable) with a pending work message, every downstream node
// blocked with its job_dependencies, so completing a node drives the real
// maybeEnqueueDownstream readiness logic (it enqueues a blocked job only once ALL
// its dependencies are completed). Nodes are document_only draft jobs with no
// required artifact, so work.complete finalizes them without git or publish — the
// reliability question here is the DAG lifecycle, not the artifact contract (which
// the publisher's own suites cover).
func seedImplementationPanelRun(t *testing.T, ctx context.Context, h *Harness, repoID string) *panelLC {
	t.Helper()
	repoRoot := t.TempDir()
	runID := "run_" + repoID
	now := time.Now().UTC()

	if err := h.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'implementation-panel',$5,16,'active')`,
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
	for _, n := range implementationPanelGraph {
		roles[panelRole(n.id)] = map[string]any{"summary": n.id}
		lanes[panelLane(n.id)] = map[string]any{"display_model": "Claude", "capabilities": []any{"claim", "write"}}
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

	lc := &panelLC{repoID: repoID, runID: runID, jobIDs: map[string]string{}}
	for _, n := range implementationPanelGraph {
		jobID := "job_" + n.id + "_" + repoID
		lc.jobIDs[n.id] = jobID
		state := "blocked"
		if len(n.dependsOn) == 0 {
			state = "queued"
		}
		laneSel, err := db.JSONBArg(h.Runner, map[string]any{"lane_id": panelLane(n.id)})
		if err != nil {
			t.Fatalf("encode lane_selector %s: %v", n.id, err)
		}
		if err := h.Runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
			  lane_selector_json, created_at
			) VALUES ($1,$2,$3,$4,1,$5,$6,$7,'draft','idem_'||$2,$8::jsonb,$9::jsonb,$10::jsonb,$11)`,
			repoID, jobID, runID, n.id, state, panelRole(n.id),
			"Panel "+n.id, writeScope, emptyArtifacts, laneSel, now); err != nil {
			t.Fatalf("seed job %s: %v", n.id, err)
		}
		if state == "queued" {
			if err := h.Runner.Exec(ctx, `
				INSERT INTO striatumd.queue_messages (
				  repository_id, message_id, run_id, job_id, kind, state, priority,
				  target_role_id, target_lane_id, created_at, updated_at
				) VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,$7)`,
				repoID, "msg_"+jobID, runID, jobID, panelRole(n.id), panelLane(n.id), now); err != nil {
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

// panelDriveComplete registers a fresh session for the node's role/lane, claims +
// acks the node's queued job, completes it through work.complete, and closes the
// session (a finished lane closes, mirroring driveImplementer).
func panelDriveComplete(t *testing.T, ctx context.Context, h *Harness, lc *panelLC, nodeID string) {
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

// panelClaimLeaveLive registers a fresh session, claims + acks the node's job, and
// returns the session id WITHOUT completing — a live claimant a fault injector can
// kill.
func panelClaimLeaveLive(t *testing.T, ctx context.Context, h *Harness, lc *panelLC, nodeID string) string {
	t.Helper()
	sessionID := lc.newSession(t, ctx, h, nodeID)
	claimAndAck(t, ctx, h, lc.repoID, sessionID, nodeID)
	return sessionID
}

// TestImplementationPanelHappyPathCompletes drives the full implementation_panel
// graph to `completed` with NO fault and NO operator: the fan-out enqueues all
// three parallel proposals at once, the scorecards join holds until the last
// proposal completes, and the arbitration→dissent→decision tail finishes the run.
// This is the baseline the parallel-branch-death cell perturbs.
func TestImplementationPanelHappyPathCompletes(t *testing.T) {
	repoID := "repo_implpanel_happy"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedImplementationPanelRun(t, ctx, h, repoID)

	// Only frame is claimable; everything downstream is blocked.
	if got := chaosJobState(t, ctx, h, lc.jobID("frame")); got != "queued" {
		t.Fatalf("frame state = %q, want queued", got)
	}
	for _, n := range []string{"proposal_a", "proposal_b", "proposal_c", "scorecards", "decision"} {
		if got := chaosJobState(t, ctx, h, lc.jobID(n)); got != "blocked" {
			t.Fatalf("%s state = %q, want blocked (pre-fan-out)", n, got)
		}
	}

	// Fan-out: completing frame must enqueue ALL THREE proposals at once.
	panelDriveComplete(t, ctx, h, lc, "frame")
	for _, n := range []string{"proposal_a", "proposal_b", "proposal_c"} {
		if got := chaosJobState(t, ctx, h, lc.jobID(n)); got != "queued" {
			t.Fatalf("post-frame %s state = %q, want queued (fan-out must enqueue every parallel proposal)", n, got)
		}
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("scorecards")); got != "blocked" {
		t.Fatalf("scorecards state = %q, want blocked (join must wait for all proposals)", got)
	}

	// Multi-predecessor join: scorecards stays blocked until the LAST proposal completes.
	panelDriveComplete(t, ctx, h, lc, "proposal_a")
	if got := chaosJobState(t, ctx, h, lc.jobID("scorecards")); got != "blocked" {
		t.Fatalf("scorecards state = %q after 1/3 proposals, want blocked", got)
	}
	panelDriveComplete(t, ctx, h, lc, "proposal_b")
	if got := chaosJobState(t, ctx, h, lc.jobID("scorecards")); got != "blocked" {
		t.Fatalf("scorecards state = %q after 2/3 proposals, want blocked", got)
	}
	panelDriveComplete(t, ctx, h, lc, "proposal_c")
	if got := chaosJobState(t, ctx, h, lc.jobID("scorecards")); got != "queued" {
		t.Fatalf("scorecards state = %q after 3/3 proposals, want queued (join must fire once all predecessors complete)", got)
	}

	// Linear tail to completion.
	panelDriveComplete(t, ctx, h, lc, "scorecards")
	panelDriveComplete(t, ctx, h, lc, "arbitration")
	panelDriveComplete(t, ctx, h, lc, "dissent")
	panelDriveComplete(t, ctx, h, lc, "decision")

	if got := chaosRunState(t, ctx, h, lc.runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (the panel drove to completion unattended)", got)
	}
}

// TestImplementationPanelParallelBranchDeathSelfRecovers is the RFC 0105 fault cell
// for the fan-out/join shape: two of three parallel proposals finish, the third's
// lane dies hard mid-flight (the #121 dead-pane shape), and the production recovery
// sweep requeues that branch on the same attempt with no operator and no
// escalation. The load-bearing assertions: the scorecards join must NOT fire while
// the recovered branch is down (it must not lose the branch), and once a fresh lane
// completes the recovered branch the join fires and the panel reaches `completed`
// unattended.
func TestImplementationPanelParallelBranchDeathSelfRecovers(t *testing.T) {
	repoID := "repo_implpanel_death"
	ctx := context.Background()
	h := NewHarness(t, repoID)
	lc := seedImplementationPanelRun(t, ctx, h, repoID)

	panelDriveComplete(t, ctx, h, lc, "frame")
	// Two of the three parallel proposals finish; the third is mid-flight.
	panelDriveComplete(t, ctx, h, lc, "proposal_a")
	panelDriveComplete(t, ctx, h, lc, "proposal_c")
	if got := chaosJobState(t, ctx, h, lc.jobID("scorecards")); got != "blocked" {
		t.Fatalf("scorecards state = %q, want blocked (proposal_b still in flight)", got)
	}

	// proposal_b's lane claims + acks, then dies hard (pane gone, session closed,
	// lease released) — a dead lane injected into ONE parallel branch.
	priorAttempt := chaosJobAttempt(t, ctx, h, lc.jobID("proposal_b"))
	deadSession := panelClaimLeaveLive(t, ctx, h, lc, "proposal_b")
	injectDeadLane(t, ctx, h.Runner, repoID, deadSession, lc.jobID("proposal_b"))
	if got := chaosJobState(t, ctx, h, lc.jobID("proposal_b")); got != "running" {
		t.Fatalf("pre-sweep proposal_b state = %q, want running (limbo after lane death)", got)
	}

	// The production recovery sweep requeues the dead branch on the same attempt.
	summary := runSweep(t, ctx, h, lc.runID)
	if summary.escalationCount != 0 {
		t.Fatalf("parallel-branch death raised %d escalations, want 0 (self-recover); result=%#v", summary.escalationCount, summary.result)
	}
	if got := chaosJobState(t, ctx, h, lc.jobID("proposal_b")); got != "queued" {
		t.Fatalf("post-sweep proposal_b state = %q, want queued (requeued); acted_count=%d", got, summary.actedCount)
	}
	if got := chaosJobAttempt(t, ctx, h, lc.jobID("proposal_b")); got != priorAttempt {
		t.Fatalf("proposal_b attempt = %d, want %d (autonomous requeue must not bump attempt)", got, priorAttempt)
	}
	// The join must NOT have fired while the recovered branch was down.
	if got := chaosJobState(t, ctx, h, lc.jobID("scorecards")); got != "blocked" {
		t.Fatalf("scorecards state = %q during proposal_b recovery, want blocked (the join must not lose the recovered branch)", got)
	}
	if got := chaosRunState(t, ctx, h, lc.runID); got != "running" {
		t.Fatalf("run state = %q, want running (no operator needed)", got)
	}

	// A fresh lane completes the recovered branch; the join then fires and the
	// panel drives to completion unattended.
	panelDriveComplete(t, ctx, h, lc, "proposal_b")
	if got := chaosJobState(t, ctx, h, lc.jobID("scorecards")); got != "queued" {
		t.Fatalf("scorecards state = %q after recovered proposal_b completed, want queued (join must fire post-recovery)", got)
	}
	panelDriveComplete(t, ctx, h, lc, "scorecards")
	panelDriveComplete(t, ctx, h, lc, "arbitration")
	panelDriveComplete(t, ctx, h, lc, "dissent")
	panelDriveComplete(t, ctx, h, lc, "decision")

	if got := chaosRunState(t, ctx, h, lc.runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (self-recovered from a parallel-branch death, no operator)", got)
	}
	if escs := chaosRecoveryExhaustedEscalations(t, ctx, h, lc.runID); len(escs) != 0 {
		t.Fatalf("a self-recovering panel raised %d recovery_exhausted escalations, want 0", len(escs))
	}
}
