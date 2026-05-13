package db

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const EnvDaemonDBURL = "STRIATUM_DAEMON_DB_URL"

type Config struct {
	URL        string
	Source     string
	ConfigPath string
}

func ResolveConfig(explicitURL string) Config {
	configPath := DefaultConfigPath()
	if explicitURL != "" {
		return Config{URL: explicitURL, Source: "--postgres-url", ConfigPath: configPath}
	}
	if envURL := os.Getenv(EnvDaemonDBURL); envURL != "" {
		return Config{URL: envURL, Source: EnvDaemonDBURL, ConfigPath: configPath}
	}
	if fileURL := readConfigURL(configPath); fileURL != "" {
		return Config{URL: fileURL, Source: configPath, ConfigPath: configPath}
	}
	return Config{ConfigPath: configPath}
}

func DefaultConfigPath() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "striatum", "daemon.toml")
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "striatum", "daemon.toml")
}

func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		if _, ok := parsed.User.Password(); ok {
			parsed.User = url.UserPassword(username, "<redacted>")
		}
	}
	query := parsed.Query()
	for _, key := range []string{"password", "pass", "token", "sslpassword"} {
		if _, ok := query[key]; ok {
			query.Set(key, "<redacted>")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

type Runner interface {
	Exec(ctx context.Context, sql string) error
	QueryScalar(ctx context.Context, sql string) (string, error)
}

type Pool struct {
	URL    string
	Runner Runner
}

func Connect(ctx context.Context, postgresURL string) (*Pool, error) {
	if postgresURL == "" {
		return nil, errors.New("daemon PostgreSQL URL is not configured")
	}
	runner := PsqlRunner{URL: postgresURL}
	if _, err := runner.QueryScalar(ctx, "SELECT 1"); err != nil {
		return nil, err
	}
	return &Pool{URL: postgresURL, Runner: runner}, nil
}

func ConnectAndMigrate(ctx context.Context, postgresURL string, daemonVersion string) (*Pool, int, error) {
	config := ResolveConfig(postgresURL)
	if config.URL == "" {
		return nil, 0, errors.New("daemon PostgreSQL URL is not configured")
	}
	pool, err := Connect(ctx, config.URL)
	if err != nil {
		return nil, 0, err
	}
	version, err := ApplyMigrations(ctx, pool.Runner, daemonVersion)
	if err != nil {
		return nil, 0, err
	}
	return pool, version, nil
}

type PsqlRunner struct {
	URL string
}

func (r PsqlRunner) Exec(ctx context.Context, sql string) error {
	cmd := exec.CommandContext(ctx, "psql", r.URL, "-v", "ON_ERROR_STOP=1", "-q", "-c", sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(output)) + ": " + err.Error())
	}
	return nil
}

func (r PsqlRunner) QueryScalar(ctx context.Context, sql string) (string, error) {
	cmd := exec.CommandContext(ctx, "psql", r.URL, "-v", "ON_ERROR_STOP=1", "-qAt", "-c", sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.New(strings.TrimSpace(string(output)) + ": " + err.Error())
	}
	return strings.TrimSpace(string(output)), nil
}

func readConfigURL(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "postgres_url") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		}
	}
	return ""
}
