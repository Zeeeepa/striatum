package supervisor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
)

const (
	tmuxSessionNameMaxLen  = 100
	tmuxSessionNameHashLen = 12
	launchEnvFileName      = "lane-env.sh"
	launchEnvFileExec      = "set -a; . \"$1\"; rm -f -- \"$1\"; shift; exec \"$@\""
)

// LaunchSpec describes a supervised child process. The fields mirror the
// Python wrapper protocol so the same workflow.json command lines work
// against either supervisor implementation.
type LaunchSpec struct {
	Command       []string          // exec argv
	Env           []string          // additional KEY=VAL entries (merged with os.Environ)
	EnvFilePath   string            // optional shell env file path used instead of argv-carried env
	WorkingDir    string            // wd for the child
	RunAsUser     string            // optional OS user for lane process/tmux execution
	StdinPipePath string            // FIFO path; daemon writes packets here
	StdoutPath    string            // DEVNULL by default; per D028, no transcript capture
	StderrPath    string            // DEVNULL by default
	UsePTY        bool              // true → allocate a PTY (creack/pty); false → plain pipes
	RequireTmux   bool              // true → fail closed instead of falling back to a plain PTY
	Extra         map[string]string // future-use metadata
}

// LaunchResult is returned by Launch. For tmux-backed PTY launches PID is the
// pane process pid; AttachPID is the transient attach client pid used only for
// byte delivery and diagnostics.
type LaunchResult struct {
	PID         int
	StdinWriter io.WriteCloser
	Cmd         *exec.Cmd
	AttachPID   int
	Metadata    map[string]any
}

type commandEnvironment struct {
	entries  []string
	filePath string
}

func getEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):]
		}
	}
	return ""
}

func commandContext(ctx context.Context, spec LaunchSpec, program string, args ...string) *exec.Cmd {
	env := commandEnvironment{entries: spec.Env, filePath: spec.EnvFilePath}
	path, cmdArgs, cmdEnv := commandInvocationWithEnvFile(strings.TrimSpace(spec.RunAsUser), env, program, args...)
	cmd := exec.CommandContext(ctx, path, cmdArgs...)
	cmd.Dir = spec.WorkingDir
	cmd.Env = cmdEnv
	return cmd
}

func commandInvocation(runAsUser string, env []string, program string, args ...string) (string, []string, []string) {
	return commandInvocationWithEnvFile(runAsUser, commandEnvironment{entries: env}, program, args...)
}

func WriteLaunchEnvFile(ctx context.Context, scratchDir string, supervisorID string, runAsUser string, env []string) (string, func(), error) {
	content, err := launchEnvFileContent(env)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(runAsUser) != "" {
		return writeRunAsLaunchEnvFile(ctx, strings.TrimSpace(runAsUser), content)
	}
	return writeSameUserLaunchEnvFile(scratchDir, supervisorID, content)
}

func EnvFileWrappedCommand(envFilePath string, command []string) []string {
	return envFileWrappedCommand(envFilePath, command)
}

func commandInvocationWithEnvFile(runAsUser string, env commandEnvironment, program string, args ...string) (string, []string, []string) {
	if strings.TrimSpace(runAsUser) == "" {
		return program, append([]string(nil), args...), mergeEnv(os.Environ(), env.entries)
	}
	if strings.TrimSpace(env.filePath) != "" {
		wrapped := envFileWrappedCommand(strings.TrimSpace(env.filePath), append([]string{program}, args...))
		sudoArgs := []string{"-n", "-u", strings.TrimSpace(runAsUser), "--", "env", "-i"}
		sudoArgs = append(sudoArgs, wrapped...)
		return "sudo", sudoArgs, nil
	}
	sudoArgs := []string{"-n", "-u", strings.TrimSpace(runAsUser), "--", "env", "-i"}
	sudoArgs = append(sudoArgs, sanitizedRunAsEnv(env.entries)...)
	sudoArgs = append(sudoArgs, program)
	sudoArgs = append(sudoArgs, args...)
	return "sudo", sudoArgs, nil
}

