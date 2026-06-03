package adapterconformance

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

// TestSupportedShapesHaveReliabilityFixture is the RFC 0106 graduation guard: a
// shape may carry support_tier=`supported` in the catalog ONLY if it has a green
// RFC 0105 reliability fixture registered in ReliabilityFixtureShapes (this
// package) — so the support tier cannot lie. It also asserts the reverse (every
// fixture-backed shape is marked supported), so the catalog classification and
// the fixture registry can never silently drift apart. Hermetic (no PG): it
// only loads the embedded catalog and checks the registry, so it runs in the
// default `make -C go test` tier too.
func TestSupportedShapesHaveReliabilityFixture(t *testing.T) {
	catalog, err := workflowtemplates.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	catalogSupported := map[string]bool{}
	for _, shape := range catalog.Shapes {
		id, _ := shape["template_id"].(string)
		tier, _ := shape["support_tier"].(string)
		if tier == "" {
			t.Errorf("shape %q has no support_tier stamped (RFC 0106); every shape must be classified", id)
			continue
		}
		if tier != workflowtemplates.SupportTierSupported {
			continue
		}
		catalogSupported[id] = true
		if !ReliabilityFixtureShapes[id] {
			t.Errorf("shape %q is marked support_tier=supported but has NO green RFC 0105 reliability fixture (ReliabilityFixtureShapes) — the tier cannot lie; add a fixture or mark it experimental", id)
		}
	}

	for id := range ReliabilityFixtureShapes {
		if !catalogSupported[id] {
			t.Errorf("shape %q has a reliability fixture but is NOT marked support_tier=supported in the catalog — graduate it (workflowtemplates.supportedShapes) or remove the fixture entry", id)
		}
	}
}
