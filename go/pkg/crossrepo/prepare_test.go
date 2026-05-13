package crossrepo

import "testing"

func TestRepositoriesParsesWorkflow(t *testing.T) {
	repos, primary, err := repositories(map[string]any{
		"primary_repository": "app",
		"repositories": map[string]any{
			"app": map[string]any{"repo_id": "repo_app"},
			"lib": map[string]any{"repo_id": "repo_lib"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if primary != "app" || repos["app"] != "repo_app" || repos["lib"] != "repo_lib" {
		t.Fatalf("unexpected repositories parse: primary=%s repos=%v", primary, repos)
	}
}

func TestRepositoriesRejectsMissingPrimary(t *testing.T) {
	_, _, err := repositories(map[string]any{
		"primary_repository": "missing",
		"repositories": map[string]any{
			"app": map[string]any{"repo_id": "repo_app"},
		},
	})
	if err == nil {
		t.Fatal("expected missing primary error")
	}
}
