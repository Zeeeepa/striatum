package agentloop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/halbritt/striatum/go/pkg/turndriver"
)

const (
	envTurnDriverLeaseSeconds     = "STRIATUM_TURN_DRIVER_LEASE_SECONDS"
	envTurnDriverPollMillis       = "STRIATUM_TURN_DRIVER_POLL_INTERVAL_MS"
	envTurnDriverGenerateSeconds  = "STRIATUM_TURN_DRIVER_GENERATE_TIMEOUT_SECONDS"
	envTurnDriverMaxAttempts      = "STRIATUM_TURN_DRIVER_MAX_ATTEMPTS"
	envTurnDriverMaxOutputBytes   = "STRIATUM_TURN_DRIVER_MAX_OUTPUT_BYTES"
	envTurnDriverConversationID   = "STRIATUM_CONVERSATION_ID"
	defaultTurnDriverLeaseSeconds = 1800
)

func RunTurnDriver(socketPath, repoRoot, runID, sessionID string, command []string) error {
	_ = socketPath
	if sessionID == "" || runID == "" || repoRoot == "" {
		return fmt.Errorf("STRIATUM_SESSION_ID, STRIATUM_RUN_ID, and STRIATUM_REPO must be in environment")
	}
	if len(command) == 0 {
		return fmt.Errorf("turn-driver content generator command is required")
	}
	endpoint, err := ResolveMCPEndpoint(repoRoot, os.Environ())
	if err != nil {
		return err
	}
	repositoryID := os.Getenv(EnvRepositoryID)
	if strings.TrimSpace(repositoryID) == "" {
		return fmt.Errorf("%s must be set for the turn-driver MCP client", EnvRepositoryID)
	}
	token, err := ResolveTokenMaterial(repoRoot, os.Environ())
	if err != nil {
		return err
	}

	ctx, stop := signalContext()
	defer stop()

	client := &mcpToolClient{
		endpoint:     endpoint,
		repositoryID: repositoryID,
		token:        token,
		httpClient:   &http.Client{Timeout: 45 * time.Second},
	}
	conversation := &mcpConversation{
		client:       client,
		sessionID:    sessionID,
		leaseSeconds: envInt(envTurnDriverLeaseSeconds, defaultTurnDriverLeaseSeconds),
	}
	_ = conversation.Report(ctx, "ready", "await_packet", "turn-driver ready", "")
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go conversation.Heartbeat(heartbeatCtx)

	loop := turndriver.Loop{
		Conversation: conversation,
		Generator: CommandGenerator{
			Command: command,
			WorkDir: repoRoot,
			BaseEnv: os.Environ(),
		},
		Options: turndriver.Options{
			PollInterval:          time.Duration(envInt(envTurnDriverPollMillis, 1000)) * time.Millisecond,
			GenerateTimeout:       time.Duration(envInt(envTurnDriverGenerateSeconds, 180)) * time.Second,
			MaxGenerateAttempts:   envInt(envTurnDriverMaxAttempts, 2),
			MaxOutputBytes:        envInt(envTurnDriverMaxOutputBytes, 64*1024),
			InitialConversationID: strings.TrimSpace(os.Getenv(envTurnDriverConversationID)),
			OnFailure: func(ctx context.Context, failure turndriver.Failure) error {
				return conversation.ReportFailure(ctx, failure)
			},
		},
	}
	return loop.Run(ctx)
}

type CommandGenerator struct {
	Command []string
	WorkDir string
	BaseEnv []string
}

func (g CommandGenerator) Generate(ctx context.Context, conversation turndriver.ConversationContext) (string, error) {
	if len(g.Command) == 0 {
		return "", fmt.Errorf("content generator command is required")
	}
	prompt := turndriver.RenderPrompt(conversation)
	args := append([]string(nil), g.Command[1:]...)
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, g.Command[0], args...)
	cmd.Dir = g.WorkDir
	cmd.Env = ContentOnlyEnv(g.BaseEnv)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("content generator failed: %w: %s", err, truncate(detail, 280))
		}
		return "", fmt.Errorf("content generator failed: %w", err)
	}
	return stdout.String(), nil
}

