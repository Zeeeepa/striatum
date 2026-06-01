package adapterconformance

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/agentloop"
)

// c0BootstrapPrompt is a fixed, harmless bootstrap-prompt placeholder used by
// the C0 golden to exercise the argv-append path. Its content is irrelevant to
// the contract; only its placement (trailing positional vs --prompt-interactive)
// is asserted.
const c0BootstrapPrompt = "<<conformance-bootstrap-prompt>>"

// c0Endpoint / c0Bearer are fixed placeholders for the codex URL-override and
// agy settings-body checks. The C0 golden asserts construction shape, not live
// reachability.
const (
	c0Endpoint = "http://127.0.0.1:65535/mcp"
	c0Bearer   = "conformance-bearer-not-a-real-secret"
)

// assertC0 is the hermetic AdapterContract golden (DESIGN §1.1 C0). It is the
// #101 regression gate: it asserts the production go/pkg/agentloop bootstrap-
// delivery construction matches the declared per-adapter contract, with NO live
// CLI, daemon, or PostgreSQL. A revert of bootstrapDeliveryModeFor to pty_submit
// for claude/codex/agy re-introduces the #101 TUI-buffering blocker and MUST
// fail C0.
//
// Per adapter it asserts:
//   - claude/codex/agy → BootstrapDeliveryModeFor == argv (NOT pty_submit);
//   - agy             → bootstrap appended as "--prompt-interactive <prompt>";
//   - codex/claude    → bootstrap appended as a trailing positional prompt;
//   - codex           → the lane command injects -c mcp_servers.striatum.url=...;
//   - agy             → the generated settings body carries
//     usageStatisticsEnabled:false (#76).
func assertC0(in AssertInput) ClauseResult {
	c := Clause{ID: C0}
	adapterName := in.Adapter.Name
	command := in.Adapter.Command
	if len(command) == 0 {
		return c.contractFail(adapterName, AdapterContractViolation,
			"adapter command is empty; cannot derive bootstrap contract", nil)
	}
	argv0 := command[0]

	mode, argv := agentloop.AdapterBootstrapContract(command, c0BootstrapPrompt)

	// (1) claude/codex/agy MUST resolve to argv delivery, never pty_submit.
	if mode != agentloop.BootstrapDeliveryArgv {
		return c.contractFail(adapterName, AdapterContractViolation,
			fmt.Sprintf("bootstrap delivery mode is %q, want %q — a revert to pty_submit is the #101 regression",
				mode, agentloop.BootstrapDeliveryArgv),
			map[string]string{"argv0": argv0, "delivery_mode": mode})
	}

	// (2) adapter-specific bootstrap placement.
	switch argv0 {
	case "agy":
		// agy takes the bootstrap as the value of --prompt-interactive.
		if len(argv) < 2 ||
			argv[len(argv)-2] != "--prompt-interactive" ||
			argv[len(argv)-1] != c0BootstrapPrompt {
			return c.contractFail(adapterName, AdapterContractViolation,
				fmt.Sprintf("agy bootstrap must be appended as --prompt-interactive <prompt>, got argv %v", argv),
				map[string]string{"argv0": argv0})
		}
		// (3b) agy settings body carries the #76 survey-suppression key.
		body, err := agentloop.GeminiSettingsBody(c0Endpoint, c0Bearer)
		if err != nil {
			return c.contractFail(adapterName, AdapterContractViolation,
				fmt.Sprintf("agy gemini settings body render failed: %v", err), nil)
		}
		if !geminiSettingsSuppressesSurvey(body) {
			return c.contractFail(adapterName, AdapterContractViolation,
				fmt.Sprintf("agy settings body must set usageStatisticsEnabled:false (#76), got %s", body),
				map[string]string{"argv0": argv0})
		}
		if !strings.Contains(string(body), `"httpUrl"`) {
			return c.contractFail(adapterName, AdapterContractViolation,
				fmt.Sprintf("agy settings body must declare the striatum mcpServers httpUrl entry, got %s", body),
				map[string]string{"argv0": argv0})
		}
	case "codex", "claude":
		// codex/claude take the bootstrap as a trailing positional prompt.
		if len(argv) == 0 || argv[len(argv)-1] != c0BootstrapPrompt {
			return c.contractFail(adapterName, AdapterContractViolation,
				fmt.Sprintf("%s bootstrap must be appended as a trailing positional prompt, got argv %v", argv0, argv),
				map[string]string{"argv0": argv0})
		}
		// (3a) codex injects the per-launch MCP URL override. The agent-loop
		// produces "-c mcp_servers.striatum.url=\"<endpoint>\"" (mcpconfig.go);
		// assert the exported override token has the documented shape and carries
		// the live endpoint, so a refactor dropping the per-launch URL override is
		// caught hermetically.
		if argv0 == "codex" {
			override := agentloop.CodexMCPURLOverrideArg(c0Endpoint)
			if !codexURLOverrideWellFormed(override, c0Endpoint) {
				return c.contractFail(adapterName, AdapterContractViolation,
					fmt.Sprintf("codex MCP URL override is malformed: %q (want mcp_servers.striatum.url=%q)", override, c0Endpoint),
					map[string]string{"argv0": argv0})
			}
		}
	default:
		return c.contractFail(adapterName, AdapterContractViolation,
			fmt.Sprintf("unexpected adapter argv0 %q for the C0 golden; the declared adapters are claude/codex/agy", argv0),
			map[string]string{"argv0": argv0})
	}

	return c.pass(adapterName, fmt.Sprintf("argv bootstrap delivery confirmed for %s", argv0))
}

// codexURLOverrideWellFormed confirms the per-launch codex MCP URL override
// (DESIGN §1.4; mcpconfig.go injectLaneMCPConfig codex branch) has the
// documented shape and carries the endpoint. The agent-loop is the single
// source of the override string (CodexMCPURLOverrideArg), so this asserts the
// construction without duplicating its logic.
func codexURLOverrideWellFormed(override, endpoint string) bool {
	return strings.HasPrefix(override, "mcp_servers.striatum.url=") &&
		strings.Contains(override, endpoint)
}

// geminiSettingsSuppressesSurvey reports whether the agy settings body sets
// usageStatisticsEnabled to false (#76).
func geminiSettingsSuppressesSurvey(body []byte) bool {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	v, ok := parsed["usageStatisticsEnabled"]
	if !ok {
		return false
	}
	enabled, ok := v.(bool)
	return ok && !enabled
}
