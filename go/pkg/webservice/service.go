package webservice

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/webassets"
	"github.com/halbritt/striatum/go/pkg/websse"
)

const MaxBodyBytes = 8 * 1024 * 1024

type Config struct {
	RPC             *rpc.Server
	RepositoryID    string
	CapabilityToken string
	ServiceToken    string
	AllowMutations  bool
	WebEnabled      bool
	StartedAt       time.Time
	SSEPollInterval time.Duration

	// RFC 0085 tailnet-identity UI auth (web-ui.sock). When IdentityAuth is
	// true the handler authenticates via the Tailscale-User-Login header
	// instead of a bearer token, and permits ONLY the read-route allowlist
	// (IdentityReadRoutes); every other route — including all mutating ones —
	// is denied 403. The main loopback listener leaves IdentityAuth false and
	// never trusts identity headers.
	IdentityAuth bool
	// IdentityAllowlist is the set of permitted Tailscale logins. An empty set
	// while IdentityAuth is true denies every identity request (fail closed).
	IdentityAllowlist map[string]struct{}
	// AllowedHost is the node's MagicDNS name that `tailscale serve` forwards;
	// accepted on the identity listener in addition to loopback.
	AllowedHost string
}

type Handler struct {
	config Config
	sse    sseSlots
}

func New(config Config) *Handler {
	if config.StartedAt.IsZero() {
		config.StartedAt = time.Now().UTC()
	}
	return &Handler{config: config}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setBaseHeaders(w)
	if h.config.IdentityAuth {
		h.serveIdentity(w, r)
		return
	}
	if !isLoopbackHost(r.Host) {
		writeJSON(w, http.StatusForbidden, errorPayload("host_refused", "Host header must be loopback"))
		return
	}
	if !h.authenticate(r) {
		writeJSON(w, http.StatusUnauthorized, errorPayload("token_missing", "missing or invalid Authorization bearer token"))
		return
	}
	if r.Method == http.MethodPost && !sameOrigin(r) && r.Header.Get("Authorization") == "" {
		writeJSON(w, http.StatusForbidden, errorPayload("cross_origin_refused", "cross-origin mutation refused"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.routeGET(w, r)
	case http.MethodPost:
		h.routePOST(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, errorPayload("method_not_allowed", "method not allowed"))
	}
}

func (h *Handler) routeGET(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean(r.URL.Path)
	switch {
	case clean == "/v1/health":
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]any{
			"started_at":      h.config.StartedAt.Format(time.RFC3339),
			"mode":            "go",
			"allow_mutations": h.config.AllowMutations,
		}})
	case clean == "/v1/runs":
		h.callAndWrite(w, r.Context(), "status", map[string]any{})
	case strings.HasPrefix(clean, "/v1/runs/"):
		h.routeRunGET(w, r, strings.TrimPrefix(clean, "/v1/runs/"))
	case strings.HasPrefix(clean, "/v1/artifacts/") && strings.HasSuffix(clean, "/raw"):
		artifactID := strings.TrimSuffix(strings.TrimPrefix(clean, "/v1/artifacts/"), "/raw")
		h.serveArtifactRaw(w, r.Context(), artifactID)
	case strings.HasPrefix(clean, "/v1/sessions/") && strings.HasSuffix(clean, "/live-pty"):
		sessionID := strings.TrimSuffix(strings.TrimPrefix(clean, "/v1/sessions/"), "/live-pty")
		h.streamLivePTY(w, r, sessionID)
	case clean == "/workflow-templates":
		params := map[string]any{}
		if kind := r.URL.Query().Get("kind"); kind != "" {
			params["kind"] = kind
		}
		h.callAndWrite(w, r.Context(), "workflow.templates.list", params)
	case strings.HasPrefix(clean, "/workflow-templates/"):
		h.callAndWrite(w, r.Context(), "workflow.templates.show", map[string]any{"template_id": strings.TrimPrefix(clean, "/workflow-templates/")})
	case h.config.WebEnabled && (clean == "/" || clean == "/run"):
		params := map[string]any{}
		if runID := r.URL.Query().Get("run_id"); runID != "" {
			params["run_id"] = runID
		} else if runID := r.URL.Query().Get("id"); runID != "" {
			params["run_id"] = runID
		}
		h.renderRPCPage(w, r.Context(), "Striatum Runs", "status", params)
	case h.config.WebEnabled && strings.HasPrefix(clean, "/static/"):
		h.serveStatic(w, strings.TrimPrefix(clean, "/static/"))
	case isRetiredRoute(clean):
		writeJSON(w, http.StatusGone, errorPayload("route_retired", "route retired for Go-only web service cutover"))
	default:
		writeJSON(w, http.StatusNotFound, errorPayload("not_found", "not found"))
	}
}

