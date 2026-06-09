package agentloop

import (
	"encoding/json"
	"fmt"
)

// This file exposes a small, deliberately tight accessor surface so the
// hermetic adapter-conformance harness (go/pkg/adapterconformance) can assert
// the bootstrap-delivery contract over THIS package's real construction logic
// without duplicating it. It adds NO behavior: every function below is a thin,
// read-only projection of the existing unexported helpers
// (bootstrapDeliveryModeFor, appendBootstrapArgv, injectLaneMCPConfig,
// writeEphemeralGeminiSettings). RFC 0101 Layer 2, Slice 1 (C0 golden).

// Bootstrap delivery mode constants, exported for the conformance golden so it
// can assert the declared mode without re-deriving the strings.
const (
	// BootstrapDeliveryArgv means the adapter accepts its bootstrap prompt via
	// argv and submits it itself (claude/codex trailing positional; agy via
	// --prompt-interactive). This is the #101 fix: a revert to pty_submit for
	// these adapters re-introduces the TUI-buffering blocker.
	BootstrapDeliveryArgv = string(bootstrapDeliveryArgv)
	// BootstrapDeliveryPTYSubmit means the bootstrap is typed into the TUI and
	// followed by a submit key-sequence (the default for unknown adapters).
	BootstrapDeliveryPTYSubmit = string(bootstrapDeliveryPTYSubmit)
)

// BootstrapDeliveryModeFor reports the bootstrap-delivery mode the production
// agent-loop would use for the given lane command. It is the exported form of
// bootstrapDeliveryModeFor; the conformance C0 golden asserts that claude,
// codex, and agy resolve to BootstrapDeliveryArgv (a revert to pty_submit is
// the #101 regression and MUST fail C0).
func BootstrapDeliveryModeFor(command []string) string {
	return string(bootstrapDeliveryModeFor(command))
}

// AdapterBootstrapContract reports, for the given lane command, the
// bootstrap-delivery mode the agent-loop would use and the argv it would launch
// with once the bootstrap prompt is attached (for argv-delivery adapters). For
// pty_submit adapters the returned argv is the command unchanged (the prompt is
// typed into the PTY at runtime, not placed on argv). This is a pure projection
// of bootstrapDeliveryModeFor + appendBootstrapArgv; it performs no I/O and
// mutates nothing.
//
// The conformance C0 golden uses this to assert:
//   - claude/codex/agy → mode == BootstrapDeliveryArgv;
//   - agy → bootstrap appended as "--prompt-interactive <prompt>";
//   - codex/claude → bootstrap appended as a trailing positional prompt.
func AdapterBootstrapContract(command []string, prompt string) (mode string, argv []string) {
	deliveryMode := bootstrapDeliveryModeFor(command)
	if deliveryMode == bootstrapDeliveryArgv {
		return string(deliveryMode), appendBootstrapArgv(command, prompt)
	}
	return string(deliveryMode), append([]string(nil), command...)
}

// CodexMCPURLOverrideArg returns the exact "-c mcp_servers.striatum.url=..."
// override argument the agent-loop injects into a codex lane command for the
// given endpoint (mcpconfig.go injectLaneMCPConfig codex branch). The
// conformance golden asserts this token is present in the prepared codex lane
// command so a refactor that drops the per-launch URL override is caught
// hermetically.
func CodexMCPURLOverrideArg(endpoint string) string {
	return fmt.Sprintf(`mcp_servers.striatum.url=%q`, endpoint)
}

// CodexMCPBearerTokenEnvOverrideArg returns the exact "-c
// mcp_servers.striatum.bearer_token_env_var=..." override argument the
// agent-loop injects into a codex lane command so Codex reads the supervised
// lane bearer from STRIATUM_MCP_TOKEN even when ~/.codex/config.toml is stale or
// incomplete.
func CodexMCPBearerTokenEnvOverrideArg() string {
	return codexMCPBearerTokenEnvOverrideArg()
}

// CodexProjectTrustOverrideArg returns the per-launch "-c projects.<repo>.trust_level"
// override injected into codex lane commands. It is best-effort; current Codex
// builds may still render the workspace-trust TUI, where the PTY responder is
// the launch backstop.
func CodexProjectTrustOverrideArg(repoRoot string) string {
	return codexProjectTrustOverrideArg(repoRoot)
}

// GeminiSettingsBody renders the ephemeral agy (.gemini/settings.json) body the
// agent-loop would write for the given endpoint and bearer, WITHOUT touching
// the filesystem. It mirrors the settings map constructed in
// writeEphemeralGeminiSettings (no pre-existing project settings), so the
// conformance golden can assert the #76 survey-suppression key
// (usageStatisticsEnabled:false) and the striatum mcpServers entry are present
// without spawning agy or writing into a work tree.
func GeminiSettingsBody(endpoint, bearer string) ([]byte, error) {
	settings := map[string]any{
		"mcpServers": map[string]any{
			"striatum": map[string]any{
				"httpUrl": endpoint,
				"headers": map[string]any{"Authorization": "Bearer " + bearer},
			},
		},
		// #76: disable the gemini-cli usage/feedback survey so a supervised agy
		// lane does not stall on it inside the PTY.
		"usageStatisticsEnabled": false,
	}
	return json.Marshal(settings)
}