func tmuxSetupLaunchSpec(spec LaunchSpec) LaunchSpec {
	spec.EnvFilePath = ""
	return spec
}

func sanitizedRunAsEnv(env []string) []string {
	out := nonSensitiveEnv(dedupeEnvLastWins(env))
	if getEnvValue(out, "PATH") == "" {
		out = append([]string{"PATH=" + defaultRunAsPath()}, out...)
	}
	return out
}

func nonSensitiveEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || sensitiveEnvKey(key) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func sensitiveEnvKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if upper == "" {
		return true
	}
	switch upper {
	case "STRIATUM_MCP_TOKEN", "STRIATUM_MCP_TOKEN_FILE", "DATABASE_URL", "PGPASSWORD":
		return true
	}
	for _, marker := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "API_KEY", "CREDENTIAL", "DSN"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func dedupeEnvLastWins(env []string) []string {
	last := map[string]int{}
	for i, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			last[key] = i
		}
	}
	out := make([]string, 0, len(last))
	for i, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || last[key] != i {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func defaultRunAsPath() string {
	if path := strings.TrimSpace(os.Getenv("PATH")); path != "" {
		return path
	}
	return "/usr/local/bin:/usr/bin:/bin"
}

func prepareLaunchEnvFile(ctx context.Context, scratchDir string, supervisorID string, spec LaunchSpec) (LaunchSpec, func(), error) {
	if strings.TrimSpace(spec.EnvFilePath) != "" || len(spec.Env) == 0 {
		return spec, func() {}, nil
	}
	path, cleanup, err := WriteLaunchEnvFile(ctx, scratchDir, supervisorID, spec.RunAsUser, spec.Env)
	if err != nil {
		return spec, nil, err
	}
	spec.EnvFilePath = path
	return spec, cleanup, nil
}

func writeSameUserLaunchEnvFile(scratchDir string, supervisorID string, content []byte) (string, func(), error) {
	if strings.TrimSpace(scratchDir) == "" {
		scratchDir = os.TempDir()
	}
	dir := filepath.Join(scratchDir, supervisorID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, launchEnvFileName)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func writeRunAsLaunchEnvFile(ctx context.Context, runAsUser string, content []byte) (string, func(), error) {
	script := strings.Join([]string{
		"set -eu",
		"tmpdir=${TMPDIR:-/tmp}",
		"path=$(mktemp \"$tmpdir/striatum-supervisor-env.XXXXXX\")",
		"chmod 600 \"$path\"",
		"cat > \"$path\"",
		"printf '%s' \"$path\"",
	}, "; ")
	cmd := exec.CommandContext(ctx, "sudo", "-n", "-u", runAsUser, "--", "sh", "-c", script)
	cmd.Stdin = bytes.NewReader(content)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", nil, fmt.Errorf("write run-as env file: %s", detail)
	}
	path := strings.TrimSpace(stdout.String())
	if path == "" {
		return "", nil, fmt.Errorf("write run-as env file: empty path")
	}
	cleanup := func() {
		_ = exec.Command("sudo", "-n", "-u", runAsUser, "--", "rm", "-f", "--", path).Run()
	}
	return path, cleanup, nil
}

func launchEnvFileContent(env []string) ([]byte, error) {
	var body strings.Builder
	for _, entry := range dedupeEnvLastWins(env) {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if !validEnvKey(key) {
			return nil, fmt.Errorf("supervisor: invalid env key %q", key)
		}
		body.WriteString("export ")
		body.WriteString(key)
		body.WriteByte('=')
		body.WriteString(shellQuote(value))
		body.WriteByte('\n')
	}
	return []byte(body.String()), nil
}

func validEnvKey(key string) bool {
	for i, r := range key {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return key != "" && (key[0] == '_' || key[0] >= 'A' && key[0] <= 'Z' || key[0] >= 'a' && key[0] <= 'z')
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func envFileWrappedCommand(envFilePath string, command []string) []string {
	if strings.TrimSpace(envFilePath) == "" {
		return append([]string(nil), command...)
	}
	wrapped := []string{"/bin/sh", "-c", launchEnvFileExec, "striatum-env", strings.TrimSpace(envFilePath)}
	wrapped = append(wrapped, command...)
	return wrapped
}

func runAsMetadata(runAsUser string) map[string]any {
	if strings.TrimSpace(runAsUser) == "" {
		return nil
	}
	return map[string]any{"run_as_user": strings.TrimSpace(runAsUser)}
}

func withRunAsMetadata(metadata map[string]any, runAsUser string) map[string]any {
	if strings.TrimSpace(runAsUser) == "" {
		return metadata
	}
	out := map[string]any{}
	for key, value := range metadata {
		out[key] = value
	}
	out["run_as_user"] = strings.TrimSpace(runAsUser)
	return out
}

func tmuxRunnerForSpec(spec LaunchSpec) TmuxRunner {
	if strings.TrimSpace(spec.RunAsUser) == "" {
		return DefaultTmuxRunner()
	}
	return RunAsTmuxRunner(spec.RunAsUser, spec.Env)
}

// Launch starts the supervised child. UsePTY=true allocates a pseudo-tty
// via creack/pty and threads the master back as the daemon's stdin handle
// for packet delivery (V1.6 F-pty closure). UsePTY=false uses os.Pipe +
// os/exec so the FIFO/stdin path is exercised by tests that do not require
// terminal semantics.
func Launch(ctx context.Context, scratchDir string, supervisorID string, spec LaunchSpec) (*LaunchResult, error) {
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("supervisor: empty command")
	}

	if err := ensureFIFO(scratchDir, supervisorID, spec.StdinPipePath); err != nil {
		return nil, err
	}

	if spec.UsePTY {
		return launchPTY(ctx, scratchDir, supervisorID, spec)
	}

	var cleanup func()
	var err error
	if strings.TrimSpace(spec.RunAsUser) != "" {
		spec, cleanup, err = prepareLaunchEnvFile(ctx, scratchDir, supervisorID, spec)
		if err != nil {
			return nil, err
		}
	}

	cmd := commandContext(ctx, spec, spec.Command[0], spec.Command[1:]...)

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("supervisor: stdin pipe: %w", err)
	}
	cmd.Stdin = stdinR

	cmd.Stdout, err = openDevNullOr(spec.StdoutPath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		_ = stdinR.Close()
		_ = stdinW.Close()
		return nil, err
	}
	cmd.Stderr, err = openDevNullOr(spec.StderrPath)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		_ = stdinR.Close()
		_ = stdinW.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		if cleanup != nil {
			cleanup()
		}
		_ = stdinR.Close()
		_ = stdinW.Close()
		return nil, fmt.Errorf("supervisor: cmd.Start: %w", err)
	}

	return &LaunchResult{
		PID:         cmd.Process.Pid,
		StdinWriter: stdinW,
		Cmd:         cmd,
		Metadata:    runAsMetadata(spec.RunAsUser),
	}, nil
}

