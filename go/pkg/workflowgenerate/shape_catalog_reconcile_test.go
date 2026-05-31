package workflowgenerate

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

// #111: the template catalog and the generator must agree on which shapes are
// generatable. The catalog advertised iterated_interrogating_panel as a shape
// but `workflow generate --shape` rejected it (it is an example fixture). This
// test fails if they drift: every catalog shape marked generatable must be a
// generator shape, and every generator shape must be advertised (generatable) in
// the catalog.
func TestCatalogAndGeneratorShapesAgree(t *testing.T) {
	catalog, err := workflowtemplates.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	catalogGeneratable := map[string]bool{} // template_id -> advertised as generatable
	for _, entry := range catalog.Shapes {
		id, _ := entry["template_id"].(string)
		if id == "" {
			continue
		}
		gen := true
		if v, ok := entry["generatable"].(bool); ok {
			gen = v
		}
		catalogGeneratable[id] = gen
	}

	// Every generator shape must be advertised as a generatable catalog shape.
	for _, shape := range SupportedShapes() {
		gen, present := catalogGeneratable[shape]
		if !present {
			t.Errorf("generator shape %q is missing from the catalog", shape)
		} else if !gen {
			t.Errorf("generator shape %q is marked generatable:false in the catalog", shape)
		}
	}

	// Every catalog shape advertised generatable must be a real generator shape.
	for id, gen := range catalogGeneratable {
		if gen && !IsSupportedShape(id) {
			t.Errorf("catalog advertises %q as generatable but the generator does not support it (#111: mark it generatable:false or add it to the generator)", id)
		}
	}
}

// #111: selecting an example-only catalog shape gives a clear, example-pointing
// error rather than the generic "must be one of" list.
func TestExampleOnlyShapeHint(t *testing.T) {
	hint := exampleOnlyShapeHint("iterated_interrogating_panel")
	for _, want := range []string{"example-only template", "examples/iterated-interrogating-panel/workflow.json", "pick a generated shape"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("hint missing %q; got: %s", want, hint)
		}
	}
	// A real generated shape is not flagged example-only.
	if h := exampleOnlyShapeHint("review"); h != "" {
		t.Fatalf("a generated shape must not produce an example-only hint, got: %s", h)
	}
	// An unknown shape produces no hint (falls through to the generic error).
	if h := exampleOnlyShapeHint("totally_made_up"); h != "" {
		t.Fatalf("unknown shape should produce no hint, got: %s", h)
	}
}
