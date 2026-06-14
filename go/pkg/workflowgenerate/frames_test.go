package workflowgenerate

import "testing"

// TestDefaultFramesWellFormed asserts every curated frame is fully populated and
// that operation frames carry the transform-then-generate fields.
func TestDefaultFramesWellFormed(t *testing.T) {
	seen := map[string]struct{}{}
	categories := map[string]struct{}{}
	for _, f := range defaultFrames {
		if f.ID == "" || f.Vantage == "" || f.Category == "" || f.Kind == "" {
			t.Errorf("frame %+v missing a required field", f)
		}
		if _, dup := seen[f.ID]; dup {
			t.Errorf("duplicate frame id %q", f.ID)
		}
		seen[f.ID] = struct{}{}
		if len(f.Dimensions) == 0 {
			t.Errorf("frame %q has no dimensions (anti-redundancy gate needs at least one)", f.ID)
		}
		if f.Kind == "operation" {
			if f.Transform == "" || f.MinStructure == "" {
				t.Errorf("operation frame %q must carry Transform and MinStructure", f.ID)
			}
		}
		categories[f.Category] = struct{}{}
	}
	for _, want := range []string{"persona", "operation", "temporal_forensic", "risk_pricing"} {
		if _, ok := categories[want]; !ok {
			t.Errorf("curated library missing category %q", want)
		}
	}
}

// TestSelectFramesDeterministicAndDistinct asserts selection is reproducible from
// the seed, returns the requested count, includes at least one wild frame, and
// never places two frames sharing >=2 distortion axes together (the RFC 0129
// anti-redundancy invariant).
func TestSelectFramesDeterministicAndDistinct(t *testing.T) {
	a := selectFrames("workflow-xyz", 5, "medium")
	b := selectFrames("workflow-xyz", 5, "medium")
	if len(a) != 5 {
		t.Fatalf("selectFrames count = %d, want 5", len(a))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("selection not deterministic for the same seed: %v vs %v", frameIDs(a), frameIDs(b))
		}
	}
	if !anyWild(a) {
		t.Errorf("selection has no wild frame: %v", frameIDs(a))
	}
	ids := map[string]struct{}{}
	for _, f := range a {
		if _, dup := ids[f.ID]; dup {
			t.Errorf("duplicate frame in selection: %q", f.ID)
		}
		ids[f.ID] = struct{}{}
	}
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if n := sharedDimensionCount(a[i], a[j]); n >= 2 {
				t.Errorf("frames %q and %q share %d distortion dimensions (>=2 violates anti-redundancy)", a[i].ID, a[j].ID, n)
			}
		}
	}
}

// TestSelectFramesVaryAcrossSeeds asserts different workflow ids explore
// different frame sets (the testable replacement for ADHD's nondeterministic
// "vary across generations").
func TestSelectFramesVaryAcrossSeeds(t *testing.T) {
	a := frameIDs(selectFrames("alpha", 5, "medium"))
	b := frameIDs(selectFrames("bravo", 5, "medium"))
	same := true
	for i := range a {
		if i >= len(b) || a[i] != b[i] {
			same = false
			break
		}
	}
	if same {
		t.Errorf("different seeds produced identical selections: %v", a)
	}
}

// TestSelectFramesMinStructureGate asserts operation frames are excluded on a
// low-structure problem, where a transform has nothing to bite into.
func TestSelectFramesMinStructureGate(t *testing.T) {
	for _, f := range selectFrames("workflow-xyz", 8, "low") {
		if f.Kind == "operation" {
			t.Errorf("operation frame %q selected on a low-structure problem", f.ID)
		}
	}
}

// TestSharedDimensionCount exercises the gate's core comparison directly.
func TestSharedDimensionCount(t *testing.T) {
	a := Frame{Dimensions: map[string]string{"time_anchor": "future", "agency": "forensic", "evidence_type": "residue"}}
	b := Frame{Dimensions: map[string]string{"time_anchor": "future", "agency": "forensic", "evidence_type": "fossil_record"}}
	if n := sharedDimensionCount(a, b); n != 2 {
		t.Fatalf("sharedDimensionCount = %d, want 2", n)
	}
	if !violatesAntiRedundancy([]Frame{a}, b) {
		t.Errorf("expected anti-redundancy violation for frames sharing 2 dimensions")
	}
	c := Frame{Dimensions: map[string]string{"axis": "magnitude"}}
	if violatesAntiRedundancy([]Frame{a}, c) {
		t.Errorf("frames sharing 0 dimensions must not violate the gate")
	}
}

func frameIDs(frames []Frame) []string {
	ids := make([]string, 0, len(frames))
	for _, f := range frames {
		ids = append(ids, f.ID)
	}
	return ids
}
