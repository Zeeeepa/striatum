package installers

import (
	"os"
	"path/filepath"
)

// scopeBase resolves the directory a bundle is rooted at for a given scope.
// user scope roots at the user's home (overridable for tests); project scope
// roots at the target repo.
func scopeBase(target, home, scope string) (string, error) {
	if scope == "user" {
		base := home
		if base == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = h
		}
		return filepath.Abs(base)
	}
	return filepath.Abs(target)
}