func (h *Handler) routeRunGET(w http.ResponseWriter, r *http.Request, suffix string) {
	parts := strings.Split(suffix, "/")
	runID := parts[0]
	if runID == "" {
		writeJSON(w, http.StatusBadRequest, errorPayload("schema_invalid", "missing run_id"))
		return
	}
	if len(parts) == 1 {
		h.callAndWrite(w, r.Context(), "status", map[string]any{"run_id": runID})
		return
	}
	switch parts[1] {
	case "why":
		target := r.URL.Query().Get("id")
		if target == "" {
			writeJSON(w, http.StatusBadRequest, errorPayload("schema_invalid", "missing ?id=<entity>"))
			return
		}
		h.callAndWrite(w, r.Context(), "why", map[string]any{"target_id": target})
	case "dashboard":
		h.callAndWrite(w, r.Context(), "dashboard", map[string]any{"run_id": runID})
	case "artifacts":
		h.callAndWrite(w, r.Context(), "list.artifacts", map[string]any{"run_id": runID})
	case "interrogations":
		// D142 / RFC 0084 build review: curated interrogation bodies must not be cached.
		w.Header().Set("Cache-Control", "no-store")
		if len(parts) >= 3 && parts[2] != "" {
			// /v1/runs/{runID}/interrogations/{id} — verify run ownership.
			h.showInterrogation(w, r, runID, parts[2])
			return
		}
		// /v1/runs/{runID}/interrogations — list for this run.
		h.callAndWrite(w, r.Context(), "interrogation.list", map[string]any{"run_id": runID})
	case "conversations":
		w.Header().Set("Cache-Control", "no-store")
		if len(parts) >= 3 && parts[2] != "" {
			h.showConversation(w, r, runID, parts[2])
			return
		}
		h.callAndWrite(w, r.Context(), "conversation.list", map[string]any{"run_id": runID})
	case "events":
		h.streamRunEvents(w, r, runID)
	case "live-dialogue":
		h.streamLiveDialogue(w, r, runID)
	default:
		writeJSON(w, http.StatusNotFound, errorPayload("not_found", "not found"))
	}
}

func (h *Handler) routePOST(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean(r.URL.Path)
	switch clean {
	case "/v1/invoke":
		h.handleInvoke(w, r)
	case "/workflows/generate/preview":
		h.handleWorkflowGenerate(w, r, false)
	case "/workflows/generate":
		h.handleWorkflowGenerate(w, r, true)
	default:
		if isRetiredRoute(clean) {
			writeJSON(w, http.StatusGone, errorPayload("route_retired", "route retired for Go-only web service cutover"))
			return
		}
		writeJSON(w, http.StatusNotFound, errorPayload("not_found", "not found"))
	}
}

func (h *Handler) handleInvoke(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if !decodeJSONBody(w, r, &body) {
		return
	}
	method, _ := body["method"].(string)
	params, _ := body["params"].(map[string]any)
	if method == "" {
		method, params = methodForArgv(body["argv"])
	}
	if method == "" {
		writeJSON(w, http.StatusBadRequest, errorPayload("schema_invalid", "method or supported argv is required"))
		return
	}
	if !h.config.AllowMutations && isMutationMethod(method) {
		writeJSON(w, http.StatusMethodNotAllowed, errorPayload("mutations_disabled", "command requires allow_mutations"))
		return
	}
	h.callAndWrite(w, r.Context(), method, params)
}

func (h *Handler) handleWorkflowGenerate(w http.ResponseWriter, r *http.Request, write bool) {
	if write && !h.config.AllowMutations {
		writeJSON(w, http.StatusMethodNotAllowed, errorPayload("mutations_disabled", "workflow generation requires allow_mutations"))
		return
	}
	var body map[string]any
	if !decodeJSONBody(w, r, &body) {
		return
	}
	method := "workflow.generate.preview"
	if write {
		method = "workflow.generate"
	}
	h.callAndWrite(w, r.Context(), method, map[string]any{"spec": body["spec"]})
}

