package mutations

import (
	"fmt"
	"testing"
)

// #356: the four claim-packet content builders — buildAdapterConstraints,
// harnessProfileView, buildReviewPolicy, and artifactAuthorIdentity — dictate
// what every lane is told (adapter_constraints / harness_profile / review_policy
// / author identity) yet had ZERO test references. They are pure functions of
// their map inputs, so these are hermetic render-assert tests.

func TestBuildAdapterConstraintsRendersEnforcement(t *testing.T) {
	lane := map[string]any{
		"adapter": "process",
		"constraints": map[string]any{
			"transcripts": "off",        // process -> "enforced"
			"network":     "forbidden",  // process -> "advisory_strict"
			"telemetry":   "off",        // process, unlisted -> "advisory"
			"ignored":     123,          // non-string value is skipped
		},
		"required_enforcement": map[string]any{
			"transcripts": "enforced",      // enforced >= enforced -> satisfied
			"network":     "enforced",      // advisory_strict < enforced -> NOT satisfied
		},
	}

	out := buildAdapterConstraints(lane)

	// The requested + required maps are echoed verbatim.
	if fmt.Sprint(out["requested"]) != fmt.Sprint(lane["constraints"]) {
		t.Fatalf("requested = %#v, want echoed constraints", out["requested"])
	}
	if fmt.Sprint(out["required_enforcement"]) != fmt.Sprint(lane["required_enforcement"]) {
		t.Fatalf("required_enforcement = %#v", out["required_enforcement"])
	}

	// Index the rendered per-constraint enforcement rows by constraint name.
	enforcement := out["enforcement"].([]any)
	byName := map[string]map[string]any{}
	for _, item := range enforcement {
		row := asMap(item)
		byName[fmt.Sprint(row["constraint"])] = row
	}
	if _, skipped := byName["ignored"]; skipped {
		t.Fatalf("non-string constraint value must be skipped, got %#v", byName["ignored"])
	}
	if len(byName) != 3 {
		t.Fatalf("rendered %d enforcement rows, want 3 (string-valued only)", len(byName))
	}

	transcripts := byName["transcripts"]
	if transcripts["requested"] != "off" || transcripts["enforcement"] != "enforced" || transcripts["satisfied"] != true {
		t.Fatalf("transcripts row = %#v, want requested=off enforcement=enforced satisfied=true", transcripts)
	}
	network := byName["network"]
	if network["enforcement"] != "advisory_strict" || network["satisfied"] != false {
		t.Fatalf("network row = %#v, want advisory_strict + NOT satisfied (required=enforced)", network)
	}
	telemetry := byName["telemetry"]
	if telemetry["enforcement"] != "advisory" || telemetry["required_enforcement"] != nil || telemetry["satisfied"] != true {
		t.Fatalf("telemetry row = %#v, want advisory, no required_enforcement, satisfied=true", telemetry)
	}

	// One unsatisfied row makes the aggregate satisfied=false.
	if out["satisfied"] != false {
		t.Fatalf("aggregate satisfied = %#v, want false (network is unsatisfied)", out["satisfied"])
	}
}

func TestBuildAdapterConstraintsUnsupportedAdapter(t *testing.T) {
	out := buildAdapterConstraints(map[string]any{
		"adapter":     "mcp",
		"constraints": map[string]any{"network": "forbidden"},
	})
	row := asMap(out["enforcement"].([]any)[0])
	if row["enforcement"] != "unsupported" {
		t.Fatalf("non-process adapter enforcement = %#v, want unsupported", row["enforcement"])
	}
	// No required_enforcement -> the row is still satisfied (nothing required).
	if row["satisfied"] != true || out["satisfied"] != true {
		t.Fatalf("with no required_enforcement the row/aggregate must be satisfied: %#v", out)
	}
}

func TestHarnessProfileView(t *testing.T) {
	workflow := map[string]any{
		"lanes": map[string]any{
			"claude": map[string]any{"harness_profile_id": "p_strict"},
			"bare":   map[string]any{}, // no profile id
		},
		"harness_profiles": map[string]any{
			"p_strict": map[string]any{"max_turns": 40, "timeout_s": 900},
		},
	}

	view := harnessProfileView(workflow, "claude")
	if view == nil {
		t.Fatal("harnessProfileView returned nil for a lane with a resolvable profile")
	}
	if view["profile_id"] != "p_strict" {
		t.Fatalf("profile_id = %#v, want p_strict", view["profile_id"])
	}
	if fmt.Sprint(view["max_turns"]) != "40" || fmt.Sprint(view["timeout_s"]) != "900" {
		t.Fatalf("profile body not merged into view: %#v", view)
	}

	// Empty lane id, a lane with no profile id, and an unknown profile body all
	// render nil (no harness_profile block in the packet).
	if harnessProfileView(workflow, "") != nil {
		t.Fatal("empty lane id must render nil")
	}
	if harnessProfileView(workflow, "bare") != nil {
		t.Fatal("lane without harness_profile_id must render nil")
	}
	if harnessProfileView(map[string]any{
		"lanes": map[string]any{"claude": map[string]any{"harness_profile_id": "missing"}},
	}, "claude") != nil {
		t.Fatal("unknown profile id must render nil")
	}
}

