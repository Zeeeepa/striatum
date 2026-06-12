package laneproviderauth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuildCodexSmokeCommand(t *testing.T) {
	got := BuildCodexSmokeCommand(CodexSmokeOptions{
		Binary:     "/opt/codex/bin/codex",
		CWD:        "/tmp/preflight",
		OutputPath: "/tmp/preflight/out.txt",
	})
	want := []string{
		"/opt/codex/bin/codex",
		"exec",
		"--ignore-user-config",
		"--ignore-rules",
		"--ephemeral",
		"--skip-git-repo-check",
		"--sandbox", "read-only",
		"-c", `approval_policy="never"`,
		"-C", "/tmp/preflight",
		"--output-last-message", "/tmp/preflight/out.txt",
		"--json",
		"Reply exactly: ok",
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
}

func TestSanitizeEnvAndLaunchSpecUseEnvI(t *testing.T) {
	env := SanitizeEnv([]string{
		"HOME=/home/lane",
		"USER=lane",
		"PATH=/usr/bin",
		"STRIATUM_MCP_TOKEN=secret",
		"DATABASE_URL=postgres://secret",
		"OPENAI_API_KEY=secret",
		"LC_ALL=C.UTF-8",
	}, []string{"/opt/codex/bin"})
	rendered := strings.Join(env, "\n")
	for _, forbidden := range []string{"STRIATUM_MCP_TOKEN", "DATABASE_URL", "OPENAI_API_KEY", "secret"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("sanitized env leaked %q: %s", forbidden, rendered)
		}
	}
	if !strings.Contains(rendered, "HOME=/home/lane") || !strings.Contains(rendered, "LC_ALL=C.UTF-8") {
		t.Fatalf("sanitized env dropped required basics: %s", rendered)
	}
	if !strings.Contains(rendered, "PATH=/opt/codex/bin:/usr/bin") {
		t.Fatalf("path_prefix was not prepended: %s", rendered)
	}

	spec := BuildLaunchSpec([]string{"codex", "exec"}, "/tmp/preflight", "striatum-lane", env)
	if spec.Name != "sudo" {
		t.Fatalf("run-as launch command = %q, want sudo", spec.Name)
	}
	joined := strings.Join(spec.Args, "\x00")
	for _, want := range []string{"-n", "-u", "striatum-lane", "env", "-i", "HOME=/home/lane", "codex", "exec"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("run-as args missing %q: %#v", want, spec.Args)
		}
	}
	if strings.Contains(joined, "STRIATUM_MCP_TOKEN") || strings.Contains(joined, "DATABASE_URL") {
		t.Fatalf("run-as env-i args leaked secret env: %#v", spec.Args)
	}
}

func TestCheckClassifiesCodexResults(t *testing.T) {
	tests := []struct {
		name          string
		run           CommandResult
		output        string
		wantStatus    string
		wantFailure   string
		wantNoOutput  bool
		wantReadError bool
	}{
		{
			name:       "success",
			run:        CommandResult{ExitCode: 0},
			output:     "ok\n",
			wantStatus: StatusPassed,
		},
		{
			name:        "stale_auth",
			run:         CommandResult{ExitCode: 1, Stderr: "not logged in; token expired", Err: errors.New("exit status 1")},
			wantStatus:  StatusFailed,
			wantFailure: FailureAuthFailed,
		},
		{
			name:        "missing_binary",
			run:         CommandResult{ExitCode: -1, Err: os.ErrNotExist},
			wantStatus:  StatusFailed,
			wantFailure: FailureBinaryMissing,
		},
		{
			name:        "timeout",
			run:         CommandResult{TimedOut: true, Err: context.DeadlineExceeded},
			wantStatus:  StatusFailed,
			wantFailure: FailureTimeout,
		},
		{
			name:        "provider_unavailable",
			run:         CommandResult{ExitCode: 1, Stderr: "service unavailable: 503", Err: errors.New("exit status 1")},
			wantStatus:  StatusFailed,
			wantFailure: FailureUnavailable,
		},
		{
			name:        "unexpected_success_output",
			run:         CommandResult{ExitCode: 0},
			output:      "hello",
			wantStatus:  StatusFailed,
			wantFailure: FailureUnexpectedResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Check(context.Background(), Params{
				Provider: ProviderCodex,
				RunID:    "run_1",
				LaneID:   "author",
				Env:      []string{"HOME=/home/lane", "PATH=/usr/bin"},
				Runner: RunnerFunc(func(_ context.Context, spec CommandSpec) CommandResult {
					if tt.output != "" {
						if err := os.WriteFile(outputPathFromSpec(t, spec), []byte(tt.output), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					return tt.run
				}),
			})
			if result.Status != tt.wantStatus || result.FailureClass != tt.wantFailure {
				t.Fatalf("result = %#v, want status=%s failure=%s", result, tt.wantStatus, tt.wantFailure)
			}
			payload, err := json.Marshal(result.ToMap())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(payload), "not logged in") || strings.Contains(string(payload), "token expired") || strings.Contains(string(payload), "hello") {
				t.Fatalf("safe result leaked raw provider output: %s", string(payload))
			}
			if result.ToMap()["raw_output_returned"] != false {
				t.Fatalf("raw_output_returned = %#v", result.ToMap()["raw_output_returned"])
			}
		})
	}
}

func TestUnsupportedProviderResultIsSafe(t *testing.T) {
	result := Check(context.Background(), Params{Provider: "agy"})
	if result.Status != StatusFailed || result.FailureClass != FailureUnsupported || result.Checked {
		t.Fatalf("unsupported result = %#v", result)
	}
}

func TestSerializationIsPerProviderAuthHome(t *testing.T) {
	var inFlight int32
	var maxInFlight int32
	runner := RunnerFunc(func(_ context.Context, spec CommandSpec) CommandResult {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			max := atomic.LoadInt32(&maxInFlight)
			if current <= max || atomic.CompareAndSwapInt32(&maxInFlight, max, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		if err := os.WriteFile(outputPathFromSpec(t, spec), []byte("ok"), 0o600); err != nil {
			t.Fatal(err)
		}
		return CommandResult{ExitCode: 0}
	})

	done := make(chan Result, 2)
	params := Params{
		Provider: ProviderCodex,
		Env:      []string{"HOME=/home/lane", "PATH=/usr/bin"},
		Runner:   runner,
	}
	go func() { done <- Check(context.Background(), params) }()
	go func() { done <- Check(context.Background(), params) }()
	for i := 0; i < 2; i++ {
		if result := <-done; !result.Passed() {
			t.Fatalf("check failed: %#v", result)
		}
	}
	if atomic.LoadInt32(&maxInFlight) != 1 {
		t.Fatalf("checks for same provider auth home ran concurrently: max=%d", maxInFlight)
	}
}

func outputPathFromSpec(t *testing.T, spec CommandSpec) string {
	t.Helper()
	args := append([]string{spec.Name}, spec.Args...)
	for i, arg := range args {
		if arg == "--output-last-message" && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("spec missing --output-last-message: %#v", spec)
	return filepath.Join(t.TempDir(), "missing")
}
