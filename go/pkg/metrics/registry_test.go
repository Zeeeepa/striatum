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
// Provenance/Forbidden family in Phase C and that Hash() is deterministic — the
// property the boot check and the guardrail test both rely on.
func TestDefaultRegistryIsOperationalAndStable(t *testing.T) {
	r := DefaultRegistry()
	specs := r.Specs()
	if len(specs) == 0 {
		t.Fatalf("DefaultRegistry is empty")
	}
	for _, f := range specs {
		if f.Classification != ClassificationOperational {
			t.Errorf("Phase C family %q is %q; want operational", f.Name, f.Classification)
		}
		if !strings.HasPrefix(f.Name, "striatum_") {
			t.Errorf("family %q does not use the striatum_ prefix", f.Name)
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
