package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/halbritt/striatum/go/pkg/agentloop"
)

// codexInvocation is the resolved plan for launching codex against the live
// daemon MCP endpoint, separated from the exec so arg/env assembly is testable.
type codexInvocation struct {
	// Args is the full argv passed to codex, including the injected
	// `-c mcp_servers.striatum.*=...` overrides and any passthrough args.
	Args []string
	// Env is the child environment with STRIATUM_MCP_TOKEN set; the token value
	// is never logged or printed.
	Env []string
	// Endpoint is the resolved live MCP endpoint (safe to display).
	Endpoint string
	// TokenSource describes where the bearer was read from (a path or env name,
	// never the token value).
	TokenSource string
}

// runCodex implements `striatum codex [args...]`: it resolves the live (rotating)
// daemon MCP endpoint and the runtime client-token the same way the daemon
// advertises them, then execs codex with the endpoint and bearer env-var key
// injected via codex's per-key TOML overrides and the bearer supplied to the
// child through STRIATUM_MCP_TOKEN. This removes the manual `export
// STRIATUM_MCP_TOKEN=... && edit ~/.codex/config.toml` dance an operator
// otherwise repeats every key rotation (#64/#225). The token value is never
// printed.
// Extra args pass through to codex unchanged.
func runCodex(args []string, stdout io.Writer, stderr io.Writer, repoRootOverride string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			// Only intercept help when it is the sole/leading intent; otherwise
			// pass through so `codex <sub> --help` still works.
			if len(args) == 1 {
				printCodexUsage(stdout)
				return 0
			}
		}
	}

	plan, err := buildCodexInvocation(args, repoRootOverride, os.Environ())
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}

	codexPath, err := exec.LookPath("codex")
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "codex not found on PATH; install the codex CLI first")
		return 127
	}

	// Surface what we wired up (endpoint + token source) WITHOUT the token value,
	// so the operator can confirm they are pointed at the live daemon.
	_, _ = fmt.Fprintf(stderr, "striatum codex: mcp endpoint %s (token from %s)\n", plan.Endpoint, plan.TokenSource)

	cmd := exec.Command(codexPath, plan.Args[1:]...)
	cmd.Env = plan.Env
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		_, _ = fmt.Fprintf(stderr, "codex failed to launch: %v\n", err)
		return 1
	}
	return 0
}

// buildCodexInvocation resolves the endpoint + token and assembles the codex
// argv and child env without execing, so the wiring is unit-testable. It refuses
// (error) when no live endpoint or token can be resolved, because launching codex
// pointed at nothing is worse than a clear message.
func buildCodexInvocation(passthrough []string, repoRoot string, env []string) (codexInvocation, error) {
	endpoint, err := agentloop.ResolveMCPEndpoint(repoRoot, env)
	if err != nil {
		return codexInvocation{}, fmt.Errorf("resolve live MCP endpoint: %w; start the daemon (`striatum daemon status`) or set %s", err, agentloop.EnvMCPURL)
	}

	material, err := agentloop.ResolveTokenMaterial(repoRoot, env)
	if err != nil {
		return codexInvocation{}, fmt.Errorf("resolve MCP client token: %w", err)
	}
	if strings.TrimSpace(material.Token) == "" {
		return codexInvocation{}, fmt.Errorf("no MCP client token found; start the daemon to mint the runtime client-token or set %s", agentloop.EnvMCPToken)
	}
	tokenSource := material.Source
	if tokenSource == "" {
		tokenSource = agentloop.EnvMCPToken
	}

	// codex has no --mcp-config; it overrides ~/.codex/config.toml per key. The
	// values are quoted so shell-significant characters are passed literally as
	// TOML override values.
	codexArgs := []string{
		"codex",
		"-c", agentloop.CodexMCPURLOverrideArg(endpoint),
		"-c", agentloop.CodexMCPBearerTokenEnvOverrideArg(),
	}
	codexArgs = append(codexArgs, passthrough...)

	// Inject STRIATUM_MCP_TOKEN into the child env, replacing any stale value so
	// the live (possibly rotated) bearer always wins. Never logged.
	childEnv := upsertEnv(env, agentloop.EnvMCPToken, material.Token)

	return codexInvocation{
		Args:        codexArgs,
		Env:         childEnv,
		Endpoint:    endpoint,
		TokenSource: tokenSource,
	}, nil
}

// upsertEnv returns a copy of env with key set to value exactly once: the first
// matching entry is replaced in place and any further duplicates for key are
// dropped, so the child env carries a single authoritative value.
func upsertEnv(env []string, key string, value string) []string {
	out := make([]string, 0, len(env)+1)
	prefix := key + "="
	replaced := false
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			if replaced {
				continue
			}
			out = append(out, prefix+value)
			replaced = true
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func printCodexUsage(out io.Writer) {
	_, _ = fmt.Fprintln(out, "usage: striatum codex [codex args ...]")
	_, _ = fmt.Fprintln(out, "launch codex wired to the live daemon MCP endpoint:")
	_, _ = fmt.Fprintln(out, "  - injects -c mcp_servers.striatum.url=<live endpoint> (overrides ~/.codex/config.toml)")
	_, _ = fmt.Fprintln(out, "  - injects -c mcp_servers.striatum.bearer_token_env_var=STRIATUM_MCP_TOKEN")
	_, _ = fmt.Fprintln(out, "  - sets STRIATUM_MCP_TOKEN in codex's env from the runtime client-token (never printed)")
	_, _ = fmt.Fprintln(out, "extra args pass through to codex unchanged.")
}
