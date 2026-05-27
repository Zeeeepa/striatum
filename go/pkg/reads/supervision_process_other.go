//go:build !linux

package reads

func processStartToken(pid int) (string, bool) {
	return "", false
}

func processZombie(pid int) bool {
	return false
}
