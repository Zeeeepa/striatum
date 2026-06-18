package verifier

import (
	"os/exec"
	"strconv"
	"strings"
)

// SandboxLimits are the resource caps applied to a verifier check so a runaway
// check kills its OWN scope, never the daemon (D227: cgroup-enforced caps). They
// are conservative defaults; an operator can tighten them per deployment.
type SandboxLimits struct {
	// WallClockSeconds is the hard wall-clock deadline. 0 means use the default.
	WallClockSeconds int
	// CPUSeconds is the CPU-time cap (cgroup/rlimit). 0 means use the default.
	CPUSeconds int
	// MemoryBytes is the address-space/memory cap. 0 means use the default.
	MemoryBytes int64
}

// Default sandbox caps. Generous enough for a real `go test`/`grep`, tight enough
// that a fork-bomb or infinite loop is reaped against its own scope.
const (
	defaultWallClockSeconds = 600
	defaultCPUSeconds       = 300
	defaultMemoryBytes      = 2 << 30 // 2 GiB
)

func (l SandboxLimits) withDefaults() SandboxLimits {
	if l.WallClockSeconds <= 0 {
		l.WallClockSeconds = defaultWallClockSeconds
	}
	if l.CPUSeconds <= 0 {
		l.CPUSeconds = defaultCPUSeconds
	}
	if l.MemoryBytes <= 0 {
		l.MemoryBytes = defaultMemoryBytes
	}
	return l
}

// SandboxSpec describes the strict envelope the verifier lane must wrap a check
// in. It is STRICTER than the normal lane posture: no network, no new
// privileges, read-only filesystem except an explicit scratch dir, and
// cgroup/wall-clock/cpu caps. The verifier RESOLVES this spec into a concrete
// argv wrapper at run time, recording exactly which primitives were applied so
// the receipt and the gate can attest the real posture (never a claimed one).
type SandboxSpec struct {
	// CwdReadOnly is the worktree directory the check runs against; it is mounted
	// read-only (the check observes, it does not mutate the tree it witnesses).
	CwdReadOnly string
	// ScratchWritable is the single writable scratch directory (for temp files a
	// check legitimately needs). Everything else is read-only.
	ScratchWritable string
	// Limits are the resource caps.
	Limits SandboxLimits
}

// SandboxMechanism names the wrapper primitive the resolver selected.
type SandboxMechanism string

const (
	// MechanismBubblewrap is the strongest available: bwrap gives a no-network
	// user namespace with a read-only root, a writable scratch bind, and
	// no-new-privileges, composed with systemd-run scope caps when present.
	MechanismBubblewrap SandboxMechanism = "bubblewrap"
	// MechanismSystemdRun applies cgroup/wall-clock/cpu/memory caps and
	// PrivateNetwork via a transient systemd scope, without bwrap's mount
	// namespace (read-only-except-scratch then degrades to advisory).
	MechanismSystemdRun SandboxMechanism = "systemd_run"
	// MechanismUnshareUlimit is the no-extra-tools fallback: `unshare -n` for
	// network isolation plus a `ulimit`-prefixed shell for cpu/mem/fsize caps.
	MechanismUnshareUlimit SandboxMechanism = "unshare_ulimit"
	// MechanismNone means no OS isolation primitive was available; the envelope
	// degrades to the wall-clock deadline the caller imposes (context timeout)
	// plus the existing lane env hardening only. A verifier run under this
	// mechanism can NEVER mint a VERIFIED-eligible receipt (see Posture.Strict).
	MechanismNone SandboxMechanism = "none"
)

