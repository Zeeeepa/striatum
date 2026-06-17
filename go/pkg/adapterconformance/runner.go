package adapterconformance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/adapterconformance/testagent"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

// LiveScope selects which clauses the live fake-agent runner evaluates. C7
// (interrogation round trip) is only meaningful when a scripted interrogation
// is run, so it is opt-in.
type LiveScope struct {
	// IncludeInterrogation runs C7: the harness opens+asks one interrogation and
	// the clause asserts the agent answered it.
	IncludeInterrogation bool
}

// LiveReport is the outcome of one live fake-agent conformance run: the agent's
// terminal observation and the per-clause results for the daemon-lifecycle
// clauses C3..C10 (+ C7 when in scope). Clauses outside this slice's scope
// (C1, C11, C12) remain Deferred and are not present here.
type LiveReport struct {
	Adapter     string
	Mode        testagent.Mode
	AgentResult testagent.Result
	AgentErr    error
	Clauses     []ClauseResult
}

// Clause returns the result for a clause id, or false when not evaluated.
func (r LiveReport) Clause(id ClauseID) (ClauseResult, bool) {
	for _, c := range r.Clauses {
		if c.Clause == id {
			return c, true
		}
	}
	return ClauseResult{}, false
}

// FirstContractFailure returns the first contract-fail clause result, or false
// when every evaluated clause passed.
func (r LiveReport) FirstContractFailure() (ClauseResult, bool) {
	for _, c := range r.Clauses {
		if c.Status == StatusContractFail {
			return c, true
		}
	}
	return ClauseResult{}, false
}

