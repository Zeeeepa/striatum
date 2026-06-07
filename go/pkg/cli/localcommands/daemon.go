package localcommands

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/db"
)

// EnvDaemonAdminDBURL is an optional owner/admin DSN used by `daemon migrate`
// to apply DDL the runtime role (striatumd_rw) cannot (RFC 0079 §5).
const EnvDaemonAdminDBURL = "STRIATUM_DAEMON_ADMIN_DB_URL"

//go:embed striatumd.service.tmpl
var systemdUnitTemplate string

const unitName = "striatumd.service"

// daemonTomlScaffold is written to ~/.config/striatum/daemon.toml only when the
// file is absent. It documents the single required key without committing the
// operator to a DSN, so it can never clobber a real configuration.
const daemonTomlScaffold = `# Striatum daemon configuration (scaffolded by ` + "`striatum daemon install`" + `).
# The daemon refuses to bind a socket without a Postgres DSN. Set one of:
#   - postgres_url below, or
#   - the STRIATUM_DAEMON_DB_URL environment variable (takes precedence).
#
# Example (adjust role, host, and database to your local Postgres):
# postgres_url = "postgres://striatum@localhost:5432/striatum?sslmode=disable"
`

// RunDaemon dispatches the local `striatum daemon {install|uninstall|status}`
// bootstrap helpers. These never touch daemon RPC routes; they manage the
// systemd user unit, scaffold daemon.toml, and report runtime layout.
func RunDaemon(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: striatum daemon {install|uninstall|status|migrate-db|owner-ddl} [flags]")
		return 2
	}
	switch args[0] {
	case "install":
		return runDaemonInstall(args[1:], stdout, stderr)
	case "uninstall":
		return runDaemonUninstall(args[1:], stdout, stderr)
	case "status":
		return runDaemonStatus(args[1:], stdout, stderr)
	case "migrate-db":
		return runDaemonMigrate(args[1:], stdout, stderr, version)
	case "owner-ddl":
		return runDaemonOwnerDDL(args[1:], stdout, stderr, version)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown daemon command: %s\n", args[0])
		return 2
	}
}

// runDaemonOwnerDDL applies the versioned owner-DDL bundles (RFC 0110 §8.1) as
// the database owner — authority registry, SECURITY DEFINER write functions,
// capability stamps, and the phased DML revokes the runtime role cannot perform.
// Owner DSN resolution mirrors `daemon migrate-db`: --owner-url (or --admin-url),
// then STRIATUM_DAEMON_ADMIN_DB_URL, then the normal daemon DSN. Applying is
// out-of-band and idempotent; re-running a stamped version is a no-op.
func runDaemonOwnerDDL(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) == 0 || args[0] != "apply" {
		_, _ = fmt.Fprintln(stderr, "usage: striatum daemon owner-ddl apply [--owner-url <dsn>] [--json]")
		return 2
	}
	ownerURL := ""
	jsonOutput := false
	for i := 1; i < len(args); i++ {
		key, value, hasValue := strings.Cut(args[i], "=")
		switch key {
		case "--owner-url", "--admin-url":
			if hasValue {
				ownerURL = value
			} else if i+1 < len(args) {
				i++
				ownerURL = args[i]
			}
		case "--json":
			jsonOutput = true
		default:
			_, _ = fmt.Fprintf(stderr, "unknown daemon owner-ddl flag: %s\n", args[i])
			return 2
		}
	}
	if ownerURL == "" {
		ownerURL = os.Getenv(EnvDaemonAdminDBURL)
	}
	cfg := db.ResolveConfig(ownerURL)
	if cfg.URL == "" {
		_, _ = fmt.Fprintln(stderr, "daemon owner-ddl: no Postgres DSN; pass --owner-url, set STRIATUM_DAEMON_ADMIN_DB_URL, or configure daemon.toml")
		return 1
	}
	if version == "" {
		version = "dev"
	}
	pool, err := db.Connect(context.Background(), cfg.URL, version)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "daemon owner-ddl connect failed: %v\n", err)
		return 1
	}
	defer pool.Close()
	applied, bundleVersion, err := db.ApplyOwnerBundles(context.Background(), pool.Runner, version)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "daemon owner-ddl apply failed: %v\n", err)
		return 1
	}
	// Re-applying a stamped bundle is a no-op, so grant-drift repair needs an
	// explicit re-assert: after the bundles, re-close every stamped write and
	// read surface (RFC 0110 §6 C-GRANT-DRIFT, RFC 0114). Re-running
	// `striatum daemon owner-ddl apply` is the documented drift-repair action.
	if err := db.ReassertWriteRevokes(context.Background(), pool.Runner); err != nil {
		_, _ = fmt.Fprintf(stderr, "daemon owner-ddl reassert write revokes failed: %v\n", err)
		return 1
	}
	if err := db.ReassertReadRevokes(context.Background(), pool.Runner); err != nil {
		_, _ = fmt.Fprintf(stderr, "daemon owner-ddl reassert read revokes failed: %v\n", err)
		return 1
	}
	if jsonOutput {
		return writeDaemonJSON(stdout, stderr, map[string]any{"ok": true, "data": map[string]any{
			"applied_versions": applied, "owner_bundle_version": bundleVersion, "dsn_source": cfg.Source,
		}})
	}
	if len(applied) == 0 {
		_, _ = fmt.Fprintf(stdout, "owner-ddl already current; owner_bundle_version=%d (dsn source: %s)\n", bundleVersion, cfg.Source)
	} else {
		_, _ = fmt.Fprintf(stdout, "owner-ddl applied versions %v; owner_bundle_version=%d (dsn source: %s)\n", applied, bundleVersion, cfg.Source)
	}
	return 0
}

