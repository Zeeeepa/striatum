//go:build darwin

package supervisor

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// readProcessStartTimeOS reads the kernel-reported process start time on
// darwin (macOS) via `ps -o lstart= -p <pid>`. The output is parsed in the
// `Mon Jan 2 15:04:05 2006` BSD format. Returns ok=false on any parse
// error so the caller falls back to signal-0 only.
//
// V1.7 implementation. The `ps` shell-out is intentional: cgo + Mach
// task_info adds binary-distribution complexity; the `ps` invocation is
// cheap and works against the standard /bin/ps on every supported macOS
// release. This is the operational-tool tradeoff documented in RFC 0039
// V1.7. If `ps` is unavailable (e.g. minimal container build) the
// fallback to signal-0 only still keeps liveness probing functional.
func readProcessStartTimeOS(pid int) (time.Time, bool) {
	out, err := exec.Command("/bin/ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return time.Time{}, false
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse("Mon Jan  2 15:04:05 2006", value)
	if err != nil {
		parsed, err = time.Parse("Mon Jan 2 15:04:05 2006", value)
		if err != nil {
			return time.Time{}, false
		}
	}
	return parsed.UTC(), true
}
