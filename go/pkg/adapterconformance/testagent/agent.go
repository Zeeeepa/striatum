// Package testagent is the RFC 0101 Layer 2 configurable fake MCP agent: one
// in-process JSON-RPC client that speaks the lane bootstrap+turn protocol to the
// conformance harness's MCP HTTP endpoint, so the daemon-lifecycle clauses
// (C3..C10) can be asserted with NO provider CLI and NO PTY (DESIGN §1.3
// testagent / §2.5).
//
// A single configurable agent drives the happy-path single-job lifecycle and a
// closed set of broken modes; each broken mode is engineered to ARM exactly one
// contract FailureClass in the harness taxonomy:
//
//	happy                -> all in-scope clauses Pass
//	never_tools_list     -> BootstrapStall   (C3: never issues tools/list)
//	await_never          -> AwaitPacketStall (C4: tools/list but never awaits)
//	ack_never            -> AckStall         (C5: receives packet, never acks)
//	no_heartbeat         -> HeartbeatMissed  (C6: acks, never heartbeats)
//	ignore_interrogation -> InterrogationIgnored (C7: never answers the question)
//	exit_before_complete -> TurnExitEarly / CompleteMissing (C9/C11: exits early)
//
// The agent talks MCP over HTTP exactly as a real lane would: an `initialize`
// handshake, a `tools/list` discovery, then `tools/call` for every mutation,
// carrying the bearer token and `repository_id`/`session_id` in each call. It
// never touches PostgreSQL directly.
//
// Slice-2b extension (DEFERRED): launch this agent through supervise.start ->
// RunHelper (the real PTY/argv path) for C11/C12. This in-process client is the
// deterministic Tier-A path that arms the taxonomy without a provider binary.
package testagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Mode selects the agent's behavior. The zero value (ModeHappy) drives the full
// lifecycle to completion.
type Mode string

const (
	// ModeHappy drives the full single-job lifecycle to work.complete and a
	// terminal no_work await. All in-scope clauses pass.
	ModeHappy Mode = "happy"
	// ModeNeverToolsList registers a session but never issues tools/list, so
	// last_tools_list_at stays null (C3 BootstrapStall).
	ModeNeverToolsList Mode = "never_tools_list"
	// ModeAwaitNever issues tools/list but never calls work.await_packet, so
	// last_await_packet_at stays null (C4 AwaitPacketStall).
	ModeAwaitNever Mode = "await_never"
	// ModeAckNever receives the packet but never acks it (C5 AckStall).
	ModeAckNever Mode = "ack_never"
	// ModeNoHeartbeat acks the packet but never heartbeats (C6 HeartbeatMissed).
	ModeNoHeartbeat Mode = "no_heartbeat"
	// ModeIgnoreInterrogation acks+heartbeats but never answers the delivered
	// interrogation question (C7 InterrogationIgnored).
	ModeIgnoreInterrogation Mode = "ignore_interrogation"
	// ModeExitBeforeComplete publishes the artifact but exits before
	// work.complete (C9 CompleteMissing / C11 TurnExitEarly).
	ModeExitBeforeComplete Mode = "exit_before_complete"
)

