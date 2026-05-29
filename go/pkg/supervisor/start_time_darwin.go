//go:build darwin

package supervisor

import (
	"syscall"
	"time"
	"unsafe"
)

const (
	ctrlKern    = 1
	kernProc    = 14
	kernProcPID = 1
)

// readProcessStartTimeOS reads the kernel-reported process start time on
// darwin (macOS) via the native sysctl kernel interface. Returns ok=false
// on any error so the caller falls back to signal-0 only.
func readProcessStartTimeOS(pid int) (time.Time, bool) {
	mib := []int32{ctrlKern, kernProc, kernProcPID, int32(pid)}
	var kproc [1024]byte
	size := uintptr(len(kproc))

	r1, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		uintptr(unsafe.Pointer(&kproc[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	if r1 != 0 || errno != 0 {
		return time.Time{}, false
	}

	if size < 28 {
		return time.Time{}, false
	}

	sec := *(*int64)(unsafe.Pointer(&kproc[16]))
	usec := *(*int32)(unsafe.Pointer(&kproc[24]))

	if sec == 0 {
		return time.Time{}, false
	}
	return time.Unix(sec, int64(usec)*1000).UTC(), true
}
