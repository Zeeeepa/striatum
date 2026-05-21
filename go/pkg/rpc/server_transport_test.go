package rpc

import (
	"context"
	"testing"
)

type auditTransportRecorder struct {
	transport string
}

func (r *auditTransportRecorder) RecordRPC(context.Context, Envelope, AuthContext, Response) (string, error) {
	r.transport = "rpc"
	return "audit_1", nil
}

func (r *auditTransportRecorder) RecordRPCTransport(_ context.Context, _ Envelope, _ AuthContext, _ Response, transport string) (string, error) {
	r.transport = transport
	return "audit_1", nil
}

func TestHandleWithoutHandshakeRecordsMCPTransport(t *testing.T) {
	recorder := &auditTransportRecorder{}
	server := NewServer()
	server.AuditRecorder = recorder
	server.Register("status", func(context.Context, Envelope) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	})

	response := server.HandleWithoutHandshake(context.Background(), Envelope{
		SchemaVersion: SupportedEnvelopeVersion,
		RequestID:     "req_mcp_transport",
		Method:        "status",
		Params:        map[string]any{"repository_id": "repo_1"},
	}, "mcp")

	if !response.OK {
		t.Fatalf("response = %#v", response)
	}
	if response.AuditID != "audit_1" {
		t.Fatalf("audit_id = %q", response.AuditID)
	}
	if recorder.transport != "mcp" {
		t.Fatalf("audit transport = %q, want mcp", recorder.transport)
	}
}