// ensureFIFO creates the supervisor's scratch dir; per D028 we do not capture
// transcripts. The FIFO itself is created by the daemon when delivering
// packets, but the supervisor scratch dir must exist for the wrapper script
// to write progress markers + pidfile.
func ensureFIFO(scratchDir string, supervisorID string, fifoPath string) error {
	dir := filepath.Join(scratchDir, supervisorID)
	return os.MkdirAll(dir, 0o700)
}

// launchPTY allocates a pseudo-tty via creack/pty (RFC 0039 V1.6 F-pty).
// pty.Start sets up the child's stdin/stdout/stderr on the slave side and
// returns the master file we hand back to the daemon as StdinWriter — the
// daemon writes packets to the master, the child reads them off the slave
// as ordinary stdin.
func launchPTY(ctx context.Context, scratchDir string, supervisorID string, spec LaunchSpec) (*LaunchResult, error) {
	runID := getEnvValue(spec.Env, "STRIATUM_RUN_ID")
	laneID := getEnvValue(spec.Env, "STRIATUM_LANE_ID")
	if runID == "" || laneID == "" {
		if spec.RequireTmux {
			return nil, tmuxRequiredError("missing_run_or_lane")
		}
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("missing_run_or_lane"))
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		if spec.RequireTmux {
			return nil, tmuxRequiredError("tmux_not_found")
		}
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_not_found"))
	}

	sessionName := tmuxSessionName(runID, laneID, supervisorID)
	setupSpec := tmuxSetupLaunchSpec(spec)

	// Kill existing session with the same name if any (to avoid collisions / stale sessions)
	_ = runTmuxSetupCommand(ctx, setupSpec, "kill-session", "-t", sessionName)

	// 1. Create a detached tmux session with a placeholder process. The real
	// lane command is respawned only after remain-on-exit is set, so even a
	// command that exits immediately leaves a dead pane for diagnostics instead
	// of destroying the session before liveness can classify it.
	// RFC 0088 P3 follow-up: pass STRIATUM_*/PATH env vars via tmux's `-e
	// KEY=VAL` so the new session's pane child sees them. A long-running global
	// tmux server inherits its environment from FIRST launch, not from our
	// `new-session`/`respawn-pane` call's createCmd.Env — so without `-e`, the
	// pane child gets the SERVER's stale env (no STRIATUM_RUN_ID/SESSION_ID/REPO
	// etc.) and the agent-loop wrapper exits code 1 on the env check before any
	// output.
	newSessionArgs := []string{"new-session", "-d", "-s", sessionName, "-c", spec.WorkingDir}
	newSessionArgs = append(newSessionArgs, tmuxEnvArgs(spec.Env)...)
	newSessionArgs = append(newSessionArgs, "--")
	newSessionArgs = append(newSessionArgs, "sh", "-c", "while :; do sleep 3600; done")
	createCmd := commandContext(context.Background(), setupSpec, "tmux", newSessionArgs...)
	if err := runPreparedTmuxSetupCommand(ctx, createCmd, newSessionArgs...); err != nil {
		return nil, fmt.Errorf("supervisor: failed to create tmux session: %w", err)
	}
	cleanupTmux := true
	defer func() {
		if cleanupTmux {
			_ = runTmuxSetupCommand(context.Background(), setupSpec, "kill-session", "-t", sessionName)
		}
	}()

	// 2. Set tmux options before the real command is allowed to run.
	if err := runTmuxSetupCommand(ctx, setupSpec, "set-option", "-t", sessionName, "status", "off"); err != nil {
		if spec.RequireTmux {
			return nil, fmt.Errorf("supervisor: failed to configure tmux status: %w", err)
		}
		_ = runTmuxSetupCommand(context.Background(), setupSpec, "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_setup_failed"))
	}
	if err := runTmuxSetupCommand(ctx, setupSpec, "set-window-option", "-t", sessionName, "remain-on-exit", "on"); err != nil {
		if spec.RequireTmux {
			return nil, fmt.Errorf("supervisor: failed to configure tmux remain-on-exit: %w", err)
		}
		_ = runTmuxSetupCommand(context.Background(), setupSpec, "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_setup_failed"))
	}

	laneSpec, cleanupEnv, err := prepareLaunchEnvFile(ctx, scratchDir, supervisorID, spec)
	if err != nil {
		return nil, err
	}
	envFileCleanupNeeded := true
	defer func() {
		if envFileCleanupNeeded {
			cleanupEnv()
		}
	}()

	respawnArgs := []string{"respawn-pane", "-k", "-t", sessionName + ":0.0", "-c", spec.WorkingDir}
	respawnArgs = append(respawnArgs, tmuxEnvArgs(spec.Env)...)
	respawnArgs = append(respawnArgs, "--")
	respawnArgs = append(respawnArgs, envFileWrappedCommand(laneSpec.EnvFilePath, laneSpec.Command)...)
	respawnCmd := commandContext(ctx, setupSpec, "tmux", respawnArgs...)
	if err := runPreparedTmuxSetupCommand(ctx, respawnCmd, respawnArgs...); err != nil {
		if spec.RequireTmux {
			return nil, fmt.Errorf("supervisor: failed to respawn tmux lane command: %w", err)
		}
		_ = runTmuxSetupCommand(context.Background(), setupSpec, "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_respawn_failed"))
	}

	identity, err := CaptureTmuxIdentity(ctx, tmuxRunnerForSpec(setupSpec), sessionName)
	if err != nil || identity.WindowID == "" || identity.PaneID == "" || identity.PanePID <= 0 {
		if spec.RequireTmux {
			if err != nil {
				return nil, fmt.Errorf("supervisor: tmux identity capture failed: %w", err)
			}
			return nil, fmt.Errorf("supervisor: tmux identity capture failed")
		}
		_ = runTmuxSetupCommand(context.Background(), setupSpec, "kill-session", "-t", sessionName)
		cleanupTmux = false
		return launchPlainPTY(ctx, spec, tmuxUnavailableMetadata("tmux_identity_capture_failed"))
	}

	// 3. Attach to the session in the PTY
	result, err := attachTmuxPTY(ctx, identity, setupSpec)
	if err != nil {
		return nil, err
	}
	cleanupTmux = false
	envFileCleanupNeeded = false

	return result, nil
}

