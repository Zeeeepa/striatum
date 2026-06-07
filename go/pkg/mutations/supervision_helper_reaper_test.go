package mutations

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestStartHelperReaperHarvestsExitedChild guards #204: supervisor-helper
// children are spawned detached and were never cmd.Wait()'d, so when one exited
// the kernel left an unreapable <defunct> zombie in the daemon's PID table until
// the daemon itself exited (the 2026-06-06 restart accumulated 21). The reaper
// must collect the child's exit status: after the returned channel closes, the
// child PID must NOT be a zombie (it must be fully gone from the process table).
func TestStartHelperReaperHarvestsExitedChild(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("zombie-state assertion reads /proc; linux-only")
	}
	// A child that exits immediately is the worst case for reaping: it becomes a
	// <defunct> entry the instant it exits and stays one until waited on.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	pid := cmd.Process.Pid

	reaped := startHelperReaper(cmd)
	select {
	case <-reaped:
	case <-time.After(5 * time.Second):
		t.Fatalf("reaper did not close its done channel within 5s for pid %d", pid)
	}

	// After the reaper has Wait()'d, the kernel has released the PID: /proc/<pid>
	// is gone (ESRCH). If the reaper had not run, the entry would persist with
	// stat state 'Z' (zombie). A live or zombie /proc entry both fail this.
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		state := procStatState(data)
		t.Fatalf("pid %d still present after reaping (stat state=%q) — zombie/leaked child not harvested", pid, state)
	}
	// Sanity: a fresh Wait4 on the reaped pid must report no child (ECHILD), i.e.
	// nothing left for an operator/init to reap.
	if processZombieLocal(pid) {
		t.Fatalf("pid %d reports zombie after reaping", pid)
	}
}

// TestHelperExitedGateReflectsReaperChannel pins the handshake-loop gate that
// replaced the (Wait-owned, now data-racy) cmd.ProcessState read: a nil channel
// (test/synchronous callers) reads not-exited; an open channel reads not-exited;
// a closed channel (reaper harvested the child) reads exited so the loop breaks.
func TestHelperExitedGateReflectsReaperChannel(t *testing.T) {
	if helperExited(nil) {
		t.Fatal("nil reaper channel must read as not-exited")
	}
	open := make(chan struct{})
	if helperExited(open) {
		t.Fatal("open reaper channel must read as not-exited")
	}
	closed := make(chan struct{})
	close(closed)
	if !helperExited(closed) {
		t.Fatal("closed reaper channel must read as exited")
	}
}

func procStatState(data []byte) string {
	text := string(data)
	idx := strings.LastIndex(text, ")")
	if idx < 0 || idx+1 >= len(text) {
		return ""
	}
	fields := strings.Fields(text[idx+1:])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
