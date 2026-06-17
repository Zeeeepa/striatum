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

func TestBuildMapsWorktreeAnchorPositionals(t *testing.T) {
	got, err := Build("worktree_anchor", []string{"run_1", "job_1", "wt_1"}, Options{RepositoryID: "repo_1"})
	if err != nil {
		t.Fatal(err)
	}
	if got["repository_id"] != "repo_1" || got["run_id"] != "run_1" || got["job_id"] != "job_1" || got["worktree_id"] != "wt_1" {
		t.Fatalf("params = %#v", got)
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

// TestBuildBoolFlagDoesNotSwallowPositional is the #312 regression: `repo add
// --init <path>` must set init=true (a presence flag) AND bind the following
// positional to path. Before the fix the value-less flag greedily consumed the
// next non-`--` arg, parsing init="<path>" and losing the path positional.
func TestBuildBoolFlagDoesNotSwallowPositional(t *testing.T) {
	got, err := Build("repo_add", []string{"--init", "/tmp/target"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got["init"] != true {
		t.Fatalf("init = %#v, want true", got["init"])
	}
	if got["path"] != "/tmp/target" {
		t.Fatalf("path = %#v, want \"/tmp/target\"", got["path"])
	}
	if _, leaked := got["args"]; leaked {
		t.Fatalf("positional leaked into args: %#v", got["args"])
	}
}

// The order-independent form (`repo add <path> --init`) and the explicit
// `--path <path>` form must both still bind path and set init=true.
func TestBuildBoolFlagOrderingAndExplicitPath(t *testing.T) {
	trailing, err := Build("repo_add", []string{"/tmp/target", "--init"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if trailing["init"] != true || trailing["path"] != "/tmp/target" {
		t.Fatalf("trailing --init = %#v", trailing)
	}

	explicit, err := Build("repo_add", []string{"--init", "--path", "/tmp/target"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if explicit["init"] != true || explicit["path"] != "/tmp/target" {
		t.Fatalf("explicit --path = %#v", explicit)
	}
}

// A Bool:true flag immediately followed by an integer token consumes it as an
// optional numeric value (the #312 fix must not break this `--tail 120` numeric
// alias, which Build maps tail->tail_lines for supervise_trajectory).
func TestBuildBoolFlagConsumesIntegerAlias(t *testing.T) {
	got, err := Build("supervise_trajectory", []string{"sess_1", "--tail", "120"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := got["tail"]; present {
		t.Fatalf("tail should be converted to tail_lines, still present: %#v", got["tail"])
	}
	if got["tail_lines"] != 120 {
		t.Fatalf("tail_lines = %#v, want 120", got["tail_lines"])
	}
	if got["session_id"] != "sess_1" {
		t.Fatalf("session_id = %#v, want \"sess_1\"", got["session_id"])
	}
	if _, leaked := got["args"]; leaked {
		t.Fatalf("integer alias leaked into args: %#v", got["args"])
	}
}

// A bare Bool:true flag (no following integer) still sets true and does not
// invent a numeric value.
func TestBuildBoolFlagBareStaysTrue(t *testing.T) {
	got, err := Build("supervise_trajectory", []string{"sess_1", "--tail"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got["tail"] != true {
		t.Fatalf("bare --tail = %#v, want true", got["tail"])
	}
	if _, present := got["tail_lines"]; present {
		t.Fatalf("tail_lines unexpectedly set on bare --tail: %#v", got["tail_lines"])
	}
}
