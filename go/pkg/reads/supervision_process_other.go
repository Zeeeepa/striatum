//go:build !linux

package reads

func processStartToken(pid int) (string, bool) {
	return "", false
}
