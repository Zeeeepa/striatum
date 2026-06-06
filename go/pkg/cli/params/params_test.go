package params

import "testing"

func TestBuildMapsPositionalsAndFlags(t *testing.T) {
	got, err := Build("run_retry_job", []string{"run_1", "job_1", "--reason", "again", "--dry-run=false"}, Options{RepositoryID: "repo_1"})
	if err != nil {
		t.Fatal(err)
	}
	if got["repository_id"] != "repo_1" || got["run_id"] != "run_1" || got["job_id"] != "job_1" {
		t.Fatalf("params = %#v", got)
	}
	if got["reason"] != "again" || got["dry_run"] != false {
		t.Fatalf("flags = %#v", got)
	}
}

func TestBuildParsesRepeatedFlags(t *testing.T) {
	got, err := Build("register_session", []string{"run_1", "reviewer", "codex", "--capability", "read", "--capability", "review"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	capabilities, ok := got["capability"].([]any)
	if !ok || len(capabilities) != 2 {
		t.Fatalf("capability = %#v", got["capability"])
	}
}

// TestBuildMapsWhyPositionalTargetID is the #185 regression: `striatum why
// <target_id>` must land the lone positional in target_id (the param the why
// handler reads), not in the catch-all args slice.
func TestBuildMapsWhyPositionalTargetID(t *testing.T) {
	got, err := Build("why", []string{"run_1"}, Options{RepositoryID: "repo_1"})
	if err != nil {
		t.Fatal(err)
	}
	if got["target_id"] != "run_1" {
		t.Fatalf("target_id = %#v, want \"run_1\"", got["target_id"])
	}
	if _, leaked := got["args"]; leaked {
		t.Fatalf("positional leaked into args: %#v", got["args"])
	}
}

func TestBuildKeepsBodyJSONAsString(t *testing.T) {
	got, err := Build("send", []string{"sess_1", "note", "--body-json", `{"text":"hi"}`}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got["body_json"] != `{"text":"hi"}` {
		t.Fatalf("body_json = %#v", got["body_json"])
	}
}

func TestBuildKeepsRepoWriteContentAsString(t *testing.T) {
	got, err := Build("repo_write", []string{"sess_1", "job_1", "lease_1", "docs/out.md", "--content", "true"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got["session_id"] != "sess_1" || got["job_id"] != "job_1" || got["lease_id"] != "lease_1" || got["path"] != "docs/out.md" {
		t.Fatalf("positionals = %#v", got)
	}
	if got["content"] != "true" {
		t.Fatalf("content = %#v", got["content"])
	}
}
