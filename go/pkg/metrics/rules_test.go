package metrics

import (
	"regexp"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// ruleFile is the minimal Prometheus rule-file shape the guardrail parses.
type ruleFile struct {
	Groups []struct {
		Name  string `yaml:"name"`
		Rules []struct {
			Record string `yaml:"record"`
			Alert  string `yaml:"alert"`
			Expr   string `yaml:"expr"`
			For    string `yaml:"for"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

// metricRefShape matches a striatum_* metric name referenced in a PromQL expr. The
// recording-rule OUTPUT names use the `striatum:level:op` colon namespace, which
// this deliberately does NOT match (a colon is not `_`), so a recorded series is
// never mistaken for a raw exported metric.
var metricRefShape = regexp.MustCompile(`striatum_[a-z0-9_]+`)

// counterOnlyFunctions are the PromQL functions that REQUIRE a monotonic counter
// range vector. A snapshot/gauge series inside one is a semantics bug — a counter
// reset is fabricated every time the gauge falls.
var counterOnlyFunctions = []string{"rate", "irate", "increase", "resets"}

// requiredAlerts are the five alerts RFC 0137 Phase D mandates "at minimum".
var requiredAlerts = []string{
	"NecrosisRate",
	"DoctorRed",
	"WedgeAgeTail",
	"LivenessMarginCollapse",
	"SupervisorOriginFlood",
}

// loadRuleFiles parses every embedded rule file, asserting each is valid YAML in
// the Prometheus rule-file shape, and returns them.
func loadRuleFiles(t *testing.T) map[string]ruleFile {
	t.Helper()
	entries, err := rulesFS.ReadDir("rules")
	if err != nil {
		t.Fatalf("read embedded rules dir: %v", err)
	}
	out := map[string]ruleFile{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		body, err := rulesFS.ReadFile("rules/" + e.Name())
		if err != nil {
			t.Fatalf("read embedded rule %s: %v", e.Name(), err)
		}
		var rf ruleFile
		if err := yaml.Unmarshal(body, &rf); err != nil {
			t.Fatalf("rule file %s is not valid YAML: %v", e.Name(), err)
		}
		if len(rf.Groups) == 0 {
			t.Errorf("rule file %s has no groups", e.Name())
		}
		out[e.Name()] = rf
	}
	if len(out) == 0 {
		t.Fatalf("no embedded rule files found")
	}
	return out
}

// knownMetric reports whether name is an exported registry family or a histogram-
// derived series of one.
func knownMetric(name string, registered map[string]bool) bool {
	if registered[name] {
		return true
	}
	for _, suffix := range histogramSeriesSuffixes {
		if strings.HasSuffix(name, suffix) && registered[strings.TrimSuffix(name, suffix)] {
			return true
		}
	}
	return false
}

// isIdentByte reports whether b can be part of a PromQL function/metric identifier,
// so a function-name scan does not match a substring (the "rate" inside "irate").
func isIdentByte(b byte) bool {
	return b == '_' || b == ':' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// counterFunctionArguments returns the argument bodies (the text between the opening
// paren and its matching close paren) of every counter-only function call in expr.
// It is a small, dependency-free PromQL scan: the project vendors no PromQL parser,
// and this is sufficient to ground the rate-semantics guardrail over the committed
// rule set.
func counterFunctionArguments(expr string) []string {
	var bodies []string
	for _, fn := range counterOnlyFunctions {
		from := 0
		for {
			idx := strings.Index(expr[from:], fn+"(")
			if idx < 0 {
				break
			}
			start := from + idx
			// Reject a match that is the tail of a longer identifier (e.g. the "rate"
			// inside "irate"); "irate" is scanned separately as its own function.
			if start > 0 && isIdentByte(expr[start-1]) {
				from = start + len(fn) + 1
				continue
			}
			open := start + len(fn) // index of '('
			depth := 0
			end := -1
			for i := open; i < len(expr); i++ {
				if expr[i] == '(' {
					depth++
				} else if expr[i] == ')' {
					depth--
					if depth == 0 {
						end = i
						break
					}
				}
			}
			if end < 0 {
				from = open + 1
				continue
			}
			bodies = append(bodies, expr[open+1:end])
			from = end + 1
		}
	}
	return bodies
}

// TestPrometheusRulesAreValidAndWellFormed asserts the rule files parse as valid
// Prometheus rule YAML and every rule is exactly one of record/alert with a
// non-empty expr.
func TestPrometheusRulesAreValidAndWellFormed(t *testing.T) {
	files := loadRuleFiles(t)
	for name, rf := range files {
		for _, g := range rf.Groups {
			if strings.TrimSpace(g.Name) == "" {
				t.Errorf("%s: a group has an empty name", name)
			}
			for _, r := range g.Rules {
				hasRecord := strings.TrimSpace(r.Record) != ""
				hasAlert := strings.TrimSpace(r.Alert) != ""
				if hasRecord == hasAlert {
					t.Errorf("%s: a rule must be exactly one of record/alert (record=%q alert=%q)", name, r.Record, r.Alert)
				}
				if strings.TrimSpace(r.Expr) == "" {
					t.Errorf("%s: rule %q%q has an empty expr", name, r.Record, r.Alert)
				}
				if hasRecord && !strings.HasPrefix(r.Record, "striatum:") {
					t.Errorf("%s: recorded series %q must use the striatum: colon namespace", name, r.Record)
				}
			}
		}
	}
}

// TestPrometheusRulesReferenceRegisteredMetrics is the deliverable #4 guardrail: a
// rule may reference ONLY a series the striatumd registry actually exports (or a
// histogram-derived series of one). A rule pointing at a non-existent metric — a
// typo, or a metric removed from the registry — fails here.
func TestPrometheusRulesReferenceRegisteredMetrics(t *testing.T) {
	registered := map[string]bool{}
	for _, f := range DefaultRegistry().Specs() {
		registered[f.Name] = true
	}
	files := loadRuleFiles(t)
	for name, rf := range files {
		for _, g := range rf.Groups {
			for _, r := range g.Rules {
				for _, ref := range metricRefShape.FindAllString(r.Expr, -1) {
					if !knownMetric(ref, registered) {
						t.Errorf("%s: rule %q%q references unregistered metric %q", name, r.Record, r.Alert, ref)
					}
				}
			}
		}
	}
}

// TestPrometheusRulesIncludeMandatedAlerts asserts the five RFC-mandated alerts are
// all present.
func TestPrometheusRulesIncludeMandatedAlerts(t *testing.T) {
	files := loadRuleFiles(t)
	present := map[string]bool{}
	for _, rf := range files {
		for _, g := range rf.Groups {
			for _, r := range g.Rules {
				if r.Alert != "" {
					present[r.Alert] = true
				}
			}
		}
	}
	for _, want := range requiredAlerts {
		if !present[want] {
			t.Errorf("required alert %q is missing from the committed rule files", want)
		}
	}
}

// TestMetricRateSemanticsCoversRegistry pins the rate-semantics map to EXACTLY the
// registry's exported families, so a new family cannot ship without a deliberate
// monotonic/snapshot declaration — the ground truth the rate-semantics guardrail
// below validates rules against.
func TestMetricRateSemanticsCoversRegistry(t *testing.T) {
	semantics := MetricRateSemantics()
	registered := map[string]bool{}
	for _, f := range DefaultRegistry().Specs() {
		registered[f.Name] = true
		if _, ok := semantics[f.Name]; !ok {
			t.Errorf("exported family %q has no declared rate semantics; add it to metricRateSemantics", f.Name)
		}
	}
	for name := range semantics {
		if !registered[name] {
			t.Errorf("metricRateSemantics declares %q, which the registry does not export", name)
		}
	}
}

// TestPrometheusRulesRespectMetricRateSemantics is the deliverable #4 / DEFECT-2
// guardrail: every metric a committed rule wraps in a counter-only function
// (rate/irate/increase/resets) MUST be a true monotonic counter. A gauge or a
// per-tick gauge-histogram (wedge-age / liveness-margin buckets, the consent /
// tick_status / lifecycle gauges, the snapshot cardinality_clipped_total) inside a
// counter function is rejected here — it must instead use a direct comparison,
// max_over_time, or histogram_quantile. This grounds each rule's query in the
// metric's actual time-series semantics, not just its `# TYPE` line.
func TestPrometheusRulesRespectMetricRateSemantics(t *testing.T) {
	files := loadRuleFiles(t)
	for name, rf := range files {
		for _, g := range rf.Groups {
			for _, r := range g.Rules {
				for _, body := range counterFunctionArguments(r.Expr) {
					for _, ref := range metricRefShape.FindAllString(body, -1) {
						kind, ok := RateSemanticsForSeries(ref)
						if !ok {
							t.Errorf("%s: rule %q%q applies a counter-only function to %q, which is not an exported family", name, r.Record, r.Alert, ref)
							continue
						}
						if kind != SeriesMonotonic {
							t.Errorf("%s: rule %q%q applies a counter-only function (rate/irate/increase/resets) to %q, which is %s; a non-monotonic series must use a direct comparison, max_over_time, or histogram_quantile", name, r.Record, r.Alert, ref, kind)
						}
					}
				}
			}
		}
	}
}

// TestCounterFunctionArgumentsScanner proves the dependency-free PromQL scanner the
// rate-semantics guardrail relies on: it extracts the counter-only function bodies,
// distinguishes rate( from the rate inside irate(, and ignores non-counter
// functions (histogram_quantile, max_over_time).
func TestCounterFunctionArgumentsScanner(t *testing.T) {
	got := counterFunctionArguments(`sum by (le) (rate(striatum_necrosis_total[5m])) + irate(striatum_apoptosis_total[1m])`)
	if len(got) != 2 {
		t.Fatalf("expected 2 counter-fn bodies, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "striatum_necrosis_total") {
		t.Errorf("first counter-fn body did not capture the rate() argument: %q", got[0])
	}
	if !strings.Contains(got[1], "striatum_apoptosis_total") {
		t.Errorf("second counter-fn body did not capture the irate() argument: %q", got[1])
	}
	// histogram_quantile / max_over_time are not counter-only; a direct gauge read
	// yields no counter-fn body.
	if bodies := counterFunctionArguments(`histogram_quantile(0.99, sum by (le) (striatum_run_wedge_age_seconds_bucket))`); len(bodies) != 0 {
		t.Errorf("histogram_quantile direct read should yield no counter-fn body, got %v", bodies)
	}
	if bodies := counterFunctionArguments(`sum (max_over_time(striatum_metrics_cardinality_clipped_total[1h]))`); len(bodies) != 0 {
		t.Errorf("max_over_time is not a counter-only function; got %v", bodies)
	}
}
