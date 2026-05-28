//go:build linux

package mutations

import (
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

func processStartToken(pid int) (string, bool) {
	return gosupervisor.ProcessStartToken(pid)
}
