package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

const (
	EndpointPath      = "/mcp"
	SSEEndpointPath   = "/mcp/sse"
	MessagePath       = "/mcp/messages"
	protocolVersion   = "2024-11-05"
	serverName        = "striatum-local"
	defaultAllowValue = "GET, POST, OPTIONS"

	jsonrpcVersion = "2.0"
)

const (
	jsonrpcParseError     = -32700
	jsonrpcInvalidRequest = -32600
	jsonrpcMethodNotFound = -32601
	jsonrpcInvalidParams  = -32602
	jsonrpcInternalError  = -32603
	jsonrpcAuthError      = -32001
	jsonrpcForbidden      = -32003
)

type HTTPHandler struct {
	Service       Service
	ServerName    string
	ServerVersion string

	mu       sync.Mutex
	sessions map[string]*sseSession
}

type sseSession struct {
	id       string
	messages chan []byte
}

type jsonrpcResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *jsonrpcError  `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func NewHTTPHandler(service Service) *HTTPHandler {
	return &HTTPHandler{
		Service:       service,
		ServerName:    serverName,
		ServerVersion: serverVersion(service),
		sessions:      map[string]*sseSession{},
	}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !isSupportedPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	if localErr := validateLocalRequest(r); localErr != nil {
		writeJSONResponseStatus(w, http.StatusForbidden, errorResponse(nil, jsonrpcForbidden, localErr.Message, errorData(localErr.Code, nil)))
		return
	}
	setLocalCORSHeaders(w, r)
	if isEndpointPath(r.URL.Path) && r.Method == http.MethodGet {
		h.serveStream(w, r)
		return
	}
	if (isEndpointPath(r.URL.Path) || r.URL.Path == MessagePath) && r.Method == http.MethodPost {
		h.servePost(w, r)
		return
	}
	if r.Method == http.MethodOptions {
		if isEndpointPath(r.URL.Path) {
			w.Header().Set("Allow", defaultAllowValue)
		} else {
			w.Header().Set("Allow", "POST, OPTIONS")
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if isEndpointPath(r.URL.Path) {
		w.Header().Set("Allow", defaultAllowValue)
	} else {
		w.Header().Set("Allow", "POST, OPTIONS")
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (h *HTTPHandler) serveStream(w http.ResponseWriter, r *http.Request) {
	if _, rpcErr := bearerToken(r); rpcErr != nil {
		writeJSONResponseStatus(w, http.StatusUnauthorized, jsonrpcResponse{JSONRPC: jsonrpcVersion, ID: nil, Error: rpcErr})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	session := h.newSession()
	defer h.closeSession(session.id)

	writeSSEHeaders(w)
	if err := writeSSEEvent(w, "endpoint", h.endpointURL(r, session.id)); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case payload := <-session.messages:
			if err := writeSSEEvent(w, "message", payload); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *HTTPHandler) servePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, rpc.MaxEnvelopeBytes))
	if err != nil {
		response := errorResponse(nil, jsonrpcInvalidRequest, err.Error(), errorData("malformed_body", nil))
		writeJSONResponse(w, response)
		return
	}
	response, respond := h.handleBody(r.Context(), r, body)
	if !respond {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		writeJSONResponse(w, errorResponse(nil, jsonrpcInternalError, err.Error(), nil))
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeJSONPayload(w, encoded)
		return
	}
	session := h.session(sessionID)
	if session == nil {
		http.Error(w, "unknown MCP SSE session", http.StatusNotFound)
		return
	}
	select {
	case session.messages <- encoded:
		w.WriteHeader(http.StatusAccepted)
	case <-r.Context().Done():
		http.Error(w, r.Context().Err().Error(), http.StatusRequestTimeout)
	}
}

func (h *HTTPHandler) handleBody(ctx context.Context, r *http.Request, body []byte) (jsonrpcResponse, bool) {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return errorResponse(nil, jsonrpcParseError, "invalid JSON: "+err.Error(), errorData("malformed_body", nil)), true
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errorResponse(nil, jsonrpcParseError, "request body must contain one JSON-RPC object", errorData("malformed_body", nil)), true
	}
	if payload == nil {
		return errorResponse(nil, jsonrpcInvalidRequest, "JSON-RPC request must be an object", errorData("schema_invalid", nil)), true
	}
	rawID, hasID := payload["id"]
	requestID, validID := jsonrpcID(rawID)
	if !validID {
		return errorResponse(nil, jsonrpcInvalidRequest, "id must be a string, number, or null", errorData("schema_invalid", nil)), true
	}
	if payload["jsonrpc"] != jsonrpcVersion {
		return errorResponse(requestID, jsonrpcInvalidRequest, "jsonrpc must be '2.0'", errorData("schema_invalid", nil)), true
	}
	method, ok := payload["method"].(string)
	if !ok || method == "" {
		return errorResponse(requestID, jsonrpcInvalidRequest, "method is required", errorData("schema_invalid", nil)), true
	}
	params, err := requestParams(payload)
	if err != nil {
		return errorResponse(requestID, jsonrpcInvalidParams, err.Error(), errorData("schema_invalid", nil)), true
	}

	result, rpcErr := h.dispatch(ctx, r, requestID, method, params)
	if !hasID {
		return jsonrpcResponse{}, false
	}
	if rpcErr != nil {
		return jsonrpcResponse{JSONRPC: jsonrpcVersion, ID: requestID, Error: rpcErr}, true
	}
	return jsonrpcResponse{JSONRPC: jsonrpcVersion, ID: requestID, Result: result}, true
}

func (h *HTTPHandler) dispatch(ctx context.Context, r *http.Request, requestID any, method string, params map[string]any) (map[string]any, *jsonrpcError) {
	switch method {
	case "initialize":
		return h.initializeResult(), nil
	case "notifications/initialized":
		return map[string]any{}, nil
	case "tools/list":
		token, rpcErr := bearerToken(r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if _, err := optionalString(params, "repository_id"); err != nil {
			return nil, invalidParams(err)
		}
		return h.Service.ToolsList(ctx, params, token), nil
	case "tools/call":
		name, err := requiredString(params, "name")
		if err != nil {
			return nil, invalidParams(err)
		}
		arguments, err := objectParam(params, "arguments")
		if err != nil {
			return nil, invalidParams(err)
		}
		repositoryID, err := optionalString(params, "repository_id")
		if err != nil {
			return nil, invalidParams(err)
		}
		if repositoryID != "" {
			if _, exists := arguments["repository_id"]; !exists {
				arguments["repository_id"] = repositoryID
			}
		}
		token, rpcErr := bearerToken(r)
		if rpcErr != nil {
			return nil, rpcErr
		}
		callRequestID, err := callRequestID(params, requestID)
		if err != nil {
			return nil, invalidParams(err)
		}
		return h.Service.ToolsCall(ctx, name, arguments, token, callRequestID), nil
	default:
		return nil, &jsonrpcError{Code: jsonrpcMethodNotFound, Message: fmt.Sprintf("unknown method %q", method), Data: errorData("method_unknown", nil)}
	}
}

func (h *HTTPHandler) initializeResult() map[string]any {
	name := h.ServerName
	if name == "" {
		name = serverName
	}
	version := h.ServerVersion
	if version == "" {
		version = serverVersion(h.Service)
	}
	return map[string]any{
		"protocolVersion": protocolVersion,
		"serverInfo": map[string]any{
			"name":    name,
			"version": version,
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	}
}

func (h *HTTPHandler) newSession() *sseSession {
	session := &sseSession{
		id:       newSessionID(),
		messages: make(chan []byte, 8),
	}
	h.mu.Lock()
	h.sessions[session.id] = session
	h.mu.Unlock()
	return session
}

func (h *HTTPHandler) closeSession(id string) {
	h.mu.Lock()
	delete(h.sessions, id)
	h.mu.Unlock()
}

func (h *HTTPHandler) session(id string) *sseSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[id]
}

func (h *HTTPHandler) endpointURL(r *http.Request, sessionID string) string {
	return MessagePath + "?session_id=" + sessionID
}

func requestParams(payload map[string]any) (map[string]any, error) {
	value, exists := payload["params"]
	if !exists || value == nil {
		return map[string]any{}, nil
	}
	params, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("params must be an object")
	}
	return cloneMap(params), nil
}

func requiredString(params map[string]any, key string) (string, error) {
	value, exists := params[key]
	if !exists || value == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return text, nil
}

func optionalString(params map[string]any, key string) (string, error) {
	value, exists := params[key]
	if !exists || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

func objectParam(params map[string]any, key string) (map[string]any, error) {
	value, exists := params[key]
	if !exists || value == nil {
		return map[string]any{}, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return cloneMap(object), nil
}

func bearerToken(r *http.Request) (string, *jsonrpcError) {
	header := r.Header.Get("Authorization")
	if strings.TrimSpace(header) == "" {
		return "", &jsonrpcError{Code: jsonrpcAuthError, Message: "Authorization bearer token is required", Data: errorData("token_missing", nil)}
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", &jsonrpcError{Code: jsonrpcAuthError, Message: "Authorization must use Bearer token syntax", Data: errorData("token_malformed", nil)}
	}
	return parts[1], nil
}

func callRequestID(params map[string]any, jsonrpcID any) (string, error) {
	requestID, err := optionalString(params, "request_id")
	if err != nil {
		return "", err
	}
	if requestID != "" {
		return requestID, nil
	}
	switch value := jsonrpcID.(type) {
	case string:
		if value != "" {
			return "mcp_" + newSessionID() + "_" + value, nil
		}
	case json.Number:
		if value.String() != "" {
			return "mcp_" + newSessionID() + "_" + value.String(), nil
		}
	case nil:
	}
	return "mcp_" + newSessionID(), nil
}

func cloneMap(source map[string]any) map[string]any {
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func jsonrpcID(value any) (any, bool) {
	if value == nil {
		return nil, true
	}
	switch v := value.(type) {
	case string, json.Number:
		return v, true
	default:
		return nil, false
	}
}

func invalidParams(err error) *jsonrpcError {
	return &jsonrpcError{Code: jsonrpcInvalidParams, Message: err.Error(), Data: errorData("schema_invalid", nil)}
}

func errorResponse(id any, code int, message string, data any) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error:   &jsonrpcError{Code: code, Message: message, Data: data},
	}
}

func writeJSONResponse(w http.ResponseWriter, response jsonrpcResponse) {
	writeJSONResponseStatus(w, http.StatusOK, response)
}

func writeJSONResponseStatus(w http.ResponseWriter, status int, response jsonrpcResponse) {
	encoded, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONPayloadStatus(w, status, encoded)
}

func writeJSONPayload(w http.ResponseWriter, payload []byte) {
	writeJSONPayloadStatus(w, http.StatusOK, payload)
}

func writeJSONPayloadStatus(w http.ResponseWriter, status int, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func writeSSEHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
}

func writeSSEEvent(w io.Writer, event string, data any) error {
	var payload []byte
	switch value := data.(type) {
	case []byte:
		payload = value
	case string:
		payload = []byte(value)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		payload = encoded
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	for _, line := range strings.Split(string(payload), "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func newSessionID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func serverVersion(service Service) string {
	if service.RPC == nil || service.RPC.DaemonVersion == "" {
		return "0.1.0"
	}
	return service.RPC.DaemonVersion
}

type localRequestError struct {
	Code    string
	Message string
}

func isSupportedPath(path string) bool {
	return isEndpointPath(path) || path == MessagePath
}

func isEndpointPath(path string) bool {
	return path == EndpointPath || path == SSEEndpointPath
}

func validateLocalRequest(r *http.Request) *localRequestError {
	if !isLoopbackHost(r.Host) {
		return &localRequestError{Code: "bad_host", Message: "Host must be loopback"}
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" && !isLoopbackOrigin(origin) {
		return &localRequestError{Code: "bad_origin", Message: "Origin must be loopback"}
	}
	return nil
}

func setLocalCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", origin)
	header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
	header.Set("Access-Control-Allow-Methods", defaultAllowValue)
	header.Add("Vary", "Origin")
}

func isLoopbackOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return isLoopbackHost(parsed.Host)
}

func isLoopbackHost(value string) bool {
	host := normalizeHost(value)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizeHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return strings.Trim(value, "[]")
	}
	return value
}

func errorData(code string, details any) map[string]any {
	data := map[string]any{"code": code}
	if details != nil {
		data["details"] = details
	}
	return data
}
