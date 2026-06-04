package workflowauthoring

import "testing"

func lintFiresRule(t *testing.T, workflow map[string]any, rule string) bool {
	t.Helper()
	result, err := Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	for _, w := range result["warnings"].([]map[string]any) {
		if w["rule"] == rule {
			return true
		}
	}
	return false
}

// claudeLaneWorkflow returns a valid workflow whose single lane is `claude` with
// the given command + agent_loop setting.
func claudeLaneWorkflow(command []any, agentLoop bool) map[string]any {
	wf := validWorkflow()
	lane := map[string]any{"adapter": "process", "command": command}
	if agentLoop {
		lane["adapter_capabilities"] = map[string]any{"agent_loop": true}
	}
	wf["lanes"] = map[string]any{"claude": lane}
	wf["coordinator"] = map[string]any{"role_id": "author", "lane_id": "claude"}
	for _, j := range wf["jobs"].([]any) {
		j.(map[string]any)["lane_id"] = "claude"
	}
	return wf
}

// TestLintFlagsDeprecatedClaudePrintLane guards the deprecation of the retired
// one-shot `claude --print` lane mode (RFC 0088 / D148). It fires regardless of
// adapter_capabilities.agent_loop, because `--print` defeats the interactive
// work-packet loop even when the agent-loop executor wraps it (#148). A bare
// interactive `claude` lane must NOT fire.
func TestLintFlagsDeprecatedClaudePrintLane(t *testing.T) {
	rule := "deprecated_claude_print_lane"

	if !lintFiresRule(t, claudeLaneWorkflow([]any{"claude", "--print"}, false), rule) {
		t.Fatal("claude --print lane must be flagged as deprecated")
	}
	// The key point: --print is broken even when the lane declares agent_loop.
	if !lintFiresRule(t, claudeLaneWorkflow([]any{"claude", "--print"}, true), rule) {
		t.Fatal("claude --print must be flagged even with agent_loop=true (--print defeats the loop)")
	}
	if !lintFiresRule(t, claudeLaneWorkflow([]any{"claude", "-p"}, false), rule) {
		t.Fatal("the -p short form must be flagged too")
	}
	// The supported shape must stay silent.
	if lintFiresRule(t, claudeLaneWorkflow([]any{"claude", "--dangerously-skip-permissions"}, true), rule) {
		t.Fatal("a bare interactive claude agent-loop lane must NOT be flagged")
	}
}
