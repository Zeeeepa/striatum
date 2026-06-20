package metrics

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// mustRender renders a snapshot to text at sentinelNow or fails the test.
func mustRender(t *testing.T, s *Snapshot) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := s.WriteText(&buf, sentinelNow); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	return buf.Bytes()
}

// countSeries counts rendered data lines for a metric family (lines beginning
// `name{`), ignoring HELP/TYPE comments.
func countSeries(body, name string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, name+"{") {
			n++
		}
	}
	return n
}

// leakSentinel is a distinctive token embedded in every adversarial id and
// summary string. It cannot collide with a metric name or help word, so its
// absence from the rendered body is a precise "no dynamic id leaked" assertion.
const leakSentinel = "Z9LEAKSENTINEL"

// adversarialDoctorResult builds a doctor result shaped like HandleDoctor(verbose)
// output: `problem_records` carrying STATIC check codes plus id-bearing fields,
// AND a `problems` summary list whose strings interpolate dynamic ids in their
// prefix (the A3/A5 leak surface). Every id carries leakSentinel. perCheck
// failing entities are seeded per check code so the count scales with `perCheck`
// while the class set stays fixed.
func adversarialDoctorResult(perCheck int) map[string]any {
	checks := []string{
		"recovery_sweep_cursor_wedged",
		"dissent_ledger_incomplete",
		"artifact_anchor_hash_mismatch",
		"strict_fanin_required_seat_unrecoverable",
	}
	records := []map[string]any{}
	problems := []string{}
	for _, check := range checks {
		for i := 0; i < perCheck; i++ {
			runID := fmt.Sprintf("run_%s_%s_%04d_deadbeef", check, leakSentinel, i)
			records = append(records, map[string]any{
				"check":         check,
				"run_id":        runID,
				"supervisor_id": fmt.Sprintf("sup_%s_%04d", leakSentinel, i),
				"gate_id":       fmt.Sprintf("gate_%s_%04d/leak", leakSentinel, i),
			})
			// The dynamic-id summary string the fold must NEVER read.
			problems = append(problems, check+"."+runID+": adversarial failure")
		}
	}
	return map[string]any{
		"ok":              false,
		"problems":        problems,
		"problem_records": records,
	}
}

// TestDoctorClassRejectsDynamicIdentifiers is the F-A8 acceptance test. It seeds
// quorum/recovery/supervisor failures with adversarial run/gate/supervisor ids,
// renders /metrics, and asserts: (a) only static problem_records[*].check codes
// appear as `class`, (b) no dynamic id ever leaks into the body, and (c) the
// doctor_problems series count stays constant regardless of how many entities are
// failing.
func TestDoctorClassRejectsDynamicIdentifiers(t *testing.T) {
	want := []string{
		"artifact_anchor_hash_mismatch",
		"dissent_ledger_incomplete",
		"recovery_sweep_cursor_wedged",
		"strict_fanin_required_seat_unrecoverable",
	}

	render := func(perCheck int) string {
		result := adversarialDoctorResult(perCheck)
		records := extractDoctorProblemRecords(result)
		snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, DoctorProblemRecords: records})
		return string(mustRender(t, snap))
	}

	body := render(1)

	// (a) Exactly the static check codes appear as class, each with the seeded count.
	for _, class := range want {
		line := fmt.Sprintf(`%s{class=%q} 1`, metricDoctorProblems, class)
		if !strings.Contains(body, line) {
			t.Errorf("missing static class series %q in body:\n%s", line, body)
		}
	}
	if got := countSeries(body, metricDoctorProblems); got != len(want) {
		t.Fatalf("doctor_problems series = %d; want %d (one per static check)", got, len(want))
	}

	// (b) No dynamic id (run/supervisor/gate), no 40-hex run, and no `problems`
	// summary string leaks. Every adversarial id carries leakSentinel, which
	// cannot collide with a metric name or help word.
	for _, leak := range []string{leakSentinel, "deadbeef", ": adversarial failure"} {
		if strings.Contains(body, leak) {
			t.Errorf("dynamic id fragment %q leaked into the doctor_problems body:\n%s", leak, body)
		}
	}

	// (c) Series count is invariant to the number of failing entities; only the
	// per-class VALUE scales.
	bodyMany := render(25)
	if got := countSeries(bodyMany, metricDoctorProblems); got != len(want) {
		t.Fatalf("doctor_problems series grew with failing-entity count: got %d, want %d", got, len(want))
	}
	for _, class := range want {
		line := fmt.Sprintf(`%s{class=%q} 25`, metricDoctorProblems, class)
		if !strings.Contains(bodyMany, line) {
			t.Errorf("class %q value did not scale to 25:\n%s", class, bodyMany)
		}
	}
}

// TestExtractDoctorProblemRecordsIgnoresProblemsList proves the extractor reads
// ONLY problem_records: a result with a populated dynamic-id `problems` list but
// no `problem_records` yields no records (and so no series).
func TestExtractDoctorProblemRecordsIgnoresProblemsList(t *testing.T) {
	result := map[string]any{
		"problems": []string{"run_needs_operator.run_abc123: recovery exhausted"},
	}
	if recs := extractDoctorProblemRecords(result); len(recs) != 0 {
		t.Fatalf("extractor read from the dynamic-id problems list: %v", recs)
	}

	// It also tolerates the decoded []any-of-maps shape.
	decoded := map[string]any{
		"problem_records": []any{
			map[string]any{"check": "recovery_sweep_cursor_wedged"},
		},
	}
	if recs := extractDoctorProblemRecords(decoded); len(recs) != 1 {
		t.Fatalf("extractor did not handle []any records: %v", recs)
	}
}

// TestFoldDoctorSanitizesNonStaticCheck proves the second wall behind the
// never-read-problems rule: a `check` value that is not a static snake_case code
// (e.g. one that accidentally interpolated an id) is bucketed to `other` rather
// than reaching the wire.
func TestFoldDoctorSanitizesNonStaticCheck(t *testing.T) {
	counts := foldDoctorProblemRecords([]map[string]any{
		{"check": "recovery_sweep_cursor_wedged"},
		{"check": "recovery_sweep_cursor_latch_error.run_abc123"}, // id-bearing → other
		{"check": "Path/With/Slashes"},                            // shape violation → other
		{"check": ""},                                             // skipped entirely
		{"other_field": "no check key"},                           // skipped entirely
	})
	if counts["recovery_sweep_cursor_wedged"] != 1 {
		t.Errorf("static code lost: %v", counts)
	}
	if counts[seriesClassOther] != 2 {
		t.Errorf("non-static check codes were not bucketed to %q: %v", seriesClassOther, counts)
	}
	if _, ok := counts["recovery_sweep_cursor_latch_error.run_abc123"]; ok {
		t.Errorf("an id-bearing check code reached the class map: %v", counts)
	}
}
