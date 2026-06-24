package agentloop

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/admin"
)

const (
	EnvMCPURL                    = "STRIATUM_MCP_URL"
	EnvDaemonMCPHTTPURL          = "STRIATUM_DAEMON_MCP_HTTP_URL"
	EnvDaemonMCPHTTPEndpointFile = "STRIATUM_DAEMON_MCP_HTTP_ENDPOINT_FILE"
	EnvDaemonMCPHTTPAddr         = "STRIATUM_DAEMON_MCP_HTTP_ADDR"
	EnvMCPAddr                   = "STRIATUM_MCP_ADDR"
	EnvMCPPort                   = "STRIATUM_MCP_PORT"
	EnvDaemonMCPHTTPPort         = "STRIATUM_DAEMON_MCP_HTTP_PORT"
	EnvDaemonRuntimeDir          = "STRIATUM_DAEMON_RUNTIME_DIR"
)

func ResolveMCPEndpoint(repoRoot string, env []string) (string, error) {
	for _, key := range []string{EnvMCPURL, EnvDaemonMCPHTTPURL} {
		if value, ok := envLookup(env, key); ok && strings.TrimSpace(value) != "" {
			return normalizeMCPEndpoint(value)
		}
	}

	if path, ok := envLookup(env, EnvDaemonMCPHTTPEndpointFile); ok && strings.TrimSpace(path) != "" {
		endpoint, found, err := endpointFromTextFile(path, true)
		if err != nil || found {
			return endpoint, err
		}
	}

	if runtimeDir, ok := envLookup(env, EnvDaemonRuntimeDir); ok && strings.TrimSpace(runtimeDir) != "" {
		endpoint, found, err := endpointFromRuntimeDir(runtimeDir)
		if err != nil || found {
			return endpoint, err
		}
	} else if runtimeDir, err := admin.RuntimeDir(); err == nil {
		endpoint, found, err := endpointFromRuntimeDir(runtimeDir)
		if err != nil || found {
			return endpoint, err
		}
	}

	if repoRoot != "" {
		endpoint, found, err := endpointFromRepoMetadata(repoRoot)
		if err != nil || found {
			return endpoint, err
		}
	}

	for _, key := range []string{EnvDaemonMCPHTTPAddr, EnvMCPAddr} {
		if value, ok := envLookup(env, key); ok && strings.TrimSpace(value) != "" {
			return normalizeMCPEndpoint(value)
		}
	}

	for _, key := range []string{EnvMCPPort, EnvDaemonMCPHTTPPort} {
		if value, ok := envLookup(env, key); ok && strings.TrimSpace(value) != "" {
			return endpointFromPort(value)
		}
	}

	return "", fmt.Errorf("agent-loop requires %s, %s, %s, %s, or a daemon MCP endpoint file", EnvMCPURL, EnvDaemonMCPHTTPURL, EnvDaemonMCPHTTPAddr, EnvMCPPort)
}

// ResolveMCPEndpointFresh re-reads the daemon's runtime MCP endpoint file
// directly, deliberately ignoring any env-baked launch literal (STRIATUM_MCP_URL
// / STRIATUM_DAEMON_MCP_HTTP_URL). It is the #323 rotation-recovery reader: when
// a daemon restarts mid-run its MCP HTTP listener rotates to a fresh dynamic
// port and rewrites the runtime endpoint file, but the supervised helper that
// survived the restart (#141) still holds the dead launch-time port. Calling
// this returns whatever endpoint the daemon last wrote on disk so the lane can
// detect the rotation and reconnect.
//
// Source order mirrors ResolveMCPEndpoint's daemon-file tier: an explicit
// endpoint-file pointer (STRIATUM_DAEMON_MCP_HTTP_ENDPOINT_FILE) wins, then the
// runtime dir (env override or admin.RuntimeDir()). The token-bearing env
// values are intentionally NOT consulted — the whole point is to bypass the
// cached launch literal.
func ResolveMCPEndpointFresh(repoRoot string) (string, error) {
	env := os.Environ()
	if path, ok := envLookup(env, EnvDaemonMCPHTTPEndpointFile); ok && strings.TrimSpace(path) != "" {
		endpoint, found, err := endpointFromTextFile(path, true)
		if err != nil || found {
			return endpoint, err
		}
	}
	if runtimeDir, ok := envLookup(env, EnvDaemonRuntimeDir); ok && strings.TrimSpace(runtimeDir) != "" {
		endpoint, found, err := endpointFromRuntimeDir(runtimeDir)
		if err != nil || found {
			return endpoint, err
		}
	} else if path, err := admin.RuntimeMCPEndpointPath(); err == nil {
		endpoint, found, err := endpointFromTextFile(path, false)
		if err != nil || found {
			return endpoint, err
		}
	}
	return "", fmt.Errorf("agent-loop fresh MCP endpoint unavailable: no runtime endpoint file")
}

