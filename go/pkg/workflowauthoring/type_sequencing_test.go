package workflowauthoring

import (
	"strings"
	"testing"
)

// typeSequencedWorkflow builds a two-phase v1.1 workflow whose second phase
// declares an RFC 0094 §2 work-packet type-sequencing gate: jobs of type
// "proposal" are withheld until the first phase's synthesis verdict clears.
// The proposal job is placed in p2 and declares the compiled dependency edge
// from the gate job (p1_synth), so the withholding is structurally real.
func typeSequencedWorkflow() map[string]any {
	workflow := phasedWorkflow()
	// Re-type the p2 follow-up job as a proposal and gate it behind p1_synth.
	jobs := workflow["jobs"].([]any)
	for _, item := range jobs {
		job := item.(map[string]any)
		if job["id"] == "follow" {
			job["type"] = "proposal"
		}
	}
	workflow["phases"] = []any{
		map[string]any{"id": "p1", "name": "Phase 1", "synthesis_job_id": "p1_synth"},
		map[string]any{
			"id":               "p2",
			"name":             "Phase 2",
			"synthesis_job_id": "p2_synth",
			"gate": map[string]any{
				"withhold_packet_types": []any{"proposal"},
				"until_verdict_clears":  "p1_synth",
			},
		},
	}
	return workflow
}

func TestTypeSequencingGateAccepted(t *testing.T) {
	if err := Validate(typeSequencedWorkflow()); err != nil {
		t.Fatalf("Validate type-sequenced workflow: %v", err)
	}
}

func TestTypeSequencingGateRejections(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(map[string]any)
		wantErr string
	}{
		{
			name: "gate job missing",
			mutate: func(workflow map[string]any) {
				gate := phaseGate(workflow, "p2")
				gate["until_verdict_clears"] = "nope"
			},
			wantErr: "does not name a declared job",
		},
		{
			name: "gate job not a verdict type",
			mutate: func(workflow map[string]any) {
				gate := phaseGate(workflow, "p2")
				gate["until_verdict_clears"] = "build"
			},
			wantErr: "must name a verdict-emitting job",
		},
		{
			name: "no until_verdict_clears",
			mutate: func(workflow map[string]any) {
				gate := phaseGate(workflow, "p2")
				delete(gate, "until_verdict_clears")
			},
			wantErr: "requires gate.until_verdict_clears",
		},
		{
			name: "empty withhold list",
			mutate: func(workflow map[string]any) {
				gate := phaseGate(workflow, "p2")
				gate["withhold_packet_types"] = []any{}
			},
			wantErr: "must list at least one work-packet type",
		},
		{
			name: "withheld type no job declares",
			mutate: func(workflow map[string]any) {
				gate := phaseGate(workflow, "p2")
				gate["withhold_packet_types"] = []any{"never_used"}
			},
			wantErr: "references type \"never_used\" that no job declares",
		},
		{
			name: "duplicate withheld type",
			mutate: func(workflow map[string]any) {
				gate := phaseGate(workflow, "p2")
				gate["withhold_packet_types"] = []any{"proposal", "proposal"}
			},
			wantErr: "lists \"proposal\" more than once",
		},
		{
			name: "withheld job missing dependency edge on gate",
			mutate: func(workflow map[string]any) {
				// Drop the p1_synth -> follow edge so the proposal job no longer
				// declares the compiled dependency from the gate job.
				edges := []any{}
				for _, item := range workflow["edges"].([]any) {
					edge := item.(map[string]any)
					if edge["from"] == "p1_synth" && edge["to"] == "follow" {
						continue
					}
					edges = append(edges, edge)
				}
				workflow["edges"] = edges
				// Keep the proposal job's needs in sync with the dropped edge.
				for _, item := range workflow["jobs"].([]any) {
					job := item.(map[string]any)
					if job["id"] == "follow" {
						delete(job, "needs")
					}
				}
			},
			wantErr: "does not declare a dependency edge from",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workflow := typeSequencedWorkflow()
			tc.mutate(workflow)
			err := Validate(workflow)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

// TestTypeSequencingCompilesToDependency proves the gate is not a new daemon
// authority: the withheld proposal job carries an ordinary RFC 0045 dependency
// edge from the gate job in the compiled plan, exactly like a commit phase
// depending on a review verdict. The daemon sees no new construct.
func TestTypeSequencingCompilesToDependency(t *testing.T) {
	plan, err := Plan(typeSequencedWorkflow())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	graph := plan["graph"].(map[string]any)
	edges := graph["edges"].([]map[string]any)
	found := false
	for _, edge := range edges {
		if stringValue(edge["from"]) == "p1_synth" && stringValue(edge["to"]) == "follow" {
			found = true
			gate, ok := edge["gate"].(map[string]any)
			if !ok {
				t.Fatalf("compiled edge p1_synth->follow has no gate")
			}
			if _, ok := gate["requires_verdict"]; !ok {
				t.Fatalf("compiled edge p1_synth->follow gate lacks requires_verdict: %v", gate)
			}
		}
	}
	if !found {
		t.Fatalf("compiled plan missing the gate->proposal dependency edge")
	}
}

// TestTypeSequencingLintSurfacesGate covers the advisory lint rule: a valid
// type-sequencing gate emits an info-level finding naming the withheld types and
// the gate job for operator visibility.
func TestTypeSequencingLintSurfacesGate(t *testing.T) {
	rules := lintRuleSet(t, typeSequencedWorkflow())
	if !rules["phase_gate_type_sequencing"] {
		t.Fatalf("expected phase_gate_type_sequencing lint info finding, got rules: %v", rules)
	}
}

// TestTypeSequencingLintSilentWithoutGate proves the advisory does not fire on a
// plain phased workflow with no type-sequencing gate.
func TestTypeSequencingLintSilentWithoutGate(t *testing.T) {
	rules := lintRuleSet(t, phasedWorkflow())
	if rules["phase_gate_type_sequencing"] {
		t.Fatalf("did not expect phase_gate_type_sequencing on a gateless phased workflow")
	}
}

func phaseGate(workflow map[string]any, phaseID string) map[string]any {
	for _, item := range workflow["phases"].([]any) {
		phase := item.(map[string]any)
		if phase["id"] == phaseID {
			return phase["gate"].(map[string]any)
		}
	}
	return nil
}
