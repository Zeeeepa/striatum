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
	if err := prepareScratchACLsForLaneUser("/repo", ""); err != nil {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("owner-run lane must not touch ACLs, got %#v", *calls)
	}
}

func TestPrepareScratchACLsSkipsWhenRepoRootEmpty(t *testing.T) {
	calls := stubScratchACL(t)
	if err := prepareScratchACLsForLaneUser("", "striatum-lane"); err != nil {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("empty repo root must not touch ACLs, got %#v", *calls)
	}
}

func TestPrepareScratchACLsGrantsTraverseAndScratchRWXPlusDefault(t *testing.T) {
	calls := stubScratchACL(t)
	repoRoot := filepath.Join(string(filepath.Separator), "home", "halbritt", "git", "hippo")
	if err := prepareScratchACLsForLaneUser(repoRoot, "striatum-lane"); err != nil {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v", err)
	}
	striatumDir := filepath.Join(repoRoot, ".striatum")
	scratchDir := filepath.Join(striatumDir, "scratch")
	want := []scratchACLCall{
		{op: "set", spec: "u:striatum-lane:--x", path: striatumDir},
		{op: "set", spec: "u:striatum-lane:rwx", path: scratchDir},
		{op: "default", spec: "u:striatum-lane:rwx", path: scratchDir},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("scratch ACL calls = %#v, want %#v", *calls, want)
	}
}

func TestScratchACLTargetsIncludesScratchDirWithDefaultACL(t *testing.T) {
	repoRoot := filepath.Join(string(filepath.Separator), "repo")
	targets := scratchACLTargets(repoRoot, "striatum-lane")
	scratchDir := filepath.Join(repoRoot, ".striatum", "scratch")
	var foundScratch bool
	for _, target := range targets {
		if target.path == scratchDir {
			foundScratch = true
			if target.spec != "u:striatum-lane:rwx" {
				t.Fatalf("scratch dir spec = %q, want u:striatum-lane:rwx", target.spec)
			}
			if !target.defACL {
				t.Fatalf("scratch dir must request a default ACL (setfacl -d)")
			}
		}
	}
	if !foundScratch {
		t.Fatalf("scratch ACL plan must include %s, got %#v", scratchDir, targets)
	}
}

func TestPrepareScratchACLsSurfacesSetfaclFailure(t *testing.T) {
	stubScratchACL(t)
	setScratchACL = func(spec, path string) error { return errors.New("setfacl failed") }
	err := prepareScratchACLsForLaneUser("/repo", "striatum-lane")
	if err == nil || !strings.Contains(err.Error(), "setfacl failed") {
		t.Fatalf("prepareScratchACLsForLaneUser() error = %v, want setfacl failure", err)
	}
}
