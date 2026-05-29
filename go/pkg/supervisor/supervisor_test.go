package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeStore struct {
	mu       sync.Mutex
	rows     map[string]PointerRow
	lostCall string
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]PointerRow{}}
}

func (f *fakeStore) UpsertSupervisorPointer(ctx context.Context, row PointerRow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[row.SupervisorID] = row
	return nil
}

func (f *fakeStore) MarkSupervisorLost(ctx context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.rows[id]
	row.State = "lost"
	row.LostReason = reason
	f.rows[id] = row
	f.lostCall = id
	return nil
}

func (f *fakeStore) GetSupervisorPointer(ctx context.Context, id string) (PointerRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok {
		return PointerRow{}, fmt.Errorf("no row")
	}
	return row, nil
}

func TestWriteAndReadPidfile(t *testing.T) {
	dir := t.TempDir()
	supID := "sup_test_001"
	pidPath, err := WritePidfile(dir, supID, 42)
	if err != nil {
		t.Fatalf("WritePidfile: %v", err)
	}
	if want := filepath.Join(dir, supID, "pid"); pidPath != want {
		t.Fatalf("pidpath: got %q want %q", pidPath, want)
	}
	got, err := ReadPidfile(dir, supID)
	if err != nil {
		t.Fatalf("ReadPidfile: %v", err)
	}
	if got != 42 {
		t.Fatalf("pid: got %d want 42", got)
	}
}

func TestReadPidfileMissing(t *testing.T) {
	_, err := ReadPidfile(t.TempDir(), "sup_missing")
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.IsNotExist, got %v", err)
	}
}

func TestUpsertPointerRejectsEmpty(t *testing.T) {
	store := newFakeStore()
	err := UpsertPointer(context.Background(), store, PointerRow{})
	if err == nil {
		t.Fatal("expected error for empty supervisor_id")
	}
}

func TestLivenessHeartbeatOnLiveProcess(t *testing.T) {
	store := newFakeStore()
	supID := "sup_live_001"
	pid := os.Getpid() // ourselves: definitely alive
	store.UpsertSupervisorPointer(context.Background(), PointerRow{
		SupervisorID: supID,
		RepositoryID: "repo_test",
		PID:          pid,
		State:        "starting",
	})

	cfg := LivenessConfig{
		HeartbeatInterval: 25 * time.Millisecond,
		LostAfter:         500 * time.Millisecond,
	}
	l := NewLiveness(cfg, store, supID, pid)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)

	time.Sleep(120 * time.Millisecond)

	if err := l.Stop(context.Background(), false); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	row, err := store.GetSupervisorPointer(context.Background(), supID)
	if err != nil {
		t.Fatalf("GetSupervisorPointer: %v", err)
	}
	if row.State != "running" {
		t.Fatalf("state: got %q want running", row.State)
	}
	if l.LastHeartbeat().IsZero() {
		t.Fatal("LastHeartbeat zero — no tick observed")
	}
}

func TestLivenessMarksLostOnDeadPid(t *testing.T) {
	store := newFakeStore()
	supID := "sup_dead_001"
	deadPid := 1 // init — signal-0 would normally succeed; use a sentinel guaranteed-dead pid below
	// Pick a pid we can be confident is not present: a very large pid.
	deadPid = 999999999
	store.UpsertSupervisorPointer(context.Background(), PointerRow{
		SupervisorID: supID,
		RepositoryID: "repo_test",
		PID:          deadPid,
		State:        "starting",
	})

	cfg := LivenessConfig{
		HeartbeatInterval: 25 * time.Millisecond,
	}
	l := NewLiveness(cfg, store, supID, deadPid)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)

	time.Sleep(120 * time.Millisecond)
	_ = l.Stop(context.Background(), false)

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.lostCall != supID {
		t.Fatalf("lost not marked: got %q", store.lostCall)
	}
}

