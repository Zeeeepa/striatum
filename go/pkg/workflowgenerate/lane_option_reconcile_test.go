package workflowgenerate

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

// #187: the lane-set / shape catalog advertises required_options that an
// operator must supply to generate a workflow. Every advertised key must be
// reachable from `workflow generate` — either through a top-level flag, through
// the `--option` allowlist, or (for lane-spec keys like lanes.author.command)
// routed into spec.lanes by ApplyGenerateOption. This reconcile test keeps the
// catalog's required_options vocabulary honest: it fails if any advertised key
// is unreachable from the CLI (the original #187 defect, where
// lanes.author.command was rejected as an unknown generator option).
func TestEveryRequiredOptionIsCLISettable(t *testing.T) {
	catalog, err := workflowtemplates.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	checked := 0
	check := func(templateID, key string) {
		t.Helper()
		checked++
		if !cliCanSetRequiredOption(t, key) {
			t.Errorf("catalog template %q advertises required_option %q, but `workflow generate` cannot set it (#187: route it through a flag, the --option allowlist, or ApplyGenerateOption)", templateID, key)
		}
	}

	for _, entry := range catalog.LaneSets {
		id, _ := entry["template_id"].(string)
		for _, key := range stringSlice(entry["required_options"]) {
			check(id, key)
		}
	}
	for _, entry := range catalog.Shapes {
		id, _ := entry["template_id"].(string)
		for _, key := range stringSlice(entry["required_options"]) {
			check(id, key)
		}
	}

	if checked == 0 {
		t.Fatalf("catalog advertised no required_options; the reconcile walk is broken")
	}
}

// cliCanSetRequiredOption reports whether the `workflow generate` CLI surface
// can set the named required_option. It mirrors runWorkflowGenerate's routing
// exactly: lane-spec keys go through ApplyGenerateOption into spec.lanes;
// everything else is a top-level flag or an --option that must survive
// SpecFromMap's allowlist.
func cliCanSetRequiredOption(t *testing.T, key string) bool {
	t.Helper()

	// Top-level generate flags / spec fields set without --option.
	switch key {
	case "workflow_id", "artifact_root":
		return true
	case "plan", "lanes", "plan.job_lane_bindings":
		// The custom lane set / custom shape composite bindings are authored
		// through the full spec (the custom plan path), not the simple
		// --option vocabulary; they are reachable via the generator spec.
		return true
	}

	// Lane-spec keys (lanes.<id>.<field>) route into spec.lanes.
	if laneID, field, ok := parseLaneOptionKey(key); ok {
		options := map[string]any{}
		lanes := map[string]any{}
		// command/capabilities take a JSON array; other fields a scalar.
		value := `["claude"]`
		if err := ApplyGenerateOption(options, lanes, key, value); err != nil {
			return false
		}
		body, ok := lanes[laneID].(map[string]any)
		if !ok {
			return false
		}
		_, ok = body[field]
		return ok
	}

	// Otherwise it must survive the --option allowlist in SpecFromMap.
	options := map[string]any{}
	lanes := map[string]any{}
	if err := ApplyGenerateOption(options, lanes, key, "placeholder"); err != nil {
		return false
	}
	_, ok := optionKeys[key]
	return ok
}

func stringSlice(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, text)
		}
	}
	return result
}