// Config parameterizes one agent run against the harness MCP endpoint.
type Config struct {
	// Endpoint is the MCP HTTP endpoint (e.g. httpServer.URL + "/mcp").
	Endpoint string
	// Token is the bearer capability token (id.secret form).
	Token string
	// RepositoryID, RunID, Role, Lane identify the session to register.
	RepositoryID string
	RunID        string
	Role         string
	Lane         string
	// Mode selects happy-path vs a broken behavior.
	Mode Mode

	// Artifact describes the single artifact the happy path publishes. The file
	// at AbsPath must already exist on disk (the fixture writes it); RepoPath is
	// the repo-relative path the publisher records.
	Artifact ArtifactSpec

	// InterrogationGate, when non-nil, is closed by the agent once it has acked
	// and heartbeated and is about to enter its interrogation-answer wait. The
	// harness uses it to seed+open+ask the interrogation at the right moment so
	// the round trip is deterministic (C7). Ignored when nil.
	//
	// RFC 0101 Phase 0a (#137): the agent no longer waits for an "ask committed"
	// signal after closing the gate — it polls work.await_packet CONCURRENTLY
	// with the harness's interrogation.open/ask/answer. That concurrency used to
	// deadlock the two transactions on the same DB (the #103 shape); the
	// daemon-side root fix (per-run interrogation advisory lock +
	// withTxRetryOnDeadlock on the interrogation handlers) makes it safe, so the
	// former InterrogationReady ("ask committed") handshake is retired and this
	// C7 path regression-proves the fix end to end.
	InterrogationGate chan struct{}
	// InterrogationSeeded, when non-nil, is awaited by the agent AFTER it closes
	// InterrogationGate and BEFORE it begins answer-polling. The harness closes it
	// once the interrogator session + its attestation rows are SEEDED (pure test
	// setup) — NOT after the interrogation is asked. This serializes only the raw
	// fixture INSERTs (which take FK row-share locks on runs/sessions and would
	// otherwise race the claim path's FOR UPDATE — a test-seeding artifact, not the
	// product deadlock), while leaving interrogation.open/ask CONCURRENT with the
	// agent's poll so the product deadlock fix is genuinely exercised. Ignored when
	// nil.
	InterrogationSeeded chan struct{}
	// InterrogationWait bounds how long the agent waits for a delivered
	// interrogation question before giving up. ModeIgnoreInterrogation also waits
	// this long but never answers.
	InterrogationWait time.Duration

	// HTTPClient overrides the default client (tests may inject a tuned one).
	HTTPClient *http.Client
}

// ArtifactSpec is the single artifact the happy path publishes.
type ArtifactSpec struct {
	Kind        string
	LogicalName string
	RepoPath    string // repo-relative path recorded by the publisher
}

// Result is the agent's terminal observation: the ids it learned and whether it
// reached a clean no_work await. Errors short-circuit the lifecycle; a broken
// mode that deliberately stops early returns nil error with Completed=false.
type Result struct {
	SessionID   string
	JobID       string
	LeaseID     string
	MessageID   string
	PacketID    string
	Heartbeats  int
	Completed   bool
	ReachedIdle bool
	AnsweredAsk bool
}

// Agent is a configured fake MCP agent.
type Agent struct {
	cfg    Config
	client *http.Client

	mu        sync.Mutex
	rpcSeq    int
	sessionID string
}

// New builds an Agent from cfg.
func New(cfg Config) *Agent {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.InterrogationWait == 0 {
		cfg.InterrogationWait = 5 * time.Second
	}
	return &Agent{cfg: cfg, client: client}
}

// SessionID returns the registered session id (empty until register runs). It is
// safe to call concurrently with Run.
func (a *Agent) SessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