// ResolveTokenMaterialFresh re-reads the daemon runtime client token directly,
// ignoring any env-baked launch literal. The token does NOT rotate on a normal
// daemon restart (only the endpoint does), so on the #323 path this typically
// returns the same bearer the lane already holds; re-reading it keeps the
// reconnect path correct if the operator ever re-minted the runtime token. It
// reuses the same session-bound bearer the lane uses (#135/#296) — the runtime
// client-token file — never a different credential.
func ResolveTokenMaterialFresh(repoRoot string) (TokenMaterial, error) {
	env := os.Environ()
	if path, ok := envLookup(env, EnvMCPTokenFile); ok && strings.TrimSpace(path) != "" {
		token, found, err := readOptionalTokenFile(path)
		if err != nil || found {
			return TokenMaterial{Token: token, Source: path}, err
		}
	}
	if runtimeDir, ok := envLookup(env, EnvDaemonRuntimeDir); ok && strings.TrimSpace(runtimeDir) != "" {
		path := filepath.Join(runtimeDir, "client-token")
		if adminTokenReachedByNonOwner(path) {
			return TokenMaterial{}, ErrUnrecoverableAcrossRotation
		}
		token, found, err := readOptionalTokenFile(path)
		if err != nil || found {
			return TokenMaterial{Token: token, Source: path}, err
		}
	} else if path, err := admin.RuntimeTokenPath(); err == nil {
		if adminTokenReachedByNonOwner(path) {
			return TokenMaterial{}, ErrUnrecoverableAcrossRotation
		}
		token, found, err := readOptionalTokenFile(path)
		if err != nil || found {
			return TokenMaterial{Token: token, Source: path}, err
		}
	}
	return TokenMaterial{}, nil
}

func endpointFromRuntimeDir(runtimeDir string) (string, bool, error) {
	for _, name := range []string{"mcp-http-endpoint", "mcp-http-url"} {
		endpoint, found, err := endpointFromTextFile(filepath.Join(runtimeDir, name), false)
		if err != nil || found {
			return endpoint, found, err
		}
	}
	return "", false, nil
}

func normalizeMCPEndpoint(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("empty MCP endpoint")
	}
	if strings.HasPrefix(value, ":") {
		value = "127.0.0.1" + value
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid MCP endpoint %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid MCP endpoint %q: scheme must be http or https", raw)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid MCP endpoint %q: missing host", raw)
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/mcp/sse"
	}
	return parsed.String(), nil
}

func endpointFromPort(raw string) (string, error) {
	port := strings.TrimSpace(raw)
	if port == "" {
		return "", fmt.Errorf("empty MCP port")
	}
	if strings.Contains(port, ":") {
		return normalizeMCPEndpoint(port)
	}
	return normalizeMCPEndpoint(net.JoinHostPort("127.0.0.1", port))
}

func endpointFromTextFile(path string, required bool) (string, bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read MCP endpoint file %s: %w", path, err)
	}
	endpoint, err := normalizeMCPEndpoint(string(body))
	if err != nil {
		return "", false, fmt.Errorf("read MCP endpoint file %s: %w", path, err)
	}
	return endpoint, true, nil
}

func endpointFromRepoMetadata(repoRoot string) (string, bool, error) {
	path := filepath.Join(repoRoot, ".striatum", "daemon-mcp.json")
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read repo MCP metadata %s: %w", path, err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(body, &metadata); err != nil {
		return "", false, fmt.Errorf("read repo MCP metadata %s: %w", path, err)
	}
	for _, key := range []string{"sse_url", "mcp_sse_url", "mcp_url", "url"} {
		if value, ok := metadata[key].(string); ok && strings.TrimSpace(value) != "" {
			endpoint, err := normalizeMCPEndpoint(value)
			if err != nil {
				return "", false, fmt.Errorf("read repo MCP metadata %s field %s: %w", path, key, err)
			}
			return endpoint, true, nil
		}
	}
	return "", false, fmt.Errorf("read repo MCP metadata %s: missing sse_url", path)
}
