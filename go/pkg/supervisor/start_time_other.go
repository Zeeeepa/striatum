//go:build !linux && !darwin

package supervisor

import "time"

// readProcessStartTimeOS is a no-op on non-Linux/non-darwin platforms.
// The caller falls back to signal-0 only.
func readProcessStartTimeOS(pid int) (time.Time, bool) {
	return time.Time{}, false
}
