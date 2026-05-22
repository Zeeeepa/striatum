package agentloop

import (
	"strings"
	"testing"
)

func TestAgentEnvironmentExportsEndpointTokenAndRepositoryID(t *testing.T) {
	env := AgentEnvironment([]string{"PATH=/bin", "STRIATUM_MCP_URL=http://old"}, BootstrapContext{
		SocketPath:   "/tmp/striatum.sock",
		RepoRoot:     "/repo",
		RepositoryID: "repo_1",
		RunID:        "run_1",
		SessionID:    "sess_1",
		Endpoint:     "http://127.0.0.1:1234/mcp/sse",
		Token:        TokenMaterial{Token: "tok", Source: EnvMCPToken},
	})

	assertEnvValue(t, env, EnvMCPURL, "http://127.0.0.1:1234/mcp/sse")
	assertEnvValue(t, env, EnvRepositoryID, "repo_1")
	assertEnvValue(t, env, "STRIATUM_REPO", "/repo")
	assertEnvValue(t, env, "STRIATUM_REPO_ROOT", "/repo")
	assertEnvValue(t, env, "STRIATUM_RUN_ID", "run_1")
	assertEnvValue(t, env, "STRIATUM_SESSION_ID", "sess_1")
	assertEnvValue(t, env, EnvDaemonSocket, "/tmp/striatum.sock")
	assertEnvValue(t, env, EnvMCPToken, "tok")
}

func TestAgentEnvironmentDoesNotInventRepositoryIDFromRepoRoot(t *testing.T) {
	env := AgentEnvironment(nil, BootstrapContext{
		RepoRoot: "/repo",
		Endpoint: "http://127.0.0.1:1234/mcp/sse",
	})
	if value, ok := envLookup(env, EnvRepositoryID); ok {
		t.Fatalf("%s = %q, want unset without a registered repository id", EnvRepositoryID, value)
	}
}

func TestAgentEnvironmentPrefersTokenFileOverPlaintextTokenEnv(t *testing.T) {
	env := AgentEnvironment(nil, BootstrapContext{
		Endpoint: "http://127.0.0.1:1234/mcp/sse",
		Token:    TokenMaterial{Token: "tok", Source: "/runtime/client-token"},
	})
	if value, ok := envLookup(env, EnvMCPToken); ok {
		t.Fatalf("%s = %q, want unset when token file is available", EnvMCPToken, value)
	}
	assertEnvValue(t, env, EnvMCPTokenFile, "/runtime/client-token")
}

func TestBuildBootstrapPromptNamesNativeMCPBoundary(t *testing.T) {
	prompt := BuildBootstrapPrompt(BootstrapContext{
		RepoRoot:     "/repo",
		RepositoryID: "repo_1",
		RunID:        "run_1",
		SessionID:    "sess_1",
		Endpoint:     "http://127.0.0.1:1234/mcp/sse",
		Token:        TokenMaterial{Source: "/runtime/client-token"},
	})
	for _, want := range []string{
		"Use the local Striatum MCP server at http://127.0.0.1:1234/mcp/sse.",
		"Use repository_id repo_1",
		"work.await_packet",
		"session.report",
		"instead of waiting silently in terminal text",
		"will not claim, complete, release, or spoon-feed packet JSON",
		"STRIATUM_MCP_TOKEN_FILE",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("bootstrap prompt missing %q:\n%s", want, prompt)
		}
	}
}

func assertEnvValue(t *testing.T, env []string, key string, want string) {
	t.Helper()
	got, ok := envLookup(env, key)
	if !ok {
		t.Fatalf("%s not found in environment", key)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