func ContentOnlyEnv(base []string) []string {
	out := make([]string, 0, len(base))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if strings.HasPrefix(key, "STRIATUM_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

type mcpConversation struct {
	client       *mcpToolClient
	sessionID    string
	leaseSeconds int
}

func (c *mcpConversation) AwaitTurn(ctx context.Context) (turndriver.Turn, error) {
	data, err := c.client.Call(ctx, "work.await_packet", map[string]any{
		"session_id":    c.sessionID,
		"lease_seconds": c.leaseSeconds,
		"conversation":  true,
		"driver_mode":   "turn_driver",
		"content_only":  true,
		"self_driving":  false,
	})
	if err != nil {
		return turndriver.Turn{}, err
	}
	kind := stringValue(data["type"])
	if kind == "" {
		kind = stringValue(data["status"])
	}
	switch kind {
	case "conversation_message":
		return turndriver.Turn{
			Kind:           turndriver.TurnOurTurn,
			ConversationID: stringValue(data["conversation_id"]),
			Topic:          stringValue(data["topic"]),
			Transcript:     transcriptEntries(data["transcript"]),
			TurnIndex:      intValue(data["turn_index"]),
		}, nil
	case "none", "no_work":
		return turndriver.Turn{Kind: turndriver.TurnNoWork}, nil
	case "conversation_closed":
		return turndriver.Turn{Kind: turndriver.TurnClosed, ConversationID: stringValue(data["conversation_id"])}, nil
	case "work_packet", "interrogation_question":
		return turndriver.Turn{}, fmt.Errorf("%w: %s", turndriver.ErrUnexpectedEnvelope, kind)
	default:
		return turndriver.Turn{}, fmt.Errorf("unsupported await_packet envelope type %q", kind)
	}
}

func (c *mcpConversation) Say(ctx context.Context, conversationID string, body string) error {
	_, err := c.client.Call(ctx, "conversation.say", map[string]any{
		"session_id":        c.sessionID,
		"conversation_id":   conversationID,
		"body":              body,
		"driver_mode":       "turn_driver",
		"content_only_body": true,
	})
	if turndriver.IsNotYourTurn(err) {
		return turndriver.ErrNotYourTurn
	}
	return err
}

func (c *mcpConversation) Show(ctx context.Context, conversationID string) (turndriver.ConversationState, error) {
	data, err := c.client.Call(ctx, "conversation.show", map[string]any{
		"conversation_id": conversationID,
		"session_id":      c.sessionID,
	})
	if err != nil {
		return turndriver.ConversationState{}, err
	}
	conversation := asMap(data["conversation"])
	return turndriver.ConversationState{
		ConversationID: stringValue(conversation["conversation_id"]),
		State:          stringValue(conversation["state"]),
	}, nil
}

func (c *mcpConversation) ReportFailure(ctx context.Context, failure turndriver.Failure) error {
	kind := "other"
	if errors.Is(failure.Err, context.DeadlineExceeded) {
		kind = "model_timeout"
	}
	message := fmt.Sprintf("turn-driver parked floor for %s after %d attempt(s): %v", failure.Turn.ConversationID, failure.Attempts, failure.Err)
	return c.Report(ctx, "escalate", "await_packet", truncate(message, 280), kind)
}

func (c *mcpConversation) Report(ctx context.Context, reportKind, phase, message, blockerKind string) error {
	args := map[string]any{
		"session_id":  c.sessionID,
		"report_kind": reportKind,
		"phase":       phase,
		"message":     truncate(message, 280),
		"driver_mode": "turn_driver",
	}
	if blockerKind != "" {
		args["blocker_kind"] = blockerKind
	}
	_, err := c.client.Call(ctx, "session.report", args)
	return err
}

func (c *mcpConversation) Heartbeat(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.Report(ctx, "heartbeat", "await_packet", "turn-driver waiting or generating", "")
		}
	}
}

type mcpToolClient struct {
	endpoint     string
	repositoryID string
	token        TokenMaterial
	httpClient   *http.Client
	requestSeq   atomic.Int64
}

func (c *mcpToolClient) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, error) {
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
	if _, ok := arguments["repository_id"]; !ok {
		arguments["repository_id"] = c.repositoryID
	}
	token, err := c.currentToken()
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("turn-driver-%d", c.requestSeq.Add(1)),
		"method":  "tools/call",
		"params": map[string]any{
			"name":          name,
			"repository_id": c.repositoryID,
			"arguments":     arguments,
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limited, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP tools/call HTTP status %d: %s", resp.StatusCode, truncate(string(limited), 280))
	}
	decoder := json.NewDecoder(bytes.NewReader(limited))
	decoder.UseNumber()
	var decoded struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    int            `json:"code"`
			Message string         `json:"message"`
			Data    map[string]any `json:"data"`
		} `json:"error"`
	}
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode MCP tools/call response: %w", err)
	}
	if decoded.Error != nil {
		code := stringValue(decoded.Error.Data["code"])
		if code == "" {
			code = "mcp_jsonrpc_error"
		}
		return nil, &toolCallError{code: code, message: decoded.Error.Message}
	}
	structured := asMap(decoded.Result["structuredContent"])
	if boolValue(decoded.Result["isError"]) || boolValue(structured["isError"]) || boolValue(structured["error"]) {
		code := stringValue(structured["error"])
		if code == "" {
			code = "command_failed"
		}
		return nil, &toolCallError{code: code, message: stringValue(structured["error_message"])}
	}
	return asMap(structured["data"]), nil
}

func (c *mcpToolClient) currentToken() (string, error) {
	if c.token.Source != "" && c.token.Source != EnvMCPToken {
		return ReadTokenFile(c.token.Source)
	}
	if strings.TrimSpace(c.token.Token) == "" {
		return "", fmt.Errorf("turn-driver requires a non-empty MCP token")
	}
	return strings.TrimSpace(c.token.Token), nil
}

type toolCallError struct {
	code    string
	message string
}

func (e *toolCallError) Error() string {
	if e.message == "" {
		return e.code
	}
	return e.message
}

func (e *toolCallError) ErrorCode() string {
	return e.code
}

func transcriptEntries(value any) []turndriver.TranscriptEntry {
	items := asList(value)
	out := make([]turndriver.TranscriptEntry, 0, len(items))
	for _, item := range items {
		row := asMap(item)
		out = append(out, turndriver.TranscriptEntry{
			TurnIndex:       intValue(row["turn_index"]),
			AuthorSessionID: stringValue(row["author_session_id"]),
			Body:            stringValue(row["body"]),
		})
	}
	return out
}

func asMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		return map[string]any{}
	}
}

func asList(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	default:
		return nil
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	default:
		return false
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
