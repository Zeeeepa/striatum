package workflowauthoring

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

// TestLintWarnsOnDegradedSeatLane: a workflow declaring a lane whose adapter seat
// is `degraded` must emit the RFC 0109 degraded_seat_lane warning naming the lane
// and the gap — closing the silent-collapse half of #139. agy graduated to
// `supported` in Phase B, so no production seat is degraded; the warning path is
// exercised with a synthetic degraded seat via the workflowtemplates test seam.
func TestLintWarnsOnDegradedSeatLane(t *testing.T) {
	cleanup := workflowtemplates.RegisterDegradedSeatForTest("phantom-cli",
		"phantom multi-turn collapse (#0000)")
	defer cleanup()

	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["phantom"] = map[string]any{
		"adapter":              "process",
		"command":              []any{"phantom-cli", "--go"},
		"adapter_capabilities": map[string]any{"agent_loop": true},
	}
	payload, err := Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	var found map[string]any
	for _, w := range payload["warnings"].([]map[string]any) {
		if w["rule"] == "degraded_seat_lane" {
			found = w
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a degraded_seat_lane warning for a declared degraded-seat lane")
	}
	if found["lane_id"] != "phantom" || found["adapter"] != "phantom-cli" {
		t.Fatalf("warning must name the offending lane/adapter: %#v", found)
	}
	if found["seat_tier"] != "degraded" {
		t.Fatalf("warning seat_tier = %v, want degraded", found["seat_tier"])
	}
	if msg, _ := found["message"].(string); !strings.Contains(msg, "RFC 0109") || !strings.Contains(msg, "#139") {
		t.Fatalf("warning message must cite RFC 0109 and #139: %q", msg)
	}
}

// TestLintSilentOnSupportedOrExperimentalSeatLane: agy (now `supported`, RFC 0109
// P3) and claude/codex (`experimental` — they hold a seat, just ungated by an
// installed-CLI fixture) must NOT trigger the degraded_seat_lane warning. Faithful
// to RFC 0109's acceptance, which surfaces degraded/unsupported seats specifically,
// and proves the graduation closed the loop (agy stops warning).
func TestLintSilentOnSupportedOrExperimentalSeatLane(t *testing.T) {
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["agy"] = map[string]any{
		"adapter":              "process",
		"command":              []any{"agy", "--dangerously-skip-permissions"},
		"adapter_capabilities": map[string]any{"agent_loop": true},
	}
	lanes["claude_code"] = map[string]any{
		"adapter":              "process",
		"command":              []any{"claude", "--dangerously-skip-permissions"},
		"adapter_capabilities": map[string]any{"agent_loop": true},
	}
	lanes["codex"] = map[string]any{
		"adapter":              "process",
		"command":              []any{"codex"},
		"adapter_capabilities": map[string]any{"agent_loop": true},
	}
	payload, err := Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	for _, w := range payload["warnings"].([]map[string]any) {
		if w["rule"] == "degraded_seat_lane" {
			t.Fatalf("supported/experimental seat must not warn: %#v", w)
		}
	}
}