func (h *Handler) callAndWrite(w http.ResponseWriter, ctx context.Context, method string, params map[string]any) {
	data, status, code, message := h.call(ctx, method, params)
	if code != "" {
		writeJSON(w, status, errorPayload(code, message))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

func (h *Handler) call(ctx context.Context, method string, params map[string]any) (map[string]any, int, string, string) {
	if h.config.RPC == nil {
		return nil, http.StatusServiceUnavailable, "daemon_rpc_missing", "daemon RPC server is not configured"
	}
	if params == nil {
		params = map[string]any{}
	}
	if _, ok := params["repository_id"]; !ok && h.config.RepositoryID != "" {
		params["repository_id"] = h.config.RepositoryID
	}
	envelope := rpc.Envelope{
		SchemaVersion:   rpc.SupportedEnvelopeVersion,
		RequestID:       fmt.Sprintf("web_%d", time.Now().UnixNano()),
		Method:          method,
		Params:          params,
		CapabilityToken: h.config.CapabilityToken,
	}
	response := h.config.RPC.HandleWithoutHandshake(ctx, envelope, "web")
	if response.OK {
		return response.Data, http.StatusOK, "", ""
	}
	code, _ := response.Data["code"].(string)
	message, _ := response.Data["message"].(string)
	if code == "" {
		code = "rpc_error"
	}
	if message == "" {
		message = "daemon RPC call failed"
	}
	return nil, statusForRPCCode(code), code, message
}

func (h *Handler) streamRunEvents(w http.ResponseWriter, r *http.Request, runID string) {
	if !h.sse.acquire(runID) {
		writeJSON(w, http.StatusTooManyRequests, errorPayload("sse_limit_exceeded", "too many concurrent SSE streams for run"))
		return
	}
	defer h.sse.release(runID)
	source := rpcEventSource{handler: h, runID: runID}
	websse.Stream(r.Context(), w, source, websse.Since(r), websse.Options{PollInterval: h.config.SSEPollInterval, Limit: 100})
}

func (h *Handler) serveArtifactRaw(w http.ResponseWriter, ctx context.Context, artifactID string) {
	data, status, code, message := h.call(ctx, "artifact.get_content", map[string]any{"artifact_id": artifactID})
	if code != "" {
		writeJSON(w, status, errorPayload(code, message))
		return
	}
	body64, _ := data["body_base64"].(string)
	body, err := base64.StdEncoding.DecodeString(body64)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorPayload("artifact_error", "artifact body_base64 is malformed"))
		return
	}
	contentType, _ := data["content_type"].(string)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) serveStatic(w http.ResponseWriter, relative string) {
	asset, err := webassets.LoadStatic(relative)
	if err != nil {
		status := http.StatusNotFound
		if errors.Is(err, http.ErrAbortHandler) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, errorPayload("not_found", "asset not found"))
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Content-Length", fmt.Sprint(len(asset.Body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(asset.Body)
}

