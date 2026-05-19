package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type processInfo struct {
	alive bool
	name  string
}

var inspectPid = inspectOSPid

func daemonPidfilePath(socketPath string) string {
	return filepath.Join(filepath.Dir(socketPath), "striatumd.pid")
}

func writeDaemonPidfile(pidPath string, pid int) error {
	if existing, err := readDaemonPidfile(pidPath); err == nil {
		info := inspectPid(existing)
		if existing != pid && info.alive && isStriatumdProcess(info.name) {
			return fmt.Errorf("daemon pidfile %s already points to live striatumd pid %d", pidPath, existing)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	tmp := pidPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write daemon pid tmp: %w", err)
	}
	if err := os.Rename(tmp, pidPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename daemon pidfile: %w", err)
	}
	return nil
}

func readDaemonPidfile(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse daemon pidfile %q: %w", pidPath, err)
	}
	return pid, nil
}

func isStriatumdProcess(name string) bool {
	base := filepath.Base(strings.TrimSpace(name))
	return base == "striatumd" || strings.HasPrefix(base, "striatumd-") || strings.HasPrefix(base, "striatumd.")
}

func inspectOSPid(pid int) processInfo {
	if pid <= 0 {
		return processInfo{}
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return processInfo{}
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return processInfo{}
	}
	name := readProcName(pid)
	return processInfo{alive: true, name: name}
}

func readProcName(pid int) string {
	if data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm")); err == nil {
		return strings.TrimSpace(string(data))
	}
	if target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe")); err == nil {
		return filepath.Base(target)
	}
	if data, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output(); err == nil {
		return filepath.Base(strings.TrimSpace(string(data)))
	}
	return ""
}
