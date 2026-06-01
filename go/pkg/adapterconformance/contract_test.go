package adapterconformance

import (
	"os"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

// TestContractOrderedC0ToC12 asserts the contract is the ordered C0..C12 list,
// each clause carrying its declared id, the documented OnFailure class, and a
// non-nil Assert.
func TestContractOrderedC0ToC12(t *testing.T) {
	clauses := Contract()
	if len(clauses) != 13 {
		t.Fatalf("contract must have 13 clauses C0..C12, got %d", len(clauses))
	}
	wantOnFailure := map[ClauseID]FailureClass{
		C0:  AdapterContractViolation,
		C1:  AdapterUnavailable,
		C2:  EnvSecretLeak,
		C3:  BootstrapStall,
		C4:  AwaitPacketStall,
		C5:  AckStall,
		C6:  HeartbeatMissed,
		C7:  InterrogationIgnored,
		C8:  ArtifactMissing,
		C9:  CompleteMissing,
		C10: NoWorkLoopMissed,
		C11: TurnExitEarly,
		C12: TokenLeakInWorkTree,
	}
	for i, c := range clauses {
		if int(c.ID) != i {
			t.Fatalf("clause at index %d has id %s, want C%d (out of order)", i, c.ID, i)
		}
		if c.Assert == nil {
			t.Errorf("clause %s has a nil Assert", c.ID)
		}
		if c.OnFailure != wantOnFailure[c.ID] {
			t.Errorf("clause %s OnFailure = %q, want %q", c.ID, c.OnFailure, wantOnFailure[c.ID])
		}
		if c.ID.Name() == "C?_Unknown" {
			t.Errorf("clause %s has no canonical name", c.ID)
		}
	}
}

// TestHermeticClausesRealRestDeferred asserts only C0 and C2 have real Slice-1
// asserts; every other declared clause returns StatusDeferred (never a faked
// result).
func TestHermeticClausesRealRestDeferred(t *testing.T) {
	spec := AdapterSpec{Name: "claude_code", Command: []string{"claude"}, Profile: Full}
	in := AssertInput{Adapter: spec}
	for _, c := range Contract() {
		res := c.Assert(in)
		if IsHermetic(c.ID) {
			if res.Status == StatusDeferred {
				t.Errorf("hermetic clause %s must not be deferred", c.ID)
			}
		} else {
			if res.Status != StatusDeferred {
				t.Errorf("live clause %s must be StatusDeferred in Slice 1, got %s", c.ID, res.Status)
			}
			if res.FailureClass != FailureNone {
				t.Errorf("deferred clause %s must carry no FailureClass, got %q", c.ID, res.FailureClass)
			}
		}
	}
}

// TestProfileScoping asserts the agy SingleShot profile skips the multi-turn
// clauses (C6, C7, C10, C11) and Full carries them.
func TestProfileScoping(t *testing.T) {
	multiTurnFullOnly := map[ClauseID]bool{C6: true, C7: true, C10: true, C11: true}
	for _, c := range Contract() {
		if multiTurnFullOnly[c.ID] {
			if !c.AppliesTo(Full) {
				t.Errorf("clause %s must apply to Full", c.ID)
			}
			if c.AppliesTo(SingleShot) {
				t.Errorf("clause %s must NOT apply to SingleShot (agy reduced profile)", c.ID)
			}
		}
	}
}

// TestDeadlineScaleDefaultAndOverride asserts the default 3.0 scale and the env
// override, and that live deadlines derive from DefaultPolicy() x scale.
func TestDeadlineScaleDefaultAndOverride(t *testing.T) {
	t.Setenv(DeadlineScaleEnv, "")
	if got := DeadlineScale(); got != DefaultDeadlineScale {
		t.Fatalf("default scale = %v, want %v", got, DefaultDeadlineScale)
	}

	t.Setenv(DeadlineScaleEnv, "5")
	if got := DeadlineScale(); got != 5.0 {
		t.Fatalf("override scale = %v, want 5.0", got)
	}

	t.Setenv(DeadlineScaleEnv, "garbage")
	if got := DeadlineScale(); got != DefaultDeadlineScale {
		t.Fatalf("unparseable scale must fall back to default, got %v", got)
	}

	t.Setenv(DeadlineScaleEnv, "0")
	if got := DeadlineScale(); got != DefaultDeadlineScale {
		t.Fatalf("non-positive scale must fall back to default, got %v", got)
	}

	// Live clause deadlines derive from DefaultPolicy x scale. C3 = Discovery.
	_ = os.Unsetenv(DeadlineScaleEnv)
	t.Setenv(DeadlineScaleEnv, "3")
	policy := sessionliveness.DefaultPolicy()
	var c3 Clause
	for _, c := range Contract() {
		if c.ID == C3 {
			c3 = c
		}
	}
	wantC3 := time.Duration(policy.DiscoverySeconds) * time.Second * 3
	if c3.Deadline != wantC3 {
		t.Fatalf("C3 deadline = %v, want %v (Discovery %ds x 3)", c3.Deadline, wantC3, policy.DiscoverySeconds)
	}
	// Hermetic clauses carry no wall-clock budget.
	for _, c := range Contract() {
		if IsHermetic(c.ID) && c.Deadline != 0 {
			t.Errorf("hermetic clause %s must have zero deadline, got %v", c.ID, c.Deadline)
		}
	}
}

// TestAdaptersDeclared asserts the three real adapters and their profiles.
func TestAdaptersDeclared(t *testing.T) {
	adapters := Adapters()
	byName := map[string]AdapterSpec{}
	for _, a := range adapters {
		byName[a.Name] = a
	}
	for name, wantProfile := range map[string]ContractProfile{
		"claude_code": Full,
		"codex":       Full,
		"agy":         SingleShot,
	} {
		spec, ok := byName[name]
		if !ok {
			t.Fatalf("adapter %q not declared", name)
		}
		if spec.Profile != wantProfile {
			t.Errorf("adapter %q profile = %q, want %q", name, spec.Profile, wantProfile)
		}
		if len(spec.Command) == 0 {
			t.Errorf("adapter %q has empty command", name)
		}
	}
}