func TestBuildReviewPolicy(t *testing.T) {
	workflow := map[string]any{
		"jobs": []any{
			map[string]any{
				"id":                      "sec_review",
				"type":                    "review",
				"reviewer_access_scope":   "artifact_augmented",
				"reviewer_context_policy": "fresh",
				"review_posture":          "security",
			},
		},
	}

	policy := buildReviewPolicy(workflow, "sec_review")
	if policy == nil {
		t.Fatal("buildReviewPolicy returned nil for a review job with policy fields")
	}
	if policy["access_scope"] != "artifact_augmented" || policy["context_policy"] != "fresh" || policy["posture"] != "security" {
		t.Fatalf("policy fields = %#v", policy)
	}
	instruction := fmt.Sprint(policy["instruction"])
	// The instruction concatenates access + context + posture text in order.
	if !contains(instruction, "supporting artifacts") {
		t.Fatalf("instruction missing the artifact_augmented access text: %q", instruction)
	}
	if !contains(instruction, "fresh-context review") {
		t.Fatalf("instruction missing the fresh context text: %q", instruction)
	}
	if !contains(instruction, "security-focused review") {
		t.Fatalf("instruction missing the security posture text: %q", instruction)
	}
}

func TestBuildReviewPolicyDefaultsAndNilCases(t *testing.T) {
	// A review job with at least one policy field but no posture: access/context
	// default, and no "posture" key is rendered.
	workflow := map[string]any{
		"jobs": []any{
			map[string]any{"id": "r", "type": "review", "reviewer_context_policy": "cross_round"},
		},
	}
	policy := buildReviewPolicy(workflow, "r")
	if policy == nil {
		t.Fatal("a review job with a context policy must render a policy block")
	}
	if policy["access_scope"] != "document_only" || policy["context_policy"] != "cross_round" {
		t.Fatalf("defaults = %#v, want document_only/cross_round", policy)
	}
	if _, hasPosture := policy["posture"]; hasPosture {
		t.Fatalf("posture key must be absent when no review_posture is set: %#v", policy)
	}

	// A review job with NO policy fields renders nil.
	if buildReviewPolicy(map[string]any{
		"jobs": []any{map[string]any{"id": "r", "type": "review"}},
	}, "r") != nil {
		t.Fatal("review job with no policy fields must render nil")
	}
	// A non-review job (or unknown id) renders nil.
	if buildReviewPolicy(map[string]any{
		"jobs": []any{map[string]any{"id": "r", "type": "build", "reviewer_access_scope": "repo_level"}},
	}, "r") != nil {
		t.Fatal("non-review job must render nil")
	}
	if buildReviewPolicy(workflow, "absent") != nil {
		t.Fatal("unknown workflow job id must render nil")
	}
}

func TestArtifactAuthorIdentity(t *testing.T) {
	workflow := map[string]any{
		"lanes": map[string]any{"claude": map[string]any{"display_model": "Claude Opus"}},
	}

	// Attested lane with an ordinal -> privacy-safe role-model-ordinal byline.
	attested := artifactAuthorIdentity(workflow, "implementer", "claude", "impl_job", 2, true, "")
	if attested["role_id"] != "implementer" || attested["lane_id"] != "claude" || attested["display_model"] != "Claude Opus" {
		t.Fatalf("identity fields = %#v", attested)
	}
	if attested["workflow_job_id"] != "impl_job" || attested["ordinal"] != 2 {
		t.Fatalf("workflow_job_id/ordinal = %#v", attested)
	}
	if got := fmt.Sprint(attested["line"]); got != "author: implementer-claude-opus-002" {
		t.Fatalf("attested author line = %q, want author: implementer-claude-opus-002", got)
	}

	// Unattested lane with an ordinal -> operator byline (no model leak).
	unattested := artifactAuthorIdentity(workflow, "implementer", "claude", "impl_job", 1, false, "")
	if got := fmt.Sprint(unattested["line"]); got != "author: operator" {
		t.Fatalf("unattested author line = %q, want author: operator", got)
	}
	withLabel := artifactAuthorIdentity(workflow, "implementer", "claude", "impl_job", 1, false, "agy-42")
	if got := fmt.Sprint(withLabel["line"]); got != "author: operator-self-declared-agy-42" {
		t.Fatalf("operator-labelled line = %q", got)
	}

	// Ordinal 0 -> no byline at all (line is nil), and an empty lane nulls the
	// lane/display fields.
	noOrdinal := artifactAuthorIdentity(workflow, "implementer", "", "impl_job", 0, true, "")
	if noOrdinal["line"] != nil {
		t.Fatalf("ordinal 0 must produce a nil line, got %#v", noOrdinal["line"])
	}
	if noOrdinal["lane_id"] != nil || noOrdinal["display_model"] != nil {
		t.Fatalf("empty lane must null lane_id/display_model: %#v", noOrdinal)
	}

	// Attested but with an unknown model -> the byline falls back to unknown-model.
	unknownModel := artifactAuthorIdentity(map[string]any{}, "reviewer", "ghost", "rev", 1, true, "")
	if got := fmt.Sprint(unknownModel["line"]); got != "author: reviewer-unknown-model-001" {
		t.Fatalf("unknown-model byline = %q, want author: reviewer-unknown-model-001", got)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}
