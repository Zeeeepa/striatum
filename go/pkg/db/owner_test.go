package db

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/sessionliveness"
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

func TestOwnerBundleNineAddsReviewGeneration(t *testing.T) {
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("OwnerBundles: %v", err)
	}
	var bundle *OwnerBundle
	for index := range bundles {
		if bundles[index].Version == 9 {
			bundle = &bundles[index]
			break
		}
	}
	if bundle == nil {
		t.Fatal("owner bundle 9 is missing")
	}
	for _, needle := range []string{
		"ALTER TABLE striatumd.jobs",
		"ALTER TABLE striatumd.verdicts",
		"ADD COLUMN IF NOT EXISTS review_generation integer NOT NULL DEFAULT 1",
	} {
		if !strings.Contains(bundle.SQL, needle) {
			t.Fatalf("bundle 9 missing %q", needle)
		}
	}
}

// TestOwnerBundleTenAddsWedgeStallClass is GH #324: the wedged_no_tool_progress
// liveness stall class is added to the sessions_liveness_stall_class_check CHECK.
// striatumd.sessions is an owner-held table, so widening the CHECK (DROP + re-ADD,
// which a CHECK cannot do in place) is owner-table DDL and MUST live in an owner
// bundle, never a runtime migration (RFC 0079 §5 / RFC 0081 owner-held ALTER
// crash-loop). The bundle must be idempotent so a re-run is a safe no-op.
func TestOwnerBundleTenAddsWedgeStallClass(t *testing.T) {
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("OwnerBundles: %v", err)
	}
	var bundle *OwnerBundle
	for index := range bundles {
		if bundles[index].Version == 10 {
			bundle = &bundles[index]
			break
		}
	}
	if bundle == nil {
		t.Fatal("owner bundle 10 is missing")
	}
	for _, needle := range []string{
		"sessions_liveness_stall_class_check",
		"DROP CONSTRAINT sessions_liveness_stall_class_check",
		"ADD CONSTRAINT sessions_liveness_stall_class_check",
		"'wedged_no_tool_progress'",
		"NOT LIKE '%wedged_no_tool_progress%'",
	} {
		if !strings.Contains(bundle.SQL, needle) {
			t.Fatalf("bundle 10 missing %q", needle)
		}
	}
	// Ownership-safety: it ALTERs the owner-held sessions table (that is the whole
	// point of the bundle), and it must NOT touch any other owner table.
	if !strings.Contains(bundle.SQL, "ALTER TABLE striatumd.sessions") {
		t.Fatal("bundle 10 must ALTER striatumd.sessions")
	}
}

// TestOwnerBundleTenWedgeClassMatchesClassifier keeps the persisted CHECK list in
// lockstep with the classifier constant — if sessionliveness renames the class,
// this fails until the bundle (and a new bundle version) follows.
func TestOwnerBundleTenWedgeClassMatchesClassifier(t *testing.T) {
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("OwnerBundles: %v", err)
	}
	var bundle *OwnerBundle
	for index := range bundles {
		if bundles[index].Version == 10 {
			bundle = &bundles[index]
			break
		}
	}
	if bundle == nil {
		t.Fatal("owner bundle 10 is missing")
	}
	if !strings.Contains(bundle.SQL, "'"+sessionliveness.StallToolProgress+"'") {
		t.Fatalf("bundle 10 CHECK must permit the classifier stall class %q", sessionliveness.StallToolProgress)
	}
}

// TestOwnerBundleTwelveAddsQuarantineState is GH #311 P0: the 'quarantined' job
// state is added to the jobs_state_check CHECK. striatumd.jobs is an owner-held
// table, so widening the CHECK (DROP + re-ADD, which a CHECK cannot do in place)
// is owner-table DDL and MUST live in an owner bundle, never a runtime migration
// (RFC 0079 §5 / RFC 0081 owner-held ALTER crash-loop). The bundle must be
// idempotent so a re-run is a safe no-op. Bundle 0011 is reserved for a
// concurrent change (GH #330), so this lands as 0012.
func TestOwnerBundleTwelveAddsQuarantineState(t *testing.T) {
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("OwnerBundles: %v", err)
	}
	var bundle *OwnerBundle
	for index := range bundles {
		if bundles[index].Version == 12 {
			bundle = &bundles[index]
			break
		}
	}
	if bundle == nil {
		t.Fatal("owner bundle 12 is missing")
	}
	for _, needle := range []string{
		"jobs_state_check",
		"DROP CONSTRAINT jobs_state_check",
		"ADD CONSTRAINT jobs_state_check",
		"'quarantined'",
		"NOT LIKE '%quarantined%'",
	} {
		if !strings.Contains(bundle.SQL, needle) {
			t.Fatalf("bundle 12 missing %q", needle)
		}
	}
	// Ownership-safety: it ALTERs the owner-held jobs table (that is the whole
	// point of the bundle), and it must NOT touch any other owner table.
	if !strings.Contains(bundle.SQL, "ALTER TABLE striatumd.jobs") {
		t.Fatal("bundle 12 must ALTER striatumd.jobs")
	}
}
