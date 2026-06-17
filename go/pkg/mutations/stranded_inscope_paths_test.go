package mutations

import (
	"context"
	"reflect"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// #297 unit: strandedInScopeAuthoredPaths returns the authored in-scope paths
// that are NOT covered by the declared-or-published set — sorted and deduped —
// and returns an empty set when every authored path is covered.
func TestStrandedInScopeAuthoredPaths(t *testing.T) {
	authored := []string{
		"go/pkg/feature.go",         // declared -> covered
		"go/pkg/feature_test.go",    // stranded
		"go/migrations/0002_up.sql", // stranded
		"go/pkg/published.go",       // source-published -> covered
	}
	covered := map[string]bool{
		"go/pkg/feature.go":   true,
		"go/pkg/published.go": true,
	}
	got := strandedInScopeAuthoredPaths(authored, covered)
	want := []string{"go/migrations/0002_up.sql", "go/pkg/feature_test.go"} // sorted
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stranded = %v, want %v", got, want)
	}

	// Everything covered -> no warning.
	all := map[string]bool{
		"go/pkg/feature.go":         true,
		"go/pkg/feature_test.go":    true,
		"go/migrations/0002_up.sql": true,
		"go/pkg/published.go":       true,
	}
	if got := strandedInScopeAuthoredPaths(authored, all); len(got) != 0 {
		t.Fatalf("fully-covered set produced stranded paths %v; want none", got)
	}

	// Nothing authored -> no warning.
	if got := strandedInScopeAuthoredPaths(nil, covered); len(got) != 0 {
		t.Fatalf("empty authored set produced %v; want none", got)
	}
}

// #297 unit: publishedAndDeclaredPathSet collects declared expected_artifacts
// (attempt-resolved) plus the source-published paths into one normalized key set.
func TestPublishedAndDeclaredPathSet(t *testing.T) {
	job := map[string]any{
		"attempt": 1,
		"expected_artifacts_json": []any{
			map[string]any{"logical_name": "draft", "kind": "handoff", "path": "./docs/draft.md", "required": true},
			map[string]any{"logical_name": "review", "kind": "handoff", "path": "docs/review.md"},
		},
	}
	covered := publishedAndDeclaredPathSet(job, []string{"go/pkg/extra.go", "./go/pkg/extra2.go"})
	for _, want := range []string{"docs/draft.md", "docs/review.md", "go/pkg/extra.go", "go/pkg/extra2.go"} {
		if !covered[want] {
			t.Fatalf("covered set %v missing %q", covered, want)
		}
	}
	if covered["docs/notpresent.md"] {
		t.Fatalf("covered set should not contain undeclared/unpublished path")
	}
}

