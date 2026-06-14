package db

import (
	"strings"
	"testing"
)

func TestOwnerBundleSevenAddsArtifactPlacement(t *testing.T) {
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("OwnerBundles: %v", err)
	}
	var bundle *OwnerBundle
	for index := range bundles {
		if bundles[index].Version == 7 {
			bundle = &bundles[index]
			break
		}
	}
	if bundle == nil {
		t.Fatal("owner bundle 7 is missing")
	}
	for _, needle := range []string{
		"ALTER TABLE striatumd.artifacts",
		"ADD COLUMN IF NOT EXISTS placement text",
		"artifacts_placement_check",
		"p_placement text",
		"NULLIF(p_placement, '')",
	} {
		if !strings.Contains(bundle.SQL, needle) {
			t.Fatalf("bundle 7 missing %q", needle)
		}
	}
}

func TestOwnerBundleEightAddsArtifactSearch(t *testing.T) {
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("OwnerBundles: %v", err)
	}
	var bundle *OwnerBundle
	for index := range bundles {
		if bundles[index].Version == 8 {
			bundle = &bundles[index]
			break
		}
	}
	if bundle == nil {
		t.Fatal("owner bundle 8 is missing")
	}
	for _, needle := range []string{
		"ADD COLUMN IF NOT EXISTS search_vector tsvector",
		"GENERATED ALWAYS AS",
		"to_tsvector('english'",
		"CREATE INDEX IF NOT EXISTS idx_artifacts_search",
		"USING GIN (search_vector)",
	} {
		if !strings.Contains(bundle.SQL, needle) {
			t.Fatalf("bundle 8 missing %q", needle)
		}
	}
}
