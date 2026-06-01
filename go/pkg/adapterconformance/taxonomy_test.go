package adapterconformance

import "testing"

// TestTaxonomyPartitionTotalAndDisjoint is the taxonomy self-validation
// (DESIGN §1.2 / build step 2): every declared FailureClass belongs to exactly
// one partition (contract XOR infra), and the contract/infra index maps cover
// exactly the declared set with no extras.
func TestTaxonomyPartitionTotalAndDisjoint(t *testing.T) {
	all := AllFailureClasses()

	seen := map[FailureClass]bool{}
	for _, class := range all {
		if class == FailureNone {
			t.Fatalf("AllFailureClasses must not include the FailureNone zero value")
		}
		if seen[class] {
			t.Fatalf("FailureClass %q appears more than once in AllFailureClasses", class)
		}
		seen[class] = true

		isContract := class.IsContract()
		isInfra := class.IsInfra()

		// Total: every class is in at least one partition.
		if !isContract && !isInfra {
			t.Errorf("FailureClass %q is in NEITHER partition (taxonomy not total)", class)
		}
		// Disjoint: no class is in both partitions.
		if isContract && isInfra {
			t.Errorf("FailureClass %q is in BOTH partitions (taxonomy not disjoint)", class)
		}
	}

	// The contract/infra index maps must not carry any class missing from
	// AllFailureClasses (no orphan partition entries).
	for class := range contractClasses {
		if !seen[class] {
			t.Errorf("contractClasses carries %q which is not in AllFailureClasses", class)
		}
	}
	for class := range infraClasses {
		if !seen[class] {
			t.Errorf("infraClasses carries %q which is not in AllFailureClasses", class)
		}
	}

	// Count check: the two partitions together must total the declared set.
	if got, want := len(contractClasses)+len(infraClasses), len(all); got != want {
		t.Errorf("partition sizes sum to %d, want %d (every class in exactly one partition)", got, want)
	}
}

// TestFailureNoneIsNeitherPartition documents that the zero value belongs to
// neither partition (it represents a pass / not-run clause).
func TestFailureNoneIsNeitherPartition(t *testing.T) {
	if FailureNone.IsContract() || FailureNone.IsInfra() {
		t.Fatalf("FailureNone must belong to neither partition")
	}
}

// TestExpectedClassesPresent pins the exact contract + infra class sets from
// DESIGN §1.2 so a class cannot be silently added/dropped without updating the
// taxonomy on purpose.
func TestExpectedClassesPresent(t *testing.T) {
	wantContract := []FailureClass{
		AdapterContractViolation, EnvSecretLeak, BootstrapStall, SurveyPromptBlocked,
		DiscoveryProbeStall, AwaitPacketStall, AckStall, HeartbeatMissed,
		InterrogationIgnored, ArtifactMissing, ArtifactRejected, CompleteMissing,
		LoopWedged, NoWorkLoopMissed, TurnExitEarly, TokenLeakInWorkTree,
		ControlPlaneHelperInWorkTree, KnownGapExpired, UnexpectedKnownGapPass,
	}
	wantInfra := []FailureClass{
		AdapterUnavailable, AdapterVersionUnknown, AdapterUnauthenticated,
		AdapterProviderUnavailable,
	}
	for _, c := range wantContract {
		if !c.IsContract() {
			t.Errorf("expected %q to be a contract class", c)
		}
	}
	for _, c := range wantInfra {
		if !c.IsInfra() {
			t.Errorf("expected %q to be an infra class", c)
		}
	}
	if len(wantContract) != len(contractClasses) {
		t.Errorf("contract class count drift: pinned %d, declared %d", len(wantContract), len(contractClasses))
	}
	if len(wantInfra) != len(infraClasses) {
		t.Errorf("infra class count drift: pinned %d, declared %d", len(wantInfra), len(infraClasses))
	}
}
