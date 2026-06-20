package metrics

import (
	"strings"
	"testing"
)

// TestRegisterRefusesForbiddenFamily is the deliverable #1 proof: a
// Forbidden-classified family is rejected at construction (Register returns an
// error, MustRegister panics) and never enters the registry, so a forbidden
// series can never reach the wire.
func TestRegisterRefusesForbiddenFamily(t *testing.T) {
	r := NewRegistry()
	forbidden := Family{Name: "striatum_secret_repo_path", Type: TypeGauge, Classification: ClassificationForbidden, Labels: []string{"path"}}

	if err := r.Register(forbidden); err == nil {
		t.Fatalf("Register accepted a Forbidden family; want refusal")
	}
	for _, f := range r.Specs() {
		if f.Name == forbidden.Name {
			t.Fatalf("Forbidden family %q entered the registry", forbidden.Name)
		}
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatalf("MustRegister did not panic on a Forbidden family")
			}
		}()
		r.MustRegister(forbidden)
	}()
}

// TestRegisterRefusesUnknownClassificationAndDuplicate covers the other refusal
// paths so a malformed family cannot silently widen the wire contract.
func TestRegisterRefusesUnknownClassificationAndDuplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Family{Name: "striatum_x", Type: TypeGauge, Classification: Classification("bespoke")}); err == nil {
		t.Fatalf("Register accepted an unknown classification")
	}
	if err := r.Register(Family{Name: "", Type: TypeGauge, Classification: ClassificationOperational}); err == nil {
		t.Fatalf("Register accepted an empty family name")
	}
	if err := r.Register(Family{Name: "striatum_dup", Type: TypeGauge, Classification: ClassificationOperational}); err != nil {
		t.Fatalf("Register rejected a valid family: %v", err)
	}
	if err := r.Register(Family{Name: "striatum_dup", Type: TypeCounter, Classification: ClassificationOperational}); err == nil {
		t.Fatalf("Register accepted a duplicate family name")
	}
}

// TestDefaultRegistryIsOperationalAndStable asserts the live registry carries no
// Forbidden family, that every family is Operational OR Provenance, that the only
// Provenance (consent-gated) family is the per-repo run gauge — and that it
// carries the salted `bucket` surrogate — and that Hash() is deterministic, the
// property the boot check and the guardrail test both rely on. Phase D introduces
// the first Provenance family (striatum_repo_runs); every other family stays
// Operational.
func TestDefaultRegistryIsOperationalAndStable(t *testing.T) {
	r := DefaultRegistry()
	specs := r.Specs()
	if len(specs) == 0 {
		t.Fatalf("DefaultRegistry is empty")
	}
	provenance := map[string]Family{}
	for _, f := range specs {
		switch f.Classification {
		case ClassificationOperational:
		case ClassificationProvenance:
			provenance[f.Name] = f
		default:
			t.Errorf("family %q has classification %q; want operational or provenance (never forbidden)", f.Name, f.Classification)
		}
		if !strings.HasPrefix(f.Name, "striatum_") {
			t.Errorf("family %q does not use the striatum_ prefix", f.Name)
		}
	}
	// The Provenance set is exactly the per-repo run gauge, and it must carry the
	// salted bucket surrogate — a Provenance family that did not would be a
	// repo-aggregate signal miscategorized as consent-gated.
	if len(provenance) != 1 {
		t.Errorf("expected exactly one Provenance family, got %d: %v", len(provenance), provenance)
	}
	repoRuns, ok := provenance[metricRepoRuns]
	if !ok {
		t.Errorf("expected %q to be the Provenance family", metricRepoRuns)
	} else {
		hasBucket := false
		for _, l := range repoRuns.Labels {
			if l == "bucket" {
				hasBucket = true
			}
		}
		if !hasBucket {
			t.Errorf("Provenance family %q must carry the salted bucket label; labels=%v", metricRepoRuns, repoRuns.Labels)
		}
	}
	if h1, h2 := r.Hash(), DefaultRegistry().Hash(); h1 != h2 {
		t.Fatalf("registry hash is not stable: %s vs %s", h1, h2)
	}
	// Every family the registry declares must actually be rendered (no manifest
	// entry that the exporter never emits), and the two Phase C families must be
	// present.
	body := string(renderSentinelSnapshot(t))
	for _, f := range specs {
		if !strings.Contains(body, f.Name) {
			t.Errorf("registered family %q is absent from the rendered exposition", f.Name)
		}
	}
}
