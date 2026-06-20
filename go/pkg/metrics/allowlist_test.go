package metrics

import (
	"os"
	"reflect"
	"testing"
)

// allowlistPath is the committed manifest the guardrail pins and regenerates.
const allowlistPath = "metrics_allowlist.json"

// TestMetricsAllowlistMatchesRegistry is the CI guardrail (deliverable #3): the
// checked-in metrics_allowlist.json must match the live registry exactly — same
// families, same labels, same SHA-256. It runs under `go test ./...` / `make
// check`, so a drift fails CI. Regenerate the manifest deliberately with
// STRIATUM_UPDATE_ALLOWLIST=1 and review the diff.
func TestMetricsAllowlistMatchesRegistry(t *testing.T) {
	want := BuildAllowlist(DefaultRegistry())

	if os.Getenv("STRIATUM_UPDATE_ALLOWLIST") == "1" {
		bytes, err := MarshalAllowlist(want)
		if err != nil {
			t.Fatalf("marshal allowlist: %v", err)
		}
		if err := os.WriteFile(allowlistPath, bytes, 0o644); err != nil {
			t.Fatalf("write allowlist: %v", err)
		}
		t.Logf("updated %s", allowlistPath)
	}

	got, err := LoadEmbeddedAllowlist()
	if err != nil {
		t.Fatalf("load embedded allowlist: %v", err)
	}
	if got.SHA256 != want.SHA256 {
		t.Fatalf("allowlist sha256 drift: committed %s, live registry %s; regenerate with STRIATUM_UPDATE_ALLOWLIST=1 go test ./pkg/metrics/...", got.SHA256, want.SHA256)
	}
	if !reflect.DeepEqual(got.Families, want.Families) {
		t.Fatalf("allowlist families drift:\ncommitted %+v\nlive      %+v", got.Families, want.Families)
	}
	// The committed hash must be the real hash of the committed family set, not a
	// stale value pasted next to a changed family list.
	if got.SHA256 != DefaultRegistry().Hash() {
		t.Fatalf("committed sha256 %s does not match the live registry hash %s", got.SHA256, DefaultRegistry().Hash())
	}
}

// TestVerifyAllowlistDetectsDrift proves the boot-abort path: the live default
// registry verifies clean, but a registry with an extra family fails the same
// check the daemon runs before /metrics binds.
func TestVerifyAllowlistDetectsDrift(t *testing.T) {
	if err := VerifyAllowlist(); err != nil {
		t.Fatalf("VerifyAllowlist failed for the live registry: %v", err)
	}

	drift := DefaultRegistry()
	drift.MustRegister(Family{Name: "striatum_unreviewed_label", Type: TypeGauge, Classification: ClassificationOperational, Labels: []string{"run_id"}})
	if err := verifyAllowlistAgainst(drift); err == nil {
		t.Fatalf("verifyAllowlistAgainst accepted a drifted registry; want boot-abort error")
	}
}
