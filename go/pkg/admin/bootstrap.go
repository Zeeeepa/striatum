package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const EnvRuntimeDir = "STRIATUM_DAEMON_RUNTIME_DIR"

var bootstrapCapabilities = []rpc.Capability{
	rpc.CapabilityAdmin,
	rpc.CapabilityRead,
	rpc.CapabilityWrite,
	rpc.CapabilityClaim,
	rpc.CapabilityReview,
	rpc.CapabilityApply,
	rpc.CapabilityRecovery,
	rpc.CapabilitySurgicalRecovery,
}

func RuntimeTokenPath() (string, error) {
	runtimeDir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "client-token"), nil
}

func RuntimeMCPEndpointPath() (string, error) {
	runtimeDir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "mcp-http-endpoint"), nil
}

func RuntimeDir() (string, error) {
	if override := os.Getenv(EnvRuntimeDir); override != "" {
		return absExpandUser(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for daemon runtime: %w", err)
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "striatum", "runtime"), nil
	}
	if base := os.Getenv("XDG_RUNTIME_DIR"); base != "" {
		return filepath.Join(base, "striatum"), nil
	}
	return filepath.Join(home, ".cache", "striatum", "runtime"), nil
}

func BootstrapRuntimeTokenIfNeeded(ctx context.Context, runner db.Runner, tokenPath string) (map[string]any, error) {
	if runner == nil {
		return nil, fmt.Errorf("bootstrap admin token requires daemon PostgreSQL")
	}
	count, err := clientCount(ctx, runner)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil
	}
	tx, err := runner.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackQuietly(ctx, tx)
	if err := tx.Exec(ctx, "LOCK TABLE striatumd.clients IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return nil, err
	}
	count, err = clientCount(ctx, tx)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}

	now := time.Now().UTC()
	grants := make([]tokenGrant, 0, len(bootstrapCapabilities))
	for _, capability := range bootstrapCapabilities {
		grants = append(grants, tokenGrant{Capability: capability})
	}
	result, err := insertTokenClient(ctx, tx, now, "cli", "bootstrap-admin", grants, nil)
	if err != nil {
		return nil, err
	}
	token, ok := result["token"].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("bootstrap token generation returned an empty token")
	}
	if err := writeRuntimeToken(tokenPath, token); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = os.Remove(tokenPath)
		return nil, err
	}
	return map[string]any{
		"client_id": result["client_id"],
		"token_id":  result["token_id"],
		"token":     token,
	}, nil
}

func clientCount(ctx context.Context, runner interface {
	QueryRow(context.Context, string, ...any) db.Row
}) (int64, error) {
	var count int64
	if err := runner.QueryRow(ctx, "SELECT COUNT(*) FROM striatumd.clients").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func writeRuntimeToken(path string, token string) error {
	if path == "" {
		return fmt.Errorf("daemon runtime token path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".client-token-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(token + "\n"); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// WriteRuntimeToken restores the daemon runtime client token from already-issued
// token material. It does not mint authority; callers supply the token value.
func WriteRuntimeToken(path string, token string) error {
	return writeRuntimeToken(path, token)
}

func absExpandUser(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand runtime directory %q: %w", path, err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}
