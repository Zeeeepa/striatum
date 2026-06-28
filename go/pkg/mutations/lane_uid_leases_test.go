package mutations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyProcStateBlocksStoppedTracedLiveAndUnknown(t *testing.T) {
	for _, state := range []byte{'R', 'S', 'D', 'T', 't', 'I', 'P'} {
		got, err := classifyProcState(state)
		if err != nil {
			t.Fatalf("classifyProcState(%q): %v", state, err)
		}
		if got != "live" {
			t.Fatalf("classifyProcState(%q) = %q, want live", state, got)
		}
	}
	for _, state := range []byte{'Z', 'X', 'x'} {
		got, err := classifyProcState(state)
		if err != nil {
			t.Fatalf("classifyProcState(%q): %v", state, err)
		}
		if got != "zombie_or_dead" {
			t.Fatalf("classifyProcState(%q) = %q, want zombie_or_dead", state, got)
		}
	}
	if _, err := classifyProcState('W'); err == nil {
		t.Fatalf("classifyProcState(W) succeeded; unknown proc states must fail closed")
	}
}

func TestScrubLaneUIDArtifactsFailsClosedOnKillFailure(t *testing.T) {
	origExec := laneUIDExec
	origRemoveAll := laneUIDRemoveAll
	origHome := laneOSUserHome
	t.Cleanup(func() {
		laneUIDExec = origExec
		laneUIDRemoveAll = origRemoveAll
		laneOSUserHome = origHome
	})

	home := t.TempDir()
	repo := t.TempDir()
	laneUIDExec = func(command string, args ...string) error {
		return errors.New("kill refused")
	}
	laneUIDRemoveAll = func(path string) error {
		return nil
	}
	laneOSUserHome = func(name string) string {
		if name == "lane-pool-1" {
			return home
		}
		return ""
	}

	proof := baseLaneUIDScrubProof(1234, "lane-pool-1", "sup_1")
	err := scrubLaneUIDArtifacts("lane-pool-1", "sup_1", repo, proof)
	if err == nil || !strings.Contains(err.Error(), "kill lane uid process domain") {
		t.Fatalf("err = %v, want fail-closed kill error", err)
	}
	checks := proof["checks"].(map[string]any)
	if got := checks["s1_kill_all"]; !strings.Contains(got.(string), "failed") {
		t.Fatalf("s1_kill_all = %#v, want failed diagnostic", got)
	}
	if _, ok := checks["p4_acl_worktree_cleanup"]; ok {
		t.Fatalf("P4 must not be a deferred placeholder: %#v", checks)
	}
	if _, ok := checks["p5_proof_recorded"]; ok {
		t.Fatalf("P5 must not be unconditional: %#v", checks)
	}
}

func TestLaneUIDHomeCleanupProofFailsWhenCredentialStoreRemains(t *testing.T) {
	origHome := laneOSUserHome
	t.Cleanup(func() { laneOSUserHome = origHome })

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir residue: %v", err)
	}
	laneOSUserHome = func(name string) string {
		if name == "lane-pool-1" {
			return home
		}
		return ""
	}

	checks := map[string]any{}
	err := proveLaneUIDHomeCleanup("lane-pool-1", checks)
	if err == nil || !strings.Contains(err.Error(), ".claude") {
		t.Fatalf("err = %v, want credential residue proof failure", err)
	}
	if checks["p3_p4_home_cleanup"] == nil {
		t.Fatalf("proof failure should be recorded in checks: %#v", checks)
	}
}
