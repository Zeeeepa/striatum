package workflowauthoring

import "testing"

func workflowWithLaneCommand(laneID string, command []any) map[string]any {
	wf := validWorkflow()
	lane := map[string]any{
		"adapter":              "process",
		"command":              command,
		"adapter_capabilities": map[string]any{"agent_loop": true},
	}
	wf["lanes"] = map[string]any{laneID: lane}
	wf["coordinator"] = map[string]any{"role_id": "author", "lane_id": laneID}
	for _, j := range wf["jobs"].([]any) {
		j.(map[string]any)["lane_id"] = laneID
	}
	return wf
}

func lintFindingForRule(t *testing.T, workflow map[string]any, rule string) (map[string]any, bool) {
	t.Helper()
	result, err := Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	for _, warning := range result["warnings"].([]map[string]any) {
		if warning["rule"] == rule {
			return warning, true
		}
	}
	return nil, false
}

func TestLintRefusesAdapterFlagMismatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command []any
		flag    string
	}{
		{
			name:    "claude unsafe flag on agy",
			command: []any{"agy", "--dangerously-skip-permissions"},
			flag:    "--dangerously-skip-permissions",
		},
		{
			name:    "codex unsafe flag on agy",
			command: []any{"agy", "--dangerously-bypass-approvals-and-sandbox"},
			flag:    "--dangerously-bypass-approvals-and-sandbox",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			finding, ok := lintFindingForRule(t, workflowWithLaneCommand("agy", tc.command), "adapter_flag_mismatch")
			if !ok {
				t.Fatal("expected adapter_flag_mismatch refusal")
			}
			if finding["severity"] != "refusal" {
				t.Fatalf("severity = %q, want refusal: %#v", finding["severity"], finding)
			}
			if finding["lane_id"] != "agy" || finding["adapter"] != "agy" || finding["flag"] != tc.flag {
				t.Fatalf("finding must name lane, adapter, and mismatched flag: %#v", finding)
			}
		})
	}
}

func TestLintAllowsAdapterSpecificFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		laneID  string
		command []any
	}{
		{
			name:    "agy sandbox flag",
			laneID:  "agy",
			command: []any{"agy", "--sandbox"},
		},
		{
			name:    "claude skip permissions",
			laneID:  "claude",
			command: []any{"claude", "--dangerously-skip-permissions"},
		},
		{
			name:    "codex bypass approvals",
			laneID:  "codex",
			command: []any{"codex", "--dangerously-bypass-approvals-and-sandbox"},
		},
		{
			name:    "codex yolo",
			laneID:  "codex",
			command: []any{"codex", "--yolo"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if finding, ok := lintFindingForRule(t, workflowWithLaneCommand(tc.laneID, tc.command), "adapter_flag_mismatch"); ok {
				t.Fatalf("valid adapter flag produced adapter_flag_mismatch: %#v", finding)
			}
		})
	}
}
