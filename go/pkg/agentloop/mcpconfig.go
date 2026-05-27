package agentloop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// injectLaneMCPConfig gives an agent-loop lane CLI a striatum MCP server
// pointed at the live, env-resolved endpoint + token (RFC 0088 Decision 5).
// The config is generated FRESH at launch into an ephemeral 0600 file under
// the target repo's .striatum scratch and removed when the lane exits — the
// rotating endpoint is never persisted to a repo-tracked or gitignored file,
// which makes the F45 stale-port class structurally impossible.
//
// P1 wires claude (the baseline): `--mcp-config <file> --strict-mcp-config`
// loads ONLY the striatum server and ignores any stale global ~/.claude.json
// entry. Other adapters (codex, agy) are passed through unchanged until P2/P3.
func injectLaneMCPConfig(command []string, repoRoot, endpoint string, token TokenMaterial) ([]string, func(), error) {
	noop := func() {}
	if len(command) == 0 || strings.TrimSpace(endpoint) == "" || strings.TrimSpace(token.Token) == "" {
		return command, noop, nil
	}
	switch laneAdapterName(command[0]) {
	case "claude":
		path, cleanup, err := writeEphemeralMCPConfig(repoRoot, endpoint, token.Token)
		if err != nil {
			return command, noop, err
		}
		out := append([]string(nil), command...)
		out = append(out, "--mcp-config", path, "--strict-mcp-config")
		return out, cleanup, nil
	default:
		return command, noop, nil
	}
}

// laneAdapterName maps a (possibly absolute) argv0 to a bare adapter name.
func laneAdapterName(arg0 string) string {
	base := filepath.Base(strings.TrimSpace(arg0))
	base = strings.TrimSuffix(base, ".exe")
	return base
}

func writeEphemeralMCPConfig(repoRoot, endpoint, bearer string) (string, func(), error) {
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"striatum": map[string]any{
				"type":    "http",
				"url":     endpoint,
				"headers": map[string]any{"Authorization": "Bearer " + bearer},
			},
		},
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return "", func() {}, err
	}
	dir := filepath.Join(repoRoot, ".striatum", "scratch")
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "lane-mcp-config-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("create ephemeral mcp config: %w", err)
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if err := os.Chmod(path, 0o600); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if _, err := f.Write(body); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}
