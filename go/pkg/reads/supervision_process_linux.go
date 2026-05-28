//go:build linux

package reads

import (
	"fmt"
	"os"

	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

func processStartToken(pid int) (string, bool) {
	return gosupervisor.ProcessStartToken(pid)
}

func processZombie(pid int) bool {
	if pid <= 0 {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	return linuxProcStatZombie(data)
}