// #297 integration: a per-job-isolated repo-write job that writes an in-scope
// file it never declared (and never opted into source-publish) still COMPLETES,
// but work.complete surfaces the stranded path loudly — in the result's
// stranded_in_scope_paths + warnings, never silently dropped.
func TestCompleteWarnsOnStrandedInScopePaths(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "stranded_warn", true)

	// #326: in-scope source publish is now the DEFAULT, so undeclared in-scope
	// files are normally PUBLISHED, not stranded. The stranded-warning therefore
	// only applies to a job that explicitly OPTED OUT — set that here so the
	// #297 warning behavior is exercised in the case where it still fires.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET write_scope_json = jsonb_set(write_scope_json, '{publish_source_changes}', 'false')
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("opt-out publish_source_changes: %v", err)
	}

	// Declare docs/out.txt as a required expected artifact and publish it, so the
	// completion passes verifyRequiredArtifacts and that declared (also-authored)
	// path is NOT reported stranded.
	declared := []byte("declared headline module\n")
	expectedArg, err := db.JSONBArg(runner, []any{
		map[string]any{"logical_name": "out", "kind": "handoff", "path": "docs/out.txt", "required": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET expected_artifacts_json = $1::jsonb
		 WHERE repository_id = $2 AND job_id = $3`, expectedArg, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("declare expected artifact: %v", err)
	}
	writeWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", string(declared))
	seedPublishedArtifact(t, ctx, runner, ids, "art_headline", "out", "docs/out.txt", declared, nil)

	// The lane also wrote two in-scope files (allowed_paths=docs/) it never
	// declared and never opted to publish — these are the stranded set.
	writeWorktreeFile(t, ids.worktreeRoot, "docs/feature_test.txt", "the lane's test\n")
	writeWorktreeFile(t, ids.worktreeRoot, "docs/migration_0002.txt", "the lane's migration\n")

	result, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	if err != nil {
		t.Fatalf("complete must still succeed (warn, not refuse): %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("complete result = %#v, want completed", result)
	}

	stranded := resultStringSlice(t, result["stranded_in_scope_paths"])
	wantStranded := map[string]bool{"docs/feature_test.txt": true, "docs/migration_0002.txt": true}
	if len(stranded) != len(wantStranded) {
		t.Fatalf("stranded_in_scope_paths = %v, want exactly %v (declared docs/out.txt must be excluded)", stranded, wantStranded)
	}
	for _, p := range stranded {
		if !wantStranded[p] {
			t.Fatalf("unexpected stranded path %q (declared artifact must be excluded): %v", p, stranded)
		}
	}

	warnings := resultStringSlice(t, result["warnings"])
	if len(warnings) == 0 {
		t.Fatalf("expected a non-empty warnings list naming the stranded paths; got none")
	}
	assertContainsString(t, warnings, "docs/feature_test.txt")
	assertContainsString(t, warnings, "docs/migration_0002.txt")
	assertNotContainsString(t, warnings, "docs/out.txt")

	// The drop is also durable provenance: a job.in_scope_paths_stranded event.
	if !runHasEvent(t, ctx, runner, ids.repoID, ids.runID, "job.in_scope_paths_stranded") {
		t.Fatalf("expected a job.in_scope_paths_stranded provenance event")
	}
}

// #297 integration: when the in-scope file IS declared as an expected artifact
// (and published durably), work.complete produces no stranded warning.
func TestCompleteNoWarningWhenInScopeFileDeclared(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "stranded_declared", true)

	// Declare docs/out.txt as a required expected artifact, write it into the
	// worktree, and publish it so verifyRequiredArtifacts + durability pass.
	expectedArg, err := db.JSONBArg(runner, []any{
		map[string]any{"logical_name": "out", "kind": "handoff", "path": "docs/out.txt", "required": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET expected_artifacts_json = $1::jsonb
		 WHERE repository_id = $2 AND job_id = $3`, expectedArg, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("declare expected artifact: %v", err)
	}
	payload := []byte("declared artifact body\n")
	writeWorktreeFile(t, ids.worktreeRoot, "docs/out.txt", string(payload))
	seedPublishedArtifact(t, ctx, runner, ids, "art_declared", "out", "docs/out.txt", payload, nil)

	result, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("complete result = %#v, want completed", result)
	}
	if stranded := resultStringSlice(t, result["stranded_in_scope_paths"]); len(stranded) != 0 {
		t.Fatalf("declared-artifact path must not be reported stranded; got %v", stranded)
	}
	if warnings := resultStringSlice(t, result["warnings"]); len(warnings) != 0 {
		t.Fatalf("declared-artifact path must produce no warning; got %v", warnings)
	}
}

// #297 integration: with the opt-in source-publish flag set, the lane's in-scope
// edit is committed to the run branch, so it is NOT reported stranded.
func TestCompleteNoWarningWithSourcePublishOptIn(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "stranded_optin", true)

	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET write_scope_json = jsonb_set(write_scope_json, '{publish_source_changes}', 'true')
		 WHERE repository_id = $1 AND job_id = $2`, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("opt-in publish_source_changes: %v", err)
	}
	writeWorktreeFile(t, ids.worktreeRoot, "docs/impl.txt", "in-scope edit, published via opt-in\n")

	result, err := HandleCompleteWork(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id": ids.sessionID,
		"job_id":     ids.jobID,
		"lease_id":   ids.leaseID,
		"summary":    "done",
	}))
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("complete result = %#v, want completed", result)
	}
	if stranded := resultStringSlice(t, result["stranded_in_scope_paths"]); len(stranded) != 0 {
		t.Fatalf("source-published path must not be reported stranded; got %v", stranded)
	}
	if warnings := resultStringSlice(t, result["warnings"]); len(warnings) != 0 {
		t.Fatalf("source-published path must produce no warning; got %v", warnings)
	}
}

// resultStringSlice reads a string list from a handler result map, tolerating the
// native []string the handler stores (asStringSlice only handles []any/json).
func resultStringSlice(t *testing.T, value any) []string {
	t.Helper()
	switch typed := value.(type) {
	case nil:
		return nil
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				t.Fatalf("non-string element %#v in %#v", item, value)
			}
			out = append(out, s)
		}
		return out
	default:
		t.Fatalf("unexpected string-slice type %T (%#v)", value, value)
		return nil
	}
}

func runHasEvent(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, eventType string) bool {
	t.Helper()
	rows, err := queryRows(ctx, runner, `
		SELECT 1 FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = $3 LIMIT 1`,
		repoID, runID, eventType)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	return len(rows) > 0
}
