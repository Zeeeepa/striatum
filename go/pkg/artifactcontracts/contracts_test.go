package artifactcontracts

import (
	"os"
	"strings"
	"testing"
)

func TestAllowedKindsAndSchemasCoverOperatorGitAndGateArtifacts(t *testing.T) {
	for _, kind := range []string{
		"operator_brief",
		"work_plan",
		"progress_note",
		"operator_report",
		"commit_request",
		"pr_request",
		"auto_finalize_gate_evidence",
	} {
		if !IsAllowedKind(kind) {
			t.Fatalf("kind %q is not allowed", kind)
		}
		if !HasFrontMatterSchema(kind) {
			t.Fatalf("kind %q has no front matter schema", kind)
		}
	}
}

func TestWorkPlanAcceptsPlannedAndNullableFields(t *testing.T) {
	payload := []byte(`---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_1"
scope_kind: "initiative"
scope_ref: "docs/operator/BRIEF.md"
state: "planned"
opened_at: "2026-05-25"
closed_at: null
closure_summary: null
supersedes: null
retrieval_priority: "high"
---

# Plan
`)
	if err := ValidateFrontMatter("work_plan", "PLAN.md", payload); err != nil {
		t.Fatalf("valid work_plan refused: %v", err)
	}
}

func TestPRRequestRequiresCommitRequestOrLocalCommit(t *testing.T) {
	payload := []byte(`---
schema_version: "striatum.pr_request.v1"
artifact_kind: "pr_request"
request_id: "pr_1"
target_branch: "main"
summary: "Update docs"
body_draft: "Body"
confirmation_status: "pending"
---

# PR
`)
	err := ValidateFrontMatter("pr_request", "PR.md", payload)
	if err == nil || !strings.Contains(err.Error(), "requires at least one") {
		t.Fatalf("error = %v", err)
	}
}

func TestAutoFinalizeSatisfiedGateRequiresEvidenceThreshold(t *testing.T) {
	payload := []byte(`---
schema_version: "striatum.auto_finalize_gate_evidence.v1"
artifact_kind: "auto_finalize_gate_evidence"
decision_id: "D125"
gate_status: "satisfied"
live_success_count: 2
lane_shape_count: 2
lane_shapes: ["review", "build"]
contested_audit_chain_events: 0
evidence_artifacts: ["docs/operator/artifacts/example/GATE.md"]
created_at: "2026-05-24T00:00:00Z"
---

# Gate
`)
	err := ValidateFrontMatter("auto_finalize_gate_evidence", "GATE.md", payload)
	if err == nil || !strings.Contains(err.Error(), "live_success_count") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseFrontMatterRejectsDuplicateFields(t *testing.T) {
	_, err := ParseFrontMatterBlock("schema_version: \"x\"\nschema_version: \"y\"")
	if err == nil || !strings.Contains(err.Error(), "declared more than once") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseFrontMatterAllowsMultilineLists(t *testing.T) {
	block := `schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs:
  - "file_a.txt"
  - "file_b.txt"`
	parsed, err := ParseFrontMatterBlock(block)
	if err != nil {
		t.Fatalf("unexpected error parsing multiline lists: %v", err)
	}
	inputs, ok := parsed["inputs"].([]string)
	if !ok || len(inputs) != 2 || inputs[0] != "file_a.txt" || inputs[1] != "file_b.txt" {
		t.Fatalf("inputs parsed incorrectly: %v", parsed["inputs"])
	}
}

func TestParseFrontMatterReturnsLineNumberedSyntaxErrors(t *testing.T) {
	// Syntax error on line 3: missing key or invalid formatting
	block := `schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
invalid yaml logic here`
	_, err := ParseFrontMatterBlock(block)
	if err == nil {
		t.Fatalf("expected error on syntax error, got nil")
	}
	if !strings.Contains(err.Error(), "line 4:") {
		t.Fatalf("expected error to mention line 4, got %v", err)
	}
}

func TestCollaborationLedgerValidClearingLedgerExercisesEntryKinds(t *testing.T) {
	if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validCollaborationLedger("accept"))); err != nil {
		t.Fatalf("valid collaboration ledger refused: %v", err)
	}
}

