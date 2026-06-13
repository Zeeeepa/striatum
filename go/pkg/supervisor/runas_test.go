package supervisor

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestCommandInvocationRunAsUsesSudoEnvI(t *testing.T) {
	path, args, cmdEnv := commandInvocation(
		"striatum-lane",
		[]string{
			"PATH=/usr/bin",
			"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
			"PATH=/custom/bin",
		},
		"tmux",
		"new-session",
		"-d",
	)
	if path != "sudo" {
		t.Fatalf("path = %q, want sudo", path)
	}
	if cmdEnv != nil {
		t.Fatalf("sudo wrapper must not inherit cmd.Env, got %#v", cmdEnv)
	}
	wantPrefix := []string{"-n", "-u", "striatum-lane", "--", "env", "-i"}
	if !reflect.DeepEqual(args[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("sudo prefix = %#v, want %#v", args[:len(wantPrefix)], wantPrefix)
	}
	want := []string{
		"-n", "-u", "striatum-lane", "--", "env", "-i",
		"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
		"PATH=/custom/bin",
		"tmux", "new-session", "-d",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestCommandInvocationRunAsAddsDefaultPath(t *testing.T) {
	path, args, _ := commandInvocation("striatum-lane", []string{"TERM=xterm"}, "tmux", "list-sessions")
	if path != "sudo" {
		t.Fatalf("path = %q, want sudo", path)
	}
	if len(args) < 7 || args[6] == "TERM=xterm" {
		t.Fatalf("expected default PATH inserted before other env entries, got %#v", args)
	}
	if args[7] != "TERM=xterm" {
		t.Fatalf("expected TERM after default PATH, got %#v", args)
	}
}

func TestCommandInvocationRunAsDoesNotExposeSensitiveEnvInArgv(t *testing.T) {
	_, args, _ := commandInvocation(
		"striatum-lane",
		[]string{
			"PATH=/usr/bin",
			"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
			"STRIATUM_MCP_TOKEN=stok_session_secret",
		},
		"tmux",
		"new-session",
		"-d",
	)
	joined := strings.Join(args, "\x00")
	for _, needle := range []string{"STRIATUM_MCP_TOKEN=", "stok_session_secret"} {
		if strings.Contains(joined, needle) {
			t.Fatalf("sensitive env leaked through sudo argv: found %q in %#v", needle, args)
		}
	}
}

func TestCommandInvocationRunAsEnvFileDoesNotExposeSensitiveEnvInArgv(t *testing.T) {
	envFile := "/tmp/striatum-lane-env"
	path, args, cmdEnv := commandInvocationWithEnvFile(
		"striatum-lane",
		commandEnvironment{
			entries: []string{
				"PATH=/usr/bin",
				"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
				"STRIATUM_MCP_TOKEN=stok_session_secret",
			},
			filePath: envFile,
		},
		"tmux",
		"new-session",
		"-d",
	)
	if path != "sudo" {
		t.Fatalf("path = %q, want sudo", path)
	}
	if cmdEnv != nil {
		t.Fatalf("sudo wrapper must not inherit cmd.Env, got %#v", cmdEnv)
	}
	joined := strings.Join(args, "\x00")
	if !strings.Contains(joined, envFile) {
		t.Fatalf("env file path missing from sudo argv: %#v", args)
	}
	for _, needle := range []string{"STRIATUM_MCP_TOKEN=", "stok_session_secret"} {
		if strings.Contains(joined, needle) {
			t.Fatalf("sensitive env leaked through sudo argv: found %q in %#v", needle, args)
		}
	}
}

func TestTmuxSetupLaunchSpecDoesNotConsumeLaneEnvFile(t *testing.T) {
	spec := LaunchSpec{
		RunAsUser:   "striatum-lane",
		Env:         []string{"PATH=/usr/bin", "STRIATUM_MCP_TOKEN=stok_session_secret"},
		EnvFilePath: "/tmp/striatum-lane-env",
	}
	setupSpec := tmuxSetupLaunchSpec(spec)
	if setupSpec.EnvFilePath != "" {
		t.Fatalf("setup env file path = %q, want empty", setupSpec.EnvFilePath)
	}
	_, args, _ := commandInvocationWithEnvFile(
		setupSpec.RunAsUser,
		commandEnvironment{entries: setupSpec.Env, filePath: setupSpec.EnvFilePath},
		"tmux",
		"set-option",
		"-t",
		"striatum-session",
		"status",
		"off",
	)
	joined := strings.Join(args, "\x00")
	for _, needle := range []string{spec.EnvFilePath, "STRIATUM_MCP_TOKEN=", "stok_session_secret"} {
		if strings.Contains(joined, needle) {
			t.Fatalf("tmux setup argv consumed sensitive lane env material %q in %#v", needle, args)
		}
	}
}

func TestTmuxEnvArgsDoesNotExposeSensitiveEnv(t *testing.T) {
	args := tmuxEnvArgs([]string{
		"PATH=/usr/bin",
		"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
		"STRIATUM_MCP_TOKEN=stok_session_secret",
		"PROVIDER_API_KEY=provider_secret",
	})
	joined := strings.Join(args, "\x00")
	for _, needle := range []string{"STRIATUM_MCP_TOKEN=", "stok_session_secret", "PROVIDER_API_KEY=", "provider_secret"} {
		if strings.Contains(joined, needle) {
			t.Fatalf("sensitive env leaked through tmux argv: found %q in %#v", needle, args)
		}
	}
	for _, needle := range []string{"PATH=/usr/bin", "STRIATUM_MCP_URL=http://127.0.0.1:1/mcp"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("non-sensitive env missing from tmux argv: want %q in %#v", needle, args)
		}
	}
}

func TestEnvFileWrappedCommandMakesSensitiveEnvAvailableToChild(t *testing.T) {
	content, err := launchEnvFileContent([]string{
		"STRIATUM_MCP_TOKEN=stok_session_secret",
		"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
		"QUOTE_TEST=can't leak",
	})
	if err != nil {
		t.Fatalf("launchEnvFileContent: %v", err)
	}
	envFile, cleanup, err := writeSameUserLaunchEnvFile(t.TempDir(), "sup_env", content)
	if err != nil {
		t.Fatalf("writeSameUserLaunchEnvFile: %v", err)
	}
	defer cleanup()
	info, err := os.Stat(envFile)
	if err != nil {
		t.Fatalf("stat env file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("env file mode = %#o, want 0600", mode)
	}

	outPath := envFile + ".out"
	args := envFileWrappedCommand(envFile, []string{
		"/bin/sh",
		"-c",
		"printf '%s|%s|%s' \"$STRIATUM_MCP_TOKEN\" \"$STRIATUM_MCP_URL\" \"$QUOTE_TEST\" > \"$1\"",
		"write-env",
		outPath,
	})
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = []string{}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("wrapped command failed: %v\n%s", err, out)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read child output: %v", err)
	}
	if got, want := string(body), "stok_session_secret|http://127.0.0.1:1/mcp|can't leak"; got != want {
		t.Fatalf("child env = %q, want %q", got, want)
	}
	if _, err := os.Stat(envFile); !os.IsNotExist(err) {
		t.Fatalf("env file should be removed after the child sources it, stat err = %v", err)
	}
}