func (h *Handler) renderRPCPage(w http.ResponseWriter, ctx context.Context, title string, method string, params map[string]any) {
	data, status, code, message := h.call(ctx, method, params)
	if code != "" {
		writeJSON(w, status, errorPayload(code, message))
		return
	}
	body, err := webassets.RenderPage(title, data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorPayload("render_failed", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// showInterrogation serves a single interrogation thread for a run.
//
// interrogation.show is keyed by interrogation_id, not run_id, so this route
// enforces run ownership: it asserts the returned interrogation.run_id equals
// the path runID and returns 404 on mismatch (or unknown id), before writing
// any turn bytes. This closes the cross-run IDOR a guessed/leaked
// interrogation_id under the wrong run path could otherwise open. Repository
// scope is already enforced by callAndWrite/call injecting repository_id.
//
// Default response is the curated JSON read. With ?view=chat the same curated
// data is rendered server-side as an html/template chat thread.
func (h *Handler) showInterrogation(w http.ResponseWriter, r *http.Request, runID, interrogationID string) {
	data, status, code, message := h.call(r.Context(), "interrogation.show", map[string]any{"interrogation_id": interrogationID})
	if code != "" {
		writeJSON(w, status, errorPayload(code, message))
		return
	}
	interrogation, _ := data["interrogation"].(map[string]any)
	if interrogation == nil || displayString(interrogation["run_id"]) != runID {
		writeJSON(w, http.StatusNotFound, errorPayload("not_found", "not found"))
		return
	}
	if r.URL.Query().Get("view") == "chat" {
		h.renderInterrogationPage(w, data, interrogation)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

func (h *Handler) renderInterrogationPage(w http.ResponseWriter, data map[string]any, interrogation map[string]any) {
	meta := webassets.InterrogationMeta{
		InterrogationID:       displayString(interrogation["interrogation_id"]),
		RunID:                 displayString(interrogation["run_id"]),
		Topic:                 displayString(interrogation["topic"]),
		State:                 displayString(interrogation["state"]),
		InterrogatorSessionID: displayString(interrogation["interrogator_session_id"]),
		TargetSessionID:       displayString(interrogation["target_session_id"]),
		OpenedAt:              displayString(interrogation["opened_at"]),
		ClosedAt:              displayString(interrogation["closed_at"]),
	}
	// interrogation.show (reads.HandleInterrogationShow) returns turns as
	// []map[string]any over in-process RPC; tolerate []any too (e.g. when the
	// result has round-tripped through JSON). A single-type assertion silently
	// drops every turn — the live D1 verification caught exactly that.
	turns := make([]webassets.InterrogationTurn, 0)
	appendTurn := func(row map[string]any) {
		turns = append(turns, webassets.InterrogationTurn{
			Kind: displayString(row["kind"]),
			Body: displayString(row["body"]),
		})
	}
	switch rows := data["turns"].(type) {
	case []map[string]any:
		for _, row := range rows {
			appendTurn(row)
		}
	case []any:
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok {
				appendTurn(row)
			}
		}
	}
	body, err := webassets.RenderInterrogation(meta, turns)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorPayload("render_failed", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// D142 / RFC 0084 build review: curated interrogation bodies must not
	// persist in a browser profile after token rotation or shared-operator use.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) showConversation(w http.ResponseWriter, r *http.Request, runID, conversationID string) {
	data, status, code, message := h.call(r.Context(), "conversation.show", map[string]any{"conversation_id": conversationID})
	if code != "" {
		writeJSON(w, status, errorPayload(code, message))
		return
	}
	conversation, _ := data["conversation"].(map[string]any)
	if conversation == nil || displayString(conversation["run_id"]) != runID {
		writeJSON(w, http.StatusNotFound, errorPayload("not_found", "not found"))
		return
	}
	if r.URL.Query().Get("view") == "chat" {
		h.renderConversationPage(w, data, conversation)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

func (h *Handler) renderConversationPage(w http.ResponseWriter, data map[string]any, conversation map[string]any) {
	meta := webassets.ConversationMeta{
		ConversationID: displayString(conversation["conversation_id"]),
		RunID:          displayString(conversation["run_id"]),
		Topic:          displayString(conversation["topic"]),
		State:          displayString(conversation["state"]),
		OpenedAt:       displayString(conversation["opened_at"]),
		ClosedAt:       displayString(conversation["closed_at"]),
	}
	turns := make([]webassets.ConversationTurn, 0)
	appendTurn := func(row map[string]any) {
		turns = append(turns, webassets.ConversationTurn{
			Speaker: displayString(row["author_session_id"]),
			Body:    displayString(row["body"]),
		})
	}
	switch rows := data["turns"].(type) {
	case []map[string]any:
		for _, row := range rows {
			appendTurn(row)
		}
	case []any:
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok {
				appendTurn(row)
			}
		}
	}
	body, err := webassets.RenderConversation(meta, turns)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorPayload("render_failed", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) authenticate(r *http.Request) bool {
	if h.config.ServiceToken == "" {
		return true
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	value, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(h.config.ServiceToken)) == 1
}

type rpcEventSource struct {
	handler *Handler
	runID   string
}

func (s rpcEventSource) Fetch(ctx context.Context, since int, limit int) (websse.Batch, error) {
	data, _, code, message := s.handler.call(ctx, "run.events", map[string]any{
		"run_id":         s.runID,
		"since_event_id": since,
		"limit":          limit,
	})
	if code != "" {
		return websse.Batch{}, fmt.Errorf("%s: %s", code, message)
	}
	batch := websse.Batch{}
	if run, ok := data["run"].(map[string]any); ok {
		batch.RunState, _ = run["state"].(string)
	}
	if terminal, ok := data["terminal"].(bool); ok {
		batch.Terminal = terminal
	}
	rows, _ := data["events"].([]any)
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := intFromAny(row["event_id"])
		batch.Events = append(batch.Events, websse.Event{ID: id, Type: "striatum.event", Data: row})
	}
	return batch, nil
}

type sseSlots struct {
	mu     sync.Mutex
	counts map[string]int
}

func (s *sseSlots) acquire(runID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counts == nil {
		s.counts = map[string]int{}
	}
	if s.counts[runID] >= 32 {
		return false
	}
	s.counts[runID]++
	return true
}

func (s *sseSlots) release(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counts == nil {
		return
	}
	if s.counts[runID] <= 1 {
		delete(s.counts, runID)
		return
	}
	s.counts[runID]--
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) bool {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, errorPayload("unsupported_media_type", "Content-Type must be application/json"))
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, errorPayload("malformed_body", err.Error()))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func setBaseHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", webassets.ContentSecurityPolicy)
}

func errorPayload(code string, message string) map[string]any {
	return map[string]any{"ok": false, "error": map[string]any{"code": code, "message": message}}
}

func isMutationMethod(method string) bool {
	entry, ok := rpc.MethodRegistry[method]
	if !ok {
		return true
	}
	return entry.RequiredCapability != nil && *entry.RequiredCapability != rpc.CapabilityRead
}

func statusForRPCCode(code string) int {
	switch code {
	case "schema_invalid", "workflow_error":
		return http.StatusBadRequest
	case "token_missing", "token_malformed", "token_invalid", "token_revoked", "token_expired", "capability_missing", "capability_scope_mismatch", "capability_expired":
		return http.StatusForbidden
	case "not_found", "repo_not_registered":
		return http.StatusNotFound
	case "invalid_transition", "branch_confirmation_required":
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func methodForArgv(raw any) (string, map[string]any) {
	argv, ok := raw.([]any)
	if !ok || len(argv) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(argv))
	for _, item := range argv {
		text, ok := item.(string)
		if !ok {
			return "", nil
		}
		parts = append(parts, text)
	}
	switch {
	case parts[0] == "status":
		params := map[string]any{}
		if value := argvValue(parts, "--run-id"); value != "" {
			params["run_id"] = value
		}
		return "status", params
	case parts[0] == "dashboard":
		return "dashboard", map[string]any{"run_id": argvValue(parts, "--run-id")}
	case parts[0] == "run" && len(parts) > 1 && parts[1] == "cancel":
		return "run.cancel", map[string]any{"run_id": argvValue(parts, "--run-id")}
	default:
		return "", nil
	}
}

func argvValue(argv []string, flag string) string {
	for i, part := range argv {
		if part == flag && i+1 < len(argv) {
			return argv[i+1]
		}
		if strings.HasPrefix(part, flag+"=") {
			return strings.TrimPrefix(part, flag+"=")
		}
	}
	return ""
}

// hostOnly returns the host portion of a Host header value, stripping any
// :port and surrounding brackets from an IPv6 literal.
func hostOnly(hostport string) string {
	host := hostport
	if strings.Contains(hostport, ":") {
		if parsed, _, err := net.SplitHostPort(hostport); err == nil {
			host = parsed
		}
	}
	return strings.Trim(host, "[]")
}

func isLoopbackHost(hostport string) bool {
	host := hostOnly(hostport)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return strings.Contains(origin, "://"+r.Host)
}

func isRetiredRoute(clean string) bool {
	return clean == "/dogfood" || strings.HasPrefix(clean, "/dogfood/") || clean == "/chat" || strings.HasPrefix(clean, "/chat/")
}

// displayString coerces a curated field value to its display string. It
// handles the string case directly and falls back to fmt.Sprint for non-nil
// non-string scalars (e.g. timestamps that scan as time.Time over the
// in-process RPC path), returning "" for nil. html/template escapes the result
// at render time.
func displayString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
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
