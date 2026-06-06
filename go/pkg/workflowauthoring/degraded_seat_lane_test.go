package workflowauthoring

import (
	"strings"
	"testing"
)

// TestLintWarnsOnDegradedSeatLane: a workflow declaring a lane whose adapter seat
// is `degraded` must emit the RFC 0109 degraded_seat_lane warning naming the lane
// and the gap — closing the silent-collapse half of #139. As of #190 (D174) agy is
// the real production degraded seat (OAuth-only CLI 1.0.6 stalls the P3 gate), so
// the warning path is exercised directly against it — no synthetic test seam needed.
func TestLintWarnsOnDegradedSeatLane(t *testing.T) {
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["agy"] = map[string]any{
		"adapter":              "process",
		"command":              []any{"agy", "--dangerously-skip-permissions"},
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
	if found["lane_id"] != "agy" || found["adapter"] != "agy" {
		t.Fatalf("warning must name the offending lane/adapter: %#v", found)
	}
	if found["seat_tier"] != "degraded" {
		t.Fatalf("warning seat_tier = %v, want degraded", found["seat_tier"])
	}
	if msg, _ := found["message"].(string); !strings.Contains(msg, "RFC 0109") || !strings.Contains(msg, "#139") {
		t.Fatalf("warning message must cite RFC 0109 and #139: %q", msg)
	}
}

// TestLintSilentOnSupportedOrExperimentalSeatLane: claude (`experimental`) and
// codex (`supported`) must NOT trigger the degraded_seat_lane warning. As of #190
// (D174) agy is `degraded`, so it is no longer in the silent set — it is covered by
// TestLintWarnsOnDegradedSeatLane.
func TestLintSilentOnSupportedOrExperimentalSeatLane(t *testing.T) {
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
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
