package workflowgenerate

import (
	"strconv"
	"strings"
	"testing"
)

// fogSynapticRaw builds a generator spec map for the fog_of_war_review /
// synaptic_prune collaboration shapes.
func fogSynapticRaw(shape, laneSet string, lanes map[string]any, options map[string]any) map[string]any {
	if options == nil {
		options = map[string]any{}
	}
	if _, ok := options["topic"]; !ok {
		options["topic"] = "shape under test"
	}
	if lanes == nil {
		lanes = map[string]any{}
	}
	return map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            shape,
		"lane_set":         laneSet,
		"workflow_id":      shape + "-test",
		"name":             shape + " test",
		"workflow_version": "2026-06-18",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/" + shape, "allow_dirty": false},
		"scaffold_root":    "docs/operator/workflows/" + shape,
		"artifact_root":    "docs/operator/artifacts/" + shape,
		"lanes":            lanes,
		"options":          options,
	}
}

func multiReviewLanes(n int) map[string]any {
	lane := func(name string) map[string]any {
		return map[string]any{"command": []any{name, "run"}, "display_model": name, "capabilities": []any{"claim", "write", "review", "synthesis", "interrogate"}}
	}
	lanes := map[string]any{"author": lane("author")}
	for i := 1; i <= n; i++ {
		lanes["reviewer_"+strconv.Itoa(i)] = lane("reviewer_" + strconv.Itoa(i))
	}
	return lanes
}

func generateFromRaw(t *testing.T, raw map[string]any) Generated {
	t.Helper()
	spec, err := SpecFromMap(raw)
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	gen, err := Generate(spec)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return gen
}

// lintWarningRules returns the set of lint rule ids present in a generated
// workflow's lint warnings (the warnings list carries info/warning findings).
func lintWarningRules(gen Generated) map[string]bool {
	rules := map[string]bool{}
	for _, item := range warningList(gen.Lint["warnings"]) {
		if rule, ok := item["rule"].(string); ok {
			rules[rule] = true
		}
	}
	return rules
}

