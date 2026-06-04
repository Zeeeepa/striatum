// Package skills wires the `striatum skills install` and `striatum plugin
// install/uninstall` local commands to the installers package. These are
// local (non-daemon) commands: they render embedded template bundles into the
// user/project skills or plugin directories and never touch daemon state.
package skills

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/halbritt/striatum/go/pkg/installers"
)

// Run dispatches a `skills`/`plugin` command. args is the full command slice
// beginning with "skills" or "plugin". repoRoot is the resolved target repo
// (empty → current working directory). version stamps written manifests.
func Run(args []string, stdout, stderr io.Writer, repoRoot, version string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: striatum {skills|plugin} ...")
		return 2
	}
	if len(args) < 2 || isHelpArg(args[1]) {
		out := stderr
		rc := 2
		if len(args) >= 2 {
			out = stdout
			rc = 0
		}
		writeUsage(out, args[0])
		return rc
	}
	repo := repoRoot
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err.Error())
			return 1
		}
		repo = wd
	}
	switch args[0] {
	case "skills":
		if args[1] != "install" {
			_, _ = fmt.Fprintf(stderr, "unknown skills command: %s\n", args[1])
			return 2
		}
		return runSkillsInstall(args[2:], stdout, stderr, repo, version)
	case "plugin":
		switch args[1] {
		case "install":
			return runPluginInstall(args[2:], stdout, stderr, repo, version)
		case "uninstall":
			return runPluginUninstall(args[2:], stdout, stderr, repo, version)
		default:
			_, _ = fmt.Fprintf(stderr, "unknown plugin command: %s\n", args[1])
			return 2
		}
	}
	_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
	return 2
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func writeUsage(out io.Writer, command string) {
	switch command {
	case "skills":
		_, _ = fmt.Fprintln(out, "usage: striatum skills install [--profile <profile>] [--scope project|user] [--namespace <prefix>] [--force] [--dry-run] [--json]")
	case "plugin":
		_, _ = fmt.Fprintln(out, "usage: striatum plugin {install|uninstall} [--profile <profile>] [--scope project|user] [--namespace <name>] [--target <path>] [--force] [--json]")
		_, _ = fmt.Fprintln(out, "  install     render an agent plugin bundle")
		_, _ = fmt.Fprintln(out, "  uninstall   remove a previously rendered bundle using its manifest")
	default:
		_, _ = fmt.Fprintf(out, "usage: striatum %s ...\n", command)
	}
}

type flagSet struct {
	values map[string]string
	bools  map[string]bool
}

func parseFlags(args []string, boolFlags map[string]bool, valueFlags map[string]bool) (flagSet, error) {
	fs := flagSet{values: map[string]string{}, bools: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return flagSet{}, fmt.Errorf("unexpected argument: %s", arg)
		}
		key, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		switch {
		case boolFlags[key]:
			if hasValue {
				parsed, err := strconv.ParseBool(value)
				if err != nil {
					return flagSet{}, fmt.Errorf("--%s must be a boolean", key)
				}
				fs.bools[key] = parsed
			} else {
				fs.bools[key] = true
			}
		case valueFlags[key]:
			if !hasValue {
				if i+1 >= len(args) {
					return flagSet{}, fmt.Errorf("--%s requires a value", key)
				}
				value = args[i+1]
				i++
			}
			fs.values[key] = value
		default:
			return flagSet{}, fmt.Errorf("unknown flag: --%s", key)
		}
	}
	return fs, nil
}

func (f flagSet) value(key, def string) string {
	if v, ok := f.values[key]; ok {
		return v
	}
	return def
}

func runSkillsInstall(args []string, stdout, stderr io.Writer, repo, version string) int {
	fs, err := parseFlags(args,
		map[string]bool{"force": true, "dry-run": true, "json": true},
		map[string]bool{"profile": true, "scope": true, "namespace": true},
	)
	if err != nil {
		return emitError(stdout, stderr, fs.bools["json"], err, 2)
	}
	jsonOut := fs.bools["json"]
	profile := fs.value("profile", "claude_code")
	params := installers.SkillsParams{
		Target:    repo,
		Profile:   profile,
		Scope:     fs.value("scope", "project"),
		Namespace: fs.value("namespace", "striatum-"),
		Force:     fs.bools["force"],
		DryRun:    fs.bools["dry-run"],
		Version:   version,
	}
	if profile == "all" {
		result, err := installers.InstallSkillsAll(params)
		if err != nil {
			return emitError(stdout, stderr, jsonOut, err, 6)
		}
		return emit(stdout, stderr, jsonOut, result, skillsAllHuman(result))
	}
	result, err := installers.InstallSkills(params)
	if err != nil {
		return emitError(stdout, stderr, jsonOut, err, 6)
	}
	return emit(stdout, stderr, jsonOut, result, skillsHuman(result))
}

