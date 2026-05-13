package rpc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

type Handler func(context.Context, Envelope) (map[string]any, error)

type AuditRecorder interface {
	RecordRPC(context.Context, Envelope, AuthContext, Response) (string, error)
}

type Server struct {
	DaemonVersion   string
	SubstrateSchema int
	SealedApply     map[string]any
	Authorizer      Authorizer
	AuditRecorder   AuditRecorder
	Handlers        map[string]Handler

	mu            sync.RWMutex
	seenRequests  map[string]struct{}
	handshakeSeen map[string]struct{}
}

func NewServer() *Server {
	return &Server{
		DaemonVersion:   "unknown",
		SubstrateSchema: 0,
		SealedApply: map[string]any{
			"supported":  false,
			"key_loaded": false,
			"public_key": nil,
		},
		Authorizer:    AllowAllAuthorizer{},
		Handlers:      map[string]Handler{},
		seenRequests:  map[string]struct{}{},
		handshakeSeen: map[string]struct{}{},
	}
}

func (s *Server) Register(method string, handler Handler) {
	s.Handlers[method] = handler
}

func (s *Server) Handle(ctx context.Context, envelope Envelope, connectionID string) Response {
	return s.handle(ctx, envelope, connectionID, true)
}

func (s *Server) HandleWithoutHandshake(ctx context.Context, envelope Envelope, connectionID string) Response {
	return s.handle(ctx, envelope, connectionID, false)
}

func (s *Server) handle(ctx context.Context, envelope Envelope, connectionID string, requireHandshake bool) Response {
	auth := AuthContext{RepositoryID: repositoryID(envelope.Params), Decision: "allowed"}
	if duplicate := s.markRequest(envelope.RequestID); duplicate {
		return ErrorResponse(envelope.RequestID, NewError("duplicate_request", "daemon RPC request_id was already used", nil), "")
	}

	var data map[string]any
	var err error
	entry, known := MethodRegistry[envelope.Method]
	if requireHandshake && envelope.Method != "daemon.hello" && !s.hasHandshake(connectionID) {
		err = NewError("version_incompatible", "daemon.hello must run before ordinary RPC routes", nil)
		auth = deniedAuth(auth, "version_incompatible")
	} else if !known {
		err = NewError("method_unknown", fmt.Sprintf("unknown daemon RPC method: %s", envelope.Method), nil)
		auth = deniedAuth(auth, "method_unknown")
	} else if entry.RepositoryScope && repositoryID(envelope.Params) == "" {
		err = NewError("repo_not_registered", "daemon RPC route requires repository_id", nil)
		auth = deniedAuth(auth, "repo_not_registered")
	} else if envelope.Method == "daemon.hello" {
		data, err = s.buildWelcome(envelope.Params)
		if err == nil {
			s.markHandshake(connectionID)
		}
	} else {
		auth = s.Authorizer.Authorize(entry.RequiredCapability, repositoryID(envelope.Params), envelope.CapabilityToken)
		if auth.RepositoryID == "" {
			auth.RepositoryID = repositoryID(envelope.Params)
		}
		err = RequireAllowed(auth)
		if err == nil {
			data, err = s.route(ctx, envelope)
		}
		if err != nil {
			if rpcErr := (&Error{}); errors.As(err, &rpcErr) {
				auth = deniedAuth(auth, rpcErr.Code)
			}
		}
	}

	var response Response
	if err != nil {
		response = ErrorResponse(envelope.RequestID, err, "")
	} else {
		response = OKResponse(envelope.RequestID, data, "")
	}
	if s.AuditRecorder != nil {
		auditID, auditErr := s.AuditRecorder.RecordRPC(ctx, envelope, auth, response)
		if auditErr == nil && auditID != "" {
			response.AuditID = auditID
		}
	}
	return response
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}
		go func() {
			_ = s.ServeConn(ctx, conn, conn.RemoteAddr().String())
		}()
	}
}

func (s *Server) ServeConn(ctx context.Context, rwc io.ReadWriteCloser, connectionID string) error {
	defer rwc.Close()
	reader := bufio.NewScanner(rwc)
	writer := bufio.NewWriter(rwc)
	for reader.Scan() {
		line := reader.Bytes()
		if len(stripSpace(line)) == 0 {
			continue
		}
		envelope, err := DecodeEnvelope(line)
		var response Response
		if err != nil {
			response = ErrorResponse("", err, "")
		} else {
			response = s.Handle(ctx, envelope, connectionID)
		}
		encoded, err := response.Encode()
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return reader.Err()
}

func ListenUnix(path string) (net.Listener, error) {
	if path == "" {
		return nil, errors.New("socket path is required")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	return listener, nil
}

func (s *Server) route(ctx context.Context, envelope Envelope) (map[string]any, error) {
	if envelope.Method == "daemon.describe" {
		return DescribeMethods(), nil
	}
	handler, ok := s.Handlers[envelope.Method]
	if !ok {
		return nil, NewError("method_unknown", fmt.Sprintf("method has no handler: %s", envelope.Method), nil)
	}
	return handler(ctx, envelope)
}

func (s *Server) buildWelcome(params map[string]any) (map[string]any, error) {
	client, ok := params["client"].(map[string]any)
	if !ok {
		return nil, NewError("schema_invalid", "daemon.hello requires a client object", nil)
	}
	if !containsInt(client["supported_envelope"], SupportedEnvelopeVersion) {
		return nil, NewError("version_incompatible", "client and daemon have no shared envelope version", map[string]any{
			"daemon_supported_envelope": []int{SupportedEnvelopeVersion},
		})
	}
	if !containsString(client["supported_framings"], DefaultFraming) {
		return nil, NewError("version_incompatible", "client and daemon have no shared framing", map[string]any{
			"daemon_supported_framings": []string{DefaultFraming},
		})
	}
	return map[string]any{
		"daemon_version":   s.DaemonVersion,
		"envelope":         SupportedEnvelopeVersion,
		"framing":          DefaultFraming,
		"substrate":        "postgres",
		"substrate_schema": s.SubstrateSchema,
		"methods_etag":     MethodsETag(),
		"sealed_apply":     s.SealedApply,
	}, nil
}

func (s *Server) markRequest(requestID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seenRequests[requestID]; ok {
		return true
	}
	if requestID != "" {
		s.seenRequests[requestID] = struct{}{}
	}
	return false
}

func (s *Server) markHandshake(connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handshakeSeen[connectionID] = struct{}{}
}

func (s *Server) hasHandshake(connectionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.handshakeSeen[connectionID]
	return ok
}

func repositoryID(params map[string]any) string {
	if value, ok := params["repository_id"]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func deniedAuth(auth AuthContext, reason string) AuthContext {
	auth.Decision = "denied"
	auth.DenialReason = reason
	return auth
}

func containsInt(value any, expected int) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if intValue, ok := intValue(item); ok && intValue == expected {
			return true
		}
	}
	return false
}

func containsString(value any, expected string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if text, ok := item.(string); ok && text == expected {
			return true
		}
	}
	return false
}

func stripSpace(value []byte) []byte {
	start := 0
	end := len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t' || value[start] == '\r' || value[start] == '\n') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t' || value[end-1] == '\r' || value[end-1] == '\n') {
		end--
	}
	return value[start:end]
}
