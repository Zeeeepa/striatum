package db

import (
	"context"
	"os"
	"strings"
	"testing"
)

// recordingTx records every SQL string executed in the transaction so a test can
// assert which owner bundles reached applyOneOwnerBundle (which Exec's the bundle
// SQL). It is a canned fake (no real DB) for the M2 synthetic-phase F16a test.
type recordingTx struct{ execed *[]string }

func (t *recordingTx) Exec(_ context.Context, sql string, _ ...any) error {
	*t.execed = append(*t.execed, sql)
	return nil
}
func (t *recordingTx) QueryRow(context.Context, string, ...any) Row { return nil }
func (t *recordingTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}
func (t *recordingTx) Commit(context.Context) error   { return nil }
func (t *recordingTx) Rollback(context.Context) error { return nil }

type recordingRunner struct{ execed []string }

func (r *recordingRunner) Exec(context.Context, string, ...any) error   { return nil }
func (r *recordingRunner) QueryRow(context.Context, string, ...any) Row { return nil }
func (r *recordingRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "0", nil
}
func (r *recordingRunner) BeginTx(context.Context) (TxRunner, error) {
	return &recordingTx{execed: &r.execed}, nil
}

const syntheticRevokeMarker = "-- SYNTHETIC 0021 REVOKE CREATE (must never be owner-ddl applied)"

// TestOwnerDDLApplyExcludesSyntheticRevokeBundle is the M2 F16a synthetic-phase
// test (PROPOSAL §3.2a / §5 F16a). It drives the non-revoke filter through a
// HAND-BUILT bundle list containing a synthetic {Version: 21} entry, WITHOUT
// asserting that production OwnerBundles() contains 0021 (it does not until the
// build's step 7). It proves the filter + both apply loops + the nil-fallback all
// exclude the DDL-revoke bundle.
func TestOwnerDDLApplyExcludesSyntheticRevokeBundle(t *testing.T) {
	ctx := context.Background()

	// (predicate) isNonRevokeBundle excludes exactly the revoke frontier.
	if !isNonRevokeBundle(LatestOwnerBundleVersion) {
		t.Fatalf("isNonRevokeBundle(%d) = false; the latest non-revoke bundle must be apply-eligible", LatestOwnerBundleVersion)
	}
	if isNonRevokeBundle(DDLRevokeOwnerBundleVersion) {
		t.Fatalf("isNonRevokeBundle(%d) = true; the DDL-revoke bundle must be excluded", DDLRevokeOwnerBundleVersion)
	}

	// (a) OwnerDDLApplyBundles never surfaces a bundle >= the revoke frontier.
	applyEligible, err := OwnerDDLApplyBundles()
	if err != nil {
		t.Fatalf("OwnerDDLApplyBundles: %v", err)
	}
	for _, b := range applyEligible {
		if b.Version >= DDLRevokeOwnerBundleVersion {
			t.Fatalf("OwnerDDLApplyBundles surfaced revoke-frontier bundle %d; it must be filtered out", b.Version)
		}
	}

	synthetic := []OwnerBundle{
		{Version: 18, Label: "synthetic non-revoke", SQL: "-- v18 non-revoke"},
		{Version: 20, Label: "synthetic non-revoke", SQL: "-- v20 non-revoke"},
		{Version: DDLRevokeOwnerBundleVersion, Label: "synthetic revoke", SQL: syntheticRevokeMarker},
	}

	// (b1) applyPendingOwnerBundles skips the hand-passed synthetic 0021.
	pending := &recordingRunner{}
	if _, _, err := applyPendingOwnerBundles(ctx, pending, synthetic, 17, "synthetic"); err != nil {
		t.Fatalf("applyPendingOwnerBundles: %v", err)
	}
	assertRevokeNotApplied(t, "applyPendingOwnerBundles", pending.execed)
	if !containsExec(pending.execed, "-- v18 non-revoke") || !containsExec(pending.execed, "-- v20 non-revoke") {
		t.Fatalf("applyPendingOwnerBundles skipped a non-revoke bundle it should have applied: %v", pending.execed)
	}

	// (b2) ReapplyAllOwnerBundles skips the hand-passed synthetic 0021.
	reapply := &recordingRunner{}
	if _, err := ReapplyAllOwnerBundles(ctx, reapply, synthetic, "synthetic"); err != nil {
		t.Fatalf("ReapplyAllOwnerBundles: %v", err)
	}
	assertRevokeNotApplied(t, "ReapplyAllOwnerBundles", reapply.execed)

	// (c) ReapplyAllOwnerBundles(nil, …) resolves its fallback to the filtered
	// loader: no bundle >= the revoke frontier reaches applyOneOwnerBundle.
	fallback := &recordingRunner{}
	if _, err := ReapplyAllOwnerBundles(ctx, fallback, nil, "synthetic"); err != nil {
		t.Fatalf("ReapplyAllOwnerBundles(nil): %v", err)
	}
	assertRevokeNotApplied(t, "ReapplyAllOwnerBundles(nil-fallback)", fallback.execed)
}

func assertRevokeNotApplied(t *testing.T, route string, execed []string) {
	t.Helper()
	if containsExec(execed, syntheticRevokeMarker) {
		t.Fatalf("%s applied the synthetic DDL-revoke bundle; M2 requires it be excluded from every owner-ddl apply route", route)
	}
}

