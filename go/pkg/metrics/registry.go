package metrics

// RFC 0137 Phase C — the metric-family registry and its Classification refusal.
//
// The registry is the SINGLE source of truth for the closed set of families the
// exporter may emit, the closed set of label NAMES each may carry, and each
// family's privacy Classification. It is what the boot-time allowlist hash and
// the CI guardrail test both hash, and it is where the Forbidden refusal
// (deliverable #1) lives: a Forbidden-classified family is rejected at
// construction so a forbidden series can never reach the wire.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Classification is the privacy/sensitivity tier every metric family carries
// (RFC 0137 §2). Operational families are aggregate runner-health signals and
// default on. Provenance families may carry a per-repo-linkable surrogate and
// are consent-gated per repo (Phase D). A Forbidden family must never reach the
// wire — Register refuses it at construction.
type Classification string

const (
	ClassificationOperational Classification = "operational"
	ClassificationProvenance  Classification = "provenance"
	ClassificationForbidden   Classification = "forbidden"
)

func isClassification(c Classification) bool {
	switch c {
	case ClassificationOperational, ClassificationProvenance, ClassificationForbidden:
		return true
	}
	return false
}

// MetricType is the Prometheus type of a family. It is carried in the manifest so
// a type change is part of the hashed, diff-reviewed contract; the hand-rendered
// exposition still writes its own # TYPE lines.
type MetricType string

const (
	TypeGauge     MetricType = "gauge"
	TypeCounter   MetricType = "counter"
	TypeHistogram MetricType = "histogram"
)

// Family is one registered metric family: its wire name, Prometheus type, the
// closed set of label NAMES it may emit (never values), and its Classification.
type Family struct {
	Name           string
	Type           MetricType
	Classification Classification
	Labels         []string
}

// Registry is the closed set of families the exporter may emit.
type Registry struct {
	families []Family
	byName   map[string]bool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byName: map[string]bool{}}
}

// Register adds a family, REFUSING a Forbidden-classified family (RFC 0137 §2
// deliverable #1): a forbidden series can never reach the wire, so it is rejected
// at construction. The daemon boot path treats this error as fatal (hard boot
// abort); MustRegister is the panic form used by the in-process default registry
// and the refusal test. Unknown classifications, empty names, and duplicate
// names are also refused.
func (r *Registry) Register(f Family) error {
	if f.Name == "" {
		return fmt.Errorf("metrics: family with empty name")
	}
	if !isClassification(f.Classification) {
		return fmt.Errorf("metrics: family %q has unknown classification %q", f.Name, f.Classification)
	}
	if f.Classification == ClassificationForbidden {
		return fmt.Errorf("metrics: refusing to register Forbidden-classified family %q; a forbidden series must never reach the wire", f.Name)
	}
	if r.byName[f.Name] {
		return fmt.Errorf("metrics: duplicate family %q", f.Name)
	}
	r.byName[f.Name] = true
	r.families = append(r.families, f)
	return nil
}

// MustRegister is Register that panics on refusal — the in-process realization of
// the hard abort ("panic in tests, hard boot abort in prod").
func (r *Registry) MustRegister(f Family) {
	if err := r.Register(f); err != nil {
		panic(err)
	}
}

// Specs returns the registered families sorted by name with each family's label
// names sorted — the canonical form the manifest hash and the committed
// metrics_allowlist.json are computed over.
func (r *Registry) Specs() []Family {
	out := make([]Family, len(r.families))
	copy(out, r.families)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	for i := range out {
		labels := append([]string{}, out[i].Labels...)
		sort.Strings(labels)
		out[i].Labels = labels
	}
	return out
}

// unitSep separates fields in the canonical hash serialization. It is a control
// byte that cannot appear in a metric name, type, classification, or label name,
// so the serialization is unambiguous.
const unitSep = '\x1f'

// Hash is the SHA-256 (hex) of the canonical (name, type, classification, sorted
// label names) serialization of every registered family. Adding or removing a
// family, a label, or changing a classification changes it; the boot check and
// the CI guardrail test both compare it against the checked-in manifest.
func (r *Registry) Hash() string {
	var b strings.Builder
	for _, f := range r.Specs() {
		b.WriteString(f.Name)
		b.WriteByte(unitSep)
		b.WriteString(string(f.Type))
		b.WriteByte(unitSep)
		b.WriteString(string(f.Classification))
		b.WriteByte(unitSep)
		b.WriteString(strings.Join(f.Labels, ","))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// DefaultRegistry is the live registry of every family the exporter emits. Every
// Phase A–C family is Operational; Provenance / consent-gated families arrive in
// Phase D. Building it also enforces the Forbidden refusal: if a future edit adds
// a Forbidden family here, MustRegister panics (in tests) and VerifyAllowlist
// aborts the daemon boot (in prod) before /metrics ever binds.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.MustRegister(Family{Name: metricSnapshotAge, Type: TypeGauge, Classification: ClassificationOperational})
	r.MustRegister(Family{Name: metricStranded, Type: TypeGauge, Classification: ClassificationOperational})
	r.MustRegister(Family{Name: metricRuns, Type: TypeGauge, Classification: ClassificationOperational, Labels: []string{"state"}})
	r.MustRegister(Family{Name: metricApoptosis, Type: TypeCounter, Classification: ClassificationOperational, Labels: []string{"origin", "reason"}})
	r.MustRegister(Family{Name: metricNecrosis, Type: TypeCounter, Classification: ClassificationOperational, Labels: []string{"origin", "reason"}})
	r.MustRegister(Family{Name: metricLivenessEvents, Type: TypeCounter, Classification: ClassificationOperational, Labels: []string{"reason"}})
	r.MustRegister(Family{Name: metricLeaseTrans, Type: TypeCounter, Classification: ClassificationOperational, Labels: []string{"from", "to", "reason"}})
	r.MustRegister(Family{Name: metricWedgeAge, Type: TypeHistogram, Classification: ClassificationOperational, Labels: []string{"origin"}})
	r.MustRegister(Family{Name: metricLivenessMargin, Type: TypeHistogram, Classification: ClassificationOperational, Labels: []string{"origin"}})
	r.MustRegister(Family{Name: metricDoctorProblems, Type: TypeGauge, Classification: ClassificationOperational, Labels: []string{"class"}})
	r.MustRegister(Family{Name: metricCardinalityClipped, Type: TypeCounter, Classification: ClassificationOperational, Labels: []string{"family"}})
	return r
}
