package workflowtemplates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if ids["implementation_panel"] != "shape" {
		t.Fatalf("implementation_panel shape missing from catalog: %#v", ids)
	}
	if ids["falsification_gate"] != "shape" {
		t.Fatalf("falsification_gate shape missing from catalog: %#v", ids)
	}
	if ids["cross_examination"] != "shape" {
		t.Fatalf("cross_examination shape missing from catalog: %#v", ids)
	}
	if ids["adjudicated_constraint_extraction"] != "shape" {
		t.Fatalf("adjudicated_constraint_extraction shape missing from catalog: %#v", ids)
	}
	if ids["implementation_panel_roles"] != "role_pack" {
		t.Fatalf("implementation_panel role pack missing from catalog: %#v", ids)
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
	if stringValue(templates[0], "kind") != "adversary_pack" {
		t.Fatalf("first sorted entry kind = %q, want adversary_pack", stringValue(templates[0], "kind"))
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
	rolePacks, err := catalog.List("role_pack")
	if err != nil {
		t.Fatalf("List(role_pack): %v", err)
	}
	for _, entry := range rolePacks {
		if stringValue(entry, "kind") != "role_pack" {
			t.Fatalf("role_pack filter returned %#v", entry)
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

func TestCatalogCollaborationShapesDeclarePackAndExamples(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, id := range []string{"falsification_gate", "cross_examination", "adjudicated_constraint_extraction"} {
		entry, err := catalog.Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}
		if stringValue(entry, "shape_family") != "collaboration" {
			t.Fatalf("%s shape_family = %#v", id, entry["shape_family"])
		}
		if stringValue(entry, "collaboration_shape_pack") != "substance_gate_v1" {
			t.Fatalf("%s collaboration_shape_pack = %#v", id, entry["collaboration_shape_pack"])
		}
		if !strings.Contains(stringValue(entry, "example_workflow_path"), "examples/") {
			t.Fatalf("%s example_workflow_path = %#v", id, entry["example_workflow_path"])
		}
	}
}

func TestRenderMarkdownIncludesMermaidGraphPreview(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	body, err := RenderMarkdown(catalog)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	for _, needle := range []string{
		"# Workflow Template Catalog\n",
		"### Review and synthesis (`review`)",
		"```mermaid\nflowchart TD\n",
		"## Role Packs",
		"## Adversary Packs",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("rendered catalog missing %q", needle)
		}
	}
}

func TestWriteMarkdownCheckAndOverwritePolicy(t *testing.T) {
	repo := t.TempDir()
	result, err := WriteMarkdown(repo, "docs/workflow-catalog.md", MarkdownWriteOptions{})
	if err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if result["status"] != "created" {
		t.Fatalf("result = %#v", result)
	}
	result, err = WriteMarkdown(repo, "docs/workflow-catalog.md", MarkdownWriteOptions{Check: true})
	if err != nil {
		t.Fatalf("WriteMarkdown check: %v", err)
	}
	if result["status"] != "up_to_date" {
		t.Fatalf("check result = %#v", result)
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
