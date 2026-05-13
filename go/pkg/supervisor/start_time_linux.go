//go:build linux

package supervisor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// readProcessStartTimeOS reads the kernel-reported process start time on
// Linux. Returns ok=false on any parse error so the caller falls back to
// signal-0 only.
func readProcessStartTimeOS(pid int) (time.Time, bool) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return time.Time{}, false
	}
	// Format: pid (comm) state ppid ... starttime ... — comm may contain
	// spaces and parens, so anchor on the last ')'.
	endComm := strings.LastIndex(string(data), ")")
	if endComm < 0 || endComm+2 >= len(data) {
		return time.Time{}, false
	}
	fields := strings.Fields(string(data[endComm+1:]))
	const starttimeIdx = 22 - 3
	if len(fields) <= starttimeIdx {
		return time.Time{}, false
	}
	clockTicks, err := strconv.ParseUint(fields[starttimeIdx], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	statData, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, false
	}
	var btime int64
	for _, line := range strings.Split(string(statData), "\n") {
		if strings.HasPrefix(line, "btime ") {
			btime, err = strconv.ParseInt(strings.TrimSpace(line[len("btime "):]), 10, 64)
			if err != nil {
				return time.Time{}, false
			}
			break
		}
	}
	if btime == 0 {
		return time.Time{}, false
	}
	const clkTck = 100
	seconds := int64(clockTicks) / clkTck
	return time.Unix(btime+seconds, 0).UTC(), true
}
