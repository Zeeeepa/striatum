package db

import (
	"errors"
	"strings"
	"testing"
)

// TestBuildPlanOrdersOwnerThenRuntimeThenRevokeTerminal proves the C3 ordering:
// pending non-revoke owner bundles, then pending runtime migrations, then — when
// the binary embeds the DDL-revoke — the revoke LAST (and only then). StepIndex is
// dense and ascending (BC-N1: stable by storage). (F1 plan-shape arm.)
func TestBuildPlanOrdersOwnerThenRuntimeThenRevokeTerminal(t *testing.T) {
	plan, err := BuildPlan(0, 0)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("BuildPlan(0,0) produced no steps")
	}
	// Dense ascending indices.
	for i, s := range plan.Steps {
		if s.StepIndex != i {
			t.Fatalf("step %d has StepIndex %d; indices must be dense+ascending (BC-N1)", i, s.StepIndex)
		}
		if s.Role != DeployRoleOwner && s.Role != DeployRoleRuntime {
			t.Fatalf("step %d has unknown role %q", i, s.Role)
		}
	}
	// No interior owner step may appear AFTER a runtime step, EXCEPT the terminal
	// revoke (which is the very last step by construction).
	seenRuntime := false
	for i, s := range plan.Steps {
		isTerminalRevoke := plan.RevokeStepIndex >= 0 && i == plan.RevokeStepIndex
		if s.Role == DeployRoleRuntime {
			seenRuntime = true
		}
		if s.Role == DeployRoleOwner && seenRuntime && !isTerminalRevoke {
			t.Fatalf("non-revoke owner step %q at index %d follows a runtime step; owner bundles must precede runtime migrations", s.StepID, i)
		}
	}
	revoke, err := RevokeBundleEmbedded()
	if err != nil {
		t.Fatalf("RevokeBundleEmbedded: %v", err)
	}
	if revoke {
		if plan.RevokeStepIndex != len(plan.Steps)-1 {
			t.Fatalf("revoke embedded but RevokeStepIndex=%d (want last=%d)", plan.RevokeStepIndex, len(plan.Steps)-1)
		}
		last := plan.Steps[len(plan.Steps)-1]
		if !strings.HasPrefix(last.StepID, "0021") || last.Role != DeployRoleOwner {
			t.Fatalf("terminal step is %q/%s; want the 0021 owner revoke last", last.StepID, last.Role)
		}
	} else if plan.RevokeStepIndex != -1 {
		t.Fatalf("no revoke embedded but RevokeStepIndex=%d (want -1)", plan.RevokeStepIndex)
	}
}

// TestPlanHashIsDeterministicAndBaseSensitive proves plan identity is a stable,
// content-addressed fact (BC-N1): the same base produces the same hash; a
// different base produces a different hash.
func TestPlanHashIsDeterministicAndBaseSensitive(t *testing.T) {
	a, err := BuildPlan(0, 0)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	b, err := BuildPlan(0, 0)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if a.PlanHash == "" {
		t.Fatal("empty plan_hash")
	}
	if a.PlanHash != b.PlanHash {
		t.Fatalf("plan_hash not deterministic: %s vs %s", a.PlanHash, b.PlanHash)
	}
	c, err := BuildPlan(5, 10)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if a.PlanHash == c.PlanHash {
		t.Fatal("plan_hash did not change with a different base watermark")
	}
}

// TestVerifyStoredTranscriptCleanPlanPasses: a freshly-built plan's stored shas
// match the binary's embedded bytes, so M1 verification is clean.
func TestVerifyStoredTranscriptCleanPlanPasses(t *testing.T) {
	plan, err := BuildPlan(0, 0)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if err := VerifyStoredTranscript(plan); err != nil {
		t.Fatalf("VerifyStoredTranscript on a clean plan: %v", err)
	}
}

// TestVerifyStoredTranscriptDetectsBinaryMismatch is the M1 byte arm (F15 pure
// core): a stored step whose sha256 diverges from the binary's embedded bytes is
// forced to a typed deploy_plan_binary_mismatch, and a missing step_id likewise.
func TestVerifyStoredTranscriptDetectsBinaryMismatch(t *testing.T) {
	plan, err := BuildPlan(0, 0)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	// Tamper a stored sha (the resume binary disagrees with the stored transcript).
	tampered := *plan
	tampered.Steps = append([]DeployStep(nil), plan.Steps...)
	tampered.Steps[0].SHA256 = strings.Repeat("0", 64)
	err = VerifyStoredTranscript(&tampered)
	if err == nil {
		t.Fatal("VerifyStoredTranscript accepted a tampered transcript")
	}
	if !errors.Is(err, ErrDeployPlanBinaryMismatch) {
		t.Fatalf("want ErrDeployPlanBinaryMismatch, got %v", err)
	}
	var typed *DeployPlanBinaryMismatchError
	if !errors.As(err, &typed) {
		t.Fatalf("want *DeployPlanBinaryMismatchError, got %T", err)
	}

	// An unknown step_id (the binary embeds no such file) is also a binary mismatch.
	missing := *plan
	missing.Steps = []DeployStep{{StepIndex: 0, StepID: "9999_not_embedded.sql", Role: DeployRoleRuntime, SHA256: strings.Repeat("a", 64)}}
	if err := VerifyStoredTranscript(&missing); !errors.Is(err, ErrDeployPlanBinaryMismatch) {
		t.Fatalf("unknown step_id: want ErrDeployPlanBinaryMismatch, got %v", err)
	}
}

// TestReceiptRowHashChains proves the per-step receipt hash chain links prev→this
// and is sensitive to the step identity (a reorder or gap breaks the chain).
func TestReceiptRowHashChains(t *testing.T) {
	s0 := DeployStep{StepIndex: 0, StepID: "0001_a.sql", Role: DeployRoleRuntime, SHA256: "aa"}
	s1 := DeployStep{StepIndex: 1, StepID: "0002_b.sql", Role: DeployRoleRuntime, SHA256: "bb"}
	h0 := receiptRowHash("", "plan", s0)
	h1 := receiptRowHash(h0, "plan", s1)
	if h0 == "" || h1 == "" || h0 == h1 {
		t.Fatalf("receipt hashes degenerate: h0=%q h1=%q", h0, h1)
	}
	// A different predecessor changes the link (chain sensitivity).
	if receiptRowHash("tampered", "plan", s1) == h1 {
		t.Fatal("receipt row hash did not depend on prev_hash")
	}
	// A different plan_hash changes the link.
	if receiptRowHash(h0, "other-plan", s1) == h1 {
		t.Fatal("receipt row hash did not depend on plan_hash")
	}
}

// TestDeployTypedHaltsUnwrapToSentinels guards the errors.Is/As contract the boot
// path relies on (main.go joins these to the non-restartable exit).
func TestDeployTypedHaltsUnwrapToSentinels(t *testing.T) {
	cases := []struct {
		err      error
		sentinel error
	}{
		{&AwaitingDeployError{CursorState: DeployStateInProgress}, ErrAwaitingDeploy},
		{&AwaitingDeployConfigError{}, ErrAwaitingDeployConfig},
		{&DeployPlanBinaryMismatchError{}, ErrDeployPlanBinaryMismatch},
		{&DeployPlanDBStampMismatchError{}, ErrDeployPlanDBStampMismatch},
	}
	for _, tc := range cases {
		if !errors.Is(tc.err, tc.sentinel) {
			t.Fatalf("%T does not unwrap to its sentinel %v", tc.err, tc.sentinel)
		}
		if tc.err.Error() == "" {
			t.Fatalf("%T renders an empty message", tc.err)
		}
	}
}