// runDaemonMigrate applies pending PostgreSQL migrations using an owner/admin
// DSN, so DDL the runtime role (striatumd_rw) cannot perform — ALTERing or
// adding foreign keys against owner-held tables — is applied by the owner
// before the daemon serves (RFC 0079 §5). Admin DSN resolution: --admin-url,
// then STRIATUM_DAEMON_ADMIN_DB_URL, then the normal daemon DSN (flag/env/
// daemon.toml) as a fallback for additive migrations the runtime role can apply.
func runDaemonMigrate(args []string, stdout, stderr io.Writer, version string) int {
	adminURL := ""
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		key, value, hasValue := strings.Cut(args[i], "=")
		switch key {
		case "--admin-url":
			if hasValue {
				adminURL = value
			} else if i+1 < len(args) {
				i++
				adminURL = args[i]
			}
		case "--json":
			jsonOutput = true
		default:
			_, _ = fmt.Fprintf(stderr, "unknown daemon migrate flag: %s\n", args[i])
			return 2
		}
	}
	if adminURL == "" {
		adminURL = os.Getenv(EnvDaemonAdminDBURL)
	}
	// ResolveConfig("") falls back to STRIATUM_DAEMON_DB_URL / daemon.toml.
	cfg := db.ResolveConfig(adminURL)
	if cfg.URL == "" {
		_, _ = fmt.Fprintln(stderr, "daemon migrate: no Postgres DSN; pass --admin-url, set STRIATUM_DAEMON_ADMIN_DB_URL, or configure daemon.toml")
		return 1
	}
	if version == "" {
		version = "dev"
	}
	pool, schemaVersion, err := db.ConnectAndMigrate(context.Background(), cfg.URL, version)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "daemon migrate failed: %v\n", err)
		return 1
	}
	defer pool.Close()
	if jsonOutput {
		return writeDaemonJSON(stdout, stderr, map[string]any{"ok": true, "data": map[string]any{"schema_version": schemaVersion, "dsn_source": cfg.Source}})
	}
	_, _ = fmt.Fprintf(stdout, "migrations applied; schema_version=%d (dsn source: %s)\n", schemaVersion, cfg.Source)
	return 0
}

type daemonFlags struct {
	json      bool
	noStart   bool
	printUnit bool
}

func parseDaemonFlags(args []string) (daemonFlags, error) {
	var flags daemonFlags
	for _, arg := range args {
		switch arg {
		case "--json":
			flags.json = true
		case "--no-start":
			flags.noStart = true
		case "--print-unit":
			flags.printUnit = true
		default:
			return daemonFlags{}, fmt.Errorf("unknown flag: %s", arg)
		}
	}
	return flags, nil
}

// renderUnit returns the systemd user unit content. The template is static —
// portability comes from systemd %h/%t specifiers, not from substitution — but
// rendering through one function keeps a single source of truth.
func renderUnit() string {
	return systemdUnitTemplate
}

