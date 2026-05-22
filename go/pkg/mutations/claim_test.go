package mutations

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestClaimNextResultSurfacesPacketIDAndSuperviseSend(t *testing.T) {
	packet := map[string]any{"packet_id": "wp_1"}
	result := claimNextResult("sess_1", "wp_1", packet)

	if result["status"] != "claimed" {
		t.Fatalf("status = %v", result["status"])
	}
	if result["packet_id"] != "wp_1" {
		t.Fatalf("packet_id = %v", result["packet_id"])
	}
	if !reflect.DeepEqual(result["packet"], packet) {
		t.Fatalf("packet = %#v", result["packet"])
	}
	nextSteps := result["next_steps"].(map[string]any)
	if nextSteps["supervise_send"] != "striatum supervise send --session-id sess_1 --packet-id wp_1" {
		t.Fatalf("supervise_send = %v", nextSteps["supervise_send"])
	}
}

func TestAwaitPacketValidatesSessionID(t *testing.T) {
	_, err := HandleAwaitPacket(context.Background(), inertRunner{}, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_await",
		Method:        "work.await_packet",
		Params:        map[string]any{"repository_id": "repo_1"}, // missing session_id
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("err = %v, want schema_invalid", err)
	}
}

func TestPacketTaskPromptResolvesWorkflowLocalPath(t *testing.T) {
	got := packetTaskPrompt(
		map[string]any{"path": "prompts/demo.md"},
		map[string]any{"source_path": "docs/operator/workflows/demo/workflow.json"},
	)

	if got["path"] != "docs/operator/workflows/demo/prompts/demo.md" {
		t.Fatalf("path = %v", got["path"])
	}
	if got["workflow_relative_path"] != "prompts/demo.md" {
		t.Fatalf("workflow_relative_path = %v", got["workflow_relative_path"])
	}
	if got["workflow_source_path"] != "docs/operator/workflows/demo/workflow.json" {
		t.Fatalf("workflow_source_path = %v", got["workflow_source_path"])
	}
}

func TestPacketTaskPromptLeavesRootRelativePath(t *testing.T) {
	got := packetTaskPrompt(
		map[string]any{"path": "prompts/demo.md"},
		map[string]any{"source_path": "workflow.json"},
	)

	if !reflect.DeepEqual(got, map[string]any{"path": "prompts/demo.md"}) {
		t.Fatalf("task prompt = %#v", got)
	}
}
