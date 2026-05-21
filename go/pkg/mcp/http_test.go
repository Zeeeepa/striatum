package mcp

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPHandlerDirectPostReturnsJSON(t *testing.T) {
	handler := NewHTTPHandler(Service{Authorizer: allowAllAuthorizer{}})
	body := `{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{"repository_id":"repo_1"}}`
	request := httptest.NewRequest(http.MethodPost, EndpointPath, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer tok")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["id"] != "list" {
		t.Fatalf("response id = %#v", response["id"])
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("response result missing: %#v", response)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools/list returned no tools: %#v", result)
	}
}

func TestHTTPHandlerStreamsMessageResponses(t *testing.T) {
	handler := NewHTTPHandler(Service{Authorizer: allowAllAuthorizer{}})
	server := httptest.NewServer(handler)
	defer server.Close()

	streamRequest, err := http.NewRequest(http.MethodGet, server.URL+EndpointPath+"?token=secret", nil)
	if err != nil {
		t.Fatalf("new stream request: %v", err)
	}
	stream, err := server.Client().Do(streamRequest)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer stream.Body.Close()
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
	postRequest.Header.Set("Authorization", "Bearer tok")
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
		if field == "event" {
			event = value
		} else if field == "data" {
			data = append(data, value)
		}
	}
}