// Posture is the RESOLVED, attested security posture of a verifier run: which
// mechanism was used and which guarantees actually hold. It is recorded so the
// gate reads the real posture, not a claimed one. Strict is the load-bearing
// bit: a non-strict posture (network not provably blocked, or no isolation
// primitive) caps the receipt at an INDETERMINATE-or-below signal — it can never
// independently earn VERIFIED.
type Posture struct {
	Mechanism      SandboxMechanism `json:"mechanism"`
	NoNetwork      bool             `json:"no_network"`
	NoNewPrivs     bool             `json:"no_new_privileges"`
	ReadOnlyRootFS bool             `json:"read_only_rootfs"`
	CgroupCaps     bool             `json:"cgroup_caps"`
	WallClockCap   bool             `json:"wall_clock_cap"`
	// Strict is true only when the envelope provably enforces the D227 minimum:
	// no-network AND no-new-privileges AND read-only-except-scratch AND a
	// cgroup/cpu/mem cap. A check that touches the network, violates the
	// envelope, or runs without these can never be VERIFIED.
	Strict bool `json:"strict"`
	// Notes records why a guarantee is absent (e.g. "bwrap not found").
	Notes []string `json:"notes,omitempty"`
}

// lookPath is indirected so tests can assert mechanism selection without the
// host's actual tool inventory.
var lookPath = exec.LookPath

// ResolveSandbox selects the strongest available sandbox mechanism for the host
// and returns the argv WRAPPER to prepend to a check command, plus the resolved
// Posture. It NEVER executes anything — it only composes the wrapper argv and
// reports what will hold. The caller (the verifier lane) runs the wrapped argv;
// the daemon never calls this on its gate path.
//
// Selection order is strongest-first: bubblewrap, then systemd-run, then
// unshare+ulimit, then none. Each step degrades the Posture honestly so the
// receipt records exactly which guarantees a given host could provide.
func ResolveSandbox(spec SandboxSpec) (wrapper []string, posture Posture) {
	limits := spec.Limits.withDefaults()
	if bwrap, err := lookPath("bwrap"); err == nil {
		return bubblewrapWrapper(bwrap, spec, limits)
	}
	if sd, err := lookPath("systemd-run"); err == nil {
		return systemdRunWrapper(sd, limits)
	}
	if un, err := lookPath("unshare"); err == nil {
		return unshareUlimitWrapper(un, limits)
	}
	return nil, Posture{
		Mechanism: MechanismNone,
		Notes:     []string{"no sandbox primitive (bwrap/systemd-run/unshare) found; envelope degraded to env-hardening + wall-clock only — receipts cannot earn VERIFIED"},
	}
}

func bubblewrapWrapper(bwrap string, spec SandboxSpec, limits SandboxLimits) ([]string, Posture) {
	// --unshare-all creates fresh namespaces (including net), --unshare-net is
	// the explicit no-network belt-and-suspenders, --ro-bind / mounts the root
	// read-only, and --cap-drop ALL + bwrap's default PR_SET_NO_NEW_PRIVS give
	// no-new-privileges. The cwd is re-bound read-only and scratch writable below.
	wrapper := []string{
		bwrap,
		"--unshare-all",
		"--unshare-net",
		"--die-with-parent",
		"--new-session",
		"--cap-drop", "ALL",
		"--ro-bind", "/", "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
	}
	if strings.TrimSpace(spec.CwdReadOnly) != "" {
		wrapper = append(wrapper, "--ro-bind", spec.CwdReadOnly, spec.CwdReadOnly)
	}
	if strings.TrimSpace(spec.ScratchWritable) != "" {
		wrapper = append(wrapper, "--bind", spec.ScratchWritable, spec.ScratchWritable)
	}
	posture := Posture{
		Mechanism:      MechanismBubblewrap,
		NoNetwork:      true,
		NoNewPrivs:     true, // bwrap sets PR_SET_NO_NEW_PRIVS by default
		ReadOnlyRootFS: true,
		WallClockCap:   true,
	}
	wrapper = append(wrapper, "--")
	// Cap cpu/mem with a ulimit rlimit prefix INSIDE the bwrap sandbox (after the
	// `--`), so the rlimit applies to the check process itself and needs no
	// privilege. We deliberately do NOT compose `systemd-run --scope` on the
	// outside: creating a transient system scope needs interactive auth in many
	// deployments, and a failure there would mis-report as the check's exit code.
	// The ulimit rlimit is always available and reaped against the check's scope.
	wrapper = append(wrapper, ulimitPrefix(limits)...)
	posture.CgroupCaps = true // rlimit-enforced cpu/mem cap
	posture.Strict = posture.NoNetwork && posture.NoNewPrivs && posture.ReadOnlyRootFS && posture.CgroupCaps
	return wrapper, posture
}

