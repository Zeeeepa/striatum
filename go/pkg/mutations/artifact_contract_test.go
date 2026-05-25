package mutations

import (
	"strings"
	"testing"
)

func TestGoArtifactKindsIncludeOperatorAndGitContracts(t *testing.T) {
	for _, kind := range []string{
		"operator_brief",
		"work_plan",
		"progress_note",
		"operator_report",
		"commit_request",
		"pr_request",
		"auto_finalize_gate_evidence",
	} {
		if !allowedArtifactKinds[kind] {
			t.Fatalf("artifact kind %q is not allowed", kind)
		}
		if _, ok := frontMatterSchemas[kind]; !ok {
			t.Fatalf("artifact kind %q has no front matter schema", kind)
		}
	}
}

func TestEscalationFrontMatterRequiresPayloadFields(t *testing.T) {
	payload := []byte(`---
schema_version: "striatum.escalation.v1"
artifact_kind: "escalation"
escalation_id: "esc_1"
run_id: "run_1"
blocker_kind: "override_required"
description: "operator decision required"
reasoning: "review lane is blocked"
requested_action: "decide whether to proceed"
created_at: "2026-05-24T00:00:00Z"
---

# Escalation
`)
	err := validateArtifactFrontMatter("escalation", "ESCALATION.md", payload)
	if err == nil {
		t.Fatal("missing escalation payload fields should be refused")
	}
	if !strings.Contains(err.Error(), `missing required field "severity"`) {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestAutoFinalizeGateEvidenceFrontMatter(t *testing.T) {
	payload := []byte(`---
schema_version: "striatum.auto_finalize_gate_evidence.v1"
artifact_kind: "auto_finalize_gate_evidence"
decision_id: "D125"
gate_status: "satisfied"
live_success_count: 3
lane_shape_count: 3
lane_shapes: ["review", "build", "synthesis"]
contested_audit_chain_events: 0
evidence_artifacts: ["docs/operator/artifacts/example/GATE.md"]
created_at: "2026-05-24T00:00:00Z"
---

# Gate
`)
	if err := validateArtifactFrontMatter("auto_finalize_gate_evidence", "GATE.md", payload); err != nil {
		t.Fatalf("valid gate evidence refused: %v", err)
	}
}

func TestFrontMatterRejectsDuplicateFields(t *testing.T) {
	_, err := parseFrontMatterBlock("schema_version: \"x\"\nschema_version: \"y\"")
	if err == nil {
		t.Fatal("duplicate front matter field should be refused")
	}
	if !strings.Contains(err.Error(), "declared more than once") {
		t.Fatalf("error = %q", err.Error())
	}
}
