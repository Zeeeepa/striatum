//go:build linux

package supervisor

import (
	"fmt"
	"os"
	"strings"
)

// ProcessStartToken returns the kernel start-time token used to detect PID
// reuse. Linux exposes it as /proc/<pid>/stat field 22.
func ProcessStartToken(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", false
	}
	text := string(data)
	endComm := strings.LastIndex(text, ")")
	if endComm < 0 || endComm+1 >= len(text) {
		return "", false
	}
	fields := strings.Fields(text[endComm+1:])
	const starttimeIndex = 22 - 3
	if len(fields) <= starttimeIndex {
		return "", false
	}
	return fields[starttimeIndex], true
}
