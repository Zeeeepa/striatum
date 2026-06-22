package laneproviderauth

// RFC 0162 F2 fold — the provider-agnostic credential-resolver contract.
//
// "Reuse the codex ResolveAuthHome shape" was REFUSED for non-codex lanes: it
// resolves bare HOME, not where a non-codex CLI resolves its token. This resolver
// names, per in-scope provider, the exact runtime credential source + precedence
// the lane CLI actually resolves, tied to the launch-env keys that select it. It
// reads the credential the lane's LAUNCH ENV resolves — never a fresher HOME
// decoy: when a provider's config-dir env key is present it wins, and HOME is only
// the documented fallback for providers that genuinely resolve from HOME.
//
// It FAILS CLOSED: when the runtime source cannot be proven (no resolver entry for
// the provider, or no resolvable path from the launch env), it returns
// ErrResolverMismatch so the sampler emits a pageable resolver_mismatch rather
// than a green gauge from a fallback/decoy path.

import (
	"errors"
	"path/filepath"
	"strings"
)

// Launch-env keys the resolver consults. These are the credential-home /
// config-dir keys the lane CLI itself reads to locate its token.
const (
	EnvCodexHome       = "CODEX_HOME"
	EnvClaudeConfigDir = "CLAUDE_CONFIG_DIR"
	EnvHome            = "HOME"

	ProviderClaude = "claude"
)

// ErrResolverMismatch is the fail-closed sentinel: the runtime credential source
// for a lane cannot be proven. The sampler maps it to a
// striatum_lane_cred_resolver_mismatch series (which pages), and emits NO
// seconds_to_expiry and NO sample_present for that lane.
var ErrResolverMismatch = errors.New("lane credential resolver: runtime credential source cannot be proven")

// ResolvedCredential is the resolver-proven credential location for a lane: the
// file the lane CLI resolves from its launch env, plus the launch-env key that
// selected it (empty when resolved from the HOME default).
type ResolvedCredential struct {
	Provider string
	Kind     string
	Path     string
	EnvKey   string
}

// ResolveCredential returns the credential file the lane CLI resolves from its
// LAUNCH ENV per the provider's precedence, or ErrResolverMismatch (fail-closed)
// when the runtime source cannot be proven. provider/kind are the closed-enum
// roster values; launchEnv is the lane process's launch environment (KEY=VALUE
// entries). For codex/claude a present config-dir env key wins over the HOME
// default — so a fresher HOME decoy is never preferred over the launch-env-
// resolved credential (F2 / FA-F2).
func ResolveCredential(provider, kind string, launchEnv []string) (ResolvedCredential, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	kind = strings.ToLower(strings.TrimSpace(kind))
	values := envValues(launchEnv)

	switch provider {
	case ProviderCodex:
		if home := strings.TrimSpace(values[EnvCodexHome]); home != "" {
			return ResolvedCredential{
				Provider: provider, Kind: kind,
				Path:   filepath.Join(filepath.Clean(home), CodexAuthFileName),
				EnvKey: EnvCodexHome,
			}, nil
		}
		if home := strings.TrimSpace(values[EnvHome]); home != "" {
			return ResolvedCredential{
				Provider: provider, Kind: kind,
				Path: filepath.Join(filepath.Clean(home), ".codex", CodexAuthFileName),
			}, nil
		}
		return ResolvedCredential{}, ErrResolverMismatch
	case ProviderClaude:
		if dir := strings.TrimSpace(values[EnvClaudeConfigDir]); dir != "" {
			return ResolvedCredential{
				Provider: provider, Kind: kind,
				Path:   filepath.Join(filepath.Clean(dir), ClaudeCredentialFileName),
				EnvKey: EnvClaudeConfigDir,
			}, nil
		}
		if home := strings.TrimSpace(values[EnvHome]); home != "" {
			return ResolvedCredential{
				Provider: provider, Kind: kind,
				Path: filepath.Join(filepath.Clean(home), ".claude", ClaudeCredentialFileName),
			}, nil
		}
		return ResolvedCredential{}, ErrResolverMismatch
	default:
		// No resolver entry for this provider — fail closed rather than guess a
		// path (which would risk reading a decoy as green).
		return ResolvedCredential{}, ErrResolverMismatch
	}
}

// ClaudeCredentialFileName is the file the claude CLI resolves its OAuth token
// from, under $CLAUDE_CONFIG_DIR (else $HOME/.claude/).
const ClaudeCredentialFileName = ".credentials.json"
