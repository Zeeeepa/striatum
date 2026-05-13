package rpc

import (
	"context"
	"testing"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	body := []byte(`{"schema_version":1,"request_id":"req_1","method":"daemon.hello","params":{"client":{"supported_envelope":[1],"supported_framings":["json"]}},"deadline_ms":0}`)
	envelope, err := DecodeEnvelope(body)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	encoded, err := envelope.Encode()
	if err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
	again, err := DecodeEnvelope(encoded)
	if err != nil {
		t.Fatalf("decode encoded envelope: %v", err)
	}
	if again.RequestID != envelope.RequestID || again.Method != envelope.Method {
		t.Fatalf("round trip mismatch: %#v != %#v", again, envelope)
	}
}

func TestDescribeRequiresHandshake(t *testing.T) {
	server := NewServer()
	envelope := Envelope{
		SchemaVersion: SupportedEnvelopeVersion,
		RequestID:     "req_1",
		Method:        "daemon.describe",
		Params:        map[string]any{},
	}
	response := server.Handle(context.Background(), envelope, "conn")
	if response.OK {
		t.Fatalf("describe without hello succeeded")
	}
	if response.Data["code"] != "version_incompatible" {
		t.Fatalf("unexpected error: %#v", response.Data)
	}
}

func TestHelloThenDescribe(t *testing.T) {
	server := NewServer()
	hello := Envelope{
		SchemaVersion: SupportedEnvelopeVersion,
		RequestID:     "req_hello",
		Method:        "daemon.hello",
		Params: map[string]any{"client": map[string]any{
			"supported_envelope": []any{float64(1)},
			"supported_framings": []any{"json"},
		}},
	}
	if response := server.Handle(context.Background(), hello, "conn"); !response.OK {
		t.Fatalf("hello failed: %#v", response.Data)
	}
	describe := Envelope{
		SchemaVersion: SupportedEnvelopeVersion,
		RequestID:     "req_describe",
		Method:        "daemon.describe",
		Params:        map[string]any{},
	}
	response := server.Handle(context.Background(), describe, "conn")
	if !response.OK {
		t.Fatalf("describe failed: %#v", response.Data)
	}
	if response.Data["methods_etag"] == "" {
		t.Fatalf("missing methods etag: %#v", response.Data)
	}
}
