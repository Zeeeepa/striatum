package adapterconformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestInterrogationAwaitDeadlock is the RFC 0101 Phase 0a regression: it races a
// target session's work.await_packet receive loop against the interrogator's
// interrogation.ask + the target's interrogation.answer on the SAME run, with NO
// InterrogationReady handshake, in a tight loop across several goroutines. Before
// the root fix this provoked a Postgres deadlock (SQLSTATE 40P01) between the two
// transactions, which the conformance harness only masked with the
// InterrogationReady channel (#103/#137 shape). The fix (advisory-lock
// serialization point + deadlock-retry wrapper on the interrogation handlers)
// must make this loop complete with NO 40P01 and a full interrogation round trip.
//
// PG-gated: pgtest.Pool SKIPs when STRIATUM_PG_TEST_URL is unset; the ephemeral
// database is made/dropped by pgtest and never touches the live striatum_daemon.
func TestInterrogationAwaitDeadlock(t *testing.T) {
	const iterations = 50

	repositoryID := "repo_intgdeadlock"
	ctx := context.Background()
	h := NewHarness(t, repositoryID)
	fx := SeedFixture(t, ctx, h.Runner, repositoryID)

	// Bring the target session to a live, attested, post-heartbeat state via the
	// real MCP stack, exactly as the happy path does up to the interrogation gate.
	target := driveTargetToInterrogationReady(t, ctx, h, fx)

	// Put the target into the RFC 0082 §5 awaiting_interrogation window: an
	// interrogable agent-loop session that completed its interrogable job and is
	// being interrogated by a panel. requireLiveTarget admits such a target
	// regardless of probe/liveness timing (interrogation.go requireLiveTarget ->
	// targetInAwaitingInterrogation), which is exactly the production #137 shape:
	// reviewers interrogate a completed interrogable target while it polls its own
	// await loop. This avoids coupling the deadlock proof to lanehealth probe
	// timing.
	seedAwaitingInterrogation(t, ctx, h.Runner, fx, target.SessionID)

	// Seed the interrogator against the now-live target.
	fx.SeedInterrogator(t, ctx, h.Runner, target.SessionID)

	for i := 0; i < iterations; i++ {
		// Open one interrogation per iteration and commit the question turn up front
		// (interrogation.ask), so the delivery + answer + sibling claims below race
		// over the SAME run's sessions/runs/queue rows. Committing the ask first
		// makes the run deterministic while keeping the deadlocking concurrency —
		// the cycle is between the claim path (sessions FOR UPDATE -> runs FOR
		// UPDATE -> append) and the answer (UPDATE sessions via sessionliveness),
		// not the ask itself.
		interrogationID := openInterrogation(t, ctx, h, fx, target.SessionID)
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
			t.Fatalf("iteration %d: interrogation.ask: %v", i, err)
		}

		// Deliver the question to the target's receive loop (acks it), so the answer
		// has a delivered question to complete. Delivery runs in its own tx.
		if _, err := mutations.HandleAwaitPacket(ctx, h.Runner, rpc.Envelope{
			SchemaVersion: rpc.SupportedEnvelopeVersion,
			Method:        "work.await_packet",
			Params:        map[string]any{"repository_id": fx.RepositoryID, "session_id": target.SessionID},
		}); err != nil {
			t.Fatalf("iteration %d: deliver question: %v", i, err)
		}

		// Race the deadlocking transactions: several claim-path transactions
		// (HandleClaimNext: sessions FOR UPDATE -> runs FOR UPDATE -> append
		// chain-head) against the target's interrogation.answer (UPDATE sessions via
		// sessionliveness.Record -> append chain-head) on the SAME run. All
		// goroutines release from a common start barrier so they hit Postgres
		// simultaneously, maximizing the window for the {sessions, runs}/chain-head
		// cycle that aborts one transaction with 40P01 before the root fix.
		const claimers = 4
		var wg sync.WaitGroup
		errs := make(chan error, claimers+1)
		start := make(chan struct{})

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := mutations.HandleInterrogationAnswer(ctx, h.Runner, rpc.Envelope{
				SchemaVersion: rpc.SupportedEnvelopeVersion,
				Method:        "interrogation.answer",
				Params: map[string]any{
					"repository_id":    fx.RepositoryID,
					"session_id":       target.SessionID,
					"interrogation_id": interrogationID,
					"body":             "because the fixture constraint requires it",
				},
			}); err != nil {
				errs <- fmt.Errorf("answer: %w", err)
			}
		}()

		for c := 0; c < claimers; c++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				// The target session holds an active lease + completed interrogable
				// job, so claim_next finds no_work — but it still locks sessions+runs
				// and appends, which is the half of the cycle under test.
				if _, err := mutations.HandleClaimNext(ctx, h.Runner, rpc.Envelope{
					SchemaVersion: rpc.SupportedEnvelopeVersion,
					Method:        "work.claim_next",
					Params:        map[string]any{"repository_id": fx.RepositoryID, "session_id": target.SessionID},
				}); err != nil {
					errs <- fmt.Errorf("claim: %w", err)
				}
			}()
		}

		close(start) // release all goroutines at once
		wg.Wait()
		close(errs)
		for err := range errs {
			if strings.Contains(err.Error(), "40P01") || strings.Contains(err.Error(), "deadlock") {
				t.Fatalf("iteration %d: deadlock surfaced (root fix failed): %v", i, err)
			}
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		answered, err := h.Observer().InterrogationAnswered(ctx, fx.RepositoryID, interrogationID)
		if err != nil {
			t.Fatalf("iteration %d: observer.InterrogationAnswered: %v", i, err)
		}
		if !answered {
			t.Fatalf("iteration %d: interrogation %s was not answered (round trip incomplete)", i, interrogationID)
		}
		// Close the interrogation so the next iteration opens a clean one; the
		// target stays live because it still holds its active lease.
		closeInterrogation(t, ctx, h, fx, interrogationID)
	}
}

