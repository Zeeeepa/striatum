package agentloop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ReadRuntimeToken(repoRoot string) (string, error) {
	tokenPath := filepath.Join(repoRoot, ".striatum", "capability_token")

	info, err := os.Stat(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	mode := info.Mode() & 0777
	if mode&0077 != 0 {
		return "", errors.New("daemon token fallback file is not owner-only")
	}

	content, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}
