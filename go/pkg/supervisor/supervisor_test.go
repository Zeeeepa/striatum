package supervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestLaunchEmptyCommandRejected(t *testing.T) {
	_, err := Launch(context.Background(), t.TempDir(), "sup_x", LaunchSpec{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestLaunchPTYWired(t *testing.T) {
	// V1.6: PTY path is functional. Spawn /bin/true under PTY and assert
	// we got a PID + StdinWriter back; the master fd is the writer.
	res, err := Launch(context.Background(), t.TempDir(), "sup_pty_001", LaunchSpec{
		Command: []string{"/bin/true"},
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

func TestLaunchPipeMode(t *testing.T) {
	dir := t.TempDir()
	res, err := Launch(context.Background(), dir, "sup_pipe_001", LaunchSpec{
		Command: []string{"/bin/true"},
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
