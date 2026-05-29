package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

func TestHTTPHandlerInitializeDirectPost(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	recorder := postJSON(t, handler, EndpointPath, `{"jsonrpc":"2.0","id":"init","method":"initialize","params":{}}`, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	response := decodeTestResponse(t, recorder)
	if response.ID != "init" || response.Error != nil {
		t.Fatalf("initialize response = %#v", response)
	}
	if response.Result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion = %#v", response.Result["protocolVersion"])
	}
}

func TestHTTPHandlerToolsListUsesBearerTokenAndHidesUnauthorized(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	recorder := postJSON(t, handler, EndpointPath, `{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{"repository_id":"repo_1"}}`, "read.secret")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	response := decodeTestResponse(t, recorder)
	if response.Error != nil {
		t.Fatalf("tools/list error = %#v", response.Error)
	}
	tools, ok := response.Result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools/list returned no tools: %#v", response.Result)
	}
	names := toolNames(tools)
	if !names["status"] {
		t.Fatalf("read token did not see status tool: %#v", names)
	}
	if !names["git.snapshot"] {
		t.Fatalf("read token did not see git.snapshot tool: %#v", names)
	}
	for _, hidden := range []string{"work.complete", "workflow.generate"} {
		if names[hidden] {
			t.Fatalf("read token saw unauthorized/hidden tool %s: %#v", hidden, names)
		}
	}
}

func TestHTTPHandlerToolsListRecordsSessionActivity(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	recorder := &activityRecorder{}
	handler.Service.ActivityRecorder = recorder
	body := `{"jsonrpc":"2.0","id":"list-activity","method":"tools/list","params":{"repository_id":"repo_1","session_id":"sess_1"}}`

	responseRecorder := postJSON(t, handler, EndpointPath, body, "read.secret")
	response := decodeTestResponse(t, responseRecorder)
	if response.Error != nil {
		t.Fatalf("tools/list error = %#v", response.Error)
	}
	if recorder.repositoryID != "repo_1" || recorder.sessionID != "sess_1" {
		t.Fatalf("activity scope = repo %q session %q", recorder.repositoryID, recorder.sessionID)
	}
	if len(recorder.columns) != 1 || recorder.columns[0] != sessionliveness.LastToolsListAt {
		t.Fatalf("activity columns = %#v", recorder.columns)
	}
}

func TestHTTPHandlerToolsCallReadPath(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	body := `{"jsonrpc":"2.0","id":"read","method":"tools/call","params":{"name":"status","arguments":{"repository_id":"repo_1"}}}`
	recorder := postJSON(t, handler, EndpointPath, body, "read.secret")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	response := decodeTestResponse(t, recorder)
	structured := structuredContent(t, response)
	if structured["ok"] != true || structured["method"] != "status" || response.Result["isError"] != false {
		t.Fatalf("read tool call result = %#v", response.Result)
	}
	data, ok := structured["data"].(map[string]any)
	if !ok || data["status"] != "ok" {
		t.Fatalf("read tool data = %#v", structured["data"])
	}
}

type activityRecorder struct {
	repositoryID string
	sessionID    string
	columns      []string
}

func (r *activityRecorder) RecordSessionActivity(_ context.Context, repositoryID string, sessionID string, columns ...string) error {
	r.repositoryID = repositoryID
	r.sessionID = sessionID
	r.columns = append([]string(nil), columns...)
	return nil
}

func TestHTTPHandlerToolsCallMutationPath(t *testing.T) {
	handler, mutationCalled, _, _ := newTestHTTPHandler(t)
	body := `{"jsonrpc":"2.0","id":"write","method":"tools/call","params":{"name":"work.complete","repository_id":"repo_1","arguments":{"session_id":"sess_1","job_id":"job_1","lease_id":"lease_1"}}}`
	recorder := postJSON(t, handler, EndpointPath, body, "write.secret")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !*mutationCalled {
		t.Fatal("work.complete handler was not called")
	}
	response := decodeTestResponse(t, recorder)
	structured := structuredContent(t, response)
	if structured["ok"] != true || structured["method"] != "work.complete" || response.Result["isError"] != false {
		t.Fatalf("mutation tool call result = %#v", response.Result)
	}
	data, ok := structured["data"].(map[string]any)
	if !ok || data["mutated"] != true {
		t.Fatalf("mutation tool data = %#v", structured["data"])
	}
}