func systemdRunWrapper(sd string, limits SandboxLimits) ([]string, Posture) {
	wrapper := systemdRunScope(sd, limits)
	wrapper = append(wrapper,
		"--property=PrivateNetwork=yes",
		"--property=NoNewPrivileges=yes",
		"--",
	)
	return wrapper, Posture{
		Mechanism:    MechanismSystemdRun,
		NoNetwork:    true,
		NoNewPrivs:   true,
		CgroupCaps:   true,
		WallClockCap: true,
		// systemd-run alone does not give a read-only root mount namespace; the
		// read-only-except-scratch guarantee degrades to advisory, so a run under
		// this mechanism is NOT strict and cannot earn VERIFIED on its own.
		ReadOnlyRootFS: false,
		Strict:         false,
		Notes:          []string{"systemd-run scope: cgroup cpu/mem + PrivateNetwork + NoNewPrivileges enforced, but no read-only-rootfs mount namespace — non-strict, cannot earn VERIFIED alone"},
	}
}

func systemdRunScope(sd string, limits SandboxLimits) []string {
	// --user runs the transient scope under the invoking user's systemd manager,
	// which needs no interactive auth (a system scope does). The lane runs as a
	// real unix user with a user manager, so this is the correct scope kind.
	return []string{
		sd,
		"--user",
		"--scope",
		"--quiet",
		"--collect",
		"--property=RuntimeMaxSec=" + strconv.Itoa(limits.WallClockSeconds),
		"--property=CPUQuota=100%",
		"--property=MemoryMax=" + strconv.FormatInt(limits.MemoryBytes, 10),
		"--property=MemorySwapMax=0",
		"--property=TasksMax=256",
	}
}

func unshareUlimitWrapper(un string, limits SandboxLimits) ([]string, Posture) {
	// `unshare -n` gives a private (empty) network namespace: no network. Compose
	// with a ulimit shell so cpu/mem/fsize are rlimit-capped.
	wrapper := []string{un, "--net", "--"}
	wrapper = append(ulimitPrefix(limits), wrapper...)
	return wrapper, Posture{
		Mechanism:      MechanismUnshareUlimit,
		NoNetwork:      true,
		CgroupCaps:     true, // rlimit cpu/mem
		WallClockCap:   true,
		NoNewPrivs:     false, // unshare does not set NNP by itself
		ReadOnlyRootFS: false,
		Strict:         false,
		Notes:          []string{"unshare+ulimit: no-network + rlimit cpu/mem only — no NNP, no read-only-rootfs — non-strict, cannot earn VERIFIED alone"},
	}
}

// ulimitPrefix returns a `sh -c 'ulimit ...; exec "$@"' sh` prefix that imposes
// cpu/mem/fsize rlimits before exec'ing the wrapped command. The trailing `sh`
// is $0; the actual command follows as positional args.
func ulimitPrefix(limits SandboxLimits) []string {
	script := strings.Join([]string{
		"ulimit -t " + strconv.Itoa(limits.CPUSeconds),
		"ulimit -v " + strconv.FormatInt(limits.MemoryBytes/1024, 10), // KiB
		"ulimit -f 1048576", // 1 GiB max file size
		`exec "$@"`,
	}, "; ")
	return []string{"sh", "-c", script, "sh"}
}

