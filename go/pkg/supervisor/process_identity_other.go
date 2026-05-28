//go:build !linux

package supervisor

// ProcessStartToken returns a stable PID start token when the platform exposes
// one. Unsupported platforms return ok=false and callers fall back to best
// effort signal probes.
func ProcessStartToken(pid int) (string, bool) {
	return "", false
}
