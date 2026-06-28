package mutations

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type scratchACLCall struct {
	op   string // "set" or "default"
	spec string
	path string
}

// stubScratchACL replaces the setfacl shell-outs with in-memory recorders so the
// ACL-prep decision logic can be unit-tested without root or the setfacl binary
// (mirrors stubDaemonSocketACL in cmd/striatumd).
func stubScratchACL(t *testing.T) *[]scratchACLCall {
	t.Helper()
	origSet := setScratchACL
	origDefault := setScratchDefaultACL
	calls := &[]scratchACLCall{}
	setScratchACL = func(spec, path string) error {
		*calls = append(*calls, scratchACLCall{op: "set", spec: spec, path: path})
		return nil
	}
	setScratchDefaultACL = func(spec, path string) error {
		*calls = append(*calls, scratchACLCall{op: "default", spec: spec, path: path})
		return nil
	}
	t.Cleanup(func() {
		setScratchACL = origSet
		setScratchDefaultACL = origDefault
	})
	return calls
}

func TestPrepareScratchACLsSkipsWhenRunAsUserEmpty(t *testing.T) {
	calls := stubScratchACL(t)
	if err := prepareScratchACLsForLaneUser("/repo", "", "sup_1"); err != nil {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("owner-run lane must not touch ACLs, got %#v", *calls)
	}
}

func TestPrepareScratchACLsSkipsWhenRepoRootEmpty(t *testing.T) {
	calls := stubScratchACL(t)
	if err := prepareScratchACLsForLaneUser("", "striatum-lane", "sup_1"); err != nil {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("empty repo root must not touch ACLs, got %#v", *calls)
	}
}

func TestPrepareScratchACLsGrantsTraverseAndSupervisorScratchRWXPlusDefault(t *testing.T) {
	calls := stubScratchACL(t)
	repoRoot := filepath.Join(string(filepath.Separator), "home", "halbritt", "git", "hippo")
	if err := prepareScratchACLsForLaneUser(repoRoot, "striatum-lane", "sup_1"); err != nil {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v", err)
	}
	striatumDir := filepath.Join(repoRoot, ".striatum")
	scratchDir := filepath.Join(striatumDir, "scratch")
	supervisorDir := filepath.Join(scratchDir, "sup_1")
	want := []scratchACLCall{
		{op: "set", spec: "u:striatum-lane:--x", path: striatumDir},
		{op: "set", spec: "u:striatum-lane:--x", path: scratchDir},
		{op: "set", spec: "u:striatum-lane:rwx", path: supervisorDir},
		{op: "default", spec: "u:striatum-lane:rwx", path: supervisorDir},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("scratch ACL calls = %#v, want %#v", *calls, want)
	}
}

func TestScratchACLTargetsIncludesSupervisorDirWithDefaultACL(t *testing.T) {
	repoRoot := filepath.Join(string(filepath.Separator), "repo")
	targets := scratchACLTargets(repoRoot, "striatum-lane", "sup_1")
	supervisorDir := filepath.Join(repoRoot, ".striatum", "scratch", "sup_1")
	var foundSupervisor bool
	for _, target := range targets {
		if target.path == supervisorDir {
			foundSupervisor = true
			if target.spec != "u:striatum-lane:rwx" {
				t.Fatalf("supervisor scratch spec = %q, want u:striatum-lane:rwx", target.spec)
			}
			if !target.defACL {
				t.Fatalf("supervisor scratch dir must request a default ACL (setfacl -d)")
			}
		}
	}
	if !foundSupervisor {
		t.Fatalf("scratch ACL plan must include %s, got %#v", supervisorDir, targets)
	}
}

func TestPrepareScratchACLsSurfacesSetfaclFailure(t *testing.T) {
	stubScratchACL(t)
	setScratchACL = func(spec, path string) error { return errors.New("setfacl failed") }
	err := prepareScratchACLsForLaneUser("/repo", "striatum-lane", "sup_1")
	if err == nil || !strings.Contains(err.Error(), "setfacl failed") {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v, want setfacl failure", err)
	}
}