func TestHTTPHandlerToolsCallInvalidTokenDenies(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	body := `{"jsonrpc":"2.0","id":"invalid-token","method":"tools/call","params":{"name":"status","arguments":{"repository_id":"repo_1"}}}`
	recorder := postJSON(t, handler, EndpointPath, body, "missing.secret")

	response := decodeTestResponse(t, recorder)
	structured := structuredContent(t, response)
	if response.Result["isError"] != true || structured["error"] != "token_invalid" {
		t.Fatalf("invalid token result = %#v", response.Result)
	}
}

func TestHTTPHandlerWriteTokenCannotCallReadOnlyGitSnapshot(t *testing.T) {
	handler, _, _, gitSnapshotCalled := newTestHTTPHandler(t)
	body := `{"jsonrpc":"2.0","id":"git-denied","method":"tools/call","params":{"name":"git.snapshot","repository_id":"repo_1","arguments":{"repository_id":"repo_1"}}}`
	recorder := postJSON(t, handler, EndpointPath, body, "write.secret")

	response := decodeTestResponse(t, recorder)
	structured := structuredContent(t, response)
	if response.Result["isError"] != true || structured["error"] != "capability_missing" {
		t.Fatalf("git.snapshot denial result = %#v", response.Result)
	}
	if *gitSnapshotCalled {
		t.Fatal("git.snapshot handler ran for write-only token")
	}
}

func TestHTTPHandlerDeniedTokenCannotCallHiddenUnauthorizedMethod(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	body := `{"jsonrpc":"2.0","id":"hidden-denied","method":"tools/call","params":{"name":"workflow.generate","repository_id":"repo_1","arguments":{}}}`
	recorder := postJSON(t, handler, EndpointPath, body, "read.secret")

	response := decodeTestResponse(t, recorder)
	structured := structuredContent(t, response)
	if response.Result["isError"] != true || structured["error"] != "tool_hidden" {
		t.Fatalf("hidden unauthorized method result = %#v", response.Result)
	}
}

func TestHTTPHandlerWriteTokenCannotCallHiddenProductionTool(t *testing.T) {
	handler, _, hiddenCalled, _ := newTestHTTPHandler(t)
	body := `{"jsonrpc":"2.0","id":"hidden-write-denied","method":"tools/call","params":{"name":"workflow.generate","repository_id":"repo_1","arguments":{}}}`
	recorder := postJSON(t, handler, EndpointPath, body, "write.secret")

	response := decodeTestResponse(t, recorder)
	structured := structuredContent(t, response)
	if response.Result["isError"] != true || structured["error"] != "tool_hidden" {
		t.Fatalf("hidden write-capable method result = %#v", response.Result)
	}
	if *hiddenCalled {
		t.Fatal("workflow.generate handler ran despite hidden production MCP denial")
	}
}

func TestHTTPHandlerToolsCallUnknownDaemonMethodReturnsMCPError(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	body := `{"jsonrpc":"2.0","id":"unknown-daemon","method":"tools/call","params":{"name":"not.a.method","arguments":{"repository_id":"repo_1"}}}`
	recorder := postJSON(t, handler, EndpointPath, body, "read.secret")

	response := decodeTestResponse(t, recorder)
	structured := structuredContent(t, response)
	if response.Result["isError"] != true || structured["error"] != "method_unknown" {
		t.Fatalf("unknown daemon method result = %#v", response.Result)
	}
}

func TestHTTPHandlerMissingAuthReturnsJSONRPCError(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	recorder := postJSON(t, handler, EndpointPath, `{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{"repository_id":"repo_1"}}`, "")

	response := decodeTestResponse(t, recorder)
	if response.Error == nil || response.Error.Code != jsonrpcAuthError {
		t.Fatalf("missing auth error = %#v", response.Error)
	}
	assertErrorDataCode(t, response.Error, "token_missing")
}

func TestHTTPHandlerRejectsBadOrigin(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	request := newJSONRequest(t, EndpointPath, `{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{"repository_id":"repo_1"}}`, "read.secret")
	request.Header.Set("Origin", "https://example.invalid")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	response := decodeTestResponse(t, recorder)
	assertErrorDataCode(t, response.Error, "bad_origin")
}