// RunLive drives one fake-agent lifecycle against the in-process daemon harness
// and evaluates the daemon-lifecycle clauses C3..C10 (+ C7 when scoped) from the
// DaemonObserver (DESIGN §1.3 runner.go steps 5-7).
//
// Determinism note: the in-process testagent commits each protocol write before
// returning, so the runner evaluates each clause against FINAL daemon state once
// the agent goroutine settles. A clause whose protocol event never fired records
// its contract FailureClass + structured fields; the broken modes are
// engineered so EXACTLY one in-scope clause fails with its declared class,
// arming the taxonomy (Tier-A self-test). Deadlines are reported (DefaultPolicy
// x DEADLINE_SCALE) but the in-process path does not idle on them.
func RunLive(t *testing.T, ctx context.Context, h *Harness, fx *Fixture, adapter string, mode testagent.Mode, scope LiveScope) LiveReport {
	t.Helper()

	report := LiveReport{Adapter: adapter, Mode: mode}

	gate := make(chan struct{})
	seeded := make(chan struct{})
	cfg := testagent.Config{
		Endpoint:     h.MCPEndpoint(),
		Token:        h.Token,
		RepositoryID: fx.RepositoryID,
		RunID:        fx.RunID,
		Role:         fx.Role,
		Lane:         fx.Lane,
		Mode:         mode,
		Artifact: testagent.ArtifactSpec{
			Kind:        fx.ArtifactKind,
			LogicalName: fx.ArtifactLogicalName,
			RepoPath:    fx.ArtifactRepoPath,
		},
		InterrogationWait: 5 * time.Second,
		// dbf2013b: attest the registered session (a real lane is attested by
		// supervise start) so work.await_packet/claim is admitted. Runs on the
		// agent goroutine, so use the error-returning attest variant.
		AfterRegister: func(sessionID string) error {
			return attestFixtureSessionErr(ctx, h.Runner, fx.RepositoryID, fx.RunID, sessionID, fx.Lane)
		},
	}
	if scope.IncludeInterrogation {
		cfg.InterrogationGate = gate
		cfg.InterrogationSeeded = seeded
	}

	agent := testagent.New(cfg)

	done := make(chan struct{})
	var agentRes testagent.Result
	var agentErr error
	go func() {
		defer close(done)
		agentRes, agentErr = agent.Run(ctx)
	}()

	var interrogationID string
	if scope.IncludeInterrogation {
		// Wait for the agent to register, ack, and heartbeat and reach the gate.
		select {
		case <-gate:
		case <-done:
			// Agent exited before reaching the interrogation gate (e.g. an early
			// failure). Fall through; C7 will record the missing answer.
		case <-ctx.Done():
		}
		sessionID := agent.SessionID()
		if sessionID != "" {
			// Seed the interrogator + attestation while the agent is paused at the
			// gate (not yet polling), so the raw fixture INSERTs do not race the
			// claim path — a pure test-seeding artifact, not the product deadlock.
			fx.SeedInterrogator(t, ctx, h.Runner, sessionID)
			// Release the agent to begin answer-polling. RFC 0101 Phase 0a (#137):
			// the open+ask below now runs CONCURRENTLY with the agent's
			// work.await_packet — the exact #103/#137 race — with NO "ask committed"
			// handshake. It completes deadlock-free because of the daemon-side root
			// fix, regression-proving it end to end.
			close(seeded)
			interrogationID = openAndAsk(t, ctx, h, fx, sessionID)
		} else {
			close(seeded)
		}
	}

	<-done
	report.AgentResult = agentRes
	report.AgentErr = agentErr

	sessionID := agentRes.SessionID
	if sessionID == "" {
		sessionID = agent.SessionID()
	}

	obs := h.Observer()
	now := time.Now().UTC()
	snap, err := obs.Session(ctx, fx.RepositoryID, sessionID, now)
	if err != nil {
		t.Fatalf("observer.Session: %v", err)
	}

	policy := sessionliveness.DefaultPolicy()
	scale := DeadlineScale()

	// C3 — BootstrapDelivered: last_tools_list_at non-null.
	report.Clauses = append(report.Clauses, evalC3(adapter, snap, scaledFrom(policy.DiscoverySeconds, scale)))

	// C4 — AwaitPacketForeground: last_await_packet_at advanced AND the foreground
	// claim created the lease for the job. The lease the agent obtained
	// (agentRes.LeaseID) is the proof the await bound a lease; it may already be
	// released by a later work.complete, so C4 does not require it to still be
	// active — it requires that the await advanced and bound a lease at all.
	report.Clauses = append(report.Clauses, evalC4(adapter, snap, agentRes, scaledFrom(policy.AwaitPacketSeconds, scale)))

	// C5 — PacketAcked: job acked/running AND last_ack_at set.
	jobState, err := obs.JobState(ctx, fx.RepositoryID, fx.JobID)
	if err != nil {
		t.Fatalf("observer.JobState: %v", err)
	}
	report.Clauses = append(report.Clauses, evalC5(adapter, snap, jobState, scaledFrom(policy.AckSeconds, scale)))

	// C6 — HeartbeatObserved: last_work_heartbeat_at advanced.
	report.Clauses = append(report.Clauses, evalC6(adapter, snap, scaledFrom(policy.LeaseHeartbeatSeconds, scale)))

	// C7 — InterrogationRoundTrip (when scoped): the agent answered the question.
	if scope.IncludeInterrogation {
		answered := false
		if interrogationID != "" {
			answered, err = obs.InterrogationAnswered(ctx, fx.RepositoryID, interrogationID)
			if err != nil {
				t.Fatalf("observer.InterrogationAnswered: %v", err)
			}
		}
		report.Clauses = append(report.Clauses, evalC7(adapter, answered, snap, scaledFrom(policy.ProtocolIdleSeconds, scale)))
	}

	// C8 — ArtifactPublished: the expected artifact row exists.
	artifact, err := obs.Artifact(ctx, fx.RepositoryID, fx.JobID, fx.ArtifactLogicalName)
	if err != nil {
		t.Fatalf("observer.Artifact: %v", err)
	}
	report.Clauses = append(report.Clauses, evalC8(adapter, artifact, fx))

	// C9 — WorkCompleted: job completed AND last_work_complete_at set.
	report.Clauses = append(report.Clauses, evalC9(adapter, snap, jobState, scaledFrom(policy.ProtocolIdleSeconds, scale)))

	// C10 — ReceiveLoopContinues: agent re-awaited to no_work AND no idle stall.
	report.Clauses = append(report.Clauses, evalC10(adapter, snap, agentRes, scaledFrom(policy.ProtocolIdleSeconds, scale)))

	return report
}

