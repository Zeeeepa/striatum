package main

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/agentloop"
)

// envWithLiveEndpointAndToken builds an env that makes the agentloop resolvers
// find a live endpoint (via STRIATUM_MCP_URL) and a token (via
// STRIATUM_MCP_TOKEN) without touching the filesystem or runtime dir.
func envWithLiveEndpointAndToken(endpoint, token string) []string {
	return []string{
		agentloop.EnvMCPURL + "=" + endpoint,
		agentloop.EnvMCPToken + "=" + token,
		"PATH=/usr/bin",
	}
}

func TestBuildCodexInvocationInjectsEndpointAndToken(t *testing.T) {
	env := envWithLiveEndpointAndToken("http://127.0.0.1:8973/mcp/sse", "secret-bearer")

	plan, err := buildCodexInvocation([]string{"exec", "--full-auto"}, "", env)
	if err != nil {
		t.Fatalf("buildCodexInvocation: %v", err)
	}

	// argv[0] is the program name; the endpoint override and passthrough follow.
	if len(plan.Args) < 4 || plan.Args[0] != "codex" || plan.Args[1] != "-c" {
		t.Fatalf("unexpected args: %#v", plan.Args)
	}
	if !strings.HasPrefix(plan.Args[2], "mcp_servers.striatum.url=") {
		t.Fatalf("override token missing: %q", plan.Args[2])
	}
	if !strings.Contains(plan.Args[2], "127.0.0.1:8973") {
		t.Fatalf("override does not carry live endpoint: %q", plan.Args[2])
	}
	if plan.Args[len(plan.Args)-2] != "exec" || plan.Args[len(plan.Args)-1] != "--full-auto" {
		t.Fatalf("passthrough args not appended: %#v", plan.Args)
	}

	// STRIATUM_MCP_TOKEN must be present in the child env, exactly once.
	count := 0
	var tokenVal string
	for _, e := range plan.Env {
		if strings.HasPrefix(e, agentloop.EnvMCPToken+"=") {
			count++
			tokenVal = strings.TrimPrefix(e, agentloop.EnvMCPToken+"=")
		}
	}
	if count != 1 {
		t.Fatalf("STRIATUM_MCP_TOKEN appears %d times in child env, want 1", count)
	}
	if tokenVal != "secret-bearer" {
		t.Fatalf("child token = %q, want secret-bearer", tokenVal)
	}
}

func TestBuildCodexInvocationReplacesStaleTokenEnv(t *testing.T) {
	// A stale STRIATUM_MCP_TOKEN already in env must be replaced (not duplicated)
	// by the resolved live bearer. Here the resolver reads the token from the
	// same env var, so the value stays — but the dedupe/replace path is exercised.
	env := []string{
		agentloop.EnvMCPURL + "=http://127.0.0.1:9000/mcp/sse",
		agentloop.EnvMCPToken + "=live-bearer",
	}
	plan, err := buildCodexInvocation(nil, "", env)
	if err != nil {
		t.Fatalf("buildCodexInvocation: %v", err)
	}
	count := 0
	for _, e := range plan.Env {
		if strings.HasPrefix(e, agentloop.EnvMCPToken+"=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("token env count = %d, want exactly 1", count)
	}
}

func TestBuildCodexInvocationErrorsWithoutEndpoint(t *testing.T) {
	// No endpoint resolvable and an empty repo root: must error rather than launch
	// codex pointed at nothing. Point the runtime dir at an empty temp dir so no
	// real daemon endpoint file on the dev host bleeds in.
	env := []string{
		agentloop.EnvMCPToken + "=t",
		agentloop.EnvDaemonRuntimeDir + "=" + t.TempDir(),
	}
	_, err := buildCodexInvocation(nil, "", env)
	if err == nil {
		t.Fatal("expected error when no MCP endpoint can be resolved")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("error should mention endpoint, got: %v", err)
	}
}

func TestBuildCodexInvocationErrorsWithoutToken(t *testing.T) {
	// Endpoint resolvable but no token (no env token, empty repo root, and a
	// runtime dir override pointing at an empty dir so no client-token file).
	env := []string{
		agentloop.EnvMCPURL + "=http://127.0.0.1:9000/mcp/sse",
		agentloop.EnvDaemonRuntimeDir + "=" + t.TempDir(),
	}
	_, err := buildCodexInvocation(nil, "", env)
	if err == nil {
		t.Fatal("expected error when no token can be resolved")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Fatalf("error should mention token, got: %v", err)
	}
}

// The launcher must never embed the token value in the displayable fields.
func TestBuildCodexInvocationDoesNotLeakTokenInDisplayFields(t *testing.T) {
	env := envWithLiveEndpointAndToken("http://127.0.0.1:8973/mcp/sse", "super-secret")
	plan, err := buildCodexInvocation(nil, "", env)
	if err != nil {
		t.Fatalf("buildCodexInvocation: %v", err)
	}
	if strings.Contains(plan.Endpoint, "super-secret") || strings.Contains(plan.TokenSource, "super-secret") {
		t.Fatalf("token leaked into display fields: endpoint=%q source=%q", plan.Endpoint, plan.TokenSource)
	}
	for _, a := range plan.Args {
		if strings.Contains(a, "super-secret") {
			t.Fatalf("token leaked into argv: %q", a)
		}
	}
}

func TestUpsertEnv(t *testing.T) {
	got := upsertEnv([]string{"A=1", "B=2", "A=stale"}, "A", "new")
	var aCount int
	for _, e := range got {
		if strings.HasPrefix(e, "A=") {
			aCount++
			if e != "A=new" {
				t.Fatalf("A not replaced: %q", e)
			}
		}
	}
	if aCount != 1 {
		t.Fatalf("A appears %d times, want 1: %v", aCount, got)
	}
	// Append when absent.
	got2 := upsertEnv([]string{"X=1"}, "Y", "2")
	if got2[len(got2)-1] != "Y=2" {
		t.Fatalf("Y not appended: %v", got2)
	}
}
