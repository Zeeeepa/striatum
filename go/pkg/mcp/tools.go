package mcp

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

type Service struct {
	RPC              *rpc.Server
	Authorizer       rpc.Authorizer
	ActivityRecorder sessionliveness.Recorder
}

func (s Service) ToolsList(ctx context.Context, params map[string]any, token string) map[string]any {
	repositoryID, _ := params["repository_id"].(string)
	authorizer := s.Authorizer
	if authorizer == nil && s.RPC != nil {
		authorizer = s.RPC.Authorizer
	}
	if authorizer == nil {
		authorizer = rpc.AllowAllAuthorizer{}
	}
	tools := VisibleTools(ctx, authorizer, token, repositoryID)
	if len(tools) > 0 {
		sessionID, _ := params["session_id"].(string)
		_ = s.recordActivity(ctx, repositoryID, sessionID, sessionliveness.LastToolsListAt)
	}
	return map[string]any{"tools": tools}
}

func (s Service) ToolsCall(ctx context.Context, name string, arguments map[string]any, token string, requestID string) map[string]any {
	if isHiddenProductionTool(name) {
		return toolResult(name, false, "", "tool_hidden", "MCP tools/call does not execute hidden production tools", nil)
	}
	if s.RPC == nil {
		return toolResult(name, false, "", "daemon_rpc_missing", "daemon RPC server is not configured", nil)
	}
	envelope := rpc.Envelope{
		SchemaVersion:   rpc.SupportedEnvelopeVersion,
		RequestID:       requestID,
		Method:          name,
		Params:          arguments,
		CapabilityToken: token,
	}
	// RFC 0101 Phase 1 (Layer 1, #83): stamp the tool-call boundary so the
	// liveness classifier can report working_tool with a visible since/deadline
	// for a lane that is blocked inside a hidden MCP/tool call (which otherwise
	// holds a live lease and reads as a contentless "live"). We record the START
	// before dispatching and the FINISH once it returns — only the timing of the
	// boundary is observed, never tool content (D028). repositoryID/sessionID
	// come from the call arguments; recordActivity is a no-op when either is
	// absent, so unscoped/anonymous tool calls are unaffected.
	repositoryID, _ := arguments["repository_id"].(string)
	sessionID, _ := arguments["session_id"].(string)
	_ = s.recordActivity(ctx, repositoryID, sessionID, sessionliveness.LastToolCallStartedAt)
	defer func() {
		_ = s.recordActivity(ctx, repositoryID, sessionID, sessionliveness.LastToolCallFinishedAt)
	}()
	response := s.RPC.HandleWithoutHandshake(ctx, envelope, "mcp")
	if response.OK {
		return toolResult(name, true, response.AuditID, "", "", response.Data)
	}
	code := "command_failed"
	message := ""
	if response.Data != nil {
		if value, ok := response.Data["code"].(string); ok {
			code = value
		}
		if value, ok := response.Data["message"].(string); ok {
			message = value
		}
	}
	return toolResult(name, false, response.AuditID, code, message, response.Data)
}

func (s Service) recordActivity(ctx context.Context, repositoryID string, sessionID string, columns ...string) error {
	if s.ActivityRecorder == nil || repositoryID == "" || sessionID == "" {
		return nil
	}
	return s.ActivityRecorder.RecordSessionActivity(ctx, repositoryID, sessionID, columns...)
}

func toolResult(name string, ok bool, auditID string, code string, message string, data map[string]any) map[string]any {
	var audit any
	if auditID != "" {
		audit = auditID
	}
	structured := map[string]any{
		"ok":       ok,
		"method":   name,
		"audit_id": audit,
	}
	if data != nil {
		structured["data"] = data
	}
	if code != "" {
		structured["error"] = code
	}
	if message != "" {
		structured["error_message"] = message
	}
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": fmt.Sprint(name)}},
		"structuredContent": structured,
		"isError":           !ok,
	}
}
