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

func TestBuildKeepsBodyJSONAsString(t *testing.T) {
	got, err := Build("send", []string{"sess_1", "note", "--body-json", `{"text":"hi"}`}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got["body_json"] != `{"text":"hi"}` {
		t.Fatalf("body_json = %#v", got["body_json"])
	}
}
