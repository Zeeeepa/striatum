package metrics

// RFC 0137 Phase D — the version-controlled Prometheus recording + alerting rules
// shipped alongside the exporter (deliverable #4). The YAML lives under rules/ so
// an operator can point `rule_files:` straight at it; it is also embedded into the
// binary so the guardrail test validates the SAME bytes that ship, and a future
// `striatum metrics rules` verb could print them without a source tree.

import "embed"

//go:embed rules/*.yml
var rulesFS embed.FS

// RulesFS exposes the embedded Prometheus rule files (recording_rules.yml,
// alerting_rules.yml). The guardrail test walks these to assert every referenced
// series is a registry-exported family.
func RulesFS() embed.FS { return rulesFS }