// Run drives the configured lifecycle. It returns nil error on a clean run OR a
// deliberate broken-mode early stop; it returns an error only on an unexpected
// protocol failure (which is itself a useful signal).
func (a *Agent) Run(ctx context.Context) (Result, error) {
	var res Result

	// initialize handshake (every real lane does this first).
	if _, err := a.rpc(ctx, "initialize", map[string]any{}); err != nil {
		return res, fmt.Errorf("initialize: %w", err)
	}

	// session.register — the agent's identity for the run/role/lane slot.
	regData, err := a.call(ctx, "session.register", map[string]any{
		"run_id": a.cfg.RunID,
		"role":   a.cfg.Role,
		"lane":   a.cfg.Lane,
		"fresh":  true,
	})
	if err != nil {
		return res, fmt.Errorf("session.register: %w", err)
	}
	res.SessionID = stringField(regData, "session_id")
	a.mu.Lock()
	a.sessionID = res.SessionID
	a.mu.Unlock()
	if res.SessionID == "" {
		return res, fmt.Errorf("session.register returned no session_id: %v", regData)
	}

	// C3 broken mode: never discover. Stop here; last_tools_list_at stays null.
	if a.cfg.Mode == ModeNeverToolsList {
		return res, nil
	}

	// tools/list — bootstrap discovery; stamps last_tools_list_at.
	if _, err := a.rpc(ctx, "tools/list", map[string]any{
		"repository_id": a.cfg.RepositoryID,
		"session_id":    res.SessionID,
	}); err != nil {
		return res, fmt.Errorf("tools/list: %w", err)
	}

	// C4 broken mode: discovered but never await. last_await_packet_at stays null.
	if a.cfg.Mode == ModeAwaitNever {
		return res, nil
	}

	// work.await_packet — foreground claim. Loops past any interrogation question
	// envelope until it gets the work packet (a real lane handles both).
	packet, err := a.awaitWorkPacket(ctx, res.SessionID)
	if err != nil {
		return res, fmt.Errorf("work.await_packet: %w", err)
	}
	lease := mapField(packet, "lease")
	res.PacketID = stringField(packet, "packet_id")
	res.LeaseID = stringField(lease, "lease_id")
	res.MessageID = stringField(lease, "message_id")
	res.JobID = stringField(mapField(packet, "job"), "job_id")
	if res.LeaseID == "" || res.JobID == "" || res.MessageID == "" {
		return res, fmt.Errorf("work packet missing lease/job/message ids: %v", packet)
	}

	// C5 broken mode: packet delivered, never acked.
	if a.cfg.Mode == ModeAckNever {
		return res, nil
	}

	// work.ack — transition the job to running.
	if _, err := a.call(ctx, "work.ack", map[string]any{
		"session_id": res.SessionID,
		"message_id": res.MessageID,
		"lease_id":   res.LeaseID,
	}); err != nil {
		return res, fmt.Errorf("work.ack: %w", err)
	}

	// C6 broken mode: acked, never heartbeats.
	if a.cfg.Mode == ModeNoHeartbeat {
		return res, nil
	}

	// work.heartbeat (>=1x) — proves the lease is held and refreshed.
	if _, err := a.call(ctx, "work.heartbeat", map[string]any{
		"session_id": res.SessionID,
		"lease_id":   res.LeaseID,
	}); err != nil {
		return res, fmt.Errorf("work.heartbeat: %w", err)
	}
	res.Heartbeats++

	// C7: answer one interrogation question (when the harness scripts one). The
	// gate lets the harness open+ask precisely now, so the round trip is
	// deterministic. ModeIgnoreInterrogation waits but never answers.
	if a.cfg.InterrogationGate != nil {
		close(a.cfg.InterrogationGate)
		// RFC 0101 Phase 0a (#137): wait only until the harness has SEEDED the
		// interrogator attestation (test-fixture setup), then poll for the question
		// while the harness's interrogation.open/ask runs CONCURRENTLY — racing the
		// answer against the ask exactly as production does. The daemon-side root
		// fix makes that race deadlock-free (no 40P01).
		if a.cfg.InterrogationSeeded != nil {
			select {
			case <-a.cfg.InterrogationSeeded:
			case <-ctx.Done():
				return res, ctx.Err()
			}
		}
		answered, err := a.answerOneInterrogation(ctx, res.SessionID)
		if err != nil {
			return res, fmt.Errorf("interrogation round trip: %w", err)
		}
		res.AnsweredAsk = answered
		if a.cfg.Mode == ModeIgnoreInterrogation {
			// Deliberately stop after ignoring the question.
			return res, nil
		}
	}

	// artifact.publish — a valid front-matter artifact (file already on disk).
	if _, err := a.call(ctx, "artifact.publish", map[string]any{
		"session_id":   res.SessionID,
		"job_id":       res.JobID,
		"lease_id":     res.LeaseID,
		"kind":         a.cfg.Artifact.Kind,
		"logical_name": a.cfg.Artifact.LogicalName,
		"path":         a.cfg.Artifact.RepoPath,
	}); err != nil {
		return res, fmt.Errorf("artifact.publish: %w", err)
	}

	// C9/C11 broken mode: publish but exit before work.complete.
	if a.cfg.Mode == ModeExitBeforeComplete {
		return res, nil
	}

	// work.complete — finalize the job.
	if _, err := a.call(ctx, "work.complete", map[string]any{
		"session_id": res.SessionID,
		"job_id":     res.JobID,
		"lease_id":   res.LeaseID,
		"summary":    "conformance happy-path complete",
	}); err != nil {
		return res, fmt.Errorf("work.complete: %w", err)
	}
	res.Completed = true

	// work.await_packet again — the receive loop continues to a clean terminal
	// state. For a single-job run, completing the only job marks the run
	// completed and closes the session, so the daemon refuses the re-await with
	// "session is closed; register a fresh session". That refusal is the correct
	// no-more-work terminal signal for a single-job lane (the loop reached its
	// natural end), NOT a stall — distinct from a wedge, which would leave the
	// session active but classified stalled. Treat both a no_work envelope and a
	// run-completed session-closed refusal as ReachedIdle.
	idle, err := a.call(ctx, "work.await_packet", map[string]any{
		"session_id": res.SessionID,
	})
	if err != nil {
		if isRunCompletedClose(err) {
			res.ReachedIdle = true
			return res, nil
		}
		return res, fmt.Errorf("terminal work.await_packet: %w", err)
	}
	if status := stringField(idle, "status"); status == "no_work" || stringField(idle, "type") == "no_work" {
		res.ReachedIdle = true
	}
	return res, nil
}

