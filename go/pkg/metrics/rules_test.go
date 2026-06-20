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

// histogramSuffixes are the derived series a histogram family emits; a rule may
// reference striatum_<name>_bucket/_sum/_count for a registered histogram family.
var histogramSuffixes = []string{"_bucket", "_sum", "_count"}

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
	for _, suffix := range histogramSuffixes {
		if strings.HasSuffix(name, suffix) && registered[strings.TrimSuffix(name, suffix)] {
			return true
		}
	}
	return false
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
