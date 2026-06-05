package supervisor

import (
	"reflect"
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