// isRunCompletedClose reports whether an await error is the benign
// "session is closed" refusal a single-job run produces after its only job
// completes (the run completes and closes the session). It is the terminal
// no-more-work signal, not a protocol stall.
func isRunCompletedClose(err error) bool {
	return err != nil && strings.Contains(err.Error(), "session is closed")
}

// awaitWorkPacket calls work.await_packet, skipping any interrogation_question
// or conversation envelope, until it gets a work_packet.
func (a *Agent) awaitWorkPacket(ctx context.Context, sessionID string) (map[string]any, error) {
	for attempt := 0; attempt < 8; attempt++ {
		data, err := a.call(ctx, "work.await_packet", map[string]any{"session_id": sessionID})
		if err != nil {
			return nil, err
		}
		switch stringField(data, "type") {
		case "work_packet":
			if packet := mapField(data, "packet"); len(packet) > 0 {
				return packet, nil
			}
			return data, nil
		case "interrogation_question", "conversation_turn":
			// A real lane would handle these; for fetching the work packet we
			// loop until work is delivered.
			continue
		default:
			if stringField(data, "status") == "claimed" {
				if packet := mapField(data, "packet"); len(packet) > 0 {
					return packet, nil
				}
			}
			// no_work or unknown -> retry briefly; work is seeded so this is rare.
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("no work packet after retries")
}

// answerOneInterrogation waits for a delivered interrogation_question and, in
// every mode except ModeIgnoreInterrogation, answers it. It returns whether an
// answer was sent. The wait is bounded by InterrogationWait.
func (a *Agent) answerOneInterrogation(ctx context.Context, sessionID string) (bool, error) {
	deadline := time.Now().Add(a.cfg.InterrogationWait)
	for time.Now().Before(deadline) {
		data, err := a.call(ctx, "work.await_packet", map[string]any{"session_id": sessionID})
		if err != nil {
			return false, err
		}
		if stringField(data, "type") == "interrogation_question" {
			if a.cfg.Mode == ModeIgnoreInterrogation {
				return false, nil
			}
			interrogationID := stringField(data, "interrogation_id")
			if interrogationID == "" {
				return false, fmt.Errorf("interrogation_question missing interrogation_id: %v", data)
			}
			if _, err := a.call(ctx, "interrogation.answer", map[string]any{
				"session_id":       sessionID,
				"interrogation_id": interrogationID,
				"body":             "because the fixture constraint requires it",
			}); err != nil {
				return false, fmt.Errorf("interrogation.answer: %w", err)
			}
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return false, nil
}

// call issues a tools/call for an RPC method, returning the handler's `data`
// map. A handler-level error (structuredContent.ok == false) is surfaced as an
// error carrying the daemon error code+message.
func (a *Agent) call(ctx context.Context, method string, arguments map[string]any) (map[string]any, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	if _, ok := arguments["repository_id"]; !ok {
		arguments["repository_id"] = a.cfg.RepositoryID
	}
	result, err := a.rpc(ctx, "tools/call", map[string]any{
		"name":          method,
		"repository_id": a.cfg.RepositoryID,
		"arguments":     arguments,
	})
	if err != nil {
		return nil, err
	}
	structured := mapField(result, "structuredContent")
	if structured == nil {
		return nil, fmt.Errorf("%s: response missing structuredContent: %v", method, result)
	}
	if ok, _ := structured["ok"].(bool); !ok {
		code := stringField(structured, "error")
		msg := stringField(structured, "error_message")
		return nil, fmt.Errorf("%s failed: %s: %s", method, code, msg)
	}
	return mapField(structured, "data"), nil
}

// rpc posts a single JSON-RPC request and returns the result object.
func (a *Agent) rpc(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	a.mu.Lock()
	a.rpcSeq++
	id := fmt.Sprintf("%s-%d", method, a.rpcSeq)
	a.mu.Unlock()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if a.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode %s response (status %d): %w; body=%s", method, resp.StatusCode, err, string(raw))
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("%s JSON-RPC error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result, nil
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func mapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}
