package workflowauthoring

import "testing"

// RFC 0106: workflow.lint warns (not blocks) when a run declares an experimental
// generator shape, and is silent on a supported shape or when no shape is
// declared.

func TestLintWarnsOnExperimentalShape(t *testing.T) {
	wf := validWorkflow()
	wf["shape"] = "cross_examination" // experimental: no RFC 0105 fixture
	if !lintRuleSet(t, wf)["experimental_shape"] {
		t.Fatalf("expected experimental_shape warning for an experimental-shape workflow")
	}
}

func TestLintSilentOnSupportedShape(t *testing.T) {
	wf := validWorkflow()
	wf["shape"] = "review" // supported: green RFC 0105 fixture
	if lintRuleSet(t, wf)["experimental_shape"] {
		t.Fatalf("did not expect experimental_shape warning for a supported-shape workflow")
	}
}

func TestLintSilentWhenNoShapeDeclared(t *testing.T) {
	wf := validWorkflow() // hand-authored, no `shape` field — not classified
	if lintRuleSet(t, wf)["experimental_shape"] {
		t.Fatalf("did not expect experimental_shape warning when no shape is declared")
	}
}