func containsExec(execed []string, marker string) bool {
	for _, sql := range execed {
		if strings.Contains(sql, marker) {
			return true
		}
	}
	return false
}

// TestOwnerBundle0021ProductionEmbedListingSplit is the F16b production-phase
// arm (M4, step 7): once 0021 is authored, production OwnerBundles() DOES embed
// it (so ExpectedFingerprint, RevokeBundleEmbedded, and BuildPlan see it), but the
// owner-ddl apply slice OwnerDDLApplyBundles() EXCLUDES it — the embed/listing
// split. The forced FMA-007 self-heal two-role assertion is the pgtest arm
// (deploy_pg_test.go), gated on STRIATUM_PG_TEST_URL.
func TestOwnerBundle0021ProductionEmbedListingSplit(t *testing.T) {
	bundles, err := OwnerBundles()
	if err != nil {
		t.Fatalf("OwnerBundles: %v", err)
	}
	var revoke *OwnerBundle
	for i := range bundles {
		if bundles[i].Version == DDLRevokeOwnerBundleVersion {
			revoke = &bundles[i]
		}
	}
	if revoke == nil {
		t.Fatalf("production OwnerBundles() does not embed bundle %d (the DDL-revoke); step 7 must author it", DDLRevokeOwnerBundleVersion)
	}
	if revoke.SHA256() == "" {
		t.Fatal("DDL-revoke bundle has an empty SHA256")
	}
	if !strings.Contains(revoke.SQL, "REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw") {
		t.Fatalf("DDL-revoke bundle SQL does not revoke CREATE from striatumd_rw:\n%s", revoke.SQL)
	}

	// (b) RevokeBundleEmbedded derives from file presence in the embed.
	embedded, err := RevokeBundleEmbedded()
	if err != nil {
		t.Fatalf("RevokeBundleEmbedded: %v", err)
	}
	if !embedded {
		t.Fatal("RevokeBundleEmbedded() = false but OwnerBundles() embeds the revoke")
	}

	// (c) production OwnerDDLApplyBundles() excludes 0021 (the listing split).
	apply, err := OwnerDDLApplyBundles()
	if err != nil {
		t.Fatalf("OwnerDDLApplyBundles: %v", err)
	}
	for _, b := range apply {
		if b.Version == DDLRevokeOwnerBundleVersion {
			t.Fatal("production OwnerDDLApplyBundles() surfaced the DDL-revoke; the listing split is broken")
		}
	}

	// (d) the watermark frontier stays 20 even though the highest embedded bundle
	// is 21 — the revoke is deploy-plan-terminal, NOT a watermark advance.
	if LatestOwnerBundleVersion != 20 || RequiredOwnerBundleVersion != 20 {
		t.Fatalf("watermark frontier moved: Latest=%d Required=%d, want 20/20 (the revoke must not advance the watermark)",
			LatestOwnerBundleVersion, RequiredOwnerBundleVersion)
	}

	// (e) BuildPlan places the embedded revoke LAST as the terminal step.
	plan, err := BuildPlan(0, 0)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.RevokeStepIndex != len(plan.Steps)-1 {
		t.Fatalf("BuildPlan RevokeStepIndex=%d, want terminal %d", plan.RevokeStepIndex, len(plan.Steps)-1)
	}
}

// TestOwnerDDLApplyRoutesUseFilteredLoader is the M4 build-time grep guard: the
// only `OwnerBundles()` (full-loader) call sites inside owner.go are the
// non-apply consumers OwnerDDLApplyBundles and RevokeBundleEmbedded. Every
// `owner-ddl apply` route must load the filtered OwnerDDLApplyBundles() so the
// DDL-revoke bundle cannot be committed through the apply path.
func TestOwnerDDLApplyRoutesUseFilteredLoader(t *testing.T) {
	body, err := os.ReadFile("owner.go")
	if err != nil {
		t.Fatalf("read owner.go: %v", err)
	}
	allowedFullLoaderFuncs := map[string]bool{
		"OwnerDDLApplyBundles": true,
		"RevokeBundleEmbedded": true,
	}
	currentFunc := ""
	for _, line := range strings.Split(string(body), "\n") {
		if name := funcNameOf(line); name != "" {
			currentFunc = name
			continue // the declaration line itself is not a call site
		}
		// Only code lines count: a doc comment that merely mentions the loader is
		// not a call site.
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if strings.Contains(line, "OwnerBundles()") {
			if !allowedFullLoaderFuncs[currentFunc] {
				t.Fatalf("owner.go func %q calls the full OwnerBundles() loader; an owner-ddl apply route must call OwnerDDLApplyBundles() so the DDL-revoke bundle stays excluded (M2/M4):\n  %s", currentFunc, strings.TrimSpace(line))
			}
		}
	}
}

// funcNameOf extracts the function name from a Go top-level `func Name(` or
// `func (recv) Name(` declaration line, or "" when the line is not a declaration.
func funcNameOf(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "func ") {
		return ""
	}
	rest := strings.TrimPrefix(trimmed, "func ")
	if strings.HasPrefix(rest, "(") { // method: skip the receiver
		if idx := strings.Index(rest, ")"); idx >= 0 {
			rest = strings.TrimSpace(rest[idx+1:])
		}
	}
	if idx := strings.IndexAny(rest, "("); idx >= 0 {
		return strings.TrimSpace(rest[:idx])
	}
	return ""
}
