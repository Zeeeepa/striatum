package agentloop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/halbritt/striatum/go/pkg/admin"
)

type TokenMaterial struct {
	Token  string
	Source string
}

func ResolveTokenMaterial(repoRoot string, env []string) (TokenMaterial, error) {
	if token, ok := envLookup(env, EnvMCPToken); ok && strings.TrimSpace(token) != "" {
		return TokenMaterial{Token: strings.TrimSpace(token), Source: EnvMCPToken}, nil
	}

	if path, ok := envLookup(env, EnvMCPTokenFile); ok && strings.TrimSpace(path) != "" {
		token, err := ReadTokenFile(path)
		if err != nil {
			return TokenMaterial{}, fmt.Errorf("read MCP token file %s: %w", path, err)
		}
		return TokenMaterial{Token: token, Source: path}, nil
	}

	if runtimeDir, ok := envLookup(env, EnvDaemonRuntimeDir); ok && strings.TrimSpace(runtimeDir) != "" {
		path := filepath.Join(runtimeDir, "client-token")
		token, found, err := readOptionalTokenFile(path)
		if err != nil || found {
			return TokenMaterial{Token: token, Source: path}, err
		}
	} else if path, err := admin.RuntimeTokenPath(); err == nil {
		token, found, err := readOptionalTokenFile(path)
		if err != nil || found {
			return TokenMaterial{Token: token, Source: path}, err
		}
	}

	if repoRoot == "" {
		return TokenMaterial{}, nil
	}
	path := filepath.Join(repoRoot, ".striatum", "capability_token")
	token, found, err := readOptionalTokenFile(path)
	if err != nil || found {
		return TokenMaterial{Token: token, Source: path}, err
	}
	return TokenMaterial{}, nil
}

func ReadRuntimeToken(repoRoot string) (string, error) {
	tokenPath := filepath.Join(repoRoot, ".striatum", "capability_token")
	token, found, err := readOptionalTokenFile(tokenPath)
	if err != nil || found {
		return token, err
	}
	return "", nil
}

func readOptionalTokenFile(tokenPath string) (string, bool, error) {
	token, err := ReadTokenFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return token, true, nil
}

func ReadTokenFile(tokenPath string) (string, error) {
	info, err := os.Stat(tokenPath)
	if err != nil {
		return "", err
	}

	mode := info.Mode() & 0777
	if mode&0077 != 0 {
		return "", errors.New("daemon token file is not owner-only")
	}

	content, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}