func TestLivenessMarksLostOnCorruptTmuxMetadata(t *testing.T) {
	store := newFakeStore()
	supID := "sup_corrupt_tmux"
	store.UpsertSupervisorPointer(context.Background(), PointerRow{
		SupervisorID: supID,
		RepositoryID: "repo_test",
		PID:          os.Getpid(),
		State:        "running",
		Metadata: map[string]any{
			"tmux": map[string]any{
				"state":        "backed",
				"session_name": "striatum-run",
			},
		},
	})

	cfg := LivenessConfig{HeartbeatInterval: 25 * time.Millisecond}
	l := NewLiveness(cfg, store, supID, os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)

	time.Sleep(120 * time.Millisecond)
	_ = l.Stop(context.Background(), false)

	row, err := store.GetSupervisorPointer(context.Background(), supID)
	if err != nil {
		t.Fatalf("GetSupervisorPointer: %v", err)
	}
	if row.State != "lost" || row.LostReason != "tmux_metadata_corrupt" {
		t.Fatalf("row = %#v, want corrupt tmux metadata lost", row)
	}
}

func TestRecordTmuxProbeUnavailableMetadata(t *testing.T) {
	now := time.Date(2026, 5, 28, 18, 0, 0, 0, time.UTC)
	metadata := map[string]any{
		"tmux": map[string]any{
			"state":      "backed",
			"last_ok_at": "2026-05-28T17:59:00Z",
		},
	}
	recordTmuxProbeUnavailable(metadata, now, 2, LaneLiveness{
		Backed: "tmux",
		Class:  string(TmuxLivenessUnavailable),
		Detail: "probe_timeout",
		Tmux: &TmuxLiveness{
			Class:  TmuxLivenessUnavailable,
			State:  "degraded",
			Detail: "probe_timeout",
		},
	})
	tmux := metadata["tmux"].(map[string]any)
	if tmux["probe_unavailable_count"] != 2 || tmux["last_unavailable_detail"] != "probe_timeout" {
		t.Fatalf("tmux metadata = %#v", tmux)
	}
	if tmux["liveness_state"] != "degraded" {
		t.Fatalf("tmux metadata missing degraded liveness state: %#v", tmux)
	}
	if tmux["probe_skipped_at"] == "" {
		t.Fatalf("tmux metadata missing probe_skipped_at: %#v", tmux)
	}
	recordTmuxProbeOK(metadata, now.Add(time.Second))
	if tmux["probe_unavailable_count"] != nil || tmux["probe_skipped_at"] != nil || tmux["last_unavailable_detail"] != nil {
		t.Fatalf("tmux metadata did not clear unavailable fields: %#v", tmux)
	}
	if tmux["last_ok_at"] == "2026-05-28T17:59:00Z" {
		t.Fatalf("last_ok_at was not refreshed: %#v", tmux)
	}
}

func TestLivenessDegradesBeforeLostOnPersistentTmuxUnavailable(t *testing.T) {
	origRunner := livenessTmuxRunner
	defer func() { livenessTmuxRunner = origRunner }()
	livenessTmuxRunner = &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}, err: exec.ErrNotFound},
	}}
	t.Setenv("STRIATUM_TMUX_UNAVAILABLE_LOST_THRESHOLD", "3")

	store := newFakeStore()
	supID := "sup_tmux_unavailable"
	store.UpsertSupervisorPointer(context.Background(), PointerRow{
		SupervisorID: supID,
		RepositoryID: "repo_test",
		PID:          os.Getpid(),
		State:        "running",
		Metadata: map[string]any{
			"tmux": map[string]any{
				"state":        "backed",
				"session_name": "striatum-run",
				"pane_id":      "%4",
				"pane_pid":     os.Getpid(),
			},
		},
	})

	l := NewLiveness(LivenessConfig{HeartbeatInterval: 30 * time.Millisecond, GraceOnTerm: 10 * time.Millisecond}, store, supID, os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)
	defer l.Stop(context.Background(), false)

	waitForSupervisorTest(t, 500*time.Millisecond, func() bool {
		row, err := store.GetSupervisorPointer(context.Background(), supID)
		if err != nil {
			return false
		}
		tmux := row.Metadata["tmux"].(map[string]any)
		return row.State == "running" && tmux["liveness_state"] == "degraded" && tmux["probe_unavailable_count"] == 1
	})
	if store.lostCall != "" {
		t.Fatalf("supervisor marked lost before degraded warning: %q", store.lostCall)
	}

	waitForSupervisorTest(t, time.Second, func() bool {
		row, err := store.GetSupervisorPointer(context.Background(), supID)
		return err == nil && row.State == "lost" && row.LostReason == "tmux_unavailable_persistent"
	})
}

