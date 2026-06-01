package adapterconformance

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/agentloop"
)

// TestC0PassesForRealAdapters is the #101 regression gate: against the real,
// unmodified go/pkg/agentloop construction, C0 must PASS for claude/codex/agy
// (argv bootstrap delivery, the codex URL override, the agy #76 survey key).
// This is the assertion that flips to a contract failure the moment
// bootstrapDeliveryModeFor is reverted to pty_submit for these adapters.
func TestC0PassesForRealAdapters(t *testing.T) {
	for _, spec := range Adapters() {
		res := assertC0(AssertInput{Adapter: spec})
		if res.Status != StatusPass {
			t.Errorf("C0 must pass for %s against real agentloop construction; got %s %q: %s",
				spec.Name, res.Status, res.FailureClass, res.Message)
		}
	}
}

// TestC0FailsOnPTYSubmitRevert proves the #101 regression gate is armed: if the
// production bootstrapDeliveryModeFor resolved to pty_submit for a declared
// adapter (the #101 revert), C0 would emit AdapterContractViolation. We can't
// mutate the production function from a test, so we drive the C0 assert through
// a command whose argv0 the agent-loop maps to pty_submit (an unknown adapter
// name), confirming that a pty_submit delivery mode is rejected by C0. A revert
// of bootstrapDeliveryModeFor for "claude"/"codex"/"agy" would put them on this
// exact rejected path.
func TestC0FailsOnPTYSubmitRevert(t *testing.T) {
	// Sanity: the agent-loop must currently map unknown adapters to pty_submit,
	// so this fixture genuinely exercises the rejection path.
	if mode := agentloop.BootstrapDeliveryModeFor([]string{"some-unknown-cli"}); mode != agentloop.BootstrapDeliveryPTYSubmit {
		t.Fatalf("precondition: unknown adapter should map to pty_submit, got %q", mode)
	}
	spec := AdapterSpec{Name: "claude_code", Command: []string{"some-unknown-cli"}, Profile: Full}
	res := assertC0(AssertInput{Adapter: spec})
	if res.Status != StatusContractFail || res.FailureClass != AdapterContractViolation {
		t.Fatalf("C0 must reject a pty_submit delivery mode with AdapterContractViolation; got %s %q: %s",
			res.Status, res.FailureClass, res.Message)
	}
}

// TestC0RegressionGateOnRealAdaptersIsArgv directly pins the #101 invariant:
// the production agent-loop resolves the three declared adapters to argv
// delivery. This is the single source of truth the C0 golden asserts; a revert
// here breaks both this test and TestC0PassesForRealAdapters.
func TestC0RegressionGateOnRealAdaptersIsArgv(t *testing.T) {
	for _, argv0 := range []string{"claude", "codex", "agy"} {
		if mode := agentloop.BootstrapDeliveryModeFor([]string{argv0}); mode != agentloop.BootstrapDeliveryArgv {
			t.Errorf("#101 regression: %s bootstrap delivery mode = %q, want argv (a revert to pty_submit re-wedges the lane)",
				argv0, mode)
		}
	}
}

// TestC0AgyPlacementAndSurveyKey asserts the agy-specific construction: the
// bootstrap is appended as --prompt-interactive <prompt> and the settings body
// carries usageStatisticsEnabled:false (#76).
func TestC0AgyPlacementAndSurveyKey(t *testing.T) {
	spec := AdapterSpec{Name: "agy", Command: []string{"agy"}, Profile: SingleShot}
	res := assertC0(AssertInput{Adapter: spec})
	if res.Status != StatusPass {
		t.Fatalf("agy C0 should pass: %s %q: %s", res.Status, res.FailureClass, res.Message)
	}
	_, argv := agentloop.AdapterBootstrapContract([]string{"agy"}, c0BootstrapPrompt)
	if len(argv) < 2 || argv[len(argv)-2] != "--prompt-interactive" || argv[len(argv)-1] != c0BootstrapPrompt {
		t.Fatalf("agy bootstrap must be --prompt-interactive <prompt>, got %v", argv)
	}
	body, err := agentloop.GeminiSettingsBody(c0Endpoint, c0Bearer)
	if err != nil {
		t.Fatal(err)
	}
	if !geminiSettingsSuppressesSurvey(body) {
		t.Fatalf("agy settings body must set usageStatisticsEnabled:false (#76): %s", body)
	}
}

// TestC0CodexPositionalAndURLOverride asserts the codex-specific construction:
// trailing positional bootstrap + the per-launch MCP URL override token.
func TestC0CodexPositionalAndURLOverride(t *testing.T) {
	spec := AdapterSpec{Name: "codex", Command: []string{"codex"}, Profile: Full}
	res := assertC0(AssertInput{Adapter: spec})
	if res.Status != StatusPass {
		t.Fatalf("codex C0 should pass: %s %q: %s", res.Status, res.FailureClass, res.Message)
	}
	_, argv := agentloop.AdapterBootstrapContract([]string{"codex"}, c0BootstrapPrompt)
	if argv[len(argv)-1] != c0BootstrapPrompt {
		t.Fatalf("codex bootstrap must be a trailing positional, got %v", argv)
	}
	override := agentloop.CodexMCPURLOverrideArg(c0Endpoint)
	if !codexURLOverrideWellFormed(override, c0Endpoint) {
		t.Fatalf("codex URL override malformed: %q", override)
	}
}

// TestC0EmptyCommandFails asserts an empty adapter command is a contract
// violation, not a panic.
func TestC0EmptyCommandFails(t *testing.T) {
	res := assertC0(AssertInput{Adapter: AdapterSpec{Name: "claude_code", Command: nil}})
	if res.Status != StatusContractFail || res.FailureClass != AdapterContractViolation {
		t.Fatalf("empty command must be AdapterContractViolation, got %s %q", res.Status, res.FailureClass)
	}
}