func runPluginInstall(args []string, stdout, stderr io.Writer, repo, version string) int {
	fs, err := parseFlags(args,
		map[string]bool{"force": true, "dry-run": true, "json": true, "with-marketplace": true, "no-marketplace": true},
		map[string]bool{"profile": true, "scope": true, "namespace": true, "target": true},
	)
	if err != nil {
		return emitError(stdout, stderr, fs.bools["json"], err, 2)
	}
	jsonOut := fs.bools["json"]
	withMarketplace := !fs.bools["no-marketplace"]
	target := repo
	if t := fs.value("target", ""); t != "" {
		target = t
	}
	profile := fs.value("profile", "claude_code")
	params := installers.PluginsParams{
		Target:          target,
		Profile:         profile,
		Scope:           fs.value("scope", "project"),
		Namespace:       fs.value("namespace", "striatum"),
		Force:           fs.bools["force"],
		DryRun:          fs.bools["dry-run"],
		WithMarketplace: withMarketplace,
		Version:         version,
	}
	if profile == "all" {
		var results []installers.PluginsResult
		for _, prof := range installers.PluginsAllProfilesOrder {
			sub := params
			sub.Profile = prof
			result, err := installers.InstallPlugin(sub)
			if err != nil {
				return emitError(stdout, stderr, jsonOut, err, 6)
			}
			results = append(results, result)
		}
		payload := map[string]any{"profile": "all", "results": results}
		return emit(stdout, stderr, jsonOut, payload, fmt.Sprintf("installed %d plugin profiles", len(results)))
	}
	result, err := installers.InstallPlugin(params)
	if err != nil {
		return emitError(stdout, stderr, jsonOut, err, 6)
	}
	return emit(stdout, stderr, jsonOut, result, pluginHuman(result))
}

func runPluginUninstall(args []string, stdout, stderr io.Writer, repo, version string) int {
	fs, err := parseFlags(args,
		map[string]bool{"force": true, "json": true},
		map[string]bool{"profile": true, "scope": true, "namespace": true, "target": true},
	)
	if err != nil {
		return emitError(stdout, stderr, fs.bools["json"], err, 2)
	}
	jsonOut := fs.bools["json"]
	target := repo
	if t := fs.value("target", ""); t != "" {
		target = t
	}
	profile := fs.value("profile", "claude_code")
	params := installers.PluginsParams{
		Target:    target,
		Profile:   profile,
		Scope:     fs.value("scope", "project"),
		Namespace: fs.value("namespace", "striatum"),
		Force:     fs.bools["force"],
		Version:   version,
	}
	if profile == "all" {
		var results []installers.PluginUninstallResult
		for _, prof := range installers.PluginsAllProfilesOrder {
			sub := params
			sub.Profile = prof
			result, err := installers.UninstallPlugin(sub)
			if err != nil {
				return emitError(stdout, stderr, jsonOut, err, 6)
			}
			results = append(results, result)
		}
		payload := map[string]any{"profile": "all", "results": results}
		return emit(stdout, stderr, jsonOut, payload, fmt.Sprintf("uninstalled %d plugin profiles", len(results)))
	}
	result, err := installers.UninstallPlugin(params)
	if err != nil {
		return emitError(stdout, stderr, jsonOut, err, 6)
	}
	return emit(stdout, stderr, jsonOut, result, fmt.Sprintf("%s: %s", result.Profile, result.Status))
}

func emit(stdout, stderr io.Writer, jsonOut bool, data any, human string) int {
	if jsonOut {
		encoded, err := json.Marshal(map[string]any{"ok": true, "data": data})
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err.Error())
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(encoded))
		return 0
	}
	_, _ = fmt.Fprintln(stdout, human)
	return 0
}

func emitError(stdout, stderr io.Writer, jsonOut bool, err error, code int) int {
	if jsonOut {
		encoded, mErr := json.Marshal(map[string]any{
			"ok":    false,
			"error": map[string]any{"message": err.Error(), "code": code},
		})
		if mErr == nil {
			_, _ = fmt.Fprintln(stdout, string(encoded))
			return code
		}
	}
	_, _ = fmt.Fprintln(stderr, err.Error())
	return code
}

func skillsHuman(r installers.SkillsResult) string {
	return fmt.Sprintf("skills %s (%s): %d files → %s", r.Profile, r.Scope, len(r.Files), r.ManifestPath)
}

func skillsAllHuman(r installers.SkillsAllResult) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "skills all (%s):", r.Scope)
	for _, sub := range r.Results {
		_, _ = fmt.Fprintf(&b, "\n  %s: %d files → %s", sub.Profile, len(sub.Files), sub.ManifestPath)
	}
	return b.String()
}

func pluginHuman(r installers.PluginsResult) string {
	return fmt.Sprintf("plugin %s (%s): %d files → %s", r.Profile, r.Scope, len(r.Files), r.BundleRoot)
}