func TestCollaborationLedgerRequiresFrontMatter(t *testing.T) {
	err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte("# Ledger\n"))
	if err == nil || !strings.Contains(err.Error(), "front matter is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollaborationLedgerRejectsUnknownTopLevelField(t *testing.T) {
	payload := strings.Replace(validCollaborationLedger("needs_revision"), "rationale: \"A challenge landed but has not been rebutted.\"", "rationale: \"A challenge landed but has not been rebutted.\"\nstdout: \"raw provider stream\"", 1)
	err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
	if err == nil || !strings.Contains(err.Error(), "unknown fields") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollaborationLedgerRejectsInvalidEntryKind(t *testing.T) {
	payload := strings.Replace(validCollaborationLedger("needs_revision"), "kind: challenge", "kind: gossip", 1)
	err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
	if err == nil || !strings.Contains(err.Error(), `field "entries" is invalid`) {
		t.Fatalf("error = %v", err)
	}
}

func TestCollaborationLedgerRejectsClearingVerdictWithoutChallengeAndRebuttal(t *testing.T) {
	payload := `---
schema_version: "striatum.collaboration_ledger.v1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "substance gate"
participants: ["sess_holder", "sess_falsifier"]
entries:
  - kind: claim
    by: sess_holder
    refs: ["dialogue:1"]
    text: "The proposal is ready."
verdict: "accept"
rationale: "Hollow dialogue should not clear."
---

# Ledger
`
	err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
	if err == nil || !strings.Contains(err.Error(), "clearing verdict requires") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollaborationLedgerRejectsEntryByOutsideParticipants(t *testing.T) {
	payload := strings.Replace(validCollaborationLedger("needs_revision"), "by: sess_falsifier", "by: sess_intruder", 1)
	err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
	if err == nil || !strings.Contains(err.Error(), "must name a participant") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollaborationLedgerRejectsInvalidVerdict(t *testing.T) {
	payload := strings.Replace(validCollaborationLedger("accept"), `verdict: "accept"`, `verdict: "maybe"`, 1)
	err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
	if err == nil || !strings.Contains(err.Error(), `field "verdict" is invalid`) {
		t.Fatalf("error = %v", err)
	}
}

func TestCollaborationLedgerRejectsMalformedRefs(t *testing.T) {
	payload := strings.Replace(validCollaborationLedger("needs_revision"), `refs: ["dialogue:2"]`, `refs: ["turn-2"]`, 1)
	err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
	if err == nil || !strings.Contains(err.Error(), `field "entries" is invalid`) {
		t.Fatalf("error = %v", err)
	}
}

func TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge(t *testing.T) {
	if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validCollaborationLedger("needs_revision"))); err != nil {
		t.Fatalf("needs_revision collaboration ledger refused: %v", err)
	}
}

func TestCollaborationLedgerV11TruthTable(t *testing.T) {
	t.Run("v1 ledger still validates without v11 fields", func(t *testing.T) {
		if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validCollaborationLedger("accept"))); err != nil {
			t.Fatalf("v1 collaboration ledger refused: %v", err)
		}
	})
	t.Run("non ACE v11 needs revision does not trigger productive refusal", func(t *testing.T) {
		payload := strings.Replace(validCollaborationLedger("needs_revision"), "striatum.collaboration_ledger.v1", "striatum.collaboration_ledger.v1.1", 1)
		if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload)); err != nil {
			t.Fatalf("non-ACE v1.1 needs_revision refused: %v", err)
		}
	})
	t.Run("ACE requires v11", func(t *testing.T) {
		payload := strings.Replace(validACECollaborationLedger("needs_revision", aceUnresolvedConstraint(), aceFinding(), ""), "striatum.collaboration_ledger.v1.1", "striatum.collaboration_ledger.v1", 1)
		err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
		if err == nil || !strings.Contains(err.Error(), "requires schema_version striatum.collaboration_ledger.v1.1") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("ACE needs revision requires productive constraints", func(t *testing.T) {
		err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", "constraints: []\n", "", "")))
		if err == nil || !strings.Contains(err.Error(), "needs_revision requires a non-empty constraints[]") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("ACE needs revision accepts grounded binding constraint", func(t *testing.T) {
		if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", aceBindingConstraint(), aceFinding(), aceBranches()))); err != nil {
			t.Fatalf("grounded binding constraint refused: %v", err)
		}
	})
	t.Run("ACE needs revision accepts unresolved question", func(t *testing.T) {
		if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", aceUnresolvedConstraint(), "", ""))); err != nil {
			t.Fatalf("unresolved question constraint refused: %v", err)
		}
	})
}

func TestCollaborationLedgerV11BindingConstraintHardening(t *testing.T) {
	t.Run("binding source finding must resolve to high or critical finding", func(t *testing.T) {
		lowFinding := strings.Replace(aceFinding(), "severity: high", "severity: medium", 1)
		err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", aceBindingConstraint(), lowFinding, "")))
		if err == nil || !strings.Contains(err.Error(), "must resolve to a high or critical findings[] row") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("binding constraints require verification", func(t *testing.T) {
		constraint := strings.Replace(aceBindingConstraint(), `    verification:
      gate: "go -C go test ./pkg/artifactcontracts/..."
`, "", 1)
		err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", constraint, aceFinding(), "")))
		if err == nil || !strings.Contains(err.Error(), "verification requires a non-empty gate or expected_stage") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("constraint rows reject raw unknown keys", func(t *testing.T) {
		constraint := strings.Replace(aceUnresolvedConstraint(), `    text: "The adjudicator needs a maintainer answer before it can clear the gate."`, `    text: "The adjudicator needs a maintainer answer before it can clear the gate."
    stdout: "raw provider stream"`, 1)
		err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", constraint, "", "")))
		if err == nil || !strings.Contains(err.Error(), "constraints[0] has unknown fields: stdout") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestCollaborationLedgerV11FindingsAndBranchVocabulary(t *testing.T) {
	t.Run("findings reject raw unknown keys", func(t *testing.T) {
		finding := strings.Replace(aceFinding(), `    source_refs: ["dialogue:2"]`, `    source_refs: ["dialogue:2"]
    stdout: "raw provider stream"`, 1)
		err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", aceBindingConstraint(), finding, "")))
		if err == nil || !strings.Contains(err.Error(), "findings[0] has unknown fields: stdout") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("natural multiline findings and advertised branch dispositions validate", func(t *testing.T) {
		if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(validACECollaborationLedger("needs_revision", aceBindingConstraint(), aceFinding(), aceBranches()))); err != nil {
			t.Fatalf("natural v1.1 ledger refused: %v", err)
		}
	})
	t.Run("refined states are branch dispositions, not verdicts", func(t *testing.T) {
		payload := strings.Replace(validACECollaborationLedger("needs_revision", aceUnresolvedConstraint(), "", ""), `verdict: "needs_revision"`, `verdict: "blocked_pending_answer"`, 1)
		err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload))
		if err == nil || !strings.Contains(err.Error(), "allowed verdicts are accept, accept_with_findings, needs_revision, reject") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestCollaborationLedgerReferenceExamplesValidate(t *testing.T) {
	specBytes, err := os.ReadFile("../../../docs/reference/spec.md")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(specBytes)
	for _, label := range []string{
		"Example productive `needs_revision` ledger:",
		"Example clearing ledger:",
	} {
		payload := yamlFenceAfter(t, spec, label)
		if err := ValidateFrontMatter("collaboration_ledger", "LEDGER.md", []byte(payload)); err != nil {
			t.Fatalf("%s example refused: %v", label, err)
		}
	}
}

func yamlFenceAfter(t *testing.T, text string, label string) string {
	t.Helper()
	start := strings.Index(text, label)
	if start == -1 {
		t.Fatalf("missing label %q", label)
	}
	rest := text[start:]
	fence := strings.Index(rest, "```yaml\n")
	if fence == -1 {
		t.Fatalf("missing yaml fence after %q", label)
	}
	rest = rest[fence+len("```yaml\n"):]
	end := strings.Index(rest, "\n```")
	if end == -1 {
		t.Fatalf("unterminated yaml fence after %q", label)
	}
	return rest[:end]
}

func validCollaborationLedger(verdict string) string {
	if verdict == "needs_revision" {
		return `---
schema_version: "striatum.collaboration_ledger.v1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "substance gate"
participants: ["sess_holder", "sess_falsifier"]
entries:
  - kind: claim
    by: sess_holder
    refs: ["dialogue:1"]
    text: "The proposal is ready."
  - kind: challenge
    by: sess_falsifier
    refs: ["dialogue:2"]
    text: "The proposal lacks migration evidence."
verdict: "needs_revision"
rationale: "A challenge landed but has not been rebutted."
---

# Ledger
`
	}
	return `---
schema_version: "striatum.collaboration_ledger.v1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "substance gate"
participants: ["sess_holder", "sess_falsifier", "sess_adjudicator"]
entries:
  - kind: claim
    by: sess_holder
    refs: ["dialogue:1"]
    text: "The proposal is ready."
  - kind: challenge
    by: sess_falsifier
    refs: ["dialogue:2"]
    text: "The proposal lacks migration evidence."
  - kind: rebuttal
    by: sess_holder
    refs: ["dialogue:3"]
    text: "The migration evidence is in the linked fixture."
  - kind: constraint
    by: sess_adjudicator
    refs: ["dialogue:4"]
    text: "The migration evidence must be cited in the handoff."
  - kind: nomination
    by: sess_falsifier
    refs: ["dialogue:5"]
    text: "Retire the unsupported no-migration claim."
verdict: "` + verdict + `"
rationale: "A material challenge landed and was rebutted on the record."
---

# Ledger
`
}

func validACECollaborationLedger(verdict string, constraints string, findings string, branches string) string {
	return `---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "adjudicated_constraint_extraction"
topic: "productive refusal gate"
participants: ["sess_holder", "sess_falsifier", "sess_adjudicator"]
entries:
  - kind: claim
    by: sess_holder
    refs: ["dialogue:1"]
    text: "The proposal is ready."
  - kind: challenge
    by: sess_falsifier
    refs: ["dialogue:2"]
    text: "The proposal lacks an enforceable constraint."
cycle: 1
verdict: "` + verdict + `"
rationale: "The adjudicator is carrying an actionable constraint forward."
` + findings + constraints + branches + `---

# Ledger
`
}

func aceFinding() string {
	return `findings:
  - id: F1
    severity: high
    posture: implementation
    status: converted_to_constraint
    challenge: |-
      The implementation did not name the concrete validation gate.
      A future revision could satisfy the prose while skipping the check.
    closest_acceptable_answer: "Add a named test gate."
    affected_invariants:
      - productive_refusal
      - artifact_contract
    requested_constraint_shape:
      kind: gate
    requires_convener_rebuttal: true
    source_refs: ["dialogue:2"]
`
}

func aceBindingConstraint() string {
	return `constraints:
  - id: C1
    source_finding: F1
    posture: implementation
    severity: high
    kind: gate
    binding: true
    text: "The next revision must add a validation gate for naked needs_revision ledgers."
    source_refs: ["dialogue:2"]
    verification:
      gate: "go -C go test ./pkg/artifactcontracts/..."
    final_review_required: true
`
}

func aceUnresolvedConstraint() string {
	return `constraints:
  - id: Q1
    posture: product
    severity: medium
    kind: unresolved_question
    binding: false
    text: "The adjudicator needs a maintainer answer before it can clear the gate."
`
}

func aceBranches() string {
	return `branches:
  implementation: blocked_pending_answer
  product: defer_with_successor
`
}