func waitForSupervisorTest(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if condition() {
		return
	}
	t.Fatalf("condition was not satisfied within %s", timeout)
}

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

// TestProcessAliveAtStartTimeRecycling locks F-pid-recycling: feeding the
// probe a startedAt that disagrees with the kernel-reported start time by
// well over the 2-second tolerance must return false even though signal-0
// would succeed on our own PID. Skips on non-Linux because the start-time
// reader is Linux-only in V1.6 (acceptance gate is Linux per RFC 0039 §6).
func TestProcessAliveAtStartTimeRecycling(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("start-time reader is Linux-only in V1.6; non-Linux falls back to signal-0")
	}
	pid := os.Getpid()
	cases := []struct {
		name        string
		expected    time.Time
		shouldAlive bool
	}{
		{
			name:        "zero_startedat_falls_back_to_signal0",
			expected:    time.Time{},
			shouldAlive: true,
		},
		{
			name:        "mismatched_startedat_far_past",
			expected:    time.Unix(1, 0).UTC(), // epoch-ish; will not match any real proc
			shouldAlive: false,
		},
		{
			name:        "mismatched_startedat_far_future",
			expected:    time.Now().Add(24 * time.Hour).UTC(),
			shouldAlive: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := processAliveAtStartTime(pid, tc.expected)
			if got != tc.shouldAlive {
				t.Fatalf("processAliveAtStartTime(self, %v) = %v want %v", tc.expected, got, tc.shouldAlive)
			}
		})
	}

	// Sanity-check that the reader, given a startedAt within tolerance of
	// the actual kernel start time, returns alive. If the reader can't
	// produce a time at all (kernel without /proc/stat btime, exotic
	// container, etc.) the probe falls back to signal-0 and returns true,
	// which still satisfies "alive" for this pid.
	t.Run("matching_startedat_within_tolerance", func(t *testing.T) {
		actual, ok := readProcessStartTime(pid)
		if !ok {
			t.Skip("readProcessStartTime returned ok=false; cannot construct in-tolerance case")
		}
		if !processAliveAtStartTime(pid, actual) {
			t.Fatalf("processAliveAtStartTime(self, %v) = false; want true", actual)
		}
	})
}

// TestScratchAndPidfilePerms locks F-perms: the scratch directory must be
// 0o700 and the pid file 0o600. We assert via filesystem mode bits after
// WritePidfile.
func TestScratchAndPidfilePerms(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantMod os.FileMode
		isDir   bool
	}{
		{name: "scratch_dir_0700", wantMod: 0o700, isDir: true},
		{name: "pidfile_0600", wantMod: 0o600, isDir: false},
	}
	dir := t.TempDir()
	supID := "sup_perms_001"
	pidPath, err := WritePidfile(dir, supID, 4242)
	if err != nil {
		t.Fatalf("WritePidfile: %v", err)
	}
	scratchPath := filepath.Join(dir, supID)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			target := pidPath
			if tc.isDir {
				target = scratchPath
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatalf("stat %q: %v", target, err)
			}
			gotMode := info.Mode().Perm()
			if gotMode != tc.wantMod {
				t.Fatalf("%s: mode = %#o want %#o", target, gotMode, tc.wantMod)
			}
			if tc.isDir && !info.IsDir() {
				t.Fatalf("%s: expected directory", target)
			}
		})
	}
}
