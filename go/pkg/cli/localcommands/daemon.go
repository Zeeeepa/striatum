package localcommands

import (
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
func RunDaemon(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: striatum daemon {install|uninstall|status} [flags]")
		return 2
	}
	switch args[0] {
	case "install":
		return runDaemonInstall(args[1:], stdout, stderr)
	case "uninstall":
		return runDaemonUninstall(args[1:], stdout, stderr)
	case "status":
		return runDaemonStatus(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown daemon command: %s\n", args[0])
		return 2
	}
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
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if flags.printUnit {
		fmt.Fprint(stdout, renderUnit())
		return 0
	}

	layout, err := resolveLayout()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	// Always scaffold daemon.toml when absent; this is host-agnostic and safe
	// regardless of whether systemd is present.
	tomlCreated, err := scaffoldDaemonTOML(layout.configTOML)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
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
		fmt.Fprintf(stderr, "create unit directory: %v\n", err)
		return 1
	}
	if err := os.WriteFile(layout.unitPath, []byte(renderUnit()), 0o644); err != nil {
		fmt.Fprintf(stderr, "write unit: %v\n", err)
		return 1
	}

	if err := systemctl(stderr, "daemon-reload"); err != nil {
		fmt.Fprintf(stderr, "systemctl daemon-reload: %v\n", err)
		return 1
	}
	started := false
	if flags.noStart {
		if err := systemctl(stderr, "enable", unitName); err != nil {
			fmt.Fprintf(stderr, "systemctl enable: %v\n", err)
			return 1
		}
	} else {
		if err := systemctl(stderr, "enable", "--now", unitName); err != nil {
			fmt.Fprintf(stderr, "systemctl enable --now: %v\n", err)
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

	fmt.Fprintf(stdout, "installed unit: %s\n", layout.unitPath)
	if tomlCreated {
		fmt.Fprintf(stdout, "scaffolded config: %s (set postgres_url before first start)\n", layout.configTOML)
	} else {
		fmt.Fprintf(stdout, "config (unchanged): %s\n", layout.configTOML)
	}
	if started {
		fmt.Fprintln(stdout, "daemon: enabled and started")
	} else {
		fmt.Fprintln(stdout, "daemon: enabled (not started; --no-start)")
	}
	fmt.Fprintf(stdout, "socket:       %s\n", layout.socket)
	fmt.Fprintf(stdout, "token:        %s\n", layout.token)
	fmt.Fprintf(stdout, "mcp endpoint: %s\n", layout.mcpEndpoint)
	return 0
}

func runDaemonUninstall(args []string, stdout, stderr io.Writer) int {
	flags, err := parseDaemonFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	layout, err := resolveLayout()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if !systemdAvailable() {
		fmt.Fprintln(stdout, "systemd not detected; nothing to uninstall.")
		fmt.Fprintf(stdout, "If you ran the daemon in the foreground, stop that process. Config left at %s.\n", layout.configTOML)
		return 0
	}
	// Best-effort disable; ignore failures so uninstall is idempotent even when
	// the unit was never loaded.
	_ = systemctl(stderr, "disable", "--now", unitName)
	removed := false
	if err := os.Remove(layout.unitPath); err == nil {
		removed = true
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "remove unit: %v\n", err)
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
		fmt.Fprintf(stdout, "removed unit: %s\n", layout.unitPath)
	} else {
		fmt.Fprintf(stdout, "no unit to remove at %s\n", layout.unitPath)
	}
	fmt.Fprintf(stdout, "left config and data intact (%s)\n", layout.configTOML)
	return 0
}

func runDaemonStatus(args []string, stdout, stderr io.Writer) int {
	flags, err := parseDaemonFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	layout, err := resolveLayout()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
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

	fmt.Fprintln(stdout, "striatum daemon status")
	if systemdAvailable() {
		fmt.Fprintf(stdout, "  unit:    %s (installed=%t, enabled=%s, active=%s)\n", layout.unitPath, unitInstalled, orDash(enabled), orDash(active))
	} else {
		fmt.Fprintf(stdout, "  unit:    systemd not detected (foreground mode; unit path %s)\n", layout.unitPath)
	}
	fmt.Fprintf(stdout, "  socket:  %s (present=%t)\n", layout.socket, socketPresent)
	fmt.Fprintf(stdout, "  token:   %s\n", layout.token)
	fmt.Fprintf(stdout, "  mcp:     %s\n", layout.mcpEndpoint)
	fmt.Fprintf(stdout, "  config:  %s (dsn_configured=%t)\n", layout.configTOML, dsnConfigured)
	fmt.Fprintf(stdout, "  doctor:  %s\n", doctor)
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
	fmt.Fprintln(stdout, "systemd user services not detected on this host.")
	fmt.Fprintln(stdout, "Run the daemon in the foreground instead:")
	fmt.Fprintf(stdout, "  1. Set a Postgres DSN in %s (postgres_url) or export STRIATUM_DAEMON_DB_URL.\n", l.configTOML)
	fmt.Fprintf(stdout, "  2. striatumd -socket %s\n", l.socket)
	fmt.Fprintln(stdout, "  3. In another shell, run `striatum doctor` to confirm health.")
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
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}