// openAndAsk opens an interrogation from the interrogator against the target
// (agent) session and asks one question, returning the interrogation id.
func openAndAsk(t *testing.T, ctx context.Context, h *Harness, fx *Fixture, targetSessionID string) string {
	t.Helper()
	openRes, err := mutations.HandleInterrogationOpen(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.open",
		Params: map[string]any{
			"repository_id":     fx.RepositoryID,
			"session_id":        fx.InterrogatorSessionID,
			"target_session_id": targetSessionID,
			"topic":             "conformance interrogation",
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
			"session_id":       fx.InterrogatorSessionID,
			"interrogation_id": interrogationID,
			"body":             "why does the fixture publish this note?",
		},
	}); err != nil {
		t.Fatalf("interrogation.ask: %v", err)
	}
	return interrogationID
}

func scaledFrom(seconds int, scale float64) time.Duration {
	base := time.Duration(seconds) * time.Second
	return time.Duration(float64(base) * scale)
}

// --- per-clause live evaluators -------------------------------------------

func evalC3(adapter string, snap SessionSnapshot, deadline time.Duration) ClauseResult {
	c := Clause{ID: C3}
	if snap.LastToolsListAt != nil {
		return c.pass(adapter, "last_tools_list_at stamped: bootstrap discovery delivered")
	}
	return c.contractFail(adapter, BootstrapStall,
		"no last_tools_list_at by deadline: the lane never completed bootstrap discovery (the #101 shape)",
		liveFields(deadline, "bootstrap_never_delivered", snap, ""))
}

func evalC4(adapter string, snap SessionSnapshot, agentRes testagent.Result, deadline time.Duration) ClauseResult {
	c := Clause{ID: C4}
	if snap.LastAwaitPacketAt == nil {
		return c.contractFail(adapter, AwaitPacketStall,
			"last_await_packet_at never advanced: the lane discovered tools/list but never entered the foreground receive loop",
			liveFields(deadline, "await_packet_never", snap, ""))
	}
	if agentRes.LeaseID == "" {
		return c.contractFail(adapter, AwaitPacketStall,
			"await_packet advanced but no lease was created for the expected job: the foreground claim did not bind the lease",
			liveFields(deadline, "no_lease_bound", snap, agentRes.JobID))
	}
	return c.pass(adapter, "last_await_packet_at advanced and the foreground claim bound the lease for the expected job")
}

func evalC5(adapter string, snap SessionSnapshot, jobState string, deadline time.Duration) ClauseResult {
	c := Clause{ID: C5}
	acked := jobState == "running" || jobState == "claimed" || jobState == "completed"
	if snap.LastAckAt != nil && acked {
		return c.pass(adapter, "packet acked: job transitioned out of claimed and last_ack_at is set")
	}
	return c.contractFail(adapter, AckStall,
		fmt.Sprintf("packet was delivered but never acked (last_ack_at=%v, job_state=%q)", snap.LastAckAt, jobState),
		liveFields(deadline, "ack_never", snap, ""))
}

func evalC6(adapter string, snap SessionSnapshot, deadline time.Duration) ClauseResult {
	c := Clause{ID: C6}
	if snap.LastWorkHeartbeatAt != nil {
		return c.pass(adapter, "last_work_heartbeat_at advanced at least once while the job was open")
	}
	return c.contractFail(adapter, HeartbeatMissed,
		"the lease was held but no work.heartbeat was observed",
		liveFields(deadline, "heartbeat_never", snap, ""))
}

func evalC7(adapter string, answered bool, snap SessionSnapshot, deadline time.Duration) ClauseResult {
	c := Clause{ID: C7}
	if answered {
		return c.pass(adapter, "interrogation question delivered and answered before the deadline")
	}
	return c.contractFail(adapter, InterrogationIgnored,
		"interrogation question stayed pending: the lane never answered the delivered question",
		liveFields(deadline, "interrogation_ignored", snap, ""))
}

