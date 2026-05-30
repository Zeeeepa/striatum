package reads

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/agentloop"
)

// runtimeClientTokenPath returns the daemon-advertised runtime client-token path
// (a variable so tests can stub it without touching the real home dir).
var runtimeClientTokenPath = admin.RuntimeTokenPath

// codexDoctorBlock reports whether ~/.codex/config.toml is wired to the live
// (rotating) daemon MCP endpoint, so an operator who launches `codex` outside a
// supervised lane discovers a stale endpoint or a missing bearer BEFORE codex
// silently fails to reach the daemon (#64). It is advisory only: every condition
// surfaces as a warning string (returned via the doctor "warnings" channel),
// never as a hard `problems` failure, and the token VALUE is never read or
// returned — only its presence/absence is reported.
//
// The home dir and env are resolved from the daemon process, which is the same
// local user on the same host as the operator's codex CLI (local-first), so the
// paths it inspects are the operator's real ~/.codex/config.toml and runtime
// endpoint file.
func codexDoctorBlock() (map[string]any, []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return map[string]any{"checked": false, "reason": "cannot resolve home directory"}, nil
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	block := map[string]any{
		"checked":     true,
		"config_path": configPath,
	}

	body, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No codex config at all: nothing to be stale, no warning.
			block["config_present"] = false
			return block, nil
		}
		block["config_present"] = false
		block["reason"] = "read config.toml failed"
		return block, nil
	}
	block["config_present"] = true

	configURL, hasStriatumServer := codexStriatumURL(string(body))
	block["references_striatum"] = hasStriatumServer

	// Resolve the live endpoint the same way the daemon advertises it (env →
	// endpoint file → runtime dir). If we cannot resolve it here we cannot judge
	// staleness, so we stay quiet rather than emit a misleading warning.
	liveEndpoint, liveErr := agentloop.ResolveMCPEndpoint("", os.Environ())
	if liveErr == nil {
		block["live_endpoint"] = liveEndpoint
	}

	warnings := []string{}

	if hasStriatumServer && liveErr == nil {
		if !codexEndpointMatches(configURL, liveEndpoint) {
			block["stale"] = true
			warnings = append(warnings, "codex_config_stale: ~/.codex/config.toml mcp_servers.striatum.url="+configURL+
				" does not match the live daemon MCP endpoint "+liveEndpoint+
				"; run `striatum codex` (it injects the live endpoint) or update config.toml")
		} else {
			block["stale"] = false
		}
	}

	// Bearer presence: codex reads STRIATUM_MCP_TOKEN from its env. Warn (without
	// reading the value) when neither the env var nor a runtime client-token file
	// is available, so codex would start with no bearer for the striatum server.
	tokenPresent, tokenSource := codexTokenPresence()
	block["token_present"] = tokenPresent
	if tokenSource != "" {
		block["token_source"] = tokenSource
	}
	if hasStriatumServer && !tokenPresent {
		warnings = append(warnings, "codex_token_absent: no STRIATUM_MCP_TOKEN in the environment and no runtime client-token file found; "+
			"codex will reach the striatum MCP server without a bearer. Launch via `striatum codex` to inject it.")
	}

	return block, warnings
}

// codexStriatumURL extracts the configured mcp_servers.striatum.url from codex
// config.toml. It tolerates both the dotted-key inline form
// (`mcp_servers.striatum.url = "..."`) and the section form
// (`[mcp_servers.striatum]` ... `url = "..."`). It returns the URL (may be empty)
// and whether a striatum MCP server is referenced at all.
func codexStriatumURL(body string) (string, bool) {
	referenced := false
	url := ""
	inStriatumSection := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(stripTOMLComment(raw))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			// Section header. Normalize [mcp_servers.striatum] /
			// [mcp_servers."striatum"] and array-of-tables [[...]].
			header := strings.Trim(line, "[]")
			header = strings.ReplaceAll(header, "\"", "")
			header = strings.ReplaceAll(header, " ", "")
			inStriatumSection = header == "mcp_servers.striatum"
			if inStriatumSection {
				referenced = true
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Dotted-key inline form anywhere in the file.
		if key == "mcp_servers.striatum.url" || key == `mcp_servers."striatum".url` {
			referenced = true
			url = trimTOMLString(value)
			continue
		}
		if strings.HasPrefix(key, "mcp_servers.striatum") {
			referenced = true
		}
		// Section-relative url key.
		if inStriatumSection && key == "url" {
			url = trimTOMLString(value)
		}
	}
	return url, referenced
}

// codexEndpointMatches compares a config URL against the live endpoint, ignoring
// a trailing-path suffix difference (codex may store the bare base URL while the
// live advertisement carries the /mcp/sse path) so we do not flag a benign shape
// difference as drift. Host:port mismatch IS drift.
func codexEndpointMatches(configURL, liveEndpoint string) bool {
	c := strings.TrimRight(strings.TrimSpace(configURL), "/")
	l := strings.TrimRight(strings.TrimSpace(liveEndpoint), "/")
	if c == "" {
		return false
	}
	if c == l {
		return true
	}
	// Compare scheme://host[:port] prefixes so an endpoint-path-only difference
	// (".../mcp/sse" vs base) is not treated as a stale endpoint.
	return codexAuthority(c) == codexAuthority(l) && codexAuthority(c) != ""
}

// codexAuthority returns scheme://host[:port] for an http(s) URL, or "" if it
// does not look like one.
func codexAuthority(raw string) string {
	idx := strings.Index(raw, "://")
	if idx < 0 {
		return ""
	}
	scheme := raw[:idx]
	rest := raw[idx+3:]
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		rest = rest[:slash]
	}
	if rest == "" {
		return ""
	}
	return scheme + "://" + rest
}

// codexTokenPresence reports whether codex would find a bearer, WITHOUT reading
// or returning the token value: it checks the STRIATUM_MCP_TOKEN env var, then
// STRIATUM_MCP_TOKEN_FILE, then the runtime client-token file path. Only
// presence and the source location (a path or env name) are returned.
func codexTokenPresence() (bool, string) {
	if strings.TrimSpace(os.Getenv(agentloop.EnvMCPToken)) != "" {
		return true, agentloop.EnvMCPToken
	}
	if path := strings.TrimSpace(os.Getenv(agentloop.EnvMCPTokenFile)); path != "" {
		if fileNonEmpty(path) {
			return true, path
		}
	}
	if path, err := runtimeClientTokenPath(); err == nil && fileNonEmpty(path) {
		return true, path
	}
	return false, ""
}

func fileNonEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func stripTOMLComment(line string) string {
	// Drop an unquoted trailing comment. Good enough for config.toml values,
	// which for our keys are quoted URLs.
	inQuote := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQuote = !inQuote
		case '#':
			if !inQuote {
				return line[:i]
			}
		}
	}
	return line
}

func trimTOMLString(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
	value = strings.TrimSuffix(strings.TrimPrefix(value, `'`), `'`)
	return strings.TrimSpace(value)
}