func warningList(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func phaseByID(gen Generated, id string) map[string]any {
	phases, _ := gen.Workflow["phases"].([]map[string]any)
	for _, p := range phases {
		if p["id"] == id {
			return p
		}
	}
	return nil
}

// TestFogOfWarReviewIsV11Phased proves the shape generates a phased
// striatum.workflow.v1.1 graph with the four expected phases and validates.
func TestFogOfWarReviewIsV11Phased(t *testing.T) {
	gen := generateFromRaw(t, fogSynapticRaw("fog_of_war_review", "local", nil, nil))
	if got := gen.Workflow["schema_version"]; got != WorkflowSchemaVersionV11 {
		t.Fatalf("schema_version = %v, want %s", got, WorkflowSchemaVersionV11)
	}
	for _, id := range []string{"distribution", "reconstruction", "coverage", "proposal"} {
		if phaseByID(gen, id) == nil {
			t.Fatalf("missing phase %q", id)
		}
	}
	if got := gen.Metadata["shape_family"]; got != "collaboration" {
		t.Fatalf("shape_family = %v, want collaboration", got)
	}
}

// TestFogOfWarReviewTypeSequencingGate proves the proposal phase declares the RFC
// 0094 §2 type-sequencing gate (withhold proposal until coverage_gate clears),
// that the proposal job is typed `proposal`, that it carries the compiled
// dependency edge from the gate job, and that the advisory lint surfaces the gate.
func TestFogOfWarReviewTypeSequencingGate(t *testing.T) {
	gen := generateFromRaw(t, fogSynapticRaw("fog_of_war_review", "local", nil, nil))

	proposalPhase := phaseByID(gen, "proposal")
	gate, ok := proposalPhase["gate"].(map[string]any)
	if !ok {
		t.Fatalf("proposal phase has no gate: %#v", proposalPhase)
	}
	withheld, _ := gate["withhold_packet_types"].([]any)
	if len(withheld) != 1 || withheld[0] != "proposal" {
		t.Fatalf("withhold_packet_types = %#v, want [proposal]", gate["withhold_packet_types"])
	}
	if gate["until_verdict_clears"] != "coverage_gate" {
		t.Fatalf("until_verdict_clears = %v, want coverage_gate", gate["until_verdict_clears"])
	}

	jobs := jobsOf(t, gen)
	proposal := jobByID(jobs, "proposal")
	if proposal == nil {
		t.Fatal("no proposal job")
	}
	if proposal["type"] != "proposal" {
		t.Fatalf("proposal job type = %v, want proposal", proposal["type"])
	}
	// The withholding compiles to a real RFC 0045 dependency: the proposal job
	// declares a completed edge from the gate job (validatePhaseGateSequencing
	// rejects the gate otherwise — proven by Generate not erroring).
	if !hasEdge(gen.Workflow["edges"], "coverage_gate", "proposal") {
		t.Fatal("missing coverage_gate -> proposal dependency edge")
	}

	if !lintWarningRules(gen)["phase_gate_type_sequencing"] {
		t.Fatalf("expected phase_gate_type_sequencing lint info, rules: %v", lintWarningRules(gen))
	}
}

// TestFogOfWarReviewReconstructorsInterrogable proves the reconstructor lanes are
// interrogable (so peers can recover their fragment's constraints at runtime) and
// that the coverage gate publishes a collaboration_ledger.
func TestFogOfWarReviewReconstructorsInterrogable(t *testing.T) {
	gen := generateFromRaw(t, fogSynapticRaw("fog_of_war_review", "multi_review", multiReviewLanes(4), map[string]any{"reviewer_count": 4}))
	jobs := jobsOf(t, gen)
	interrogable := 0
	for _, j := range jobs {
		if id, _ := j["id"].(string); strings.HasPrefix(id, "reconstructor_") {
			if j["interrogable"] != true {
				t.Fatalf("reconstructor %q is not interrogable", id)
			}
			interrogable++
		}
	}
	if interrogable < 2 {
		t.Fatalf("expected >=2 interrogable reconstructors, got %d", interrogable)
	}
	coverage := jobByID(jobs, "coverage_gate")
	arts, _ := coverage["expected_artifacts"].([]map[string]any)
	if len(arts) == 0 || arts[0]["kind"] != "collaboration_ledger" {
		t.Fatalf("coverage_gate artifact kind = %#v, want collaboration_ledger", arts)
	}
}

// TestFogReconstructorCountFloor proves a single reconstructor is rejected (no
// peer to interrogate defeats the shape).
func TestFogReconstructorCountFloor(t *testing.T) {
	spec, err := SpecFromMap(fogSynapticRaw("fog_of_war_review", "local", nil, map[string]any{"reconstructor_count": 1}))
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	if _, gerr := Generate(spec); gerr == nil || !strings.Contains(gerr.Error(), "reconstructor_count must be an integer >= 2") {
		t.Fatalf("reconstructor_count=1 error = %v, want >=2 message", gerr)
	}
}

// TestSynapticPruneIsV11Phased proves the prune shape generates a phased v1.1
// graph with the three expected phases and validates.
func TestSynapticPruneIsV11Phased(t *testing.T) {
	gen := generateFromRaw(t, fogSynapticRaw("synaptic_prune", "local", nil, nil))
	if got := gen.Workflow["schema_version"]; got != WorkflowSchemaVersionV11 {
		t.Fatalf("schema_version = %v, want %s", got, WorkflowSchemaVersionV11)
	}
	for _, id := range []string{"forum", "nomination", "prune_tally"} {
		if phaseByID(gen, id) == nil {
			t.Fatalf("missing phase %q", id)
		}
	}
}

// TestSynapticPrunePostDialogHookWired proves the forum_open packet instructs the
// coordinator to declare a post_dialog_hook with packet_type prune on
// conversation.open (RFC 0094 §1 emit-before-teardown), and that the tally
// publishes the synaptic_prune collaboration_ledger as the negative preamble.
func TestSynapticPrunePostDialogHookWired(t *testing.T) {
	gen := generateFromRaw(t, fogSynapticRaw("synaptic_prune", "author_reviewer", map[string]any{
		"author":   map[string]any{"command": []any{"claude", "run"}, "display_model": "claude", "capabilities": []any{"claim", "write", "review", "synthesis", "interrogate"}},
		"reviewer": map[string]any{"command": []any{"codex", "run"}, "display_model": "codex", "capabilities": []any{"claim", "write", "review", "synthesis", "interrogate"}},
	}, nil))
	jobs := jobsOf(t, gen)

	forum := jobByID(jobs, "forum_open")
	if forum == nil {
		t.Fatal("no forum_open job")
	}
	if forum["interrogable"] != true {
		t.Fatal("forum_open must be interrogable for the post-close prune fan-out")
	}
	objective, _ := forum["objective"].(string)
	if !strings.Contains(objective, "post_dialog_hook") || !strings.Contains(objective, "packet_type: prune") {
		t.Fatalf("forum_open objective does not wire the post_dialog_hook prune packet: %q", objective)
	}

	tally := jobByID(jobs, "prune_tally")
	arts, _ := tally["expected_artifacts"].([]map[string]any)
	if len(arts) == 0 || arts[0]["kind"] != "collaboration_ledger" {
		t.Fatalf("prune_tally artifact kind = %#v, want collaboration_ledger", arts)
	}
}

// TestSynapticPruneParticipantFloor proves <2 participants is rejected (a
// >=2-vote retirement needs at least two nominators).
func TestSynapticPruneParticipantFloor(t *testing.T) {
	spec, err := SpecFromMap(fogSynapticRaw("synaptic_prune", "local", nil, map[string]any{"participant_count": 1}))
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	if _, gerr := Generate(spec); gerr == nil || !strings.Contains(gerr.Error(), "participant_count must be an integer >= 2") {
		t.Fatalf("participant_count=1 error = %v, want >=2 message", gerr)
	}
}

// TestFogSynapticAreCollaborationShapes proves both new shapes register as
// collaboration shapes (so they get the substance_gate_v1 pack metadata and the
// v1.1 schema, and the validateModifierMatrix single_agent refusal).
func TestFogSynapticAreCollaborationShapes(t *testing.T) {
	for _, shape := range []string{"fog_of_war_review", "synaptic_prune"} {
		if !isCollaborationShape(shape) {
			t.Fatalf("%s is not a collaboration shape", shape)
		}
		if !IsSupportedShape(shape) {
			t.Fatalf("%s is not a generatable generator shape", shape)
		}
	}
}
