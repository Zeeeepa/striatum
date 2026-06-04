package reads

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestRepoConcurrentRunsSurfacesFanOutAndCollisions is the RFC 0108 Phase 5 gate:
// the repo-scoped concurrent-runs view lists every `running` run with its branch,
// repo-write paths, and lane sessions, and flags the live collisions the Phase
// 2/3 run.start gates reason about — a shared branch (definite) and an
// overlapping repo-write scope (potential merge conflict).
func TestRepoConcurrentRunsSurfacesFanOutAndCollisions(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_cr"
	now := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)
	crSeedRepo(t, ctx, runner, repoID, now)

	// A and B share a branch (definite collision); C overlaps A's write scope
	// (docs ⊃ docs/rfcs); D is document-only on its own branch (no collision).
	crSeedRun(t, ctx, runner, repoID, "run_a", "striatum/shared", now, true, []string{"docs"})
	crSeedRun(t, ctx, runner, repoID, "run_b", "striatum/shared", now.Add(time.Minute), true, []string{"src"})
	crSeedRun(t, ctx, runner, repoID, "run_c", "striatum/feat-c", now.Add(2*time.Minute), true, []string{"docs/rfcs"})
	crSeedRun(t, ctx, runner, repoID, "run_d", "striatum/feat-d", now.Add(3*time.Minute), false, nil)

	view, err := repoConcurrentRuns(ctx, runner, repoID)
	if err != nil {
		t.Fatalf("repoConcurrentRuns: %v", err)
	}
	if len(view) != 4 {
		t.Fatalf("concurrent-runs view has %d runs, want 4", len(view))
	}
	byRun := map[string]map[string]any{}
	for _, r := range view {
		byRun[stringFrom(r, "run_id")] = r
	}

	// Each run carries its branch, repo-write paths, and at least one lane session.
	a := byRun["run_a"]
	if stringFrom(a, "branch") != "striatum/shared" {
		t.Fatalf("run_a branch = %q", stringFrom(a, "branch"))
	}
	if paths := a["write_scope_paths"].([]string); len(paths) != 1 || paths[0] != "docs" {
		t.Fatalf("run_a write_scope_paths = %#v, want [docs]", a["write_scope_paths"])
	}
	if lanes := a["lanes"].([]map[string]any); len(lanes) != 1 || stringFrom(lanes[0], "role_id") != "author" {
		t.Fatalf("run_a lanes = %#v", a["lanes"])
	}

	// run_a collides with run_b (branch) AND run_c (write_scope).
	assertCollides(t, a, map[string]string{"run_b": "branch", "run_c": "write_scope"})
	// run_b collides only with run_a (branch); src is disjoint from docs/rfcs.
	assertCollides(t, byRun["run_b"], map[string]string{"run_a": "branch"})
	// run_c collides only with run_a (write_scope); docs/rfcs is disjoint from src.
	assertCollides(t, byRun["run_c"], map[string]string{"run_a": "write_scope"})

	// run_d is document-only on its own branch: no repo-write paths, no collision.
	d := byRun["run_d"]
	if paths := d["write_scope_paths"].([]string); len(paths) != 0 {
		t.Fatalf("run_d (document-only) write_scope_paths = %#v, want empty", paths)
	}
	if d["collides"] != false {
		t.Fatalf("run_d collides = %v, want false", d["collides"])
	}
	if cols := d["collisions"].([]map[string]any); len(cols) != 0 {
		t.Fatalf("run_d collisions = %#v, want none", cols)
	}
}

// TestRepoConcurrentRunsExcludesTerminalRuns proves the view is the LIVE fan-out:
// only `running` runs appear, not completed/failed ones.
func TestRepoConcurrentRunsExcludesTerminalRuns(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_cr_terminal"
	now := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)
	crSeedRepo(t, ctx, runner, repoID, now)
	crSeedRun(t, ctx, runner, repoID, "run_live", "striatum/live", now, true, []string{"docs"})
	crSeedRunState(t, ctx, runner, repoID, "run_done", "striatum/done", now, "completed")

	view, err := repoConcurrentRuns(ctx, runner, repoID)
	if err != nil {
		t.Fatalf("repoConcurrentRuns: %v", err)
	}
	if len(view) != 1 || stringFrom(view[0], "run_id") != "run_live" {
		t.Fatalf("view = %#v, want only run_live", view)
	}
	if view[0]["collides"] != false {
		t.Fatalf("the only live run should not collide with a terminal run: %#v", view[0])
	}
}

func assertCollides(t *testing.T, run map[string]any, want map[string]string) {
	t.Helper()
	cols, _ := run["collisions"].([]map[string]any)
	got := map[string]string{}
	for _, c := range cols {
		got[stringFrom(c, "run_id")] = stringFrom(c, "kind")
	}
	if len(got) != len(want) {
		t.Fatalf("run %s collisions = %#v, want %#v", stringFrom(run, "run_id"), got, want)
	}
	for runID, kind := range want {
		if got[runID] != kind {
			t.Fatalf("run %s collision with %s = %q, want %q (all: %#v)", stringFrom(run, "run_id"), runID, got[runID], kind, got)
		}
	}
	if run["collides"] != (len(want) > 0) {
		t.Fatalf("run %s collides = %v, want %v", stringFrom(run, "run_id"), run["collides"], len(want) > 0)
	}
}

func crSeedRepo(t *testing.T, ctx context.Context, runner db.Runner, repoID string, now time.Time) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,$5,$6,23,'active')`,
		repoID, "ident_"+repoID, "/tmp/"+repoID, "/tmp/"+repoID+"/.striatum", repoID, now); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha_'||$2,'{}'::jsonb,$3)`,
		repoID, "snap_"+repoID, now); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
}

// crSeedRun seeds one running run with a branch, one repo-write or document-only
// job carrying allowed_paths, and one active author session.
func crSeedRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, branch string, now time.Time, repoWrite bool, allowed []string) {
	t.Helper()
	crSeedRunState(t, ctx, runner, repoID, runID, branch, now, "running")

	mode := "document_only"
	repoWriteFlag := "false"
	if repoWrite {
		mode = "repo_write"
		repoWriteFlag = "true"
	}
	allowedJSON := "["
	for i, p := range allowed {
		if i > 0 {
			allowedJSON += ","
		}
		allowedJSON += fmt.Sprintf("%q", p)
	}
	allowedJSON += "]"
	writeScope := fmt.Sprintf(`{"mode":%q,"repo_write":%s,"allowed_paths":%s}`, mode, repoWriteFlag, allowedJSON)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,'build',1,'running','author','Build','draft',$4,$5::jsonb,'[]'::jsonb,
		         '{"lane_id":"claude"}'::jsonb,$6)`,
		repoID, "job_"+runID, runID, "idem_"+runID, writeScope, now); err != nil {
		t.Fatalf("seed job %s: %v", runID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, operator_label, registered_at
		) VALUES ($1,$2,$3,'author','claude',$4,1,'["claim","write"]'::jsonb,'active','codex',$5)`,
		repoID, "sess_"+runID, runID, runID+"-author-1", now); err != nil {
		t.Fatalf("seed session %s: %v", runID, err)
	}
}

func crSeedRunState(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, branch string, now time.Time, state string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at,
		  branch_name, branch_confirmed_at, branch_confirmed_by
		) VALUES ($1,$2,$3,'/tmp/'||$1,$4,$5,$6,$5,'human')`,
		repoID, runID, "snap_"+repoID, state, now, branch); err != nil {
		t.Fatalf("seed run %s: %v", runID, err)
	}
}