func TestHTTPHandlerRejectsBadHost(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	request := newJSONRequest(t, EndpointPath, `{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{"repository_id":"repo_1"}}`, "read.secret")
	request.Host = "example.invalid"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	response := decodeTestResponse(t, recorder)
	assertErrorDataCode(t, response.Error, "bad_host")
}

func TestHTTPHandlerMalformedBodyReturnsStableError(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	recorder := postJSON(t, handler, EndpointPath, `{`, "read.secret")

	response := decodeTestResponse(t, recorder)
	if response.Error == nil || response.Error.Code != jsonrpcParseError {
		t.Fatalf("malformed body error = %#v", response.Error)
	}
	assertErrorDataCode(t, response.Error, "malformed_body")
}

func TestHTTPHandlerUnknownJSONRPCMethodReturnsStableError(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	recorder := postJSON(t, handler, EndpointPath, `{"jsonrpc":"2.0","id":"unknown","method":"resources/list","params":{}}`, "")

	response := decodeTestResponse(t, recorder)
	if response.Error == nil || response.Error.Code != jsonrpcMethodNotFound {
		t.Fatalf("unknown method error = %#v", response.Error)
	}
	assertErrorDataCode(t, response.Error, "method_unknown")
}

func TestHTTPHandlerStreamsMessageResponsesFromSSEAlias(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	streamRequest, err := http.NewRequest(http.MethodGet, server.URL+SSEEndpointPath+"?token=secret", nil)
	if err != nil {
		t.Fatalf("new stream request: %v", err)
	}
	streamRequest.Header.Set("Authorization", "Bearer read.secret")
	stream, err := server.Client().Do(streamRequest)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer func() { _ = stream.Body.Close() }()
	reader := bufio.NewReader(stream.Body)

	event, data := readTestSSEEvent(t, reader)
	if event != "endpoint" {
		t.Fatalf("first event = %q, want endpoint", event)
	}
	if !strings.HasPrefix(data, MessagePath+"?session_id=") {
		t.Fatalf("endpoint data = %q, want %s session endpoint", data, MessagePath)
	}
	if strings.Contains(data, "secret") {
		t.Fatalf("endpoint data leaked query token: %q", data)
	}

	payload := `{"jsonrpc":"2.0","id":"stream-list","method":"tools/list","params":{"repository_id":"repo_1"}}`
	postRequest, err := http.NewRequest(http.MethodPost, server.URL+data, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("new post request: %v", err)
	}
	postRequest.Header.Set("Authorization", "Bearer read.secret")
	postRequest.Header.Set("Content-Type", "application/json")
	postResponse, err := server.Client().Do(postRequest)
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	_ = postResponse.Body.Close()
	if postResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("post status = %d, want 202", postResponse.StatusCode)
	}

	event, data = readTestSSEEvent(t, reader)
	if event != "message" {
		t.Fatalf("response event = %q, want message", event)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		t.Fatalf("decode streamed response: %v", err)
	}
	if response["id"] != "stream-list" {
		t.Fatalf("streamed response id = %#v", response["id"])
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("streamed response result missing: %#v", response)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("streamed tools/list returned no tools: %#v", result)
	}
}

