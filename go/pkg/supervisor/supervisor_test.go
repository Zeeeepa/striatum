package supervisor

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLaunchEmptyCommandRejected(t *testing.T) {
	_, err := Launch(context.Background(), t.TempDir(), "sup_x", LaunchSpec{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestTmuxSessionNameIncludesSupervisorIDAndIsSanitized(t *testing.T) {
	got := tmuxSessionName("run/id:one", "lane one", "sup:123")
	if !strings.HasPrefix(got, "striatum-run_id_one-lane_one-sup_123-") || len(got) != len("striatum-run_id_one-lane_one-sup_123-")+tmuxSessionNameHashLen {
		t.Fatalf("session name = %q", got)
	}
	long := tmuxSessionName(strings.Repeat("r", 90), strings.Repeat("l", 90), strings.Repeat("s", 90))
	if len(long) > tmuxSessionNameMaxLen {
		t.Fatalf("session name length = %d, want <= 100", len(long))
	}
}

func TestTmuxSessionNameHashSuffixAvoidsTruncationCollision(t *testing.T) {
	runID := "run_" + strings.Repeat("r", 80)
	lanePrefix := "lane_" + strings.Repeat("l", 140)
	first := tmuxSessionName(runID, lanePrefix+"a", "sup_same")
	second := tmuxSessionName(runID, lanePrefix+"b", "sup_same")
	if first == second {
		t.Fatalf("distinct long lane ids collided: %q", first)
	}
	if len(first) > tmuxSessionNameMaxLen || len(second) > tmuxSessionNameMaxLen {
		t.Fatalf("session names too long: %d %d", len(first), len(second))
	}

	supervisorPrefix := "sup_" + strings.Repeat("s", 140)
	third := tmuxSessionName(runID, lanePrefix, supervisorPrefix+"a")
	fourth := tmuxSessionName(runID, lanePrefix, supervisorPrefix+"b")
	if third == fourth {
		t.Fatalf("distinct long supervisor ids collided: %q", third)
	}
}

func testCommandPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not present in PATH; skipping process launch test", name)
	}
	return path
}

func TestLaunchPTYWired(t *testing.T) {
	truePath := testCommandPath(t, "true")
	// V1.6: PTY path is functional. Spawn true under PTY and assert
	// we got a PID + StdinWriter back; the master fd is the writer.
	res, err := Launch(context.Background(), t.TempDir(), "sup_pty_001", LaunchSpec{
		Command: []string{truePath},
		UsePTY:  true,
	})
	if err != nil {
		t.Fatalf("Launch PTY: %v", err)
	}
	if res.PID <= 0 {
		t.Fatalf("expected positive PID, got %d", res.PID)
	}
	if res.StdinWriter == nil {
		t.Fatal("expected non-nil StdinWriter from PTY launch")
	}
	_ = res.StdinWriter.Close()
	_ = res.Cmd.Wait()
}

func TestLaunchPTYRequireTmuxRefusesWhenUnavailable(t *testing.T) {
	truePath := testCommandPath(t, "true")
	t.Setenv("PATH", t.TempDir())

	_, err := Launch(context.Background(), t.TempDir(), "sup_tmux_required", LaunchSpec{
		Command: []string{truePath},
		UsePTY:  true,
		Env: []string{
			"STRIATUM_RUN_ID=run_tmux_required",
			"STRIATUM_LANE_ID=lane_tmux_required",
		},
		RequireTmux: true,
	})
	if err == nil {
		t.Fatal("expected required tmux launch to fail when tmux is unavailable")
	}
	if !strings.Contains(err.Error(), "tmux required") || !strings.Contains(err.Error(), "tmux was not found") {
		t.Fatalf("error = %q, want clean tmux-required refusal", err.Error())
	}
}

func TestLaunchPTYOptionalTmuxFallsBackWhenUnavailable(t *testing.T) {
	truePath := testCommandPath(t, "true")
	t.Setenv("PATH", t.TempDir())

	res, err := Launch(context.Background(), t.TempDir(), "sup_tmux_optional", LaunchSpec{
		Command: []string{truePath},
		UsePTY:  true,
		Env: []string{
			"STRIATUM_RUN_ID=run_tmux_optional",
			"STRIATUM_LANE_ID=lane_tmux_optional",
		},
	})
	if err != nil {
		t.Fatalf("Launch optional tmux: %v", err)
	}
	if res.PID <= 0 {
		t.Fatalf("expected positive PID, got %d", res.PID)
	}
	if res.StdinWriter == nil {
		t.Fatal("expected non-nil StdinWriter from fallback PTY launch")
	}
	tmux, ok := res.Metadata["tmux"].(map[string]any)
	if !ok || tmux["unavailable_reason"] != "tmux_not_found" {
		t.Fatalf("fallback tmux metadata = %#v", res.Metadata["tmux"])
	}
	_ = res.StdinWriter.Close()
	_ = res.Cmd.Wait()
}

func TestLaunchPTYTmuxSetupCommandsAreBounded(t *testing.T) {
	truePath := testCommandPath(t, "true")
	dir := t.TempDir()
	tmuxPath := filepath.Join(dir, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexec /bin/sleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("STRIATUM_TMUX_SETUP_TIMEOUT", "50ms")

	start := time.Now()
	_, err := Launch(context.Background(), t.TempDir(), "sup_tmux_setup_timeout", LaunchSpec{
		Command: []string{truePath},
		UsePTY:  true,
		Env: []string{
			"STRIATUM_RUN_ID=run_tmux_setup_timeout",
			"STRIATUM_LANE_ID=lane_tmux_setup_timeout",
		},
		RequireTmux: true,
	})
	if err == nil {
		t.Fatal("expected tmux setup timeout")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Launch returned after %s, want bounded tmux setup timeout", elapsed)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %q, want timeout detail", err.Error())
	}
}

func TestLaunchPTYTmuxImmediateExitPreservesDeadPane(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	shPath := testCommandPath(t, "sh")
	dir := t.TempDir()
	runID := "run_tmux_exit"
	laneID := "lane_tmux_exit"
	supervisorID := "sup_tmux_exit"
	sessionName := tmuxSessionName(runID, laneID, supervisorID)
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	t.Cleanup(func() { _ = exec.Command("tmux", "kill-session", "-t", sessionName).Run() })

	res, err := Launch(context.Background(), dir, supervisorID, LaunchSpec{
		Command:     []string{shPath, "-c", "exit 42"},
		WorkingDir:  dir,
		UsePTY:      true,
		RequireTmux: true,
		Env: []string{
			"STRIATUM_RUN_ID=" + runID,
			"STRIATUM_LANE_ID=" + laneID,
		},
	})
	if err != nil {
		t.Fatalf("Launch tmux immediate-exit command: %v", err)
	}
	if res.StdinWriter != nil {
		_ = res.StdinWriter.Close()
	}
	if res.Cmd != nil && res.Cmd.Process != nil {
		_ = res.Cmd.Process.Kill()
		_ = res.Cmd.Wait()
	}
	id, ok := TmuxIdentityFromMetadata(res.Metadata)
	if !ok {
		t.Fatalf("tmux identity missing from metadata: %#v", res.Metadata)
	}
	var live TmuxLiveness
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		live = ProbeTmuxLiveness(context.Background(), DefaultTmuxRunner(), id)
		if live.Class == TmuxLivenessPaneDead {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("liveness = %#v, want %s; launch must preserve dead panes for diagnostics", live, TmuxLivenessPaneDead)
}

func TestLaunchPipeMode(t *testing.T) {
	truePath := testCommandPath(t, "true")
	dir := t.TempDir()
	res, err := Launch(context.Background(), dir, "sup_pipe_001", LaunchSpec{
		Command: []string{truePath},
		UsePTY:  false,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if res.PID <= 0 {
		t.Fatalf("expected positive PID, got %d", res.PID)
	}
	if res.Cmd == nil {
		t.Fatal("expected non-nil Cmd")
	}
	_ = res.StdinWriter.Close()
	_ = res.Cmd.Wait()
}

// TestLaunchPTYEchoTable exercises F-pty end-to-end: spawn cat under
// PTY, write a payload to the master fd (the daemon's StdinWriter handle),
// and read it back. The slave side reaches cat as ordinary stdin and
// is echoed to stdout. This is the only test that proves packet delivery
// actually traverses the PTY round trip — the true case in
// TestLaunchPTYWired only proves Start succeeded.
func TestLaunchPTYEchoTable(t *testing.T) {
	catPath := testCommandPath(t, "cat")
	cases := []struct {
		name    string
		payload string
	}{
		{name: "short", payload: "ping\n"},
		{name: "embedded_spaces", payload: "hello striatum\n"},
		{name: "multi_line", payload: "alpha\nbeta\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			res, err := Launch(ctx, t.TempDir(), "sup_pty_echo_"+tc.name, LaunchSpec{
				Command: []string{catPath},
				UsePTY:  true,
			})
			if err != nil {
				t.Fatalf("Launch PTY %s: %v", tc.name, err)
			}
			if res.PID <= 0 {
				t.Fatalf("PID not set: %d", res.PID)
			}
			if res.StdinWriter == nil {
				t.Fatal("StdinWriter nil")
			}
			reader, ok := res.StdinWriter.(io.Reader)
			if !ok {
				t.Fatal("PTY master is not also a Reader; cannot verify echo")
			}
			if _, err := res.StdinWriter.Write([]byte(tc.payload)); err != nil {
				t.Fatalf("write payload: %v", err)
			}
			buf := make([]byte, len(tc.payload)*2)
			done := make(chan struct{})
			var n int
			go func() {
				defer close(done)
				deadline := time.Now().Add(2 * time.Second)
				for n < len(tc.payload) && time.Now().Before(deadline) {
					m, err := reader.Read(buf[n:])
					if m > 0 {
						n += m
					}
					if err != nil {
						return
					}
				}
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("timed out waiting for PTY echo read")
			}
			// PTY mode translates LF → CRLF on output. Compare with
			// CR-stripped echo so the substring assertion is
			// portable across PTY and pipe modes.
			got := bytes.ReplaceAll(buf[:n], []byte{'\r'}, nil)
			if !bytes.Contains(got, []byte(tc.payload[:len(tc.payload)-1])) {
				t.Fatalf("echo mismatch: got %q want substring %q", buf[:n], tc.payload)
			}
			_ = res.StdinWriter.Close()
			if res.Cmd != nil && res.Cmd.Process != nil {
				_ = res.Cmd.Process.Kill()
				_ = res.Cmd.Wait()
			}
		})
	}
}