// targetState is the minimal identity the deadlock race needs.
type targetState struct {
	SessionID string
	LeaseID   string
}

// seedAwaitingInterrogation records the RFC 0082 §5 session.awaiting_interrogation
// window event for the target's interrogable job, so requireLiveTarget admits it
// as a live interrogation target (interrogation.go targetInAwaitingInterrogation)
// without depending on lanehealth probe/stall timing. This mirrors the
// production #137 panel: an interrogable agent-loop target that completed its job
// and is interrogated while it continues to poll its own await loop.
func seedAwaitingInterrogation(t *testing.T, ctx context.Context, runner db.Runner, fx *Fixture, targetSessionID string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.events (repository_id, run_id, event_type, actor_session_id, job_id, created_at)
		VALUES ($1,$2,'session.awaiting_interrogation',$3,$4,now())`,
		fx.RepositoryID, fx.RunID, targetSessionID, fx.JobID); err != nil {
		t.Fatalf("seed awaiting_interrogation event: %v", err)
	}
}

// driveTargetToInterrogationReady registers the fake agent's session and drives
// register -> tools/list -> await(claim) -> ack -> heartbeat through the real MCP
// stack so the target session is live and holds its job lease, mirroring the
// happy path up to the interrogation gate (testagent.Run :226-247).
func driveTargetToInterrogationReady(t *testing.T, ctx context.Context, h *Harness, fx *Fixture) targetState {
	t.Helper()
	c := &mcpClient{t: t, ctx: ctx, endpoint: h.MCPEndpoint(), token: h.Token, repositoryID: fx.RepositoryID}

	c.rpc("initialize", map[string]any{})
	reg := c.call("session.register", map[string]any{
		"run_id": fx.RunID, "role": fx.Role, "lane": fx.Lane, "fresh": true,
	})
	sessionID := fmt.Sprint(reg["session_id"])
	if sessionID == "" {
		t.Fatalf("session.register returned no session_id: %v", reg)
	}
	// dbf2013b: attest the registered session (a real lane is attested by
	// supervise start) so work.await_packet/claim below is admitted.
	attestFixtureSession(t, ctx, h.Runner, fx.RepositoryID, fx.RunID, sessionID, fx.Lane)
	c.rpc("tools/list", map[string]any{"repository_id": fx.RepositoryID, "session_id": sessionID})

	var leaseID, messageID string
	for attempt := 0; attempt < 16; attempt++ {
		env := c.call("work.await_packet", map[string]any{"session_id": sessionID})
		if fmt.Sprint(env["type"]) == "work_packet" {
			packet, _ := env["packet"].(map[string]any)
			lease, _ := packet["lease"].(map[string]any)
			leaseID = fmt.Sprint(lease["lease_id"])
			messageID = fmt.Sprint(lease["message_id"])
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if leaseID == "" {
		t.Fatalf("never received a work packet for the target session")
	}
	c.call("work.ack", map[string]any{"session_id": sessionID, "message_id": messageID, "lease_id": leaseID})
	c.call("work.heartbeat", map[string]any{"session_id": sessionID, "lease_id": leaseID})

	return targetState{SessionID: sessionID, LeaseID: leaseID}
}

func openInterrogation(t *testing.T, ctx context.Context, h *Harness, fx *Fixture, targetSessionID string) string {
	t.Helper()
	res, err := mutations.HandleInterrogationOpen(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.open",
		Params: map[string]any{
			"repository_id":     fx.RepositoryID,
			"session_id":        fx.InterrogatorSessionID,
			"target_session_id": targetSessionID,
			"topic":             "deadlock race",
		},
	})
	if err != nil {
		t.Fatalf("interrogation.open (target=%s): %v", targetSessionID, err)
	}
	return fmt.Sprint(res["interrogation_id"])
}

func closeInterrogation(t *testing.T, ctx context.Context, h *Harness, fx *Fixture, interrogationID string) {
	t.Helper()
	if _, err := mutations.HandleInterrogationClose(ctx, h.Runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "interrogation.close",
		Params: map[string]any{
			"repository_id":    fx.RepositoryID,
			"session_id":       fx.InterrogatorSessionID,
			"interrogation_id": interrogationID,
		},
	}); err != nil {
		t.Fatalf("interrogation.close: %v", err)
	}
}

// mcpClient is a tiny synchronous MCP JSON-RPC client for the deadlock fixture's
// single-threaded setup phase. It reuses the testagent wire shape but is local to
// this test so the production agent stays untouched.
type mcpClient struct {
	t            *testing.T
	ctx          context.Context
	endpoint     string
	token        string
	repositoryID string
	seq          int
}

func (c *mcpClient) call(method string, arguments map[string]any) map[string]any {
	c.t.Helper()
	if arguments == nil {
		arguments = map[string]any{}
	}
	if _, ok := arguments["repository_id"]; !ok {
		arguments["repository_id"] = c.repositoryID
	}
	result := c.rpc("tools/call", map[string]any{
		"name":          method,
		"repository_id": c.repositoryID,
		"arguments":     arguments,
	})
	structured, _ := result["structuredContent"].(map[string]any)
	if structured == nil {
		c.t.Fatalf("%s: response missing structuredContent: %v", method, result)
	}
	if ok, _ := structured["ok"].(bool); !ok {
		c.t.Fatalf("%s failed: %v: %v", method, structured["error"], structured["error_message"])
	}
	data, _ := structured["data"].(map[string]any)
	return data
}

func (c *mcpClient) rpc(method string, params map[string]any) map[string]any {
	c.t.Helper()
	c.seq++
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("%s-%d", method, c.seq),
		"method":  method,
		"params":  params,
	})
	if err != nil {
		c.t.Fatalf("%s: marshal: %v", method, err)
	}
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		c.t.Fatalf("%s: new request: %v", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s: do: %v", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("%s: read body: %v", method, err)
	}
	var envelope struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		c.t.Fatalf("%s: decode (status %d): %v; body=%s", method, resp.StatusCode, err, string(raw))
	}
	if envelope.Error != nil {
		c.t.Fatalf("%s: JSON-RPC error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result
}