func newTestHTTPHandler(t *testing.T) (*HTTPHandler, *bool, *bool, *bool) {
	t.Helper()
	authorizer := rpc.NewMemoryAuthorizer()
	authorizer.AddToken("read.secret", "reader", map[rpc.Capability]rpc.CapabilityGrant{
		rpc.CapabilityRead: {},
	}, time.Now().Add(time.Hour))
	authorizer.AddToken("write.secret", "writer", map[rpc.Capability]rpc.CapabilityGrant{
		rpc.CapabilityWrite: {RepositoryID: "repo_1"},
	}, time.Now().Add(time.Hour))
	authorizer.AddToken("admin.secret", "admin", map[rpc.Capability]rpc.CapabilityGrant{
		rpc.CapabilityAdmin: {RepositoryID: "repo_1"},
	}, time.Now().Add(time.Hour))

	server := rpc.NewServer()
	server.Authorizer = authorizer
	server.Register("status", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		return map[string]any{
			"status":        "ok",
			"repository_id": envelope.Params["repository_id"],
		}, nil
	})
	mutationCalled := false
	server.Register("work.complete", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		mutationCalled = true
		return map[string]any{
			"mutated":       true,
			"repository_id": envelope.Params["repository_id"],
		}, nil
	})
	gitSnapshotCalled := false
	server.Register("git.snapshot", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		gitSnapshotCalled = true
		return map[string]any{
			"schema_version": "striatum.git_snapshot.v1",
			"repository_id":  envelope.Params["repository_id"],
		}, nil
	})
	hiddenCalled := false
	server.Register("workflow.generate", func(context.Context, rpc.Envelope) (map[string]any, error) {
		hiddenCalled = true
		return map[string]any{"hidden": true}, nil
	})
	handler := NewHTTPHandler(Service{RPC: server, Authorizer: authorizer})
	return handler, &mutationCalled, &hiddenCalled, &gitSnapshotCalled
}

func TestHTTPHandlerToolsCallAcceptedRiskRequiresAdmin(t *testing.T) {
	handler, _, _, _ := newTestHTTPHandler(t)
	called := false
	handler.Service.RPC.Register("workflow.accept_risk", func(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
		called = true
		return map[string]any{"accepted": true, "repository_id": envelope.Params["repository_id"]}, nil
	})
	body := `{"jsonrpc":"2.0","id":"risk-denied","method":"tools/call","params":{"name":"workflow.accept_risk","repository_id":"repo_1","arguments":{"repository_id":"repo_1"}}}`

	response := decodeTestResponse(t, postJSON(t, handler, EndpointPath, body, "read.secret"))
	structured := structuredContent(t, response)
	if structured["ok"] != false || structured["error"] != "capability_missing" {
		t.Fatalf("read token accepted-risk denial = %#v", structured)
	}
	if called {
		t.Fatal("workflow.accept_risk handler ran for read-only token")
	}

	body = `{"jsonrpc":"2.0","id":"risk-admin","method":"tools/call","params":{"name":"workflow.accept_risk","repository_id":"repo_1","arguments":{"repository_id":"repo_1"}}}`
	response = decodeTestResponse(t, postJSON(t, handler, EndpointPath, body, "admin.secret"))
	structured = structuredContent(t, response)
	if structured["ok"] != true || !called {
		t.Fatalf("admin accepted-risk call = %#v called=%v", structured, called)
	}
}

func postJSON(t *testing.T, handler http.Handler, path string, body string, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := newJSONRequest(t, path, body, token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func newJSONRequest(t *testing.T, path string, body string, token string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Host = "127.0.0.1:8765"
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func decodeTestResponse(t *testing.T, recorder *httptest.ResponseRecorder) jsonrpcResponse {
	t.Helper()
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json; body = %s", contentType, recorder.Body.String())
	}
	var response jsonrpcResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, recorder.Body.String())
	}
	return response
}

func structuredContent(t *testing.T, response jsonrpcResponse) map[string]any {
	t.Helper()
	if response.Error != nil {
		t.Fatalf("unexpected JSON-RPC error = %#v", response.Error)
	}
	structured, ok := response.Result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent missing: %#v", response.Result)
	}
	return structured
}

func assertErrorDataCode(t *testing.T, rpcErr *jsonrpcError, code string) {
	t.Helper()
	if rpcErr == nil {
		t.Fatalf("missing JSON-RPC error, want data code %s", code)
	}
	data, ok := rpcErr.Data.(map[string]any)
	if !ok || data["code"] != code {
		t.Fatalf("error data = %#v, want code %s", rpcErr.Data, code)
	}
}

func toolNames(tools []any) map[string]bool {
	names := map[string]bool{}
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tool["name"].(string)
		if name != "" {
			names[name] = true
		}
	}
	return names
}

func readTestSSEEvent(t *testing.T, reader *bufio.Reader) (string, string) {
	t.Helper()
	event := "message"
	var data []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE event: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(data) > 0 {
				return event, strings.Join(data, "\n")
			}
			event = "message"
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event = value
		case "data":
			data = append(data, value)
		}
	}
}