func attachTmuxPTY(ctx context.Context, identity TmuxIdentity, spec LaunchSpec) (*LaunchResult, error) {
	if strings.TrimSpace(identity.SessionName) == "" {
		return nil, fmt.Errorf("supervisor: tmux session name is required")
	}
	attachCmd := commandContext(ctx, spec, "tmux", "attach-session", "-t", identity.SessionName)

	ptmx, err := pty.Start(attachCmd)
	if err != nil {
		return nil, fmt.Errorf("supervisor: pty.Start (tmux attach): %w", err)
	}
	return &LaunchResult{
		PID:         identity.PanePID,
		StdinWriter: ptmx,
		Cmd:         attachCmd,
		AttachPID:   attachCmd.Process.Pid,
		Metadata:    tmuxResultMetadata(identity, attachCmd.Process.Pid, spec.RunAsUser),
	}, nil
}

func tmuxEnvArgs(env []string) []string {
	args := make([]string, 0, len(env)*2)
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || sensitiveEnvKey(key) {
			continue
		}
		args = append(args, "-e", entry)
	}
	return args
}

func runTmuxSetupCommand(ctx context.Context, spec LaunchSpec, args ...string) error {
	cmd := commandContext(ctx, spec, "tmux", args...)
	return runPreparedTmuxSetupCommand(ctx, cmd, args...)
}

