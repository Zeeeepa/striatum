package workflowtemplates

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCurrentCatalogListsAndInjectsMultiPhase(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	templates, err := catalog.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	ids := map[string]string{}
	for _, entry := range templates {
		ids[stringValue(entry, "template_id")] = stringValue(entry, "kind")
	}
	if ids["code_change"] != "shape" {
		t.Fatalf("code_change entry missing from catalog: %#v", ids)
	}
	if ids["multi_phase"] != "shape" {
		t.Fatalf("multi_phase fallback entry missing from catalog: %#v", ids)
	}
	if ids["author_reviewer"] != "lane_set" {
		t.Fatalf("author_reviewer lane set missing from catalog: %#v", ids)
	}
}

func TestListFiltersAndSortsLikePythonCatalog(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	templates, err := catalog.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(templates) < 2 {
		t.Fatalf("expected multiple templates")
	}
	if stringValue(templates[0], "kind") != "lane_set" {
		t.Fatalf("first sorted entry kind = %q, want lane_set", stringValue(templates[0], "kind"))
	}

	shapes, err := catalog.List("shape")
	if err != nil {
		t.Fatalf("List(shape): %v", err)
	}
	for _, entry := range shapes {
		if stringValue(entry, "kind") != "shape" {
			t.Fatalf("shape filter returned %#v", entry)
		}
	}
}

func TestGetReturnsTemplateEntry(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	entry, err := catalog.Get("author_reviewer")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stringValue(entry, "kind") != "lane_set" {
		t.Fatalf("kind = %v, want lane_set", entry["kind"])
	}
}

func TestEmbeddedCatalogMatchesPythonSource(t *testing.T) {
	source := filepath.Join("..", "..", "..", "src", "striatum", "workflow_templates", "catalog.json")
	sourceBody, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read source catalog: %v", err)
	}
	embeddedBody, err := catalogFS.ReadFile("catalog.json")
	if err != nil {
		t.Fatalf("read embedded catalog: %v", err)
	}
	if !bytes.Equal(embeddedBody, sourceBody) {
		t.Fatalf("embedded Go workflow template catalog differs from Python source catalog")
	}
}

func TestLoadFileValidatesHarnessProfileFragments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	body := map[string]any{
		"schema_version": CatalogSchemaVersion,
		"shapes": []any{
			map[string]any{"template_id": "minimal", "kind": "shape"},
		},
		"lane_sets": []any{
			map[string]any{"template_id": "local", "kind": "lane_set"},
		},
		"harness_profile_fragments": []any{
			map[string]any{
				"profile_id":                    "bad",
				"kind":                          "harness_profile_fragment",
				"tool_family":                   "unknown",
				"native_delegation_instruction": "text",
			},
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = LoadFile(path)
	if err == nil {
		t.Fatalf("expected invalid tool_family to be rejected")
	}
}
