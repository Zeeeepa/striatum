//go:build !linux

package mutations

func processStartToken(pid int) (string, bool) {
	return "", false
}
