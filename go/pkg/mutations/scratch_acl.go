package mutations

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// setScratchACL applies (`setfacl -m`) an ACL entry to a path. It is a package
// variable so tests can observe the planned ACL operations without requiring
// root or the setfacl binary (mirrors setDaemonSocketACL in cmd/striatumd).
var setScratchACL = func(spec, path string) error {
	return exec.Command("setfacl", "-m", spec, path).Run()
}

// setScratchDefaultACL applies a DEFAULT ACL entry (`setfacl -d -m`) to a
// directory so files the lane user later creates inside it (e.g. the next
// ephemeral MCP config) inherit the grant. Overridable for tests.
var setScratchDefaultACL = func(spec, path string) error {
	return exec.Command("setfacl", "-d", "-m", spec, path).Run()
}

type scratchACLTarget struct {
	path   string
	spec   string
	defACL bool // also apply a default ACL (`setfacl -d`) on this directory
}

// scratchACLTargets returns the ACL grants a non-owner lane user needs so a
// supervised lane can create private files under its own supervisor scratch
// directory, while sibling lanes can only traverse the scratch roots. The Claude
// MCP bearer is written under .striatum/scratch/<supervisor_id>, not directly
// under .striatum/scratch, so the root no longer needs a writable/default ACL.
// We grant:
//   - `.striatum`        : u:<lane>:--x  (traverse only — never broaden read of
//     private operator state; matches the working remediation)
//   - `.striatum/scratch`: u:<lane>:--x  (traverse only)
//   - `.striatum/scratch/<supervisor_id>`: u:<lane>:rwx + a default ACL so lane
//     private files are writable only in the lane's own subdir.
func scratchACLTargets(repoRoot, laneUser, supervisorID string) []scratchACLTarget {
	striatumDir := filepath.Join(repoRoot, ".striatum")
	scratchDir := filepath.Join(striatumDir, "scratch")
	targets := []scratchACLTarget{
		{path: striatumDir, spec: "u:" + laneUser + ":--x"},
		{path: scratchDir, spec: "u:" + laneUser + ":--x"},
	}
	if strings.TrimSpace(supervisorID) != "" {
		targets = append(targets, scratchACLTarget{
			path:   filepath.Join(scratchDir, supervisorID),
			spec:   "u:" + laneUser + ":rwx",
			defACL: true,
		})
	}
	return targets
}

// prepareScratchACLsForLaneUser grants the lane OS user the ACLs it needs to
// create its ephemeral MCP config under <repoRoot>/.striatum/scratch before the
// lane is launched (#279). It is a no-op when the lane runs as the daemon owner
// (laneUser empty — configuredLaneRunAsUser already collapses the owner case to
// "") or on platforms without setfacl semantics. Only the daemon-managed scratch
// directory is touched; broader private state is left alone (stay on the daemon
// boundary). The caller is expected to have already created the scratch dir.
func prepareScratchACLsForLaneUser(repoRoot, laneUser, supervisorID string) error {
	laneUser = strings.TrimSpace(laneUser)
	if laneUser == "" || strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	if runtime.GOOS == "windows" {
		// configuredLaneRunAsUser already rejects RunAsUser on windows at config
		// time; guard here too so the helper is safe to call unconditionally.
		return nil
	}
	for _, target := range scratchACLTargets(repoRoot, laneUser, supervisorID) {
		if err := setScratchACL(target.spec, target.path); err != nil {
			return fmt.Errorf("grant scratch ACL %s on %s: %w", target.spec, target.path, err)
		}
		if target.defACL {
			if err := setScratchDefaultACL(target.spec, target.path); err != nil {
				return fmt.Errorf("grant scratch default ACL %s on %s: %w", target.spec, target.path, err)
			}
		}
	}
	return nil
}
