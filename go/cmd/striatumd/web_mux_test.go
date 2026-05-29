package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/mcp"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/webtest"
)

// TestDaemonHTTPHandlerRoutesMCPAndWeb asserts the single loopback listener
// dispatches /mcp to the MCP JSON-RPC handler and /v1/health to the Go web
// service, with no second port.
func TestDaemonHTTPHandlerRoutesMCPAndWeb(t *testing.T) {
	fixture := webtest.NewFixture()
	mcpHandler := mcp.NewHTTPHandler(mcp.Service{
		RPC:        fixture.Server,
		Authorizer: rpc.AllowAllAuthorizer{},
	})
	webHandler := newWebServiceHandler(fixture.Server, webServiceOptions{
		RepositoryID:   webtest.RepositoryID,
		AllowMutations: false,
		WebEnabled:     true,
	})
	mux := newDaemonHTTPHandler(mcpHandler, webHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	// /mcp routes to the MCP handler: tools/list returns a JSON-RPC result.
	body := `{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{"repository_id":"repo_test"}}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build /mcp request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	mcpBody := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/mcp status = %d body=%s", resp.StatusCode, mcpBody)
	}
	var mcpPayload struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	if err := json.Unmarshal([]byte(mcpBody), &mcpPayload); err != nil {
		t.Fatalf("decode /mcp body %q: %v", mcpBody, err)
	}
	if mcpPayload.Error != nil {
		t.Fatalf("/mcp tools/list returned error: %s", mcpBody)
	}
	if _, ok := mcpPayload.Result["tools"]; !ok {
		t.Fatalf("/mcp tools/list missing tools list: %s", mcpBody)
	}

	// /v1/health routes to the web service handler.
	healthReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1/health", nil)
	if err != nil {
		t.Fatalf("build /v1/health request: %v", err)
	}
	healthResp, err := server.Client().Do(healthReq)
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	healthBody := readAll(t, healthResp)
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("/v1/health status = %d body=%s", healthResp.StatusCode, healthBody)
	}
	if !strings.Contains(healthBody, `"mode":"go"`) {
		t.Fatalf("/v1/health missing web payload: %s", healthBody)
	}
}

// TestDaemonWebFailsClosedWithDenyToken asserts the RFC 0084 D1 build-review fix:
// when the runtime token is unreadable the daemon substitutes randomDenyToken, so
// the mounted /v1 surface rejects unauthenticated requests (fail closed) instead
// of authenticating open on an empty ServiceToken.
func TestDaemonWebFailsClosedWithDenyToken(t *testing.T) {
	fixture := webtest.NewFixture()
	mcpHandler := mcp.NewHTTPHandler(mcp.Service{RPC: fixture.Server, Authorizer: rpc.AllowAllAuthorizer{}})
	webHandler := newWebServiceHandler(fixture.Server, resolveWebServiceOptions(randomDenyToken()))
	server := httptest.NewServer(newDaemonHTTPHandler(mcpHandler, webHandler))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v1/health", nil)
	if err != nil {
		t.Fatalf("build /v1/health request: %v", err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("web must fail closed (401) with a deny token and no bearer; got %d: %s", resp.StatusCode, body)
	}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(data)
}