func runDaemonInstall(args []string, stdout, stderr io.Writer) int {
	flags, err := parseDaemonFlags(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if flags.printUnit {
		_, _ = fmt.Fprint(stdout, renderUnit())
		return 0
	}

	layout, err := resolveLayout()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}

	// Always scaffold daemon.toml when absent; this is host-agnostic and safe
	// regardless of whether systemd is present.
	tomlCreated, err := scaffoldDaemonTOML(layout.configTOML)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}

	if !systemdAvailable() {
		printForegroundRecipe(stdout, layout)
		if flags.json {
			return writeDaemonJSON(stdout, stderr, map[string]any{
				"ok": true,
				"data": map[string]any{
					"systemd":         false,
					"daemon_toml":     layout.configTOML,
					"daemon_toml_new": tomlCreated,
					"socket":          layout.socket,
				},
			})
		}
		return 0
	}

	if err := os.MkdirAll(filepath.Dir(layout.unitPath), 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "create unit directory: %v\n", err)
		return 1
	}
	if err := os.WriteFile(layout.unitPath, []byte(renderUnit()), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "write unit: %v\n", err)
		return 1
	}

	if err := systemctl(stderr, "daemon-reload"); err != nil {
		_, _ = fmt.Fprintf(stderr, "systemctl daemon-reload: %v\n", err)
		return 1
	}
	started := false
	if flags.noStart {
		if err := systemctl(stderr, "enable", unitName); err != nil {
			_, _ = fmt.Fprintf(stderr, "systemctl enable: %v\n", err)
			return 1
		}
	} else {
		if err := systemctl(stderr, "enable", "--now", unitName); err != nil {
			_, _ = fmt.Fprintf(stderr, "systemctl enable --now: %v\n", err)
			return 1
		}
		started = true
	}

	if flags.json {
		return writeDaemonJSON(stdout, stderr, map[string]any{
			"ok": true,
			"data": map[string]any{
				"systemd":         true,
				"unit_path":       layout.unitPath,
				"started":         started,
				"daemon_toml":     layout.configTOML,
				"daemon_toml_new": tomlCreated,
				"socket":          layout.socket,
				"token":           layout.token,
				"mcp_endpoint":    layout.mcpEndpoint,
			},
		})
	}

	_, _ = fmt.Fprintf(stdout, "installed unit: %s\n", layout.unitPath)
	if tomlCreated {
		_, _ = fmt.Fprintf(stdout, "scaffolded config: %s (set postgres_url before first start)\n", layout.configTOML)
	} else {
		_, _ = fmt.Fprintf(stdout, "config (unchanged): %s\n", layout.configTOML)
	}
	if started {
		_, _ = fmt.Fprintln(stdout, "daemon: enabled and started")
	} else {
		_, _ = fmt.Fprintln(stdout, "daemon: enabled (not started; --no-start)")
	}
	_, _ = fmt.Fprintf(stdout, "socket:       %s\n", layout.socket)
	_, _ = fmt.Fprintf(stdout, "token:        %s\n", layout.token)
	_, _ = fmt.Fprintf(stdout, "mcp endpoint: %s\n", layout.mcpEndpoint)
	return 0
}

func runDaemonUninstall(args []string, stdout, stderr io.Writer) int {
	flags, err := parseDaemonFlags(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 2
	}
	layout, err := resolveLayout()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if !systemdAvailable() {
		_, _ = fmt.Fprintln(stdout, "systemd not detected; nothing to uninstall.")
		_, _ = fmt.Fprintf(stdout, "If you ran the daemon in the foreground, stop that process. Config left at %s.\n", layout.configTOML)
		return 0
	}
	// Best-effort disable; ignore failures so uninstall is idempotent even when
	// the unit was never loaded.
	_ = systemctl(stderr, "disable", "--now", unitName)
	removed := false
	if err := os.Remove(layout.unitPath); err == nil {
		removed = true
	} else if !os.IsNotExist(err) {
		_, _ = fmt.Fprintf(stderr, "remove unit: %v\n", err)
		return 1
	}
	_ = systemctl(stderr, "daemon-reload")

	if flags.json {
		return writeDaemonJSON(stdout, stderr, map[string]any{
			"ok": true,
			"data": map[string]any{
				"unit_path":   layout.unitPath,
				"removed":     removed,
				"daemon_toml": layout.configTOML,
			},
		})
	}
	if removed {
		_, _ = fmt.Fprintf(stdout, "removed unit: %s\n", layout.unitPath)
	} else {
		_, _ = fmt.Fprintf(stdout, "no unit to remove at %s\n", layout.unitPath)
	}
	_, _ = fmt.Fprintf(stdout, "left config and data intact (%s)\n", layout.configTOML)
	return 0
}

