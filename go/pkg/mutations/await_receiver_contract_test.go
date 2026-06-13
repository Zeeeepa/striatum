package mutations

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestAwaitPacketTerminalEnvelopeDrivesReceiverExit is the RFC 0120 Phase 1
// daemon↔receiver exit contract, asserted end to end across the two packages
// that own it. The closed-session daemon behavior is locked by
// TestAwaitPacketClosedSessionReturnsTerminalEnvelope (the envelope shape) and
// the receiver predicate is unit-tested in agentloop on hand-built envelopes;
// neither catches a drift in the SEAM between them. Here the real
// work.await_packet handler runs against a real non-active session for every
// terminal state, and its actual envelope is fed to the real
// agentloop.EnvelopeRequestsIdleExit predicate. This is the F3 regression
// guard: before the fix the handler returned a plain invalid_transition error
// for closed/expired/lost sessions (only `stopped` got an envelope), so the
// receiver never saw an exit signal and error-looped every backoff. A future
// change that renames an envelope field (daemon side) or tightens the predicate
// (receiver side) without updating the other reopens that loop and turns this
// red, where the two in-package suites would both stay green.
func TestAwaitPacketTerminalEnvelopeDrivesReceiverExit(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	role := "worker"
	lane := "claude"

	// Every non-active session state from the sessions.state CHECK except
	// 'active' itself: the await loop must turn each into a receiver-exit
	// envelope, never an error.
	for _, state := range []string{"closed", "expired", "lost", "stopped"} {
		t.Run(state, func(t *testing.T) {
			repoID := "repo_await_recv_" + state
			runID := "run_await_recv_" + state
			sessionID := "sess_await_recv_" + state
			jobID := "job_await_recv_" + state

			intgSeedRepo(t, ctx, runner, repoID)
			intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
				"workflow_id": "wf",
				"roles":       map[string]any{role: map[string]any{}},
				"lanes":       map[string]any{lane: map[string]any{"display_model": "Claude"}},
			})
			intgSeedSession(t, ctx, runner, repoID, runID, sessionID, role, lane, nil, state)
			// Committed eligible work exists; a terminal session must still exit
			// (and must not claim it) rather than spin.
			intgSeedClaimableWork(t, ctx, runner, repoID, runID, jobID, "work", role, lane)

			env, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
			if err != nil {
				t.Fatalf("await_packet against a %s session returned an error (the F3 error-loop): %v", state, err)
			}
			if env["type"] != "session_terminal" || env["session_state"] != state {
				t.Fatalf("%s envelope = %#v, want type=session_terminal session_state=%s", state, env, state)
			}
			if !agentloop.EnvelopeRequestsIdleExit(env) {
				t.Fatalf("the agent-loop receiver would NOT exit on the daemon's %s envelope %#v: the daemon↔receiver exit contract is broken", state, env)
			}
		})
	}
}