func runPreparedTmuxSetupCommand(ctx context.Context, cmd *exec.Cmd, args ...string) error {
	timeout := tmuxSetupTimeout()
	if timeout <= 0 {
		timeout = tmuxProbeTimeout()
	}
	dir := cmd.Dir
	env := cmd.Env
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd = exec.CommandContext(runCtx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if runCtx.Err() != nil {
		return fmt.Errorf("tmux setup command %s timed out: %w", strings.Join(args, " "), runCtx.Err())
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("tmux setup command %s failed: %s", strings.Join(args, " "), detail)
	}
	return nil
}

func tmuxSetupTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("STRIATUM_TMUX_SETUP_TIMEOUT"))
	if raw == "" {
		return tmuxProbeTimeout()
	}
	if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
		return duration
	}
	if seconds, err := strconv.ParseFloat(raw, 64); err == nil && seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	return tmuxProbeTimeout()
}

func launchPlainPTY(ctx context.Context, spec LaunchSpec, metadata map[string]any) (*LaunchResult, error) {
	var cleanup func()
	var err error
	if strings.TrimSpace(spec.RunAsUser) != "" {
		spec, cleanup, err = prepareLaunchEnvFile(ctx, "", "", spec)
		if err != nil {
			return nil, err
		}
	}
	cmd := commandContext(ctx, spec, spec.Command[0], spec.Command[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, fmt.Errorf("supervisor: pty.Start: %w", err)
	}
	return &LaunchResult{
		PID:         cmd.Process.Pid,
		StdinWriter: ptmx,
		Cmd:         cmd,
		Metadata:    withRunAsMetadata(metadata, spec.RunAsUser),
	}, nil
}

func tmuxSessionName(runID, laneID, supervisorID string) string {
	prefix := "striatum-" + sanitizeTmuxName(runID) + "-" + sanitizeTmuxName(laneID)
	if supervisorID != "" {
		prefix += "-" + sanitizeTmuxName(supervisorID)
	}
	hashInput := runID + "\x00" + laneID + "\x00" + supervisorID
	sum := sha256.Sum256([]byte(hashInput))
	suffix := "-" + hex.EncodeToString(sum[:])[:tmuxSessionNameHashLen]
	maxPrefix := tmuxSessionNameMaxLen - len(suffix)
	if len(prefix) > maxPrefix {
		prefix = prefix[:maxPrefix]
	}
	return prefix + suffix
}

func sanitizeTmuxName(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}

func mergeEnv(base []string, updates []string) []string {
	keys := map[string]bool{}
	for _, entry := range updates {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			keys[key] = true
		}
	}
	out := make([]string, 0, len(base)+len(updates))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || keys[key] {
			continue
		}
		out = append(out, entry)
	}
	return append(out, updates...)
}

