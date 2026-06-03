package workflowauthoring

import (
	"strings"
	"testing"
)

// TestLintWarnsOnDegradedSeatLane: a workflow declaring an `agy` agent-loop lane
// (a degraded seat, #95/#85/#76/#139) must emit the RFC 0109 degraded_seat_lane
// warning naming the lane and the gap — closing the silent-collapse half of #139.
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
		t.Fatalf("expected a degraded_seat_lane warning for a declared agy lane")
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

// TestLintSilentOnExperimentalSeatLane: claude/codex seats hold a lane today (they
// are `experimental` — ungated by an installed-CLI fixture, not degraded), so they
// must NOT trigger the degraded_seat_lane warning. Faithful to RFC 0109: surface
// degraded/unsupported seats, not working-but-ungated ones.
func TestLintSilentOnExperimentalSeatLane(t *testing.T) {
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
			t.Fatalf("experimental (working) seat must not warn: %#v", w)
		}
	}
}
