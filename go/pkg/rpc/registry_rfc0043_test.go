package rpc

import "testing"

func TestRFC0043CanonicalCapabilities(t *testing.T) {
	expected := map[string]Capability{
		"session.register":       CapabilityClaim,
		"session.report":         CapabilityClaim,
		"work.claim_next":        CapabilityClaim,
		"work.ack":               CapabilityClaim,
		"work.heartbeat":         CapabilityClaim,
		"work.complete":          CapabilityWrite,
		"work.block":             CapabilityWrite,
		"artifact.publish":       CapabilityWrite,
		"repo.write":             CapabilityWrite,
		"review.submit":          CapabilityReview,
		"review.verdict":         CapabilityReview,
		"decision.record":        CapabilityAdmin,
		"checkpoint.resolve":     CapabilityAdmin,
		"recovery.requeue_stale": CapabilityRecovery,
		"recovery.cancel_job":    CapabilityRecovery,
		"recovery.resume":        CapabilityRecovery,
		"worktree.create":        CapabilityWrite,
		"branch.confirm":         CapabilityAdmin,
		"run.prepare":            CapabilityAdmin,
		"run.start":              CapabilityAdmin,
		"run.pause":              CapabilityAdmin,
		"run.resume":             CapabilityAdmin,
		"run.cancel":             CapabilityAdmin,
		"workflow.validate":      CapabilityRead,
		"workflow.generate":      CapabilityWrite,
	}
	for method, capability := range expected {
		entry, ok := MethodRegistry[method]
		if !ok {
			t.Fatalf("missing method %s", method)
		}
		if entry.RequiredCapability == nil || *entry.RequiredCapability != capability {
			t.Fatalf("%s capability = %v, want %s", method, entry.RequiredCapability, capability)
		}
		if !entry.RepositoryScope {
			t.Fatalf("%s should be repository scoped", method)
		}
	}
}

func TestDeprecatedAliasesRemainFlagged(t *testing.T) {
	for _, method := range []string{"ack", "heartbeat", "release", "block", "complete", "publish_artifact", "claim_next", "verdict", "submit_review"} {
		entry, ok := MethodRegistry[method]
		if !ok {
			t.Fatalf("missing deprecated alias %s", method)
		}
		if !entry.Deprecated {
			t.Fatalf("%s should be marked deprecated", method)
		}
	}
}

func TestRepoResolveIsDaemonGlobalRead(t *testing.T) {
	entry, ok := MethodRegistry["repo.resolve"]
	if !ok {
		t.Fatal("missing method repo.resolve")
	}
	if entry.RequiredCapability == nil || *entry.RequiredCapability != CapabilityRead {
		t.Fatalf("repo.resolve capability = %v, want read", entry.RequiredCapability)
	}
	if entry.RepositoryScope || entry.RepositoryScopeMode != ScopeDaemonGlobal {
		t.Fatalf("repo.resolve scope = %v/%s, want daemon_global", entry.RepositoryScope, entry.RepositoryScopeMode)
	}
}