func tmuxResultMetadata(identity TmuxIdentity, attachPID int, runAsUser string) map[string]any {
	metadata := map[string]any{
		"tmux": tmuxBackedMetadata(identity, attachPID, runAsUser),
	}
	if strings.TrimSpace(runAsUser) != "" {
		metadata["run_as_user"] = strings.TrimSpace(runAsUser)
	}
	return metadata
}

func tmuxBackedMetadata(identity TmuxIdentity, attachPID int, runAsUser string) map[string]any {
	metadata := map[string]any{
		"state":             "backed",
		"session_name":      identity.SessionName,
		"window_id":         identity.WindowID,
		"pane_id":           identity.PaneID,
		"pane_pid":          identity.PanePID,
		"attach_command":    "tmux attach-session -t " + identity.SessionName,
		"attach_client_pid": attachPID,
		"captured_at":       time.Now().UTC().Format(time.RFC3339Nano),
	}
	if identity.PaneStartToken != "" {
		metadata["pane_start_token"] = identity.PaneStartToken
	}
	if strings.TrimSpace(runAsUser) != "" {
		metadata["run_as_user"] = strings.TrimSpace(runAsUser)
	}
	return metadata
}

func tmuxUnavailableMetadata(reason string) map[string]any {
	return map[string]any{
		"tmux": map[string]any{
			"state":              "unavailable",
			"unavailable_reason": reason,
		},
	}
}

func tmuxRequiredError(reason string) error {
	switch reason {
	case "missing_run_or_lane":
		return fmt.Errorf("supervisor: tmux required but STRIATUM_RUN_ID or STRIATUM_LANE_ID is missing")
	case "tmux_not_found":
		return fmt.Errorf("supervisor: tmux required but tmux was not found in PATH; install tmux or unset supervision.require_tmux for non-interactive lanes")
	default:
		return fmt.Errorf("supervisor: tmux required but unavailable: %s", reason)
	}
}

// openDevNullOr returns the file at path if non-empty, else os.DevNull.
func openDevNullOr(path string) (*os.File, error) {
	target := path
	if target == "" {
		target = os.DevNull
	}
	return os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}
