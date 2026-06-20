package metrics

import (
	"fmt"
	"strings"
	"testing"
)

// TestSeriesBudgetClipsOverflow is the deliverable #2 proof: feeding N+1 distinct
// label-tuples collapses exactly the overflow onto one reserved `other` series
// and reports one clipped tuple; the emitted series stay bounded at N+1.
func TestSeriesBudgetClipsOverflow(t *testing.T) {
	const limit = 4
	counts := map[string]int{}
	for i := 0; i < limit+1; i++ { // N+1 distinct keys
		counts[fmt.Sprintf("class_%02d", i)] = i + 1
	}

	clipped := applySeriesBudget(counts, limit, seriesClassOther)
	if clipped != 1 {
		t.Fatalf("clipped = %d; want 1 (one distinct tuple over budget)", clipped)
	}
	if _, ok := counts[seriesClassOther]; !ok {
		t.Fatalf("overflow did not collapse onto the reserved %q bucket: %v", seriesClassOther, counts)
	}
	// The single overflow key (the lexicographically largest) is gone; its value
	// moved into `other`.
	overflowKey := fmt.Sprintf("class_%02d", limit)
	if _, ok := counts[overflowKey]; ok {
		t.Fatalf("overflow key %q was not clipped", overflowKey)
	}
	if counts[seriesClassOther] != limit+1 {
		t.Fatalf("other bucket = %d; want %d (the clipped key's value)", counts[seriesClassOther], limit+1)
	}
	if len(counts) != limit+1 { // limit kept + 1 other
		t.Fatalf("emitted series = %d; want %d (bounded at limit+1)", len(counts), limit+1)
	}
}

// TestSeriesBudgetNoClipUnderLimit asserts a within-budget family is untouched
// and reports zero clips (no spurious `other` series).
func TestSeriesBudgetNoClipUnderLimit(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 2, "c": 3}
	if clipped := applySeriesBudget(counts, 8, seriesClassOther); clipped != 0 {
		t.Fatalf("clipped = %d; want 0 under budget", clipped)
	}
	if _, ok := counts[seriesClassOther]; ok {
		t.Fatalf("an under-budget family must not synthesize an %q series", seriesClassOther)
	}
	if len(counts) != 3 {
		t.Fatalf("under-budget family was mutated: %v", counts)
	}
}

// TestDoctorProblemsBudgetClipsThroughBuild proves the budget is wired into the
// snapshot fold end to end: more than doctorProblemsSeriesBudget distinct static
// classes render a single class="other" series plus the clip counter.
func TestDoctorProblemsBudgetClipsThroughBuild(t *testing.T) {
	records := make([]map[string]any, 0, doctorProblemsSeriesBudget+5)
	for i := 0; i < doctorProblemsSeriesBudget+5; i++ {
		records = append(records, map[string]any{"check": fmt.Sprintf("synthetic_check_%03d", i)})
	}
	snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, DoctorProblemRecords: records})
	body := string(mustRender(t, snap))

	if !strings.Contains(body, metricDoctorProblems+`{class="other"}`) {
		t.Fatalf("over-budget doctor classes did not collapse to class=\"other\":\n%s", body)
	}
	if !strings.Contains(body, metricCardinalityClipped+`{family="`+clipFamilyDoctorProblems+`"} 5`) {
		t.Fatalf("cardinality clip counter did not report 5 clipped tuples:\n%s", body)
	}
	if got, want := countSeries(body, metricDoctorProblems), doctorProblemsSeriesBudget+1; got != want {
		t.Fatalf("doctor_problems series = %d; want %d (budget+other)", got, want)
	}
}
