package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestAugmentationReferencesInspectLocalCorpusBundle(t *testing.T) {
	repoRoot := t.TempDir()
	bundle := filepath.Join(repoRoot, "exports", "corpus")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"schema_version":           "striatum.corpus_export.v1",
		"corpus_contract_version":  2,
		"corpus_id":                "striatum:" + strings.Repeat("a", 64),
		"redaction_tier":           "public",
		"verification_depth":       "deep_chain",
		"bundle_sha256":            strings.Repeat("b", 64),
		"row_counts":               map[string]any{"rfc": float64(2)},
		"repo_root":                "/absolute/path/not-exposed",
		"incremental_export_token": "not surfaced",
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	workflow := map[string]any{
		"augmentation": map[string]any{
			"mode":                    "reference_only",
			"required":                false,
			"budget_per_packet_lines": 25,
			"sources": []any{
				map[string]any{
					"id":          "local-corpus",
					"kind":        "corpus_bundle",
					"path":        "exports/corpus",
					"description": "Local corpus bundle",
				},
				map[string]any{"id": "missing", "kind": "corpus_bundle", "path": "exports/missing"},
			},
			"jobs": []any{"draft"},
		},
	}

	got := augmentationReferences(workflow, "draft", repoRoot)

	if got["mode"] != "reference_only" || got["required"] != false {
		t.Fatalf("augmentation policy = %#v", got)
	}
	if got["budget_per_packet_lines"] != 25 {
		t.Fatalf("budget = %v", got["budget_per_packet_lines"])
	}
	sources := got["sources"].([]any)
	first := sources[0].(map[string]any)
	if first["status"] != "available" || first["available"] != true {
		t.Fatalf("first source = %#v", first)
	}
	summary := first["manifest"].(map[string]any)
	if summary["corpus_id"] != "striatum:"+strings.Repeat("a", 64) {
		t.Fatalf("summary = %#v", summary)
	}
	if _, ok := summary["repo_root"]; ok {
		t.Fatalf("absolute repo_root leaked into summary: %#v", summary)
	}
	second := sources[1].(map[string]any)
	if second["status"] != "missing" || second["reason"] != "bundle_not_found" {
		t.Fatalf("second source = %#v", second)
	}
}

func TestAugmentationReferencesOmittedForNonOptedInJob(t *testing.T) {
	workflow := map[string]any{
		"augmentation": map[string]any{
			"mode":    "reference_only",
			"sources": []any{map[string]any{"id": "local", "kind": "corpus_bundle", "path": "exports/corpus"}},
			"jobs":    []any{"draft"},
		},
	}

	if got := augmentationReferences(workflow, "review", t.TempDir()); got != nil {
		t.Fatalf("augmentation references = %#v, want nil", got)
	}
}
