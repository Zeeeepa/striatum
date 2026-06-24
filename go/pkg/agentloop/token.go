package agentloop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

// adminTokenReachedByNonOwner reports whether the credential-resolution chain has
// reached the owner-only admin runtime client-token while THIS process is NOT its
// owner — the RFC 0143 Slice A Spot-1 narrowing (#512). It NEVER reads the token
// contents; detection is local process state only:
//
//   - the file exists and its owner uid != this process euid           -> refuse; or
//   - stat is denied with EACCES/EPERM on the file or its 0700 parent  -> refuse.
//
// A missing file (ENOENT) is NOT the floor: it returns false so the caller falls
// through to the next resolver tier exactly as today. When the file owner uid ==
// euid (the OWNER/operator process) it returns false and the caller reads the token
// normally — the owner is unaffected. This is a NARROWING: it removes a read step
// for a non-owner lane (returning the typed ErrUnrecoverableAcrossRotation sentinel
// → reserved exit 97), and adds NO new read path. It never widens admin-token
// exposure; the owner-mode guard in ReadTokenFile is retained independently.
func adminTokenReachedByNonOwner(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false // ENOENT: not the floor; fall through to the next tier.
		}
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
			// The 0700 owner-only parent dir denied even the stat to a non-owner:
			// the chain has reached the admin token but this process cannot own it.
			return true
		}
		// Any other stat error is indeterminate — do NOT claim the floor; let the
		// existing read attempt surface the real error unchanged.
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false // platform without uid metadata: leave behavior unchanged.
	}
	return int(stat.Uid) != os.Geteuid()
}
