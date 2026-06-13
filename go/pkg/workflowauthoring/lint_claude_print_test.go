package workflowauthoring

import (
	"strings"
	"testing"
)

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

func codexLaneWorkflow(command []any, agentLoop bool) map[string]any {
	wf := validWorkflow()
	lane := map[string]any{"adapter": "process", "command": command}
	if agentLoop {
		lane["adapter_capabilities"] = map[string]any{"agent_loop": true}
	}
	wf["lanes"] = map[string]any{"codex": lane}
	wf["coordinator"] = map[string]any{"role_id": "author", "lane_id": "codex"}
	for _, j := range wf["jobs"].([]any) {
		j.(map[string]any)["lane_id"] = "codex"
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

// TestRefuseClaudePrintLanes proves that `claude --print`/`-p` lanes are a HARD
// REFUSAL, not a warning. RefuseClaudePrintLanes is the choke point the
// `workflow validate` CLI and `run.prepare` call. After the 2026-06-15 deadline
// a live `claude --print` invocation bills API tokens instead of plan usage, so
// the refusal error must name that cost consequence. (#199)
func TestRefuseClaudePrintLanes(t *testing.T) {
	for _, command := range [][]any{
		{"claude", "--print"},
		{"claude", "-p"},
		{"claude", "--print", "--permission-mode", "acceptEdits"},
	} {
		err := RefuseClaudePrintLanes(claudeLaneWorkflow(command, true))
		if err == nil {
			t.Fatalf("RefuseClaudePrintLanes(%v): expected a refusal, got nil", command)
		}
		if !strings.Contains(err.Error(), "2026-06-15") {
			t.Fatalf("RefuseClaudePrintLanes(%v): refusal must name the 2026-06-15 cost consequence; got %q", command, err.Error())
		}
	}
}

// TestRefuseClaudePrintLanesAllowsOverride proves the explicit per-lane override
// (`allow_claude_print: true`) keeps genuine compatibility cases working. (#199)
func TestRefuseClaudePrintLanesAllowsOverride(t *testing.T) {
	wf := claudeLaneWorkflow([]any{"claude", "--print"}, true)
	lane := wf["lanes"].(map[string]any)["claude"].(map[string]any)
	lane["allow_claude_print"] = true
	if err := RefuseClaudePrintLanes(wf); err != nil {
		t.Fatalf("RefuseClaudePrintLanes with allow_claude_print override: %v", err)
	}
	// The lint finding must also fall silent once overridden.
	if lintFiresRule(t, wf, "deprecated_claude_print_lane") {
		t.Fatal("an overridden claude --print lane must not fire the lint finding")
	}
}

// TestRefuseClaudePrintLanesAllowsBareInteractive proves the supported
// agent-loop shape is not refused. (#199)
func TestRefuseClaudePrintLanesAllowsBareInteractive(t *testing.T) {
	if err := RefuseClaudePrintLanes(claudeLaneWorkflow([]any{"claude", "--dangerously-skip-permissions"}, true)); err != nil {
		t.Fatalf("RefuseClaudePrintLanes(bare interactive claude): %v", err)
	}
}

func TestLintFlagsDeprecatedCodexExecLane(t *testing.T) {
	rule := "deprecated_codex_exec_lane"

	if !lintFiresRule(t, codexLaneWorkflow([]any{"codex", "exec", "-"}, false), rule) {
		t.Fatal("codex exec lane must be flagged as deprecated")
	}
	if !lintFiresRule(t, codexLaneWorkflow([]any{"sh", "-c", "IFS= read -r prompt; exec codex exec --model gpt-5.5 \"$prompt\" </dev/null"}, false), rule) {
		t.Fatal("shell-shim codex exec lane must be flagged as deprecated")
	}
	if lintFiresRule(t, codexLaneWorkflow([]any{"codex", "--dangerously-bypass-approvals-and-sandbox", "--no-alt-screen"}, true), rule) {
		t.Fatal("a bare interactive codex agent-loop lane must NOT be flagged")
	}
}

func TestRefuseCodexExecLanes(t *testing.T) {
	for _, command := range [][]any{
		{"codex", "exec", "-"},
		{"sh", "-c", "IFS= read -r prompt; exec codex exec --model gpt-5.5 \"$prompt\" </dev/null"},
	} {
		err := RefuseCodexExecLanes(codexLaneWorkflow(command, true))
		if err == nil {
			t.Fatalf("RefuseCodexExecLanes(%v): expected a refusal, got nil", command)
		}
		if !strings.Contains(err.Error(), "codex exec") || !strings.Contains(err.Error(), "#267") {
			t.Fatalf("RefuseCodexExecLanes(%v): refusal should name codex exec and #267; got %q", command, err.Error())
		}
	}
}

func TestRefuseRetiredOneShotLanesAllowsCodexInteractive(t *testing.T) {
	if err := RefuseRetiredOneShotLanes(codexLaneWorkflow([]any{"codex", "--dangerously-bypass-approvals-and-sandbox", "--no-alt-screen"}, true)); err != nil {
		t.Fatalf("RefuseRetiredOneShotLanes(bare interactive codex): %v", err)
	}
}