func runDaemonStatus(args []string, stdout, stderr io.Writer) int {
	flags, err := parseDaemonFlags(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 2
	}
	layout, err := resolveLayout()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}

	unitInstalled := fileExists(layout.unitPath)
	enabled := ""
	active := ""
	if systemdAvailable() {
		enabled = systemctlOutput("is-enabled", unitName)
		active = systemctlOutput("is-active", unitName)
	}
	socketPresent := fileExists(layout.socket)
	configURL := db.ResolveConfig("")
	dsnConfigured := strings.TrimSpace(configURL.URL) != ""
	doctor := runDoctor()

	if flags.json {
		return writeDaemonJSON(stdout, stderr, map[string]any{
			"ok": true,
			"data": map[string]any{
				"systemd":        systemdAvailable(),
				"unit_path":      layout.unitPath,
				"unit_installed": unitInstalled,
				"enabled":        enabled,
				"active":         active,
				"socket":         layout.socket,
				"socket_present": socketPresent,
				"token":          layout.token,
				"mcp_endpoint":   layout.mcpEndpoint,
				"daemon_toml":    layout.configTOML,
				"dsn_configured": dsnConfigured,
				"dsn_source":     configURL.Source,
				"doctor":         doctor,
			},
		})
	}

	_, _ = fmt.Fprintln(stdout, "striatum daemon status")
	if systemdAvailable() {
		_, _ = fmt.Fprintf(stdout, "  unit:    %s (installed=%t, enabled=%s, active=%s)\n", layout.unitPath, unitInstalled, orDash(enabled), orDash(active))
	} else {
		_, _ = fmt.Fprintf(stdout, "  unit:    systemd not detected (foreground mode; unit path %s)\n", layout.unitPath)
	}
	_, _ = fmt.Fprintf(stdout, "  socket:  %s (present=%t)\n", layout.socket, socketPresent)
	_, _ = fmt.Fprintf(stdout, "  token:   %s\n", layout.token)
	_, _ = fmt.Fprintf(stdout, "  mcp:     %s\n", layout.mcpEndpoint)
	_, _ = fmt.Fprintf(stdout, "  config:  %s (dsn_configured=%t)\n", layout.configTOML, dsnConfigured)
	_, _ = fmt.Fprintf(stdout, "  doctor:  %s\n", doctor)
	return 0
}

type layout struct {
	unitPath    string
	configTOML  string
	socket      string
	token       string
	mcpEndpoint string
}

func resolveLayout() (layout, error) {
	runtimeDir, err := admin.RuntimeDir()
	if err != nil {
		return layout{}, err
	}
	token, err := admin.RuntimeTokenPath()
	if err != nil {
		return layout{}, err
	}
	endpoint, err := admin.RuntimeMCPEndpointPath()
	if err != nil {
		return layout{}, err
	}
	return layout{
		unitPath:    filepath.Join(configHome(), "systemd", "user", unitName),
		configTOML:  db.DefaultConfigPath(),
		socket:      filepath.Join(runtimeDir, "daemon-go.sock"),
		token:       token,
		mcpEndpoint: endpoint,
	}, nil
}

func configHome() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return base
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config"
	}
	return filepath.Join(home, ".config")
}

func scaffoldDaemonTOML(path string) (bool, error) {
	if fileExists(path) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(daemonTomlScaffold), 0o600); err != nil {
		return false, fmt.Errorf("scaffold daemon.toml: %w", err)
	}
	return true, nil
}

func systemdAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func systemctl(stderr io.Writer, args ...string) error {
	full := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", full...)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	return cmd.Run()
}

func systemctlOutput(args ...string) string {
	full := append([]string{"--user"}, args...)
	out, _ := exec.Command("systemctl", full...).Output()
	return strings.TrimSpace(string(out))
}

// runDoctor shells out to this same striatum binary so `daemon status` can fold
// the read-only daemon health check into one view without re-implementing it.
func runDoctor() string {
	self, err := os.Executable()
	if err != nil {
		return "unknown (cannot resolve striatum binary)"
	}
	out, err := exec.Command(self, "doctor").CombinedOutput()
	summary := strings.TrimSpace(string(out))
	if summary == "" {
		if err != nil {
			return fmt.Sprintf("unreachable (%v)", err)
		}
		return "ok"
	}
	// Collapse to the first line for the one-line status view.
	if idx := strings.IndexByte(summary, '\n'); idx >= 0 {
		summary = summary[:idx]
	}
	if err != nil {
		return summary + " (exit error)"
	}
	return summary
}

func printForegroundRecipe(stdout io.Writer, l layout) {
	_, _ = fmt.Fprintln(stdout, "systemd user services not detected on this host.")
	_, _ = fmt.Fprintln(stdout, "Run the daemon in the foreground instead:")
	_, _ = fmt.Fprintf(stdout, "  1. Set a Postgres DSN in %s (postgres_url) or export STRIATUM_DAEMON_DB_URL.\n", l.configTOML)
	_, _ = fmt.Fprintf(stdout, "  2. striatumd -socket %s\n", l.socket)
	_, _ = fmt.Fprintln(stdout, "  3. In another shell, run `striatum doctor` to confirm health.")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func writeDaemonJSON(stdout, stderr io.Writer, payload map[string]any) int {
	encoded, err := json.Marshal(payload)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	_, _ = fmt.Fprintln(stdout, string(encoded))
	return 0
}