func evalC8(adapter string, artifact map[string]any, fx *Fixture) ClauseResult {
	c := Clause{ID: C8}
	if artifact == nil {
		return c.contractFail(adapter, ArtifactMissing,
			fmt.Sprintf("expected artifact row missing: logical_name=%q kind=%q", fx.ArtifactLogicalName, fx.ArtifactKind),
			map[string]string{"logical_name": fx.ArtifactLogicalName, "kind": fx.ArtifactKind, "path": fx.ArtifactRepoPath})
	}
	gotKind := fmt.Sprint(artifact["artifact_kind"])
	gotPath := fmt.Sprint(artifact["repo_path"])
	if gotKind != fx.ArtifactKind || gotPath != fx.ArtifactRepoPath {
		return c.contractFail(adapter, ArtifactRejected,
			fmt.Sprintf("published artifact does not match expected kind/path (got kind=%q path=%q)", gotKind, gotPath),
			map[string]string{"reason": "mismatch", "kind": gotKind, "path": gotPath})
	}
	return c.pass(adapter, "expected artifact row present with valid front matter (publisher accepted)")
}

func evalC9(adapter string, snap SessionSnapshot, jobState string, deadline time.Duration) ClauseResult {
	c := Clause{ID: C9}
	if jobState == "completed" && snap.LastWorkCompleteAt != nil {
		return c.pass(adapter, "job completed and last_work_complete_at is set")
	}
	return c.contractFail(adapter, CompleteMissing,
		fmt.Sprintf("job never completed (job_state=%q, last_work_complete_at=%v)", jobState, snap.LastWorkCompleteAt),
		liveFields(deadline, "complete_missing", snap, ""))
}

func evalC10(adapter string, snap SessionSnapshot, agentRes testagent.Result, deadline time.Duration) ClauseResult {
	c := Clause{ID: C10}
	stall := snap.Classification.StallClass
	if stall == sessionliveness.StallProtocolIdle ||
		stall == sessionliveness.StallLeaseHeartbeat ||
		stall == sessionliveness.StallToolProgress {
		// StallToolProgress (#324) is likewise a wedged receive loop: the lane is
		// alive at the PTY layer (a spinner) but has made no tool-call progress, so
		// it never cleanly returns to idle.
		return c.contractFail(adapter, LoopWedged,
			fmt.Sprintf("the receive loop wedged: liveness classified a %s stall instead of clean idle", stall),
			liveFields(deadline, "loop_wedged", snap, ""))
	}
	if !agentRes.ReachedIdle {
		return c.contractFail(adapter, NoWorkLoopMissed,
			"the lane never re-awaited after work.complete: the no_work receive loop did not continue",
			liveFields(deadline, "no_work_loop_missed", snap, ""))
	}
	return c.pass(adapter, "the next work.await_packet returned no_work with no idle/lease stall raised")
}

// liveFields builds the structured routing fields for a live clause result
// (DESIGN §1.2): the deadline, the last observed protocol event, the supervisor
// state proxy (the session liveness classification), and the active lease id.
func liveFields(deadline time.Duration, situation string, snap SessionSnapshot, jobID string) map[string]string {
	fields := map[string]string{
		"deadline":            deadline.String(),
		"situation":           situation,
		"last_protocol_event": lastProtocolEvent(snap),
		"liveness_protocol":   snap.Classification.Protocol,
		"liveness_stall":      snap.Classification.StallClass,
	}
	if jobID != "" {
		fields["job_id"] = jobID
	}
	return fields
}

// lastProtocolEvent names the most advanced protocol column observed, for
// routing fields.
func lastProtocolEvent(snap SessionSnapshot) string {
	switch {
	case snap.LastWorkCompleteAt != nil:
		return sessionliveness.LastWorkCompleteAt
	case snap.LastWorkHeartbeatAt != nil:
		return sessionliveness.LastWorkHeartbeatAt
	case snap.LastAckAt != nil:
		return sessionliveness.LastAckAt
	case snap.LastAwaitPacketAt != nil:
		return sessionliveness.LastAwaitPacketAt
	case snap.LastToolsListAt != nil:
		return sessionliveness.LastToolsListAt
	default:
		return "none"
	}
}
