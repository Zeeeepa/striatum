package agentloop

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAdapterBootstrapContractArgvAdapters asserts the exported accessor mirrors
// the production bootstrapDeliveryModeFor + appendBootstrapArgv for the three
// real adapters: argv delivery, with agy via --prompt-interactive and
// codex/claude as a trailing positional. This is the surface the hermetic C0
// conformance golden asserts.
func TestAdapterBootstrapContractArgvAdapters(t *testing.T) {
	const prompt = "PROMPT-BODY"

	for _, argv0 := range []string{"claude", "codex", "agy"} {
		mode, argv := AdapterBootstrapContract([]string{argv0}, prompt)
		if mode != BootstrapDeliveryArgv {
			t.Errorf("%s mode = %q, want %q", argv0, mode, BootstrapDeliveryArgv)
		}
		switch argv0 {
		case "agy":
			if len(argv) < 2 || argv[len(argv)-2] != "--prompt-interactive" || argv[len(argv)-1] != prompt {
				t.Errorf("agy argv = %v, want trailing --prompt-interactive %q", argv, prompt)
			}
		default:
			if argv[len(argv)-1] != prompt {
				t.Errorf("%s argv = %v, want trailing positional %q", argv0, argv, prompt)
			}
		}
	}
}

// TestAdapterBootstrapContractPTYSubmit asserts an unknown adapter resolves to
// pty_submit and leaves argv unchanged (the prompt is typed at runtime).
func TestAdapterBootstrapContractPTYSubmit(t *testing.T) {
	mode, argv := AdapterBootstrapContract([]string{"some-other-cli", "--flag"}, "PROMPT")
	if mode != BootstrapDeliveryPTYSubmit {
		t.Fatalf("mode = %q, want %q", mode, BootstrapDeliveryPTYSubmit)
	}
	if strings.Join(argv, " ") != "some-other-cli --flag" {
		t.Fatalf("pty_submit argv should be unchanged, got %v", argv)
	}
}

// TestBootstrapDeliveryModeForExported asserts the exported mode accessor
// matches the unexported production function.
func TestBootstrapDeliveryModeForExported(t *testing.T) {
	cases := map[string]string{
		"claude":         BootstrapDeliveryArgv,
		"codex":          BootstrapDeliveryArgv,
		"agy":            BootstrapDeliveryArgv,
		"some-other-cli": BootstrapDeliveryPTYSubmit,
	}
	for argv0, want := range cases {
		if got := BootstrapDeliveryModeFor([]string{argv0}); got != want {
			t.Errorf("BootstrapDeliveryModeFor(%q) = %q, want %q", argv0, got, want)
		}
	}
	if got := BootstrapDeliveryModeFor(nil); got != BootstrapDeliveryPTYSubmit {
		t.Errorf("empty command should map to pty_submit, got %q", got)
	}
}

// TestCodexMCPURLOverrideArg asserts the exported override token matches the
// shape injectLaneMCPConfig produces for codex.
func TestCodexMCPURLOverrideArg(t *testing.T) {
	const endpoint = "http://127.0.0.1:42727/mcp"
	got := CodexMCPURLOverrideArg(endpoint)
	want := `mcp_servers.striatum.url="http://127.0.0.1:42727/mcp"`
	if got != want {
		t.Fatalf("override = %q, want %q", got, want)
	}
}

func TestCodexMCPBearerTokenEnvOverrideArg(t *testing.T) {
	got := CodexMCPBearerTokenEnvOverrideArg()
	want := `mcp_servers.striatum.bearer_token_env_var="STRIATUM_MCP_TOKEN"`
	if got != want {
		t.Fatalf("override = %q, want %q", got, want)
	}
}

func TestCodexProjectTrustOverrideArg(t *testing.T) {
	got := CodexProjectTrustOverrideArg(`/tmp/striatum repo "quoted"`)
	want := `projects."/tmp/striatum repo \"quoted\"".trust_level="trusted"`
	if got != want {
		t.Fatalf("override = %q, want %q", got, want)
	}
}

// TestGeminiSettingsBody asserts the rendered agy settings body carries the
// striatum mcpServers entry and the #76 survey-suppression key, without touching
// the filesystem.
func TestGeminiSettingsBody(t *testing.T) {
	body, err := GeminiSettingsBody("http://127.0.0.1:1/mcp", "tok")
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		MCPServers map[string]struct {
			HTTPURL string            `json:"httpUrl"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
		UsageStatisticsEnabled bool `json:"usageStatisticsEnabled"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("settings body json: %v", err)
	}
	s, ok := parsed.MCPServers["striatum"]
	if !ok || s.HTTPURL != "http://127.0.0.1:1/mcp" || s.Headers["Authorization"] != "Bearer tok" {
		t.Fatalf("settings body striatum entry wrong: %s", body)
	}
	if parsed.UsageStatisticsEnabled {
		t.Fatalf("settings body must set usageStatisticsEnabled:false (#76): %s", body)
	}
}
