package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDaemonPidfilePathUsesSocketDirectory(t *testing.T) {
	got := daemonPidfilePath(filepath.Join("tmp", "runtime", "striatumd.sock"))
	want := filepath.Join("tmp", "runtime", "striatumd.pid")
	if got != want {
		t.Fatalf("daemonPidfilePath() = %q, want %q", got, want)
	}
}

func TestWriteDaemonPidfileWritesPIDModeAndNoTmp(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "striatumd.pid")
	if err := writeDaemonPidfile(pidPath, 12345); err != nil {
		t.Fatalf("writeDaemonPidfile() error = %v", err)
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read pidfile: %v", err)
	}
	if string(data) != "12345\n" {
		t.Fatalf("pidfile content = %q", string(data))
	}
	info, err := os.Stat(pidPath)
	if err != nil {
		t.Fatalf("stat pidfile: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("pidfile mode = %o, want 0600", got)
	}
	if _, err := os.Stat(pidPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary pidfile still exists or unexpected stat error: %v", err)
	}
}

func TestWriteDaemonPidfileRefusesLiveForeignStriatumd(t *testing.T) {
	restore := stubInspectPid(processInfo{alive: true, name: "striatumd"})
	defer restore()

	pidPath := filepath.Join(t.TempDir(), "striatumd.pid")
	if err := os.WriteFile(pidPath, []byte("4242\n"), 0o600); err != nil {
		t.Fatalf("seed pidfile: %v", err)
	}
	err := writeDaemonPidfile(pidPath, 12345)
	if err == nil || !strings.Contains(err.Error(), "live striatumd pid 4242") {
		t.Fatalf("writeDaemonPidfile() error = %v, want live striatumd refusal", err)
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read pidfile: %v", err)
	}
	if string(data) != "4242\n" {
		t.Fatalf("pidfile was overwritten: %q", string(data))
	}
}

func TestReserveDaemonRuntimeRefusesLiveForeignStriatumd(t *testing.T) {
	restore := stubInspectPid(processInfo{alive: true, name: "striatumd"})
	defer restore()

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "daemon-go.sock")
	pidPath := daemonPidfilePath(socketPath)
	if err := os.WriteFile(pidPath, []byte("4242\n"), 0o600); err != nil {
		t.Fatalf("seed pidfile: %v", err)
	}
	cleanup, err := reserveDaemonRuntime(socketPath, 12345)
	if err == nil {
		cleanup()
		t.Fatal("reserveDaemonRuntime() succeeded, want live daemon refusal")
	}
	if !strings.Contains(err.Error(), "live striatumd pid 4242") {
		t.Fatalf("reserveDaemonRuntime() error = %v, want live daemon refusal", err)
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read pidfile: %v", err)
	}
	if string(data) != "4242\n" {
		t.Fatalf("pidfile was overwritten: %q", string(data))
	}
}

func TestReserveDaemonRuntimeRefusesLiveSocketWithoutPidfile(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "daemon-go.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	cleanup, err := reserveDaemonRuntime(socketPath, 12345)
	if err == nil {
		cleanup()
		t.Fatal("reserveDaemonRuntime() succeeded, want live socket refusal")
	}
	if !strings.Contains(err.Error(), "already accepts connections") {
		t.Fatalf("reserveDaemonRuntime() error = %v, want live socket refusal", err)
	}
	if _, err := os.Stat(daemonPidfilePath(socketPath)); !os.IsNotExist(err) {
		t.Fatalf("pidfile should not be written when live socket exists; stat error: %v", err)
	}
}

func TestWriteDaemonPidfileOverwritesStalePidfile(t *testing.T) {
	restore := stubInspectPid(processInfo{alive: false, name: ""})
	defer restore()

	pidPath := filepath.Join(t.TempDir(), "striatumd.pid")
	if err := os.WriteFile(pidPath, []byte("4242\n"), 0o600); err != nil {
		t.Fatalf("seed pidfile: %v", err)
	}
	if err := writeDaemonPidfile(pidPath, 12345); err != nil {
		t.Fatalf("writeDaemonPidfile() error = %v", err)
	}
	pid, err := readDaemonPidfile(pidPath)
	if err != nil {
		t.Fatalf("readDaemonPidfile() error = %v", err)
	}
	if pid != 12345 {
		t.Fatalf("pidfile pid = %d, want 12345", pid)
	}
}

func TestWriteDaemonPidfileOverwritesLiveNonStriatumdPidfile(t *testing.T) {
	restore := stubInspectPid(processInfo{alive: true, name: "bash"})
	defer restore()

	pidPath := filepath.Join(t.TempDir(), "striatumd.pid")
	if err := os.WriteFile(pidPath, []byte("4242\n"), 0o600); err != nil {
		t.Fatalf("seed pidfile: %v", err)
	}
	if err := writeDaemonPidfile(pidPath, 12345); err != nil {
		t.Fatalf("writeDaemonPidfile() error = %v", err)
	}
	pid, err := readDaemonPidfile(pidPath)
	if err != nil {
		t.Fatalf("readDaemonPidfile() error = %v", err)
	}
	if pid != 12345 {
		t.Fatalf("pidfile pid = %d, want 12345", pid)
	}
}

func TestCleanShutdownRemovesDaemonPidfile(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "striatumd.pid")
	if err := writeDaemonPidfile(pidPath, 12345); err != nil {
		t.Fatalf("writeDaemonPidfile() error = %v", err)
	}
	if err := os.Remove(pidPath); err != nil {
		t.Fatalf("remove pidfile: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pidfile still exists or unexpected stat error: %v", err)
	}
}

func stubInspectPid(info processInfo) func() {
	original := inspectPid
	inspectPid = func(int) processInfo {
		return info
	}
	return func() {
		inspectPid = original
	}
}
