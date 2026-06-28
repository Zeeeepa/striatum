package mutations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/laneproviderauth"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func enforceLaneCredentialDomain(_ context.Context, config supervisionStartConfig) error {
	provider := strings.ToLower(strings.TrimSpace(config.adapterName()))
	switch provider {
	case laneproviderauth.ProviderClaude, laneproviderauth.ProviderCodex:
	default:
		return nil
	}
	env := laneCredentialGuardEnv(config)
	candidates, err := laneproviderauth.ResolveCredentialCandidates(provider, laneproviderauth.KindOAuth, env)
	if err != nil && !errors.Is(err, laneproviderauth.ErrResolverMismatch) {
		return err
	}
	for _, candidate := range candidates {
		if pathInsideRepo(filepath.Dir(candidate.Path), config.RepoRoot) {
			return rpc.NewError("lane_credential_cache_inside_repo", fmt.Sprintf("%s resolves %s credential cache inside the target repository", provider, candidate.EnvKey), map[string]any{
				"provider": provider,
				"env_key":  nullableString(candidate.EnvKey),
				"path":     candidate.Path,
			})
		}
	}
	for key, value := range config.LaunchEnv {
		if laneproviderauth.ModelsCredentialSelector(provider, key) {
			continue
		}
		if !providerOwnedCredentialSelector(provider, key) {
			continue
		}
		if pathInsideRepo(value, config.RepoRoot) {
			return rpc.NewError("lane_uncovered_credential_selector_inside_repo", fmt.Sprintf("%s command_env selector %s is provider-owned credential/cache state but has no resolver model", provider, key), map[string]any{
				"provider": provider,
				"env_key":  key,
				"path":     value,
			})
		}
	}
	return nil
}

func laneCredentialGuardEnv(config supervisionStartConfig) []string {
	base := []string{}
	if strings.TrimSpace(config.RunAsUser) != "" {
		base = laneUserIdentityEnv(config.RunAsUser)
	}
	return applyLaneLaunchEnv(config, base)
}

func providerOwnedCredentialSelector(provider, key string) bool {
	key = strings.ToUpper(strings.TrimSpace(key))
	switch provider {
	case laneproviderauth.ProviderClaude:
		if key == "CLAUDE_HOME" || key == "ANTHROPIC_HOME" {
			return true
		}
		if strings.HasPrefix(key, "CLAUDE_") || strings.HasPrefix(key, "ANTHROPIC_") {
			return strings.HasSuffix(key, "_CONFIG_DIR") ||
				strings.HasSuffix(key, "_CACHE_DIR") ||
				strings.HasSuffix(key, "_CREDENTIAL_DIR") ||
				strings.HasSuffix(key, "_CREDENTIALS_DIR") ||
				strings.HasSuffix(key, "_SECURESTORAGE_CONFIG_DIR")
		}
	case laneproviderauth.ProviderCodex:
		if key == "CODEX_HOME" {
			return true
		}
		if strings.HasPrefix(key, "CODEX_") {
			return strings.HasSuffix(key, "_CONFIG_DIR") ||
				strings.HasSuffix(key, "_CACHE_DIR") ||
				strings.HasSuffix(key, "_CREDENTIAL_DIR") ||
				strings.HasSuffix(key, "_CREDENTIALS_DIR") ||
				strings.HasSuffix(key, "_HOME")
		}
	}
	return false
}

func pathInsideRepo(path, repoRoot string) bool {
	path = strings.TrimSpace(path)
	repoRoot = strings.TrimSpace(repoRoot)
	if path == "" || repoRoot == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
	}
	root := cleanOrEval(repoRoot)
	candidate := cleanOrEval(path)
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}

func cleanOrEval(path string) string {
	path = filepath.Clean(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	if real, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		return filepath.Join(real, filepath.Base(path))
	}
	return path
}
